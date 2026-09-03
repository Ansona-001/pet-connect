package realtime

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTicketTTL = 30 * time.Second

type TicketStore struct {
	redis  redis.UniversalClient
	prefix string
	ttl    time.Duration
	now    func() time.Time
}

func NewTicketStore(client redis.UniversalClient, prefix string, ttl time.Duration) *TicketStore {
	if strings.TrimSpace(prefix) == "" {
		prefix = "petconnect:realtime:ticket:"
	}
	if ttl <= 0 || ttl > time.Minute {
		ttl = defaultTicketTTL
	}
	return &TicketStore{redis: client, prefix: prefix, ttl: ttl, now: time.Now}
}

func (s *TicketStore) Issue(ctx context.Context, identity Identity) (Ticket, error) {
	if s.redis == nil {
		return Ticket{}, ErrTicketStore
	}
	if identity.UserID.String() == "00000000-0000-0000-0000-000000000000" || identity.SessionID.String() == "00000000-0000-0000-0000-000000000000" {
		return Ticket{}, ErrInvalidTicket
	}
	now := s.now().UTC()
	ttl := s.ttl
	if remaining := identity.AccessExpiresAt.Sub(now); remaining < ttl {
		ttl = remaining
	}
	if ttl <= 0 {
		return Ticket{}, ErrInvalidTicket
	}
	payload, err := json.Marshal(identity)
	if err != nil {
		return Ticket{}, fmt.Errorf("encode realtime identity: %w", err)
	}

	for attempt := 0; attempt < 3; attempt++ {
		value, err := randomTicket()
		if err != nil {
			return Ticket{}, err
		}
		stored, err := s.redis.SetNX(ctx, s.prefix+value, payload, ttl).Result()
		if err != nil {
			return Ticket{}, fmt.Errorf("%w: %v", ErrTicketStore, err)
		}
		if stored {
			return Ticket{Value: value, ExpiresAt: now.Add(ttl)}, nil
		}
	}
	return Ticket{}, fmt.Errorf("%w: could not allocate a unique ticket", ErrTicketStore)
}

func (s *TicketStore) Consume(ctx context.Context, value string) (Identity, error) {
	if s.redis == nil || !validTicket(value) {
		return Identity{}, ErrInvalidTicket
	}
	payload, err := s.redis.GetDel(ctx, s.prefix+value).Bytes()
	if errors.Is(err, redis.Nil) {
		return Identity{}, ErrInvalidTicket
	}
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrTicketStore, err)
	}
	var identity Identity
	if err := json.Unmarshal(payload, &identity); err != nil {
		return Identity{}, ErrInvalidTicket
	}
	if identity.UserID.String() == "00000000-0000-0000-0000-000000000000" ||
		identity.SessionID.String() == "00000000-0000-0000-0000-000000000000" ||
		!identity.AccessExpiresAt.After(s.now().UTC()) {
		return Identity{}, ErrInvalidTicket
	}
	return identity, nil
}

func randomTicket() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate realtime ticket: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func validTicket(value string) bool {
	if len(value) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32
}
