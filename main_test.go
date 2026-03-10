package main

import (
	"math"
	"strings"
	"testing"
)

func TestParseFloatField(t *testing.T) {
	value, err := parseFloatField("120 日均线", " 123.45 ")
	if err != nil {
		t.Fatalf("parseFloatField returned error: %v", err)
	}
	if value != 123.45 {
		t.Fatalf("parseFloatField returned %v, want 123.45", value)
	}
}

func TestParseFloatField_Invalid(t *testing.T) {
	_, err := parseFloatField("20 日均线", "abc")
	if err == nil {
		t.Fatal("parseFloatField returned nil error, want invalid input error")
	}
	if !strings.Contains(err.Error(), "20 日均线格式不正确") {
		t.Fatalf("parseFloatField error = %q, want field name in error", err.Error())
	}
}

func TestParsePosition(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  float64
	}{
		{name: "full", input: "满仓", want: 1.0},
		{name: "two thirds", input: "2/3", want: 2.0 / 3.0},
		{name: "one third", input: "1/3", want: 1.0 / 3.0},
		{name: "empty", input: "空仓", want: 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePosition(tc.input)
			if err != nil {
				t.Fatalf("parsePosition returned error: %v", err)
			}
			if !isSamePosition(got, tc.want) {
				t.Fatalf("parsePosition(%q) = %.10f, want %.10f", tc.input, got, tc.want)
			}
		})
	}
}

func TestParsePosition_Invalid(t *testing.T) {
	_, err := parsePosition("0.5")
	if err == nil {
		t.Fatal("parsePosition returned nil error, want invalid option error")
	}
}

func TestBuildDecisionText(t *testing.T) {
	decision := Decision{
		Action:  ActionReduceToTwo3,
		State:   State{Position: 2.0 / 3.0, HighPhase: true},
		BiasPct: 8.1234,
		Reason:  "bias reached 8% and position is above 2/3",
		Valid:   true,
		Warnings: []string{
			"position is not one of the canonical states: 1.0, 2/3, 1/3, 0.0",
		},
	}

	text := buildDecisionText(decision)

	expectedSnippets := []string{
		"结果有效：是",
		"操作建议：减仓到 2/3",
		"120 日乖离率：8.1234%",
		"原因说明：bias reached 8% and position is above 2/3",
		"下一仓位：2/3",
		"下一步高位阶段：是",
		"提示信息：",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(text, snippet) {
			t.Fatalf("buildDecisionText() missing %q in %q", snippet, text)
		}
	}
}

func TestDecide_InvalidInput(t *testing.T) {
	testCases := []struct {
		name    string
		input   Input
		wantMsg string
	}{
		{
			name:    "invalid index value",
			input:   Input{IndexValue: 0, MA120: 100, MA20: 90},
			wantMsg: "指数点位无效",
		},
		{
			name:    "invalid ma120",
			input:   Input{IndexValue: 100, MA120: 0, MA20: 90},
			wantMsg: "120 日均线无效",
		},
		{
			name:    "invalid ma20",
			input:   Input{IndexValue: 100, MA120: 100, MA20: 0},
			wantMsg: "20 日均线无效",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(tc.input, State{Position: 1.0})

			if got.Valid {
				t.Fatal("Decide() Valid = true, want false")
			}
			if got.Action != ActionHold {
				t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionHold)
			}
			if !strings.Contains(got.Reason, tc.wantMsg) {
				t.Fatalf("Decide() Reason = %q, want substring %q", got.Reason, tc.wantMsg)
			}
		})
	}
}

func TestDecide_EmptyPositionReenter(t *testing.T) {
	got := Decide(
		Input{IndexValue: 96.9, MA120: 100, MA20: 98},
		State{Position: 0, HighPhase: true},
	)

	if !got.Valid {
		t.Fatalf("Decide() Valid = false, want true, reason = %q", got.Reason)
	}
	if got.Action != ActionReenterFull {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionReenterFull)
	}
	if !isSamePosition(got.State.Position, 1.0) {
		t.Fatalf("Decide() next position = %.10f, want 1.0", got.State.Position)
	}
	if got.State.HighPhase {
		t.Fatal("Decide() next high phase = true, want false")
	}
	if got.Reason != "当前为空仓，且指数低于 MA120 的 97%，满足重新满仓条件" {
		t.Fatalf("Decide() Reason = %q", got.Reason)
	}
}

func TestDecide_EmptyPositionHold(t *testing.T) {
	got := Decide(
		Input{IndexValue: 97, MA120: 100, MA20: 98},
		State{Position: 0, HighPhase: false},
	)

	if got.Action != ActionHold {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionHold)
	}
	if !isZeroPosition(got.State.Position) {
		t.Fatalf("Decide() next position = %.10f, want 0", got.State.Position)
	}
	if got.State.HighPhase {
		t.Fatal("Decide() next high phase = true, want false")
	}
	if got.Reason != "当前为空仓，暂未满足重新满仓条件" {
		t.Fatalf("Decide() Reason = %q", got.Reason)
	}
}

func TestDecide_ReduceToTwoThirds(t *testing.T) {
	got := Decide(
		Input{IndexValue: 108, MA120: 100, MA20: 100},
		State{Position: 1.0, HighPhase: false},
	)

	if got.Action != ActionReduceToTwo3 {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionReduceToTwo3)
	}
	if !isSamePosition(got.State.Position, 2.0/3.0) {
		t.Fatalf("Decide() next position = %.10f, want %.10f", got.State.Position, 2.0/3.0)
	}
	if !got.State.HighPhase {
		t.Fatal("Decide() next high phase = false, want true")
	}
	if !isNearlyEqual(got.BiasPct, 8.0) {
		t.Fatalf("Decide() BiasPct = %.4f, want 8.0000", got.BiasPct)
	}
	if got.Reason != "乖离率达到 8%，当前仓位高于 2/3，建议减仓到 2/3" {
		t.Fatalf("Decide() Reason = %q", got.Reason)
	}
}

func TestDecide_ReduceToOneThird(t *testing.T) {
	got := Decide(
		Input{IndexValue: 110, MA120: 100, MA20: 100},
		State{Position: 2.0 / 3.0, HighPhase: false},
	)

	if got.Action != ActionReduceToOne3 {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionReduceToOne3)
	}
	if !isSamePosition(got.State.Position, 1.0/3.0) {
		t.Fatalf("Decide() next position = %.10f, want %.10f", got.State.Position, 1.0/3.0)
	}
	if !got.State.HighPhase {
		t.Fatal("Decide() next high phase = false, want true")
	}
	if !isNearlyEqual(got.BiasPct, 10.0) {
		t.Fatalf("Decide() BiasPct = %.4f, want 10.0000", got.BiasPct)
	}
	if got.Reason != "乖离率达到 10%，当前仓位高于 1/3，建议减仓到 1/3" {
		t.Fatalf("Decide() Reason = %q", got.Reason)
	}
}

func TestDecide_ClearAllAfterHighPhase(t *testing.T) {
	got := Decide(
		Input{IndexValue: 95, MA120: 100, MA20: 96},
		State{Position: 1.0 / 3.0, HighPhase: true},
	)

	if got.Action != ActionClearAll {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionClearAll)
	}
	if !isZeroPosition(got.State.Position) {
		t.Fatalf("Decide() next position = %.10f, want 0", got.State.Position)
	}
	if got.State.HighPhase {
		t.Fatal("Decide() next high phase = true, want false")
	}
	if got.Reason != "已经历高位阶段，且指数跌破 MA20，建议清仓" {
		t.Fatalf("Decide() Reason = %q", got.Reason)
	}
}

func TestDecide_DoesNotClearWithoutHighPhase(t *testing.T) {
	got := Decide(
		Input{IndexValue: 95, MA120: 100, MA20: 96},
		State{Position: 1.0 / 3.0, HighPhase: false},
	)

	if got.Action != ActionHold {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionHold)
	}
	if !isSamePosition(got.State.Position, 1.0/3.0) {
		t.Fatalf("Decide() next position = %.10f, want %.10f", got.State.Position, 1.0/3.0)
	}
	if got.State.HighPhase {
		t.Fatal("Decide() next high phase = true, want false")
	}
	if got.Reason != "当前未触发新的交易规则，继续持有观察" {
		t.Fatalf("Decide() Reason = %q", got.Reason)
	}
}

func TestDecide_HoldWhenNoRuleMatches(t *testing.T) {
	got := Decide(
		Input{IndexValue: 105, MA120: 100, MA20: 100},
		State{Position: 1.0, HighPhase: false},
	)

	if got.Action != ActionHold {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionHold)
	}
	if !isSamePosition(got.State.Position, 1.0) {
		t.Fatalf("Decide() next position = %.10f, want 1.0", got.State.Position)
	}
	if got.State.HighPhase {
		t.Fatal("Decide() next high phase = true, want false")
	}
	if got.Reason != "当前未触发新的交易规则，继续持有观察" {
		t.Fatalf("Decide() Reason = %q", got.Reason)
	}
}

func TestDecide_WarnsOnNonCanonicalPosition(t *testing.T) {
	got := Decide(
		Input{IndexValue: 105, MA120: 100, MA20: 100},
		State{Position: 0.5, HighPhase: false},
	)

	if got.Valid == false {
		t.Fatalf("Decide() Valid = false, want true, reason = %q", got.Reason)
	}
	if got.Action != ActionHold {
		t.Fatalf("Decide() Action = %q, want %q", got.Action, ActionHold)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("Decide() warnings len = %d, want 1", len(got.Warnings))
	}
	if got.Warnings[0] != "当前仓位不是预设状态：满仓、2/3、1/3、空仓" {
		t.Fatalf("Decide() warning = %q", got.Warnings[0])
	}
}

func isNearlyEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
