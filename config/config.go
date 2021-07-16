package config

import (
	"encoding/json"
	"os"
	"time"

	"github.com/pkg/errors"
)

const (
	defaultIngressClassName = "globo-networkapi"
)

type Config struct {
	LogLevel                 int
	NetworkAPIURL            string
	NetworkAPIUsername       string
	NetworkAPIPassword       string
	ClusterName              string
	IngressClassName         string
	PodNetworkID             int
	LBNetworkID              int
	ReconcileInterval        time.Duration
	Equipment                EquipmentConfig
	DefaultVIPEnvironmentID  int
	DefaultPoolEnvironmentID int
	DefaultCacheGroupID      int
	DefaultTrafficReturnID   int
	DefaultTimeoutID         int
	DefaultPersistenceID     int
	DefaultVIPL7RuleID       int
	DefaultVIPL4ProtocolID   int
	DefaultVIPL7ProtocolID   int
	DebugReconcileOnce       bool
	DebugDisableCleanup      bool
}

type EquipmentConfig struct {
	Type        int
	Model       int
	Group       int
	Environment int
}

func (cfg Config) validate() error {
	if cfg.ClusterName == "" {
		return errors.New("clusterName cannot be empty")
	}
	if cfg.PodNetworkID == 0 {
		return errors.New("podNetworkID cannot be empty")
	}
	if cfg.LBNetworkID == 0 {
		return errors.New("lbNetworkID cannot be empty")
	}
	if cfg.ReconcileInterval < 1*time.Minute {
		return errors.New("reconcileInterval cannot be less than 1 minute")
	}
	if cfg.NetworkAPIURL == "" {
		return errors.New("networkAPIURL cannot be empty")
	}
	if cfg.DefaultVIPEnvironmentID == 0 {
		return errors.New("defaultVIPEnvironmentID cannot be empty")
	}
	if cfg.DefaultPoolEnvironmentID == 0 {
		return errors.New("defaultPoolEnvironmentID cannot be empty")
	}
	if cfg.DefaultCacheGroupID == 0 {
		return errors.New("defaultCacheGroupID cannot be empty")
	}
	if cfg.DefaultTrafficReturnID == 0 {
		return errors.New("defaultTrafficReturnID cannot be empty")
	}
	if cfg.DefaultTimeoutID == 0 {
		return errors.New("defaultTimeoutID cannot be empty")
	}
	if cfg.DefaultPersistenceID == 0 {
		return errors.New("defaultPersistenceID cannot be empty")
	}
	if cfg.DefaultVIPL7RuleID == 0 {
		return errors.New("defaultVIPL7RuleID cannot be empty")
	}
	if cfg.DefaultVIPL4ProtocolID == 0 {
		return errors.New("defaultVIPL4ProtocolID cannot be empty")
	}
	if cfg.DefaultVIPL7ProtocolID == 0 {
		return errors.New("defaultVIPL7ProtocolID cannot be empty")
	}
	if cfg.Equipment.Type == 0 {
		return errors.New("equipment.type cannot be empty")
	}
	if cfg.Equipment.Model == 0 {
		return errors.New("equipment.model cannot be empty")
	}
	if cfg.Equipment.Group == 0 {
		return errors.New("equipment.group cannot be empty")
	}
	if cfg.Equipment.Environment == 0 {
		return errors.New("equipment.environment cannot be empty")
	}
	return nil
}

func setDefaults(cfg *Config) {
	if cfg.ReconcileInterval == 0 {
		cfg.ReconcileInterval = 5 * time.Minute
	}
	if cfg.IngressClassName == "" {
		cfg.IngressClassName = defaultIngressClassName
	}
}

func Get() (Config, error) {
	var cfg Config
	if len(os.Args) < 2 {
		return cfg, errors.New("required config file argument")
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, err
	}
	setDefaults(&cfg)
	err = cfg.validate()
	return cfg, err
}
