package simulator

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// TrendSpec describes a synthetic trend to ingest.
type TrendSpec struct {
	ID          string
	Name        string
	Description string
	Categories  []string
	Source      string
}

// SignalData holds one synthetic hourly signal sample.
type SignalData struct {
	TrendID           string
	Timestamp         time.Time
	UsageCount        int64
	UniqueCreators    int64
	AvgViews          float64
	AvgEngagement     float64
	ViewConcentration float64
}

// SignalBatch groups all signals for one trend across one day, plus the
// AsOf timestamp (the latest signal in the batch) used to trigger calculation.
type SignalBatch struct {
	TrendID string
	Signals []SignalData // sorted ascending by Timestamp
	AsOf    time.Time    // latest timestamp in this batch
}

// GeneratorConfig controls synthetic data generation.
type GeneratorConfig struct {
	TrendCount   int
	Days         int
	NoisePercent float64

	// Growth pattern weights (must sum to 1.0)
	SteadyEmerging float64
	ViralSpike     float64
	SlowBurn       float64
	AlreadyPeaking float64
	DecliningOnly  float64

	// Base params
	EmergingBaseUsage int64
	RisingPeakUsage   int64
	PeakingBaseViews  float64

	Categories []string
	Sources    []string
}

type growthPattern int

const (
	patternSteadyEmerging growthPattern = iota
	patternViralSpike
	patternSlowBurn
	patternAlreadyPeaking
	patternDecliningOnly
)

// Generate creates trend specs and signal batches.
// Signals are produced hourly over cfg.Days days ending at endTime.
// One SignalBatch is produced per (trend, day) pair.
func Generate(cfg GeneratorConfig, endTime time.Time) ([]TrendSpec, []SignalBatch) {
	rng := rand.New(rand.NewSource(endTime.UnixNano()))

	// Build cumulative distribution for pattern selection.
	weights := []float64{
		cfg.SteadyEmerging,
		cfg.ViralSpike,
		cfg.SlowBurn,
		cfg.AlreadyPeaking,
		cfg.DecliningOnly,
	}
	cumulative := make([]float64, len(weights))
	sum := 0.0
	for i, w := range weights {
		sum += w
		cumulative[i] = sum
	}

	// Fallback defaults if weights are all zero.
	if sum == 0 {
		cfg.SteadyEmerging = 0.30
		cfg.ViralSpike = 0.25
		cfg.SlowBurn = 0.20
		cfg.AlreadyPeaking = 0.15
		cfg.DecliningOnly = 0.10
		weights = []float64{cfg.SteadyEmerging, cfg.ViralSpike, cfg.SlowBurn, cfg.AlreadyPeaking, cfg.DecliningOnly}
		sum = 0
		for i, w := range weights {
			sum += w
			cumulative[i] = sum
		}
	}

	categories := cfg.Categories
	if len(categories) == 0 {
		categories = []string{"general"}
	}
	sources := cfg.Sources
	if len(sources) == 0 {
		sources = []string{"hashtag"}
	}

	baseUsage := cfg.EmergingBaseUsage
	if baseUsage == 0 {
		baseUsage = 500
	}
	peakUsage := cfg.RisingPeakUsage
	if peakUsage == 0 {
		peakUsage = 50000
	}
	baseViews := cfg.PeakingBaseViews
	if baseViews == 0 {
		baseViews = 1_000_000
	}

	trends := make([]TrendSpec, 0, cfg.TrendCount)
	var batches []SignalBatch

	// Total hours in the simulation window.
	totalHours := cfg.Days * 24

	// Start time of simulation.
	startTime := endTime.Add(-time.Duration(totalHours) * time.Hour)

	for i := 0; i < cfg.TrendCount; i++ {
		id := fmt.Sprintf("trend-%04d", i+1)
		pattern := selectPattern(rng, cumulative, sum)

		// Assign 1-3 random categories.
		numCats := 1 + rng.Intn(3)
		cats := make([]string, numCats)
		for j := range cats {
			cats[j] = categories[rng.Intn(len(categories))]
		}
		source := sources[rng.Intn(len(sources))]

		spec := TrendSpec{
			ID:          id,
			Name:        fmt.Sprintf("Trend %04d", i+1),
			Description: fmt.Sprintf("Synthetic trend %d (%s)", i+1, patternName(pattern)),
			Categories:  cats,
			Source:      source,
		}
		trends = append(trends, spec)

		// Generate hourly signals for the full window, grouped by day.
		for day := 0; day < cfg.Days; day++ {
			dayBatch := SignalBatch{TrendID: id}
			for hour := 0; hour < 24; hour++ {
				absoluteHour := day*24 + hour
				t := startTime.Add(time.Duration(absoluteHour) * time.Hour)

				progress := float64(absoluteHour) / float64(totalHours-1)
				if totalHours <= 1 {
					progress = 1
				}

				usage, views, creators, engagement, concentration := computeSignal(
					pattern, progress, baseUsage, peakUsage, baseViews, rng, cfg.NoisePercent,
				)

				sig := SignalData{
					TrendID:           id,
					Timestamp:         t,
					UsageCount:        usage,
					UniqueCreators:    creators,
					AvgViews:          views,
					AvgEngagement:     engagement,
					ViewConcentration: concentration,
				}
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

// selectPattern picks a growth pattern based on cumulative weights.
func selectPattern(rng *rand.Rand, cumulative []float64, total float64) growthPattern {
	r := rng.Float64() * total
	for i, c := range cumulative {
		if r < c {
			return growthPattern(i)
		}
	}
	return patternSteadyEmerging
}

// computeSignal generates the five signal dimensions for a given pattern and progress [0,1].
func computeSignal(
	pattern growthPattern,
	progress float64,
	baseUsage, peakUsage int64,
	baseViews float64,
	rng *rand.Rand,
	noisePct float64,
) (usageCount int64, avgViews float64, uniqueCreators int64, avgEngagement float64, viewConcentration float64) {

	noise := func(v float64) float64 {
		if noisePct == 0 {
			return v
		}
		factor := 1.0 + (rng.Float64()*2-1)*noisePct
		return math.Max(0, v*factor)
	}

	switch pattern {
	case patternSteadyEmerging:
		// Linear growth ~5% per hour compounds to strong growth over the window.
		scale := progress
		u := noise(float64(baseUsage) * (1 + 3*scale))
		usageCount = int64(u)
		uniqueCreators = int64(noise(float64(baseUsage/10) * (1 + 2*scale)))
		avgViews = noise(baseViews * 0.1 * (1 + 2*scale))
		avgEngagement = noise(0.05 + 0.03*scale)
		viewConcentration = noise(0.3 + 0.2*scale)

	case patternViralSpike:
		// Flat then explosive growth in last 25% of the window.
		const spikeStart = 0.75
		if progress < spikeStart {
			u := noise(float64(baseUsage) * (1 + 0.5*progress))
			usageCount = int64(u)
			uniqueCreators = int64(noise(float64(baseUsage / 20)))
			avgViews = noise(baseViews * 0.05)
			avgEngagement = noise(0.04)
			viewConcentration = noise(0.2)
		} else {
			spikeProgress := (progress - spikeStart) / (1 - spikeStart)
			multiplier := math.Pow(float64(peakUsage)/float64(baseUsage), spikeProgress)
			u := noise(float64(baseUsage) * multiplier)
			usageCount = int64(u)
			uniqueCreators = int64(noise(float64(baseUsage/5) * multiplier))
			avgViews = noise(baseViews * 0.8 * spikeProgress)
			avgEngagement = noise(0.04 + 0.15*spikeProgress)
			viewConcentration = noise(0.5 + 0.4*spikeProgress)
		}

	case patternSlowBurn:
		// Very slow linear growth.
		scale := progress * 0.3
		u := noise(float64(baseUsage) * (1 + scale))
		usageCount = int64(u)
		uniqueCreators = int64(noise(float64(baseUsage/15) * (1 + scale*0.5)))
		avgViews = noise(baseViews * 0.08 * (1 + scale))
		avgEngagement = noise(0.03 + 0.01*scale)
		viewConcentration = noise(0.15 + 0.1*scale)

	case patternAlreadyPeaking:
		// Starts high, plateaus, slight decline in last quarter.
		var scale float64
		if progress < 0.75 {
			scale = 1.0 + 0.05*progress
		} else {
			scale = 1.05 * (1 - 0.1*(progress-0.75)/0.25)
		}
		u := noise(float64(peakUsage) * scale)
		usageCount = int64(u)
		uniqueCreators = int64(noise(float64(peakUsage/8) * scale))
		avgViews = noise(baseViews * scale)
		avgEngagement = noise(0.12 + 0.02*progress)
		viewConcentration = noise(0.6 + 0.1*progress)

	case patternDecliningOnly:
		// Starts at medium level, declines throughout.
		scale := 1 - 0.7*progress
		u := noise(float64(baseUsage) * 5 * scale)
		usageCount = int64(u)
		uniqueCreators = int64(noise(float64(baseUsage/4) * scale))
		avgViews = noise(baseViews * 0.4 * scale)
		avgEngagement = noise(0.08 * scale)
		viewConcentration = noise(0.4 * scale)
	}

	return
}

func patternName(p growthPattern) string {
	switch p {
	case patternSteadyEmerging:
		return "steady_emerging"
	case patternViralSpike:
		return "viral_spike"
	case patternSlowBurn:
		return "slow_burn"
	case patternAlreadyPeaking:
		return "already_peaking"
	case patternDecliningOnly:
		return "declining_only"
	default:
		return "unknown"
	}
}

// PatternName returns the human-readable name for a growth pattern.
func PatternName(p growthPattern) string {
	return patternName(p)
}

// HourlyBatch groups all trends' signals for a single hour.
type HourlyBatch struct {
	Index   int          // 0-based hour index
	AsOf    time.Time    // the hour timestamp
	Signals []SignalData // one signal per trend for this hour
}

// GroupByHour reorganizes per-trend SignalBatches into per-hour HourlyBatches.
func GroupByHour(batches []SignalBatch) []HourlyBatch {
	byTime := make(map[time.Time][]SignalData)
	for _, batch := range batches {
		for _, sig := range batch.Signals {
			hour := sig.Timestamp.Truncate(time.Hour)
			byTime[hour] = append(byTime[hour], sig)
		}
	}

	times := make([]time.Time, 0, len(byTime))
	for t := range byTime {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	result := make([]HourlyBatch, len(times))
	for i, t := range times {
		result[i] = HourlyBatch{
			Index:   i,
			AsOf:    t,
			Signals: byTime[t],
		}
	}
	return result
}

// PatternDistribution returns how many trends per pattern were generated.
func PatternDistribution(trends []TrendSpec) map[string]int {
	dist := make(map[string]int)
	for _, t := range trends {
		for _, p := range []string{"steady_emerging", "viral_spike", "slow_burn", "already_peaking", "declining_only"} {
			if strings.Contains(t.Description, p) {
				dist[p]++
				break
			}
		}
	}
	return dist
}
