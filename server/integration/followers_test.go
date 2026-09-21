// Followers/following lists (brief Milestone 2, the day after "Follow/
// unfollow") — drives GET /v1/users/:userId/{followers,following}
// against a real Postgres, checking they agree with the exact same
// aggregate GET /v1/users/:userId already reports as follower_count/
// following_count (profile.go computes both independently; these tests
// are what catches the two drifting apart), plus pagination, privacy,
// and block behavior.
package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestFollowersFollowingLists(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("followers list matches the profile's follower_count and dedupes across pets", func(t *testing.T) {
		owner := env.createUser(t, "flw-owner")
		secondPet := env.freshPet(t, owner)
		followerA := env.createUser(t, "flw-a")
		followerB := env.createUser(t, "flw-b")

		// followerA follows both of the owner's pets — must count once as
		// a follower, not twice.
		mustFollow(t, env, followerA, owner.PetID)
		mustFollow(t, env, followerA, secondPet)
		mustFollow(t, env, followerB, owner.PetID)

		resp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String()+"/followers", owner.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var page struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
			TotalCount int64 `json:"total_count"`
		}
		decodeInto(t, resp, &page)
		if page.TotalCount != 2 {
			t.Fatalf("total_count = %d, want 2 (deduped across 2 pets)", page.TotalCount)
		}
		if len(page.Items) != 2 {
			t.Fatalf("items = %d, want 2", len(page.Items))
		}

		// Cross-check against the profile endpoint's own count.
		profileResp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), owner.Token, nil)
		profile := decodeProfile(t, profileResp)
		if profile.FollowerCount == nil || *profile.FollowerCount != page.TotalCount {
			t.Fatalf("profile follower_count = %v, followers list total_count = %d — must agree", profile.FollowerCount, page.TotalCount)
		}
	})

	t.Run("following list matches the profile's following_count", func(t *testing.T) {
		owner := env.createUser(t, "flg-owner")
		petOne := env.freshPet(t, owner)
		petTwo := env.freshPet(t, owner)
		follower := env.createUser(t, "flg-follower")

		mustFollow(t, env, follower, petOne)
		mustFollow(t, env, follower, petTwo)

		resp := env.do(t, http.MethodGet, "/v1/users/"+follower.ID.String()+"/following", follower.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var page struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
			TotalCount int64 `json:"total_count"`
		}
		decodeInto(t, resp, &page)
		if page.TotalCount != 2 {
			t.Fatalf("total_count = %d, want 2", page.TotalCount)
		}

		profileResp := env.do(t, http.MethodGet, "/v1/users/"+follower.ID.String(), follower.Token, nil)
		profile := decodeProfile(t, profileResp)
		if profile.FollowingCount == nil || *profile.FollowingCount != page.TotalCount {
			t.Fatalf("profile following_count = %v, following list total_count = %d — must agree", profile.FollowingCount, page.TotalCount)
		}
	})

	t.Run("pagination returns every item exactly once across pages", func(t *testing.T) {
		owner := env.createUser(t, "page-owner")
		follower := env.createUser(t, "page-follower")
		petIDs := map[uuid.UUID]bool{owner.PetID: true}
		for i := 0; i < 4; i++ {
			petIDs[env.freshPet(t, owner)] = true
		}
		for petID := range petIDs {
			mustFollow(t, env, follower, petID)
		}

		seen := map[uuid.UUID]bool{}
		cursor := ""
		for page := 0; page < 10; page++ {
			path := "/v1/users/" + follower.ID.String() + "/following?limit=2"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			resp := env.do(t, http.MethodGet, path, follower.Token, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("page %d: status = %d, want 200", page, resp.StatusCode)
			}
			var body struct {
				Items []struct {
					ID uuid.UUID `json:"id"`
				} `json:"items"`
				NextCursor string `json:"next_cursor"`
			}
			decodeInto(t, resp, &body)
			for _, item := range body.Items {
				if seen[item.ID] {
					t.Fatalf("pet %s returned twice across pages", item.ID)
				}
				seen[item.ID] = true
			}
			if body.NextCursor == "" {
				break
			}
			cursor = body.NextCursor
		}
		if len(seen) != len(petIDs) {
			t.Fatalf("saw %d distinct pets across all pages, want %d", len(seen), len(petIDs))
		}
	})

	t.Run("a private account's lists are hidden from non-owner viewers", func(t *testing.T) {
		owner := env.createUser(t, "priv-owner")
		follower := env.createUser(t, "priv-follower")
		mustFollow(t, env, follower, owner.PetID)

		if _, err := env.db.Exec(context.Background(), `UPDATE users SET is_private = true WHERE id = $1`, owner.ID); err != nil {
			t.Fatalf("set is_private: %v", err)
		}
		t.Cleanup(func() {
			_, _ = env.db.Exec(context.Background(), `UPDATE users SET is_private = false WHERE id = $1`, owner.ID)
		})

		resp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String()+"/followers", follower.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var page struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
			Restricted bool `json:"restricted"`
		}
		decodeInto(t, resp, &page)
		if !page.Restricted || len(page.Items) != 0 {
			t.Fatalf("expected an empty, restricted list for a non-owner viewer, got %+v", page)
		}

		// The owner viewing their own list still sees it. A fresh variable,
		// not a reused `page`: the server omits `restricted` entirely when
		// false (omitempty), so decoding into an already-`true` struct
		// would leave the stale value in place rather than overwriting it.
		resp = env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String()+"/followers", owner.Token, nil)
		var ownView struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
			Restricted bool `json:"restricted"`
		}
		decodeInto(t, resp, &ownView)
		if ownView.Restricted || len(ownView.Items) != 1 {
			t.Fatalf("expected the owner's own view to be unrestricted, got %+v", ownView)
		}
	})

	t.Run("a blocked relationship hides the lists as not found", func(t *testing.T) {
		owner := env.createUser(t, "blk-owner")
		blocker := env.createUser(t, "blk-viewer")
		if resp := env.do(t, http.MethodPost, "/v1/blocks", blocker.Token, map[string]any{
			"user_id": owner.ID.String(),
		}); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("create block: status = %d, want 204", resp.StatusCode)
		}

		for _, path := range []string{"/followers", "/following"} {
			resp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String()+path, blocker.Token, nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("%s: status = %d, want 404", path, resp.StatusCode)
			}
		}
	})

	t.Run("nonexistent user returns not found", func(t *testing.T) {
		viewer := env.createUser(t, "missing-user-viewer")
		for _, path := range []string{"/followers", "/following"} {
			resp := env.do(t, http.MethodGet, "/v1/users/"+uuid.NewString()+path, viewer.Token, nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("%s: status = %d, want 404", path, resp.StatusCode)
			}
		}
	})
}

func mustFollow(t *testing.T, env *testEnv, follower testUser, petID uuid.UUID) {
	t.Helper()
	resp := env.do(t, http.MethodPut, "/v1/pets/"+petID.String()+"/follow", follower.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("follow %s -> %s: status = %d, want 200", follower.ID, petID, resp.StatusCode)
	}
}
