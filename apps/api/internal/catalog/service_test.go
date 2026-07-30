package catalog

import (
	"testing"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

func TestValidateCaseTagScope(t *testing.T) {
	scenarioID := id.MustNew()
	tests := []struct {
		name       string
		scope      string
		scenarioID *id.UUID
		wantField  string
	}{
		{name: "global without scenario", scope: CaseTagScopeGlobal},
		{name: "scenario with owner", scope: CaseTagScopeScenario, scenarioID: &scenarioID},
		{name: "global rejects scenario", scope: CaseTagScopeGlobal, scenarioID: &scenarioID, wantField: "scenario_id"},
		{name: "scenario requires owner", scope: CaseTagScopeScenario, wantField: "scenario_id"},
		{name: "unknown scope", scope: "shared", wantField: "scope"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCaseTagScope(test.scope, test.scenarioID)
			if test.wantField == "" {
				if err != nil {
					t.Fatalf("validateCaseTagScope() error = %v", err)
				}
				return
			}
			appError := apperror.As(err)
			if len(appError.FieldErrors) != 1 || appError.FieldErrors[0].Field != test.wantField {
				t.Fatalf("field errors = %#v, want field %q", appError.FieldErrors, test.wantField)
			}
		})
	}
}
