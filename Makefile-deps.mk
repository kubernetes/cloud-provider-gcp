# Copyright 2026 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This file contains targets for installing external tools and dependencies.

GOLANGCI_LINT_VERSION ?= v1.64.5
GOLANGCI_LINT := $(LOCAL_BIN)/golangci-lint

.PHONY: golangci-lint
golangci-lint:
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		CURRENT_VER=$$($(GOLANGCI_LINT) --version 2>/dev/null | awk '{print $$4}'); \
		if echo "$$CURRENT_VER" | grep -q "$$(echo $(GOLANGCI_LINT_VERSION) | sed 's/^v//')"; then \
			exit 0; \
		fi; \
	fi; \
	echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
	mkdir -p $(LOCAL_BIN); \
	GOBIN=$(LOCAL_BIN) GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: install-test-cluster-deps
install-test-cluster-deps: ## Install kubetest2 and other dependencies.
	@echo "Installing kubetest2 and plugins..."
	@mkdir -p $(LOCAL_BIN)
	@GOBIN=$(LOCAL_BIN) go install sigs.k8s.io/kubetest2@latest
	@GOBIN=$(LOCAL_BIN) go install sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo@latest
	@TEMP_DIR=$$(mktemp -d); \
	trap 'rm -rf "$$TEMP_DIR"' EXIT; \
	git clone --depth 1 https://github.com/kubernetes/kops.git "$$TEMP_DIR"; \
	cd "$$TEMP_DIR/tests/e2e" && GOBIN=$(LOCAL_BIN) go install ./kubetest2-kops ./kubetest2-tester-kops
	@echo "Downloading latest green kOps binary..."
	@KOPS_BASE_URL=$$(curl -s https://storage.googleapis.com/k8s-staging-kops/kops/releases/markers/master/latest-ci-updown-green.txt); \
	mkdir -p $(LOCAL_BIN); \
	wget -qO $(LOCAL_BIN)/kops.tmp $${KOPS_BASE_URL}/linux/amd64/kops; \
	chmod +x $(LOCAL_BIN)/kops.tmp; \
	mv $(LOCAL_BIN)/kops.tmp $(LOCAL_BIN)/kops
