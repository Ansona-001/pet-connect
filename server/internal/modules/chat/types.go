package chat

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrChatNotFound        = errors.New("chat not found")
	ErrMessageNotFound     = errors.New("message not found")
	ErrChatInactive        = errors.New("chat is not active")
	ErrInvalidMessage      = errors.New("invalid message")
	ErrSenderPetForbidden  = errors.New("sender pet is not part of this match")
	ErrIdempotencyConflict = errors.New("the client message id was reused for different input")
)

type Message struct {
	ID              uuid.UUID  `json:"id"`
	ChatID          uuid.UUID  `json:"chat_id"`
	SenderUserID    uuid.UUID  `json:"sender_user_id"`
	SenderPetID     *uuid.UUID `json:"sender_pet_id,omitempty"`
	ClientMessageID uuid.UUID  `json:"client_message_id"`
	MessageType     string     `json:"message_type"`
	Body            string     `json:"body"`
	MediaURL        string     `json:"media_url,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	EditedAt        *time.Time `json:"edited_at,omitempty"`
}

type Chat struct {
	ID             uuid.UUID  `json:"id"`
	MatchID        uuid.UUID  `json:"match_id"`
	MatchedAt      time.Time  `json:"matched_at"`
	OwnPetID       uuid.UUID  `json:"own_pet_id"`
	OwnPetName     string     `json:"own_pet_name"`
	OtherPetID     uuid.UUID  `json:"other_pet_id"`
	OtherPetName   string     `json:"other_pet_name"`
	OtherPetType   string     `json:"other_pet_type"`
	OtherPetBreed  string     `json:"other_pet_breed"`
	OtherPetImage  string     `json:"other_pet_image_url"`
	OtherOwnerID   uuid.UUID  `json:"other_owner_id"`
	OtherOwnerName string     `json:"other_owner_name"`
	LastMessage    *Message   `json:"last_message,omitempty"`
	UnreadCount    int64      `json:"unread_count"`
	LastReadAt     *time.Time `json:"last_read_at,omitempty"`
}

type MessagePage struct {
	Items      []Message `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

type SendMessageRequest struct {
	SenderPetID     *uuid.UUID
	ClientMessageID uuid.UUID
	MessageType     string
	Body            string
	MediaURL        string
}

type SendMessageResult struct {
	Message Message `json:"message"`
	Created bool    `json:"-"`
}

type ReadCursor struct {
	ChatID    uuid.UUID `json:"chat_id"`
	MessageID uuid.UUID `json:"message_id"`
	ReadAt    time.Time `json:"read_at"`
}

type EventPublisher interface {
	PublishToUsers(userIDs []uuid.UUID, eventType string, payload any) error
}
