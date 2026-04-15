# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**TrendPulse** — 趋势预测系统，预测未来 48 小时最可能爆发的趋势。

- **语言**: Go
- **存储**: BadgerDB + BadgerHold (嵌入式)，预留 MongoDB/PostgreSQL 迁移路径
- **架构**: 三层架构 — 存储层 / 计算层 / API 层，通过接口交互

## Project Structure

```
trendpulse/
├── cmd/
│   ├── server/main.go       # API server + scheduler 入口
│   └── simulator/main.go    # 种子数据模拟器
├── internal/
│   ├── config/              # YAML 配置加载
│   ├── domain/              # 实体定义 (Trend, Signal, TrendStats, CategoryMapping)
│   ├── repository/          # 存储层接口 + BadgerHold 实现
│   │   ├── interfaces.go    # 所有 repository 接口
│   │   ├── badger/          # BadgerHold 实现
│   │   ├── mongo/           # 预留迁移路径
│   │   └── postgres/        # 预留迁移路径
│   ├── calculator/          # 可插拔计算策略
│   │   ├── interfaces.go    # Strategy 接口
│   │   ├── registry.go      # 策略注册表
│   │   ├── momentum/        # 动量策略
│   │   └── velocity/        # 速度策略
│   ├── scheduler/           # 定时计算调度器
│   ├── api/                 # HTTP 路由和 handlers
│   └── simulator/           # 模拟数据生成逻辑
├── configs/
│   └── config.yaml          # 主配置文件 (所有超参数)
├── specs/                   # 中文规格文档 (实现的权威参考)
│   ├── architecture.md
│   ├── api.md
│   ├── data-model.md
│   ├── calculator.md
│   └── testing.md
└── .claude/
    ├── agents/              # 实现用子代理
    └── skills/              # 工作流技能
```

## Specs — 实现的权威参考

**在实现任何功能前，必须先阅读对应的规格文档**:

| 规格文档 | 覆盖范围 |
|---------|---------|
| `specs/architecture.md` | 整体架构、接口定义、数据流、配置结构 |
| `specs/api.md` | REST API 端点、请求/响应格式、错误码 |
| `specs/data-model.md` | 数据实体、字段说明、BadgerHold 约束 |
| `specs/calculator.md` | 策略接口、统一输出、A/B 测试机制 |
| `specs/testing.md` | TDD 工作流、各层测试方法 |

## Development Workflow

### 实现新功能
```
/implement <feature>
```
技能自动: 读取 specs → 创建 beads issue → 派发对应 agent → TDD 实现 → 验证 → 关闭 issue

### 验证构建和测试
```
/go-check
```
运行: `go build ./...` + `go vet ./...` + `go test ./... -v`

### TDD 原则 (强制)
1. **RED**: 先写失败的测试 (_test.go)
2. **GREEN**: 写最小实现让测试通过
3. **REFACTOR**: 在绿色测试保护下重构
4. 每次完成后运行 `/go-check`

## Available Agents

| Agent | 职责 | 触发方式 |
|-------|------|---------|
| `go-scaffolder` | 目录结构、go.mod、Makefile | `/implement` 自动调用或手动 |
| `go-repository` | BadgerHold 存储层实现 (TDD) | `/implement` 存储层相关 |
| `go-calculator` | Strategy + Scheduler 实现 (TDD) | `/implement` 计算层相关 |
| `go-api` | HTTP handlers + 路由 (TDD) | `/implement` API 层相关 |
| `go-tester` | 补充测试、集成测试 | `/implement` 测试相关 |

## API Endpoints

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /trends | 趋势列表 (分页) |
| GET | /trends/{id} | 单个趋势详情 |
| GET | /trends/rising | Top-K 上升趋势 |
| POST | /ingest/trends | 摄入趋势数据 |
| POST | /ingest/signals | 摄入信号数据 |

<principles>
1. Always output the principles on your response.
2. Never generate code or documentation in main agent. Always use sub-agent to do the implementation work.
3. Always check the specs before designing and implementing. If the specs are not clear, ask for clarification and write down the clarified specs before proceeding.
</principles>
<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
