package sources

import (
	"errors"
	"fmt"
	"testing"
)

func TestRedactError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          error
		secrets      []string
		wantMessage  string
		wantIdentity bool
	}{
		{
			name:         "nil error",
			err:          nil,
			secrets:      []string{"secret"},
			wantIdentity: true,
		},
		{
			name:         "no secrets",
			err:          errors.New("plain message"),
			wantMessage:  "plain message",
			wantIdentity: true,
		},
		{
			name:        "secret replaced",
			err:         errors.New("url https://x/?k=secret-key failed"),
			secrets:     []string{"secret-key"},
			wantMessage: "url https://x/?k=[REDACTED] failed",
		},
		{
			name:        "multiple occurrences and secrets",
			err:         errors.New("a=first b=second a=first"),
			secrets:     []string{"first", "second"},
			wantMessage: "a=[REDACTED] b=[REDACTED] a=[REDACTED]",
		},
		{
			name:         "empty secret is skipped",
			err:          errors.New("unchanged"),
			secrets:      []string{""},
			wantMessage:  "unchanged",
			wantIdentity: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			redacted := RedactError(test.err, test.secrets...)

			if test.wantIdentity {
				if test.err == nil {
					if redacted != nil {
						t.Fatalf("got a new error %v, want nil", redacted)
					}

					return
				}

				if !errors.Is(redacted, test.err) || redacted.Error() != test.err.Error() {
					t.Fatalf("got %v, want the same error %v", redacted, test.err)
				}

				return
			}

			if redacted.Error() != test.wantMessage {
				t.Errorf("message = %q, want %q", redacted.Error(), test.wantMessage)
			}

			if !errors.Is(redacted, test.err) {
				t.Errorf("errors.Is failed to follow the chain")
			}
		})
	}

	t.Run("unwraps to a sentinel", func(t *testing.T) {
		t.Parallel()

		sentinel := errors.New("sentinel")

		redacted := RedactError(fmt.Errorf("wrapping: %w", sentinel), "sentinel")

		if redacted.Error() != "wrapping: [REDACTED]" {
			t.Errorf("message = %q, want %q", redacted.Error(), "wrapping: [REDACTED]")
		}

		if !errors.Is(redacted, sentinel) {
			t.Errorf("errors.Is(redacted, sentinel) = false, want true")
		}
	})
}
