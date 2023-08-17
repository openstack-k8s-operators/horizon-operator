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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	horizonv1beta1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	horizon "github.com/openstack-k8s-operators/horizon-operator/pkg/horizon"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	configmap "github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	deployment "github.com/openstack-k8s-operators/lib-common/modules/common/deployment"
	endpoint "github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	labels "github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	oko_secret "github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GetClient -
func (r *HorizonReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *HorizonReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetScheme -
func (r *HorizonReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// GetLog returns a logger object with a prefix of "conroller.name" and aditional controller context fields
func GetLog(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("Horizon")
}

// HorizonReconciler reconciles a Horizon object
type HorizonReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneendpoints,verbs=get;list;watch;
//+kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;

// service account, role, rolebinding
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update
// service account permissions that are needed to grant permission to the above
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Horizon object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *HorizonReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	// Fetch the Horizon instance
	instance := &horizonv1beta1.Horizon{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		GetLog(ctx),
	)

	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the Ready condition based on the sub conditions
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, condition.ReadyMessage)
		} else {
			// something is not ready so reset the Ready condition
			instance.Status.Conditions.MarkUnknown(
				condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
			// and recalculate it based on the state of the rest of the conditions
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, nil
	}

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}

		cl := condition.CreateList(
			condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
			condition.UnknownCondition(condition.MemcachedReadyCondition, condition.InitReason, condition.MemcachedReadyInitMessage),
			condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
			condition.UnknownCondition(condition.ExposeServiceReadyCondition, condition.InitReason, condition.ExposeServiceReadyInitMessage),
			condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage),
			// service account, role, rolebinding conditions
			condition.UnknownCondition(condition.ServiceAccountReadyCondition, condition.InitReason, condition.ServiceAccountReadyInitMessage),
			condition.UnknownCondition(condition.RoleReadyCondition, condition.InitReason, condition.RoleReadyInitMessage),
			condition.UnknownCondition(condition.RoleBindingReadyCondition, condition.InitReason, condition.RoleBindingReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager -
func (r *HorizonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger()

	memcachedFn := func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all Horizon CRs
		horizons := &horizonv1beta1.HorizonList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), horizons, listOpts...); err != nil {
			logger.Error(err, "Unable to retrieve Horizon CRs %w")
			return nil
		}

		for _, cr := range horizons.Items {
			if o.GetName() == cr.Spec.MemcachedInstance {
				name := client.ObjectKey{
					Namespace: o.GetNamespace(),
					Name:      cr.Name,
				}
				logger.Info(fmt.Sprintf("Memcached %s is used by Horizon CR %s", o.GetName(), cr.Name))
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&horizonv1beta1.Horizon{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Watches(&source.Kind{Type: &memcachedv1.Memcached{}},
			handler.EnqueueRequestsFromMapFunc(memcachedFn)).
		Complete(r)
}

func (r *HorizonReconciler) reconcileDelete(ctx context.Context, instance *horizonv1beta1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	l := GetLog(ctx)
	l.Info("Reconciling Service delete")

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	l.Info("Reconciled Service delete successfully")

	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileInit(
	ctx context.Context,
	instance *horizonv1beta1.Horizon,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	l := GetLog(ctx)
	l.Info("Reconciling Service init")

	//
	// expose the service (create service and return the created endpoint URLs)
	//
	var horizonEndpoints = map[service.Endpoint]endpoint.Data{
		service.EndpointPublic: {
			Port: horizon.HorizonPublicPort,
		},
	}

	apiEndpoints := make(map[string]string)

	for endpointType, data := range horizonEndpoints {
		endpointTypeStr := string(endpointType)
		endpointName := horizon.ServiceName + "-" + endpointTypeStr

		svcOverride := instance.Spec.Override.Service[endpointTypeStr]

		exportLabels := util.MergeStringMaps(
			serviceLabels,
			map[string]string{
				string(endpointType): "true",
			},
		)

		// Create the service
		svc, err := service.NewService(
			service.GenericService(&service.GenericServiceDetails{
				Name:      endpointName,
				Namespace: instance.Namespace,
				Labels:    exportLabels,
				Selector:  serviceLabels,
				Port: service.GenericServicePort{
					Name:     endpointName,
					Port:     data.Port,
					Protocol: corev1.ProtocolTCP,
				},
			}),
			5,
			&svcOverride,
		)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ExposeServiceReadyErrorMessage,
				err.Error()))

			return ctrl.Result{}, err
		}

		ctrlResult, err := svc.CreateOrPatch(ctx, helper)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ExposeServiceReadyErrorMessage,
				err.Error()))

			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.ExposeServiceReadyRunningMessage))
			return ctrlResult, nil
		}
		// create service - end

		// TODO: TLS, pass in https as protocol, create TLS cert
		apiEndpoints[string(endpointType)], err = svc.GetAPIEndpoint(
			&svcOverride, data.Protocol, data.Path)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	instance.Status.Conditions.MarkTrue(condition.ExposeServiceReadyCondition, condition.ExposeServiceReadyMessage)

	//
	// Update instance status with service endpoint url information
	//
	instance.Status.Endpoint = apiEndpoints[string(endpoint.EndpointPublic)]

	// expose service - end

	l.Info("Reconciled Service init successfully")
	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileUpdate(ctx context.Context, instance *horizonv1beta1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	l := GetLog(ctx)
	l.Info("Reconciling Service update")

	// TODO: should have minor update tasks if required
	// - delete dbsync hash from status to rerun it?

	l.Info("Reconciled Service update successfully")
	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileUpgrade(ctx context.Context, instance *horizonv1beta1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	l := GetLog(ctx)
	l.Info("Reconciling Service upgrade")

	// TODO: should have major version upgrade tasks
	// -delete dbsync hash from status to rerun it?

	l.Info("Reconciled Service upgrade successfully")
	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileNormal(ctx context.Context, instance *horizonv1beta1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	l := GetLog(ctx)
	l.Info("Reconciling Service")

	// Service account, role, binding
	rbacResult, err := configureHorizonRbac(ctx, helper, instance)
	if err != nil {
		return rbacResult, err
	} else if (rbacResult != ctrl.Result{}) {
		return rbacResult, nil
	}

	// ConfigMap
	configMapVars := make(map[string]env.Setter)

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//
	ospSecret, hash, err := oko_secret.GetSecret(ctx, helper, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.InputReadyWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("openstack secret %s not found", instance.Spec.Secret)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	configMapVars[ospSecret.Name] = env.SetValue(hash)

	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	// run check OpenStack secret - end

	//
	// Check for required memcached used for caching
	//
	memcached, err := r.getHorizonMemcached(ctx, helper, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.MemcachedReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.MemcachedReadyWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("memcached %s not found", instance.Spec.MemcachedInstance)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.MemcachedReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if !memcached.IsReady() {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.MemcachedReadyWaitingMessage))
		return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("memcached %s is not ready", memcached.Name)
	}
	// Mark the Memcached Service as Ready if we get to this point with no errors
	instance.Status.Conditions.MarkTrue(
		condition.MemcachedReadyCondition, condition.MemcachedReadyMessage)
	// run check memcached - end

	//
	// Create ConfigMaps and Secrets required as input for the Service and calculate an overall hash of hashes
	//

	//
	// create Configmap required for horizon input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal horizon config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the OpenStack secret via the init container
	//
	err = r.generateServiceConfigMaps(ctx, instance, helper, &configMapVars, memcached)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	err = r.ensureHorizonSecret(ctx, instance, helper, &configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	//
	// create hash over all the different input resources to identify if any those changed
	// and a restart/recreate is required.
	//
	inputHash, hashChanged, err := r.createHashOfInputHashes(ctx, instance, configMapVars)
	if err != nil {
		return ctrl.Result{}, err
	} else if hashChanged {
		// Hash changed and instance status should be updated (which will be done by main defer func),
		// so we need to return and reconcile again
		return ctrl.Result{}, nil
	}

	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)

	// Create ConfigMaps and Secrets - end

	//
	// TODO check when/if Init, Update, or Upgrade should/could be skipped
	//

	serviceLabels := map[string]string{
		common.AppSelector: horizon.ServiceName,
	}

	// Handle service init
	ctrlResult, err := r.reconcileInit(ctx, instance, helper, serviceLabels)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service update
	ctrlResult, err = r.reconcileUpdate(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service upgrade
	ctrlResult, err = r.reconcileUpgrade(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	//
	// normal reconcile tasks
	//

	// Define a new Deployment object
	deplDef := horizon.Deployment(instance, inputHash, serviceLabels)

	depl := deployment.NewDeployment(
		deplDef,
		time.Second*5,
	)

	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage))
		return ctrlResult, nil
	}
	instance.Status.ReadyCount = depl.GetDeployment().Status.ReadyReplicas
	if instance.Status.ReadyCount > 0 {
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition, condition.DeploymentReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage))
	}
	// create Deployment - end

	l.Info("Reconciled Service successfully")
	return ctrl.Result{}, nil
}

// generateServiceConfigMaps - create configmaps which hold scripts and service configuration
// TODO add DefaultConfigOverwrite
func (r *HorizonReconciler) generateServiceConfigMaps(
	ctx context.Context,
	instance *horizonv1beta1.Horizon,
	h *helper.Helper,
	envVars *map[string]env.Setter,
	mc *memcachedv1.Memcached,
) error {
	//
	// create Configmap/Secret required for horizon input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal horizon config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the ospSecret via the init container
	//

	cmLabels := labels.GetLabels(instance, labels.GetGroupLabel(horizon.ServiceName), map[string]string{})

	// customData hold any customization for the service.
	// custom.conf is going to /etc/<service>/<service>.conf.d
	// all other files get placed into /etc/<service> to allow overwrite of e.g. logging.conf or policy.json
	// TODO: make sure custom.conf can not be overwritten
	customData := map[string]string{"9999_custom_settings.py": instance.Spec.CustomServiceConfig}
	for key, data := range instance.Spec.DefaultConfigOverwrite {
		customData[key] = data
	}

	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return err
	}
	keystonePublicURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointPublic)
	if err != nil {
		return err
	}

	memcachedServers := strings.Join(mc.Status.ServerList, "', '")

	url := strings.TrimPrefix(instance.Status.Endpoint, "http://")

	templateParameters := map[string]interface{}{
		"keystoneURL":        keystonePublicURL,
		"horizonEndpointUrl": url,
		"memcachedServers":   fmt.Sprintf("'%s'", memcachedServers),
	}

	cms := []util.Template{
		// ConfigMap
		{
			Name:          fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			CustomData:    customData,
			ConfigOptions: templateParameters,
			Labels:        cmLabels,
		},
	}
	return configmap.EnsureConfigMaps(ctx, h, instance, cms, envVars)
}

// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
// returns the hash, whether the hash changed (as a bool) and any error
func (r *HorizonReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *horizonv1beta1.Horizon,
	envVars map[string]env.Setter,
) (string, bool, error) {
	var hashMap map[string]string
	changed := false
	mergedMapVars := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := util.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, changed, err
	}
	if hashMap, changed = util.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		GetLog(ctx).Info(fmt.Sprintf("Input maps hash %s - %s", common.InputHashName, hash))
	}
	return hash, changed, nil
}

// ensureHorizonSecret - Creates a k8s secret to hold the Horizon SECRET_KEY.
func (r *HorizonReconciler) ensureHorizonSecret(
	ctx context.Context,
	instance *horizonv1beta1.Horizon,
	h *helper.Helper,
	envVars *map[string]env.Setter,
) error {

	Labels := labels.GetLabels(instance, labels.GetGroupLabel(horizon.ServiceName), map[string]string{})
	//
	// check if secret already exist
	//
	scrt, _, err := oko_secret.GetSecret(ctx, h, horizon.ServiceName, instance.Namespace)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return err
	} else if k8s_errors.IsNotFound(err) || !validateHorizonSecret(scrt) {
		GetLog(ctx).Info("Creating Horizon Secret")
		// Create k8s secret to store Horizon Secret
		tmpl := []util.Template{
			{
				Name:       horizon.ServiceName,
				Namespace:  instance.Namespace,
				Type:       util.TemplateTypeNone,
				CustomData: map[string]string{"horizon-secret": rand.String(10)},
				Labels:     Labels,
			},
		}

		err := oko_secret.EnsureSecrets(ctx, h, instance, tmpl, envVars)

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *HorizonReconciler) getHorizonMemcached(
	ctx context.Context,
	h *helper.Helper,
	instance *horizonv1beta1.Horizon,
) (*memcachedv1.Memcached, error) {
	memcached := &memcachedv1.Memcached{}
	err := h.GetClient().Get(
		ctx,
		types.NamespacedName{
			Name:      instance.Spec.MemcachedInstance,
			Namespace: instance.Namespace,
		},
		memcached)
	if err != nil {
		return nil, err
	}
	return memcached, err
}

func validateHorizonSecret(secret *corev1.Secret) bool {
	return len(secret.Data["horizon-secret"]) != 0
}

func configureHorizonRbac(ctx context.Context, helper *helper.Helper, instance *horizonv1beta1.Horizon) (rbacResult ctrl.Result, err error) {
	rbacRules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{"anyuid"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		},
	}
	return common_rbac.ReconcileRbac(ctx, helper, instance, rbacRules)
}
