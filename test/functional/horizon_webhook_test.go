/*
Copyright 2023.

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

package functional_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
)

var _ = Describe("Horizon Webhook", func() {

	var horizonName types.NamespacedName

	BeforeEach(func() {

		horizonName = types.NamespacedName{
			Name:      "horizon",
			Namespace: namespace,
		}

		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())
	})

	When("A Horizon instance is created without container images", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
		})

		It("should have the defaults initialized by webhook", func() {
			Horizon := GetHorizon(horizonName)
			Expect(Horizon.Spec.ContainerImage).Should(Equal(
				horizonv1.ContainerImage,
			))
		})
	})

	When("A Horizon instance is created with container images", func() {
		BeforeEach(func() {
			horizonSpec := GetDefaultHorizonSpec()
			horizonSpec["containerImage"] = "container-image"
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, horizonSpec))
		})

		It("should use the given values", func() {
			Horizon := GetHorizon(horizonName)
			Expect(Horizon.Spec.ContainerImage).Should(Equal(
				"container-image",
			))
		})
	})

	It("rejects a wrong TopologyRef on a different namespace", func() {
		horizonSpec := GetDefaultHorizonSpec()
		// Inject a topologyRef that points to a different namespace
		horizonSpec["topologyRef"] = map[string]any{
			"name":      "foo",
			"namespace": "bar",
		}
		raw := map[string]any{
			"apiVersion": "horizon.openstack.org/v1beta1",
			"kind":       "Horizon",
			"metadata": map[string]any{
				"name":      "horizon",
				"namespace": namespace,
			},
			"spec": horizonSpec,
		}
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(
			ContainSubstring(
				"spec.topologyRef.namespace: Invalid value: \"namespace\": Customizing namespace field is not supported"),
		)
	})

	It("accepts public port name and rejects wrong names on create and update", func() {
		horizonWithPortName := func(name, portName string) *unstructured.Unstructured {
			spec := GetDefaultHorizonSpec()
			spec["override"] = map[string]any{
				"service": map[string]any{
					"spec": map[string]any{
						"ports": []any{map[string]any{"name": portName, "port": int64(13778)}},
					},
				},
			}
			return &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "horizon.openstack.org/v1beta1",
				"kind":       "Horizon",
				"metadata": map[string]any{
					"name":      name,
					"namespace": namespace,
				},
				"spec": spec,
			}}
		}

		accepted := horizonWithPortName(horizonName.Name, "public")
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, accepted, func() error { return nil })
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(th.DeleteInstance, accepted)

		invalidCreate := horizonWithPortName(horizonName.Name+"-invalid", "wrong")
		_, err = controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, invalidCreate, func() error { return nil })
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`must be "public"`))

		current := &horizonv1.Horizon{}
		current.Name = horizonName.Name
		current.Namespace = horizonName.Namespace
		_, err = controllerutil.CreateOrPatch(th.Ctx, th.K8sClient, current, func() error {
			current.Spec.Override.Service.Spec.Ports[0].Name = "wrong"
			return nil
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`must be "public"`))
	})

	It("validates the endpointURL of the service override", func() {
		horizonWithOverride := func(name string, svc map[string]any) *unstructured.Unstructured {
			spec := GetDefaultHorizonSpec()
			spec["override"] = map[string]any{"service": svc}
			return &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "horizon.openstack.org/v1beta1",
				"kind":       "Horizon",
				"metadata":   map[string]any{"name": name, "namespace": namespace},
				"spec":       spec,
			}}
		}
		create := func(name string, svc map[string]any) error {
			_, err := controllerutil.CreateOrPatch(th.Ctx, th.K8sClient,
				horizonWithOverride(name, svc), func() error { return nil })
			return err
		}

		Expect(create(horizonName.Name+"-ok", map[string]any{
			"endpointURL": "https://dashboard.example.com:8443",
			"spec":        map[string]any{"ports": []any{map[string]any{"name": "public", "port": int64(8443)}}},
		})).To(Succeed())
		DeferCleanup(th.DeleteInstance, horizonWithOverride(horizonName.Name+"-ok", nil))

		Expect(create(horizonName.Name+"-short", map[string]any{
			"endpointURL": "https://horizon"})).To(MatchError(ContainSubstring("endpointURL")))
		Expect(create(horizonName.Name+"-scheme", map[string]any{
			"endpointURL": "dashboard.example.com"})).To(MatchError(ContainSubstring("must be a valid URL with a scheme and host")))
		Expect(create(horizonName.Name+"-ip", map[string]any{
			"endpointURL": "https://192.168.1.1"})).To(MatchError(ContainSubstring("host must not be an IP literal")))
		Expect(create(horizonName.Name+"-port", map[string]any{
			"endpointURL": "https://dashboard.example.com:8443",
			"spec":        map[string]any{"ports": []any{map[string]any{"name": "public", "port": int64(9443)}}},
		})).To(MatchError(ContainSubstring("must match the port in endpointURL")))
	})
})
