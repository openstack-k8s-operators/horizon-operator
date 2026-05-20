# AGENTS.md - horizon-operator

## Project overview

See [README.md](README.md) for a full project overview, configuration examples,
and contributing guidelines.

## Tech stack

| Layer | Technology |
|-------|------------|
| Language | Go (modules; `go.work` is local-only, never commit it) |
| Scaffolding | [Kubebuilder v4](https://book.kubebuilder.io/) + [Operator SDK](https://sdk.operatorframework.io/) |
| CRD generation | controller-gen (DeepCopy, CRDs, RBAC, webhooks) |
| Config management | Kustomize |
| Packaging | OLM bundle |
| Testing | Ginkgo/Gomega + envtest (functional), KUTTL (integration) |
| Linting | golangci-lint (`.golangci.yaml`) |
| CI | Zuul (`zuul.d/`), Prow (`.ci-operator.yaml`), GitHub Actions |

## Custom Resources

| Kind | Purpose |
|------|---------|
| `Horizon` | Single CR managing the Horizon dashboard deployment (httpd/WSGI). No sub-CRs. |

The `Horizon` CR has defaulting and validating admission webhooks.

## Directory structure

**Maintenance rule:** when directories are added, removed, or renamed, or when
their purpose changes, update this table to match.

| Directory | Contents |
|-----------|----------|
| `api/v1beta1/` | CRD types (`horizon_types.go`), conditions, webhook markers |
| `cmd/` | `main.go` entry point |
| `internal/controller/` | Reconciler: `horizon_controller.go` |
| `internal/horizon/` | Horizon resource builders (deployment, volumes, config) |
| `internal/webhook/` | Webhook implementation |
| `templates/` | Config files and scripts mounted into pods via `OPERATOR_TEMPLATES` env var |
| `config/crd,rbac,manager,webhook/` | Generated Kubernetes manifests (CRDs, RBAC, deployment, webhooks) |
| `config/samples/` | Example CRs (Kustomize overlays). `base/`, `custom-theme/`, `httpd-overrides/` for customization. |
| `test/functional/` | envtest-based Ginkgo/Gomega tests |
| `test/kuttl/` | KUTTL integration tests |
| `hack/` | Helper scripts (CRD schema checker, local webhook runner) |

## Build commands

After modifying Go code, always run: `make generate manifests fmt vet`.
Before sending a commit, always run: `pre-commit run --all-files`.

## Code style guidelines

- Follow standard openstack-k8s-operators conventions and lib-common patterns.
- Use `lib-common` modules for conditions, endpoints, TLS, storage, and other
  cross-cutting concerns rather than re-implementing them.
- CRD types go in `api/v1beta1/`. Controller logic goes in
  `internal/controller/`. Resource-building helpers go in `internal/horizon/`.
- Config templates are plain files in `templates/` -- they are mounted at
  runtime via the `OPERATOR_TEMPLATES` environment variable.
- Webhook logic is split between the kubebuilder markers in `api/v1beta1/` and
  the implementation in `internal/webhook/`.

## Testing

- Functional tests use the envtest framework with Ginkgo/Gomega and live in
  `test/functional/`.
- KUTTL integration tests live in `test/kuttl/`.
- Run all functional tests: `make test`.
- When adding a new field or feature, add corresponding test cases in
  `test/functional/` and update fixture data accordingly.

## Key dependencies

- [lib-common](https://github.com/openstack-k8s-operators/lib-common): shared modules for conditions, endpoints, TLS, secrets, etc.
- [infra-operator](https://github.com/openstack-k8s-operators/infra-operator): Memcached and topology APIs.
- [keystone-operator](https://github.com/openstack-k8s-operators/keystone-operator): identity service registration.
- [dev-docs/developer.md](https://github.com/openstack-k8s-operators/dev-docs/blob/main/developer.md): developer guide and coding conventions.
