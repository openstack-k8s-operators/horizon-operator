package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

const (
	// HorizonMemcachedError - Provides a error that occured during the provisioning of the memcached instance
	HorizonMemcachedError = "Error creating Memcached instance: %s"

	// HorizonMemcachedServiceError - Provides an error received while trying to retrieve the memcached svc
	HorizonMemcachedServiceError = "Error retrieving the memcached service: %s"

	// HorizonMemcachedReadyCondition -  Indicates the Horizon memcached service is ready to be consumed
	// by Horizon
	HorizonMemcachedReadyCondition condition.Type = "HorizonMemcached"

	// HorizonMemcachedReadyMessage - Provides the message to clarify memcached has been provisioned
	HorizonMemcachedReadyMessage = "Horizon Memcached instance has been provisioned"
)
