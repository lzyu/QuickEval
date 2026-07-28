package evaluation

import (
	"testing"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
)

func TestValidateResultInput(t *testing.T) {
	answer := "Agent 回答"
	reason := "环境不可用"
	score := uint8(4)
	tests := []struct {
		name      string
		input     ResultInput
		wantError bool
	}{
		{
			name:  "evaluated answer and optional score",
			input: ResultInput{Status: ResultEvaluated, AnswerText: &answer, Score: &score},
		},
		{
			name:  "evaluated evidence is checked against attachments in service",
			input: ResultInput{Status: ResultEvaluated},
		},
		{
			name:      "skipped requires reason",
			input:     ResultInput{Status: ResultSkipped},
			wantError: true,
		},
		{
			name:  "skipped clears score",
			input: ResultInput{Status: ResultSkipped, SkipReason: &reason, Score: &score},
		},
		{
			name:      "score lower bound",
			input:     ResultInput{Status: ResultEvaluated, AnswerText: &answer, Score: scorePointer(0)},
			wantError: true,
		},
		{
			name:      "score upper bound",
			input:     ResultInput{Status: ResultEvaluated, AnswerText: &answer, Score: scorePointer(6)},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := validateResultInput(test.input)
			if test.wantError {
				if err == nil {
					t.Fatal("expected validation error")
				}
				if apperror.As(err).Code != "VALIDATION_FAILED" {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status == ResultSkipped && result.Score != nil {
				t.Fatal("skipped result must clear score")
			}
		})
	}
}

func TestValidateRunInput(t *testing.T) {
	for _, environment := range []string{"test", "staging", "production", "other"} {
		_, err := validateRunInput(RunInput{
			AgentVersion: "2026.07.1",
			Environment:  environment,
		})
		if err != nil {
			t.Fatalf("environment %s should be valid: %v", environment, err)
		}
	}
	if _, err := validateRunInput(RunInput{
		AgentVersion: "2026.07.1",
		Environment:  "local",
	}); err == nil {
		t.Fatal("unexpected environment should be rejected")
	}
}

func scorePointer(value uint8) *uint8 { return &value }
