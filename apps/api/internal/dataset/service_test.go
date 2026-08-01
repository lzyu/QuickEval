package dataset

import (
	"testing"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
)

func TestValidateScenarioAvailability(t *testing.T) {
	tests := []struct {
		name     string
		info     ScenarioInfo
		wantCode string
	}{
		{name: "active ownership chain", info: ScenarioInfo{Status: "active", TargetStatus: "active"}},
		{name: "disabled scenario", info: ScenarioInfo{Status: "disabled", TargetStatus: "active"}, wantCode: "SCENARIO_DISABLED"},
		{name: "disabled target", info: ScenarioInfo{Status: "active", TargetStatus: "disabled"}, wantCode: "EVALUATION_TARGET_DISABLED"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateScenarioAvailability(test.info)
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
		Status:         DatasetActive,
		ScenarioStatus: "active",
		TargetStatus:   "disabled",
	})
	if err == nil || apperror.As(err).Code != "EVALUATION_TARGET_DISABLED" {
		t.Fatalf("disabled target should freeze dataset mutations: %v", err)
	}
}
