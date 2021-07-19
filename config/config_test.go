package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFromInstance(t *testing.T) {
	m := metav1.ObjectMeta{
		Annotations: map[string]string{
			"kube-napi-ingress.tsuru.io/vipEnvironmentID":  "99",
			"kube-napi-ingress.tsuru.io/POOLEnvironmentID": "101",
		},
	}
	baseCfg := Config{
		DefaultVIPEnvironmentID:  8,
		DefaultPoolEnvironmentID: 7,
		DefaultCacheGroupID:      6,
	}
	cfg := FromInstance(&m, baseCfg)
	require.Equal(t, InstanceConfig{
		VIPEnvironmentID:  99,
		PoolEnvironmentID: 101,
		CacheGroupID:      6,
		BaseConfig:        baseCfg,
	}, cfg)
}
