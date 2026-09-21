// Follow/unfollow (brief Milestone 2, day after "Public pet profile &
// media tabs") — drives PUT/DELETE /v1/pets/:petId/follow against a real
// Postgres, verifying the properties that matter most for this endpoint:
// idempotency in both directions, a single notification per follow (not
// one per retried PUT), the owner/self guard, and the same block-aware
// 404 the public profile and media tabs already use.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// decodeInto unwraps httpx.OK's `{"data": ...}` envelope into target.
func decodeInto(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response envelope: %v", err)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
}

func TestFollowUnfollow(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("follow is idempotent and notifies the owner exactly once", func(t *testing.T) {
		owner := env.createUser(t, "follow-owner")
		follower := env.createUser(t, "follow-follower")

		resp := env.do(t, http.MethodPut, "/v1/pets/"+owner.PetID.String()+"/follow", follower.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("first follow: status = %d, want 200", resp.StatusCode)
		}
		var body struct {
			Following bool `json:"following"`
		}
		decodeInto(t, resp, &body)
		if !body.Following {
			t.Fatal("expected following=true")
		}

		resp = env.do(t, http.MethodPut, "/v1/pets/"+owner.PetID.String()+"/follow", follower.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("repeat follow: status = %d, want 200", resp.StatusCode)
		}
		decodeInto(t, resp, &body)
		if !body.Following {
			t.Fatal("expected following=true on a repeat follow")
		}

		var followRows int
		if err := env.db.QueryRow(context.Background(), `
			SELECT count(*) FROM follows WHERE user_id = $1 AND pet_id = $2`,
			follower.ID, owner.PetID).Scan(&followRows); err != nil {
			t.Fatalf("count follows: %v", err)
		}
		if followRows != 1 {
			t.Fatalf("follows rows = %d, want 1 (PUT must be idempotent)", followRows)
		}

		var notificationRows int
		if err := env.db.QueryRow(context.Background(), `
			SELECT count(*) FROM notifications
			WHERE user_id = $1 AND notification_type = 'follow' AND payload->>'pet_id' = $2`,
			owner.ID, owner.PetID.String()).Scan(&notificationRows); err != nil {
			t.Fatalf("count notifications: %v", err)
		}
		if notificationRows != 1 {
			t.Fatalf("follow notifications = %d, want 1 (a repeat follow must not renotify)", notificationRows)
		}
	})

	t.Run("unfollow is idempotent", func(t *testing.T) {
		owner := env.createUser(t, "unfollow-owner")
		follower := env.createUser(t, "unfollow-follower")

		// Unfollowing a pet never followed must succeed as a no-op, not error.
		resp := env.do(t, http.MethodDelete, "/v1/pets/"+owner.PetID.String()+"/follow", follower.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unfollow with no prior follow: status = %d, want 200", resp.StatusCode)
		}

		if resp := env.do(t, http.MethodPut, "/v1/pets/"+owner.PetID.String()+"/follow", follower.Token, nil); resp.StatusCode != http.StatusOK {
			t.Fatalf("follow: status = %d, want 200", resp.StatusCode)
		}

		for i := 0; i < 2; i++ {
			resp := env.do(t, http.MethodDelete, "/v1/pets/"+owner.PetID.String()+"/follow", follower.Token, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("unfollow #%d: status = %d, want 200", i+1, resp.StatusCode)
			}
			var body struct {
				Following bool `json:"following"`
			}
			decodeInto(t, resp, &body)
			if body.Following {
				t.Fatalf("unfollow #%d: expected following=false", i+1)
			}
		}

		var followRows int
		if err := env.db.QueryRow(context.Background(), `
			SELECT count(*) FROM follows WHERE user_id = $1 AND pet_id = $2`,
			follower.ID, owner.PetID).Scan(&followRows); err != nil {
			t.Fatalf("count follows: %v", err)
		}
		if followRows != 0 {
			t.Fatalf("follows rows = %d, want 0 after unfollow", followRows)
		}
	})

	t.Run("cannot follow your own pet", func(t *testing.T) {
		owner := env.createUser(t, "self-follow")
		resp := env.do(t, http.MethodPut, "/v1/pets/"+owner.PetID.String()+"/follow", owner.Token, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "cannot_follow_own_pet" {
			t.Fatalf("error code = %q, want cannot_follow_own_pet", code)
		}
	})

	t.Run("a blocked relationship 404s instead of following", func(t *testing.T) {
		owner := env.createUser(t, "blocked-owner")
		follower := env.createUser(t, "blocked-follower")
		if resp := env.do(t, http.MethodPost, "/v1/blocks", follower.Token, map[string]any{
			"user_id": owner.ID.String(),
		}); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("create block: status = %d, want 204", resp.StatusCode)
		}

		resp := env.do(t, http.MethodPut, "/v1/pets/"+owner.PetID.String()+"/follow", follower.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("follow across a block: status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("nonexistent pet returns not found", func(t *testing.T) {
		follower := env.createUser(t, "missing-pet-follower")
		resp := env.do(t, http.MethodPut, "/v1/pets/"+uuid.NewString()+"/follow", follower.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}
