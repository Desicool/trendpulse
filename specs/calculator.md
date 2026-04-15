# TrendPulse 计算层规格

## 设计理念

计算层（Calculator Layer）负责将原始时序信号转化为可解释的趋势预测统计数据。其核心设计理念是：

1. **可插拔策略（Pluggable Strategies）**：所有预测算法均实现统一的 `Strategy` 接口。一个策略定义一套**完整的算法**：Score 的计算方式与 Phase 的判定逻辑在同一策略内共同设计，共享特征提取逻辑。不同策略可以有完全不同的特征定义和算法，只要输出统一的 `TrendStats` 结构。
2. **统一输出合约**：无论算法内部如何计算，所有策略必须输出相同结构的 `TrendStats`，确保 API 层无需关心算法细节
3. **计算与读取分离**：调度器在后台异步计算并存储结果，API 层仅读取已计算结果，绝不触发实时计算
4. **并行执行**：多个策略可并行对同一趋势集运行，互不阻塞
5. **支持 A/B 测试**：通过配置 `active_strategy` 决定 API 层使用哪个策略的结果，其余策略结果保留在存储中供分析

---

## Strategy 接口定义与合约

```go
// internal/calculator/interfaces.go

package calculator

import (
    "context"

    "github.com/yourorg/trendpulse/internal/domain"
)

// Strategy 定义趋势预测策略的统一接口
// 所有策略实现必须满足此接口
type Strategy interface {
    // ID 返回策略的唯一标识符
    // 此 ID 用于：注册表查找、配置引用、TrendStats.StrategyID 字段
    // 格式约定：小写字母 + 数字 + 下划线，如 "momentum_v1"
    ID() string

    // Name 返回策略的人类可读名称
    // 用于日志、监控和 API 元数据展示
    Name() string

    // Calculate 同时计算 Score（爆发概率）和 Phase（当前阶段）
    // Score 和 Phase 在策略内部共享信号读取和特征提取逻辑
    // 两者的定义由策略自身决定：不同策略可以有不同的 Phase 语义
    //
    // 参数：
    //   ctx     - 上下文，支持超时与取消
    //   trend   - 趋势文档（包含元数据）
    //   reader  - 信号懒加载读取器，绑定到当前 trend，策略按需调用
    //             reader.Latest(n)            — 最近 n 条信号
    //             reader.Range(from, to)      — 时间范围内的信号
    //             reader.Aggregate(window)    — 按 window 聚合后的信号
    //
    // 返回：
    //   *domain.TrendStats - 计算结果，ID 由策略按约定格式构造："{strategy_id}:{trend_id}"
    //   error              - 计算失败时返回非 nil 错误
    Calculate(ctx context.Context, trend *domain.Trend, reader SignalReader) (*domain.TrendStats, error)
}
```

### 接口合约（所有实现必须遵守）

1. **ID 唯一性**：同一进程内不得注册两个相同 ID 的策略
2. **TrendStats.ID 格式**：必须为 `"{strategy_id}:{trend_id}"`，即 `fmt.Sprintf("%s:%s", s.ID(), trend.ID)`
3. **Score 与 Phase 共享设计**：同一策略内，Score 公式与 Phase 判定应共享特征提取逻辑。策略作者定义整套算法，不得由框架强制分离。
4. **空信号处理**：当 SignalReader 返回空结果时，不得返回 error，应返回低置信度的 TrendStats（Score=0, Confidence=0, Phase="emerging"）
5. **上下文遵守**：当 ctx 取消时应及早返回，不得继续长时间计算
6. **无副作用**：`Calculate` 不得写入任何存储，不得调用外部服务
7. **并发安全**：策略实现必须是并发安全的（goroutine-safe）

---

## SignalReader 接口

`SignalReader` 为策略提供懒加载的信号读取能力。每个 `Strategy.Calculate` 调用独享一个 `SignalReader` 实例，绑定到特定 trend_id。策略按需调用，不加载不需要的数据。

```go
// internal/calculator/interfaces.go

// AggregatedSignal 是对原始信号按时间窗口聚合后的结果
type AggregatedSignal struct {
    WindowStart          time.Time
    WindowEnd            time.Time
    AvgUsageCount        float64
    AvgUniqueCreators    float64
    AvgViews             float64
    AvgEngagement        float64
    AvgViewConcentration float64
    SampleCount          int  // 窗口内原始信号数量
}

// SignalReader 提供懒加载的信号读取能力
type SignalReader interface {
    // Latest 返回最近 n 条信号，按 Timestamp 升序（最新在末尾）
    Latest(ctx context.Context, n int) ([]*domain.Signal, error)
    // Range 返回指定时间范围内的所有信号，按 Timestamp 升序
    Range(ctx context.Context, from, to time.Time) ([]*domain.Signal, error)
    // Aggregate 按 window 时长聚合信号，返回每个窗口的统计值
    // 例：Aggregate(1*time.Hour) 将最近 signal_lookback 内的信号按每小时聚合
    Aggregate(ctx context.Context, window time.Duration) ([]*AggregatedSignal, error)
}
```

### SignalReader 实现（调度器内部）

调度器为每个 trend 创建 `repositorySignalReader`，实现 `SignalReader` 接口：

- 文件路径：`internal/scheduler/signal_reader.go`
- 绑定 `trendID`、`SignalRepository` 和 `lookback` 时间窗口
- `Aggregate` 方法在内存中对 `Range` 结果按窗口分组聚合

---

## Registry 机制

注册表（Registry）负责管理所有已注册策略的生命周期，支持按 ID 查找和列举所有策略。

```go
// internal/calculator/registry.go

package calculator

import (
    "fmt"
    "sync"
)

// Registry 管理所有已注册的策略
type Registry struct {
    mu         sync.RWMutex
    strategies map[string]Strategy
}

// NewRegistry 创建空注册表
func NewRegistry() *Registry {
    return &Registry{
        strategies: make(map[string]Strategy),
    }
}

// Register 注册一个策略
// 若已存在相同 ID 的策略，返回错误
func (r *Registry) Register(s Strategy) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.strategies[s.ID()]; exists {
        return fmt.Errorf("strategy %q already registered", s.ID())
    }
    r.strategies[s.ID()] = s
    return nil
}

// Get 按 ID 查找策略
// 若不存在，返回 nil, false
func (r *Registry) Get(id string) (Strategy, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    s, ok := r.strategies[id]
    return s, ok
}

// All 返回所有已注册策略的切片
// 顺序不保证
func (r *Registry) All() []Strategy {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]Strategy, 0, len(r.strategies))
    for _, s := range r.strategies {
        result = append(result, s)
    }
    return result
}
```

### 初始化示例（cmd/server/main.go）

```go
registry := calculator.NewRegistry()

if cfg.Strategies.SigmoidV1.Enabled {
    registry.Register(sigmoid.NewSigmoidV1(cfg.Strategies.SigmoidV1))
}
if cfg.Strategies.RegionalV1.Enabled {
    registry.Register(regional.NewRegionalV1(cfg.Strategies.RegionalV1))
}
```

---

## 统一输出合约

所有策略的 `Calculate` 方法必须返回完整填充的 `TrendStats`。下表说明每个字段的填充要求：

| 字段 | 填充要求 | 说明 |
|------|----------|------|
| `ID` | **必须** | 格式：`"{strategy_id}:{trend_id}"` |
| `TrendID` | **必须** | 等于 `trend.ID` |
| `StrategyID` | **必须** | 等于 `s.ID()` |
| `CalculatedAt` | **必须** | 调用 `time.Now().UTC()` |
| `Score` | **必须** | 爆发概率评分，范围 [0, 100]，表示未来 48h 进入爆发期或高峰期的可能性 |
| `Phase` | **必须** | 当前阶段枚举：`"emerging"`\|`"exploding"`\|`"peaking"`\|`"declining"` |
| `Confidence` | **必须** | 范围 [0, 1]，信号数量越多应越高 |
| `LatestUsageCount` | **必须** | 取最新信号的 UsageCount；无信号时为 0 |
| `LatestUniqueCreators` | **必须** | 取最新信号的 UniqueCreators；无信号时为 0 |
| `LatestAvgViews` | **必须** | 取最新信号的 AvgViews；无信号时为 0.0 |
| `LatestAvgEngagement` | **必须** | 取最新信号的 AvgEngagement；无信号时为 0.0 |
| `LatestViewConcentration` | **必须** | 取最新信号的 ViewConcentration；无信号时为 0.0 |
| `Metadata` | 推荐 | 存储策略内部计算的中间值，便于调试 |

---

## A/B 测试机制

### 配置

```yaml
scheduler:
  active_strategy: "sigmoid_v1"  # API 层使用的策略 ID
```

### 运行时行为

1. 调度器并行运行**所有已启用**的策略（不仅仅是 active_strategy）
2. 每个策略的计算结果均存储至 StatsRepository，复合键为 `"{strategy_id}:{trend_id}"`
3. API 层（`GET /trends/rising`，`GET /trends/{id}`）使用 `active_strategy` 作为 strategyID 查询 StatsRepository
4. 切换 active_strategy 只需修改配置并重启服务，历史数据保留

### A/B 测试工作流

```
┌─────────────────────────────────────────────────────────┐
│                      调度器运行                          │
│                                                         │
│  goroutine A: sigmoid_v1.Calculate() → StatsRepo       │
│  goroutine B: regional_v1.Calculate() → StatsRepo      │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                  StatsRepository 存储                    │
│                                                         │
│  "sigmoid_v1:trend-001"  → TrendStats{Score: 78.5}     │
│  "regional_v1:trend-001" → TrendStats{Score: 72.1}     │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│             API 层（active_strategy: sigmoid_v1）        │
│                                                         │
│  GET /trends/rising → 查询 "sigmoid_v1" 的结果          │
└─────────────────────────────────────────────────────────┘
```

---

## 调度器设计

**文件路径**：`internal/scheduler/scheduler.go`

### 核心职责

1. 定期触发所有已启用策略对所有趋势进行计算
2. 并行运行多个策略，提高吞吐量
3. 批量写入计算结果到 StatsRepository

### 调度器结构

```go
// internal/scheduler/scheduler.go

type Scheduler struct {
    trendRepo    repository.TrendRepository
    signalRepo   repository.SignalRepository
    statsRepo    repository.StatsRepository
    registry     *calculator.Registry
    interval     time.Duration       // 来自 config.scheduler.interval
    lookback     time.Duration       // 来自 config.scheduler.signal_lookback
    logger       *slog.Logger
}
```

### 核心调度循环

```go
func (s *Scheduler) Run(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    // 启动后立即执行一次，无需等待第一个 tick
    if err := s.runOnce(ctx); err != nil {
        s.logger.Error("scheduler run failed", "error", err)
    }

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := s.runOnce(ctx); err != nil {
                s.logger.Error("scheduler run failed", "error", err)
                // 不因单次失败退出，继续下一轮
            }
        }
    }
}
```

### 单次计算流程

```go
func (s *Scheduler) runOnce(ctx context.Context) error {
    // 1. 分页获取所有趋势（避免一次性加载所有数据到内存）
    var allTrends []*domain.Trend
    offset := 0
    const batchSize = 100
    for {
        trends, _, err := s.trendRepo.List(ctx, offset, batchSize)
        if err != nil { return err }
        allTrends = append(allTrends, trends...)
        if len(trends) < batchSize { break }
        offset += batchSize
    }

    strategies := s.registry.All()

    // 2. 对每个策略启动 goroutine，并行计算
    var wg sync.WaitGroup
    for _, strategy := range strategies {
        wg.Add(1)
        go func(strat calculator.Strategy) {
            defer wg.Done()
            s.runStrategy(ctx, strat, allTrends)
        }(strategy)
    }
    wg.Wait()
    return nil
}

func (s *Scheduler) runStrategy(ctx context.Context, strat calculator.Strategy, trends []*domain.Trend) {
    now := time.Now().UTC()
    from := now.Add(-s.lookback)

    var statsToWrite []*domain.TrendStats
    for _, trend := range trends {
        signals, err := s.signalRepo.ListByTrendID(ctx, trend.ID, from, now)
        if err != nil {
            s.logger.Error("failed to fetch signals", "trend_id", trend.ID, "error", err)
            continue
        }

        stats, err := strat.Calculate(ctx, trend, signals)
        if err != nil {
            s.logger.Error("strategy calculate failed", "strategy", strat.ID(), "trend_id", trend.ID, "error", err)
            continue
        }

        statsToWrite = append(statsToWrite, stats)
    }

    if err := s.statsRepo.BatchUpsert(ctx, statsToWrite); err != nil {
        s.logger.Error("batch upsert failed", "strategy", strat.ID(), "error", err)
    }
}
```

### 配置参数

| 配置项 | 说明 | 典型值 |
|--------|------|--------|
| `scheduler.interval` | 调度触发间隔 | "5m" |
| `scheduler.signal_lookback` | 每次计算读取的信号时间窗口 | "48h" |
| `scheduler.active_strategy` | API 使用的策略 ID | "sigmoid_v1" |

---

## Score 公式（爆发概率评分）

> 以下描述 `sigmoid_v1` 参考策略的实现。不同策略可以采用完全不同的计算方式。

Score 回答"该趋势在未来 48 小时内是否会进入爆发期或高峰期"，范围 [0, 100]。

### 计算维度

| 维度 | 变量名 | 计算方式 |
|------|--------|---------|
| 播放量加速度 | `view_accel` | 二阶差分：`(v[t] - 2·v[t-1] + v[t-2]) / v[t-2]`，使用最近 3 个聚合窗口 |
| Post 增长率 | `post_growth` | 短窗口一阶增长率：`(u[t] - u[t-k]) / u[t-k]`，k = lookback_short |
| 创作者增长率 | `creator_growth` | 短窗口一阶增长率：`(c[t] - c[t-k]) / c[t-k]`，k = lookback_short |
| 互动激增率 | `engagement_surge` | `avg_engagement[t] / moving_avg_engagement`，移动均值窗口 = lookback_short |
| 播放集中度 | `view_conc` | 直接使用最新信号的 `ViewConcentration` 字段，范围 [0, 1] |

### 公式

```
raw = α·view_accel + β·post_growth + γ·creator_growth + δ·engagement_surge + ε·view_conc - bias

Score = 100 × sigmoid(raw)    其中 sigmoid(x) = 1 / (1 + e^(−x))
```

所有系数通过配置文件调整，默认值：

```yaml
calculator:
  default_strategy:
    weights:
      view_acceleration:   2.5   # α — 播放量加速度权重（最重要信号）
      post_growth_rate:    1.5   # β — post 增长率权重
      creator_growth_rate: 1.5   # γ — 创作者增长率权重
      engagement_surge:    2.0   # δ — 互动激增率权重
      view_concentration:  1.0   # ε — 播放集中度权重
    bias: 3.0                    # sigmoid 中心偏移（控制基准分）
    lookback_short: "6h"         # 短期窗口（post/creator 增长率计算）
    lookback_accel: "3h"         # 加速度计算窗口（需 3 个数据点）
```

---

## Phase 检测逻辑

> 以下描述 `sigmoid_v1` 参考策略的 Phase 判定逻辑。不同策略可自定义 Phase 的判定方式。

Phase 回答"该趋势此刻处于哪个阶段"，基于当前增长率模式判定。

### 判定输入

```go
type PhaseSignals struct {
    PostGrowthRate      float64  // post 数量近期增长率（短窗口一阶差分）
    ViewAcceleration    float64  // 播放量加速度（二阶差分）
    EngagementGrowth    float64  // 互动率增长（最新值/移动均值）
    ViewConcentration   float64  // 播放集中度（最新信号值）
    AllMetricsNegative  bool     // 所有指标均为负增长
}
```

### 判定规则（按优先级）

```go
func DeterminePhase(s PhaseSignals, cfg PhaseConfig) string {
    // 1. 衰退期：各维度均为负增长
    if s.AllMetricsNegative {
        return "declining"
    }
    // 2. 爆发期：播放量加速度高 AND 互动激增
    if s.ViewAcceleration > cfg.ExplodingAccelThreshold &&
       s.EngagementGrowth > cfg.ExplodingEngagementThreshold {
        return "exploding"
    }
    // 3. 高峰期：post 增长率趋近于零（正负均可，但绝对值小）
    if s.PostGrowthRate < cfg.PeakingGrowthRateMax &&
       s.PostGrowthRate > cfg.PeakingGrowthRateMin {
        return "peaking"
    }
    // 4. 萌芽期：其余情况
    return "emerging"
}
```

### Phase 阈值配置

```yaml
calculator:
  default_strategy:
    phase_thresholds:
      exploding_accel_threshold:       0.5   # view_acceleration > 0.5 判定为爆发期
      exploding_engagement_threshold:  0.3   # engagement_growth > 0.3 判定为爆发期
      peaking_growth_rate_max:         0.05  # post_growth < 5% 才可能是高峰期
      peaking_growth_rate_min:        -0.02  # post_growth > -2% 才是高峰期（非衰退）
```

> **注意**：`DeterminePhase` 是定义在 `internal/calculator/` 包级别的**可选辅助函数**，`sigmoid_v1` 策略使用它，但其他策略可以完全自定义 Phase 判定逻辑，无需调用此函数。

---

## 内置策略

当前版本暂无预设策略实现。添加新策略请参考"扩展指南"章节。策略应使用 `SignalReader` 接口按需读取信号，在同一 `Calculate` 调用中同时计算 Score（爆发概率）和 Phase（当前阶段）。

---

## 信号维度要求

**至少使用 2 个信号维度 (Required)**：算法必须综合至少 2 个信号维度进行评分，不得仅依赖单一指标。内置策略使用以下维度：

1. `usage_count` 增长率 — 衡量内容发布量的增速
2. `unique_creators` 增长率 — 衡量参与创作者的多样性增长（权重最高，因为有机增长信号）
3. `avg_views` 增长率 — 衡量内容消费端的反应
4. `avg_engagement` — 衡量互动激增程度
5. `view_concentration` — 衡量播放量是否向少数视频集中

---

## Sparse Field Handling

某些数据来源不提供 `avg_engagement` 和 `view_concentration` 字段，这两个字段在摄取时可选，缺失时默认为 `0.0`。所有策略实现必须按以下规则处理稀疏信号：

### 0.0 = 数据不可用

对于可选字段 `avg_engagement` 和 `view_concentration`，`0.0` 表示数据不可用（非真实零值）。

### 维度级别过滤

计算 `engagement_surge` 或 `view_conc` 维度时：

1. 过滤掉该字段为 `0.0` 的信号
2. 若过滤后有效数据点 < 2，跳过该维度（将该维度权重视为 0 参与此次评分计算）

核心字段 `usage_count`、`unique_creators`、`avg_views` 始终为必填字段，不做过滤。

### 置信度公式

```
confidence = (valid_signal_count / expected_signal_count) × dimension_coverage
```

- `valid_signal_count`：当前 lookback 窗口内实际有效信号数量
- `expected_signal_count`：`lookback / signal_interval`（来自配置）
- `dimension_coverage`：5 个维度中拥有 ≥ 2 个非零数据点的维度比例

### 最小数据保护

若 `valid_signal_count < 2`，策略应提前返回：

```go
return &domain.TrendStats{
    ID:           fmt.Sprintf("%s:%s", s.ID(), trend.ID),
    TrendID:      trend.ID,
    StrategyID:   s.ID(),
    CalculatedAt: time.Now().UTC(),
    Score:        0,
    Confidence:   0,
    Phase:        "emerging",
}, nil
```

此规则是对空信号合约（见"接口合约"第4条）的扩展，适用于信号数量极少的情形。

---

## 扩展指南：添加新策略

以下步骤说明如何添加名为 `regional_v1` 的新策略：

有意义的新策略应定义不同的特征提取方式（如引入新的信号维度）、不同的 Score 公式（如基于机器学习模型），或适应不同地区/内容类型的 Phase 阈值——而非仅修改权重系数（权重修改通过配置文件即可实现）。

### 步骤 1：创建策略目录和文件

```
internal/calculator/regional/
└── regional_v1.go
```

### 步骤 2：实现 Strategy 接口

```go
package regional

import (
    "context"
    "fmt"
    "time"

    "github.com/yourorg/trendpulse/internal/calculator"
    "github.com/yourorg/trendpulse/internal/domain"
)

type Config struct {
    // 策略特定配置字段
    Region string // 适用地区
}

type RegionalV1 struct {
    cfg Config
}

func NewRegionalV1(cfg Config) *RegionalV1 {
    return &RegionalV1{cfg: cfg}
}

func (r *RegionalV1) ID() string   { return "regional_v1" }
func (r *RegionalV1) Name() string { return "Regional Strategy V1" }

func (r *RegionalV1) Calculate(ctx context.Context, trend *domain.Trend, reader calculator.SignalReader) (*domain.TrendStats, error) {
    // 处理空信号
    signals, err := reader.Latest(ctx, 1)
    if err != nil {
        return nil, err
    }
    if len(signals) == 0 {
        return &domain.TrendStats{
            ID:           fmt.Sprintf("%s:%s", r.ID(), trend.ID),
            TrendID:      trend.ID,
            StrategyID:   r.ID(),
            CalculatedAt: time.Now().UTC(),
            Score:        0,
            Confidence:   0,
            Phase:        "emerging",
        }, nil
    }

    // 实现算法逻辑...
    score := 0.0      // 计算 score
    confidence := 0.0 // 计算 confidence

    latest := signals[len(signals)-1]

    return &domain.TrendStats{
        ID:                      fmt.Sprintf("%s:%s", r.ID(), trend.ID),
        TrendID:                 trend.ID,
        StrategyID:              r.ID(),
        CalculatedAt:            time.Now().UTC(),
        Score:                   score,
        Confidence:              confidence,
        Phase:                   calculator.DeterminePhase(phaseSignals, cfg.PhaseThresholds),
        LatestUsageCount:        latest.UsageCount,
        LatestUniqueCreators:    latest.UniqueCreators,
        LatestAvgViews:          latest.AvgViews,
        LatestAvgEngagement:     latest.AvgEngagement,
        LatestViewConcentration: latest.ViewConcentration,
        Metadata: map[string]interface{}{
            "signal_count": len(signals),
        },
    }, nil
}
```

### 步骤 3：添加配置结构

在 `internal/config/config.go` 中的 `StrategiesConfig` 添加新字段：

```go
type StrategiesConfig struct {
    SigmoidV1  SigmoidV1Config  `yaml:"sigmoid_v1"`
    RegionalV1 RegionalV1Config `yaml:"regional_v1"`  // 新增
}
```

### 步骤 4：在 main.go 中注册

```go
if cfg.Strategies.RegionalV1.Enabled {
    registry.Register(regional.NewRegionalV1(cfg.Strategies.RegionalV1))
}
```

### 步骤 5：编写测试

在 `internal/calculator/regional/regional_v1_test.go` 中编写完整的单元测试（见 testing.md 的测试规范）。

### 步骤 6：更新配置文件

在 `configs/config.yaml` 中添加新策略的配置块。

---

## 并发模型说明

调度器在运行多策略时使用 `sync.WaitGroup` 协调 goroutine：

```
Scheduler.runOnce()
  │
  ├── goroutine: sigmoid_v1.runStrategy(trends)
  │     ├── 串行对每个 trend 查询信号、计算
  │     └── BatchUpsert 所有结果
  │
  └── goroutine: regional_v1.runStrategy(trends)
        ├── 串行对每个 trend 查询信号、计算
        └── BatchUpsert 所有结果

两个 goroutine 并发执行，wg.Wait() 等待全部完成
```

**注意**：多个 goroutine 同时读取 TrendRepository 和 SignalRepository 是安全的（只读操作）。BatchUpsert 写入不同的 key（复合键含 strategy_id），因此也不会冲突。
