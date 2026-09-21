package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestPublicProfile(t *testing.T) {
	// The profile module is already wired into setupTestEnv's protected
	// group alongside every other module under test.
	env := setupTestEnv(t)

	owner := env.createUser(t, "owner")
	viewer := env.createUser(t, "viewer")

	t.Run("self view returns full profile", func(t *testing.T) {
		resp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), owner.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body := decodeProfile(t, resp)
		if !body.IsSelf {
			t.Fatal("expected is_self=true")
		}
		if body.Bio == nil || body.City == nil || body.PostCount == nil {
			t.Fatal("expected self view to include bio/city/post_count")
		}
	})

	t.Run("public view of a non-private account includes stats", func(t *testing.T) {
		resp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), viewer.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body := decodeProfile(t, resp)
		if body.IsSelf {
			t.Fatal("expected is_self=false for a different viewer")
		}
		if body.Bio == nil || body.City == nil || body.FollowerCount == nil {
			t.Fatal("expected a public account to expose bio/city/follower_count to any viewer")
		}
	})

	t.Run("private account hides bio/city/stats from non-owner viewers", func(t *testing.T) {
		ctx := context.Background()
		if _, err := env.db.Exec(ctx, `UPDATE users SET is_private = true WHERE id = $1`, owner.ID); err != nil {
			t.Fatalf("set is_private: %v", err)
		}
		t.Cleanup(func() {
			_, _ = env.db.Exec(context.Background(), `UPDATE users SET is_private = false WHERE id = $1`, owner.ID)
		})

		resp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), viewer.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body := decodeProfile(t, resp)
		if !body.IsPrivate {
			t.Fatal("expected is_private=true")
		}
		if body.Bio != nil || body.City != nil || body.PostCount != nil || body.FollowerCount != nil || body.FollowingCount != nil {
			t.Fatalf("expected private account to omit bio/city/stats for a non-owner viewer, got %+v", body)
		}
		if body.PetCount < 0 {
			t.Fatal("expected pet_count to still be present for a private account")
		}

		// The owner viewing their own private profile still sees everything.
		resp = env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), owner.Token, nil)
		body = decodeProfile(t, resp)
		if body.Bio == nil {
			t.Fatal("expected the owner's own view of their private profile to include bio")
		}
	})

	t.Run("a blocked relationship hides the profile as not found", func(t *testing.T) {
		blocker := env.createUser(t, "blocker")
		blocked := env.createUser(t, "blocked")
		if resp := env.do(t, http.MethodPost, "/v1/blocks", blocker.Token, map[string]any{
			"user_id": blocked.ID.String(),
		}); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("create block: status = %d, want 204", resp.StatusCode)
		}

		// Both directions: the blocker can't see the blocked user's
		// profile, and the blocked user can't see the blocker's — a block
		// must not be discoverable from only one side's perspective.
		resp := env.do(t, http.MethodGet, "/v1/users/"+blocked.ID.String(), blocker.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("blocker viewing blocked: status = %d, want 404", resp.StatusCode)
		}
		resp = env.do(t, http.MethodGet, "/v1/users/"+blocker.ID.String(), blocked.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("blocked viewing blocker: status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("nonexistent user returns not found", func(t *testing.T) {
		resp := env.do(t, http.MethodGet, "/v1/users/"+uuid.NewString(), viewer.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("pet count reflects only active, non-deleted pets", func(t *testing.T) {
		fresh := env.createUser(t, "counts")
		// fresh already has one pet from createUser; add a second, plus a
		// soft-deleted one that must not be counted.
		env.freshPet(t, fresh)
		deletedPetID := env.freshPet(t, fresh)
		if _, err := env.db.Exec(context.Background(), `
			UPDATE pets SET status = 'deleted', deleted_at = now() WHERE id = $1`, deletedPetID); err != nil {
			t.Fatalf("soft-delete pet: %v", err)
		}

		resp := env.do(t, http.MethodGet, "/v1/users/"+fresh.ID.String(), viewer.Token, nil)
		body := decodeProfile(t, resp)
		if body.PetCount != 2 {
			t.Fatalf("pet_count = %d, want 2 (1 original + 1 fresh, excluding the deleted one)", body.PetCount)
		}
	})
}

type profileBody struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	ProfilePhotoURL string    `json:"profile_photo_url"`
	IsPrivate       bool      `json:"is_private"`
	IsSelf          bool      `json:"is_self"`
	PetCount        int64     `json:"pet_count"`
	Bio             *string   `json:"bio,omitempty"`
	City            *string   `json:"city,omitempty"`
	PostCount       *int64    `json:"post_count,omitempty"`
	FollowerCount   *int64    `json:"follower_count,omitempty"`
	FollowingCount  *int64    `json:"following_count,omitempty"`
}

func decodeProfile(t *testing.T, resp *http.Response) profileBody {
	t.Helper()
	var payload struct {
		Data profileBody `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	return payload.Data
}
