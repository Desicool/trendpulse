# TrendPulse MVP 快速启动规格

## 概述

MVP 由两个独立进程组成：
1. **API Server**（含 Scheduler）：提供 REST API，定时执行趋势计算
2. **模拟器**（Simulator）：生成符合 TikTok 行为特征的种子数据，通过交互式 CLI 按批次投递

---

## 运行模型

```
┌─────────────────────┐          ┌──────────────────────────┐
│   cmd/simulator     │          │      cmd/server           │
│                     │  HTTP    │                           │
│  交互式 CLI         │─POST────▶│  POST /ingest/trends      │
│  按批次投递数据     │  /ingest │  POST /ingest/signals     │
│                     │          │                           │
└─────────────────────┘          │  GET /trends              │
                                 │  GET /trends/rising       │
                                 │  GET /trends/{id}         │
                                 │                           │
                                 │  [Scheduler goroutine]    │
                                 │  每 N 分钟计算 TrendStats  │
                                 └──────────────────────────┘
```

---

## 模拟器交互设计

### 启动输出

模拟器启动时，先生成完整的种子数据计划，打印摘要，然后进入交互等待：

```
$ go run ./cmd/simulator

正在生成种子数据...

═══════════════════════════════════════════════════════
  TrendPulse 模拟器
═══════════════════════════════════════════════════════
  趋势总数  : 50
  模拟时长  : 4 天（96 批次，每批次 = 1 小时）
  时间起点  : 2026-04-11 00:00 UTC
  时间终点  : 2026-04-14 23:00 UTC

  曲线分布:
    viral_spike     (爆发后快速衰退)   : 13 个趋势
    slow_burn       (缓慢积累持久爆发) : 10 个趋势
    steady_emerging (萌芽期稳定增长)   : 15 个趋势
    already_peaking (从高峰期开始)     :  7 个趋势
    declining_only  (衰退中)           :  5 个趋势

  阶段覆盖 (最终批次):
    emerging  : 15 个  peaking  :  7 个
    exploding : 13 个  declining: 15 个

  目标 Server : http://localhost:8080
═══════════════════════════════════════════════════════

[批次 1/96] t=Day1 00:00 (2026-04-11 00:00)
按 Enter 发送，输入 'all' 全量发送，'q' 退出
>
```

### 批次发送交互

```
> [Enter]
✓ 批次 1 已发送: 50 条信号 | t=2026-04-11 00:00 | 耗时 120ms

[批次 2/96] t=Day1 01:00 (2026-04-11 01:00)
按 Enter 发送，'all' 全量，'q' 退出
>
```

### 命令说明

| 输入 | 行为 |
|------|------|
| `Enter`（直接回车） | 发送当前批次，推进到下一批次 |
| `all` | 发送剩余所有批次，自动完成 |
| `status` | 打印当前各趋势的阶段分布统计 |
| `q` / `quit` | 退出模拟器（已发送的数据保留） |

### 全量发送模式

```
> all
正在发送剩余 94 批次...
  批次 3/96  ✓  批次 4/96  ✓  批次 5/96  ✓  ...
  批次 96/96 ✓

全部完成！共发送 4800 条信号（50 趋势 × 96 批次）
```

---

## 种子数据生成规格

### 时间线设计

- **总时长**：4 天 = 96 小时 = 96 批次
- **每批次**：为每个 trend 生成 1 条 Signal，timestamp = 模拟起始时间 + batch_index × 1h
- **模拟起始时间**：运行时取 `now - 4d`，使数据落在 Scheduler 的 `signal_lookback` 窗口内

### 趋势曲线类型

每个 Trend 在生成时被分配一种曲线类型，决定其 96 小时内各信号字段的数值走向：

#### 1. `viral_spike`（爆发后快速衰退）
- **Phase 路径**：emerging (0–23h) → exploding (24–47h) → peaking (48–59h) → declining (60–96h)
- **信号特征**：
  - 0–23h：`usage_count` 缓慢线性增长，`avg_engagement` 低位平稳
  - 24–47h：`avg_views` 指数级加速（加速度 > 0.8），`avg_engagement` 激增（surge > 0.5），`view_concentration` 从 0.1 升至 0.8
  - 48–59h：`usage_count` 增长率回落至 ±2%，各绝对值维持高位
  - 60–96h：所有指标环比下降，`view_concentration` 回落

#### 2. `slow_burn`（缓慢积累后持久爆发）
- **Phase 路径**：emerging (0–47h) → exploding (48–71h) → peaking (72–96h)
- **信号特征**：
  - 0–47h：更缓慢的线性增长，积累期更长
  - 48–71h：爆发，加速度稍低于 viral_spike（加速度 > 0.5），但更持久
  - 72–96h：高峰期持续到模拟结束，不出现衰退

#### 3. `steady_emerging`（萌芽期稳定增长，未爆发）
- **Phase 路径**：emerging 全程（0–96h）
- **信号特征**：`usage_count` 线性增长（斜率小），`avg_views` 缓慢增长，`avg_engagement` 低，`view_concentration` 持续 < 0.15，无明显加速

#### 4. `already_peaking`（从高峰期开始）
- **Phase 路径**：peaking (0–47h) → declining (48–96h)
- **信号特征**：
  - 0–47h：所有指标绝对值高位，`usage_count` 增长率 ≈ 0
  - 48–96h：开始全面负增长

#### 5. `declining_only`（衰退中的趋势）
- **Phase 路径**：declining 全程（0–96h）
- **信号特征**：各维度持续负增长，`view_concentration` 低，`avg_engagement` 持续下滑

### 信号字段生成细则

各字段需满足 Phase 特征，使 `sigmoid_v1` 策略的 Phase 判定与预期一致：

| Phase | `usage_count` 增长率 | `avg_views` 加速度 | `avg_engagement` surge | `view_concentration` |
|-------|---------------------|-------------------|----------------------|---------------------|
| emerging | 0%–5% / 小时 | ≈ 0 | ≤ 1.0 倍均值 | < 0.20 |
| exploding | 15%–50% / 小时 | > 0.5 | > 1.3 倍均值 | 0.50–0.85 |
| peaking | -2%–+5% / 小时 | ≈ 0 | 1.0–1.1 倍均值 | 0.20–0.40 |
| declining | < -5% / 小时 | < 0 | < 1.0 倍均值（下降） | < 0.20 |

每个字段添加 **±noise_pct** 随机噪声（默认 10%），使数据更真实。

### 初始基准值（从 config base_params 读取）

各曲线类型的初始 `usage_count`（即 batch 0 的值）：

| 曲线类型 | 初始 usage_count |
|---------|----------------|
| steady_emerging | `emerging_base_usage`（默认 500）|
| viral_spike | `emerging_base_usage` × 1.5 |
| slow_burn | `emerging_base_usage` × 0.8 |
| already_peaking | `exploding_peak_usage` × 0.9 |
| declining_only | `exploding_peak_usage` × 0.7 |

---

## 配置参数（configs/config.yaml）

```yaml
simulator:
  base_url: "http://localhost:8080"
  generation:
    trend_count: 50          # 生成趋势总数
    days: 4                  # 模拟天数（每天 24 批次）
    noise_pct: 0.10          # 信号随机噪声比例（± noise_pct × 值）
    distribution:            # 各曲线类型占比（之和应为 1.0）
      steady_emerging: 0.30
      viral_spike:     0.25
      slow_burn:       0.20
      already_peaking: 0.15
      declining_only:  0.10
    base_params:
      emerging_base_usage:     500      # 萌芽期初始 post 数/小时
      exploding_peak_usage:    50000    # 爆发期峰值 post 数/小时
      peaking_base_views:      1000000  # 高峰期平均播放量
  categories:
    - "时尚"
    - "生活"
    - "美食"
    - "旅行"
    - "科技"
    - "健身"
    - "音乐"
    - "搞笑"
  regions:
    - "cn"
    - "us"
    - "jp"
    - "kr"
  trend_types:
    - "hashtag"
    - "sound"
    - "effect"
```

**调参建议**：
- 增加 `trend_count` 可测试大数据量下的查询性能
- 调整 `distribution` 比例可控制各阶段趋势的数量，用于测试 Phase 过滤
- 降低 `noise_pct` 至 0 可生成"完美曲线"数据，便于验证算法正确性
- 修改 `base_params` 可调整数值量级，验证 Score 归一化是否稳健

---

## 完整运行示例

```bash
# 终端 1：启动 Server
$ go run ./cmd/server
{"level":"info","msg":"server starting","addr":"0.0.0.0:8080"}
{"level":"info","msg":"scheduler started","interval":"5m"}

# 终端 2：启动模拟器，发送前 3 批
$ go run ./cmd/simulator
[批次 1/96] > [Enter]  ✓ 已发送批次 1
[批次 2/96] > [Enter]  ✓ 已发送批次 2
[批次 3/96] > all
正在发送剩余 94 批次... 完成！

# 终端 2：验证
$ curl -s http://localhost:8080/trends/rising | jq '.data.items[0].stats.score'
87.3

$ curl -s "http://localhost:8080/trends?phase=exploding&limit=5" | jq '.data.items | length'
13
```
