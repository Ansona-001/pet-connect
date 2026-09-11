package auth

import (
	"testing"
	"time"
)

func TestEvaluateOneTimeToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	usedAt := now.Add(-time.Minute)

	tests := []struct {
		name      string
		found     bool
		usedAt    *time.Time
		expiresAt time.Time
		want      oneTimeTokenStatus
	}{
		{
			name:      "happy path: found, unused, not yet expired",
			found:     true,
			usedAt:    nil,
			expiresAt: future,
			want:      tokenValid,
		},
		{
			name:      "not found: no row for the given token hash",
			found:     false,
			usedAt:    nil,
			expiresAt: future,
			want:      tokenNotFound,
		},
		{
			name:      "reuse: already used, even though still within its expiry window",
			found:     true,
			usedAt:    &usedAt,
			expiresAt: future,
			want:      tokenAlreadyUsed,
		},
		{
			name:      "expiry: unused, but expiry has passed",
			found:     true,
			usedAt:    nil,
			expiresAt: past,
			want:      tokenExpired,
		},
		{
			name:      "expiry boundary: expires_at exactly now counts as expired",
			found:     true,
			usedAt:    nil,
			expiresAt: now,
			want:      tokenExpired,
		},
		{
			name:      "reuse takes priority over expiry when both are true",
			found:     true,
			usedAt:    &usedAt,
			expiresAt: past,
			want:      tokenAlreadyUsed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := evaluateOneTimeToken(now, tt.found, tt.usedAt, tt.expiresAt)
			if got != tt.want {
				t.Fatalf("evaluateOneTimeToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
