// This is Milestone 2's closing day: rather than adding a new feature,
// it proves the milestone's features (profile stats, follow/unfollow,
// likes, comments) are backed by real Postgres state rather than any
// in-process cache, by deliberately simulating an app restart mid-test —
// a brand new *fiber.App and *pgxpool.Pool built from setupTestEnv,
// sharing nothing with the instance state was written through — and
// checking every reader still agrees afterward. There is no in-memory
// caching layer anywhere in this codebase today, so this test is meant
// to keep it that way: if one is ever added to a read path without also
// invalidating correctly, this is the test that will catch it.
package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestCrossUserPersistenceAfterRestart writes state (a follow, a like, a
// comment) via one app instance, then reads all of it back via a series
// of freshly constructed instances — simulating the server restarting
// between every step — and checks two independent users still see
// mutually consistent state throughout.
func TestCrossUserPersistenceAfterRestart(t *testing.T) {
	env1 := setupTestEnv(t)
	userA := env1.createUser(t, "persist-a")
	userB := env1.createUser(t, "persist-b")

	mustFollow(t, env1, userA, userB.PetID)

	postID := env1.insertPost(t, userB.PetID, userB.ID)
	if resp := env1.do(t, http.MethodPut, "/v1/posts/"+postID.String()+"/like", userA.Token, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("like post: status = %d, want 200", resp.StatusCode)
	}
	if resp := env1.do(t, http.MethodPost, "/v1/posts/"+postID.String()+"/comments", userA.Token, map[string]any{
		"body": "what a good pet!",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create comment: status = %d, want 201", resp.StatusCode)
	}

	// Simulate a restart: a brand new app + connection pool, sharing
	// nothing with env1 but the same underlying Postgres database.
	env2 := setupTestEnv(t)

	t.Run("userB's followers list survives the restart", func(t *testing.T) {
		resp := env2.do(t, http.MethodGet, "/v1/users/"+userB.ID.String()+"/followers", userB.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var page struct {
			Items      []struct{ ID uuid.UUID }
			TotalCount int64 `json:"total_count"`
		}
		decodeInto(t, resp, &page)
		if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].ID != userA.ID {
			t.Fatalf("followers = %+v, want exactly userA", page)
		}
	})

	t.Run("userA's profile following_count survives the restart", func(t *testing.T) {
		resp := env2.do(t, http.MethodGet, "/v1/users/"+userA.ID.String(), userA.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		profile := decodeProfile(t, resp)
		if profile.FollowingCount == nil || *profile.FollowingCount != 1 {
			t.Fatalf("following_count = %v, want 1", profile.FollowingCount)
		}
	})

	t.Run("the like and comment survive the restart, attributed to the right user", func(t *testing.T) {
		resp := env2.do(t, http.MethodGet, "/v1/posts/"+postID.String()+"/comments", userB.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var page struct {
			Items []struct {
				Body   string    `json:"body"`
				UserID uuid.UUID `json:"user_id"`
			} `json:"items"`
		}
		decodeInto(t, resp, &page)
		if len(page.Items) != 1 || page.Items[0].Body != "what a good pet!" || page.Items[0].UserID != userA.ID {
			t.Fatalf("comments = %+v, want exactly userA's comment", page.Items)
		}

		var likeCount int64
		if err := env2.db.QueryRow(t.Context(), `
			SELECT count(*) FROM post_likes WHERE post_id = $1 AND user_id = $2`,
			postID, userA.ID).Scan(&likeCount); err != nil {
			t.Fatalf("count likes: %v", err)
		}
		if likeCount != 1 {
			t.Fatalf("like rows = %d, want 1", likeCount)
		}
	})

	// Simulate a second restart, this time performing a write (unfollow)
	// through the new instance, to prove writes made post-restart are
	// themselves durable — not just reads of pre-restart writes.
	env3 := setupTestEnv(t)
	if resp := env3.do(t, http.MethodDelete, "/v1/pets/"+userB.PetID.String()+"/follow", userA.Token, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("unfollow: status = %d, want 200", resp.StatusCode)
	}

	env4 := setupTestEnv(t)
	t.Run("the unfollow written through env3 survives a third restart", func(t *testing.T) {
		resp := env4.do(t, http.MethodGet, "/v1/users/"+userA.ID.String(), userA.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		profile := decodeProfile(t, resp)
		if profile.FollowingCount == nil || *profile.FollowingCount != 0 {
			t.Fatalf("following_count = %v, want 0 after unfollow", profile.FollowingCount)
		}

		resp = env4.do(t, http.MethodGet, "/v1/users/"+userB.ID.String()+"/followers", userB.Token, nil)
		var page struct {
			TotalCount int64 `json:"total_count"`
		}
		decodeInto(t, resp, &page)
		if page.TotalCount != 0 {
			t.Fatalf("followers total_count = %d, want 0 after unfollow", page.TotalCount)
		}
	})
}

// TestPrivateAccountRestrictionSurvivesRestart is
// TestCrossUserPersistenceAfterRestart's counterpart for the privacy
// state Day 28/29 added, and specifically exercises a subtlety the
// codebase deliberately gets right: internal/platform/visibility.ForUser
// (which gates GET /users/:userId — profile.go's getPublicProfile has no
// follow exception at all, see its `response.IsSelf || !isPrivate` check)
// is intentionally *stricter* than ForPet (which gates a pet's own media
// tabs and grants a per-pet follow exception). A follower of the owner's
// *pet* must still see the owner's full account profile as restricted,
// while that same follow simultaneously unlocks that specific pet's
// media tab. Both halves of that distinction must survive a restart
// identically, or the two visibility models have drifted apart.
func TestPrivateAccountRestrictionSurvivesRestart(t *testing.T) {
	env1 := setupTestEnv(t)
	owner := env1.createUser(t, "restart-private-owner")
	follower := env1.createUser(t, "restart-private-follower")
	stranger := env1.createUser(t, "restart-private-stranger")

	env1.setPrivate(t, owner, true)
	mustFollow(t, env1, follower, owner.PetID)
	env1.insertPost(t, owner.PetID, owner.ID)

	env2 := setupTestEnv(t)

	t.Run("the owner's follower still sees a restricted account profile after the restart", func(t *testing.T) {
		resp := env2.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), follower.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		profile := decodeProfile(t, resp)
		if profile.PostCount != nil {
			t.Fatalf("expected post_count to stay hidden even from a pet-follower (ForUser grants no follow exception), got %v", *profile.PostCount)
		}
	})

	t.Run("a stranger also sees the restricted account profile after the restart", func(t *testing.T) {
		resp := env2.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), stranger.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		profile := decodeProfile(t, resp)
		if profile.PostCount != nil {
			t.Fatalf("expected post_count to stay hidden from a stranger, got %v", *profile.PostCount)
		}
		if !profile.IsPrivate {
			t.Fatalf("expected is_private = true to itself survive the restart")
		}
	})

	t.Run("but that same follower's pet-level follow still unlocks the pet's media tab after the restart", func(t *testing.T) {
		resp := env2.do(t, http.MethodGet, "/v1/pets/"+owner.PetID.String()+"/posts", follower.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		items := decodeItemsPage(t, resp)
		if len(items) != 1 {
			t.Fatalf("expected the follower to see the pet's 1 post via ForPet's follow exception, got %d items", len(items))
		}
	})

	t.Run("a non-follower stranger still sees an empty pet media tab after the restart", func(t *testing.T) {
		resp := env2.do(t, http.MethodGet, "/v1/pets/"+owner.PetID.String()+"/posts", stranger.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		items := decodeItemsPage(t, resp)
		if len(items) != 0 {
			t.Fatalf("expected a non-follower stranger to see 0 posts, got %d", len(items))
		}
	})
}
