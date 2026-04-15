package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"trendpulse/internal/config"
	"trendpulse/internal/simulator"
)

func main() {
	cfgPath := flag.String("config", "configs/config.yaml", "config file path")
	dryRun := flag.Bool("dry-run", false, "print data without sending")
	seed := flag.Bool("seed", true, "use fixed seed data for reproducible demo")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	gen := cfg.Simulator.Generation
	dist := gen.Distribution
	base := gen.BaseParams

	genCfg := simulator.GeneratorConfig{
		TrendCount:   gen.TrendCount,
		Days:         gen.Days,
		NoisePercent: gen.NoisePct,

		SteadyEmerging: dist.SteadyEmerging,
		ViralSpike:     dist.ViralSpike,
		SlowBurn:       dist.SlowBurn,
		AlreadyPeaking: dist.AlreadyPeaking,
		DecliningOnly:  dist.DecliningOnly,

		EmergingBaseUsage: int64(base.EmergingBaseUsage),
		RisingPeakUsage:   int64(base.RisingPeakUsage),
		PeakingBaseViews:  float64(base.PeakingBaseViews),

		Categories: cfg.Simulator.Categories,
		Sources:    cfg.Simulator.TrendTypes,
	}

	endTime := time.Now().UTC()

	var trends []simulator.TrendSpec
	var batches []simulator.SignalBatch

	if *seed {
		trends, batches = simulator.GenerateSeed(endTime)
	} else {
		trends, batches = simulator.Generate(genCfg, endTime)
	}
	hourlyBatches := simulator.GroupByHour(batches)

	totalHours := len(hourlyBatches)
	days := totalHours / 24
	if totalHours%24 != 0 {
		days++
	}
	startTime := endTime.Add(-time.Duration(totalHours) * time.Hour)

	patternDist := simulator.PatternDistribution(trends)

	// Print banner
	if *seed {
		printSeedBanner(trends, totalHours, days, startTime, endTime, patternDist, cfg.Simulator.BaseURL)
	} else {
		printBanner(len(trends), days, totalHours, startTime, endTime, patternDist, cfg.Simulator.BaseURL)
	}

	if *dryRun {
		fmt.Println("  (dry-run 模式，不发送数据)")
		return
	}

	client := simulator.NewClient(cfg.Simulator.BaseURL)
	ctx := context.Background()

	// Ingest all trends first
	fmt.Printf("\n正在摄入 %d 个趋势...\n", len(trends))
	for _, trend := range trends {
		if err := client.IngestTrend(ctx, trend); err != nil {
			slog.Warn("failed to ingest trend", "id", trend.ID, "error", err)
		}
	}
	fmt.Printf("✓ %d 个趋势已摄入\n\n", len(trends))

	// Interactive loop
	scanner := bufio.NewScanner(os.Stdin)
	i := 0
	for i < len(hourlyBatches) {
		hb := hourlyBatches[i]
		dayNum := i/24 + 1
		hourInDay := i % 24
		fmt.Printf("[批次 %d/%d] t=Day%d %02d:00 (%s)\n",
			i+1, len(hourlyBatches), dayNum, hourInDay,
			hb.AsOf.Format("2006-01-02 15:04"))
		fmt.Printf("按 Enter 发送，输入 'all' 全量发送，'status' 查看分布，'q' 退出\n> ")

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		switch input {
		case "q", "quit":
			fmt.Println("退出模拟器。")
			return
		case "all":
			// Send all remaining batches
			for j := i; j < len(hourlyBatches); j++ {
				rb := hourlyBatches[j]
				if err := sendHourlyBatch(ctx, client, rb); err != nil {
					fmt.Printf("✗ 批次 %d 发送失败: %v\n", j+1, err)
					continue
				}
				fmt.Printf("✓ 批次 %d/%d 已发送: %d 条信号 | t=%s\n",
					j+1, len(hourlyBatches), len(rb.Signals),
					rb.AsOf.Format("2006-01-02 15:04"))
			}
			fmt.Println("\n全部批次发送完成。")
			return
		case "status":
			fmt.Println("\n  曲线分布:")
			printPatternDist(patternDist)
			fmt.Println()
			continue // re-prompt same batch (i not incremented)
		default:
			// Enter or anything else: send this batch
			start := time.Now()
			if err := sendHourlyBatch(ctx, client, hb); err != nil {
				fmt.Printf("✗ 批次 %d 发送失败: %v\n", i+1, err)
				i++
				continue
			}
			elapsed := time.Since(start)
			fmt.Printf("✓ 批次 %d 已发送: %d 条信号 | t=%s | 耗时 %dms\n\n",
				i+1, len(hb.Signals),
				hb.AsOf.Format("2006-01-02 15:04"),
				elapsed.Milliseconds())
			i++
		}
	}

	fmt.Println("全部批次发送完成。")
}

func sendHourlyBatch(ctx context.Context, client *simulator.Client, hb simulator.HourlyBatch) error {
	// Build a SignalBatch per trend from the hourly signals
	byTrend := make(map[string][]simulator.SignalData)
	for _, sig := range hb.Signals {
		byTrend[sig.TrendID] = append(byTrend[sig.TrendID], sig)
	}

	for trendID, signals := range byTrend {
		batch := simulator.SignalBatch{
			TrendID: trendID,
			Signals: signals,
			AsOf:    hb.AsOf,
		}
		if err := client.IngestSignalBatch(ctx, batch); err != nil {
			return fmt.Errorf("ingest signals for %s: %w", trendID, err)
		}
	}

	// Trigger calculation once for this hour
	if err := client.TriggerCalculation(ctx, hb.AsOf); err != nil {
		return fmt.Errorf("trigger calculation: %w", err)
	}
	return nil
}

func printBanner(trendCount, days, totalHours int, startTime, endTime time.Time, patternDist map[string]int, baseURL string) {
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  TrendPulse 模拟器")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  趋势总数  : %d\n", trendCount)
	fmt.Printf("  模拟时长  : %d 天（%d 批次，每批次 = 1 小时）\n", days, totalHours)
	fmt.Printf("  时间起点  : %s\n", startTime.Format("2006-01-02 15:04 MST"))
	fmt.Printf("  时间终点  : %s\n", endTime.Add(-time.Hour).Format("2006-01-02 15:04 MST"))
	fmt.Println()
	fmt.Println("  曲线分布:")
	printPatternDist(patternDist)
	fmt.Println()
	fmt.Printf("  目标 Server : %s\n", baseURL)
	fmt.Println("═══════════════════════════════════════════════════════")
}

func printPatternDist(dist map[string]int) {
	// Pre-formatted lines to avoid CJK alignment issues
	for pat, count := range dist {
		fmt.Printf("    %-20s: %2d 个趋势\n", pat, count)
	}
}

func printSeedBanner(trends []simulator.TrendSpec, totalHours, days int, startTime, endTime time.Time, patternDist map[string]int, baseURL string) {
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  TrendPulse 模拟器 — Seed 模式")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  趋势总数  : %d\n", len(trends))
	fmt.Printf("  模拟时长  : %d 天（%d 批次，每批次 = 1 小时）\n", days, totalHours)
	fmt.Printf("  时间起点  : %s\n", startTime.Format("2006-01-02 15:04 MST"))
	fmt.Printf("  时间终点  : %s\n", endTime.Add(-time.Hour).Format("2006-01-02 15:04 MST"))
	fmt.Println()
	fmt.Println("  曲线分布:")
	printPatternDist(patternDist)
	fmt.Println()
	fmt.Println("  Seed 趋势:")
	seedPatterns := map[string]string{
		"seed-0001": "viral_spike (h55 起爆)",
		"seed-0002": "steady_emerging",
		"seed-0003": "viral_spike (h20 起爆)",
		"seed-0004": "slow_burn",
		"seed-0005": "already_peaking",
		"seed-0006": "declining",
		"seed-0007": "flat",
		"seed-0008": "very_slow_burn",
	}
	for _, t := range trends {
		pat := seedPatterns[t.ID]
		fmt.Printf("    %-10s %-16s %s\n", t.ID, t.Name, pat)
	}
	fmt.Println()
	fmt.Println("  预期 Rising 时间线:")
	fmt.Println("    批次 ~10  : #城市骑行日记 (指数增长从 h0 开始，最先达到阈值)")
	fmt.Println("    批次 ~30  : #深夜食堂翻车 (h20 起爆，9h 后可计算加速度)")
	fmt.Println("    批次 ~65  : #AI绘画挑战 (h55 起爆，9h 后可计算加速度)")
	fmt.Println("    批次 ~70+ : #宿舍健身挑战 (缓慢燃烧，可能勉强触及阈值)")
	fmt.Println()
	fmt.Printf("  目标 Server : %s\n", baseURL)
	fmt.Println("═══════════════════════════════════════════════════════")
}
