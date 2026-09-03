package account

import "testing"

func TestProfileValidation(t *testing.T) {
	t.Parallel()

	if !validCoordinates(25.2048, 55.2708) {
		t.Fatal("expected Dubai coordinates to be valid")
	}
	if validCoordinates(91, 55) {
		t.Fatal("expected out-of-range latitude to be invalid")
	}
	for _, value := range []string{"", "/media/profile/avatar.jpg", "https://cdn.example.com/avatar.jpg"} {
		if !validMediaURL(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
	if validMediaURL("javascript:alert(1)") {
		t.Fatal("expected unsafe URL scheme to be rejected")
	}
}
