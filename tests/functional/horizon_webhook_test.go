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
		horizonSpec["topologyRef"] = map[string]interface{}{
			"name":      "foo",
			"namespace": "bar",
		}
		raw := map[string]interface{}{
			"apiVersion": "horizon.openstack.org/v1beta1",
			"kind":       "Horizon",
			"metadata": map[string]interface{}{
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
})
