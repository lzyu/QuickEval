package badcase

import (
	"testing"

	"github.com/lzyu/QuickEval/apps/api/internal/evaluation"
)

func TestValidateMarkInputRequiresScoreForResultPatch(t *testing.T) {
	description := "商品参数与实际约束冲突"
	score := uint8(2)
	valid := MarkInput{
		Description: &description,
		ResultPatch: &ResultPatch{
			Status:  evaluation.ResultEvaluated,
			Score:   &score,
			Comment: &description,
		},
	}

	if _, err := validateMarkInput(valid); err != nil {
		t.Fatalf("valid mark input returned error: %v", err)
	}

	valid.ResultPatch.Score = nil
	if _, err := validateMarkInput(valid); err == nil {
		t.Fatal("mark input without a score should fail validation")
	}
}

func TestProblemTitleUsesDescriptionAndLimitsLength(t *testing.T) {
	if got := problemTitle("  商品参数\n与实际约束冲突  "); got != "商品参数 与实际约束冲突" {
		t.Fatalf("problemTitle() = %q", got)
	}
	long := make([]rune, 205)
	for index := range long {
		long[index] = '问'
	}
	got := []rune(problemTitle(string(long)))
	if len(got) != 200 || string(got[197:]) != "..." {
		t.Fatalf("long problemTitle() length = %d, suffix = %q", len(got), string(got[197:]))
	}
}
