// Carousel post API (brief Milestone 3, "Carousel post API") — drives
// POST/PATCH/DELETE /v1/posts against a real Postgres: an ordered
// media_ids carousel on create, replacing that carousel wholesale on
// edit alongside plain field updates, author-only mutation, and soft
// delete.
package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func (env *testEnv) insertMedia(t *testing.T, owner testUser, mediaType, storagePath string) uuid.UUID {
	t.Helper()
	mediaID := uuid.New()
	if _, err := env.db.Exec(context.Background(), `
		INSERT INTO media (id, owner_user_id, media_type, storage_path, content_type, byte_size)
		VALUES ($1, $2, $3, $4, $5, 100)`,
		mediaID, owner.ID, mediaType, storagePath, contentTypeFor(mediaType)); err != nil {
		t.Fatalf("insert media: %v", err)
	}
	return mediaID
}

func contentTypeFor(mediaType string) string {
	if mediaType == "video" {
		return "video/mp4"
	}
	return "image/jpeg"
}

type postBody struct {
	ID        uuid.UUID `json:"id"`
	MediaURL  string    `json:"media_url"`
	MediaType string    `json:"media_type"`
	Caption   string    `json:"caption"`
	Media     []struct {
		ID        uuid.UUID `json:"id"`
		MediaURL  string    `json:"media_url"`
		MediaType string    `json:"media_type"`
	} `json:"media"`
}

func TestCreatePostWithCarousel(t *testing.T) {
	env := setupTestEnv(t)
	owner := env.createUser(t, "carousel-owner")
	imageID := env.insertMedia(t, owner, "image", "uploads/x/a.jpg")
	videoID := env.insertMedia(t, owner, "video", "uploads/x/b.mp4")

	resp := env.do(t, http.MethodPost, "/v1/posts", owner.Token, map[string]any{
		"pet_id":    owner.PetID.String(),
		"caption":   "a carousel",
		"media_ids": []string{imageID.String(), videoID.String()},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var post postBody
	decodeInto(t, resp, &post)

	if len(post.Media) != 2 {
		t.Fatalf("media items = %d, want 2", len(post.Media))
	}
	if post.Media[0].ID != imageID || post.Media[1].ID != videoID {
		t.Fatalf("media order = %+v, want [%s, %s]", post.Media, imageID, videoID)
	}
	if post.MediaURL != post.Media[0].MediaURL || post.MediaType != "image" {
		t.Fatalf("legacy media_url/media_type = %q/%q, want to mirror the first carousel item %q/image", post.MediaURL, post.MediaType, post.Media[0].MediaURL)
	}

	var positions []int
	rows, err := env.db.Query(context.Background(), `
		SELECT position FROM post_media WHERE post_id = $1 ORDER BY position`, post.ID)
	if err != nil {
		t.Fatalf("query post_media: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var position int
		if err := rows.Scan(&position); err != nil {
			t.Fatalf("scan position: %v", err)
		}
		positions = append(positions, position)
	}
	if len(positions) != 2 || positions[0] != 0 || positions[1] != 1 {
		t.Fatalf("post_media positions = %v, want [0 1]", positions)
	}
}

func TestCreatePostCarouselValidation(t *testing.T) {
	env := setupTestEnv(t)
	owner := env.createUser(t, "carousel-valid-owner")
	stranger := env.createUser(t, "carousel-valid-stranger")
	ownMedia := env.insertMedia(t, owner, "image", "uploads/x/mine.jpg")
	strangerMedia := env.insertMedia(t, stranger, "image", "uploads/x/theirs.jpg")

	t.Run("too many media items", func(t *testing.T) {
		ids := make([]string, 11)
		for i := range ids {
			ids[i] = ownMedia.String()
		}
		resp := env.do(t, http.MethodPost, "/v1/posts", owner.Token, map[string]any{
			"pet_id": owner.PetID.String(), "media_ids": ids,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "too_many_media_items" {
			t.Fatalf("code = %q, want too_many_media_items", code)
		}
	})

	t.Run("invalid media id", func(t *testing.T) {
		resp := env.do(t, http.MethodPost, "/v1/posts", owner.Token, map[string]any{
			"pet_id": owner.PetID.String(), "media_ids": []string{"not-a-uuid"},
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "invalid_media_id" {
			t.Fatalf("code = %q, want invalid_media_id", code)
		}
	})

	t.Run("unowned media id", func(t *testing.T) {
		resp := env.do(t, http.MethodPost, "/v1/posts", owner.Token, map[string]any{
			"pet_id": owner.PetID.String(), "media_ids": []string{strangerMedia.String()},
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "media_not_owned" {
			t.Fatalf("code = %q, want media_not_owned", code)
		}
	})

	t.Run("reels cannot use a carousel", func(t *testing.T) {
		resp := env.do(t, http.MethodPost, "/v1/reels", owner.Token, map[string]any{
			"pet_id": owner.PetID.String(), "media_ids": []string{ownMedia.String()},
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "reel_no_carousel" {
			t.Fatalf("code = %q, want reel_no_carousel", code)
		}
	})
}

func TestPatchPostUpdatesFieldsAndCarousel(t *testing.T) {
	env := setupTestEnv(t)
	owner := env.createUser(t, "patch-owner")
	stranger := env.createUser(t, "patch-stranger")
	firstMedia := env.insertMedia(t, owner, "image", "uploads/x/first.jpg")
	secondMedia := env.insertMedia(t, owner, "image", "uploads/x/second.jpg")
	thirdMedia := env.insertMedia(t, owner, "image", "uploads/x/third.jpg")

	createResp := env.do(t, http.MethodPost, "/v1/posts", owner.Token, map[string]any{
		"pet_id": owner.PetID.String(), "caption": "original", "media_ids": []string{firstMedia.String()},
	})
	var created postBody
	decodeInto(t, createResp, &created)

	t.Run("editing caption alone leaves media untouched", func(t *testing.T) {
		resp := env.do(t, http.MethodPatch, "/v1/posts/"+created.ID.String(), owner.Token, map[string]any{
			"caption": "edited caption",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var patched postBody
		decodeInto(t, resp, &patched)
		if patched.Caption != "edited caption" {
			t.Fatalf("caption = %q, want %q", patched.Caption, "edited caption")
		}
		if len(patched.Media) != 1 || patched.Media[0].ID != firstMedia {
			t.Fatalf("media = %+v, want unchanged [%s]", patched.Media, firstMedia)
		}
	})

	t.Run("replacing media_ids replaces the whole carousel", func(t *testing.T) {
		resp := env.do(t, http.MethodPatch, "/v1/posts/"+created.ID.String(), owner.Token, map[string]any{
			"media_ids": []string{secondMedia.String(), thirdMedia.String()},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var patched postBody
		decodeInto(t, resp, &patched)
		if len(patched.Media) != 2 || patched.Media[0].ID != secondMedia || patched.Media[1].ID != thirdMedia {
			t.Fatalf("media = %+v, want [%s, %s]", patched.Media, secondMedia, thirdMedia)
		}
		if patched.MediaURL != patched.Media[0].MediaURL {
			t.Fatalf("legacy media_url = %q, want to mirror the new first item %q", patched.MediaURL, patched.Media[0].MediaURL)
		}

		var oldRows int
		if err := env.db.QueryRow(context.Background(), `
			SELECT count(*) FROM post_media WHERE post_id = $1 AND media_id = $2`,
			created.ID, firstMedia).Scan(&oldRows); err != nil {
			t.Fatalf("count old post_media rows: %v", err)
		}
		if oldRows != 0 {
			t.Fatalf("old carousel item still linked after replacement, want 0 rows")
		}
	})

	t.Run("an empty patch is rejected", func(t *testing.T) {
		resp := env.do(t, http.MethodPatch, "/v1/posts/"+created.ID.String(), owner.Token, map[string]any{})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "empty_patch" {
			t.Fatalf("code = %q, want empty_patch", code)
		}
	})

	t.Run("a non-owner cannot edit the post", func(t *testing.T) {
		resp := env.do(t, http.MethodPatch, "/v1/posts/"+created.ID.String(), stranger.Token, map[string]any{
			"caption": "hijacked",
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDeletePost(t *testing.T) {
	env := setupTestEnv(t)
	owner := env.createUser(t, "delete-owner")
	stranger := env.createUser(t, "delete-stranger")
	postID := env.insertPost(t, owner.PetID, owner.ID)

	t.Run("a non-owner cannot delete the post", func(t *testing.T) {
		resp := env.do(t, http.MethodDelete, "/v1/posts/"+postID.String(), stranger.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("the owner can delete the post, and it disappears from the pet's tab", func(t *testing.T) {
		resp := env.do(t, http.MethodDelete, "/v1/posts/"+postID.String(), owner.Token, nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", resp.StatusCode)
		}

		tabResp := env.do(t, http.MethodGet, "/v1/pets/"+owner.PetID.String()+"/posts", owner.Token, nil)
		for _, item := range decodeItemsPage(t, tabResp) {
			if item.ID == postID {
				t.Fatalf("deleted post still appears in the pet's posts tab")
			}
		}
	})

	t.Run("deleting an already-deleted post is a 404, not idempotent success", func(t *testing.T) {
		resp := env.do(t, http.MethodDelete, "/v1/posts/"+postID.String(), owner.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}
