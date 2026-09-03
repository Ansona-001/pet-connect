package auth

import (
	"strings"
	"testing"
)

func TestValidateRegistration(t *testing.T) {
	t.Parallel()

	valid := registerRequest{Email: "pet@example.com", Password: "correct-horse", Name: "Alex"}
	if field, message := validateRegistration(valid); field != "" {
		t.Fatalf("valid request rejected on %s: %s", field, message)
	}

	tooLong := valid
	tooLong.Password = strings.Repeat("x", maximumPasswordBytes+1)
	if field, _ := validateRegistration(tooLong); field != "password" {
		t.Fatalf("expected password error, got %q", field)
	}
}

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()
	if got := normalizeEmail("  Pet.Owner@Example.COM "); got != "pet.owner@example.com" {
		t.Fatalf("unexpected normalized email %q", got)
	}
}
