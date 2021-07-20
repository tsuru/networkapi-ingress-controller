package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IngressControllerName   = "kube-napi-ingress"
	FinalizerName           = IngressControllerName + ".tsuru.io/cleanup"
	defaultIngressClassName = "globo-networkapi"
	annotationsConfigPrefix = IngressControllerName + ".tsuru.io/"
)

type Config struct {
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

func Get(configFileName string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(configFileName)
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

type InstanceConfig struct {
	VIPEnvironmentID  int
	PoolEnvironmentID int
	CacheGroupID      int
	TrafficReturnID   int
	TimeoutID         int
	PersistenceID     int
	VIPL7RuleID       int
	VIPL4ProtocolID   int
	VIPL7ProtocolID   int
	BaseConfig        Config
}

func FromInstance(obj metav1.Object, cfg Config) InstanceConfig {
	instConfig := InstanceConfig{
		VIPEnvironmentID:  cfg.DefaultVIPEnvironmentID,
		PoolEnvironmentID: cfg.DefaultPoolEnvironmentID,
		CacheGroupID:      cfg.DefaultCacheGroupID,
		TrafficReturnID:   cfg.DefaultTrafficReturnID,
		TimeoutID:         cfg.DefaultTimeoutID,
		PersistenceID:     cfg.DefaultPersistenceID,
		VIPL7RuleID:       cfg.DefaultVIPL7RuleID,
		VIPL4ProtocolID:   cfg.DefaultVIPL4ProtocolID,
		VIPL7ProtocolID:   cfg.DefaultVIPL7ProtocolID,
	}

	lowerAnnotations := map[string]string{}
	for key, value := range obj.GetAnnotations() {
		lowerAnnotations[strings.ToLower(key)] = value
	}

	instElem := reflect.ValueOf(&instConfig).Elem()
	instType := instElem.Type()
	for i := 0; i < instElem.NumField(); i++ {
		fieldName := instType.Field(i).Name
		annotationValue, ok := lowerAnnotations[strings.ToLower(annotationsConfigPrefix+fieldName)]
		if !ok {
			continue
		}
		fmt.Sscanf(annotationValue, "%v", instElem.Field(i).Addr().Interface())
	}

	instConfig.BaseConfig = cfg
	return instConfig
}
