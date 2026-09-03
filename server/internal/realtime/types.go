package realtime

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidTicket   = errors.New("realtime ticket is invalid or expired")
	ErrTicketStore     = errors.New("realtime ticket store is unavailable")
	ErrOriginForbidden = errors.New("websocket origin is not allowed")
)

type Identity struct {
	UserID          uuid.UUID `json:"user_id"`
	SessionID       uuid.UUID `json:"session_id"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
}

type Ticket struct {
	Value     string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Event struct {
	ID         uuid.UUID       `json:"id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

type brokerEnvelope struct {
	OriginInstanceID uuid.UUID   `json:"origin_instance_id"`
	UserIDs          []uuid.UUID `json:"user_ids"`
	Event            Event       `json:"event"`
}
