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
	routev1 "github.com/openshift/api/route/v1"
	horizonv1alpha1 "github.com/openstack-k8s-operators/horizon-operator/api/v1alpha1"
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
	oko_secret "github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	oko_svc "github.com/openstack-k8s-operators/lib-common/modules/common/service"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetClient -
func (r *HorizonReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *HorizonReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *HorizonReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *HorizonReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// HorizonReconciler reconciles a Horizon object
type HorizonReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneendpoints,verbs=get;list;watch;
//+kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds/finalizers,verbs=update

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
	_ = log.FromContext(ctx)

	instance := &horizonv1alpha1.Horizon{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("Error retrieving %s: %w", instance.Name, err)
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Error creating new helper: %w", err)
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the overall status condition if service is ready
		if instance.IsReady() {
			instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		}

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, fmt.Errorf("Error adding finalizer to service: %s, %w", instance.Name, err)
	}

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}

		cl := condition.CreateList(
			condition.UnknownCondition(condition.ExposeServiceReadyCondition, condition.InitReason, condition.ExposeServiceReadyInitMessage),
			condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
			condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
			condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage))

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}
	if instance.Status.HorizonEndpoints == nil {
		instance.Status.HorizonEndpoints = map[string]string{}
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	r.Log.Info("Starting Reconcile")
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager -
func (r *HorizonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&horizonv1alpha1.Horizon{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&routev1.Route{}).
		Owns(&memcachedv1.Memcached{}).
		Complete(r)
}

func (r *HorizonReconciler) reconcileDelete(ctx context.Context, instance *horizonv1alpha1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service delete")

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info("Reconciled Service delete successfully")

	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileInit(
	ctx context.Context,
	instance *horizonv1alpha1.Horizon,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service init")

	//
	// expose the service (create service, route and return the created endpoint URLs)
	//
	var horizonPorts = map[endpoint.Endpoint]endpoint.Data{
		endpoint.EndpointPublic: endpoint.Data{
			Port: horizon.HorizonPublicPort,
		},
	}

	apiEndpoints, ctrlResult, err := endpoint.ExposeEndpoints(
		ctx,
		helper,
		horizon.ServiceName,
		serviceLabels,
		horizonPorts,
		time.Duration(5)*time.Second,
	)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ExposeServiceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ExposeServiceReadyErrorMessage,
			err.Error()))
		return ctrlResult, fmt.Errorf("Error exposing endpoints for service: %w", err)
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ExposeServiceReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ExposeServiceReadyRunningMessage))
		return ctrlResult, nil
	}
	instance.Status.Conditions.MarkTrue(condition.ExposeServiceReadyCondition, condition.ExposeServiceReadyMessage)

	//
	// Update instance status with service endpoint url from route host information
	//
	// TODO: need to support https default here
	if instance.Status.HorizonEndpoints == nil {
		instance.Status.HorizonEndpoints = map[string]string{}
	}
	instance.Status.HorizonEndpoints = apiEndpoints

	// expose service - end

	r.Log.Info("Reconciled Service init successfully")
	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileUpdate(ctx context.Context, instance *horizonv1alpha1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service update")

	// TODO: should have minor update tasks if required
	// - delete dbsync hash from status to rerun it?

	r.Log.Info("Reconciled Service update successfully")
	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileUpgrade(ctx context.Context, instance *horizonv1alpha1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service upgrade")

	// TODO: should have major version upgrade tasks
	// -delete dbsync hash from status to rerun it?

	r.Log.Info("Reconciled Service upgrade successfully")
	return ctrl.Result{}, nil
}

func (r *HorizonReconciler) reconcileNormal(ctx context.Context, instance *horizonv1alpha1.Horizon, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service")

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
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Secret)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, fmt.Errorf("Error getting OpenStack secret: %w", err)
	}
	configMapVars[ospSecret.Name] = env.SetValue(hash)

	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)

	// run check OpenStack secret - end

	// Create Memcached instance if no shared instance exists.
	if instance.Spec.SharedMemcached == "" {
		memcached := r.renderMemcached(instance)
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), memcached, func() error {
			memcached.Labels = map[string]string{"service": instance.Name}
			memcached.Spec.Replicas = instance.Spec.Replicas

			err := controllerutil.SetControllerReference(helper.GetBeforeObject(), memcached, helper.GetScheme())
			if err != nil {
				return fmt.Errorf("Error setting Owner Reference for instance %s: %w", instance.Name, err)
			}

			return nil
		})
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				r.Log.Info(fmt.Sprintf("Memcached %s not found", memcached.Name))
				return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
			}
			return ctrl.Result{}, fmt.Errorf("Error creating memcached instance: %w", err)
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Memcached %s - %s", memcached.Name, op))
		}

		// Mark the Memcached Service as Ready if we get to this point with no errors
		instance.Status.Conditions.MarkTrue(
			horizonv1alpha1.HorizonMemcachedReadyCondition, horizonv1alpha1.HorizonMemcachedReadyMessage)
	}

	//
	// Create ConfigMaps and Secrets required as input for the Service and calculate an overall hash of hashes
	//

	//
	// create Configmap required for horizon input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal horizon config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the OpenStack secret via the init container
	//
	err = r.generateServiceConfigMaps(ctx, instance, helper, &configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, fmt.Errorf("Error generating Config Map for instance %s: %w", instance.Name, err)
	}

	err = r.ensureHorizonSecret(ctx, instance, helper, &configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, fmt.Errorf("Error creating secret for %s: %w", instance.Name, err)
	}

	//
	// create hash over all the different input resources to identify if any those changed
	// and a restart/recreate is required.
	//
	inputHash, hashChanged, err := r.createHashOfInputHashes(ctx, instance, configMapVars)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Error creating hash of config inputs: %w", err)
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
		return ctrlResult, fmt.Errorf("Error initializing service %s: %w", instance.Name, err)
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service update
	ctrlResult, err = r.reconcileUpdate(ctx, instance, helper)
	if err != nil {
		return ctrlResult, fmt.Errorf("Error updating service %s: %w", instance.Name, err)
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service upgrade
	ctrlResult, err = r.reconcileUpgrade(ctx, instance, helper)
	if err != nil {
		return ctrlResult, fmt.Errorf("Error upgrading service %s: %w", instance.Name, err)
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
		time.Duration(5)*time.Second,
	)

	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrlResult, fmt.Errorf("Error creating deployment for %s: %w", instance.Name, err)
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
	}
	// create Deployment - end

	r.Log.Info("Reconciled Service successfully")
	return ctrl.Result{}, nil
}

// generateServiceConfigMaps - create configmaps which hold scripts and service configuration
// TODO add DefaultConfigOverwrite
func (r *HorizonReconciler) generateServiceConfigMaps(
	ctx context.Context,
	instance *horizonv1alpha1.Horizon,
	h *helper.Helper,
	envVars *map[string]env.Setter,
) error {
	var memcachedSvc string
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
		return fmt.Errorf("Error getting KeystoneAPI: %s", err)
	}
	keystonePublicURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointPublic)
	if err != nil {
		return fmt.Errorf("Error getting KeystoneAPI Endpoint: %w", err)
	}

	if instance.Spec.SharedMemcached == "" {
		memcachedSvc, err = r.getMemcachedSvc(ctx, fmt.Sprintf("%s-memcached", instance.Name), h, instance.Namespace)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				horizonv1alpha1.HorizonMemcachedReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				horizonv1alpha1.HorizonMemcachedServiceError,
				err.Error()))
			return fmt.Errorf("Error retrieving Memcached instance: %w", err)
		}
	} else {
		memcachedSvc, err = r.getMemcachedSvc(ctx, instance.Spec.SharedMemcached, h, instance.Namespace)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				horizonv1alpha1.HorizonMemcachedReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				horizonv1alpha1.HorizonMemcachedServiceError,
				err.Error()))
			return fmt.Errorf("Error getting Memcached service: %w", err)
		}
	}

	url := strings.TrimPrefix(instance.Status.HorizonEndpoints["public"], "http://")

	templateParameters := map[string]interface{}{
		"keystoneURL":        keystonePublicURL,
		"horizonDebug":       instance.Spec.Debug,
		"horizonEndpointUrl": url,
		"memcachedSvc":       memcachedSvc,
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
	r.Log.Info(fmt.Sprintf("Creating ConfigMaps with details: %v", cms))
	return configmap.EnsureConfigMaps(ctx, h, instance, cms, envVars)
}

// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
// returns the hash, whether the hash changed (as a bool) and any error
func (r *HorizonReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *horizonv1alpha1.Horizon,
	envVars map[string]env.Setter,
) (string, bool, error) {
	var hashMap map[string]string
	changed := false
	mergedMapVars := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := util.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, changed, fmt.Errorf("Error creating hash of config inputs: %w", err)
	}
	if hashMap, changed = util.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		r.Log.Info(fmt.Sprintf("Input maps hash %s - %s", common.InputHashName, hash))
	}
	return hash, changed, nil
}

// ensureHorizonSecret - Creates a k8s secret to hold the Horizon SECRET_KEY.
func (r *HorizonReconciler) ensureHorizonSecret(
	ctx context.Context,
	instance *horizonv1alpha1.Horizon,
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
		r.Log.Info("Creating Horizon Secret")
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
			return fmt.Errorf("Error creating secret for %s: %w", instance.Name, err)
		}
	}

	return nil
}

func (r *HorizonReconciler) renderMemcached(instance *horizonv1alpha1.Horizon) *memcachedv1.Memcached {
	return &memcachedv1.Memcached{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "memcached.openstack.org/v1beta1",
			Kind:       "Memcached",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-memcached", instance.Name),
			Namespace: instance.Namespace,
		},
	}
}

func (r *HorizonReconciler) getMemcachedSvc(ctx context.Context, memcachedName string, h *helper.Helper, namespace string) (svc string, err error) {
	labelSelector := k8s_labels.Set{"app": "memcached"}
	svcList, err := oko_svc.GetServicesListWithLabel(ctx, h, namespace, labelSelector)

	if err != nil {
		r.Log.Info("Error getting service list: %s", err)
		return "", fmt.Errorf("Error getting memcached service with labels: %w", err)
	}
	for _, service := range svcList.Items {
		if service.Name == memcachedName {
			return service.Name, nil
		}
	}
	return "", fmt.Errorf("No memcached service was found")
}

func validateHorizonSecret(secret *corev1.Secret) bool {
	return len(secret.Data["horizon-secret"]) != 0
}
