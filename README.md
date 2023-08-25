# horizon-operator

The Horizon Operator deploys the OpenStack Horizon project in a OpenShift cluster.

## Description

This project should be used to deploy the OpenStack Horizon project. It expects that there is an existing Keystone and Memcached service available to connect to.

## Getting Started

This operator is deployed via Operator Lifecycle Manager as part of the OpenStack Operator bundle:
https://github.com/openstack-k8s-operators/openstack-operator

To configure Horizon specifically, we expose options to add custom configuration and the number of replicas for both Horizon API and Horizon Engine:

The following is taken from the Sample config in this repo:

```yaml
spec:
  replicas: 1
  secret: "osp-secret"
  customServiceConfig: |
    SESSION_TIMEOUT = 3600
```

We can see in this example, that we're making some customizations to the config. In this case, we're using `SESSION_TIMEOUT = 3600`.

Customisations are added to the `horizon-config-data` ConfigMap. If we look at a default ConfigMap with no customizations, we can see it has the following Keys:
```sh
❯ oc get cm horizon-config-data -o jsonpath={.data} | jq '. | keys'
[
  "9999_custom_settings.py",
  "horizon.json",
  "httpd.conf",
  "local_settings.py",
]
```
Any user provided customizations will go into the `9999_custom_settings.py` section. Without any customizations, it will just contain the default description:
```sh
❯ oc get cm horizon-config-data -o jsonpath={.data} | jq '."9999_custom_settings.py"'
"# add your customization here"
```

After applying our changes via the `OpenStackControlPlane` Custom Resource, we can see that this is now populated with our custom `SESSION_TIMEOUT` setting:
```sh
❯ oc patch openstackcontrolplane/openstack -p '{"spec": {"horizon": {"template": {"customServiceConfig": "SESSION_TIMEOUT = 3600" }}}}' --type=merge
openstackcontrolplane.core.openstack.org/openstack patched

❯ oc get cm horizon-config-data -o jsonpath={.data} | jq '."9999_custom_settings.py"'
"SESSION_TIMEOUT = 3600"
```


We can see this change reflected in the `/etc/openstack-dashboard/local_settings.d/9999_custom_settings.py` file within the Horizon pod:

```sh
❯ oc get po -l service=horizon
NAME                       READY   STATUS    RESTARTS   AGE
horizon-65c6b8fff8-8sqr9   0/1     Running   0          63s


❯ oc exec horizon-65c6b8fff8-8sqr9 -- cat /etc/openstack-dashboard/local_settings.d/9999_custom_settings.py
SESSION_TIMEOUT = 3600%
```

### Running on the cluster

To enable the Horizon service, we simply need to set the Horizon service to enabled in the `OpenStackControlPlane`.
The following snippet is taken from the `OpenStackControlPlane` Custom Resource:

```yaml
❯ oc get openstackcontrolplane openstack -o yaml | yq .spec.horizon
enabled: true
template:
  containerImage: ""
  customServiceConfig: SESSION_TIMEOUT = 3600
  debug:
    service: false
  preserveJobs: false
  replicas: 1
  resources: {}
  route:
    routeName: horizon
  secret: osp-secret
```

### Memcached
Horizon uses the default memcached service deployed via the `OpenStackControlPlane`. This service should be enabled by default, but can be verified like so:
```sh
❯ oc get openstackcontrolplane openstack -o yaml | yq .spec.memcached.enabled
true
```

By default, this instance of memcached is simply called `memcached` and we will default to that name:
```go
	// +kubebuilder:validation:Required
	// +kubebuilder:default=memcached
	// Memcached instance name.
	MemcachedInstance string `json:"memcachedInstance"`
```

However, if a dedicated instance of memcached has been deployed, Horizon can be informed about this using the `spec.memcachedInstance` key like so:
```yaml
enabled: true
template:
  containerImage: ""
  customServiceConfig: SESSION_TIMEOUT = 3600
  debug:
    service: false
  preserveJobs: false
  replicas: 1
  resources: {}
  route:
    routeName: horizon
  secret: osp-secret
  memcachedInstance: my-custom-memcached #<<-- Custom memcached instance supplied here.
```

### Undeploy controller

To undeploy the operator, simply set the `enabled` value to false from within the `OpenStackControlPlane` resource.

## Contributing

The following guide relies on a already deployed `OpenStackControlPlane`. If you don't already have this, you can
follow the guides located on the following repo:
https://github.com/openstack-k8s-operators/install_yamls

To contribute, you can disabled the Horizon service in the `OpenStackControlPlane` resouce and run the Operator locally
from your laptop using `make install run`. This will start the operator locally, and debugging can be done without
building and pushing a new container.

Once you have tested your feature, you can use `pre-commit run --all-files` to ensure the change passes preliminary
testing.

Ensure you have created an issue against the project and send a PR linking to the relevant issue. You can reach out
to the maintainers from the included OWNERS file for change reviews.

### How it works

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster

### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
