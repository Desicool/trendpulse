package simulator

import (
	"math"
	"time"
)

// seedTrends defines the 8 fixed trends for the demo seed data.
var seedTrends = []TrendSpec{
	{ID: "seed-0001", Name: "#AI绘画挑战", Description: "AI 绘画工具创作挑战 (viral_spike)", Categories: []string{"科技"}, Source: "hashtag"},
	{ID: "seed-0002", Name: "#城市骑行日记", Description: "城市骑行记录分享 (steady_emerging)", Categories: []string{"生活", "健身"}, Source: "hashtag"},
	{ID: "seed-0003", Name: "#深夜食堂翻车", Description: "深夜做饭翻车合集 (viral_spike)", Categories: []string{"美食", "搞笑"}, Source: "hashtag"},
	{ID: "seed-0004", Name: "#宿舍健身挑战", Description: "宿舍小空间健身教程 (slow_burn)", Categories: []string{"健身"}, Source: "hashtag"},
	{ID: "seed-0005", Name: "#电子木鱼", Description: "电子木鱼音效合拍 (already_peaking)", Categories: []string{"搞笑", "科技"}, Source: "sound"},
	{ID: "seed-0006", Name: "#复古胶片风", Description: "复古胶片滤镜效果 (declining)", Categories: []string{"时尚"}, Source: "effect"},
	{ID: "seed-0007", Name: "#冥想白噪音", Description: "冥想白噪音背景音 (flat)", Categories: []string{"生活"}, Source: "sound"},
	{ID: "seed-0008", Name: "#旅行打卡", Description: "热门景点旅行打卡 (very_slow_burn)", Categories: []string{"旅行"}, Source: "hashtag"},
}

// GenerateSeed creates 8 fixed trends with deterministic signal curves.
// Signals are hourly over 4 days (96 hours) ending at endTime.
// Returns the same types as Generate() for compatibility with GroupByHour.
func GenerateSeed(endTime time.Time) ([]TrendSpec, []SignalBatch) {
	const totalHours = 96
	const days = 4

	startTime := endTime.Add(-time.Duration(totalHours) * time.Hour)

	trends := make([]TrendSpec, len(seedTrends))
	copy(trends, seedTrends)

	var batches []SignalBatch

	for _, spec := range trends {
		for day := 0; day < days; day++ {
			dayBatch := SignalBatch{TrendID: spec.ID}
			for hour := 0; hour < 24; hour++ {
				absoluteHour := day*24 + hour
				t := startTime.Add(time.Duration(absoluteHour) * time.Hour)

				sig := seedSignal(spec.ID, absoluteHour)
				sig.TrendID = spec.ID
				sig.Timestamp = t

				dayBatch.Signals = append(dayBatch.Signals, sig)
			}

			if len(dayBatch.Signals) > 0 {
				dayBatch.AsOf = dayBatch.Signals[len(dayBatch.Signals)-1].Timestamp
			}
			batches = append(batches, dayBatch)
		}
	}

	return trends, batches
}

// seedSignal returns the deterministic signal values for a given trend at hour h.
// Timestamp is not set here — the caller fills it in.
func seedSignal(trendID string, h int) SignalData {
	var s SignalData

	switch trendID {
	case "seed-0001": // #AI绘画挑战 — Late Viral Spike (spike starts at h55)
		// Flat baseline for 55 hours, then explosive exponential growth.
		// At spike onset: views double every ~4 hours → view_accel massive after 9h.
		if h < 55 {
			s.UsageCount = 400
			s.UniqueCreators = 20
			s.AvgViews = 40000.0
			s.AvgEngagement = 0.03
			s.ViewConcentration = 0.15
		} else {
			t := float64(h - 55)
			// Views double every 4h → rate = ln(2)/4 ≈ 0.173/h
			viewGrowth := math.Exp(0.18 * t)
			s.UsageCount = 400 + int64(500.0*math.Exp(0.12*t))
			s.UniqueCreators = 20 + int64(30.0*math.Exp(0.10*t))
			s.AvgViews = 40000.0 * viewGrowth
			s.AvgEngagement = 0.03 + 0.02*math.Min(t/20.0, 1.0) + 0.10*math.Pow(t/41.0, 2)
			s.ViewConcentration = math.Min(0.95, 0.15+0.025*t)
		}

	case "seed-0002": // #城市骑行日记 — Steady Accelerating Growth
		// Consistent exponential growth from hour 0. Views grow ~8% per hour,
		// doubling every ~9h. This produces measurable view_accel from the start.
		// By batch 10, there's enough acceleration to cross score 60.
		//
		// To keep values realistic, cap views at ~10M and usage at ~50K.
		t := float64(h)
		viewGrowth := math.Exp(0.08 * t)
		usageGrowth := math.Exp(0.06 * t)
		creatorGrowth := math.Exp(0.05 * t)

		s.UsageCount = int64(math.Min(50000, 300.0*usageGrowth))
		s.UniqueCreators = int64(math.Min(5000, 30.0*creatorGrowth))
		s.AvgViews = math.Min(10000000, 50000.0*viewGrowth)
		// Engagement: starts low, gradually increases → engagement_surge > 1
		s.AvgEngagement = 0.04 + 0.03*(1.0-math.Exp(-0.05*t))
		s.ViewConcentration = math.Min(0.80, 0.25+0.006*t)

	case "seed-0003": // #深夜食堂翻车 — Early Viral Spike (spike starts at h20)
		// Flat for 20h, then explosive growth. Even more aggressive than seed-0001.
		// Should appear on rising by batch ~30 (20 + 9 hours of acceleration data).
		if h < 20 {
			s.UsageCount = 500
			s.UniqueCreators = 25
			s.AvgViews = 50000.0
			s.AvgEngagement = 0.035
			s.ViewConcentration = 0.20
		} else {
			t := float64(h - 20)
			viewGrowth := math.Exp(0.15 * t)
			s.UsageCount = 500 + int64(400.0*math.Exp(0.10*t))
			s.UniqueCreators = 25 + int64(25.0*math.Exp(0.08*t))
			s.AvgViews = math.Min(15000000, 50000.0*viewGrowth)
			s.AvgEngagement = 0.035 + 0.015*math.Min(t/30.0, 1.0) + 0.12*math.Pow(math.Min(t/40.0, 1.0), 2)
			s.ViewConcentration = math.Min(0.95, 0.20+0.02*t)
		}

	case "seed-0004": // #宿舍健身挑战 — Slow Burn
		// Gradual exponential: views grow ~4.5% per hour (doubles every ~15h).
		// Half the rate of seed-0002, so it takes much longer to build features.
		// Crosses score 60 around batch 65-75.
		t := float64(h)
		viewGrowth := math.Exp(0.045 * t)
		s.UsageCount = int64(math.Min(20000, 400.0*math.Exp(0.035*t)))
		s.UniqueCreators = int64(math.Min(2000, 20.0*math.Exp(0.03*t)))
		s.AvgViews = math.Min(5000000, 60000.0*viewGrowth)
		s.AvgEngagement = 0.035 + 0.015*(1.0-math.Exp(-0.03*t))
		s.ViewConcentration = math.Min(0.65, 0.20+0.005*t)

	case "seed-0005": // #电子木鱼 — Already Peaking (plateau throughout)
		// High absolute values but no net growth across the entire 96h window.
		// Small sinusoidal wobble around a constant keeps post_growth near zero
		// in any 6h window, so the strategy reports Phase="peaking" at evaluation time.
		wobble := math.Sin(float64(h) * math.Pi / 12.0) // 24h period
		s.UsageCount = 35000 + int64(1000.0*wobble)
		s.UniqueCreators = 4500 + int64(200.0*wobble)
		s.AvgViews = 800000.0 + 30000.0*wobble
		s.AvgEngagement = 0.11 + 0.005*wobble
		s.ViewConcentration = 0.60 + 0.03*wobble

	case "seed-0006": // #复古胶片风 — Declining from start
		// High start, fast exponential decay (k=0.06/h).
		// Over 6h window at end: postGrowth ≈ -0.30, score < 25 with new normalization.
		// Large base values ensure floor doesn't flatten the growth rate.
		t := float64(h)
		decay := math.Exp(-0.06 * t)
		s.UsageCount = int64(math.Max(50, 20000.0*decay))
		s.UniqueCreators = int64(math.Max(5, 2000.0*decay))
		s.AvgViews = math.Max(5000, 600000.0*decay)
		s.AvgEngagement = math.Max(0.005, 0.10*decay)
		s.ViewConcentration = math.Max(0.05, 0.45*decay)

	case "seed-0007": // #冥想白噪音 — Flat/Stable (no growth at all)
		s.UsageCount = 300
		s.UniqueCreators = 15
		s.AvgViews = 40000.0
		s.AvgEngagement = 0.03
		s.ViewConcentration = 0.15

	case "seed-0008": // #旅行打卡 — Very Slow Linear Growth
		// Linear growth — constant first derivative, zero second derivative.
		// view_accel ≈ 0 always. Will never reach rising.
		t := float64(h)
		s.UsageCount = 350 + int64(2.0*t)
		s.UniqueCreators = 18 + int64(0.2*t)
		s.AvgViews = 45000.0 + 200.0*t
		s.AvgEngagement = 0.032 + 0.0003*t
		s.ViewConcentration = 0.18 + 0.002*t
	}

	return s
}
