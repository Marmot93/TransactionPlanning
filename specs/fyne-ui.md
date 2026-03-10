# main.go Fyne 图形界面改造方案

## 1. 问题说明

当前 [main.go](/Users/marmotmr./GolandProjects/diff_str/main.go) 中的策略决策逻辑已经可用，但 `main()` 仍然依赖命令行交互输入参数、在终端打印结果。文件顶部的 TODO 要求改为使用 `fyne.io/fyne/v2` 构建图形界面，让输入和输出都通过桌面窗口完成，而不是通过标准输入输出。

本次任务的目标是：

- 保留现有 `Decide`、`validateInput`、`validateState` 等策略逻辑不变。
- 将启动入口从命令行交互改为 Fyne 窗口。
- 窗口标题使用“红利低波100交易规划器”。
- 允许用户在界面中输入指数、均线、仓位和高位阶段状态。
- 所有输入项默认保持为空，不预填数值。
- 在界面中直接显示中文 `Decision` 结果和中文校验错误/警告。

## 2. 对现有项目的影响

受影响的内容主要集中在单文件应用入口：

- [main.go](/Users/marmotmr./GolandProjects/diff_str/main.go)
  - `main()` 将从命令行读写改为启动 Fyne 应用。
  - 新增 UI 组件构造、事件处理、结果展示逻辑。
  - 命令行专用函数 `mustReadFloat`、`mustReadBool`、`mustReadLine` 将删除或不再使用。
- 测试文件
  - 新增 `main_test.go`，补充对 UI 解析辅助逻辑或关键输出格式函数的测试，避免只改入口却没有回归验证。

不涉及：

- `go.mod` 依赖调整：当前已经包含 `fyne.io/fyne/v2 v2.7.1`，预期无需新增依赖。
- 策略规则调整：仓位变化、高位阶段与均线判断规则保持不变。
- 数据存储、网络请求、数据库或外部 API。

## 3. 实现方案

### 3.1 总体思路

保留现有纯计算逻辑，把 UI 作为输入收集和结果渲染层。这样可以避免把策略逻辑混进组件事件中，也便于后续测试。

`main()` 将改为：

1. 创建 Fyne app 和主窗口。
2. 窗口标题设置为“红利低波100交易规划器”。
3. 构建输入表单：
   - `IndexValue`
   - `MA120`
   - `MA20`
   - `Position`（下拉选择固定仓位）
   - `HighPhase`（布尔勾选）
4. 所有输入控件初始不设置默认数值；`Position` 可保持未选择状态，或按实现需要提示用户先选择。
5. 提供“计算”按钮。
6. 点击按钮后：
   - 解析输入框内容；
   - 失败则在界面中显示错误；
   - 成功则调用 `Decide(input, state)`；
   - 将 `Decision` 渲染到结果区域，结果文案使用中文。

### 3.2 代码组织

仍然尽量保持在 [main.go](/Users/marmotmr./GolandProjects/diff_str/main.go) 中完成，避免对这个小工具做无意义拆分。但会新增少量辅助函数，以便测试和降低 UI 事件处理复杂度。

计划新增的辅助函数：

- `parseFloatField(label, value string) (float64, error)`
  - 负责把文本输入转换为数值，并携带中文字段名返回错误。
- `parsePosition(value string) (float64, error)`
  - 负责把下拉框选中的仓位字符串映射为策略使用的浮点值。
- `buildDecisionText(decision Decision) string`
  - 负责把决策结果格式化成中文多行文本，供界面展示。
- `buildUI(...) fyne.CanvasObject` 或在 `main()` 中直接组装控件
  - 负责布局输入框、按钮和结果展示区域。

### 3.3 界面布局

界面使用 Fyne 的基础组件即可，优先保证可用性：

- `widget.Entry`：数值输入
- `widget.Select`：仓位下拉选择
- `widget.Check`：高位阶段布尔输入
- `widget.Button`：触发计算
- `widget.Label` 或只读 `widget.Entry`：显示结果
- `container.NewVBox` / `container.NewGridWithColumns` / `widget.Form`：组织布局

拟采用 `widget.Form` 组织输入，结果区域放在按钮下方。结果展示示意：

```go
result.SetText(buildDecisionText(decision))
```

其中 `Position` 不再允许自由输入浮点数，而是限定为以下中文标签选项：

- `满仓`
- `2/3`
- `1/3`
- `空仓`

界面提交时通过 `parsePosition` 将中文选项映射为 `float64`：

- `满仓` -> `1.0`
- `2/3` -> `2.0 / 3.0`
- `1/3` -> `1.0 / 3.0`
- `空仓` -> `0.0`

这样可以避免非法仓位输入，同时让界面文案更自然。

错误展示方式：

- 输入解析失败：直接在结果区域展示中文错误文本。
- `decision.Valid == false`：展示中文原因。
- `Warnings` 非空：附加在结果文本末尾，使用中文标签。

### 3.4 文案与默认值约束

- 窗口标题固定为“红利低波100交易规划器”。
- 所有输入框初始化为空，不预填任何默认数值。
- `Position` 使用固定下拉选项：`满仓`、`2/3`、`1/3`、`空仓`。
- `HighPhase` 使用勾选框，勾选表示 `true`，未勾选表示 `false`。
- 结果区所有标签与内容描述尽量使用中文，避免混用英文键名。
- 不在本次改造中引入复杂状态管理、实时联动计算或多窗口。
- 不修改 `Decision` 数据结构，避免影响已有逻辑。

### 3.5 验证方案

由于 Fyne 主窗口本身不适合在这里做完整交互测试，测试聚焦在可稳定验证的非 GUI 辅助逻辑：

- `parseFloatField`
  - 正常数字输入
  - 非法数字输入
- `parsePosition`
  - 下拉选项到浮点仓位的映射正确
  - 非预期选项返回错误
- `buildDecisionText`
  - 包含 `Warnings`
  - 不包含 `Warnings`
- 如有必要，继续保留对 `Decide` 的行为验证，确保 UI 改造没有破坏策略逻辑接入。

本轮按你的要求，不在这里执行自动化测试，由你手动验证 GUI 行为。

## 4. Todo List

- [x] 在 [main.go](/Users/marmotmr./GolandProjects/diff_str/main.go) 中移除命令行交互入口，改为 Fyne 应用入口
- [x] 新增输入控件与结果展示控件，完成窗口布局
- [x] 新增输入解析辅助函数，处理数值字段错误提示
- [x] 将 `Position` 改为固定选项下拉框，并完成选项到仓位值的映射
- [x] 将 `HighPhase` 改为布尔勾选框
- [x] 将窗口标题调整为“红利低波100交易规划器”
- [x] 移除所有输入默认值，窗口初始状态保持空白
- [x] 将结果区输出改为中文文案
- [x] 将输入校验错误改为中文提示
- [x] 删除或停用命令行专用读取函数
- [x] 新增/更新测试，覆盖新增的可测试辅助逻辑
- [x] 按你的要求跳过本地自动化测试，改为由你手动验证
