package pets

import (
	"reflect"
	"testing"
	"time"
)

func TestNormalizeCreate(t *testing.T) {
	t.Parallel()

	birthDate := "2022-04-03"
	input := createRequest{
		Name: "  Luna  ", PetType: "Dog", Gender: "Female", BirthDate: &birthDate,
		Personality: []string{" Friendly ", "friendly", "Playful"},
		Interests:   []string{},
	}
	parsed, err := normalizeCreate(&input, time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if parsed == nil || parsed.Format("2006-01-02") != birthDate {
		t.Fatalf("unexpected birth date: %v", parsed)
	}
	if input.Name != "Luna" || input.PetType != "dog" || input.Gender != "female" {
		t.Fatalf("input was not normalized: %#v", input)
	}
	if !reflect.DeepEqual(input.Personality, []string{"Friendly", "Playful"}) {
		t.Fatalf("unexpected personality values: %#v", input.Personality)
	}
}

func TestFutureBirthDateRejected(t *testing.T) {
	t.Parallel()

	birthDate := "2027-01-01"
	input := createRequest{Name: "Luna", PetType: "dog", Gender: "unknown", BirthDate: &birthDate}
	_, err := normalizeCreate(&input, time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC))
	if err == nil || err.field != "birth_date" {
		t.Fatalf("expected birth_date error, got %#v", err)
	}
}
