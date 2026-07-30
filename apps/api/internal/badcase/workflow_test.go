package badcase

import (
	"testing"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

func TestValidTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{StatusPending, StatusProcessing, true},
		{StatusPending, StatusResolved, true},
		{StatusPending, StatusDeferred, true},
		{StatusProcessing, StatusResolved, true},
		{StatusProcessing, StatusDeferred, true},
		{StatusResolved, StatusPending, true},
		{StatusDeferred, StatusPending, true},
		{StatusPending, StatusPending, false},
		{StatusResolved, StatusProcessing, false},
		{StatusDeferred, StatusResolved, false},
	}
	for _, test := range tests {
		t.Run(test.from+"_"+test.to, func(t *testing.T) {
			if got := validTransition(test.from, test.to); got != test.want {
				t.Fatalf("validTransition(%q, %q) = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}

func TestValidateBusinessInput(t *testing.T) {
	input := BusinessInput{
		ScenarioID: id.MustNew(), Title: "预算约束失效",
		Environment: "production", OccurredAt: time.Now().UTC(),
		IssueTagIDs: []id.UUID{id.MustNew()},
	}
	got, err := validateBusinessInput(input, true)
	if err != nil {
		t.Fatalf("validateBusinessInput() error = %v", err)
	}
	if got.Title != input.Title || got.Description != nil {
		t.Fatalf("unexpected normalized input: %#v", got)
	}
}

func TestValidateBusinessInputNormalizesBlankDescription(t *testing.T) {
	description := "   "
	input := BusinessInput{
		ScenarioID: id.MustNew(), Title: "预算约束失效", Description: &description,
		Environment: "production", OccurredAt: time.Now().UTC(),
		IssueTagIDs: []id.UUID{id.MustNew()},
	}
	got, err := validateBusinessInput(input, true)
	if err != nil {
		t.Fatalf("validateBusinessInput() error = %v", err)
	}
	if got.Description != nil {
		t.Fatalf("description = %#v, want nil", got.Description)
	}
}

func TestAllowedActionsRespectValidityAndOwnership(t *testing.T) {
	ownerID := id.MustNew()
	otherID := id.MustNew()
	item := Badcase{CreatedBy: ownerID, SourceType: "business", Status: StatusPending}
	ownerActions := allowedActions(item, ownerID, false)
	if !contains(ownerActions, "edit") || !contains(ownerActions, "invalidate") ||
		!contains(ownerActions, "resolve") {
		t.Fatalf("owner actions = %v", ownerActions)
	}
	memberActions := allowedActions(item, otherID, false)
	if contains(memberActions, "edit") || contains(memberActions, "invalidate") ||
		!contains(memberActions, "add_note") {
		t.Fatalf("member actions = %v", memberActions)
	}
	now := time.Now().UTC()
	item.InvalidatedAt = &now
	if actions := allowedActions(item, otherID, false); len(actions) != 0 {
		t.Fatalf("invalid item member actions = %v", actions)
	}
	if actions := allowedActions(item, ownerID, false); len(actions) != 1 ||
		actions[0] != "reactivate" {
		t.Fatalf("invalid item owner actions = %v", actions)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
