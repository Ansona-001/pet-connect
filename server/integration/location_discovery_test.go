// This file tests the "Location visibility controls" build day: the new
// users.is_discoverable column (migrations/000006_location_discovery_
// participation.sql) and its enforcement in discovery/matching, plus
// PATCH /me's ability to set it.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func (env *testEnv) setDiscoverable(t *testing.T, user testUser, discoverable bool) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(),
		`UPDATE users SET is_discoverable = $1 WHERE id = $2`, discoverable, user.ID); err != nil {
		t.Fatalf("set is_discoverable: %v", err)
	}
}

// TestPatchMeUpdatesDiscoverability regression-tests
// internal/modules/account/account.go's patchMe: is_discoverable is
// independent of is_private, defaults to true, and round-trips through
// PATCH /me exactly like is_private does.
func TestPatchMeUpdatesDiscoverability(t *testing.T) {
	env := setupTestEnv(t)
	user := env.createUser(t, "discoverable-patch")

	getResp := env.do(t, http.MethodGet, "/v1/me", user.Token, nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", getResp.StatusCode)
	}
	initial := decodeMe(t, getResp)
	if !initial.IsDiscoverable {
		t.Fatalf("is_discoverable default = %v, want true", initial.IsDiscoverable)
	}

	patchResp := env.do(t, http.MethodPatch, "/v1/me", user.Token, map[string]any{
		"is_discoverable": false,
	})
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", patchResp.StatusCode)
	}
	patched := decodeMe(t, patchResp)
	if patched.IsDiscoverable {
		t.Fatalf("is_discoverable after patch = %v, want false", patched.IsDiscoverable)
	}
	if patched.IsPrivate {
		t.Fatalf("is_private changed to %v from an is_discoverable-only patch, want unchanged false", patched.IsPrivate)
	}
}

// TestDiscoveryExcludesOptedOutUsers regression-tests
// internal/modules/discovery/handler.go's `nearbyPets`: a user who has
// opted out of discovery is excluded even from a viewer who already
// follows their pet — unlike is_private, there's no follow exception,
// since opting out of discovery means opting out.
func TestDiscoveryExcludesOptedOutUsers(t *testing.T) {
	env := setupTestEnv(t)
	viewer := env.createUser(t, "disc-optout-viewer")
	optedOut := env.createUser(t, "disc-optout-target")
	env.setDiscoverable(t, optedOut, false)
	env.setPetLocation(t, optedOut.PetID, 25.2048, 55.2708)
	env.follow(t, viewer, optedOut.PetID)

	resp := env.do(t, http.MethodGet, "/v1/discovery/pets?lat=25.2048&lng=55.2708&radius_km=10", viewer.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	for _, item := range decodeItemsPage(t, resp) {
		if item.ID == optedOut.PetID {
			t.Fatalf("opted-out user's pet appeared in discovery results despite being followed")
		}
	}
}

// TestMatchingExcludesOptedOutUsers is TestDiscoveryExcludesOptedOutUsers's
// counterpart for internal/modules/matching/service.go's `Candidates`.
func TestMatchingExcludesOptedOutUsers(t *testing.T) {
	env := setupTestEnv(t)
	viewer := env.createUser(t, "match-optout-viewer")
	optedOut := env.createUser(t, "match-optout-target")
	env.setDiscoverable(t, optedOut, false)
	env.setPetLocation(t, viewer.PetID, 25.2048, 55.2708)
	env.setPetLocation(t, optedOut.PetID, 25.21, 55.28)

	resp := env.do(t, http.MethodGet, "/v1/pets/"+viewer.PetID.String()+"/candidates?max_distance_km=50", viewer.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	for _, item := range decodeItemsPage(t, resp) {
		if item.ID == optedOut.PetID {
			t.Fatalf("opted-out user's pet appeared in match candidates")
		}
	}
}

type meBody struct {
	IsPrivate      bool `json:"is_private"`
	IsDiscoverable bool `json:"is_discoverable"`
}

func decodeMe(t *testing.T, resp *http.Response) meBody {
	t.Helper()
	var payload struct {
		Data meBody `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode /me response: %v", err)
	}
	return payload.Data
}
