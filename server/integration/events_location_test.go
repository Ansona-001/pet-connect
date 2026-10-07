// This file closes the gap ADR 0002 flagged and docs/requirements-
// traceability.md's "Events" row carried forward unfixed: events/
// handler.go returned an event's exact latitude/longitude to every
// caller, including a stranger just browsing — a higher-risk leak than
// the already-fixed /me and /pets/:petId cases, since an event implies a
// real meeting point. The fix (handler.go's canSeeExactLocation) withholds
// exact coordinates from anyone but the event's creator or a "going"
// RSVP; location_name stays public for everyone. These tests drive that
// boundary through the real HTTP routes rather than asserting on
// handler.go's unexported helper directly.
package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"petconnect/server/internal/modules/events"
)

func (env *testEnv) createEvent(t *testing.T, creator testUser, lat, lng float64) events.Event {
	t.Helper()
	resp := env.do(t, "POST", "/v1/events", creator.Token, map[string]any{
		"title":         "Dog park meetup",
		"description":   "Bring tennis balls.",
		"location_name": "Al Barsha Pond Park",
		"latitude":      lat,
		"longitude":     lng,
		"starts_at":     time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create event status = %d, want 201", resp.StatusCode)
	}
	return decodeEventData(t, resp)
}

func (env *testEnv) findEvent(t *testing.T, viewer testUser, eventID string) events.Event {
	t.Helper()
	resp := env.do(t, "GET", "/v1/events?limit=50", viewer.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list events status = %d, want 200", resp.StatusCode)
	}
	for _, item := range decodeEventPage(t, resp).Items {
		if item.ID.String() == eventID {
			return item
		}
	}
	t.Fatalf("event %s not found in list response", eventID)
	return events.Event{}
}

// TestEventLocationWithheldFromNonAttendees regression-tests the exact
// boundary ADR 0002 specifies: creator always sees exact coordinates; a
// stranger and an "interested" (not "going") RSVP both get location_name
// only; a "going" RSVP unlocks exact coordinates, matching "confirmed
// attendance" in the ADR's own wording.
func TestEventLocationWithheldFromNonAttendees(t *testing.T) {
	env := setupTestEnv(t)
	creator := env.createUser(t, "event-creator")
	stranger := env.createUser(t, "event-stranger")
	attendee := env.createUser(t, "event-attendee")

	created := env.createEvent(t, creator, 25.2048, 55.2708)
	if created.Latitude == nil || created.Longitude == nil {
		t.Fatalf("creator's own create response omitted coordinates, want exact lat/lng")
	}
	if created.LocationName != "Al Barsha Pond Park" {
		t.Fatalf("location_name = %q, want the human-readable name regardless of viewer", created.LocationName)
	}

	byCreator := env.findEvent(t, creator, created.ID.String())
	if byCreator.Latitude == nil || byCreator.Longitude == nil {
		t.Fatalf("creator listing their own event got no coordinates, want exact lat/lng")
	}

	byStranger := env.findEvent(t, stranger, created.ID.String())
	if byStranger.Latitude != nil || byStranger.Longitude != nil {
		t.Fatalf("stranger saw exact coordinates (%v, %v), want withheld", byStranger.Latitude, byStranger.Longitude)
	}
	if byStranger.LocationName != "Al Barsha Pond Park" {
		t.Fatalf("stranger's location_name = %q, want still public", byStranger.LocationName)
	}

	rsvpResp := env.do(t, "PUT", "/v1/events/"+created.ID.String()+"/rsvps/me", attendee.Token, map[string]any{"status": "interested"})
	if rsvpResp.StatusCode != http.StatusOK {
		t.Fatalf("rsvp interested status = %d, want 200", rsvpResp.StatusCode)
	}
	byInterested := env.findEvent(t, attendee, created.ID.String())
	if byInterested.Latitude != nil || byInterested.Longitude != nil {
		t.Fatalf("'interested' RSVP saw exact coordinates, want withheld until 'going'")
	}

	goingResp := env.do(t, "PUT", "/v1/events/"+created.ID.String()+"/rsvps/me", attendee.Token, map[string]any{"status": "going"})
	if goingResp.StatusCode != http.StatusOK {
		t.Fatalf("rsvp going status = %d, want 200", goingResp.StatusCode)
	}
	byGoing := env.findEvent(t, attendee, created.ID.String())
	if byGoing.Latitude == nil || byGoing.Longitude == nil {
		t.Fatalf("'going' RSVP did not unlock exact coordinates")
	}
	if *byGoing.Latitude != 25.2048 || *byGoing.Longitude != 55.2708 {
		t.Fatalf("exact coordinates = (%v, %v), want (25.2048, 55.2708)", *byGoing.Latitude, *byGoing.Longitude)
	}
}

type eventDataBody struct {
	Data events.Event `json:"data"`
}

type eventPageDataBody struct {
	Data struct {
		Items []events.Event `json:"items"`
	} `json:"data"`
}

func decodeEventData(t *testing.T, resp *http.Response) events.Event {
	t.Helper()
	var payload eventDataBody
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode event response: %v", err)
	}
	return payload.Data
}

func decodeEventPage(t *testing.T, resp *http.Response) struct{ Items []events.Event } {
	t.Helper()
	var payload eventPageDataBody
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode event page response: %v", err)
	}
	return struct{ Items []events.Event }{Items: payload.Data.Items}
}
