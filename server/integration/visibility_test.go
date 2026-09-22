// This file regression-tests the visibility-enforcement gaps found and
// fixed on the "Visibility enforcement" build day: private accounts'
// pets leaking into discovery/matching, blocked users still able to
// interact with a blocker's posts, soft-deleted pets' posts staying
// interactable forever, a story feed's follow-gate bypass, and two
// profile stat subqueries over-counting soft-deleted pets' activity.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type visibilityItem struct {
	ID uuid.UUID `json:"id"`
}

// decodeItemsPage decodes any `{"data":{"items":[{"id":...}, ...]}}`
// response — the shared envelope shape of discovery, matching, and
// stories list endpoints.
func decodeItemsPage(t *testing.T, resp *http.Response) []visibilityItem {
	t.Helper()
	var payload struct {
		Data struct {
			Items []visibilityItem `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode items page: %v", err)
	}
	return payload.Data.Items
}

func containsStoryID(t *testing.T, resp *http.Response, storyID uuid.UUID) bool {
	t.Helper()
	for _, item := range decodeItemsPage(t, resp) {
		if item.ID == storyID {
			return true
		}
	}
	return false
}

func (env *testEnv) setPrivate(t *testing.T, user testUser, private bool) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(),
		`UPDATE users SET is_private = $1 WHERE id = $2`, private, user.ID); err != nil {
		t.Fatalf("set is_private: %v", err)
	}
}

func (env *testEnv) setPetLocation(t *testing.T, petID uuid.UUID, lat, lng float64) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(), `
		UPDATE pets SET location = ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography WHERE id = $3`,
		lng, lat, petID); err != nil {
		t.Fatalf("set pet location: %v", err)
	}
}

func (env *testEnv) softDeletePet(t *testing.T, petID uuid.UUID) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(),
		`UPDATE pets SET status = 'deleted', deleted_at = now() WHERE id = $1`, petID); err != nil {
		t.Fatalf("soft-delete pet: %v", err)
	}
}

func (env *testEnv) insertPost(t *testing.T, petID, authorUserID uuid.UUID) uuid.UUID {
	t.Helper()
	postID := uuid.New()
	if _, err := env.db.Exec(context.Background(), `
		INSERT INTO posts (id, pet_id, author_user_id, caption, media_url, visibility)
		VALUES ($1, $2, $3, 'test post', 'http://example.com/x.jpg', 'public')`,
		postID, petID, authorUserID); err != nil {
		t.Fatalf("insert post: %v", err)
	}
	return postID
}

func (env *testEnv) insertStory(t *testing.T, petID, authorUserID uuid.UUID) uuid.UUID {
	t.Helper()
	storyID := uuid.New()
	if _, err := env.db.Exec(context.Background(), `
		INSERT INTO stories (id, pet_id, author_user_id, media_url)
		VALUES ($1, $2, $3, 'http://example.com/story.jpg')`,
		storyID, petID, authorUserID); err != nil {
		t.Fatalf("insert story: %v", err)
	}
	return storyID
}

func (env *testEnv) follow(t *testing.T, follower testUser, petID uuid.UUID) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(),
		`INSERT INTO follows (user_id, pet_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		follower.ID, petID); err != nil {
		t.Fatalf("follow pet: %v", err)
	}
}

// TestDiscoveryExcludesPrivateAccounts regression-tests
// internal/modules/discovery/handler.go's `nearbyPets`: a private
// account's pet must not be discoverable by a stranger, but becomes
// visible again once the viewer already follows that specific pet —
// the same exception internal/platform/visibility.ForPet grants.
func TestDiscoveryExcludesPrivateAccounts(t *testing.T) {
	env := setupTestEnv(t)
	viewer := env.createUser(t, "disc-viewer")
	stranger := env.createUser(t, "disc-private")
	env.setPrivate(t, stranger, true)
	env.setPetLocation(t, stranger.PetID, 25.2048, 55.2708)

	discoverURL := "/v1/discovery/pets?lat=25.2048&lng=55.2708&radius_km=10"

	t.Run("private stranger's pet is excluded", func(t *testing.T) {
		resp := env.do(t, http.MethodGet, discoverURL, viewer.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		items := decodeItemsPage(t, resp)
		for _, item := range items {
			if item.ID == stranger.PetID {
				t.Fatalf("private stranger's pet appeared in discovery results")
			}
		}
	})

	t.Run("private pet reappears once followed", func(t *testing.T) {
		env.follow(t, viewer, stranger.PetID)
		resp := env.do(t, http.MethodGet, discoverURL, viewer.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		items := decodeItemsPage(t, resp)
		found := false
		for _, item := range items {
			if item.ID == stranger.PetID {
				found = true
			}
		}
		if !found {
			t.Fatalf("followed private pet did not appear in discovery results")
		}
	})
}

// TestMatchingExcludesPrivateAccounts regression-tests
// internal/modules/matching/service.go's `Candidates`: a private
// account's pet is excluded from swipe candidates outright, with no
// follow-based exception (a swipe candidate is by definition a
// stranger, unlike a discovery/profile visit).
func TestMatchingExcludesPrivateAccounts(t *testing.T) {
	env := setupTestEnv(t)
	viewer := env.createUser(t, "match-viewer")
	stranger := env.createUser(t, "match-private")
	env.setPrivate(t, stranger, true)
	env.setPetLocation(t, viewer.PetID, 25.2048, 55.2708)
	env.setPetLocation(t, stranger.PetID, 25.21, 55.28)
	env.follow(t, viewer, stranger.PetID)

	resp := env.do(t, http.MethodGet, "/v1/pets/"+viewer.PetID.String()+"/candidates?max_distance_km=50", viewer.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	items := decodeItemsPage(t, resp)
	for _, item := range items {
		if item.ID == stranger.PetID {
			t.Fatalf("private stranger's pet appeared in match candidates even though followed")
		}
	}
}

// TestCanViewPostEnforcesBlocks regression-tests
// internal/modules/social/handlers.go's `canViewPost`, the shared gate
// for like/save/comment/list-comments: a block in either direction must
// make the post behave as not-found for interaction purposes, exactly
// like the main feed query already does.
func TestCanViewPostEnforcesBlocks(t *testing.T) {
	env := setupTestEnv(t)
	author := env.createUser(t, "block-author")
	blocked := env.createUser(t, "block-blocked")
	postID := env.insertPost(t, author.PetID, author.ID)

	if _, err := env.db.Exec(context.Background(),
		`INSERT INTO blocks (blocker_user_id, blocked_user_id) VALUES ($1, $2)`,
		author.ID, blocked.ID); err != nil {
		t.Fatalf("insert block: %v", err)
	}

	t.Run("blocked user cannot like the post", func(t *testing.T) {
		resp := env.do(t, http.MethodPut, "/v1/posts/"+postID.String()+"/like", blocked.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "post_not_found" {
			t.Fatalf("error code = %q, want post_not_found", code)
		}
	})

	t.Run("blocked user cannot comment on the post", func(t *testing.T) {
		resp := env.do(t, http.MethodPost, "/v1/posts/"+postID.String()+"/comments", blocked.Token, map[string]any{
			"body": "hello",
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("blocked user cannot list comments on the post", func(t *testing.T) {
		resp := env.do(t, http.MethodGet, "/v1/posts/"+postID.String()+"/comments", blocked.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}

// TestCanViewPostExcludesDeletedPet regression-tests that a post whose
// pet was later soft-deleted/paused stops being interactable — pet
// deletion doesn't cascade to posts, so without this join the post
// would otherwise stay likeable/commentable forever.
func TestCanViewPostExcludesDeletedPet(t *testing.T) {
	env := setupTestEnv(t)
	owner := env.createUser(t, "deleted-pet-owner")
	viewer := env.createUser(t, "deleted-pet-viewer")
	postID := env.insertPost(t, owner.PetID, owner.ID)

	env.softDeletePet(t, owner.PetID)

	resp := env.do(t, http.MethodPut, "/v1/posts/"+postID.String()+"/like", viewer.Token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if code := decodeErrorCode(t, resp); code != "post_not_found" {
		t.Fatalf("error code = %q, want post_not_found", code)
	}
}

// TestListStoriesEnforcesVisibility regression-tests
// internal/modules/social/handlers.go's `listStories`: it previously
// had `OR NOT EXISTS (SELECT 1 FROM follows WHERE user_id = $1)`, which
// showed every active story — including private accounts' — to any
// viewer following zero pets in the entire app. It also had no block
// check at all.
func TestListStoriesEnforcesVisibility(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("a zero-follow viewer does not see a private stranger's story", func(t *testing.T) {
		viewer := env.createUser(t, "story-zero-follow")
		stranger := env.createUser(t, "story-private")
		env.setPrivate(t, stranger, true)
		storyID := env.insertStory(t, stranger.PetID, stranger.ID)

		resp := env.do(t, http.MethodGet, "/v1/stories", viewer.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if containsStoryID(t, resp, storyID) {
			t.Fatalf("zero-follow viewer saw a private stranger's story")
		}
	})

	t.Run("a blocked user does not see the blocker's story", func(t *testing.T) {
		author := env.createUser(t, "story-block-author")
		blocked := env.createUser(t, "story-block-blocked")
		storyID := env.insertStory(t, author.PetID, author.ID)
		env.follow(t, blocked, author.PetID)
		if _, err := env.db.Exec(context.Background(),
			`INSERT INTO blocks (blocker_user_id, blocked_user_id) VALUES ($1, $2)`,
			author.ID, blocked.ID); err != nil {
			t.Fatalf("insert block: %v", err)
		}

		resp := env.do(t, http.MethodGet, "/v1/stories", blocked.Token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if containsStoryID(t, resp, storyID) {
			t.Fatalf("blocked user saw the blocker's story despite following that pet")
		}
	})
}

// TestProfileCountsExcludeDeletedPets regression-tests
// internal/modules/profile/profile.go's `getPublicProfile`: post_count
// and following_count must not count posts/follows belonging to a
// soft-deleted/paused pet, matching pet_count/follower_count's own
// (already-correct) filtering.
func TestProfileCountsExcludeDeletedPets(t *testing.T) {
	env := setupTestEnv(t)
	owner := env.createUser(t, "counts-owner")
	env.insertPost(t, owner.PetID, owner.ID)
	follower := env.createUser(t, "counts-follower")
	env.follow(t, follower, owner.PetID)

	env.softDeletePet(t, owner.PetID)

	resp := env.do(t, http.MethodGet, "/v1/users/"+owner.ID.String(), owner.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	profile := decodeProfile(t, resp)
	if profile.PostCount == nil {
		t.Fatalf("post_count missing from self-view response")
	}
	if *profile.PostCount != 0 {
		t.Fatalf("post_count = %d, want 0 (pet is soft-deleted)", *profile.PostCount)
	}

	followerResp := env.do(t, http.MethodGet, "/v1/users/"+follower.ID.String(), follower.Token, nil)
	if followerResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", followerResp.StatusCode)
	}
	followerProfile := decodeProfile(t, followerResp)
	if followerProfile.FollowingCount == nil {
		t.Fatalf("following_count missing from self-view response")
	}
	if *followerProfile.FollowingCount != 0 {
		t.Fatalf("following_count = %d, want 0 (followed pet is soft-deleted)", *followerProfile.FollowingCount)
	}
}
