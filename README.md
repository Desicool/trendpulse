# TrendPulse

> 基于 TikTok 信号的趋势预测系统，预测未来 48 小时最可能爆发的内容趋势。

## 架构说明

三层架构，各层通过接口交互，互不直接依赖。

### 存储层

使用 **BadgerDB + BadgerHold** 嵌入式存储，无需外部数据库进程，单二进制部署。Repository interface 抽象了所有存储操作，`internal/repository/mongo/` 和 `internal/repository/postgres/` 目录预留了未来迁移路径。

存储四种实体：
- **Trend**：趋势文档，包含名称、类型、类别、地区等元数据
- **Signal**：时序指标快照，每小时一条，记录 post 数量、创作者数、播放量、互动量、播放集中度
- **TrendStats**：策略的预计算结果，由调度器写入，API 层只读不算
- **CategoryMapping**：类别反向索引，绕开 BadgerHold 不支持数组字段索引的限制

### 计算层

采用**可插拔 Strategy 接口**设计。每个策略实现 `Strategy` 接口，接收 `SignalReader`（懒加载，按需读取信号），输出统一的 `TrendStats` 结构。

策略通过 Registry 注册，Scheduler 按配置间隔定时触发所有已注册策略**并行计算**，结果批量写入 StatsRepository。A/B 测试通过 `active_strategy` 配置切换，历史数据保留。

**API 层绝不触发计算**，只读取调度器写入的预计算结果。

### API 层

使用 Go 标准库 `net/http`（Go 1.22+ 方法模式路由），RPC 风格统一响应格式：

```json
{"code": 0, "message": "ok", "data": {...}}
```

五个端点：
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/trends` | 趋势列表（分页，支持 `?phase=` 过滤） |
| GET | `/trends/{id}` | 趋势详情 + 当前策略统计 |
| GET | `/trends/rising` | Top-K 爆发概率最高的趋势 |
| POST | `/ingest/trends` | 写入趋势文档 |
| POST | `/ingest/signals` | 写入信号数据 |

---

## 评分算法说明及设计思路

### 设计思路

Score（爆发概率）和 Phase（当前阶段）由**同一个 Strategy 共同计算**，共享特征提取逻辑。这是有意为之的设计：Phase 的定义（什么样的信号组合 = 爆发期）决定了 Score 要预测的目标，强制分离会产生人为耦合。

不同策略可以定义完全不同的 Phase 语义和 Score 算法，只要输出统一的 `TrendStats` 结构即可替换。

### TikTok 趋势生命周期（4 个阶段）

| 阶段 | 英文值 | TikTok 行为特征 | 是否"正在趋势中" |
|------|--------|----------------|----------------|
| 萌芽期 | `emerging` | post 数量稳定线性增长，播放量无明显加速，互动率低位平稳 | 否 |
| 爆发期 | `exploding` | 少数视频爆火（view_concentration 激增）→ 带动 post 高速增长；播放量二阶加速度显著；点赞/评论/分享激增 | **是** |
| 高峰期 | `peaking` | post 增长率趋近于零（线性平稳）；各指标绝对值保持高位；算法持续分发 | **是** |
| 衰退期 | `declining` | post 数量环比下降；播放量、互动量全面负增长 | 否 |

只有 `exploding` 和 `peaking` 被视为"正在趋势中"，是系统的核心关注对象。

### 爆发概率评分（Score）

Score 回答：**该趋势在未来 48 小时内进入爆发期或高峰期的概率是多少？**

`sigmoid_v1` 参考策略使用加权线性 + Sigmoid 公式：

```
raw = α × 播放量加速度
    + β × post 增长率
    + γ × 创作者增长率
    + δ × 互动激增率
    + ε × 播放集中度
    − bias

Score = 100 × sigmoid(raw)     sigmoid(x) = 1 / (1 + e^(−x))
```

各维度说明：
- **播放量加速度**（权重最高）：`(v[t] - 2·v[t-1] + v[t-2]) / v[t-2]`，二阶差分，爆发前最早出现的信号
- **post 增长率**：创作者跟进速度，反映趋势正在扩散
- **创作者增长率**：有机传播程度，大量新创作者入场是强爆发信号
- **互动激增率**：单内容互动量 / 移动平均，捕捉"一条视频带飞整个趋势"的现象
- **播放集中度**：top-1 视频播放量 / 总播放量，越高说明流量越集中在爆款视频上

所有权重和阈值均在 `configs/config.yaml` 中配置，无需修改代码即可调参。

---

## 本地启动指南

**环境要求**：Go 1.22+

```bash
# 1. 克隆项目并安装依赖
git clone <repo>
cd trendpulse
go mod download

# 2. 启动 API Server（同时启动 Scheduler 定时计算）
make run
# 或：go run ./cmd/server
# Server 默认监听 http://localhost:8080

# 3. 另开一个终端，启动交互式模拟器
make simulate
# 或：go run ./cmd/simulator
```

模拟器启动后显示数据生成计划，按 **Enter** 逐批发送信号，输入 `all` 全量发送，输入 `q` 退出：

```
已生成种子数据计划:
  50 个趋势 × 96 批次 (4天 × 24小时)
  阶段分布: 15 viral_spike / 10 slow_burn / 15 steady_emerging / ...

[批次 1/96] t=2026-04-15 00:00 | 按 Enter 发送，'all' 全量，'q' 退出
>
✓ 已发送批次 1: 50 条信号
```

**验证数据**：

```bash
# 查看已摄入的趋势
curl http://localhost:8080/trends | jq .

# 查看爆发概率最高的趋势（Scheduler 运行后可用）
curl "http://localhost:8080/trends/rising?limit=10" | jq .

# 按阶段过滤
curl "http://localhost:8080/trends?phase=exploding" | jq .
```

**配置调参**：所有超参数（策略权重、调度间隔、信号窗口、模拟器分布）均在 `configs/config.yaml` 中，修改后重启服务生效。

---

## 如果时间更充裕，你会如何改进

**数据存储升级**

BadgerDB 是 MVP 的合理选择，但不支持复杂聚合查询。迁移路径已预留（`internal/repository/mongo/`、`internal/repository/postgres/`）：
- **MongoDB**：原生时序集合，聚合管道适合多维度趋势分析
- **PostgreSQL + TimescaleDB**：结构化数据 + 时序优化，适合多地区多平台数据关联查询

**实时计算**

当前 Scheduler 按固定间隔批量计算（最高延迟 = 调度间隔）。引入事件驱动架构（Kafka / NATS）后，每条信号写入即触发增量计算，可将延迟降至秒级，更适合捕捉 TikTok 的分钟级爆发窗口。

**ML 策略**

当前 `sigmoid_v1` 的权重是人工设定的。积累足够历史数据后，可以：
- 训练 **Gradient Boosting 分类器**预测爆发概率（有监督，标注历史爆发事件）
- 引入 **LSTM / Transformer** 捕捉时序模式中的非线性特征
- 新策略实现 `Strategy` 接口即可插入，无需修改其他代码

**多平台信号融合**

当前 Signal 模型面向 TikTok 设计。可扩展 `avg_engagement` 字段语义，引入跨平台权重，检测在 TikTok 爆发前已在 Instagram Reels / YouTube Shorts 出现苗头的趋势。

**认证与多租户**

Ingest 端点当前无认证。生产部署需在反向代理层（或 Middleware）加入 API Key 验证，并为不同数据源隔离数据空间。

**可观测性**

接入 Prometheus 暴露以下指标，配合 Grafana 可视化：
- 调度器执行延迟 / 失败率
- 各策略计算耗时
- 信号摄入速率 / 积压量
- 当前各阶段趋势分布（emerging/exploding/peaking/declining 占比）
