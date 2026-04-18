package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that unmarshals from a YAML string like "15s", "5m", "48h".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	s := value.Value
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

// ServerConfig holds HTTP server parameters.
type ServerConfig struct {
	Host         string   `yaml:"host"`
	Port         int      `yaml:"port"`
	ReadTimeout  Duration `yaml:"read_timeout"`
	WriteTimeout Duration `yaml:"write_timeout"`
}

// BadgerConfig holds BadgerDB/BadgerHold storage parameters.
type BadgerConfig struct {
	Dir      string `yaml:"dir"`
	ValueDir string `yaml:"value_dir"`
	InMemory bool   `yaml:"in_memory"`
}

// StorageConfig selects the storage backend and its configuration.
type StorageConfig struct {
	Type   string       `yaml:"type"`
	Badger BadgerConfig `yaml:"badger"`
}

// SchedulerConfig controls the periodic calculation scheduler.
type SchedulerConfig struct {
	Interval         Duration `yaml:"interval"`
	SignalLookback   Duration `yaml:"signal_lookback"`
	ActiveStrategies []string `yaml:"active_strategies"`
	ActiveStrategy   string   `yaml:"active_strategy"`
}

// WeightsConfig holds the score component weights for the default strategy.
type WeightsConfig struct {
	ViewAcceleration  float64 `yaml:"view_acceleration"`
	PostGrowthRate    float64 `yaml:"post_growth_rate"`
	CreatorGrowthRate float64 `yaml:"creator_growth_rate"`
	EngagementSurge   float64 `yaml:"engagement_surge"`
	ViewConcentration float64 `yaml:"view_concentration"`
}

// FeatureNormConfig holds the per-feature logistic normalization parameters.
type FeatureNormConfig struct {
	Center float64 `yaml:"center"`
	Scale  float64 `yaml:"scale"`
}

// FeatureNormsConfig holds normalization parameters for all five score features.
type FeatureNormsConfig struct {
	ViewAcceleration  FeatureNormConfig `yaml:"view_acceleration"`
	PostGrowthRate    FeatureNormConfig `yaml:"post_growth_rate"`
	CreatorGrowthRate FeatureNormConfig `yaml:"creator_growth_rate"`
	EngagementSurge   FeatureNormConfig `yaml:"engagement_surge"`
	ViewConcentration FeatureNormConfig `yaml:"view_concentration"`
}

// PhaseThresholdsConfig holds the phase classification thresholds.
type PhaseThresholdsConfig struct {
	RisingAccelThreshold      float64 `yaml:"rising_accel_threshold"`
	RisingEngagementThreshold float64 `yaml:"rising_engagement_threshold"`
	PeakingGrowthRateMax      float64 `yaml:"peaking_growth_rate_max"`
	PeakingGrowthRateMin      float64 `yaml:"peaking_growth_rate_min"`
}

// StrategyConfig holds configuration for a single strategy.
type StrategyConfig struct {
	Weights         WeightsConfig         `yaml:"weights"`
	FeatureNorms    FeatureNormsConfig    `yaml:"feature_norms"`
	Bias            float64               `yaml:"bias"`
	LookbackShort   Duration              `yaml:"lookback_short"`
	LookbackAccel   Duration              `yaml:"lookback_accel"`
	PhaseThresholds PhaseThresholdsConfig `yaml:"phase_thresholds"`
}

// CalculatorConfig holds configuration for the calculator layer.
type CalculatorConfig struct {
	DefaultStrategy StrategyConfig `yaml:"default_strategy"`
}

// APIConfig holds REST API tuning parameters.
type APIConfig struct {
	DefaultPageSize    int     `yaml:"default_page_size"`
	MaxPageSize        int     `yaml:"max_page_size"`
	RisingDefaultLimit int     `yaml:"rising_default_limit"`
	RisingMaxLimit     int     `yaml:"rising_max_limit"`
	RisingMinScore     float64 `yaml:"rising_min_score"`
}

// SimulatorDistributionConfig controls the mix of trend lifecycle types.
type SimulatorDistributionConfig struct {
	SteadyEmerging float64 `yaml:"steady_emerging"`
	ViralSpike     float64 `yaml:"viral_spike"`
	SlowBurn       float64 `yaml:"slow_burn"`
	AlreadyPeaking float64 `yaml:"already_peaking"`
	DecliningOnly  float64 `yaml:"declining_only"`
}

// SimulatorBaseParamsConfig holds baseline numeric parameters for data generation.
type SimulatorBaseParamsConfig struct {
	EmergingBaseUsage int `yaml:"emerging_base_usage"`
	RisingPeakUsage   int `yaml:"rising_peak_usage"`
	PeakingBaseViews  int `yaml:"peaking_base_views"`
}

// SimulatorGenerationConfig controls the seed data generation details.
type SimulatorGenerationConfig struct {
	TrendCount   int                         `yaml:"trend_count"`
	Days         int                         `yaml:"days"`
	NoisePct     float64                     `yaml:"noise_pct"`
	Distribution SimulatorDistributionConfig `yaml:"distribution"`
	BaseParams   SimulatorBaseParamsConfig   `yaml:"base_params"`
}

// SimulatorConfig holds all simulator-related configuration.
type SimulatorConfig struct {
	BaseURL    string                    `yaml:"base_url"`
	Generation SimulatorGenerationConfig `yaml:"generation"`
	Categories []string                  `yaml:"categories"`
	Regions    []string                  `yaml:"regions"`
	TrendTypes []string                  `yaml:"trend_types"`
}

// LoggingConfig controls log output.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Config is the top-level configuration structure for TrendPulse.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Storage    StorageConfig    `yaml:"storage"`
	Scheduler  SchedulerConfig  `yaml:"scheduler"`
	Calculator CalculatorConfig `yaml:"calculator"`
	API        APIConfig        `yaml:"api"`
	Simulator  SimulatorConfig  `yaml:"simulator"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// Load reads a YAML configuration file from path and returns a populated Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal yaml: %w", err)
	}

	return &cfg, nil
}
