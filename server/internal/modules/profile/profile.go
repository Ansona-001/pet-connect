// Package profile serves other users' (and the caller's own) public
// owner profiles — brief Milestone 2's first day.
package profile

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
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
		          WHERE po.author_user_id = u.id AND po.kind = 'post' AND po.deleted_at IS NULL),
		       (SELECT count(DISTINCT f.user_id) FROM follows f
		          JOIN pets fp ON fp.id = f.pet_id
		          WHERE fp.owner_id = u.id AND fp.deleted_at IS NULL AND fp.status = 'active'),
		       (SELECT count(*) FROM follows f WHERE f.user_id = u.id)
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
