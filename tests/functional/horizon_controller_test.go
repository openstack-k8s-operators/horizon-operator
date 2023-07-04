package functional_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	. "github.com/openstack-k8s-operators/lib-common/modules/test/helpers"

	horizonv1beta1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
)

var _ = Describe("Horizon controller", func() {

	var horizonName types.NamespacedName
	var secret *corev1.Secret

	BeforeEach(func() {
		horizonName = types.NamespacedName{
			Name:      "horizon",
			Namespace: namespace,
		}

		// lib-common uses OPERATOR_TEMPLATES env var to locate the "templates"
		// directory of the operator. We need to set them othervise lib-common
		// will fail to generate the ConfigMap as it does not find common.sh
		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())
	})

	When("A Horizon instance is created", func() {
		BeforeEach(func() {
			DeferCleanup(DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
		})

		It("should have the Spec and Status fields initialized", func() {
			horizon := GetHorizon(horizonName)
			Expect(horizon.Spec.Secret).Should(Equal("test-osp-secret"))
			// TODO(gibi): Why defaulting does not work?
			// Expect(horizon.Instance.Spec.ServiceUser).Should(Equal("horizon"))
		})

		It("should have a finalizer", func() {
			// the reconciler loop adds the finalizer so we have to wait for
			// it to run
			Eventually(func() []string {
				return GetHorizon(horizonName).Finalizers
			}, timeout, interval).Should(ContainElement("Horizon"))
		})

		It("should have Unknown Conditions initialized as transporturl not created", func() {
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
			)
		})
	})

	When("an unrelated secret is provided", func() {
		BeforeEach(func() {
			DeferCleanup(DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
		})
		It("should remain in a state of waiting for the proper secret", func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "an-unrelated-secret",
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
			)
		})
	})

	When("the proper secret is provided", func() {
		BeforeEach(func() {
			DeferCleanup(DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
		})
		It("should be in a state of having the input ready", func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("using a shared memcached instance", func() {
		BeforeEach(func() {
			DeferCleanup(DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
			DeferCleanup(DeleteInstance, CreateHorizonMemcached())
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
		})
		It("should be in a state of having the memcached ready", func() {
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				horizonv1beta1.HorizonMemcachedReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
})
