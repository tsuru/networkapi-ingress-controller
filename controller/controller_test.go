package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/networkapi-ingress-controller/config"
	"github.com/tsuru/networkapi-ingress-controller/networkapi"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileIngress_validateIngress(t *testing.T) {
	tests := map[string]struct {
		ingress       *networkingv1.Ingress
		expectedError string
	}{
		"ingress nil": {
			expectedError: "Ingress cannot be nil",
		},

		"with invalid class name": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("unknown-class"),
				},
			},
			expectedError: "Invalid ingress class detected, predicate failed",
		},

		"without default backend and rules": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
				},
			},
			expectedError: "Ingress must have either default backend or one rule",
		},

		"with default backend and rules at same time": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "www.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "another-service",
												},
											},
										},
									},
								},
							},
						},
					},
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "example-service",
							Port: networkingv1.ServiceBackendPort{
								Number: int32(80),
							},
						},
					},
				},
			},
			expectedError: "Ingress cannot have different Services by rule",
		},

		"with only default backend": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "example-service",
							Port: networkingv1.ServiceBackendPort{
								Number: int32(80),
							},
						},
					},
				},
			},
		},

		"with more than one path per rule": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "www.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/app1"},
										{Path: "/app2"},
										{Path: "/app3"},
									},
								},
							},
						},
					},
				},
			},
			expectedError: "Ingress can have only one path",
		},

		"with invalid path": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "www.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/app/foo/bar"},
									},
								},
							},
						},
					},
				},
			},
			expectedError: "Ingress path must be unset, / or /*",
		},

		"path without backend service": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "www.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{Path: "/*"},
									},
								},
							},
						},
					},
				},
			},
			expectedError: "Ingress path must have a Service",
		},

		"path with empty Service name": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "www.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/*",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedError: "Service backend must have a name",
		},

		"with backends pointing up to different Services": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "www.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/*",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "example-service",
												},
											},
										},
									},
								},
							},
						},
						{
							Host: "blog.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/*",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "blog-example",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedError: "Ingress cannot have different Services by rule",
		},

		"with more than one HTTP rule backing to the same Service": {
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					IngressClassName: StringPtr("globo-networkapi"),
					Rules: []networkingv1.IngressRule{
						{
							Host: "www.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/*",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "example-service",
													Port: networkingv1.ServiceBackendPort{
														Number: int32(80),
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Host: "blog.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/*",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "example-service",
													Port: networkingv1.ServiceBackendPort{
														Number: int32(80),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &reconcileIngress{
				cfg: config.Config{
					IngressClassName: "globo-networkapi",
				},
			}

			got := r.validateIngress(tt.ingress)
			if tt.expectedError == "" {
				assert.NoError(t, got)
				return
			}

			assert.EqualError(t, got, tt.expectedError)
		})
	}
}

func TestReconcileIngress_svcAndPortFromIngress(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "example-service",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name: "https",
					Port: 443,
				},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "10.1.1.1",
					},
				},
			},
		},
	}

	r := &reconcileIngress{
		client: fake.NewClientBuilder().
			WithObjects(svc).
			Build(),
		cfg: config.Config{
			IngressClassName: "globo-networkapi",
		},
	}

	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "www.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/*",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "example-service",
											Port: networkingv1.ServiceBackendPort{
												Number: int32(443),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	foundSvc, ports, err := r.svcAndPortFromIngress(context.TODO(), ing)
	assert.NoError(t, err)

	assert.Equal(t, svc.ObjectMeta, foundSvc.ObjectMeta)
	assert.Equal(t, svc.Spec, foundSvc.Spec)

	assert.Equal(t, []corev1.ServicePort{
		{
			Name: "https",
			Port: 443,
		},
	}, ports)

	targets, err := r.targetsForService(context.TODO(), ing, foundSvc, ports)
	assert.NoError(t, err)
	assert.Len(t, targets, 1)
	assert.Equal(t, "10.1.1.1", targets[0].IP.String())
	assert.Equal(t, 443, targets[0].Port)
	assert.Equal(t, 0, targets[0].NetworkID)
	assert.True(t, targets[0].TLS)

}

func TestReconcileTakeOver(t *testing.T) {
	fakeNetworkAPIClient := &networkapi.FakeNetworkAPI{
		VIPs: map[string]networkapi.VIP{
			"vip-blah": {
				Name: "vip-blah",
				Ports: []networkapi.VIPPort{
					{
						ID:   29,
						Port: 80,
						Pools: []networkapi.VIPPool{
							{
								ID: 10,
								ServerPool: networkapi.IntOrID{
									ID: 111,
								},
							},
						},
					},
				},
				IPv4: &networkapi.IntOrID{
					ID: 8000,
				},
			},
		},
		IPsByID: map[int]networkapi.IP{
			8000: {
				ID:   8000,
				Oct1: 100,
				Oct2: 10,
				Oct3: 10,
				Oct4: 10,
			},
		},
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ingress-1",
			Namespace: "default",
			Annotations: map[string]string{
				config.TakeOverAnnotation: "vip-blah",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: StringPtr("globo-networkapi"),
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "example-service",
					Port: networkingv1.ServiceBackendPort{
						Number: int32(80),
					},
				},
			},
		},
	}

	service1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "10.1.1.1",
					},
				},
			},
		},
	}

	endpoints1 := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(ingress, service1, endpoints1).Build()

	r := NewReconciler(
		client,
		&record.FakeRecorder{
			Events: make(chan string, 10000),
		},
		config.Config{
			IngressClassName: "globo-networkapi",
		},
	)
	r.networkAPIClient = fakeNetworkAPIClient

	ctx := log.IntoContext(context.TODO(), zap.New(zap.UseDevMode(true)))

	_, err := r.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
		},
	})

	assert.NoError(t, err)
	assert.Len(t, fakeNetworkAPIClient.VIPDeploys, 1)
	assert.Len(t, fakeNetworkAPIClient.VIPUpdates, 1)

	updatedIngress := &networkingv1.Ingress{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
	}, updatedIngress)
	require.NoError(t, err)
	assert.Equal(t, "100.10.10.10", updatedIngress.Status.LoadBalancer.Ingress[0].IP)
}

func TestReconcileTakeOverOverHTTPS(t *testing.T) {
	fakeNetworkAPIClient := &networkapi.FakeNetworkAPI{
		VIPs: map[string]networkapi.VIP{
			"vip-blah": {
				Name: "vip-blah",
				Ports: []networkapi.VIPPort{
					{
						ID:   29,
						Port: 80,
						Pools: []networkapi.VIPPool{
							{
								ID: 10,
								ServerPool: networkapi.IntOrID{
									ID: 111,
								},
							},
						},
					},
				},
				IPv4: &networkapi.IntOrID{
					ID: 8000,
				},
			},
		},
		IPsByID: map[int]networkapi.IP{
			8000: {
				ID:   8000,
				Oct1: 100,
				Oct2: 10,
				Oct3: 10,
				Oct4: 10,
			},
		},
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ingress-1",
			Namespace: "default",
			Annotations: map[string]string{
				config.TakeOverAnnotation: "vip-blah",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: StringPtr("globo-networkapi"),
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "example-service",
					Port: networkingv1.ServiceBackendPort{
						Number: int32(443),
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"example.com"},
				},
			},
		},
	}

	service1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name: "https",
					Port: 443,
				},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "10.1.1.1",
					},
				},
			},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(ingress, service1).Build()

	r := NewReconciler(
		client,
		&record.FakeRecorder{
			Events: make(chan string, 10000),
		},
		config.Config{
			IngressClassName: "globo-networkapi",
		},
	)
	r.networkAPIClient = fakeNetworkAPIClient

	ctx := log.IntoContext(context.TODO(), zap.New(zap.UseDevMode(true)))

	_, err := r.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
		},
	})

	assert.NoError(t, err)
	assert.Len(t, fakeNetworkAPIClient.VIPDeploys, 1)
	assert.Len(t, fakeNetworkAPIClient.VIPUpdates, 1)

	updatedIngress := &networkingv1.Ingress{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
	}, updatedIngress)
	require.NoError(t, err)
	assert.Equal(t, "100.10.10.10", updatedIngress.Status.LoadBalancer.Ingress[0].IP)

	assert.Len(t, fakeNetworkAPIClient.Pools, 1)
	pool := fakeNetworkAPIClient.Pools["kube-napi-ingress__default_ingress-1_https"]
	assert.Equal(t, "kube-napi-ingress__default_ingress-1_https", pool.Identifier)
	assert.Len(t, pool.Members, 1)
	assert.Equal(t, "10.1.1.1", pool.Members[0].IP.IPFormated)

}

func StringPtr(s string) *string {
	return &s
}
