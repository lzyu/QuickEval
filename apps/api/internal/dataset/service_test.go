package dataset

import (
	"testing"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

func TestValidateTargetAvailability(t *testing.T) {
	tests := []struct {
		name     string
		info     TargetInfo
		wantCode string
	}{
		{name: "active target", info: TargetInfo{Status: "active"}},
		{name: "disabled target", info: TargetInfo{Status: "disabled"}, wantCode: "EVALUATION_TARGET_DISABLED"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTargetAvailability(test.info)
			if test.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected availability error")
			}
			if code := apperror.As(err).Code; code != test.wantCode {
				t.Fatalf("unexpected error code: got %s want %s", code, test.wantCode)
			}
		})
	}
}

func TestValidateDatasetAvailability(t *testing.T) {
	err := validateDatasetAvailability(Dataset{
		Status:       DatasetActive,
		TargetStatus: "disabled",
	})
	if err == nil || apperror.As(err).Code != "EVALUATION_TARGET_DISABLED" {
		t.Fatalf("disabled target should freeze dataset mutations: %v", err)
	}
}

func TestAssignmentStatusAllowsUnclassifiedCases(t *testing.T) {
	if got := assignmentStatus(nil); got != "unclassified" {
		t.Fatalf("missing scenario should remain unclassified, got %s", got)
	}
	scenarioID := id.MustNew()
	if got := assignmentStatus(&scenarioID); got != "confirmed" {
		t.Fatalf("manual scenario should be confirmed, got %s", got)
	}
}
