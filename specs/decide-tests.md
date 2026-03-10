# Decide 完整测试补充方案

## 1. 问题说明

当前项目里的 [main.go](/Users/marmotmr./GolandProjects/diff_str/main.go) 已经实现了 `Decide(input, current)` 这段核心策略逻辑，但 [main_test.go](/Users/marmotmr./GolandProjects/diff_str/main_test.go) 还没有针对 `Decide` 本身建立系统化测试。现有测试主要覆盖：

- `parseFloatField`
- `parsePosition`
- `buildDecisionText`

这意味着最核心的交易决策规则仍然缺少回归保护。一旦后续调整中文文案、状态处理或者阈值判断，很容易在不自知的情况下破坏策略行为。

本次任务的目标是：

- 为 `Decide` 补充完整的表驱动测试。
- 覆盖主要交易分支、边界条件、非法输入和状态警告。
- 保持当前策略实现不变，只增加测试，不修改生产逻辑。

## 2. 对现有项目的影响

本次只影响测试层：

- [main_test.go](/Users/marmotmr./GolandProjects/diff_str/main_test.go)
  - 新增 `TestDecide...` 系列测试。
  - 通过表驱动方式覆盖不同输入和仓位状态。

不涉及：

- [main.go](/Users/marmotmr./GolandProjects/diff_str/main.go) 业务逻辑修改
- UI 行为变更
- `go.mod` 依赖调整
- 外部服务、数据库或文件系统

## 3. 实现方案

### 3.1 测试组织方式

在 [main_test.go](/Users/marmotmr./GolandProjects/diff_str/main_test.go) 中新增面向 `Decide` 的测试，优先采用表驱动组织，避免每个场景写成完全重复的断言。

建议结构：

```go
func TestDecide(t *testing.T) {
    testCases := []struct {
        name    string
        input   Input
        current State
        check   func(t *testing.T, got Decision)
    }{
        ...
    }
}
```

如果某些分支断言差异较大，也可以拆成多个测试函数，例如：

- `TestDecide_InvalidInput`
- `TestDecide_EmptyPosition`
- `TestDecide_ReduceAndClearRules`
- `TestDecide_Warnings`

### 3.2 需要覆盖的规则分支

#### A. 非法输入

覆盖 `validateInput` 失败分支，确认：

- `IndexValue <= 0`
- `MA120 <= 0`
- `MA20 <= 0`

断言点：

- `Valid == false`
- `Reason` 为对应中文错误
- `Action` 保持 `hold`

#### B. 空仓分支

覆盖 `isZeroPosition(current.Position)` 之后的两条路径：

1. 满足回补条件：`IndexValue < MA120 * 0.97`
   - `Action == ActionReenterFull`
   - `State.Position == 1.0`
   - `State.HighPhase == false`
2. 不满足回补条件
   - `Action == ActionHold`
   - `Reason` 为“当前为空仓，暂未满足重新满仓条件”

#### C. 8% 减仓分支

覆盖 `biasPct >= 8.0` 且当前仓位大于 `2/3`：

- 从满仓进入 `2/3`
- `State.HighPhase` 应变为 `true`
- `Action == ActionReduceToTwo3`

同时补一个边界场景：

- `biasPct == 8.0` 时也应触发

#### D. 10% 再减仓分支

覆盖 `biasPct >= 10.0` 且当前仓位大于 `1/3`：

- 从 `2/3` 进入 `1/3`
- `Action == ActionReduceToOne3`
- `State.HighPhase == true`

同时补一个边界场景：

- `biasPct == 10.0` 时也应触发

#### E. 高位跌破 MA20 清仓分支

覆盖以下条件组合：

- `HighPhase == true`
- `IndexValue < MA20`
- `Position > 0`

断言点：

- `Action == ActionClearAll`
- `State.Position == 0`
- `State.HighPhase == false`

还需要一个对照场景，确认不满足条件时不会误清仓：

- `HighPhase == false` 且 `IndexValue < MA20`

#### F. 无动作持有分支

覆盖没有命中任何规则时的默认返回：

- `Action == ActionHold`
- `Reason == "当前未触发新的交易规则，继续持有观察"`

#### G. 状态警告分支

覆盖非法仓位但输入仍可计算的情况，例如：

- `Position = 0.5`

断言点：

- `Warnings` 包含“当前仓位不是预设状态”
- 其余逻辑仍正常执行，而不是直接报错退出

### 3.3 断言策略

测试中除了校验 `Action`，还要校验以下字段，避免只测“动作”而漏掉状态更新：

- `Valid`
- `BiasPct`
- `State.Position`
- `State.HighPhase`
- `Reason`
- `Warnings`

浮点仓位断言继续复用项目现有的 `isSamePosition`，避免因为浮点误差导致测试不稳定。

### 3.4 测试命名建议

建议至少包含以下测试名之一，便于后续快速定位：

- `TestDecide_InvalidInput`
- `TestDecide_EmptyPositionReenter`
- `TestDecide_EmptyPositionHold`
- `TestDecide_ReduceToTwoThirds`
- `TestDecide_ReduceToOneThird`
- `TestDecide_ClearAllAfterHighPhase`
- `TestDecide_HoldWhenNoRuleMatches`
- `TestDecide_WarnsOnNonCanonicalPosition`

## 4. Todo List

- [x] 梳理 `Decide` 当前所有可达分支与边界条件
- [x] 在 [main_test.go](/Users/marmotmr./GolandProjects/diff_str/main_test.go) 中新增 `Decide` 测试
- [x] 覆盖非法输入场景
- [x] 覆盖空仓回补与空仓继续等待场景
- [x] 覆盖 8% 减仓与 10% 再减仓场景
- [x] 覆盖高位跌破 MA20 清仓场景
- [x] 覆盖未命中规则时继续持有场景
- [x] 覆盖非标准仓位警告场景
- [x] 运行 `go test ./...` 验证所有测试通过
