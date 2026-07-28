package reporting

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
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
	expected := []string{
		"target", "scenario", "dataset", "case", "evaluation_result", "badcase",
	}
	if !ok || !reflect.DeepEqual(defaults, expected) {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}
	if _, ok := searchTypes("scenario,unknown"); ok {
		t.Fatal("unknown search type must be rejected")
	}
}

func TestExportRejectsRowsAboveSynchronousLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	if exportSizeValid(ctx, ExportLimit+1) {
		t.Fatal("oversized export must be rejected")
	}
	if recorder.Code != 422 {
		t.Fatalf("expected 422, got %d", recorder.Code)
	}
}

func TestEscapeLikeTreatsUserWildcardsLiterally(t *testing.T) {
	if actual := escapeLike(`50%_done\ok`); actual != `50\%\_done\\ok` {
		t.Fatalf("unexpected escaped LIKE value: %q", actual)
	}
}
