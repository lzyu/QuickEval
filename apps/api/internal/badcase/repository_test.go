package badcase

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestResultContextColumnsMatchLockQueryAliases(t *testing.T) {
	parsed, err := schema.Parse(&ResultContext{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse ResultContext schema: %v", err)
	}
	for fieldName, wantColumn := range map[string]string{
		"TargetID":         "evaluation_target_id",
		"AssignmentStatus": "scenario_assignment_status",
	} {
		field := parsed.LookUpField(fieldName)
		if field == nil {
			t.Fatalf("field %s not found", fieldName)
		}
		if field.DBName != wantColumn {
			t.Errorf("%s column = %q, want %q", fieldName, field.DBName, wantColumn)
		}
	}
}
