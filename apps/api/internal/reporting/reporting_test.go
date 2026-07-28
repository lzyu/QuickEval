package reporting

import (
	"reflect"
	"testing"
)

func TestRatioEmptySampleIsNull(t *testing.T) {
	if ratio(0, 0) != nil {
		t.Fatal("empty sample must not be represented as a misleading zero rate")
	}
}

func TestRatioUsesEvaluatedDenominator(t *testing.T) {
	value := ratio(2, 5)
	if value == nil || *value != 0.4 {
		t.Fatalf("expected 0.4, got %v", value)
	}
}

func TestCSVSafeRowPreventsSpreadsheetFormulaExecution(t *testing.T) {
	input := []string{"=1+1", " +cmd", "-2", "@SUM(A1)", "normal"}
	expected := []string{"'=1+1", "' +cmd", "'-2", "'@SUM(A1)", "normal"}
	if actual := csvSafeRow(input); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected escaped values: %#v", actual)
	}
}

func TestSearchTypesDefaultsAndRejectsUnknown(t *testing.T) {
	defaults, ok := searchTypes("")
	if !ok || !reflect.DeepEqual(defaults, []string{"scenario", "dataset", "case", "badcase"}) {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}
	if _, ok := searchTypes("scenario,unknown"); ok {
		t.Fatal("unknown search type must be rejected")
	}
}

func TestEscapeLikeTreatsUserWildcardsLiterally(t *testing.T) {
	if actual := escapeLike(`50%_done\ok`); actual != `50\%\_done\\ok` {
		t.Fatalf("unexpected escaped LIKE value: %q", actual)
	}
}
