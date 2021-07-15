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
	NetworkAPIURL           string
	NetworkAPIUsername      string
	NetworkAPIPassword      string
	ClusterName             string
	IngressClassName        string
	DefaultVIPEnvironmentID int
	PodNetworkID            int
	ReconcileInterval       time.Duration
}

func (cfg Config) validate() error {
	if cfg.ClusterName == "" {
		return errors.New("clusterName cannot be empty")
	}
	if cfg.DefaultVIPEnvironmentID == 0 {
		return errors.New("defaultVIPEnvironmentID cannot be empty")
	}
	if cfg.PodNetworkID == 0 {
		return errors.New("podNetworkID cannot be empty")
	}
	if cfg.ReconcileInterval < 1*time.Minute {
		return errors.New("reconcileInterval cannot be less than 1 minute")
	}
	if cfg.NetworkAPIURL == "" {
		return errors.New("NetworkAPIURL cannot be empty")
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
