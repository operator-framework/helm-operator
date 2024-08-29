/*
Copyright 2020 The Operator-SDK Authors.

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

package hook_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	sdkhandler "github.com/operator-framework/operator-lib/handler"

	"github.com/operator-framework/helm-operator-plugins/pkg/hook"
	"github.com/operator-framework/helm-operator-plugins/pkg/internal/fake"
	internalhook "github.com/operator-framework/helm-operator-plugins/pkg/reconciler/internal/hook"
)

var _ = Describe("Hook", func() {
	Describe("dependentResourceWatcher", func() {
		var (
			drw   hook.PostHook
			c     *fake.Controller
			rm    *meta.DefaultRESTMapper
			cache cache.Cache
			owner *unstructured.Unstructured
			rel   *release.Release
			sch   *runtime.Scheme
			log   logr.Logger
			ctx   context.Context
		)

		BeforeEach(func() {
			rm = meta.NewDefaultRESTMapper([]schema.GroupVersion{})
			c = &fake.Controller{}
			log = logr.Discard()
			cache = &informertest.FakeInformers{}
			sch = runtime.NewScheme()
			ctx = context.Background()

			// Since this is a fake informer and controller, no need to wait for sync.
			Expect(cache.Start(ctx)).NotTo(HaveOccurred())
		})

		Context("with unknown APIs", func() {
			BeforeEach(func() {
				owner = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "testDeployment",
							"namespace": "ownerNamespace",
						},
					},
				}
				rel = &release.Release{
					Manifest: strings.Join([]string{rsOwnerNamespace}, "---\n"),
				}
				drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
			})
			It("should fail with an invalid release manifest", func() {
				rel.Manifest = "---\nfoobar"
				err := drw.Exec(owner, *rel, log)
				Expect(err).To(HaveOccurred())
			})
			It("should fail with unknown owner kind", func() {
				var err error = &meta.NoKindMatchError{
					GroupKind:        schema.GroupKind{Group: "apps", Kind: "Deployment"},
					SearchedVersions: []string{"v1"},
				}

				Expect(drw.Exec(owner, *rel, log)).To(MatchError(err))
			})
			It("should fail with unknown dependent kind", func() {
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
				var err error = &meta.NoKindMatchError{
					GroupKind:        schema.GroupKind{Group: "apps", Kind: "ReplicaSet"},
					SearchedVersions: []string{"v1"},
				}
				Expect(drw.Exec(owner, *rel, log)).To(MatchError(err))
			})
		})

		Context("with known APIs", func() {
			BeforeEach(func() {
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}, meta.RESTScopeNamespace)
				rm.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, meta.RESTScopeNamespace)
				rm.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, meta.RESTScopeRoot)
				rm.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}, meta.RESTScopeRoot)
			})

			It("should watch resource kinds only once each", func() {
				owner = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "rbac.authorization.k8s.io/v1",
						"kind":       "ClusterRole",
						"metadata": map[string]interface{}{
							"name": "testClusterRole",
						},
					},
				}
				rel = &release.Release{
					Manifest: strings.Join([]string{clusterRole, clusterRole, rsOwnerNamespace, rsOwnerNamespace}, "---\n"),
				}
				drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
				Expect(drw.Exec(owner, *rel, log)).To(Succeed())
				Expect(c.WatchCalls).To(HaveLen(2))
				Expect(validateSourceHandlerType(c.WatchCalls[0].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
				Expect(validateSourceHandlerType(c.WatchCalls[1].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
			})

			Context("when the owner is cluster-scoped", func() {
				BeforeEach(func() {
					owner = &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "rbac.authorization.k8s.io/v1",
							"kind":       "ClusterRole",
							"metadata": map[string]interface{}{
								"name": "testClusterRole",
							},
						},
					}
				})
				It("should watch namespace-scoped resources with ownerRef handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespace, ssOtherNamespace}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(2))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[1].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
				})
				It("should watch cluster-scoped resources with ownerRef handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{clusterRole, clusterRoleBinding}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(2))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[1].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
				})
				It("should watch resource policy keep resources with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespaceWithKeep, ssOtherNamespaceWithKeep, clusterRoleWithKeep, clusterRoleBindingWithKeep}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(4))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[1].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[2].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[3].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
				})
			})

			Context("when the owner is namespace-scoped", func() {
				BeforeEach(func() {
					owner = &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]interface{}{
								"name":      "testDeployment",
								"namespace": "ownerNamespace",
							},
						},
					}
				})
				It("should watch namespace-scoped dependent resources in the same namespace with ownerRef handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespace}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(1))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
				})
				It("should watch cluster-scoped resources with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{clusterRole}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(1))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
				})
				It("should watch namespace-scoped resources in a different namespace with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{ssOtherNamespace}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(1))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
				})
				It("should watch resource policy keep resources with annotation handler", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{rsOwnerNamespaceWithKeep, ssOtherNamespaceWithKeep, clusterRoleWithKeep}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(3))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[1].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[2].Source, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
				})
				It("should iterate the kind list and be able to set watches on each item", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{replicaSetList}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					Expect(drw.Exec(owner, *rel, log)).To(Succeed())
					Expect(c.WatchCalls).To(HaveLen(2))
					Expect(validateSourceHandlerType(c.WatchCalls[0].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
					Expect(validateSourceHandlerType(c.WatchCalls[1].Source, handler.TypedEnqueueRequestForOwner[*unstructured.Unstructured](sch, rm, owner, handler.OnlyControllerOwner()))).To(Succeed())
				})
				It("should error when unable to list objects", func() {
					rel = &release.Release{
						Manifest: strings.Join([]string{errReplicaSetList}, "---\n"),
					}
					drw = internalhook.NewDependentResourceWatcher(c, rm, cache, sch)
					err := drw.Exec(owner, *rel, log)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})

var _ = Describe("validateSourceHandlerType", func() {
	It("should return an error when source.Source is nil", func() {
		Expect(validateSourceHandlerType(nil, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(HaveOccurred())
	})
	It("should return an error when source.Kind.Handler is nil", func() {
		Expect(validateSourceHandlerType(source.Kind(nil, &unstructured.Unstructured{}, nil, nil), &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(HaveOccurred())
	})
	It("should return an error when expected is nil", func() {
		Expect(validateSourceHandlerType(source.Kind(nil, &unstructured.Unstructured{}, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{}, nil), nil)).To(HaveOccurred())
	})
	It("should return an error when source.Kind.Handler does not match expected type", func() {
		Expect(validateSourceHandlerType(source.Kind(nil, &unstructured.Unstructured{}, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{}, nil), "string")).To(HaveOccurred())
	})
	It("should not return an error when source.Kind.Handler matches expectedType", func() {
		Expect(validateSourceHandlerType(source.Kind(nil, &unstructured.Unstructured{}, &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{}, nil), &sdkhandler.EnqueueRequestForAnnotation[*unstructured.Unstructured]{})).To(Succeed())
	})
})

// validateSourceHandlerType takes in a source.Source and uses reflection to determine
// if the handler used by the source matches the expected type.
// It is assumed that the source.Source was created via the source.Kind() function.
func validateSourceHandlerType(s source.Source, expected interface{}) error {
	if s == nil {
		return errors.New("nil source.Source provided")
	}
	sourceVal := reflect.Indirect(reflect.ValueOf(s))
	if !sourceVal.IsValid() {
		return errors.New("provided source.Source value is invalid")
	}
	handlerFieldVal := sourceVal.FieldByName("Handler")
	if !handlerFieldVal.IsValid() {
		return errors.New("provided source.Source's Handler field is invalid")
	}
	handlerField := reflect.Indirect(handlerFieldVal.Elem())
	if !handlerField.IsValid() {
		return errors.New("provided source.Source's Handler field value is invalid")
	}
	handlerType := handlerField.Type()

	expectedValue := reflect.Indirect(reflect.ValueOf(expected))
	if !expectedValue.IsValid() {
		return errors.New("provided expected value is invalid")
	}

	expectedType := expectedValue.Type()
	if handlerType != expectedType {
		return fmt.Errorf("detected source.Source handler type %q does not match expected type %q", handlerType, expectedType)
	}
	return nil
}

var (
	rsOwnerNamespace = `
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: testReplicaSet
  namespace: ownerNamespace
`
	rsOwnerNamespaceWithKeep = `
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: testReplicaSet
  namespace: ownerNamespace
  annotations:
    helm.sh/resource-policy: keep
`
	ssOtherNamespace = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: otherTestStatefulSet
  namespace: otherNamespace
`
	ssOtherNamespaceWithKeep = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: otherTestStatefulSet
  namespace: otherNamespace
  annotations:
    helm.sh/resource-policy: keep
`
	clusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: testClusterRole
`
	clusterRoleWithKeep = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: testClusterRole
  annotations:
    helm.sh/resource-policy: keep
`
	clusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: testClusterRoleBinding
`
	clusterRoleBindingWithKeep = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: testClusterRoleBinding
  annotations:
    helm.sh/resource-policy: keep
`
	replicaSetList = `
apiVersion: v1
kind: List
items:
  - apiVersion: apps/v1
    kind: ReplicaSet
    metadata: 
      name: testReplicaSet1
      namespace: ownerNamespace
  - apiVersion: apps/v1
    kind: ReplicaSet
    metadata: 
      name: testReplicaSet2
      namespace: ownerNamespace
`
	errReplicaSetList = `
apiVersion: v1
kind: List
items:
`
)
