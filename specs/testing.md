# TrendPulse 测试策略规格（TDD）

## TDD 原则

### RED → GREEN → REFACTOR 工作流

TrendPulse 项目严格遵循 TDD（测试驱动开发）工作流：

```
┌─────────────────────────────────────────────────────────┐
│                    TDD 循环                              │
│                                                         │
│   RED        → 先写一个失败的测试                        │
│   GREEN      → 写最少的代码使测试通过                    │
│   REFACTOR   → 在测试保护下重构代码                     │
│                                                         │
│   重复以上步骤，直到功能完整                             │
└─────────────────────────────────────────────────────────┘
```

### 为何在 TrendPulse 中采用 TDD

1. **接口先行**：Repository 和 Strategy 接口在实现前已定义，TDD 确保接口设计合理
2. **算法正确性**：预测算法（momentum, velocity）需要确定性验证，TDD 迫使开发者明确预期结果
3. **防止回归**：随着策略增加，测试套件保护现有功能不被破坏
4. **文档即测试**：测试用例本身就是策略行为的活文档

### 开发流程示例

以实现 `momentum_v1` 为例：

```
1. 写测试：TestMomentumV1_StableSignals_LowMomentum（RED）
2. 创建 momentum_v1.go 的空壳实现（仍 RED）
3. 实现 EWMA 计算逻辑（GREEN）
4. 重构：提取 normalizeScore 辅助函数（REFACTOR，测试仍 GREEN）
5. 写下一个测试：TestMomentumV1_RisingSignals_HighScore（RED）
6. 扩展实现...
```

---

## 测试分层

TrendPulse 的测试分为三层，各层关注不同范围：

| 层级 | 关注范围 | 速度 | 依赖 |
|------|----------|------|------|
| 单元测试（Unit） | 单个函数/方法 | 极快（毫秒级） | 无外部依赖 |
| 集成测试（Integration） | 跨组件协作 | 快（秒级） | BadgerHold InMemory |
| 端到端测试（E2E） | 完整 HTTP 流 | 中（秒级） | httptest.NewServer + BadgerHold InMemory |

**覆盖率目标**：

| 层级 | 目标覆盖率 |
|------|-----------|
| 存储层（repository/badger） | ≥ 80% |
| 计算层（calculator） | ≥ 80% |
| API 层（api/handler） | ≥ 80% |
| 调度器（scheduler） | ≥ 70% |

---

## 测试文件组织

测试文件与被测代码**同目录存放**（co-located），使用 `_test.go` 后缀：

```
internal/
├── repository/
│   └── badger/
│       ├── trend_repo.go
│       ├── trend_repo_test.go       # TrendRepository 的存储层测试
│       ├── signal_repo.go
│       ├── signal_repo_test.go
│       ├── stats_repo.go
│       ├── stats_repo_test.go
│       └── category_index_test.go
├── calculator/
│   ├── registry.go
│   ├── registry_test.go
│   ├── phase_test.go                # DeterminePhase 单元测试
│   ├── momentum/
│   │   ├── momentum_v1.go
│   │   └── momentum_v1_test.go
│   └── velocity/
│       ├── velocity_v1.go
│       └── velocity_v1_test.go
├── scheduler/
│   ├── scheduler.go
│   └── scheduler_test.go
└── api/
    └── handler/
        ├── trend_handler.go
        ├── trend_handler_test.go
        ├── ingest_handler.go
        └── ingest_handler_test.go

internal/testutil/               # 共享测试工具
    ├── mock_trend_repo.go       # TrendRepository 的手写 mock
    ├── mock_signal_repo.go      # SignalRepository 的手写 mock
    ├── mock_stats_repo.go       # StatsRepository 的手写 mock
    ├── mock_category_index.go   # CategoryIndex 的手写 mock
    ├── mock_strategy.go         # Strategy 的手写 mock
    ├── fixtures.go              # 可复用的测试数据构造函数
    └── badger_helper.go         # InMemory BadgerDB 初始化辅助函数
```

---

## 测试命名约定

所有测试函数遵循以下命名格式：

```
Test{函数名}_{场景描述}_{预期行为}
```

**示例**：

| 测试函数名 | 说明 |
|-----------|------|
| `TestMomentumV1_EmptySignals_ReturnsZeroScore` | 空信号时返回零分 |
| `TestMomentumV1_RisingSignals_HighMomentum` | 上升信号序列时动量为正 |
| `TestMomentumV1_DecliningSignals_NegativeMomentum` | 下降信号序列时动量为负 |
| `TestTrendRepo_Insert_Success` | 插入趋势成功 |
| `TestTrendRepo_Insert_DuplicateID_Error` | 插入重复 ID 返回错误 |
| `TestTrendRepo_GetByID_NotFound_Error` | 查询不存在的 ID 返回错误 |
| `TestTrendHandler_GetRising_ReturnsTopK` | 返回前 K 个上升趋势 |
| `TestTrendHandler_GetRising_StatsNotAvailable_EmptyList` | 无统计数据时返回空列表 |
| `TestIngestHandler_IngestTrend_DuplicateID_Returns409` | 重复 ID 返回 409 |

**表格驱动测试（Table-driven tests）**：

当同一逻辑需要测试多个输入场景时，使用表格驱动风格：

```go
func TestDeterminePhase(t *testing.T) {
    tests := []struct {
        name      string
        score     float64
        momentum  float64
        wantPhase string
    }{
        {"high_score_low_momentum", 80.0, 2.0, "peaking"},
        {"high_score_high_momentum", 80.0, 15.0, "growing"},
        {"negative_momentum", 60.0, -5.0, "declining"},
        {"low_score_low_momentum", 20.0, 3.0, "emerging"},
        {"mid_score", 50.0, 5.0, "growing"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := DeterminePhase(tt.score, tt.momentum)
            if got != tt.wantPhase {
                t.Errorf("DeterminePhase(%v, %v) = %q, want %q",
                    tt.score, tt.momentum, got, tt.wantPhase)
            }
        })
    }
}
```

---

## 各层测试方法

### 存储层测试

**使用 InMemory BadgerHold**，无需磁盘 I/O，测试快速且可重复：

```go
// internal/testutil/badger_helper.go

// NewTestDB 创建一个 InMemory BadgerHold 实例，用于测试
// 调用方负责在测试结束后调用 db.Close()
func NewTestDB(t *testing.T) *badgerhold.Store {
    t.Helper()
    opts := badgerhold.DefaultOptions
    opts.InMemory = true
    // InMemory 模式下 Dir 可为临时目录（某些版本要求）
    opts.Dir = t.TempDir()
    opts.ValueDir = opts.Dir

    store, err := badgerhold.Open(opts)
    if err != nil {
        t.Fatalf("failed to open in-memory badgerhold: %v", err)
    }
    t.Cleanup(func() { store.Close() })
    return store
}
```

**TrendRepository 测试示例（表格驱动）**：

```go
// internal/repository/badger/trend_repo_test.go

func TestTrendRepo_Insert(t *testing.T) {
    tests := []struct {
        name    string
        trend   *domain.Trend
        wantErr bool
    }{
        {
            name:    "success",
            trend:   fixtures.NewTrend("trend-001"),
            wantErr: false,
        },
        {
            name:    "duplicate_id_returns_error",
            trend:   fixtures.NewTrend("trend-001"), // 同 ID 插入两次
            wantErr: true,
        },
    }

    db := testutil.NewTestDB(t)
    repo := NewTrendRepository(db)
    ctx := context.Background()

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := repo.Insert(ctx, tt.trend)
            if (err != nil) != tt.wantErr {
                t.Errorf("Insert() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

**SignalRepository 测试重点**：

- `ListByTrendID`：验证时间范围过滤正确（边界值：from 包含，to 包含/不包含）
- `GetLatestByTrendID`：验证返回 Timestamp 最大的记录
- `ListByTimeRange`：验证跨趋势的时间范围查询

**StatsRepository 测试重点**：

- `Upsert`：验证幂等性（相同 ID 第二次 Upsert 覆盖而非报错）
- `BatchUpsert`：验证批量写入所有记录
- `ListRising`：验证按 Score 降序排列，limit 有效

**CategoryIndex 测试重点**：

- `SetCategories`：验证为每个 category 创建了对应的 CategoryMapping 记录
- `GetTrendIDsByCategory`：验证按类别正确返回 trendID 列表
- 多趋势共享类别：类别 "music" 有 3 个趋势，查询应返回全部 3 个

---

### 计算层测试

计算层测试的关键是使用**确定性信号序列**，精确断言计算结果：

**构造确定性信号序列**：

```go
// internal/testutil/fixtures.go

// NewSignalSequence 构造指定增长模式的信号序列
// base: 基础值，growthRate: 每步增长倍数（1.0=无增长，1.1=10%增长）
func NewSignalSequence(trendID string, count int, base int64, growthRate float64) []*domain.Signal {
    signals := make([]*domain.Signal, count)
    now := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
    for i := 0; i < count; i++ {
        multiplier := math.Pow(growthRate, float64(i))
        signals[i] = &domain.Signal{
            ID:             fmt.Sprintf("signal-%s-%d", trendID, i),
            TrendID:        trendID,
            Timestamp:      now.Add(time.Duration(i) * time.Hour),
            UsageCount:     int64(float64(base) * multiplier),
            UniqueCreators: int64(float64(base/10) * multiplier),
            AvgViews:       float64(base) * multiplier * 3.5,
        }
    }
    return signals
}
```

**momentum_v1 测试示例**：

```go
// internal/calculator/momentum/momentum_v1_test.go

func TestMomentumV1_RisingSignals_PositiveMomentumAndGrowingPhase(t *testing.T) {
    cfg := Config{
        DecayFactor: 0.9,
        ScoreWeights: ScoreWeights{UsageCount: 0.4, UniqueCreators: 0.3, AvgViews: 0.3},
    }
    strategy := NewMomentumV1(cfg)
    trend := fixtures.NewTrend("trend-001")

    // 构造 20 步 20% 增长的信号序列
    signals := fixtures.NewSignalSequence("trend-001", 20, 1000, 1.2)

    stats, err := strategy.Calculate(context.Background(), trend, signals)
    if err != nil {
        t.Fatalf("Calculate() error = %v", err)
    }

    // 断言核心字段
    if stats.ID != "momentum_v1:trend-001" {
        t.Errorf("ID = %q, want %q", stats.ID, "momentum_v1:trend-001")
    }
    if stats.Momentum <= 0 {
        t.Errorf("Momentum = %v, want > 0 for rising signals", stats.Momentum)
    }
    if stats.Score <= 50 {
        t.Errorf("Score = %v, want > 50 for strongly rising signals", stats.Score)
    }
    if stats.Phase != "growing" && stats.Phase != "peaking" {
        t.Errorf("Phase = %q, want growing or peaking", stats.Phase)
    }
    if stats.Confidence <= 0.5 {
        t.Errorf("Confidence = %v, want > 0.5 for 20 signals", stats.Confidence)
    }
    // 验证最新指标正确赋值
    latestSignal := signals[len(signals)-1]
    if stats.LatestUsageCount != latestSignal.UsageCount {
        t.Errorf("LatestUsageCount = %d, want %d", stats.LatestUsageCount, latestSignal.UsageCount)
    }
}

func TestMomentumV1_EmptySignals_ZeroScoreAndConfidence(t *testing.T) {
    strategy := NewMomentumV1(defaultConfig())
    trend := fixtures.NewTrend("trend-001")

    stats, err := strategy.Calculate(context.Background(), trend, nil)

    if err != nil {
        t.Fatalf("Calculate() with empty signals should not error, got: %v", err)
    }
    if stats.Score != 0 {
        t.Errorf("Score = %v, want 0 for empty signals", stats.Score)
    }
    if stats.Confidence != 0 {
        t.Errorf("Confidence = %v, want 0 for empty signals", stats.Confidence)
    }
    if stats.Phase != "emerging" {
        t.Errorf("Phase = %q, want emerging for empty signals", stats.Phase)
    }
}
```

**velocity_v1 测试重点**：

- 确定性序列（等差增长）验证 velocity 计算值精确匹配
- window_size 边界：信号数量 < window_size 时，Confidence 应 < 1
- 信号数量 = 1 时：无法计算差分，应返回零动量

---

### API 层测试

使用 `httptest.NewServer` 进行黑盒 HTTP 测试，不依赖真实数据库：

```go
// internal/api/handler/trend_handler_test.go

func setupTestServer(t *testing.T, trendRepo repository.TrendRepository, statsRepo repository.StatsRepository) *httptest.Server {
    t.Helper()
    // 使用 mock 或 in-memory 真实实现
    router := api.NewRouter(trendRepo, statsRepo, /* 其他依赖 */)
    return httptest.NewServer(router)
}
```

**GET /trends 测试重点**：

```go
func TestTrendHandler_List_DefaultPagination(t *testing.T) {
    // 准备：写入 25 条趋势
    // 请求：GET /trends（不带参数）
    // 断言：返回 20 条（默认 limit），meta.total = 25
}

func TestTrendHandler_List_InvalidOffset_Returns400(t *testing.T) {
    // 请求：GET /trends?offset=-1
    // 断言：400, error.code = "INVALID_PAGINATION"
}

func TestTrendHandler_List_LimitExceedsMax_Returns400(t *testing.T) {
    // 请求：GET /trends?limit=101
    // 断言：400, error.code = "INVALID_PAGINATION"
}
```

**GET /trends/{id} 测试重点**：

```go
func TestTrendHandler_Get_WithStats_ReturnsStatsEmbedded(t *testing.T) {
    // 准备：写入趋势 + 对应 TrendStats
    // 请求：GET /trends/trend-001
    // 断言：返回 trend + stats 不为 null
}

func TestTrendHandler_Get_WithoutStats_ReturnsNullStats(t *testing.T) {
    // 准备：写入趋势，不写入 TrendStats
    // 请求：GET /trends/trend-001
    // 断言：200, data.stats = null
}

func TestTrendHandler_Get_NotFound_Returns404(t *testing.T) {
    // 请求：GET /trends/nonexistent
    // 断言：404, error.code = "TREND_NOT_FOUND"
}
```

**GET /trends/rising 测试重点**：

```go
func TestTrendHandler_Rising_ReturnsSortedByScore(t *testing.T) {
    // 准备：写入 5 个趋势及其 TrendStats（不同 Score）
    // 请求：GET /trends/rising?limit=3
    // 断言：返回 3 条，按 Score 降序排列
}

func TestTrendHandler_Rising_NoStats_ReturnsEmptyList(t *testing.T) {
    // 准备：写入趋势，不写入 TrendStats
    // 请求：GET /trends/rising
    // 断言：200, data = []
}
```

**POST /ingest/trends 测试重点**：

```go
func TestIngestHandler_IngestTrend_ValidBody_Returns201(t *testing.T) {
    // 请求：POST /ingest/trends with valid JSON
    // 断言：201, 返回创建的趋势，包含 created_at
}

func TestIngestHandler_IngestTrend_DuplicateID_Returns409(t *testing.T) {
    // 第一次插入成功，第二次插入相同 ID
    // 断言：409, error.code = "TREND_ALREADY_EXISTS"
}

func TestIngestHandler_IngestTrend_MissingName_Returns422(t *testing.T) {
    // 请求体缺少 name 字段
    // 断言：422, error.code = "MISSING_REQUIRED_FIELD"
}

func TestIngestHandler_IngestTrend_InvalidJSON_Returns400(t *testing.T) {
    // 发送非法 JSON
    // 断言：400, error.code = "INVALID_REQUEST"
}
```

**POST /ingest/signals 测试重点**：

```go
func TestIngestHandler_IngestSignal_ValidBody_Returns201(t *testing.T) {
    // 先插入趋势，再插入信号
    // 断言：201, 返回创建的信号
}

func TestIngestHandler_IngestSignal_TrendNotFound_Returns404(t *testing.T) {
    // trend_id 不存在
    // 断言：404, error.code = "TREND_NOT_FOUND"
}

func TestIngestHandler_IngestSignal_InvalidTimestamp_Returns422(t *testing.T) {
    // timestamp 格式错误（非 RFC3339）
    // 断言：422, error.code = "MISSING_REQUIRED_FIELD"
}
```

---

### 调度器测试

调度器测试使用**手写 mock 实现**，完全隔离存储依赖：

```go
// internal/scheduler/scheduler_test.go

func TestScheduler_RunOnce_CallsAllStrategies(t *testing.T) {
    // 准备
    mockTrendRepo := testutil.NewMockTrendRepo()
    mockTrendRepo.SetTrends([]*domain.Trend{
        fixtures.NewTrend("trend-001"),
        fixtures.NewTrend("trend-002"),
    })
    mockSignalRepo := testutil.NewMockSignalRepo()
    mockStatsRepo := testutil.NewMockStatsRepo()

    mockStrategyA := testutil.NewMockStrategy("strategy_a")
    mockStrategyB := testutil.NewMockStrategy("strategy_b")

    registry := calculator.NewRegistry()
    registry.Register(mockStrategyA)
    registry.Register(mockStrategyB)

    scheduler := NewScheduler(mockTrendRepo, mockSignalRepo, mockStatsRepo, registry, cfg)

    // 执行
    err := scheduler.runOnce(context.Background())

    // 断言
    if err != nil {
        t.Fatalf("runOnce() error = %v", err)
    }
    // 两个策略各计算了 2 个趋势，共 4 次计算
    if mockStrategyA.CalculateCallCount() != 2 {
        t.Errorf("strategy_a.Calculate called %d times, want 2", mockStrategyA.CalculateCallCount())
    }
    if mockStrategyB.CalculateCallCount() != 2 {
        t.Errorf("strategy_b.Calculate called %d times, want 2", mockStrategyB.CalculateCallCount())
    }
    // BatchUpsert 被调用两次（每个策略一次）
    if mockStatsRepo.BatchUpsertCallCount() != 2 {
        t.Errorf("BatchUpsert called %d times, want 2", mockStatsRepo.BatchUpsertCallCount())
    }
}

func TestScheduler_RunOnce_StrategyError_ContinuesOtherStrategies(t *testing.T) {
    // 策略 A 的 Calculate 返回错误
    // 断言：策略 B 仍然被调用，runOnce 不返回错误
}

func TestScheduler_RunOnce_ContextCancelled_Stops(t *testing.T) {
    // 传入已取消的 context
    // 断言：runOnce 尽早返回
}
```

---

## Mock 策略

所有 mock 均为**手写实现**（不使用 mockgen 等工具），存放在 `internal/testutil/` 中：

```go
// internal/testutil/mock_trend_repo.go

type MockTrendRepo struct {
    mu     sync.RWMutex
    trends map[string]*domain.Trend
    // 调用计数，用于断言
    insertCalls int
}

func NewMockTrendRepo() *MockTrendRepo {
    return &MockTrendRepo{trends: make(map[string]*domain.Trend)}
}

func (m *MockTrendRepo) Insert(ctx context.Context, trend *domain.Trend) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if _, exists := m.trends[trend.ID]; exists {
        return fmt.Errorf("trend %q already exists", trend.ID)
    }
    m.trends[trend.ID] = trend
    m.insertCalls++
    return nil
}

// 其余接口方法类似实现...

func (m *MockTrendRepo) InsertCallCount() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.insertCalls
}
```

```go
// internal/testutil/mock_strategy.go

type MockStrategy struct {
    id             string
    calculateCalls int
    mu             sync.Mutex
    // 可配置的返回值，用于错误注入
    ReturnError error
}

func NewMockStrategy(id string) *MockStrategy {
    return &MockStrategy{id: id}
}

func (m *MockStrategy) ID() string   { return m.id }
func (m *MockStrategy) Name() string { return "Mock Strategy " + m.id }

func (m *MockStrategy) Calculate(ctx context.Context, trend *domain.Trend, signals []*domain.Signal) (*domain.TrendStats, error) {
    m.mu.Lock()
    m.calculateCalls++
    m.mu.Unlock()

    if m.ReturnError != nil {
        return nil, m.ReturnError
    }
    return &domain.TrendStats{
        ID:           fmt.Sprintf("%s:%s", m.id, trend.ID),
        TrendID:      trend.ID,
        StrategyID:   m.id,
        CalculatedAt: time.Now().UTC(),
        Score:        50.0,
        Phase:        "growing",
        Confidence:   0.7,
    }, nil
}

func (m *MockStrategy) CalculateCallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.calculateCalls
}
```

---

## /go-check 技能的使用时机

`/go-check` 是项目的 Go 代码质量检查技能，应在以下时机运行：

| 时机 | 说明 |
|------|------|
| 完成一个 Repository 实现后 | 验证存储层代码编译通过且测试绿色 |
| 完成一个 Strategy 实现后 | 确认算法逻辑测试全部通过 |
| 完成 API Handler 实现后 | 确认所有端点测试（含错误情况）通过 |
| PR 提交前 | 运行全套测试，确认覆盖率达标 |
| 重构代码后 | 确认重构未破坏现有功能 |

**典型使用方式**：

```
# 检查特定包
/go-check internal/calculator/momentum

# 检查整个项目
/go-check ./...

# 带覆盖率报告
/go-check --coverage ./...
```

---

## 测试辅助函数（Fixtures）

```go
// internal/testutil/fixtures.go

// NewTrend 构造一个用于测试的 Trend 对象
func NewTrend(id string) *domain.Trend {
    return &domain.Trend{
        ID:          id,
        Name:        "Test Trend " + id,
        Description: "A test trend",
        Categories:  []string{"test"},
        Source:      "test-source",
        CreatedAt:   time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
        UpdatedAt:   time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
    }
}

// NewSignal 构造一个用于测试的 Signal 对象
func NewSignal(trendID string, timestamp time.Time, usageCount int64) *domain.Signal {
    return &domain.Signal{
        ID:             "signal-" + trendID + "-" + timestamp.Format("150405"),
        TrendID:        trendID,
        Timestamp:      timestamp,
        UsageCount:     usageCount,
        UniqueCreators: usageCount / 10,
        AvgViews:       float64(usageCount) * 3.5,
        CreatedAt:      time.Now().UTC(),
    }
}

// NewTrendStats 构造一个用于测试的 TrendStats 对象
func NewTrendStats(strategyID, trendID string, score float64) *domain.TrendStats {
    return &domain.TrendStats{
        ID:           fmt.Sprintf("%s:%s", strategyID, trendID),
        TrendID:      trendID,
        StrategyID:   strategyID,
        CalculatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
        Score:        score,
        Momentum:     score / 10,
        Confidence:   0.8,
        Phase:        "growing",
    }
}
```

---

## 持续集成说明

所有测试应在无外部依赖的环境中运行（使用 InMemory BadgerDB，无网络调用）：

```bash
# 运行所有测试
go test ./...

# 运行测试并生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 运行特定包测试
go test ./internal/calculator/momentum/...

# 详细模式（显示每个测试用例）
go test -v ./...

# 竞态检测（并发安全验证）
go test -race ./...
```

**CI 最低要求**：

- 所有测试通过（`go test ./...` 退出码为 0）
- 无竞态条件（`go test -race ./...` 通过）
- 各层覆盖率达到目标值（见测试分层表）
