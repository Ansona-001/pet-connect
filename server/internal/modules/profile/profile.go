// Package profile serves other users' (and the caller's own) public
// owner profiles — brief Milestone 2's first day.
package profile

import (
	"errors"
	"strings"

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

func New(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// RegisterRoutes mounts profile routes on an authenticated router rooted at /v1.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	New(db).RegisterRoutes(router)
}

// RegisterRoutes mounts routes. The supplied router must already use authentication middleware.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/users/:userId", h.getPublicProfile)
	router.Get("/users/:userId/followers", h.listFollowers)
	router.Get("/users/:userId/following", h.listFollowing)
}

// publicProfileResponse is deliberately its own type, not account.go's
// `user` struct: that struct carries exact lat/lng, which ADR 0002
// explicitly calls out as a value this endpoint must never reuse
// as-is — a public profile gets city/area text at most, never
// coordinates. Bio/City/PostCount/FollowerCount/FollowingCount are
// pointers so they can be omitted entirely (nil -> absent from the JSON)
// for a private account's non-owner viewers, rather than sent as
// misleadingly-present zero values.
type publicProfileResponse struct {
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

// getPublicProfile enforces two deliberately minimal privacy rules:
//
//  1. A block in either direction hides the profile entirely (404, not
//     403 or a "this account blocked you" message — the same
//     enumeration-safety reasoning as auth's generic errors: a block's
//     existence must not be detectable from response shape).
//  2. A private account's non-owner viewers get name/avatar/pet-count
//     only; bio, city, and every social count are omitted.
//
// This is this endpoint's own reasonable default, not the full sweep the
// brief's later "Visibility enforcement" day performs across every fetch
// path (feed/discovery/search too) — it exists so this endpoint doesn't
// ship with zero privacy awareness in the meantime.
func (h *Handler) getPublicProfile(c *fiber.Ctx) error {
	viewerID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	targetID, err := httpx.UUIDParam(c, "userId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_user_id", "user id must be a valid UUID.")
	}

	var (
		name, bio, city, photoURL     string
		isPrivate                     bool
		petCount, postCount           int64
		followerCount, followingCount int64
	)
	err = h.db.QueryRow(c.UserContext(), `
		SELECT u.name, u.bio, u.city, u.profile_photo_url, u.is_private,
		       (SELECT count(*) FROM pets p
		          WHERE p.owner_id = u.id AND p.deleted_at IS NULL AND p.status = 'active'),
		       (SELECT count(*) FROM posts po
		          JOIN pets pt ON pt.id = po.pet_id AND pt.deleted_at IS NULL AND pt.status = 'active'
		          WHERE po.author_user_id = u.id AND po.kind = 'post' AND po.deleted_at IS NULL),
		       (SELECT count(DISTINCT f.user_id) FROM follows f
		          JOIN pets fp ON fp.id = f.pet_id
		          WHERE fp.owner_id = u.id AND fp.deleted_at IS NULL AND fp.status = 'active'),
		       (SELECT count(*) FROM follows f
		          JOIN pets p ON p.id = f.pet_id
		          WHERE f.user_id = u.id AND p.deleted_at IS NULL AND p.status = 'active')
		FROM users u
		WHERE u.id = $1 AND u.deleted_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM blocks b
		    WHERE (b.blocker_user_id = $2 AND b.blocked_user_id = u.id)
		       OR (b.blocker_user_id = u.id AND b.blocked_user_id = $2)
		  )`, targetID, viewerID).Scan(
		&name, &bio, &city, &photoURL, &isPrivate,
		&petCount, &postCount, &followerCount, &followingCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusNotFound, "user_not_found", "That user could not be found.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}

	response := publicProfileResponse{
		ID:              targetID,
		Name:            name,
		ProfilePhotoURL: photoURL,
		IsPrivate:       isPrivate,
		IsSelf:          targetID == viewerID,
		PetCount:        petCount,
	}
	if response.IsSelf || !isPrivate {
		response.Bio = &bio
		response.City = &city
		response.PostCount = &postCount
		response.FollowerCount = &followerCount
		response.FollowingCount = &followingCount
	}
	return httpx.OK(c, response)
}

// listFollowers serves the followers-list counterpart to
// getPublicProfile's follower_count — brief Milestone 2: "Followers/
// following lists". A "follower" here is a user following *any* of
// targetID's active pets, deduplicated by follower and ordered by their
// most recent follow of this owner — the exact same aggregate
// getPublicProfile's follower_count computes, just paginated instead of
// counted, so the two must never drift apart (both restated below rather
// than shared, since the count is a single scalar with no page shape to
// share).
func (h *Handler) listFollowers(c *fiber.Ctx) error {
	viewerID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	targetID, err := httpx.UUIDParam(c, "userId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_user_id", "user id must be a valid UUID.")
	}
	limit, err := pageSize(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}

	access, err := visibility.ForUser(c.UserContext(), h.db, targetID, viewerID)
	if errors.Is(err, visibility.ErrNotFound) {
		return httpx.Problem(c, fiber.StatusNotFound, "user_not_found", "That user could not be found.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
	if access.Restricted {
		return httpx.OK(c, followersPage{Items: []followerUser{}, Restricted: true})
	}

	var totalCount int64
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT count(DISTINCT f.user_id) FROM follows f
		JOIN pets p ON p.id = f.pet_id
		WHERE p.owner_id = $1 AND p.deleted_at IS NULL AND p.status = 'active'`, targetID).Scan(&totalCount); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}

	var cursorAt any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorAt, cursorID = cursor.At, cursor.ID
	}

	rows, err := h.db.Query(c.UserContext(), `
		WITH owner_followers AS (
		  SELECT f.user_id, max(f.created_at) AS last_followed_at
		  FROM follows f
		  JOIN pets p ON p.id = f.pet_id
		  WHERE p.owner_id = $1 AND p.deleted_at IS NULL AND p.status = 'active'
		  GROUP BY f.user_id
		)
		SELECT of.user_id, u.name, u.profile_photo_url, of.last_followed_at
		FROM owner_followers of
		JOIN users u ON u.id = of.user_id AND u.deleted_at IS NULL
		WHERE ($2::timestamptz IS NULL OR (of.last_followed_at, of.user_id) < ($2::timestamptz, $3::uuid))
		ORDER BY of.last_followed_at DESC, of.user_id DESC
		LIMIT $4`, targetID, cursorAt, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
	defer rows.Close()

	items := make([]followerUser, 0, limit)
	for rows.Next() {
		var item followerUser
		if err := rows.Scan(&item.ID, &item.Name, &item.ProfilePhotoURL, &item.FollowedAt); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeCursor(last.FollowedAt, last.ID)
	}
	return httpx.OK(c, followersPage{Items: items, TotalCount: totalCount, NextCursor: nextCursor})
}

// listFollowing serves the following-list counterpart to
// getPublicProfile's following_count — the pets targetID follows.
func (h *Handler) listFollowing(c *fiber.Ctx) error {
	viewerID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	targetID, err := httpx.UUIDParam(c, "userId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_user_id", "user id must be a valid UUID.")
	}
	limit, err := pageSize(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}

	access, err := visibility.ForUser(c.UserContext(), h.db, targetID, viewerID)
	if errors.Is(err, visibility.ErrNotFound) {
		return httpx.Problem(c, fiber.StatusNotFound, "user_not_found", "That user could not be found.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
	if access.Restricted {
		return httpx.OK(c, followingPage{Items: []followedPet{}, Restricted: true})
	}

	var totalCount int64
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT count(*) FROM follows f
		JOIN pets p ON p.id = f.pet_id
		WHERE f.user_id = $1 AND p.deleted_at IS NULL AND p.status = 'active'`, targetID).Scan(&totalCount); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}

	var cursorAt any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorAt, cursorID = cursor.At, cursor.ID
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT p.id, p.name, p.pet_type, p.breed, p.primary_image_url,
		       p.owner_id, owner.name, f.created_at
		FROM follows f
		JOIN pets p ON p.id = f.pet_id
		JOIN users owner ON owner.id = p.owner_id AND owner.deleted_at IS NULL
		WHERE f.user_id = $1 AND p.deleted_at IS NULL AND p.status = 'active'
		  AND ($2::timestamptz IS NULL OR (f.created_at, f.pet_id) < ($2::timestamptz, $3::uuid))
		ORDER BY f.created_at DESC, f.pet_id DESC
		LIMIT $4`, targetID, cursorAt, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
	defer rows.Close()

	items := make([]followedPet, 0, limit)
	for rows.Next() {
		var item followedPet
		if err := rows.Scan(
			&item.ID, &item.Name, &item.PetType, &item.Breed, &item.PrimaryImageURL,
			&item.OwnerID, &item.OwnerName, &item.FollowedAt,
		); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeCursor(last.FollowedAt, last.ID)
	}
	return httpx.OK(c, followingPage{Items: items, TotalCount: totalCount, NextCursor: nextCursor})
}
