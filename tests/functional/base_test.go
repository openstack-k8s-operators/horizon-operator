/*
Copyright 2023

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
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega" //revive:disable:dot-imports
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	horizon "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

func CreateHorizon(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "horizon.openstack.org/v1beta1",
		"kind":       "Horizon",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func GetDefaultHorizonSpec() map[string]interface{} {
	return map[string]interface{}{
		"secret":            SecretName,
		"memcachedInstance": "memcached",
	}
}

func GetTLSHorizonSpec() map[string]interface{} {
	spec := GetDefaultHorizonSpec()
	spec["tls"] = map[string]interface{}{
		"caBundleSecretName": CABundleSecretName,
		"secretName":         InternalCertSecretName,
	}
	return spec
}

func GetHorizon(name types.NamespacedName) *horizon.Horizon {
	instance := &horizon.Horizon{}
	gomega.Eventually(func(g gomega.Gomega) error {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
		return nil
	}, timeout, interval).Should(Succeed())
	return instance
}

func CreateHorizonSecret(namespace string, name string) *corev1.Secret {
	return th.CreateSecret(
		types.NamespacedName{Namespace: namespace, Name: name},
		map[string][]byte{},
	)
}

func HorizonConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetHorizon(name)
	return instance.Status.Conditions
}
