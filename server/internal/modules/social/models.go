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
}

type Comment struct {
	ID             uuid.UUID `json:"id"`
	PostID         uuid.UUID `json:"post_id"`
	UserID         uuid.UUID `json:"user_id"`
	AuthorName     string    `json:"author_name"`
	AuthorPhotoURL string    `json:"author_photo_url"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
}

type createStoryRequest struct {
	PetID       string         `json:"pet_id"`
	MediaURL    string         `json:"media_url"`
	MediaType   string         `json:"media_type"`
	TextOverlay map[string]any `json:"text_overlay"`
}

type createCommentRequest struct {
	Body string `json:"body"`
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
