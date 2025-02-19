/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	network "knative.dev/networking/pkg"
	"knative.dev/networking/pkg/apis/networking"
	netv1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/kmeta"
	pkgnet "knative.dev/pkg/network"
	apiConfig "knative.dev/serving/pkg/apis/config"
	"knative.dev/serving/pkg/apis/serving"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"knative.dev/serving/pkg/reconciler/route/config"
	"knative.dev/serving/pkg/reconciler/route/traffic"

	. "knative.dev/serving/pkg/testing/v1"
)

var (
	r            = Route("test-ns", "test-route")
	expectedMeta = metav1.ObjectMeta{
		Name:      "test-route",
		Namespace: "test-ns",
		OwnerReferences: []metav1.OwnerReference{
			*kmeta.NewControllerRef(r),
		},
		Labels: map[string]string{
			serving.RouteLabelKey: r.Name,
		},
	}
)

func TestNewMakeK8SService(t *testing.T) {
	tests := []struct {
		name         string
		route        *v1.Route
		ingress      *netv1alpha1.Ingress
		targetName   string
		expectedSpec corev1.ServiceSpec
		expectedMeta metav1.ObjectMeta
		shouldFail   bool
	}{{
		name:  "no-loadbalancer",
		route: r,
		ingress: &netv1alpha1.Ingress{
			Status: netv1alpha1.IngressStatus{},
		},
		expectedMeta: expectedMeta,
		shouldFail:   true,
	}, {
		name:  "empty-loadbalancer",
		route: r,
		ingress: &netv1alpha1.Ingress{
			Status: netv1alpha1.IngressStatus{
				DeprecatedLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{}},
				},
				PublicLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{}},
				},
				PrivateLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{}},
				},
			},
		},
		expectedMeta: expectedMeta,
		shouldFail:   true,
	}, {
		name:  "multi-loadbalancer",
		route: r,
		ingress: &netv1alpha1.Ingress{
			Status: netv1alpha1.IngressStatus{
				DeprecatedLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{
						Domain: "domain.com",
					}, {
						DomainInternal: "domain.com",
					}},
				},
				PublicLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{
						Domain: "domain.com",
					}, {
						DomainInternal: "domain.com",
					}},
				},
				PrivateLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{
						Domain: "domain.com",
					}, {
						DomainInternal: "domain.com",
					}},
				},
			},
		},
		expectedMeta: expectedMeta,
		shouldFail:   true,
	}, {
		name:  "ingress-with-domain",
		route: r,
		ingress: &netv1alpha1.Ingress{
			Status: netv1alpha1.IngressStatus{
				DeprecatedLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{Domain: "domain.com"}},
				},
				PublicLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{Domain: "domain.com"}},
				},
				PrivateLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{Domain: "domain.com"}},
				},
			},
		},
		expectedMeta: expectedMeta,
		expectedSpec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "domain.com",
			Ports: []corev1.ServicePort{{
				Name:       networking.ServicePortNameH2C,
				Port:       int32(80),
				TargetPort: intstr.FromInt(80),
			}},
		},
	}, {
		name:  "ingress-with-domaininternal",
		route: r,
		ingress: &netv1alpha1.Ingress{
			Status: netv1alpha1.IngressStatus{
				DeprecatedLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{DomainInternal: pkgnet.GetServiceHostname("istio-ingressgateway", "istio-system")}},
				},
				PublicLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{DomainInternal: pkgnet.GetServiceHostname("istio-ingressgateway", "istio-system")}},
				},
				PrivateLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{DomainInternal: pkgnet.GetServiceHostname("private-istio-ingressgateway", "istio-system")}},
				},
			},
		},
		expectedMeta: expectedMeta,
		expectedSpec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeExternalName,
			ExternalName:    pkgnet.GetServiceHostname("private-istio-ingressgateway", "istio-system"),
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{{
				Name:       networking.ServicePortNameH2C,
				Port:       int32(80),
				TargetPort: intstr.FromInt(80),
			}},
		},
	}, {
		name:  "ingress-with-only-mesh",
		route: r,
		ingress: &netv1alpha1.Ingress{
			Status: netv1alpha1.IngressStatus{
				DeprecatedLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{MeshOnly: true}},
				},
				PublicLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{MeshOnly: true}},
				},
				PrivateLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{MeshOnly: true}},
				},
			},
		},
		expectedMeta: expectedMeta,
		expectedSpec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Name: "http",
				Port: 80,
			}},
		},
	}, {
		name:       "with-target-name-specified",
		route:      r,
		targetName: "my-target-name",
		ingress: &netv1alpha1.Ingress{
			Status: netv1alpha1.IngressStatus{
				DeprecatedLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{MeshOnly: true}},
				},
				PublicLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{MeshOnly: true}},
				},
				PrivateLoadBalancer: &netv1alpha1.LoadBalancerStatus{
					Ingress: []netv1alpha1.LoadBalancerIngressStatus{{MeshOnly: true}},
				},
			},
		},
		expectedMeta: metav1.ObjectMeta{
			Name:      "my-target-name-test-route",
			Namespace: r.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(r),
			},
			Labels: map[string]string{
				serving.RouteLabelKey: r.Name,
			},
		},
		expectedSpec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Name: "http",
				Port: 80,
			}},
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			ctx := config.ToContext(context.Background(), cfg)
			service, err := MakeK8sService(ctx, tc.route, tc.targetName, tc.ingress, false, "")
			// Validate
			if tc.shouldFail && err == nil {
				t.Fatal("MakeK8sService returned success but expected error")
			}
			if !tc.shouldFail {
				if err != nil {
					t.Fatal("MakeK8sService:", err)
				}

				if !cmp.Equal(tc.expectedMeta, service.ObjectMeta) {
					t.Error("Unexpected Metadata (-want +got):", cmp.Diff(tc.expectedMeta, service.ObjectMeta))
				}
				if !cmp.Equal(tc.expectedSpec, service.Spec) {
					t.Error("Unexpected ServiceSpec (-want +got):", cmp.Diff(tc.expectedSpec, service.Spec))
				}
			}
		})
	}
}

func TestMakeK8sPlaceholderService(t *testing.T) {
	tests := []struct {
		name           string
		expectedSpec   corev1.ServiceSpec
		expectedLabels map[string]string
		expectedAnnos  map[string]string
		wantErr        bool
		route          *v1.Route
	}{{
		name:  "default public domain route",
		route: r,
		expectedSpec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeExternalName,
			ExternalName:    "foo-test-route.test-ns.example.com",
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{{
				Name:       networking.ServicePortNameH2C,
				Port:       int32(80),
				TargetPort: intstr.FromInt(80),
			}},
		},
		expectedLabels: map[string]string{
			serving.RouteLabelKey: "test-route",
		},
		wantErr: false,
	}, {
		name: "labels and annotations are propagated from the route",
		route: Route("test-ns", "test-route",
			WithRouteLabel(map[string]string{"route-label": "foo"}), WithRouteAnnotation(map[string]string{"route-anno": "bar"})),
		expectedSpec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeExternalName,
			ExternalName:    "foo-test-route.test-ns.example.com",
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{{
				Name:       networking.ServicePortNameH2C,
				Port:       int32(80),
				TargetPort: intstr.FromInt(80),
			}},
		},
		expectedLabels: map[string]string{
			serving.RouteLabelKey: "test-route",
			"route-label":         "foo",
		},
		expectedAnnos: map[string]string{
			"route-anno": "bar",
		},
		wantErr: false,
	}, {
		name:  "cluster local route",
		route: Route("test-ns", "test-route", WithRouteLabel(map[string]string{network.VisibilityLabelKey: serving.VisibilityClusterLocal})),
		expectedSpec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeExternalName,
			ExternalName:    pkgnet.GetServiceHostname("foo-test-route", "test-ns"),
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{{
				Name:       networking.ServicePortNameH2C,
				Port:       int32(80),
				TargetPort: intstr.FromInt(80),
			}},
		},
		expectedLabels: map[string]string{
			serving.RouteLabelKey: "test-route",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			ctx := config.ToContext(context.Background(), cfg)
			target := traffic.RevisionTarget{
				TrafficTarget: v1.TrafficTarget{
					Tag: "foo",
				},
			}

			got, err := MakeK8sPlaceholderService(ctx, tt.route, target.Tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeK8sPlaceholderService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got == nil {
				t.Fatal("Unexpected nil service")
			}

			if !cmp.Equal(tt.expectedLabels, got.Labels) {
				t.Error("Unexpected Labels (-want +got):", cmp.Diff(tt.expectedLabels, got.Labels))
			}
			if !cmp.Equal(tt.expectedAnnos, got.ObjectMeta.Annotations) {
				t.Error("Unexpected Annotations (-want +got):", cmp.Diff(tt.expectedAnnos, got.ObjectMeta.Annotations))
			}
			if !cmp.Equal(tt.expectedSpec, got.Spec) {
				t.Error("Unexpected ServiceSpec (-want +got):", cmp.Diff(tt.expectedSpec, got.Spec))
			}
		})
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Domain: &config.Domain{
			Domains: map[string]*config.LabelSelector{
				"example.com": {},
				"another-example.com": {
					Selector: map[string]string{"app": "prod"},
				},
			},
		},
		Network: &network.Config{
			DefaultIngressClass: "test-ingress-class",
			DomainTemplate:      network.DefaultDomainTemplate,
			TagTemplate:         network.DefaultTagTemplate,
		},
		Features: &apiConfig.Features{
			MultiContainer:           apiConfig.Disabled,
			PodSpecAffinity:          apiConfig.Disabled,
			PodSpecFieldRef:          apiConfig.Disabled,
			PodSpecDryRun:            apiConfig.Enabled,
			PodSpecHostAliases:       apiConfig.Disabled,
			PodSpecNodeSelector:      apiConfig.Disabled,
			PodSpecTolerations:       apiConfig.Disabled,
			PodSpecVolumesEmptyDir:   apiConfig.Disabled,
			PodSpecPriorityClassName: apiConfig.Disabled,
			TagHeaderBasedRouting:    apiConfig.Disabled,
		},
	}
}
