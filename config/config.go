package config

import "time"

type Config struct {
	ClusterName             string
	DefaultVIPEnvironmentID int
	ReconcileInterval       time.Duration
}

func Get() Config {
	return Config{
		ReconcileInterval: 5 * time.Minute,
	}
}
