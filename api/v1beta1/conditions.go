package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// Horizon Condition Types used by API objects.
const (
	// HorizonMemcachedReadyCondition - Indicates the Horizon memcached service is ready to be consumed by Horizon
	HorizonMemcachedReadyCondition condition.Type = "HorizonMemcached"
)

// Horizon Messages used by API objects.
const (
	// HorizonMemcachedReadyInitMessage -
	HorizonMemcachedReadyInitMessage = "Horizon Memcached create not started"

	// HorizonMemcachedReadyMessage - Provides the message to clarify memcached has been provisioned
	HorizonMemcachedReadyMessage = "Horizon Memcached instance has been provisioned"

	// HorizonMemcachedReadyWaitingMessage - Provides the message to clarify memcached has not been provisioned
	HorizonMemcachedReadyWaitingMessage = "Horizon Memcached instance has not been provisioned"

	// HorizonMemcachedReadyErrorMessage -
	HorizonMemcachedReadyErrorMessage = "Horizon Memcached error occurred %s"
)
