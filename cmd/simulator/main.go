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
	trends, batches := simulator.Generate(genCfg, endTime)
	hourlyBatches := simulator.GroupByHour(batches)

	totalHours := genCfg.Days * 24
	startTime := endTime.Add(-time.Duration(totalHours) * time.Hour)

	patternDist := simulator.PatternDistribution(trends)

	// Print banner
	printBanner(len(trends), genCfg.Days, totalHours, startTime, endTime, patternDist, cfg.Simulator.BaseURL)

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
	fmt.Printf("    viral_spike     (爆发后快速衰退)   : %2d 个趋势\n", dist["viral_spike"])
	fmt.Printf("    slow_burn       (缓慢积累持久爆发) : %2d 个趋势\n", dist["slow_burn"])
	fmt.Printf("    steady_emerging (萌芽期稳定增长)   : %2d 个趋势\n", dist["steady_emerging"])
	fmt.Printf("    already_peaking (从高峰期开始)     : %2d 个趋势\n", dist["already_peaking"])
	fmt.Printf("    declining_only  (衰退中)           : %2d 个趋势\n", dist["declining_only"])
}
