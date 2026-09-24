# Contributing Guidelines

The **kubernetes/cloud-provider-gcp** project accepts contributions via GitHub [pull requests](https://help.github.com/articles/about-pull-requests/). This document outlines how the repository is organized, how to test and verify changes, and how review routing works. Please also read the [Kubernetes contributor guide](https://github.com/kubernetes/community/blob/master/contributors/guide/README.md) and the [Kubernetes code of conduct](https://github.com/kubernetes/community/blob/master/code-of-conduct.md).

## Sign the Contributor License Agreement

We would love to accept your patches. Before we can accept them you need to sign the Cloud Native Computing Foundation (CNCF) [CLA](https://github.com/kubernetes/community/blob/master/CLA.md).

## Community, ownership, and scope

This repository implements the Kubernetes [cloud provider](https://github.com/kubernetes/cloud-provider) interface for Google Cloud Platform. It ships several **distinct binaries and artifacts** (see the [README](README.md) introduction and **Components** section). Day-to-day maintenance is driven by the broader Kubernetes GCP and cloud provider community; use the contacts below when you are unsure where a change should go or who can help.

* [Slack](https://kubernetes.slack.com/messages/sig-gcp): `#sig-gcp`
* [Mailing list](https://groups.google.com/forum/#!forum/kubernetes-sig-gcp)
* [SIG GCP community page](https://github.com/kubernetes/community/blob/master/sig-gcp/README.md) for meeting times and charter

## Reporting an issue

If you find a bug or want to propose a feature for cloud-provider-gcp, open a [GitHub issue](https://github.com/kubernetes/cloud-provider-gcp/issues) in this repository. Describe which component or directory your request applies to (for example `gke-gcloud-auth-plugin` or `cluster/gce` manifests) so maintainers can triage it quickly.

## Contributing a patch (high level)

1. Open an issue (or reference an existing one) describing the proposed change when it is non-trivial or needs design discussion.
2. Fork the repository, develop on a branch, and keep commits focused.
3. Run the **tests and verification** that apply to your change (see [Testing and verification](#testing-and-verification)).
4. Open a pull request. [Prow](https://prow.k8s.io/) will run CI and can suggest reviewers based on **OWNERS** files. See [Code review](#code-review) and the full list of bot commands on [prow command help](https://prow.k8s.io/command-help).

## Repository layout (where to put your change)

| Area | What it is | Notable paths |
|------|------------|----------------|
| Cloud Controller Manager (CCM) | GCP cloud controller and related wiring | `cmd/cloud-controller-manager/`, shared controllers under `pkg/controller/`, GCP provider logic under `providers/gce/` |
| GCP auth provider (kubelet) | Credential provider binary for image pulls from GCR / Artifact Registry | `cmd/auth-provider-gcp/` |
| GKE auth plugin (kubectl) | `client-go` exec credential plugin for GKE | `cmd/gke-gcloud-auth-plugin/` |
| Cluster / GCE assets | Addons, GCE/GCI manifests, shell helpers used in releases | `cluster/addons/`, `cluster/gce/`, `cluster/common.sh`, … |
| End-to-end tests | Tests that run against a cluster | `e2e/`, `test/` |
| Build / release / CI helpers | Make targets, verify scripts, image publish | `Makefile`, `tools/` |

For dependency and vendor updates, follow [Dependency management](README.md#dependency-management) in the README (`go get`, `make update-vendor`, and related targets such as `make pin-k8s-deps` when you touch Kubernetes module pins).

## Component guides

Use the sections below to decide how to **build**, **test**, and **route** a change. Reviewers are chosen from the nearest **OWNERS** file on the paths you edit (see [Kubernetes OWNERS](https://go.k8s.io/owners)).

### `cloud-controller-manager`

* **Role:** Runs cloud-specific control plane controllers for clusters on GCP (see [Cloud Controller Manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) in the Kubernetes docs).
* **Typical paths:** `cmd/cloud-controller-manager/`, `pkg/controller/`, `providers/gce/`.
* **Build:** `make cloud-controller-manager-linux-amd64` (or `make build-all`). For container images see [Publishing cloud-controller-manager image](README.md#publishing-cloud-controller-manager-image) in the README.
* **Test:** `go test -race ./cmd/cloud-controller-manager/...` and packages you touch under `./pkg/...` and `./providers/...`. Before opening a PR, run `make test` and `make verify`. Changes that affect cluster behavior may need `make run-e2e-test` and/or the kOps flows documented under **kOps E2E** in the `Makefile` (`make help`).
* **Review:** Approvers for the CCM command live in [`cmd/cloud-controller-manager/OWNERS`](cmd/cloud-controller-manager/OWNERS). Subdirectories such as [`pkg/controller/service/OWNERS`](pkg/controller/service/OWNERS) and [`pkg/controller/nodeipam/OWNERS`](pkg/controller/nodeipam/OWNERS) add reviewers for those areas. GCP provider changes also fall under [`providers/gce/OWNERS`](providers/gce/OWNERS).

### `auth-provider-gcp`

* **Role:** Supplies credentials for the kubelet (CRI credential provider flow) when pulling from Google container registries.
* **Typical paths:** `cmd/auth-provider-gcp/`, and related packages such as `pkg/gcpcredential/` when applicable.
* **Build:** `make auth-provider-gcp-linux-amd64` (or `make build-all`).
* **Test:** `go test -race ./cmd/auth-provider-gcp/...` plus `make test` / `make verify`.
* **Review:** [`cmd/auth-provider-gcp/OWNERS`](cmd/auth-provider-gcp/OWNERS).

### `gke-gcloud-auth-plugin`

* **Role:** Exec authentication plugin used by `kubectl` and other clients against GKE clusters.
* **Typical paths:** `cmd/gke-gcloud-auth-plugin/`.
* **Build:** e.g. `make gke-gcloud-auth-plugin-darwin-arm64` or other `gke-gcloud-auth-plugin-*` targets from `make help`; `make build-all` builds all published platforms.
* **Test:** `go test -race ./cmd/gke-gcloud-auth-plugin/...` plus `make test` / `make verify`.
* **Review:** [`cmd/gke-gcloud-auth-plugin/OWNERS`](cmd/gke-gcloud-auth-plugin/OWNERS).

### Cluster manifests, addons, and GCE scripts

* **Role:** YAML addons, GCE/GCI manifests, and shell that are packaged into release artifacts (for example `kubernetes-manifests.tar.gz`).
* **Typical paths:** `cluster/addons/`, `cluster/gce/`, `cluster/*.sh`.
* **Test:** `make test-sh` for shell syntax; run `make test` / `make verify` when Go or generated code is involved. Validate manifest changes in context of `make release-manifests` if you need a full bundle.
* **Review:** [`cluster/OWNERS`](cluster/OWNERS).

### End-to-end and integration tests

* **Typical paths:** `e2e/`, `test/`.
* **Test:** Follow scripts under `tools/` (for example `make run-e2e-test`) and targets in the **kOps E2E** section of the `Makefile` when you change how tests are run or provisioned.
* **Review:** [`test/OWNERS`](test/OWNERS) for `test/`. The `e2e/` tree does not define its own `OWNERS` file; use the root [`OWNERS`](OWNERS) and paths touched by your change for reviewer routing.

## Testing and verification

Run these from the repository root unless your change’s component guide above suggests something narrower.

| Command | Purpose |
|---------|---------|
| `make test` | Unit tests (`go test -race` for main and `providers` trees). |
| `make verify` | Format, lint, vet, vendor, and related checks via [`tools/verify-all.sh`](tools/verify-all.sh). |
| `make test-sh` | Shell syntax checks for selected `cluster/` scripts. |
| `make run-e2e-test` | E2E suite (requires a suitable GCP / cluster setup; see script output). |

Use `make help` for the full target list (builds, kOps, image publish, etc.).

## Code review

* **Automation:** Kubernetes Prow runs presubmits on your PR; fix failing jobs before requesting re-review.
* **OWNERS:** The bot uses OWNERS files to suggest approvers and reviewers. The repository root [`OWNERS`](OWNERS) lists top-level approvers; deeper directories override or extend that list for those paths.
* **Commands:** Comment with Prow commands as needed (for example `/assign @user`, `/retest`). See [Prow command help](https://prow.k8s.io/command-help).

## Contact

* Slack: `#sig-gcp` on [Kubernetes Slack](https://kubernetes.slack.com/messages/sig-gcp)
* [SIG GCP mailing list](https://groups.google.com/forum/#!forum/kubernetes-sig-gcp)
* [SIG GCP community page](https://github.com/kubernetes/community/blob/master/sig-gcp/README.md)
