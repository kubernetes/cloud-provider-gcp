# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Commands

### Development and Testing
- `./openshift-hack/test-unit.sh` - Run unit tests (preferred for local development)
- `openshift-hack/verify-vendor.sh` - Verify vendor directory consistency
- `openshift-hack/update-vendor.sh` - Update vendor directory
- `tools/verify-govet.sh` - Run Go vet analysis

### Code Quality
- `tools/verify-gofmt.sh` - Check Go formatting
- `tools/verify-golint.sh` - Run Go linting
- `tools/verify-govet.sh` - Run Go vet analysis
- `openshift-hack/verify-vendor.sh` - Verify vendor directory consistency
- `openshift-hack/update-vendor.sh` - Update vendor directory

### Dependency Management
- `go get github.com/new/dependency && ./openshift-hack/update-vendor.sh` - Add new dependency
- `go get -u github.com/existing/dependency && ./openshift-hack/update-vendor.sh` - Update existing dependency
- `openshift-hack/update-vendor.sh` - Update vendor dependencies after `go mod` changes

## Project Architecture

This repository implements the Google Cloud Platform (GCP) cloud provider for Kubernetes, containing:

### Core Components
- **Cloud Controller Manager** (`cmd/cloud-controller-manager/`) - Main entry point that initializes the GCP cloud provider and controllers
- **GCE Provider** (`providers/gce/`) - Core GCP cloud provider implementation with compute, storage, and networking integration
- **Node IPAM Controller** (`pkg/controller/nodeipam/`) - Manages IP address allocation for cluster nodes
- **GKE Network Param Set Controller** (`pkg/controller/gkenetworkparamset/`) - Handles GKE-specific network parameter configurations

## Development Workflow (OpenShift Fork)

### Git Remotes
- `kubernetes` - Kubernetes upstream remote
- `origin` - Personal fork
- `upstream` - OpenShift fork
- PRs typically flow: `origin` → OpenShift `upstream`

### Before Making Changes
1. **Propose a plan** and get user approval before implementing
2. **Ensure clean git state** - nothing staged (`git status` should be clean)
3. **Research existing implementations** in the codebase to justify changes
4. **Find similar patterns** in the codebase to follow established conventions

### Development Steps
1. Make code changes following existing patterns
2. Run `./openshift-hack/test-unit.sh` for fast local testing
3. Run `./tools/verify-govet.sh` to check code compiles
4. Run `./openshift-hack/verify-vendor.sh` to check vendoinrg
5. For integration/e2e testing, ask the user to run tests (requires Kubernetes/OpenShift cluster)

## Important Notes

- **Local Development**: Use `./openshift-hack/test-unit.sh` and `go build` commands rather than Bazel
- **Build/Release**: Don't run build or release targets locally - handled by CI/CD pipelines
- **E2E Testing**: Ask user to run e2e tests as they require live cluster access
- **Authentication**: Most functionality requires valid GCP credentials and project access
- **Vendoring**: Never revendor unless explicitly requested by the user
- **OpenShift Context**: This is a fork for OpenShift, so consider OpenShift-specific requirements and patterns, be aware of the different remotes

