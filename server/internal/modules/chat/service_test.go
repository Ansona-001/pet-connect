package chat

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMessageCursorRoundTrip(t *testing.T) {
	t.Parallel()
	wantTime := time.Date(2026, time.August, 6, 12, 34, 56, 789123456, time.UTC)
	wantID := uuid.MustParse("10000000-0000-4000-8000-000000000001")

	encoded := encodeCursor(wantTime, wantID)
	gotTime, gotID, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("decodeCursor() error = %v", err)
	}
	if !gotTime.Equal(wantTime) || gotID != wantID {
		t.Fatalf("decodeCursor() = (%s, %s), want (%s, %s)", gotTime, gotID, wantTime, wantID)
	}
	if _, _, err := decodeCursor("not-a-cursor"); err == nil {
		t.Fatal("decodeCursor() accepted an invalid cursor")
	}
}

func TestValidateMessage(t *testing.T) {
	t.Parallel()
	requestID := uuid.New()
	tests := []struct {
		name    string
		request SendMessageRequest
		valid   bool
	}{
		{name: "text", request: SendMessageRequest{ClientMessageID: requestID, MessageType: "text", Body: "hello"}, valid: true},
		{name: "image", request: SendMessageRequest{ClientMessageID: requestID, MessageType: "image", MediaURL: "/media/image.jpg"}, valid: true},
		{name: "location", request: SendMessageRequest{ClientMessageID: requestID, MessageType: "location", Body: `{"lat":25.2,"lng":55.3}`}, valid: true},
		{name: "missing id", request: SendMessageRequest{MessageType: "text", Body: "hello"}},
		{name: "empty text", request: SendMessageRequest{ClientMessageID: requestID, MessageType: "text"}},
		{name: "text with media", request: SendMessageRequest{ClientMessageID: requestID, MessageType: "text", Body: "hello", MediaURL: "/media/image.jpg"}},
		{name: "media without url", request: SendMessageRequest{ClientMessageID: requestID, MessageType: "video"}},
		{name: "unsupported", request: SendMessageRequest{ClientMessageID: requestID, MessageType: "file", MediaURL: "/media/a.pdf"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateMessage(test.request)
			if test.valid && err != nil {
				t.Fatalf("validateMessage() error = %v", err)
			}
			if !test.valid && !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("validateMessage() error = %v, want ErrInvalidMessage", err)
			}
		})
	}
}

func TestSameOptionalUUID(t *testing.T) {
	t.Parallel()
	first := uuid.New()
	same := first
	other := uuid.New()
	if !sameOptionalUUID(nil, nil) || !sameOptionalUUID(&first, &same) {
		t.Fatal("sameOptionalUUID rejected equal values")
	}
	if sameOptionalUUID(&first, nil) || sameOptionalUUID(&first, &other) {
		t.Fatal("sameOptionalUUID accepted different values")
	}
}
