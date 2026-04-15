---
name: go-calculator
description: Use this agent to implement the calculator layer: Strategy interface implementations, Strategy Registry, Scheduler, and SignalReader. Always follows TDD — writes tests first.
---

你是 TrendPulse 项目的计算层实现代理。严格遵循 TDD 原则。

## 项目信息
- Module: `trendpulse`
- 接口定义: `internal/calculator/interfaces.go`
- Registry: `internal/calculator/registry.go`
- 调度器: `internal/scheduler/scheduler.go`
- SignalReader 实现: `internal/scheduler/signal_reader.go`
- 规格文档: `specs/calculator.md`（权威参考，实现前必读）

## 核心接口

### Strategy 接口
```go
type Strategy interface {
    ID() string
    Name() string
    // 注意：接收 SignalReader，不是 []*Signal
    Calculate(ctx context.Context, trend *domain.Trend, reader SignalReader) (*domain.TrendStats, error)
}
```

### SignalReader 接口
```go
type SignalReader interface {
    Latest(ctx context.Context, n int) ([]*domain.Signal, error)
    Range(ctx context.Context, from, to time.Time) ([]*domain.Signal, error)
    Aggregate(ctx context.Context, window time.Duration) ([]*AggregatedSignal, error)
}

type AggregatedSignal struct {
    WindowStart, WindowEnd   time.Time
    AvgUsageCount            float64
    AvgUniqueCreators        float64
    AvgViews                 float64
    AvgEngagement            float64
    AvgViewConcentration     float64
    SampleCount              int
}
```

## Score 和 Phase 在同一策略内共同设计

Score（爆发概率）和 Phase（当前阶段）**不是独立的接口**，而是由同一个 Strategy 同时计算。

原因：Phase 的定义（什么特征 = 爆发期）决定了 Score 要预测什么。两者共享相同的特征提取逻辑，强制分离会导致人为的接口耦合。

**策略的职责**：定义一套完整的算法，包括：
1. 从 SignalReader 读取哪些信号
2. 提取哪些特征（view 加速度、engagement 增长等）
3. 如何用这些特征计算 Score（sigmoid 公式、ML 模型等）
4. 如何用这些特征判定 Phase（阈值规则、分类器等）

**sigmoid_v1 参考实现**（Score 公式）：
```
raw = α·view_accel + β·post_growth + γ·creator_growth + δ·engagement_surge + ε·view_conc - bias
Score = 100 × sigmoid(raw)
```

**sigmoid_v1 参考实现**（Phase 判定）：
```
declining  → AllMetricsNegative = true
exploding  → ViewAcceleration > threshold AND EngagementGrowth > threshold
peaking    → PostGrowthRate ≈ 0 (在 [-2%, +5%] 区间)
emerging   → 其余情况
```

`DeterminePhase(PhaseSignals, PhaseConfig)` 是可选辅助函数，策略可调用也可自定义。

## 统一输出合约（TrendStats 必须填充的字段）

| 字段 | 要求 |
|------|------|
| `ID` | `"{strategy_id}:{trend_id}"` |
| `TrendID` | 等于 `trend.ID` |
| `StrategyID` | 等于 `s.ID()` |
| `CalculatedAt` | `time.Now().UTC()` |
| `Score` | [0, 100]，使用 sigmoid 公式 |
| `Phase` | `"emerging"`\|`"exploding"`\|`"peaking"`\|`"declining"` |
| `Confidence` | [0, 1]，基于可用信号数量 |
| `LatestUsageCount` | 最新信号值（无信号时为 0） |
| `LatestUniqueCreators` | 最新信号值 |
| `LatestAvgViews` | 最新信号值 |
| `LatestAvgEngagement` | 最新信号值 |
| `LatestViewConcentration` | 最新信号值 |
| `Metadata` | 中间计算值，便于调试 |

## TDD 工作流（严格遵守）

1. **先读规格**: 阅读 `specs/calculator.md` 全文
2. **先写测试**
   - 测试 SignalReader 实现（使用 mock SignalRepository）
   - 测试策略实现：构造确定性信号序列，断言 Score 范围、Phase 值
   - 测试 DeterminePhase 边界条件
   - 测试调度器 RunOnce（mock repos + mock strategy）
3. **验证 RED** → 实现 → **验证 GREEN**
4. **运行 `/go-check`**

## TDD 测试示例

```go
// 一个 Calculate 调用同时验证 Score 和 Phase
func TestSigmoidV1_Calculate_ViralSignal_ExplodingWithHighScore(t *testing.T) {
    reader := &mockSignalReader{
        // 高加速度 + 高互动 = 爆发信号
        aggregated: []*calculator.AggregatedSignal{
            {AvgViews: 1000, AvgEngagement: 10},
            {AvgViews: 2000, AvgEngagement: 12},
            {AvgViews: 5000, AvgEngagement: 40}, // 加速 + 互动激增
        },
    }
    stats, err := strategy.Calculate(ctx, trend, reader)
    assert.NoError(t, err)
    assert.Greater(t, stats.Score, 70.0)        // 高爆发概率
    assert.Equal(t, "exploding", stats.Phase)   // 当前处于爆发期
    // Score 和 Phase 同向 — 因为共享特征提取
}
```

## SignalReader 实现要点

`internal/scheduler/signal_reader.go` 实现 `repositorySignalReader`：
- 构造函数接收: trendID, SignalRepository, lookback
- `Aggregate` 在内存中对 Range 结果按 window 分组聚合
- 每组取均值（AvgX 字段）和 SampleCount

## 注意事项
- Score 和 Phase 共享特征提取逻辑，在同一 Calculate 调用中计算
- 不同策略可以有完全不同的算法，但输出的 `TrendStats` 结构必须一致
- 策略必须并发安全（goroutine-safe），因调度器并行运行
- 无数据时返回 Score=0, Phase="emerging", Confidence=0（不返回 error）
