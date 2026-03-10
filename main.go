package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Action 表示当前时点策略给出的操作建议。
type Action string

const (
	ActionHold         Action = "hold"
	ActionReduceToTwo3 Action = "reduce_to_2_3"
	ActionReduceToOne3 Action = "reduce_to_1_3"
	ActionClearAll     Action = "clear_all"
	ActionReenterFull  Action = "reenter_full"
)

// State 保存策略运行所需的最小状态。
//
// Position 取值约定：
//   - 1.0   表示满仓
//   - 2/3   表示第一次减仓后
//   - 1/3   表示第二次减仓后
//   - 0.0   表示空仓
//
// HighPhase 表示是否已经进入过“高位阶段”。
// 只有在出现过 bias >= 8% 之后，close < MA20 才允许触发清仓。
type State struct {
	Position  float64
	HighPhase bool
}

// Input 是单次决策所需的市场输入。
type Input struct {
	IndexValue float64
	MA120      float64
	MA20       float64
}

// Decision 返回一次决策的完整结果。
type Decision struct {
	Action   Action
	State    State
	BiasPct  float64
	Reason   string
	Valid    bool
	Warnings []string
}

// Decide 根据当前指数、均线和历史状态，返回本次应执行的操作。
//
// 规则如下：
// 1. bias < 8%：持有
// 2. bias >= 8%：若当前仓位高于 2/3，则减仓到 2/3
// 3. bias >= 10%：若当前仓位高于 1/3，则再减仓到 1/3
// 4. 高位后 close < MA20：清掉剩余仓位
// 5. 空仓后 close < MA120 * 0.97：全额买回
//
// 这里默认“信号在当前时点生成，由调用方决定是否次日执行”。
// 函数本身只负责给出应执行的动作和更新后的状态。
func Decide(input Input, current State) Decision {
	decision := Decision{
		Action: ActionHold,
		State:  current,
		Valid:  true,
	}

	if err := validateInput(input); err != nil {
		decision.Valid = false
		decision.Reason = err.Error()
		return decision
	}

	if warning := validateState(current); warning != "" {
		decision.Warnings = append(decision.Warnings, warning)
	}

	biasPct := calcBiasPct(input.IndexValue, input.MA120)
	decision.BiasPct = biasPct

	// 空仓状态下，只看是否满足深度回撤后的重新买回条件。
	if isZeroPosition(current.Position) {
		if input.IndexValue < input.MA120*0.97 {
			decision.Action = ActionReenterFull
			decision.State.Position = 1.0
			decision.State.HighPhase = false
			decision.Reason = "当前为空仓，且指数低于 MA120 的 97%，满足重新满仓条件"
			return decision
		}

		decision.Reason = "当前为空仓，暂未满足重新满仓条件"
		return decision
	}

	// 一旦达到 8% 偏离，进入高位阶段。
	if biasPct >= 8.0 {
		decision.State.HighPhase = true

		if current.Position > 2.0/3.0 {
			decision.Action = ActionReduceToTwo3
			decision.State.Position = 2.0 / 3.0
			decision.Reason = "乖离率达到 8%，当前仓位高于 2/3，建议减仓到 2/3"
			return decision
		}

		if biasPct >= 10.0 && current.Position > 1.0/3.0 {
			decision.Action = ActionReduceToOne3
			decision.State.Position = 1.0 / 3.0
			decision.Reason = "乖离率达到 10%，当前仓位高于 1/3，建议减仓到 1/3"
			return decision
		}
	}

	// 只有经历过高位阶段，跌破 MA20 才视为卖出剩余仓位的确认信号。
	if decision.State.HighPhase && input.IndexValue < input.MA20 && current.Position > 0 {
		decision.Action = ActionClearAll
		decision.State.Position = 0
		decision.State.HighPhase = false
		decision.Reason = "已经历高位阶段，且指数跌破 MA20，建议清仓"
		return decision
	}

	decision.Reason = "当前未触发新的交易规则，继续持有观察"
	return decision
}

func calcBiasPct(indexValue, ma120 float64) float64 {
	return (indexValue/ma120 - 1.0) * 100.0
}

func validateInput(input Input) error {
	if input.IndexValue <= 0 {
		return fmt.Errorf("指数点位无效：%.4f", input.IndexValue)
	}
	if input.MA120 <= 0 {
		return fmt.Errorf("120 日均线无效：%.4f", input.MA120)
	}
	if input.MA20 <= 0 {
		return fmt.Errorf("20 日均线无效：%.4f", input.MA20)
	}
	return nil
}

func validateState(state State) string {
	switch {
	case isSamePosition(state.Position, 1.0):
		return ""
	case isSamePosition(state.Position, 2.0/3.0):
		return ""
	case isSamePosition(state.Position, 1.0/3.0):
		return ""
	case isZeroPosition(state.Position):
		return ""
	default:
		return "当前仓位不是预设状态：满仓、2/3、1/3、空仓"
	}
}

func isZeroPosition(position float64) bool {
	return isSamePosition(position, 0)
}

func isSamePosition(left, right float64) bool {
	const epsilon = 1e-9
	diff := left - right
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

func main() {
	application := app.New()
	window := application.NewWindow("红利低波100(930955)交易规划器")
	window.Resize(fyne.NewSize(560, 420))

	indexEntry := widget.NewEntry()
	indexEntry.SetPlaceHolder("请输入当前指数点位")

	ma120Entry := widget.NewEntry()
	ma120Entry.SetPlaceHolder("请输入 120 日均线")

	ma20Entry := widget.NewEntry()
	ma20Entry.SetPlaceHolder("请输入 20 日均线")

	positionSelect := widget.NewSelect([]string{
		"满仓",
		"2/3",
		"1/3",
		"空仓",
	}, nil)
	positionSelect.PlaceHolder = "请选择当前仓位"

	highPhaseCheck := widget.NewCheck("", nil)

	result := widget.NewLabel("请填写参数后点击“计算”。")
	result.Wrapping = fyne.TextWrapWord

	runDecision := func() {
		indexValue, err := parseFloatField("指数点位", indexEntry.Text)
		if err != nil {
			result.SetText(err.Error())
			return
		}

		ma120, err := parseFloatField("120 日均线", ma120Entry.Text)
		if err != nil {
			result.SetText(err.Error())
			return
		}

		ma20, err := parseFloatField("20 日均线", ma20Entry.Text)
		if err != nil {
			result.SetText(err.Error())
			return
		}

		position, err := parsePosition(positionSelect.Selected)
		if err != nil {
			result.SetText(err.Error())
			return
		}

		decision := Decide(Input{
			IndexValue: indexValue,
			MA120:      ma120,
			MA20:       ma20,
		}, State{
			Position:  position,
			HighPhase: highPhaseCheck.Checked,
		})

		result.SetText(buildDecisionText(decision))
	}

	form := widget.NewForm(
		widget.NewFormItem("指数点位", indexEntry),
		widget.NewFormItem("120 日均线", ma120Entry),
		widget.NewFormItem("20 日均线", ma20Entry),
		widget.NewFormItem("当前仓位", positionSelect),
		widget.NewFormItem("已进入高位阶段", highPhaseCheck),
	)
	form.SubmitText = "计算"
	form.OnSubmit = runDecision

	content := container.NewVBox(
		widget.NewLabel("红利低波100交易规划器"),
		widget.NewSeparator(),
		form,
		widget.NewSeparator(),
		widget.NewLabel("计算结果"),
		result,
	)

	window.SetContent(container.NewPadded(content))
	window.ShowAndRun()
}

func parseFloatField(label, value string) (float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("请输入%s", label)
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("%s格式不正确：%q", label, value)
	}
	return parsed, nil
}

func parsePosition(value string) (float64, error) {
	switch value {
	case "满仓":
		return 1.0, nil
	case "2/3":
		return 2.0 / 3.0, nil
	case "1/3":
		return 1.0 / 3.0, nil
	case "空仓":
		return 0.0, nil
	default:
		return 0, fmt.Errorf("请选择当前仓位")
	}
}

func buildDecisionText(decision Decision) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "结果有效：%s\n", boolText(decision.Valid))
	fmt.Fprintf(&builder, "操作建议：%s\n", actionText(decision.Action))
	fmt.Fprintf(&builder, "120 日乖离率：%.4f%%\n", decision.BiasPct)
	fmt.Fprintf(&builder, "原因说明：%s\n", decision.Reason)
	fmt.Fprintf(&builder, "下一仓位：%s\n", positionText(decision.State.Position))
	fmt.Fprintf(&builder, "下一步高位阶段：%s", boolText(decision.State.HighPhase))

	if len(decision.Warnings) > 0 {
		builder.WriteString("\n提示信息：")
		for _, warning := range decision.Warnings {
			builder.WriteString("\n- ")
			builder.WriteString(warning)
		}
	}

	return builder.String()
}

func actionText(action Action) string {
	switch action {
	case ActionHold:
		return "持有"
	case ActionReduceToTwo3:
		return "减仓到 2/3"
	case ActionReduceToOne3:
		return "减仓到 1/3"
	case ActionClearAll:
		return "清仓"
	case ActionReenterFull:
		return "重新满仓"
	default:
		return string(action)
	}
}

func positionText(position float64) string {
	switch {
	case isSamePosition(position, 1.0):
		return "满仓"
	case isSamePosition(position, 2.0/3.0):
		return "2/3"
	case isSamePosition(position, 1.0/3.0):
		return "1/3"
	case isZeroPosition(position):
		return "空仓"
	default:
		return fmt.Sprintf("%.10f", position)
	}
}

func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}
