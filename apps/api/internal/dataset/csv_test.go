package dataset

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCaseCSVSupportsBOMQuotesCommasAndMultilineChinese(t *testing.T) {
	source := csvBOM + strings.Join(csvHeaders, ",") + "\n" +
		"采购询价,\"请比较 A,B 两款商品\",\"第一步\n第二步\",,不得编造,采购|询价,是\n"
	rows, err := parseCaseCSV(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 1 || rows[0].UserPrompt != "请比较 A,B 两款商品" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if rows[0].Precondition != "第一步\n第二步" || len(rows[0].TagNames) != 2 {
		t.Fatalf("multiline or tags lost: %#v", rows[0])
	}
	if rows[0].ExpectedResult != "" {
		t.Fatal("expected result must remain optional")
	}
}

func TestParseCaseCSVReportsRowAndFieldErrors(t *testing.T) {
	source := strings.Join(csvHeaders, ",") + "\n" + "名称,,,,,标签,未知\n"
	rows, err := parseCaseCSV(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 1 || rows[0].RowNumber != 2 || len(rows[0].Errors) < 2 {
		t.Fatalf("expected row errors, got %#v", rows)
	}
}

func TestWriteCaseCSVAddsBOM(t *testing.T) {
	var output bytes.Buffer
	if err := WriteCaseCSV(&output, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), csvBOM) {
		t.Fatal("CSV export must contain UTF-8 BOM")
	}
}
