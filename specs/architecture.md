# TrendPulse 系统架构规格

## 系统概述

TrendPulse 是一个趋势预测系统，使用 Go 语言编写，能够预测未来 48 小时内最可能出现的热门趋势。系统采用三层架构设计：

- **存储层（Storage Layer）**：负责持久化趋势、信号与统计数据
- **计算层（Calculator Layer）**：负责运行可插拔的预测策略，定期生成 TrendStats
- **API 层（API Layer）**：对外暴露 REST 接口，仅读取预计算结果，不触发任何计算

核心设计原则：

1. **嵌入式数据库**：使用 BadgerDB + BadgerHold，无需外部进程，开箱即用
2. **可插拔策略**：所有预测算法实现统一 `Strategy` 接口，输出统一 `TrendStats`
3. **计算与读取分离**：调度器负责计算，API 层只读预计算结果
4. **测试优先（TDD）**：先写测试再实现，确保代码质量

---

## 目录结构

```
trendpulse/
├── AGENTS.md
├── CLAUDE.md
├── Makefile
├── README.md
├── cmd/
│   ├── server/
│   │   └── main.go              # HTTP 服务入口，加载配置、初始化依赖、启动调度器与 API
│   └── simulator/
│       └── main.go              # 模拟数据生成脚本，通过 REST API 写入测试数据
├── configs/
│   ├── config.example.yaml      # 示例配置文件
│   └── config.yaml              # 默认配置文件
├── data/
│   └── badger/                  # BadgerDB 运行时数据目录（已 gitignore）
├── internal/
│   ├── api/
│   │   ├── handler/
│   │   │   ├── ingest_handler.go    # POST /ingest/trends, POST /ingest/signals
│   │   │   ├── ingest_handler_test.go
│   │   │   ├── trend_handler.go     # GET /trends, GET /trends/{id}, GET /trends/rising
│   │   │   └── trend_handler_test.go
│   │   ├── middleware/
│   │   │   └── logging.go           # 请求日志中间件
│   │   ├── response/
│   │   │   └── response.go          # 统一响应格式封装
│   │   └── router.go                # HTTP 路由注册
│   ├── calculator/
│   │   ├── interfaces.go            # Strategy 接口定义
│   │   ├── phase.go                 # 趋势阶段（Phase）判断逻辑
│   │   ├── phase_test.go
│   │   ├── registry.go              # 策略注册表，支持按 ID 查找与列举
│   │   ├── registry_test.go
│   │   └── sigmoid/
│   │       ├── sigmoid_v1.go        # sigmoid_v1 策略实现（Sigmoid 评分曲线）
│   │       └── sigmoid_v1_test.go
│   ├── config/
│   │   └── config.go                # YAML 配置加载与结构体定义
│   ├── domain/
│   │   ├── category.go              # CategoryMapping 实体定义（BadgerHold 数组字段绕过方案）
│   │   ├── signal.go                # Signal / Metrics 实体定义
│   │   ├── stats.go                 # TrendStats 实体定义（所有策略的统一输出）
│   │   └── trend.go                 # Trend 实体定义
│   ├── repository/
│   │   ├── badger/                  # BadgerHold 实现（当前主实现）
│   │   │   ├── category_index.go
│   │   │   ├── category_index_test.go
│   │   │   ├── signal_repo.go
│   │   │   ├── signal_repo_test.go
│   │   │   ├── stats_repo.go
│   │   │   ├── stats_repo_test.go
│   │   │   ├── store.go
│   │   │   ├── trend_repo.go
│   │   │   └── trend_repo_test.go
│   │   ├── interfaces.go            # TrendRepository / SignalRepository / StatsRepository / CategoryIndex 接口定义
│   │   ├── mongo/                   # 预留目录，未来迁移 MongoDB 时实现
│   │   └── postgres/                # 预留目录，未来迁移 PostgreSQL 时实现
│   ├── scheduler/
│   │   ├── scheduler.go             # 定时调度器，定期对所有趋势运行所有策略
│   │   ├── scheduler_test.go
│   │   ├── signal_reader.go         # 为调度器读取信号的辅助逻辑
│   │   └── signal_reader_test.go
│   ├── simulator/
│   │   ├── client.go                # HTTP 客户端，调用 ingest 接口写入数据
│   │   ├── generator.go             # 模拟数据生成逻辑
│   │   └── seed_data.go             # 确定性种子数据（可复现的演示数据）
│   └── testutil/
│       ├── badger_helper.go         # 测试用 BadgerDB 辅助函数
│       ├── compile_check_test.go
│       ├── fixtures.go              # 测试固定数据
│       ├── mock_category_index.go   # CategoryIndex mock
│       ├── mock_signal_repo.go      # SignalRepository mock
│       ├── mock_stats_repo.go       # StatsRepository mock
│       ├── mock_strategy.go         # Strategy mock
│       └── mock_trend_repo.go       # TrendRepository mock
├── specs/                           # 规格文档目录（本文件所在位置）
├── .claude/
│   ├── agents/                      # Claude Code 子代理定义
│   └── skills/                      # Claude Code 技能定义
├── go.mod
└── go.sum
```

---

## 三层架构详解

### 存储层（Storage Layer）

**职责**：

- 持久化原始趋势数据（Trend）
- 持久化时序信号数据（Signal）
- 持久化预计算统计结果（TrendStats）
- 维护类别索引（CategoryMapping）

**技术选型**：

- **BadgerDB**：高性能嵌入式 KV 存储，LSM-tree 结构，适合写多读少场景
- **BadgerHold**：基于 BadgerDB 的结构化查询层，支持字段索引与范围查询
- 数据以 **JSON** 编码（非 Gob），便于调试与未来迁移

**当前实现目录**：`internal/repository/badger/`

**约束**：

- BadgerHold 不支持对 `[]string` 类型字段建立索引
- 解决方案见"BadgerHold 数组字段限制"章节

---

### 计算层（Calculator Layer）

**职责**：

- 定义 `Strategy` 接口，统一计算协议
- 维护策略注册表（Registry）
- 调度器定期触发所有策略对所有趋势进行计算
- 将计算结果批量写入 StatsRepository

**设计要点**：

- 每个策略独立运行，互不依赖
- 所有策略输出相同的 `TrendStats` 结构
- 支持 A/B 测试：通过配置 `active_strategy` 决定 API 返回哪个策略的结果
- 调度器并发运行多个策略（goroutine per strategy）

---

### API 层（API Layer）

**职责**：

- 对外暴露 REST 接口
- 处理数据摄取（ingest）请求，将数据写入存储层
- 读取预计算的 TrendStats，组装响应

**关键约束**：

> **API 层绝对不触发任何计算逻辑。** 所有 GET 端点只从 StatsRepository 读取预计算结果。

---

## 层间接口

### 存储层接口

```go
// internal/repository/interfaces.go

package repository

import (
    "context"
    "time"

    "github.com/yourorg/trendpulse/internal/domain"
)

// TrendRepository 管理趋势文档的持久化
type TrendRepository interface {
    Insert(ctx context.Context, trend *domain.Trend) error
    GetByID(ctx context.Context, id string) (*domain.Trend, error)
    List(ctx context.Context, offset, limit int) ([]*domain.Trend, int, error)
    Update(ctx context.Context, trend *domain.Trend) error
    ListByIDs(ctx context.Context, ids []string) ([]*domain.Trend, error)
}

// SignalRepository 管理时序信号数据的持久化
type SignalRepository interface {
    Insert(ctx context.Context, signal *domain.Signal) error
    ListByTrendID(ctx context.Context, trendID string, from, to time.Time) ([]*domain.Signal, error)
    GetLatestByTrendID(ctx context.Context, trendID string) (*domain.Signal, error)
    ListByTimeRange(ctx context.Context, from, to time.Time) ([]*domain.Signal, error)
}

// StatsRepository 管理预计算统计结果的持久化
type StatsRepository interface {
    Upsert(ctx context.Context, stat *domain.TrendStats) error
    BatchUpsert(ctx context.Context, stats []*domain.TrendStats) error
    GetByTrendID(ctx context.Context, trendID string, strategyID string) (*domain.TrendStats, error)
    ListRising(ctx context.Context, strategyID string, limit int) ([]*domain.TrendStats, error)
    ListByStrategyID(ctx context.Context, strategyID string) ([]*domain.TrendStats, error)
}

// CategoryIndex 维护趋势类别的反向索引
type CategoryIndex interface {
    SetCategories(ctx context.Context, trendID string, categories []string) error
    GetTrendIDsByCategory(ctx context.Context, category string) ([]string, error)
}
```

### 计算层接口

```go
// internal/calculator/interfaces.go

package calculator

import (
    "context"

    "github.com/yourorg/trendpulse/internal/domain"
)

// Strategy 定义趋势预测策略的统一接口
// 所有策略实现必须满足此接口，并输出标准 TrendStats
type Strategy interface {
    // ID 返回策略的唯一标识符，用于注册表查找与配置引用
    ID() string
    // Name 返回策略的人类可读名称，用于日志与 API 展示
    Name() string
    // Calculate 基于趋势文档和历史信号计算统计结果
    // signals 按时间升序排列，最新信号在末尾
    Calculate(ctx context.Context, trend *domain.Trend, signals []*domain.Signal) (*domain.TrendStats, error)
}
```

---

## 数据流

### 数据摄取流（Ingestion Flow）

```
Simulator
  │
  ├─ POST /ingest/trends  ──► IngestHandler.IngestTrend()
  │                               │
  │                               ├─ TrendRepository.Insert()
  │                               └─ CategoryIndex.SetCategories()
  │
  └─ POST /ingest/signals ──► IngestHandler.IngestSignal()
                                  │
                                  └─ SignalRepository.Insert()
```

### 计算流（Calculation Flow）

```
Scheduler（ticker 定时触发，间隔由 config.scheduler.interval 决定）
  │
  ├─ TrendRepository.List()          # 获取所有趋势
  ├─ SignalRepository.ListByTrendID() # 获取 signal_lookback 窗口内的信号
  │
  └─ 对每个 Strategy 启动 goroutine：
       │
       ├─ Strategy.Calculate(trend, signals) → TrendStats
       └─ StatsRepository.BatchUpsert([]*TrendStats)
```

### 读取流（Read Flow）

```
HTTP Client
  │
  ├─ GET /trends          ──► TrendHandler.List()
  │                               └─ TrendRepository.List(offset, limit)
  │
  ├─ GET /trends/{id}     ──► TrendHandler.Get()
  │                               ├─ TrendRepository.GetByID(id)
  │                               └─ StatsRepository.GetByTrendID(id, active_strategy)
  │
  └─ GET /trends/rising   ──► TrendHandler.Rising()
                                  └─ StatsRepository.ListRising(active_strategy, limit)
```

---

## 配置结构

配置文件位于 `configs/config.yaml`，完整结构如下：

```yaml
# TrendPulse 主配置文件

# 数据库配置
database:
  badger:
    path: "./data/badger"          # BadgerDB 数据目录
    in_memory: false               # 设为 true 时使用内存模式（测试用）
    sync_writes: false             # 是否同步写入磁盘（false 提升性能）

# HTTP 服务器配置
server:
  host: "0.0.0.0"                  # 监听地址
  port: 8080                       # 监听端口
  read_timeout: "15s"              # 读超时
  write_timeout: "15s"             # 写超时

# 调度器配置
scheduler:
  interval: "5m"                   # 计算触发间隔（如 "1m", "5m", "1h"）
  signal_lookback: "48h"           # 每次计算读取的信号时间窗口
  active_strategy: "momentum_v1"   # A/B 测试：API 默认使用的策略 ID

# 策略配置
strategies:
  momentum_v1:
    enabled: true
    decay_factor: 0.9              # 指数加权衰减因子，范围 (0, 1)
    score_weights:                 # Score 各分量的权重，总和应为 1.0
      usage_count: 0.4
      unique_creators: 0.3
      avg_views: 0.3

  velocity_v1:
    enabled: true
    window_size: 6                 # 滑动窗口大小（信号数量）

# API 配置
api:
  default_limit: 20               # 列表端点默认分页大小
  max_limit: 100                  # 列表端点最大分页大小
  rising_top_k: 20                # /trends/rising 默认返回数量

# 日志配置
log:
  level: "info"                   # 日志级别：debug / info / warn / error
  format: "json"                  # 日志格式：json / text
```

---

## BadgerHold 数组字段限制与解决方案

### 问题描述

BadgerHold 使用反射分析结构体字段并建立索引。当字段类型为 `[]string` 时，BadgerHold **无法对其建立索引**，也无法通过 `badgerhold.Where("Categories").Contains("music")` 类型的查询进行过滤。

### 解决方案：CategoryMapping 去规范化实体

每当一个趋势拥有多个类别时，系统不在 Trend 实体上存储 `[]string` 类别字段，而是为每个类别单独创建一条 `CategoryMapping` 记录：

```go
// internal/domain/category.go

// CategoryMapping 是类别到趋势的反向索引记录
// 复合主键格式："{category}:{trend_id}"
type CategoryMapping struct {
    ID       string `badgerhold:"key"`           // 复合键，格式："{category}:{trend_id}"
    Category string `badgerhold:"index"`         // 可索引的字符串字段
    TrendID  string `badgerhold:"index"`         // 可索引的字符串字段
}
```

### 操作流程

**写入（趋势摄取时）**：

```
POST /ingest/trends  { categories: ["music", "dance"] }
  │
  ├─ TrendRepository.Insert(trend)
  └─ CategoryIndex.SetCategories(trendID, ["music", "dance"])
       │
       ├─ 写入 CategoryMapping{ID: "music:trend-123", Category: "music", TrendID: "trend-123"}
       └─ 写入 CategoryMapping{ID: "dance:trend-123", Category: "dance", TrendID: "trend-123"}
```

**读取（按类别查询趋势）**：

```
CategoryIndex.GetTrendIDsByCategory("music")
  │
  ├─ badgerhold.Where("Category").Eq("music") → [CategoryMapping, ...]
  ├─ 提取所有 TrendID
  └─ TrendRepository.ListByIDs(trendIDs)
```

### 权衡

| 方面 | 影响 |
|------|------|
| 存储开销 | 每个类别-趋势关联额外一条记录，可接受 |
| 写入复杂度 | 需原子写入 Trend + N 条 CategoryMapping，需注意部分失败场景 |
| 查询性能 | 两步查询（索引查 → 批量取），性能可接受 |
| 迁移兼容性 | MongoDB / PostgreSQL 原生支持数组查询，迁移后可删除此绕过逻辑 |

---

## 迁移路径

系统在设计上通过接口隔离存储实现，迁移到 MongoDB 或 PostgreSQL 仅需实现对应接口，无需修改业务逻辑。

### 迁移步骤

1. 在 `internal/repository/mongo/` 或 `internal/repository/postgres/` 目录下实现全部接口：
   - `TrendRepository`
   - `SignalRepository`
   - `StatsRepository`
   - `CategoryIndex`（MongoDB/PostgreSQL 原生支持数组字段查询，可简化此实现）

2. 在 `internal/config/config.go` 中添加新数据库配置块

3. 在 `cmd/server/main.go` 的依赖注入处，根据配置选择具体实现：

```go
var trendRepo repository.TrendRepository
switch cfg.Database.Driver {
case "badger":
    trendRepo = badger.NewTrendRepository(db)
case "mongo":
    trendRepo = mongo.NewTrendRepository(mongoClient)
case "postgres":
    trendRepo = postgres.NewTrendRepository(pgPool)
}
```

4. 数据迁移脚本：读取 BadgerDB 数据，批量写入新数据库

5. 切换配置，验证功能

### 注意事项

- `CategoryMapping` 在 MongoDB（使用数组字段 + 多键索引）和 PostgreSQL（使用关联表或数组列）下可删除
- TrendStats 的复合键 `"{strategy_id}:{trend_id}"` 在关系数据库中可改为联合主键
- 确保新实现通过现有集成测试套件
