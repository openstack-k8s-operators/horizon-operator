# Customizing Horizon Dashboard Theme with horizon-operator

The horizon-operator provides mechanisms to customize the Horizon Dashboard
theme. A theme is a directory containing assets that override the default
styling of the dashboard. Horizon can be configured to support multiple themes
simultaneously, making them available at runtime. Through the horizon-operator,
you can load custom themes using the
[ExtraMounts](https://github.com/openstack-k8s-operators/dev-docs/blob/main/extra_mounts.md)
feature.

## Theme configuration

To provide additional themes, you must specify them in the `AVAILABLE_THEMES`
setting within the `_11_custom_theme.py` file. Themes are defined as a list of
tuples with the following format:

```
('name', 'label', 'path')
```

Where:

- name: The key by which the theme value is resolved
- label: The display name or description of the custom theme
- path: The directory location where the theme is unpacked and stored

Currently, the `horizon-operator` only supports the default path:
`/usr/share/openstack-dashboard/openstack_dashboard/themes`

## Procedure

To use a custom theme, create a `_11_custom_theme.py` file and configure the
`AVAILABLE_THEMES` as follows:

```python
# override the CUSTOM_THEME_PATH variable with this settings snippet
AVAILABLE_THEMES = [('custom', 'Custom Theme', 'themes/custom')]
```

In case of multiple themes, additional entries in the AVAILABLE_THEMES tuple can
be provided:

```python
# override the CUSTOM_THEME_PATH variable with this settings snippet
AVAILABLE_THEMES = [('custom', 'Custom Theme', 'themes/custom'), ('custom_alt', 'Custom Alt Theme', 'themes/custom_alt')]
```
See the [Horizon Theme
Documentation](https://docs.openstack.org/horizon/latest/configuration/themes.html)
for more details.

> **Note**: If `AVAILABLE_THEMES` contains only a single theme tuple, the theme
toggle functionality will not be available in the dashboard UI.


The configuration file must be mounted in
`/etc/openstack-dashboard/local_settings.d` within the Pod. The horizon
bootstrap process uses this file to resolve the theme path during the `python
manage.py collectstatic` operation.

After creating the `_11_custom_theme.py` file, you must provide the theme assets
as a **tar.gz** tarball. A script in the Horizon operator will unpack this
tarball in `/usr/share/openstack-dashboard/openstack_dashboard/themes`, which
is currently the only supported path.

> **Note:** Only tar.gz format is currently supported.

This guide assumes you have already created a **custom custom** theme and it's
ready to be mounted to the horizon deployment.

### Deployment Steps

#### 1. Create the horizon assets tarball

Organize your theme files in the proper directory structure:

```
$ tree -d
.
└── custom
    ├── static
    │   ├── bootstrap
    │   │   └── components
    │   ├── fonts
    │   ├── horizon
    │   │   └── components
    │   ├── images
    │   ├── img
    │   └── custom
    └── templates
        ├── auth
        ├── context_selection
        └── horizon
            └── common
```

Create the tarball:

```
$ tar -cvzf custom.tar.gz custom/
custom/
custom/static/
custom/static/_custom.scss
custom/static/_styles.scss
custom/static/_variables.scss
custom/static/bootstrap/
custom/static/bootstrap/_styles.scss
custom/static/bootstrap/_variables.scss
custom/static/bootstrap/components/
custom/static/bootstrap/components/_dropdowns.scss
custom/static/bootstrap/components/_forms.scss
custom/static/bootstrap/components/_navbar.scss
custom/static/bootstrap/components/_navs.scss
custom/static/bootstrap/components/_type.scss
custom/static/fonts/
...
```

#### 2. Create the theme configuration file

Create `_11_custom_theme.py` with the following content:

```python
# Override the CUSTOM_THEME_PATH variable with this settings snippet
AVAILABLE_THEMES = [('custom', 'Custom Theme', 'themes/custom')]
```

#### 3. Create a ConfigMap with the required files

```bash
$ oc create cm horizon-theme --from-file=custom.tar.gz --from-file=_11_custom_theme.py
```

#### 4. Update the OpenStackControlPlane configuration

Edit the `OpenStackControlPlane` resource and update the horizon section:

```yaml
kind: OpenStackControlPlane
spec:
  ...
  horizon:
    enabled: true
    template:
      extraMounts:
      - extraVol:
        - extraVolType: HorizonTheme
          mounts:
          - mountPath: /etc/openstack-dashboard/local_settings.d/_11_custom_theme.py
            name: horizon-theme
            readOnly: true
            subPath: _11_custom_theme.py
          - mountPath: /etc/openstack-dashboard/theme/custom.tar.gz
            name: horizon-theme
            readOnly: true
            subPath: custom.tar.gz
          volumes:
          - configMap:
              name: horizon-theme
            name: horizon-theme
  ...
```

> **Note:** Set the `extraVolType` parameter to `HorizonTheme` to allow the
horizon-operator to validate the mountPath provided by the user.


### Deploy Multiple custom themes

Deploying multiple custom themes is a variation of the single theme deployment
procedure. This guide demonstrates how to implement two different themes, but
the same approach can be generalized for any number of custom themes.

#### Prerequisites
This guide assumes you have already prepared two different theme tarballs:

- custom.tar.gz - First custom theme
- custom-alt.tar.gz - Second custom theme

#### 1. Prepare the theme configuration file

Create `_11_custom_theme.py` with multiple themes defined:

```python
# Override the CUSTOM_THEME_PATH variable with this settings snippet
AVAILABLE_THEMES = [
    ('custom', 'Custom Theme', 'themes/custom'),
    ('custom_alt', 'Custom Alt Theme', 'themes/custom_alt')
]
```

#### 2. Create ConfigMap with the all required files

```bash
$ oc create cm horizon-theme --from-file=custom.tar.gz --from-file=_11_custom_theme.py
$ oc create cm horizon-theme-alt --from-file=custom_alt.tar.gz
```

> **Note**: It is convenient to have multiple ConfigMaps due to the size of the
tarball that might exceed the ConfigMap allowed size.

#### 4. Update the OpenStackControlPlane configuration

Edit the `OpenStackControlPlane` resource and update the horizon section:

```yaml
kind: OpenStackControlPlane
spec:
...
  horizon:
    apiOverride: {}
    enabled: true
    template:
      customServiceConfig: '# add your customization here'
      extraMounts:
      - extraVol:
        - extraVolType: HorizonTheme
          mounts:
          - mountPath: /etc/openstack-dashboard/local_settings.d/_11_custom_theme.py
            name: horizon-theme
            readOnly: true
            subPath: _11_custom_theme.py
          - mountPath: /etc/openstack-dashboard/theme/custom.tar.gz
            name: horizon-theme
            readOnly: true
            subPath: custom.tar.gz
          - mountPath: /etc/openstack-dashboard/theme/custom_alt.tar.gz
            name: horizon-theme-alt
            readOnly: true
            subPath: custom_alt.tar.gz
          volumes:
          - configMap:
              name: horizon-theme
            name: horizon-theme
          - configMap:
              name: horizon-theme-alt
            name: horizon-theme-alt
```

#### User Experience

Having multiple themes can improve the user experience:

- Users will see a theme selector in the Horizon dashboard interface
- The theme selector allows switching between all available themes
- The selected theme is stored in the user's browser preferences

> **Note:** Ensure each theme has a distinctive label in the `AVAILABLE_THEMES`
configuration to help users identify them in the theme selector.

### Deploy the existing sample

The horizon-operator repository includes a sample that can be used to deploy a
custom theme. This sample is provided as a working reference example and is not
necessarily meant to serve as a deployment recommendation for production
environments.

If you're using
[`install_yamls`](https://github.com/openstack-k8s-operators/install_yamls) and
already have CRC (Code Ready Containers) running, you can deploy the custom
theme example with the following steps:

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
