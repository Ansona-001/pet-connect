package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOriginAllowed(t *testing.T) {
	t.Parallel()
	allowed := []string{"https://petconnect.app", "http://localhost:*", "http://127.0.0.1:*"}
	tests := []struct {
		origin string
		want   bool
	}{
		{origin: "", want: true},
		{origin: "https://petconnect.app", want: true},
		{origin: "https://petconnect.app/", want: true},
		{origin: "http://localhost:5173", want: true},
		{origin: "http://127.0.0.1:3000", want: true},
		{origin: "https://localhost:5173", want: false},
		{origin: "https://evil.example", want: false},
		{origin: "not a URL", want: false},
	}
	for _, test := range tests {
		if got := originAllowed(test.origin, allowed); got != test.want {
			t.Fatalf("originAllowed(%q) = %v, want %v", test.origin, got, test.want)
		}
	}
}

func TestRandomTicketShape(t *testing.T) {
	t.Parallel()
	first, err := randomTicket()
	if err != nil {
		t.Fatalf("randomTicket() error = %v", err)
	}
	second, err := randomTicket()
	if err != nil {
		t.Fatalf("randomTicket() second error = %v", err)
	}
	if !validTicket(first) || !validTicket(second) {
		t.Fatal("randomTicket() returned an invalid ticket shape")
	}
	if first == second {
		t.Fatal("randomTicket() returned a duplicate value")
	}
	if validTicket("short") {
		t.Fatal("validTicket() accepted malformed input")
	}
}

func TestHubLocalDelivery(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil, "")
	userID := uuid.New()
	client := newConnection(Identity{
		UserID: userID, SessionID: uuid.New(), AccessExpiresAt: time.Now().Add(time.Minute),
	})
	if err := hub.register(client); err != nil {
		t.Fatalf("register() error = %v", err)
	}
	defer hub.unregister(client)

	if err := hub.PublishToUsers([]uuid.UUID{userID, userID}, "message.created", map[string]string{"body": "hello"}); err != nil {
		t.Fatalf("PublishToUsers() error = %v", err)
	}
	select {
	case encoded := <-client.send:
		var event Event
		if err := json.Unmarshal(encoded, &event); err != nil {
			t.Fatalf("decode delivered event: %v", err)
		}
		if event.Type != "message.created" || event.ID == uuid.Nil {
			t.Fatalf("delivered event = %#v", event)
		}
		select {
		case duplicate := <-client.send:
			t.Fatalf("duplicate user id caused duplicate event: %s", duplicate)
		default:
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for local event")
	}
}
