package config

import "time"

const (
	defaultIngressClassName = "globo-networkapi"
)

type Config struct {
	ClusterName             string
	IngressClassName        string
	DefaultVIPEnvironmentID int
	ReconcileInterval       time.Duration
}

func Get() Config {
	return Config{
		ReconcileInterval: 5 * time.Minute,
		IngressClassName:  defaultIngressClassName,
	}
}
