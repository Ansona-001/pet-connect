package safety

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeReportInput(t *testing.T) {
	t.Parallel()

	input := normalizeReportInput(reportRequest{
		SubjectType: "  Post ",
		Reason:      " SPAM ",
		Details:     "  looks fake  ",
	})
	if input.SubjectType != "post" {
		t.Fatalf("SubjectType = %q, want %q", input.SubjectType, "post")
	}
	if input.Reason != "spam" {
		t.Fatalf("Reason = %q, want %q", input.Reason, "spam")
	}
	if input.Details != "looks fake" {
		t.Fatalf("Details = %q, want %q", input.Details, "looks fake")
	}
}

func TestValidateReportInput(t *testing.T) {
	t.Parallel()

	valid := reportRequest{SubjectType: "post", SubjectID: uuid.New(), Reason: "spam"}
	if err := validateReportInput(valid); err != nil {
		t.Fatalf("expected valid input to pass, got field %q: %s", err.field, err.message)
	}

	cases := []struct {
		name  string
		input reportRequest
		field string
	}{
		{"unknown subject type", reportRequest{SubjectType: "banana", SubjectID: uuid.New(), Reason: "spam"}, "subject_type"},
		{"missing subject id", reportRequest{SubjectType: "post", Reason: "spam"}, "subject_id"},
		{"unknown reason", reportRequest{SubjectType: "post", SubjectID: uuid.New(), Reason: "because"}, "reason"},
		{"details too long", reportRequest{SubjectType: "post", SubjectID: uuid.New(), Reason: "spam", Details: string(make([]rune, maxReportDetailsLength+1))}, "details"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateReportInput(tc.input)
			if err == nil {
				t.Fatalf("expected a validation error for field %q", tc.field)
			}
			if err.field != tc.field {
				t.Fatalf("field = %q, want %q", err.field, tc.field)
			}
		})
	}
}

func TestAllowedSubjectTypesAndReasons(t *testing.T) {
	t.Parallel()

	for _, subjectType := range []string{"user", "pet", "post", "comment", "story", "message"} {
		if _, ok := allowedSubjectTypes[subjectType]; !ok {
			t.Errorf("expected %q to be an allowed subject type", subjectType)
		}
	}
	for _, reason := range []string{"spam", "harassment", "inappropriate_content", "fake_profile", "animal_welfare", "other"} {
		if _, ok := allowedReportReasons[reason]; !ok {
			t.Errorf("expected %q to be an allowed report reason", reason)
		}
	}
}
