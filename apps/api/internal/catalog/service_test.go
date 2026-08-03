package catalog

import (
	"testing"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

func TestValidateCaseTagScope(t *testing.T) {
	targetID := id.MustNew()
	tests := []struct {
		name      string
		scope     string
		targetID  *id.UUID
		wantField string
	}{
		{name: "global without target", scope: CaseTagScopeGlobal},
		{name: "target with owner", scope: CaseTagScopeTarget, targetID: &targetID},
		{name: "global rejects target", scope: CaseTagScopeGlobal, targetID: &targetID, wantField: "evaluation_target_id"},
		{name: "target requires owner", scope: CaseTagScopeTarget, wantField: "evaluation_target_id"},
		{name: "unknown scope", scope: "shared", wantField: "scope"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCaseTagScope(test.scope, test.targetID)
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
