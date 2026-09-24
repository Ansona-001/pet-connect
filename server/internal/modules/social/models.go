package social

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
	// maxCarouselItems caps how many media items a single post can
	// carry — generous enough for any real use, small enough to bound
	// the ordered-insert transaction and the feed payload size.
	maxCarouselItems = 10
)

type Post struct {
	ID           uuid.UUID `json:"id"`
	PetID        uuid.UUID `json:"pet_id"`
	PetName      string    `json:"pet_name"`
	PetImageURL  string    `json:"pet_image_url"`
	AuthorUserID uuid.UUID `json:"author_user_id"`
	AuthorName   string    `json:"author_name"`
	Kind         string    `json:"kind"`
	Caption      string    `json:"caption"`
	LocationName string    `json:"location_name"`
	MediaURL     string    `json:"media_url"`
	MediaType    string    `json:"media_type"`
	Visibility   string    `json:"visibility"`
	LikeCount    int64     `json:"like_count"`
	CommentCount int64     `json:"comment_count"`
	LikedByMe    bool      `json:"liked_by_me"`
	SavedByMe    bool      `json:"saved_by_me"`
	FollowedByMe bool      `json:"followed_by_me"`
	CreatedAt    time.Time `json:"created_at"`
	// Media is the ordered carousel (migrations/000007_durable_media_
	// schema.sql's post_media), populated only for posts created or
	// edited with media_ids. MediaURL/MediaType above always mirror the
	// carousel's first item when one exists, so a client that has never
	// heard of carousels still renders something reasonable — Media is
	// additive, never a replacement those two fields need updating for.
	Media []PostMedia `json:"media,omitempty"`
}

// PostMedia is one ordered item of a post's carousel — see Post.Media.
type PostMedia struct {
	ID        uuid.UUID `json:"id"`
	MediaURL  string    `json:"media_url"`
	MediaType string    `json:"media_type"`
	Width     *int      `json:"width,omitempty"`
	Height    *int      `json:"height,omitempty"`
}

// Comment follows ADR 0001's identity-attribution model: UserID is
// always the authorization/audit anchor (never spoofable, never absent),
// while ActorPetID is the optional *display* identity — when set,
// AuthorName/AuthorPhotoURL already resolve to that pet's name/photo
// server-side (see the COALESCE in handlers.go's queries) rather than
// making every client re-derive "which identity to show."
type Comment struct {
	ID             uuid.UUID  `json:"id"`
	PostID         uuid.UUID  `json:"post_id"`
	UserID         uuid.UUID  `json:"user_id"`
	ActorPetID     *uuid.UUID `json:"actor_pet_id,omitempty"`
	AuthorName     string     `json:"author_name"`
	AuthorPhotoURL string     `json:"author_photo_url"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Story struct {
	ID          uuid.UUID      `json:"id"`
	PetID       uuid.UUID      `json:"pet_id"`
	PetName     string         `json:"pet_name"`
	PetImageURL string         `json:"pet_image_url"`
	AuthorName  string         `json:"author_name"`
	MediaURL    string         `json:"media_url"`
	MediaType   string         `json:"media_type"`
	TextOverlay map[string]any `json:"text_overlay"`
	ExpiresAt   time.Time      `json:"expires_at"`
	CreatedAt   time.Time      `json:"created_at"`
}

type createPostRequest struct {
	PetID        string `json:"pet_id"`
	Caption      string `json:"caption"`
	LocationName string `json:"location_name"`
	MediaURL     string `json:"media_url"`
	MediaType    string `json:"media_type"`
	Visibility   string `json:"visibility"`
	// MediaIDs, when present, is an ordered carousel of previously
	// uploaded media/handler.go media rows (each must belong to the
	// caller) — brief Milestone 3's "Carousel post API". Omit it (or
	// leave it empty) to use the legacy single MediaURL/MediaType path
	// unchanged. Reels never accept it — a reel is always one video.
	MediaIDs []string `json:"media_ids"`
}

type patchPostRequest struct {
	Caption      *string   `json:"caption"`
	LocationName *string   `json:"location_name"`
	Visibility   *string   `json:"visibility"`
	MediaIDs     *[]string `json:"media_ids"`
}

type createStoryRequest struct {
	PetID       string         `json:"pet_id"`
	MediaURL    string         `json:"media_url"`
	MediaType   string         `json:"media_type"`
	TextOverlay map[string]any `json:"text_overlay"`
}

type createCommentRequest struct {
	Body       string `json:"body"`
	ActorPetID string `json:"actor_pet_id"`
}

type likeRequest struct {
	ActorPetID string `json:"actor_pet_id"`
}

type postPage struct {
	Items      []Post `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type commentPage struct {
	Items      []Comment `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

type storyPage struct {
	Items      []Story `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type timeCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func encodeTimeCursor(createdAt time.Time, id uuid.UUID) string {
	value := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeTimeCursor(value string) (timeCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return timeCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return timeCursor{}, fmt.Errorf("cursor has an invalid shape")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return timeCursor{}, fmt.Errorf("decode cursor timestamp: %w", err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return timeCursor{}, fmt.Errorf("decode cursor id: %w", err)
	}
	return timeCursor{CreatedAt: createdAt, ID: id}, nil
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
