package social

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/visibility"
)

type Handler struct {
	db *pgxpool.Pool
}

func (h *Handler) listFeed(c *fiber.Ctx) error {
	return h.listPostsByKind(c, "post")
}

func (h *Handler) listReels(c *fiber.Ctx) error {
	return h.listPostsByKind(c, "reel")
}

func (h *Handler) listPostsByKind(c *fiber.Ctx, kind string) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	limit, err := pageSize(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}

	var cursorTime any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeTimeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorTime, cursorID = cursor.CreatedAt, cursor.ID
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT p.id, p.pet_id, pet.name, pet.primary_image_url,
		       p.author_user_id, author.name, p.kind, p.caption,
		       p.location_name, p.media_url, p.media_type, p.visibility,
		       (SELECT count(*) FROM post_likes pl WHERE pl.post_id = p.id),
		       (SELECT count(*) FROM comments cm WHERE cm.post_id = p.id AND cm.deleted_at IS NULL),
		       EXISTS (SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1),
		       EXISTS (SELECT 1 FROM post_saves ps WHERE ps.post_id = p.id AND ps.user_id = $1),
		       EXISTS (SELECT 1 FROM follows f WHERE f.user_id = $1 AND f.pet_id = p.pet_id),
		       p.created_at
		FROM posts p
		JOIN pets pet ON pet.id = p.pet_id
		JOIN users author ON author.id = p.author_user_id
		WHERE p.deleted_at IS NULL
		  AND pet.deleted_at IS NULL
		  AND pet.status = 'active'
		  AND p.kind = $2
		  AND (
		    p.visibility = 'public'
		    OR p.author_user_id = $1
		    OR EXISTS (SELECT 1 FROM follows f WHERE f.user_id = $1 AND f.pet_id = p.pet_id)
		  )
		  -- Blocking is enforced both directions (neither party sees the
		  -- other's posts regardless of who pressed "block"); muting only
		  -- hides content in the muter's own feed, per brief safety
		  -- primitives — see internal/modules/safety.
		  AND NOT EXISTS (
		    SELECT 1 FROM blocks bl
		    WHERE (bl.blocker_user_id = $1 AND bl.blocked_user_id = p.author_user_id)
		       OR (bl.blocker_user_id = p.author_user_id AND bl.blocked_user_id = $1)
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM mutes mu
		    WHERE mu.muter_user_id = $1 AND mu.muted_user_id = p.author_user_id
		  )
		  AND ($3::timestamptz IS NULL OR (p.created_at, p.id) < ($3::timestamptz, $4::uuid))
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $5`, userID, kind, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "social_query_failed", "The social feed could not be loaded.")
	}
	defer rows.Close()

	items := make([]Post, 0, limit)
	for rows.Next() {
		var item Post
		if err := rows.Scan(
			&item.ID, &item.PetID, &item.PetName, &item.PetImageURL,
			&item.AuthorUserID, &item.AuthorName, &item.Kind, &item.Caption,
			&item.LocationName, &item.MediaURL, &item.MediaType, &item.Visibility,
			&item.LikeCount, &item.CommentCount, &item.LikedByMe, &item.SavedByMe,
			&item.FollowedByMe, &item.CreatedAt,
		); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "social_query_failed", "The social feed could not be loaded.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "social_query_failed", "The social feed could not be loaded.")
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeTimeCursor(last.CreatedAt, last.ID)
	}
	return httpx.OK(c, postPage{Items: items, NextCursor: nextCursor})
}

func (h *Handler) listPetPosts(c *fiber.Ctx) error {
	return h.listPetPostsByKind(c, "post")
}

func (h *Handler) listPetReels(c *fiber.Ctx) error {
	return h.listPetPostsByKind(c, "reel")
}

// listPetPostsByKind serves one pet's own posts/reels tab — brief
// Milestone 2: "Public pet profile & media tabs". Deliberately its own
// query rather than a parameterized variant of listPostsByKind: that
// query's inclusion rule is "everything public, own, or followed across
// every pet"; this one is "this one pet's posts, subject to its own
// visibility/follow rule" — different enough shapes that forcing them
// through one function would obscure both. Muting is skipped here on
// purpose (unlike the global feed): muting only declutters a viewer's
// own passive feed, it doesn't block a deliberate visit to a profile —
// blocking, via visibility.ForPet, still applies.
func (h *Handler) listPetPostsByKind(c *fiber.Ctx, kind string) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	petID, err := httpx.UUIDParam(c, "petId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "pet id must be a valid UUID.")
	}
	limit, err := pageSize(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}

	access, err := visibility.ForPet(c.UserContext(), h.db, petID, userID)
	if errors.Is(err, visibility.ErrNotFound) {
		return httpx.Problem(c, fiber.StatusNotFound, "pet_not_found", "The pet was not found.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "social_query_failed", "The pet's media could not be loaded.")
	}
	if access.Restricted {
		return httpx.OK(c, postPage{Items: []Post{}})
	}

	var cursorTime any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeTimeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorTime, cursorID = cursor.CreatedAt, cursor.ID
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT p.id, p.pet_id, pet.name, pet.primary_image_url,
		       p.author_user_id, author.name, p.kind, p.caption,
		       p.location_name, p.media_url, p.media_type, p.visibility,
		       (SELECT count(*) FROM post_likes pl WHERE pl.post_id = p.id),
		       (SELECT count(*) FROM comments cm WHERE cm.post_id = p.id AND cm.deleted_at IS NULL),
		       EXISTS (SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1),
		       EXISTS (SELECT 1 FROM post_saves ps WHERE ps.post_id = p.id AND ps.user_id = $1),
		       EXISTS (SELECT 1 FROM follows f WHERE f.user_id = $1 AND f.pet_id = p.pet_id),
		       p.created_at
		FROM posts p
		JOIN pets pet ON pet.id = p.pet_id
		JOIN users author ON author.id = p.author_user_id
		WHERE p.pet_id = $2
		  AND p.deleted_at IS NULL
		  AND p.kind = $3
		  AND (
		    p.visibility = 'public'
		    OR p.author_user_id = $1
		    OR EXISTS (SELECT 1 FROM follows f WHERE f.user_id = $1 AND f.pet_id = p.pet_id)
		  )
		  AND ($4::timestamptz IS NULL OR (p.created_at, p.id) < ($4::timestamptz, $5::uuid))
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $6`, userID, petID, kind, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "social_query_failed", "The pet's media could not be loaded.")
	}
	defer rows.Close()

	items := make([]Post, 0, limit)
	for rows.Next() {
		var item Post
		if err := rows.Scan(
			&item.ID, &item.PetID, &item.PetName, &item.PetImageURL,
			&item.AuthorUserID, &item.AuthorName, &item.Kind, &item.Caption,
			&item.LocationName, &item.MediaURL, &item.MediaType, &item.Visibility,
			&item.LikeCount, &item.CommentCount, &item.LikedByMe, &item.SavedByMe,
			&item.FollowedByMe, &item.CreatedAt,
		); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "social_query_failed", "The pet's media could not be loaded.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "social_query_failed", "The pet's media could not be loaded.")
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeTimeCursor(last.CreatedAt, last.ID)
	}
	return httpx.OK(c, postPage{Items: items, NextCursor: nextCursor})
}

func (h *Handler) createPost(c *fiber.Ctx) error {
	return h.createPostByKind(c, "post")
}

func (h *Handler) createReel(c *fiber.Ctx) error {
	return h.createPostByKind(c, "reel")
}

func (h *Handler) createPostByKind(c *fiber.Ctx, kind string) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	var request createPostRequest
	if err := c.BodyParser(&request); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body must be valid JSON.")
	}
	petID, err := uuid.Parse(strings.TrimSpace(request.PetID))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "pet_id must be a valid UUID.")
	}
	request.Caption = strings.TrimSpace(request.Caption)
	request.LocationName = strings.TrimSpace(request.LocationName)
	request.MediaURL = strings.TrimSpace(request.MediaURL)
	request.MediaType = strings.ToLower(strings.TrimSpace(request.MediaType))
	request.Visibility = strings.ToLower(strings.TrimSpace(request.Visibility))
	if request.MediaType == "" {
		request.MediaType = "image"
	}
	if request.Visibility == "" {
		request.Visibility = "public"
	}
	if utf8.RuneCountInString(request.Caption) > 2200 {
		return httpx.Problem(c, fiber.StatusBadRequest, "caption_too_long", "caption must contain at most 2200 characters.")
	}
	if utf8.RuneCountInString(request.LocationName) > 200 {
		return httpx.Problem(c, fiber.StatusBadRequest, "location_too_long", "location_name must contain at most 200 characters.")
	}
	if !validMediaURL(request.MediaURL) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_media_url", "media_url must be an HTTP(S) URL or an application-relative media path.")
	}
	if request.MediaType != "image" && request.MediaType != "video" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_media_type", "media_type must be image or video.")
	}
	if kind == "reel" && request.MediaType != "video" {
		return httpx.Problem(c, fiber.StatusBadRequest, "reel_requires_video", "A reel must use video media.")
	}
	if request.Visibility != "public" && request.Visibility != "followers" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_visibility", "visibility must be public or followers.")
	}

	var postID uuid.UUID
	err = h.db.QueryRow(c.UserContext(), `
		INSERT INTO posts (pet_id, author_user_id, kind, caption, location_name, media_url, media_type, visibility)
		SELECT p.id, $1, $3, $4, $5, $6, $7, $8
		FROM pets p
		WHERE p.id = $2 AND p.owner_id = $1 AND p.deleted_at IS NULL AND p.status <> 'deleted'
		RETURNING id`, userID, petID, kind, request.Caption, request.LocationName, request.MediaURL, request.MediaType, request.Visibility).Scan(&postID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusForbidden, "pet_not_owned", "The selected pet does not belong to the authenticated user.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "post_create_failed", "The post could not be created.")
	}
	post, err := h.postByID(c, userID, postID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "post_create_failed", "The post was created but could not be loaded.")
	}
	return httpx.Created(c, post)
}

func (h *Handler) postByID(c *fiber.Ctx, userID, postID uuid.UUID) (Post, error) {
	var item Post
	err := h.db.QueryRow(c.UserContext(), `
		SELECT p.id, p.pet_id, pet.name, pet.primary_image_url,
		       p.author_user_id, author.name, p.kind, p.caption,
		       p.location_name, p.media_url, p.media_type, p.visibility,
		       (SELECT count(*) FROM post_likes pl WHERE pl.post_id = p.id),
		       (SELECT count(*) FROM comments cm WHERE cm.post_id = p.id AND cm.deleted_at IS NULL),
		       EXISTS (SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1),
		       EXISTS (SELECT 1 FROM post_saves ps WHERE ps.post_id = p.id AND ps.user_id = $1),
		       p.created_at
		FROM posts p
		JOIN pets pet ON pet.id = p.pet_id
		JOIN users author ON author.id = p.author_user_id
		WHERE p.id = $2 AND p.deleted_at IS NULL`, userID, postID).Scan(
		&item.ID, &item.PetID, &item.PetName, &item.PetImageURL,
		&item.AuthorUserID, &item.AuthorName, &item.Kind, &item.Caption,
		&item.LocationName, &item.MediaURL, &item.MediaType, &item.Visibility,
		&item.LikeCount, &item.CommentCount, &item.LikedByMe, &item.SavedByMe,
		&item.CreatedAt,
	)
	return item, err
}

// likePost has its own implementation rather than routing through
// setPostRelation like every other post relation: a like is the one
// action here ADR 0001 covers (alongside comments) — it can optionally
// carry an actor_pet_id, which post_saves has no use for (saving is a
// private bookmarking action with nothing to display an identity for).
func (h *Handler) likePost(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	postID, err := httpx.UUIDParam(c, "postId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_post_id", "postId must be a valid UUID.")
	}
	visible, err := h.canViewPost(c, userID, postID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "post_query_failed", "The post could not be loaded.")
	}
	if !visible {
		return httpx.Problem(c, fiber.StatusNotFound, "post_not_found", "The requested post was not found.")
	}

	var request likeRequest
	// A PUT with no body is the common case (liking carries no
	// attribution most of the time); BodyParser only errors on genuinely
	// malformed JSON, not an absent/empty body, so this is safe to ignore.
	_ = c.BodyParser(&request)
	actorPetID, fieldErr, err := resolveOwnedActorPet(c.UserContext(), h.db, userID, request.ActorPetID)
	if fieldErr != nil {
		return httpx.Problem(c, fieldErr.status, fieldErr.field, fieldErr.message)
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "post_update_failed", "The post could not be updated.")
	}

	if _, err := h.db.Exec(c.UserContext(), `
		INSERT INTO post_likes (user_id, post_id, actor_pet_id) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, post_id) DO UPDATE SET actor_pet_id = EXCLUDED.actor_pet_id`,
		userID, postID, actorPetID); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "post_update_failed", "The post could not be updated.")
	}
	return httpx.OK(c, fiber.Map{"liked": true})
}

// resolveOwnedActorPet parses an optional, client-supplied pet id and
// confirms it belongs to userID — the concrete enforcement ADR 0001
// requires of "a user may never act through a pet they do not own",
// shared by likePost and createComment so both check it identically. An
// empty rawPetID is valid and means "no pet attribution" (nil, nil, nil).
func resolveOwnedActorPet(ctx context.Context, db *pgxpool.Pool, userID uuid.UUID, rawPetID string) (petID *uuid.UUID, fieldErr *actorPetError, err error) {
	trimmed := strings.TrimSpace(rawPetID)
	if trimmed == "" {
		return nil, nil, nil
	}
	parsed, parseErr := uuid.Parse(trimmed)
	if parseErr != nil {
		return nil, &actorPetError{fiber.StatusBadRequest, "invalid_actor_pet_id", "actor_pet_id must be a valid UUID."}, nil
	}
	var owned bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pets WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL)`,
		parsed, userID).Scan(&owned); err != nil {
		return nil, nil, err
	}
	if !owned {
		return nil, &actorPetError{fiber.StatusForbidden, "pet_not_owned", "The selected pet does not belong to the authenticated user."}, nil
	}
	return &parsed, nil, nil
}

type actorPetError struct {
	status  int
	field   string
	message string
}

func (h *Handler) unlikePost(c *fiber.Ctx) error {
	return h.setPostRelation(c, "post_likes", "liked", false)
}

func (h *Handler) savePost(c *fiber.Ctx) error {
	return h.setPostRelation(c, "post_saves", "saved", true)
}

func (h *Handler) unsavePost(c *fiber.Ctx) error {
	return h.setPostRelation(c, "post_saves", "saved", false)
}

func (h *Handler) setPostRelation(c *fiber.Ctx, table, responseField string, enabled bool) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	postID, err := httpx.UUIDParam(c, "postId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_post_id", "postId must be a valid UUID.")
	}
	visible, err := h.canViewPost(c, userID, postID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "post_query_failed", "The post could not be loaded.")
	}
	if !visible {
		return httpx.Problem(c, fiber.StatusNotFound, "post_not_found", "The requested post was not found.")
	}

	if enabled {
		_, err = h.db.Exec(c.UserContext(), "INSERT INTO "+table+" (user_id, post_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, postID)
	} else {
		_, err = h.db.Exec(c.UserContext(), "DELETE FROM "+table+" WHERE user_id = $1 AND post_id = $2", userID, postID)
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "post_update_failed", "The post could not be updated.")
	}
	return httpx.OK(c, fiber.Map{responseField: enabled})
}

// canViewPost is the shared gate for likePost/unlikePost/savePost/
// unsavePost/listComments/createComment — it must enforce everything the
// main feed query (listPostsByKind) does, since a post reachable there
// must not become un-interactable-with here and vice versa: the pet's
// own deleted/status check (a soft-deleted/paused pet's posts otherwise
// stay likeable/commentable forever, since pet deletion doesn't cascade
// to posts) and the bidirectional blocks check.
func (h *Handler) canViewPost(c *fiber.Ctx, userID, postID uuid.UUID) (bool, error) {
	var visible bool
	err := h.db.QueryRow(c.UserContext(), `
		SELECT EXISTS (
		  SELECT 1 FROM posts p
		  JOIN pets pet ON pet.id = p.pet_id AND pet.deleted_at IS NULL AND pet.status = 'active'
		  WHERE p.id = $2 AND p.deleted_at IS NULL
		    AND (p.visibility = 'public' OR p.author_user_id = $1
		      OR EXISTS (SELECT 1 FROM follows f WHERE f.user_id = $1 AND f.pet_id = p.pet_id))
		    AND NOT EXISTS (
		      SELECT 1 FROM blocks bl
		      WHERE (bl.blocker_user_id = $1 AND bl.blocked_user_id = p.author_user_id)
		         OR (bl.blocker_user_id = p.author_user_id AND bl.blocked_user_id = $1)
		    )
		)`, userID, postID).Scan(&visible)
	return visible, err
}

func (h *Handler) listComments(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	postID, err := httpx.UUIDParam(c, "postId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_post_id", "postId must be a valid UUID.")
	}
	visible, err := h.canViewPost(c, userID, postID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "comment_query_failed", "Comments could not be loaded.")
	}
	if !visible {
		return httpx.Problem(c, fiber.StatusNotFound, "post_not_found", "The requested post was not found.")
	}
	limit, err := pageSize(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}
	var cursorTime any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeTimeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorTime, cursorID = cursor.CreatedAt, cursor.ID
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT cm.id, cm.post_id, cm.user_id, cm.actor_pet_id,
		       COALESCE(pet.name, u.name), COALESCE(pet.primary_image_url, u.profile_photo_url),
		       cm.body, cm.created_at, cm.updated_at
		FROM comments cm
		JOIN users u ON u.id = cm.user_id
		LEFT JOIN pets pet ON pet.id = cm.actor_pet_id AND pet.deleted_at IS NULL
		WHERE cm.post_id = $1 AND cm.deleted_at IS NULL
		  AND ($2::timestamptz IS NULL OR (cm.created_at, cm.id) < ($2::timestamptz, $3::uuid))
		ORDER BY cm.created_at DESC, cm.id DESC
		LIMIT $4`, postID, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "comment_query_failed", "Comments could not be loaded.")
	}
	defer rows.Close()

	items := make([]Comment, 0, limit)
	for rows.Next() {
		var item Comment
		if err := rows.Scan(&item.ID, &item.PostID, &item.UserID, &item.ActorPetID, &item.AuthorName, &item.AuthorPhotoURL, &item.Body, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "comment_query_failed", "Comments could not be loaded.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "comment_query_failed", "Comments could not be loaded.")
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeTimeCursor(last.CreatedAt, last.ID)
	}
	return httpx.OK(c, commentPage{Items: items, NextCursor: nextCursor})
}

func (h *Handler) createComment(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	postID, err := httpx.UUIDParam(c, "postId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_post_id", "postId must be a valid UUID.")
	}
	var request createCommentRequest
	if err := c.BodyParser(&request); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body must be valid JSON.")
	}
	request.Body = strings.TrimSpace(request.Body)
	if length := utf8.RuneCountInString(request.Body); length < 1 || length > 2000 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_comment", "body must contain between 1 and 2000 characters.")
	}
	visible, err := h.canViewPost(c, userID, postID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "comment_create_failed", "The comment could not be created.")
	}
	if !visible {
		return httpx.Problem(c, fiber.StatusNotFound, "post_not_found", "The requested post was not found.")
	}
	actorPetID, fieldErr, err := resolveOwnedActorPet(c.UserContext(), h.db, userID, request.ActorPetID)
	if fieldErr != nil {
		return httpx.Problem(c, fieldErr.status, fieldErr.field, fieldErr.message)
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "comment_create_failed", "The comment could not be created.")
	}

	var item Comment
	err = h.db.QueryRow(c.UserContext(), `
		WITH inserted AS (
		  INSERT INTO comments (post_id, user_id, body, actor_pet_id)
		  VALUES ($1, $2, $3, $4)
		  RETURNING id, post_id, user_id, actor_pet_id, body, created_at, updated_at
		)
		SELECT i.id, i.post_id, i.user_id, i.actor_pet_id,
		       COALESCE(pet.name, u.name), COALESCE(pet.primary_image_url, u.profile_photo_url),
		       i.body, i.created_at, i.updated_at
		FROM inserted i
		JOIN users u ON u.id = i.user_id
		LEFT JOIN pets pet ON pet.id = i.actor_pet_id AND pet.deleted_at IS NULL`,
		postID, userID, request.Body, actorPetID).Scan(
		&item.ID, &item.PostID, &item.UserID, &item.ActorPetID, &item.AuthorName, &item.AuthorPhotoURL,
		&item.Body, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "comment_create_failed", "The comment could not be created.")
	}
	return httpx.Created(c, item)
}

func (h *Handler) listStories(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	limit, err := pageSize(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}
	var cursorTime any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeTimeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorTime, cursorID = cursor.CreatedAt, cursor.ID
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT s.id, s.pet_id, p.name, p.primary_image_url, u.name,
		       s.media_url, s.media_type, s.text_overlay, s.expires_at, s.created_at
		FROM stories s
		JOIN pets p ON p.id = s.pet_id
		JOIN users u ON u.id = s.author_user_id
		WHERE s.deleted_at IS NULL AND s.expires_at > now()
		  AND p.deleted_at IS NULL AND p.status = 'active'
		  AND (s.author_user_id = $1 OR EXISTS (
		    SELECT 1 FROM follows f WHERE f.user_id = $1 AND f.pet_id = s.pet_id
		  ))
		  -- Blocking is enforced both directions — see listPostsByKind's
		  -- identical clause and reasoning.
		  AND NOT EXISTS (
		    SELECT 1 FROM blocks bl
		    WHERE (bl.blocker_user_id = $1 AND bl.blocked_user_id = s.author_user_id)
		       OR (bl.blocker_user_id = s.author_user_id AND bl.blocked_user_id = $1)
		  )
		  AND ($2::timestamptz IS NULL OR (s.created_at, s.id) < ($2::timestamptz, $3::uuid))
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT $4`, userID, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
	}
	defer rows.Close()
	items := make([]Story, 0, limit)
	for rows.Next() {
		var item Story
		var overlay []byte
		if err := rows.Scan(&item.ID, &item.PetID, &item.PetName, &item.PetImageURL, &item.AuthorName, &item.MediaURL, &item.MediaType, &overlay, &item.ExpiresAt, &item.CreatedAt); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
		}
		item.TextOverlay = map[string]any{}
		if len(overlay) > 0 && json.Unmarshal(overlay, &item.TextOverlay) != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeTimeCursor(last.CreatedAt, last.ID)
	}
	return httpx.OK(c, storyPage{Items: items, NextCursor: nextCursor})
}

// listPetStories serves one pet's own active stories — the third public
// pet-profile media tab, alongside listPetPostsByKind's posts/reels.
func (h *Handler) listPetStories(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	petID, err := httpx.UUIDParam(c, "petId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "pet id must be a valid UUID.")
	}
	limit, err := pageSize(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}

	access, err := visibility.ForPet(c.UserContext(), h.db, petID, userID)
	if errors.Is(err, visibility.ErrNotFound) {
		return httpx.Problem(c, fiber.StatusNotFound, "pet_not_found", "The pet was not found.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
	}
	if access.Restricted {
		return httpx.OK(c, storyPage{Items: []Story{}})
	}

	var cursorTime any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeTimeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorTime, cursorID = cursor.CreatedAt, cursor.ID
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT s.id, s.pet_id, p.name, p.primary_image_url, u.name,
		       s.media_url, s.media_type, s.text_overlay, s.expires_at, s.created_at
		FROM stories s
		JOIN pets p ON p.id = s.pet_id
		JOIN users u ON u.id = s.author_user_id
		WHERE s.pet_id = $2 AND s.deleted_at IS NULL AND s.expires_at > now()
		  AND (s.author_user_id = $1 OR EXISTS (
		    SELECT 1 FROM follows f WHERE f.user_id = $1 AND f.pet_id = s.pet_id
		  ))
		  AND ($3::timestamptz IS NULL OR (s.created_at, s.id) < ($3::timestamptz, $4::uuid))
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT $5`, userID, petID, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
	}
	defer rows.Close()
	items := make([]Story, 0, limit)
	for rows.Next() {
		var item Story
		var overlay []byte
		if err := rows.Scan(&item.ID, &item.PetID, &item.PetName, &item.PetImageURL, &item.AuthorName, &item.MediaURL, &item.MediaType, &overlay, &item.ExpiresAt, &item.CreatedAt); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
		}
		item.TextOverlay = map[string]any{}
		if len(overlay) > 0 && json.Unmarshal(overlay, &item.TextOverlay) != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "story_query_failed", "Stories could not be loaded.")
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeTimeCursor(last.CreatedAt, last.ID)
	}
	return httpx.OK(c, storyPage{Items: items, NextCursor: nextCursor})
}

func (h *Handler) followPet(c *fiber.Ctx) error {
	return h.setFollow(c, true)
}

func (h *Handler) unfollowPet(c *fiber.Ctx) error {
	return h.setFollow(c, false)
}

// setFollow toggles the caller's follow of a pet — brief Milestone 2's
// "Follow/unfollow". Both directions are idempotent: following an
// already-followed pet, or unfollowing one never followed, both succeed
// as a no-op rather than erroring. Existence/access uses visibility.ForPet,
// the same block-aware check the public profile and media tabs use, so a
// blocked pet 404s here exactly as it does for GET /pets/:petId instead of
// leaking block state through a differently-shaped follow error. The
// insert and its notification share one transaction: without that, a
// notification-insert failure after a committed follow-insert would be
// unrecoverable — ON CONFLICT DO NOTHING makes a retried follow a silent
// no-op, so the notification would never get a second chance to send.
func (h *Handler) setFollow(c *fiber.Ctx, enabled bool) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	petID, err := httpx.UUIDParam(c, "petId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "pet id must be a valid UUID.")
	}

	access, err := visibility.ForPet(c.UserContext(), h.db, petID, userID)
	if errors.Is(err, visibility.ErrNotFound) {
		return httpx.Problem(c, fiber.StatusNotFound, "pet_not_found", "The pet was not found.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "follow_update_failed", "The follow could not be updated.")
	}
	if access.IsOwner {
		return httpx.Problem(c, fiber.StatusBadRequest, "cannot_follow_own_pet", "You cannot follow your own pet.")
	}

	if !enabled {
		if _, err := h.db.Exec(c.UserContext(), `DELETE FROM follows WHERE user_id = $1 AND pet_id = $2`, userID, petID); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "follow_update_failed", "The follow could not be updated.")
		}
		return httpx.OK(c, fiber.Map{"following": false})
	}

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "follow_update_failed", "The follow could not be updated.")
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	tag, err := tx.Exec(c.UserContext(), `INSERT INTO follows (user_id, pet_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, petID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "follow_update_failed", "The follow could not be updated.")
	}
	if tag.RowsAffected() > 0 {
		if _, err := tx.Exec(c.UserContext(), `
			INSERT INTO notifications (user_id, notification_type, payload)
			VALUES ($1, 'follow', jsonb_build_object('pet_id', $2::text, 'follower_user_id', $3::text))`,
			access.OwnerID, petID, userID); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "follow_update_failed", "The follow could not be updated.")
		}
	}
	if err := tx.Commit(c.UserContext()); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "follow_update_failed", "The follow could not be updated.")
	}
	return httpx.OK(c, fiber.Map{"following": true})
}

func (h *Handler) createStory(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	var request createStoryRequest
	if err := c.BodyParser(&request); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body must be valid JSON.")
	}
	petID, err := uuid.Parse(strings.TrimSpace(request.PetID))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "pet_id must be a valid UUID.")
	}
	request.MediaURL = strings.TrimSpace(request.MediaURL)
	request.MediaType = strings.ToLower(strings.TrimSpace(request.MediaType))
	if request.MediaType == "" {
		request.MediaType = "image"
	}
	if !validMediaURL(request.MediaURL) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_media_url", "media_url must be an HTTP(S) URL or an application-relative media path.")
	}
	if request.MediaType != "image" && request.MediaType != "video" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_media_type", "media_type must be image or video.")
	}
	if request.TextOverlay == nil {
		request.TextOverlay = map[string]any{}
	}
	overlay, err := json.Marshal(request.TextOverlay)
	if err != nil || len(overlay) > 16*1024 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_text_overlay", "text_overlay is invalid or too large.")
	}

	var storyID uuid.UUID
	err = h.db.QueryRow(c.UserContext(), `
		INSERT INTO stories (pet_id, author_user_id, media_url, media_type, text_overlay)
		SELECT p.id, $1, $3, $4, $5::jsonb
		FROM pets p
		WHERE p.id = $2 AND p.owner_id = $1 AND p.deleted_at IS NULL AND p.status <> 'deleted'
		RETURNING id`, userID, petID, request.MediaURL, request.MediaType, overlay).Scan(&storyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusForbidden, "pet_not_owned", "The selected pet does not belong to the authenticated user.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "story_create_failed", "The story could not be created.")
	}

	var item Story
	var storedOverlay []byte
	err = h.db.QueryRow(c.UserContext(), `
		SELECT s.id, s.pet_id, p.name, p.primary_image_url, u.name,
		       s.media_url, s.media_type, s.text_overlay, s.expires_at, s.created_at
		FROM stories s JOIN pets p ON p.id = s.pet_id JOIN users u ON u.id = s.author_user_id
		WHERE s.id = $1`, storyID).Scan(&item.ID, &item.PetID, &item.PetName, &item.PetImageURL, &item.AuthorName, &item.MediaURL, &item.MediaType, &storedOverlay, &item.ExpiresAt, &item.CreatedAt)
	if err != nil || json.Unmarshal(storedOverlay, &item.TextOverlay) != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "story_create_failed", "The story was created but could not be loaded.")
	}
	return httpx.Created(c, item)
}

func validMediaURL(value string) bool {
	if value == "" || len(value) > 4096 {
		return false
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}
