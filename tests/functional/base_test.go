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
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	horizon "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

func CreateHorizon(name types.NamespacedName, spec horizon.HorizonSpec) *horizon.Horizon {
	instance := &horizon.Horizon{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "horizon.openstack.org/v1alpha1",
			Kind:       "Horizon",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Spec: spec,
	}
	err := k8sClient.Create(ctx, instance)
	Expect(err).NotTo(HaveOccurred())

	return instance
}

func GetDefaultHorizonSpec() horizon.HorizonSpec {
	return horizon.HorizonSpec{
		ContainerImage:    "test-horizon-container-image",
		Secret:            SecretName,
		MemcachedInstance: "memcached",
	}
}

func GetHorizon(name types.NamespacedName) *horizon.Horizon {
	instance := &horizon.Horizon{}
	gomega.Eventually(func(g gomega.Gomega) error {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
		return nil
	}, timeout, interval).Should(Succeed())
	return instance
}

func HorizonMemcached() *memcachedv1.Memcached {
	return &memcachedv1.Memcached{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "memcached.openstack.org/v1beta1",
			Kind:       "Memcached",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "memcached",
			Namespace: namespace,
		},
	}
}

func CreateHorizonMemcached() *memcachedv1.Memcached {
	instance := HorizonMemcached()
	err := k8sClient.Create(ctx, instance)
	Expect(err).NotTo(HaveOccurred())

	instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
	instance.Status.ReadyCount = int32(1)
	instance.Status.ServerList = []string{"memcached-0.memcached:11211"}
	instance.Status.ServerListWithInet = []string{"inet:[memcached-0.memcached]:11211"}
	Expect(k8sClient.Status().Update(ctx, instance)).Should(Succeed())

	return instance
}

func GetMemcached(name types.NamespacedName) *memcachedv1.Memcached {
	instance := &memcachedv1.Memcached{}
	gomega.Eventually(func(g gomega.Gomega) error {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
		return nil
	}, timeout, interval).Should(Succeed())
	return instance
}

func HorizonConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetHorizon(name)
	return instance.Status.Conditions
}
