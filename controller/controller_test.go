package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/networkapi-ingress-controller/config"
	networkingv1 "k8s.io/api/networking/v1"
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

func StringPtr(s string) *string {
	return &s
}
