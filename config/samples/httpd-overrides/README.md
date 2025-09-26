# Customizing Apache HTTPD Configuration with horizon-operator

The horizon-operator provides mechanisms to customize the Apache HTTPD server
configuration through the use of custom configuration files. This feature
leverages the
[ExtraMounts](https://github.com/openstack-k8s-operators/dev-docs/blob/main/extra_mounts.md)
functionality to mount custom HTTPD configuration files into the Horizon
deployment.

## Overview

Custom HTTPD configuration files can be used to override or extend the default
Apache configuration. These files are mounted into the `/etc/httpd/conf_custom/`
directory within the Horizon pods and are automatically included in the main
`HTTPD` configuration.

## Configuration Files

### httpd_custom_timeout.conf

This example demonstrates how to customize the Apache timeout settings:

```apache
# Custom timeout configuration sample file
# Set the httpd timeout to 120 seconds
Timeout 120
```

This configuration file modifies the default `Apache timeout` from the standard
value to `120 seconds`, which can be useful for environments with longer-running
requests or slower network conditions.

## Procedure

### 1. Create Custom Configuration Files

Create your custom HTTPD configuration files, for example:

```bash
# Create a custom timeout configuration
cat > httpd_custom_timeout.conf << EOF
# Custom timeout configuration sample file
# Set the httpd timeout to 120 seconds
Timeout 120
EOF
```

### 2. Create a ConfigMap

Create a Kubernetes ConfigMap containing your custom configuration files:

```bash
oc create configmap httpd-overrides --from-file=httpd_custom_timeout.conf
```

It is possible to add multiple configuration files containing dedicated
configuration directives:

```bash
oc create configmap httpd-overrides \
  --from-file=httpd_custom_timeout.conf \
  --from-file=httpd_custom_security.conf \
  --from-file=httpd_custom_logging.conf
```

The following example is based on a single customization file and demonstrates
how to set a custom `Timeout` parameter.

### 3. Configure ExtraMounts in OpenStackControlPlane

Update your `OpenStackControlPlane` resource to include the custom HTTPD
configuration files using `extraMounts`. The simplest approach is to mount
the entire ConfigMap to the target directory:

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
  horizon:
    enabled: true
    template:
      extraMounts:
      - extraVol:
        - extraVolType: httpd-overrides
          mounts:
          - mountPath: /etc/httpd/conf_custom
            name: httpd-overrides
            readOnly: true
          volumes:
          - configMap:
              name: httpd-overrides
            name: httpd-overrides
```

The `extraMounts` above mounts all files from the `httpd-overrides` `ConfigMap`
into th`/etc/httpd/conf_custom/` directory, making them available to httpd.

#### Selective File Mounting with subPath

If you need to mount only specific files from the ConfigMap or control exactly
where each file is placed, you can use the `subPath` option:

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
  horizon:
    enabled: true
    template:
      extraMounts:
        - extraVol:
          - extraVolType: httpd-overrides
            mounts:
            - mountPath: /etc/httpd/conf_custom/httpd_custom_timeout.conf
              name: httpd-overrides
              readOnly: true
              subPath: httpd_custom_timeout.conf
            - mountPath: /etc/httpd/conf_custom/httpd_custom_security.conf
              name: httpd-overrides
              readOnly: true
              subPath: httpd_custom_security.conf
            - mountPath: /etc/httpd/conf_custom/httpd_custom_logging.conf
              name: httpd-overrides
              readOnly: true
              subPath: httpd_custom_logging.conf
            volumes:
            - configMap:
                name: httpd-overrides
              name: httpd-overrides
```

## ExtraMounts Configuration Details

The `extraMounts` feature uses the following key components:

- **extraVolType**: Set to `httpd-overrides` to indicate the type of volume
  being mounted
- **mountPath**: The full path where the configuration file will be mounted
  inside the container (`/etc/httpd/conf_custom/`)
- **subPath**: The specific file from the ConfigMap to mount
- **readOnly**: Set to `true` to mount the configuration files as read-only
- **volumes**: References the `ConfigMap` containing the configuration files

Similar to the custom theme functionality (as seen in the `../custom-theme/`
directory), the `HTTPD` overrides feature:

1. **Uses ConfigMaps**: Both features store configuration data in Kubernetes
   ConfigMaps
2. **Leverages ExtraMounts**: Both use the `extraMounts` mechanism to inject
   files into pods
3. **Requires Specific Mount Paths**: HTTPD overrides mount to `/etc/httpd/conf_custom/`
   as specified in the httpd.conf `IncludeOptional` directive

## Common Use Cases

- **Timeout Adjustments**: Modify request timeout values for specific environments
- **Security Headers**: Add custom security headers or configurations
- **Logging**: Customize Apache logging configuration
- **Performance Tuning**: Adjust worker processes, connection limits, etc.

## Verification

After deploying your custom HTTPD configuration, you can verify that the
settings have been properly applied:

### 1. Find the Horizon Pod

First, identify the running Horizon pod:

```bash
$ oc get pods -l service=horizon
```

### 2. Verify Configuration Loading

Connect to the Horizon pod and check that your custom configuration has been
loaded:

```bash
# Replace <horizon-pod-name> with the actual pod name from step 1
oc rsh -c horizon <horizon-pod-name>

# Inside the pod, dump the HTTPD configuration and check for your custom settings
httpd -D DUMP_CONFIG | grep -i timeout
```

For the `httpd_custom_timeout.conf` example, you should see output similar to:

```
Timeout 120
```

### 3. Additional Verification Commands

You can also verify other aspects of the configuration:

```bash
$ httpd -D DUMP_INCLUDES

Included configuration files:
  (*) /etc/httpd/conf/httpd.conf
    (15) /etc/httpd/conf.modules.d/00-base.conf
    (15) /etc/httpd/conf.modules.d/00-brotli.conf
    (15) /etc/httpd/conf.modules.d/00-dav.conf
    (15) /etc/httpd/conf.modules.d/00-mpm.conf
    (15) /etc/httpd/conf.modules.d/00-optional.conf
    (15) /etc/httpd/conf.modules.d/00-proxy.conf
    (15) /etc/httpd/conf.modules.d/00-ssl.conf
    (15) /etc/httpd/conf.modules.d/00-systemd.conf
    (15) /etc/httpd/conf.modules.d/01-cgi.conf
    (15) /etc/httpd/conf.modules.d/10-wsgi-python3.conf
    (16) /etc/httpd/conf.d/autoindex.conf
    (16) /etc/httpd/conf.d/openstack-dashboard.conf
    (16) /etc/httpd/conf.d/ssl.conf
    (16) /etc/httpd/conf.d/userdir.conf
    (16) /etc/httpd/conf.d/welcome.conf
    (61) /etc/httpd/conf_custom/httpd_custom_timeout.conf
```

### 4. Verify ConfigMap Mount via ExtraMounts

Outside the pod, you can also verify that the ConfigMap is properly mounted
through extraMounts:

```bash
# Check that the ConfigMap exists
oc get configmap httpd-overrides -o yaml

# Verify the mount in the pod description
oc describe pod <horizon-pod-name>
```

## Deploy the Sample

The horizon-operator repository includes a sample that can be used to deploy
horizon with httpd overrides (it set a particular Timeout to 120s). This sample
is provided as a working reference example and is not necessarily meant to
serve as a deployment recommendation for production environments.

If you're using
[`install_yamls`](https://github.com/openstack-k8s-operators/install_yamls) and
already have CRC (Code Ready Containers) running, you can deploy the httpd
overrides example with the following steps:

```bash
# Navigate to the install_yamls directory
$ cd install_yamls

# Set up the CRC storage and deploy OpenStack Catalog
$ make crc_storage openstack

# Deploy OpenStack operators
$ make openstack_init

# Generate the OpenStack deployment file
$ oc kustomize . > ~/openstack-deployment.yaml

# Set the path to the deployment file
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
```

This will create the necessary ConfigMap and a deployable OpenStackControlPlane
yaml with the custom timeout configuration applied.
