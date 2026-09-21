package profile

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maximumPageSize = 50
)

// followerUser is one entry in GET /users/:userId/followers — a user
// following any of that owner's pets.
type followerUser struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	ProfilePhotoURL string    `json:"profile_photo_url"`
	FollowedAt      time.Time `json:"followed_at"`
}

// followedPet is one entry in GET /users/:userId/following — a pet that
// user follows.
type followedPet struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	PetType         string    `json:"pet_type"`
	Breed           string    `json:"breed"`
	PrimaryImageURL string    `json:"primary_image_url"`
	OwnerID         uuid.UUID `json:"owner_id"`
	OwnerName       string    `json:"owner_name"`
	FollowedAt      time.Time `json:"followed_at"`
}

// followersPage/followingPage both carry TotalCount independent of the
// current page (brief: "...endpoints with counts") and Restricted, set
// only when the target's private account hides the list from this
// viewer — Items/NextCursor are empty/absent in that case rather than an
// error, the same reduced-not-rejected pattern applyVisibility and the
// pet media tabs already use.
type followersPage struct {
	Items      []followerUser `json:"items"`
	TotalCount int64          `json:"total_count"`
	NextCursor string         `json:"next_cursor,omitempty"`
	Restricted bool           `json:"restricted,omitempty"`
}

type followingPage struct {
	Items      []followedPet `json:"items"`
	TotalCount int64         `json:"total_count"`
	NextCursor string        `json:"next_cursor,omitempty"`
	Restricted bool          `json:"restricted,omitempty"`
}

type timeCursor struct {
	At time.Time
	ID uuid.UUID
}

func encodeCursor(at time.Time, id uuid.UUID) string {
	value := at.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCursor(value string) (timeCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return timeCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return timeCursor{}, fmt.Errorf("cursor has an invalid shape")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return timeCursor{}, fmt.Errorf("decode cursor timestamp: %w", err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return timeCursor{}, fmt.Errorf("decode cursor id: %w", err)
	}
	return timeCursor{At: at, ID: id}, nil
}

func pageSize(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultPageSize, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximumPageSize {
		return 0, fmt.Errorf("limit must be between 1 and %d", maximumPageSize)
	}
	return value, nil
}
