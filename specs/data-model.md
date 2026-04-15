# TrendPulse 数据模型规格

## 概述

TrendPulse 使用 BadgerDB + BadgerHold 作为嵌入式存储引擎。所有实体以 **JSON** 编码持久化（非 Gob），原因如下：

- JSON 格式人类可读，便于开发调试
- 与未来迁移至 MongoDB / PostgreSQL 兼容（BSON / JSONB）
- 避免 Gob 编码的版本兼容性问题（字段增删需特殊处理）

BadgerHold 通过结构体标签 `badgerhold:"key"` 和 `badgerhold:"index"` 声明主键与索引字段。

---

## 实体定义

### 1. Trend（趋势文档）

**文件路径**：`internal/domain/trend.go`

**描述**：代表一个被追踪的内容趋势，是系统中的核心实体。由数据摄取端点写入，由 API 查询端点读取。

```go
// Trend 表示一个被追踪的内容趋势
type Trend struct {
    ID          string    `json:"id"           badgerhold:"key"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    // Categories 不建立索引，类别查询通过 CategoryMapping 实现
    // 此字段仅用于展示，不用于过滤查询
    Categories  []string  `json:"categories"`
    Source      string    `json:"source"       badgerhold:"index"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 索引 | 说明 |
|------|------|------|------|------|
| `ID` | string | 是 | 主键 | 由摄取方提供，全局唯一，建议使用 UUID 或平台原生 ID |
| `Name` | string | 是 | 否 | 趋势名称，如"AI 绘画挑战" |
| `Description` | string | 否 | 否 | 趋势详细描述，可为空字符串 |
| `Categories` | []string | 否 | 否 | 类别标签，仅用于展示；类别检索通过 CategoryMapping 完成 |
| `Source` | string | 是 | 是 | 数据来源平台，如 "tiktok", "youtube", "instagram" |
| `CreatedAt` | time.Time | 服务器生成 | 否 | 趋势首次写入时间，服务器端赋值 |
| `UpdatedAt` | time.Time | 服务器生成 | 否 | 趋势最后更新时间，服务器端赋值 |

**约束**：
- `ID` 一旦写入不可更改
- `Categories` 字段为空数组时序列化为 `[]`，不为 `null`

---

### 2. Signal（信号 / 时序指标）

**文件路径**：`internal/domain/signal.go`

**描述**：代表某趋势在某一时刻的量化指标快照。信号是时序数据，由模拟器或真实数据源批量写入。调度器读取信号序列来计算 TrendStats。

```go
// Signal 表示某趋势在某时刻的量化指标快照
type Signal struct {
    ID             string    `json:"id"              badgerhold:"key"`
    TrendID        string    `json:"trend_id"        badgerhold:"index"`
    Timestamp      time.Time `json:"timestamp"       badgerhold:"index"`
    UsageCount     int64     `json:"usage_count"`
    UniqueCreators int64     `json:"unique_creators"`
    AvgViews          float64   `json:"avg_views"`
    AvgEngagement     float64   `json:"avg_engagement"`     // 每 post 平均互动数 (likes+comments+shares)
    ViewConcentration float64   `json:"view_concentration"` // top-1 视频播放量/总播放量，范围 [0,1]
    CreatedAt         time.Time `json:"created_at"`
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 索引 | 说明 |
|------|------|------|------|------|
| `ID` | string | 服务器生成 | 主键 | UUID，服务器端生成 |
| `TrendID` | string | 是 | 是 | 关联的 Trend.ID，用于按趋势查询信号序列 |
| `Timestamp` | time.Time | 是 | 是 | 信号采集时间，用于时间范围查询 |
| `UsageCount` | int64 | 是 | 否 | 该时刻趋势内容的发布/使用次数 |
| `UniqueCreators` | int64 | 是 | 否 | 该时刻参与该趋势的独立创作者数量 |
| `AvgViews` | float64 | 是 | 否 | 该时刻趋势内容的平均播放量 |
| `AvgEngagement` | float64 | 是 | 否 | 每 post 平均互动数（点赞+评论+分享），由数据来源方计算后传入 |
| `ViewConcentration` | float64 | 是 | 否 | 播放量集中度：最热门单视频播放量/该趋势所有视频总播放量，范围 [0,1]，越高说明少数视频拿走大量流量，爆发期早期信号 |
| `CreatedAt` | time.Time | 服务器生成 | 否 | 记录写入时间 |

**查询模式**：

| 查询方法 | 使用的索引 | 说明 |
|----------|-----------|------|
| `ListByTrendID(trendID, from, to)` | TrendID + Timestamp | 获取指定趋势在时间窗口内的信号序列 |
| `GetLatestByTrendID(trendID)` | TrendID + Timestamp (DESC) | 获取趋势的最新一条信号 |
| `ListByTimeRange(from, to)` | Timestamp | 获取所有趋势在时间范围内的信号（跨趋势） |

---

### 3. TrendStats（趋势统计 / 策略输出）

**文件路径**：`internal/domain/stats.go`

**描述**：所有预测策略的统一输出格式。由调度器写入，由 API 层读取。每个（strategy_id, trend_id）组合对应一条记录，使用 Upsert 语义（覆盖写）。

```go
// TrendStats 是所有预测策略的统一输出结构
// 复合主键格式："{strategy_id}:{trend_id}"
// Score 和 Phase 完全独立计算，语义不同
type TrendStats struct {
    ID           string    `json:"id"            badgerhold:"key"`
    TrendID      string    `json:"trend_id"      badgerhold:"index"`
    StrategyID   string    `json:"strategy_id"   badgerhold:"index"`
    CalculatedAt time.Time `json:"calculated_at"`

    // 爆发概率评分：未来 48h 进入爆发期或高峰期的概率，与 Phase 独立计算
    Score      float64 `json:"score"`
    // 当前阶段：基于当前增长率模式判定，与 Score 独立计算
    Phase      string  `json:"phase"      badgerhold:"index"`
    Confidence float64 `json:"confidence"`

    // 最新信号快照（冗余存储，避免 join 查询）
    LatestUsageCount        int64   `json:"latest_usage_count"`
    LatestUniqueCreators    int64   `json:"latest_unique_creators"`
    LatestAvgViews          float64 `json:"latest_avg_views"`
    LatestAvgEngagement     float64 `json:"latest_avg_engagement"`
    LatestViewConcentration float64 `json:"latest_view_concentration"`

    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

**字段说明**：

| 字段 | 类型 | 索引 | 说明 |
|------|------|------|------|
| `ID` | string | 主键 | 复合键：`"{strategy_id}:{trend_id}"` |
| `TrendID` | string | 是 | 关联的 Trend.ID |
| `StrategyID` | string | 是 | 计算本统计的策略 ID |
| `CalculatedAt` | time.Time | 否 | 本次计算完成时间 |
| `Score` | float64 | 否 | 爆发概率评分 [0, 100]：该趋势在未来 48 小时内进入爆发期或高峰期的可能性。与 Phase 独立计算。 |
| `Phase` | string | 是 | 当前生命周期阶段：`"emerging"`\|`"exploding"`\|`"peaking"`\|`"declining"` |
| `Confidence` | float64 | 否 | 预测置信度 [0, 1]，基于可用信号数量和质量 |
| `LatestUsageCount` | int64 | 否 | 最新信号的 post 发布数 |
| `LatestUniqueCreators` | int64 | 否 | 最新信号的独立创作者数 |
| `LatestAvgViews` | float64 | 否 | 最新信号的平均播放量 |
| `LatestAvgEngagement` | float64 | 否 | 最新信号的每 post 平均互动数 |
| `LatestViewConcentration` | float64 | 否 | 最新信号的播放集中度 |
| `Metadata` | map[string]interface{} | 否 | 策略内部中间计算值，供调试 |

**复合键设计说明**：

```
ID = "{strategy_id}:{trend_id}"

示例：
  "momentum_v1:trend-001"
  "velocity_v1:trend-001"
  "momentum_v1:trend-002"
```

使用复合键的好处：
1. 同一趋势可被多个策略计算，互不覆盖
2. 天然支持 A/B 测试（不同策略并行存储结果）
3. Upsert 语义：相同复合键的新计算结果覆盖旧结果

**Phase 枚举值**：

| 值 | 中文名 | TikTok 特征 | 是否"趋势中" |
|----|--------|-------------|------------|
| `emerging` | 萌芽期 | post count 稳定或轻微线性增长；views 稳定增长；无明显加速度；view_concentration 低 | 否 |
| `exploding` | 爆发期 | 一个视频爆火 → avg_views 加速度极高；avg_engagement 激增；post_count 高增长率；view_concentration 高（少数视频拿走大量流量） | **是** |
| `peaking` | 高峰期 | post 增长率趋近于零（线性平稳）；views/engagement 增长率≈0；各指标绝对值保持高位 | **是** |
| `declining` | 衰退期 | post count 环比下降；views/engagement 负增长；各维度均下滑 | 否 |

> **注意**：只有 `exploding` 和 `peaking` 被视为"正在趋势中（Trending）"。Score 评分预测趋势在未来 48h 内进入这两个阶段的概率。

> **Score 与 Phase 解耦**：Score 与 Phase 完全解耦：一个 `emerging` 趋势可能有 Score=90（即将爆发），一个 `peaking` 趋势可能有 Score=10（已过高峰）。

---

### 4. CategoryMapping（类别反向索引）

**文件路径**：`internal/domain/category.go`

**描述**：BadgerHold 无法对 `[]string` 字段建立索引，因此维护此去规范化实体作为类别的反向索引。每个（category, trend_id）组合对应一条记录。

```go
// CategoryMapping 是类别到趋势的反向索引
// 解决 BadgerHold 无法索引 []string 字段的问题
// 复合主键格式："{category}:{trend_id}"
type CategoryMapping struct {
    ID       string `json:"id"       badgerhold:"key"`
    Category string `json:"category" badgerhold:"index"`
    TrendID  string `json:"trend_id" badgerhold:"index"`
}
```

**字段说明**：

| 字段 | 类型 | 索引 | 说明 |
|------|------|------|------|
| `ID` | string | 主键 | 复合键：`"{category}:{trend_id}"` |
| `Category` | string | 是 | 类别名称，如 "music", "fitness" |
| `TrendID` | string | 是 | 关联的 Trend.ID |

**复合键示例**：

```
类别 ["music", "dance"] 的趋势 "trend-001"，生成以下记录：

CategoryMapping{ID: "music:trend-001",  Category: "music",  TrendID: "trend-001"}
CategoryMapping{ID: "dance:trend-001",  Category: "dance",  TrendID: "trend-001"}
```

---

## 数据生命周期

| 实体 | 创建时机 | 更新时机 | 读取时机 |
|------|----------|----------|----------|
| Trend | `POST /ingest/trends` | 未来扩展（当前只写入不更新） | `GET /trends`, `GET /trends/{id}`, `GET /trends/rising` |
| Signal | `POST /ingest/signals` | 不更新（追加写） | 调度器计算时 |
| TrendStats | 调度器首次计算后 | 调度器每次计算（Upsert） | `GET /trends/{id}`, `GET /trends/rising` |
| CategoryMapping | `POST /ingest/trends`（同步写入） | 不更新（当前不支持修改类别） | 类别过滤查询时 |

---

## JSON 编码说明

### 为何选择 JSON 而非 Gob

| 对比维度 | JSON | Gob |
|----------|------|-----|
| 可读性 | 人类可读，可直接用工具查看 | 二进制，不可读 |
| 版本兼容性 | 新增字段向后兼容，旧数据可反序列化 | 字段增删需额外处理 |
| 迁移兼容性 | 与 MongoDB BSON / PostgreSQL JSONB 高度兼容 | 迁移需重新编码所有数据 |
| 性能 | 略慢于 Gob | 更快 |
| 包大小 | 略大于 Gob | 更小 |

对于 TrendPulse 的使用场景，可读性和迁移兼容性优先于性能，故选择 JSON。

### 时间字段编码

所有 `time.Time` 字段序列化为 RFC3339 格式（含纳秒精度）：

```
"2026-04-15T10:00:00.000000000Z"
```

可空时间字段（如 `PredictedPeak *time.Time`）在 nil 时序列化为 `null`，并使用 `omitempty` tag 在为 nil 时省略该字段。

### map[string]interface{} 字段

`TrendStats.Metadata` 使用 `map[string]interface{}`，JSON 序列化时保留原始类型。各策略应在此字段中存储可调试的中间计算值，便于问题排查。

---

## 模拟数据格式

以下是模拟器（`cmd/simulator/main.go`）生成和发送的数据格式示例：

### 摄取趋势（POST /ingest/trends）

```json
{
  "id": "trend-sim-001",
  "name": "AI 绘画挑战",
  "description": "用户使用 AI 工具生成艺术画作并分享作品",
  "categories": ["art", "technology", "challenge"],
  "source": "tiktok"
}
```

```json
{
  "id": "trend-sim-002",
  "name": "慢生活农场 Vlog",
  "description": "记录田园生活，种植、烹饪与自然风光",
  "categories": ["lifestyle", "food", "nature"],
  "source": "youtube"
}
```

### 摄取信号序列（POST /ingest/signals）

同一趋势的时序信号，模拟上升趋势（每小时一条，共 48 条）：

```json
{
  "trend_id": "trend-sim-001",
  "timestamp": "2026-04-13T10:00:00Z",
  "usage_count": 1200,
  "unique_creators": 89,
  "avg_views": 3200.0
}
```

```json
{
  "trend_id": "trend-sim-001",
  "timestamp": "2026-04-13T11:00:00Z",
  "usage_count": 1580,
  "unique_creators": 115,
  "avg_views": 4100.0
}
```

```json
{
  "trend_id": "trend-sim-001",
  "timestamp": "2026-04-14T10:00:00Z",
  "usage_count": 28900,
  "unique_creators": 2100,
  "avg_views": 11500.0
}
```

```json
{
  "trend_id": "trend-sim-001",
  "timestamp": "2026-04-15T10:00:00Z",
  "usage_count": 45200,
  "unique_creators": 3800,
  "avg_views": 12500.0
}
```

### 模拟器行为说明

模拟器 (`cmd/simulator/main.go`) 应：

1. 读取种子配置（趋势列表 + 增长模式）
2. 批量 POST 趋势到 `/ingest/trends`
3. 为每个趋势生成 48 小时 × N 条信号，模拟不同增长曲线（线性增长、指数增长、先增后减）
4. 批量 POST 信号到 `/ingest/signals`
5. 支持 `--dry-run` 标志，仅打印生成的数据不发送

**信号生成参数**（可通过命令行或配置文件控制）：

```yaml
simulator:
  base_url: "http://localhost:8080"
  trends_count: 20          # 生成的趋势数量
  signals_per_trend: 48     # 每个趋势的信号数量
  signal_interval: "1h"     # 信号时间间隔
  growth_patterns:          # 增长模式分布
    - type: "exponential"   # 指数增长
      weight: 0.4
    - type: "linear"        # 线性增长
      weight: 0.3
    - type: "peak_and_decline"  # 先增后减
      weight: 0.3
```
