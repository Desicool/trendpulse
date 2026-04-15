# TrendPulse REST API 规格

## 概述

### 基本信息

| 项目 | 值 |
|------|-----|
| Base URL | `http://localhost:8080` |
| 内容类型 | `application/json` |
| 字符编码 | UTF-8 |
| 协议版本 | HTTP/1.1 |

### 认证

当前版本不包含认证机制。生产部署时应在反向代理层（如 nginx）添加访问控制。

### 重要约束

> **API 层绝对不触发任何计算逻辑。** 所有 GET 端点仅从预计算的 StatsRepository 读取数据。计算由调度器异步完成。

---

## 统一响应格式

所有响应均使用经典 RPC 风格，包含 `code`、`message`、`data` 三个字段。

### 成功响应（单个资源）

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... }
}
```

### 成功响应（列表 + 分页）

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [...],
    "pagination": {
      "offset": 0,
      "limit": 20,
      "total": 100
    }
  }
}
```

- `code: 0` 表示成功
- 列表响应的 `data` 包含 `items` 数组和 `pagination` 对象
- 单资源响应的 `data` 直接为对象

### 错误响应

```json
{
  "code": 40401,
  "message": "趋势 ID 不存在",
  "data": null
}
```

---

## 错误码表

| HTTP 状态码 | 错误码 | 常量名 | 说明 |
|-------------|--------|--------|------|
| 400 | `40001` | `ErrInvalidRequest` | 请求体格式错误或字段校验失败 |
| 400 | `40002` | `ErrInvalidPagination` | offset 或 limit 参数非法（如负数、超出最大值） |
| 404 | `40401` | `ErrTrendNotFound` | 指定 ID 的趋势不存在 |
| 404 | `40402` | `ErrStatsNotAvailable` | 趋势存在但尚无预计算统计（调度器未运行或刚启动） |
| 409 | `40901` | `ErrTrendAlreadyExists` | 尝试插入已存在 ID 的趋势 |
| 422 | `42201` | `ErrMissingRequiredField` | 请求体缺少必填字段 |
| 500 | `50001` | `ErrInternal` | 服务器内部错误 |

---

## 分页说明

列表端点（`GET /trends`）使用基于偏移量的分页：

| 参数 | 类型 | 默认值 | 最大值 | 说明 |
|------|------|--------|--------|------|
| `offset` | int | 0 | — | 跳过的记录数 |
| `limit` | int | 20 | 100 | 返回的最大记录数 |

**示例**：获取第 3 页，每页 10 条：`GET /trends?offset=20&limit=10`

响应中的 `meta.total` 表示满足条件的记录总数（不受 limit 影响），客户端可据此计算总页数。

---

## 端点详情

### 1. 列出趋势列表

**方法**：`GET`
**路径**：`/trends`
**描述**：返回所有趋势的分页列表，按创建时间倒序排列。

#### 查询参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `offset` | int | 0 | 偏移量（跳过记录数） |
| `limit` | int | 20 | 每页记录数，最大 100 |
| `phase` | string | — | 按生命周期阶段过滤，可选值：`emerging`\|`exploding`\|`peaking`\|`declining`，留空返回全部 |

> 若传入 `phase` 参数，仅返回当前活跃策略统计结果中 Phase 字段与之匹配的趋势。实现时需先按 Phase 索引查询 TrendStats，提取 TrendIDs，再批量查询 TrendRepository.ListByIDs 返回 Trend 列表。

#### 响应体结构

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [Trend, ...],
    "pagination": {
      "total": int,
      "offset": int,
      "limit": int
    }
  }
}
```

**Trend 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 趋势唯一标识符 |
| `name` | string | 趋势名称 |
| `description` | string | 趋势描述 |
| `categories` | []string | 类别标签列表 |
| `source` | string | 数据来源（如 "tiktok", "youtube"） |
| `created_at` | string (RFC3339) | 创建时间 |
| `updated_at` | string (RFC3339) | 最后更新时间 |

#### 示例请求

```
GET /trends?offset=0&limit=2
```

使用 phase 过滤：

```
GET /trends?offset=0&limit=10&phase=exploding
```

#### 示例响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "id": "trend-001",
        "name": "AI 绘画挑战",
        "description": "用户使用 AI 工具生成艺术画作并分享",
        "categories": ["art", "technology"],
        "source": "tiktok",
        "created_at": "2026-04-15T08:00:00Z",
        "updated_at": "2026-04-15T10:30:00Z"
      },
      {
        "id": "trend-002",
        "name": "慢生活 Vlog",
        "description": "记录田园生活与自然风光",
        "categories": ["lifestyle", "nature"],
        "source": "youtube",
        "created_at": "2026-04-15T07:00:00Z",
        "updated_at": "2026-04-15T09:00:00Z"
      }
    ],
    "pagination": {
      "total": 150,
      "offset": 0,
      "limit": 2
    }
  }
}
```

#### 错误情况

| 条件 | 状态码 | 错误码 |
|------|--------|--------|
| offset 为负数 | 400 | `40002` |
| limit 超过 100 | 400 | `40002` |
| limit 为 0 或负数 | 400 | `40002` |
| `phase` 参数值不合法 | 400 | `40001` | 提示有效值为 emerging/exploding/peaking/declining |

---

### 2. 获取单个趋势详情

**方法**：`GET`
**路径**：`/trends/{id}`
**描述**：返回指定 ID 的趋势详情，包含当前活跃策略的预计算统计数据。

#### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 趋势唯一标识符 |

#### 响应体结构

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "trend": Trend,
    "stats": TrendStats | null
  }
}
```

**TrendStats 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `trend_id` | string | 对应的趋势 ID |
| `strategy_id` | string | 计算本统计的策略 ID |
| `calculated_at` | string (RFC3339) | 计算时间 |
| `score` | float64 | 爆发概率评分，范围 [0, 100]，表示未来 48h 进入爆发期或高峰期的概率 |
| `confidence` | float64 | 预测置信度，0-1 |
| `phase` | string | 所处阶段：`"emerging"`\|`"exploding"`\|`"peaking"`\|`"declining"` |
| `latest_usage_count` | int64 | 最新使用次数 |
| `latest_unique_creators` | int64 | 最新独立创作者数 |
| `latest_avg_views` | float64 | 最新平均播放量 |
| `latest_avg_engagement` | float64 | 最新每 post 平均互动数 |
| `latest_view_concentration` | float64 | 最新播放集中度，范围 [0, 1] |

- 若趋势存在但尚无预计算统计，`stats` 字段为 `null`（不返回 404，返回 200 且 stats 为 null）

#### 示例请求

```
GET /trends/trend-001
```

#### 示例响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "trend": {
      "id": "trend-001",
      "name": "AI 绘画挑战",
      "description": "用户使用 AI 工具生成艺术画作并分享",
      "categories": ["art", "technology"],
      "source": "tiktok",
      "created_at": "2026-04-15T08:00:00Z",
      "updated_at": "2026-04-15T10:30:00Z"
    },
    "stats": {
      "trend_id": "trend-001",
      "strategy_id": "momentum_v1",
      "calculated_at": "2026-04-15T10:00:00Z",
      "score": 78.5,
      "confidence": 0.82,
      "phase": "exploding",
      "latest_usage_count": 45200,
      "latest_unique_creators": 3800,
      "latest_avg_views": 12500.0,
      "latest_avg_engagement": 340.5,
      "latest_view_concentration": 0.72
    }
  }
}
```

#### 错误情况

| 条件 | 状态码 | 错误码 |
|------|--------|--------|
| 趋势 ID 不存在 | 404 | `40401` |

---

### 3. 获取上升趋势排行

**方法**：`GET`
**路径**：`/trends/rising`
**描述**：返回 Score 最高的上升趋势列表，基于活跃策略的预计算统计数据，按 Score 降序排列。

> **注意**：此端点仅读取预计算结果，不触发实时计算。

#### 查询参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 20 | 返回数量，最大 100 |

#### 响应体结构

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "trend": Trend,
        "stats": TrendStats
      },
      ...
    ],
    "pagination": {
      "total": int,
      "limit": int,
      "strategy_id": "momentum_v1"
    }
  }
}
```

`data.pagination.strategy_id` 表明本次响应使用的策略（即 `config.scheduler.active_strategy`）。

#### 示例请求

```
GET /trends/rising?limit=3
```

#### 示例响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "trend": {
          "id": "trend-001",
          "name": "AI 绘画挑战",
          "categories": ["art", "technology"],
          "source": "tiktok",
          "created_at": "2026-04-15T08:00:00Z",
          "updated_at": "2026-04-15T10:30:00Z"
        },
        "stats": {
          "trend_id": "trend-001",
          "strategy_id": "momentum_v1",
          "calculated_at": "2026-04-15T10:00:00Z",
          "score": 92.1,
          "confidence": 0.91,
          "phase": "exploding",
          "latest_usage_count": 89000,
          "latest_unique_creators": 6500,
          "latest_avg_views": 21000.0,
          "latest_avg_engagement": 580.2,
          "latest_view_concentration": 0.85
        }
      },
      {
        "trend": {
          "id": "trend-005",
          "name": "早安跑步打卡",
          "categories": ["fitness", "lifestyle"],
          "source": "tiktok",
          "created_at": "2026-04-14T06:00:00Z",
          "updated_at": "2026-04-15T08:00:00Z"
        },
        "stats": {
          "trend_id": "trend-005",
          "strategy_id": "momentum_v1",
          "calculated_at": "2026-04-15T10:00:00Z",
          "score": 85.3,
          "confidence": 0.78,
          "phase": "exploding",
          "latest_usage_count": 55000,
          "latest_unique_creators": 4200,
          "latest_avg_views": 9800.0,
          "latest_avg_engagement": 412.7,
          "latest_view_concentration": 0.68
        }
      }
    ],
    "pagination": {
      "total": 2,
      "limit": 3,
      "strategy_id": "momentum_v1"
    }
  }
}
```

#### 错误情况

| 条件 | 状态码 | 错误码 |
|------|--------|--------|
| limit 超过 100 | 400 | `40002` |
| limit 为 0 或负数 | 400 | `40002` |

---

### 4. 摄取趋势数据

**方法**：`POST`
**路径**：`/ingest/trends`
**描述**：写入一条新的趋势文档。若趋势 ID 已存在，返回 409 冲突错误。

#### 请求体

```json
{
  "id": "string (必填，全局唯一)",
  "name": "string (必填)",
  "description": "string (可选)",
  "categories": ["string", ...],
  "source": "string (必填，如 'tiktok')"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 趋势唯一标识符，由调用方生成 |
| `name` | string | 是 | 趋势名称 |
| `description` | string | 否 | 趋势详细描述，默认为空字符串 |
| `categories` | []string | 否 | 类别标签，默认为空数组 |
| `source` | string | 是 | 数据来源平台标识 |

#### 响应体

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "trend-001",
    "name": "AI 绘画挑战",
    "description": "用户使用 AI 工具生成艺术画作并分享",
    "categories": ["art", "technology"],
    "source": "tiktok",
    "created_at": "2026-04-15T08:00:00Z",
    "updated_at": "2026-04-15T08:00:00Z"
  }
}
```

返回创建后的完整趋势对象（含服务器生成的 `created_at`、`updated_at`）。

#### 示例请求

```
POST /ingest/trends
Content-Type: application/json

{
  "id": "trend-001",
  "name": "AI 绘画挑战",
  "description": "用户使用 AI 工具生成艺术画作并分享",
  "categories": ["art", "technology"],
  "source": "tiktok"
}
```

#### 错误情况

| 条件 | 状态码 | 错误码 |
|------|--------|--------|
| 请求体非法 JSON | 400 | `40001` |
| id 字段缺失 | 422 | `42201` |
| name 字段缺失 | 422 | `42201` |
| source 字段缺失 | 422 | `42201` |
| 趋势 ID 已存在 | 409 | `40901` |

---

### 5. 摄取信号数据

**方法**：`POST`
**路径**：`/ingest/signals`
**描述**：写入一条趋势信号（时序指标数据）。信号代表某一时刻该趋势的量化指标快照。

#### 请求体

```json
{
  "trend_id": "string (必填)",
  "timestamp": "string RFC3339 (必填)",
  "usage_count": 12345,
  "unique_creators": 890,
  "avg_views": 5600.0
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `trend_id` | string | 是 | 关联的趋势 ID（必须已存在） |
| `timestamp` | string (RFC3339) | 是 | 信号采集时间 |
| `usage_count` | int64 | 是 | 该时刻的使用/发布次数 |
| `unique_creators` | int64 | 是 | 该时刻的独立创作者数量 |
| `avg_views` | float64 | 是 | 该时刻内容的平均播放量 |

#### 响应体

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "signal-uuid-xxx",
    "trend_id": "trend-001",
    "timestamp": "2026-04-15T10:00:00Z",
    "usage_count": 12345,
    "unique_creators": 890,
    "avg_views": 5600.0,
    "created_at": "2026-04-15T10:00:05Z"
  }
}
```

返回创建后的完整信号对象，`id` 由服务器生成（UUID）。

#### 示例请求

```
POST /ingest/signals
Content-Type: application/json

{
  "trend_id": "trend-001",
  "timestamp": "2026-04-15T10:00:00Z",
  "usage_count": 45200,
  "unique_creators": 3800,
  "avg_views": 12500.0
}
```

#### 错误情况

| 条件 | 状态码 | 错误码 |
|------|--------|--------|
| 请求体非法 JSON | 400 | `40001` |
| trend_id 缺失 | 422 | `42201` |
| timestamp 缺失或格式错误 | 422 | `42201` |
| trend_id 对应趋势不存在 | 404 | `40401` |
| usage_count 为负数 | 400 | `40001` |

---

## 路由注册顺序说明

由于 Go 标准库路由（或 chi/gorilla 路由器）对 `/trends/rising` 与 `/trends/{id}` 可能产生歧义，路由必须按以下顺序注册：

1. `GET /trends/rising`（精确路径，优先注册）
2. `GET /trends/{id}`（参数路径，后注册）
3. `GET /trends`
4. `POST /ingest/trends`
5. `POST /ingest/signals`

确保 `rising` 不被误识别为 `{id}` 的值。
