# Copyright 2018  Google & GKE.
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

PROTOCOL ?= gs:
REGISTRY_PATH ?= kubernetes-release/release/v1.12.1
REGISTRY ?= ${PROTOCOL}//${REGISTRY_PATH}

build:
	mkdir -p build
	mkdir -p build/server
	mkdir -p build/src
	mkdir -p build/manifests

_output:
	mkdir -p _output
	mkdir -p _output/server/kubernetes

vendor:
	mkdir -p vendor

build/%.tar.gz.sha512: build
	# echo -n does not work below because the problem is the newline in the sha512 file not the echo.
	gsutil cp ${REGISTRY}/$*.tar.gz.sha512 build; \
	mv $@ $@.tmp; \
	echo " build/$*.tar.gz" >> $@.tmp; \
	tr -d '\n' < $@.tmp > $@; \
	rm $@.tmp

build/%.tar.gz: build/%.tar.gz.sha512
	@echo Fetching from ${REGISTRY}/$*.tar.gz
	gsutil cp ${REGISTRY}/$*.tar.gz build; \
	sha512sum -c $<
	echo "Success"

build/server: build build/kubernetes-server-linux-amd64.tar.gz
	# build/server is the location of the unpacked build/kubernetes-server-linux-amd64.tar.gz
	tar xvfz build/kubernetes-server-linux-amd64.tar.gz -C build/server

build/src: build build/server
	# build/server is the location of the unpacked build/build/server/kubernetes/kubernetes-src.tar.gz
	# Cannot have a rule for kubernetes-src.tar.gz as there is no shasum for it
	tar xvfz build/server/kubernetes/kubernetes-src.tar.gz -C build/src

_output/ccm-vendor.tar.gz: build/src
	mkdir -p _output/ccm/vendor/github.com/golang _output/ccm/vendor/github.com/spf13
	cp -rf build/src/vendor/github.com/golang/glog _output/ccm/vendor/github.com/golang
	cp -rf build/src/vendor/github.com/spf13/cobra _output/ccm/vendor/github.com/spf13
	cp -rf build/src/vendor/github.com/spf13/pflag _output/ccm/vendor/github.com/spf13
	mkdir -p _output/ccm/vendor/k8s.io/api/core _output/ccm/vendor/k8s.io/api/admission _output/ccm/vendor/k8s.io/api/admissionregistration _output/ccm/vendor/k8s.io/api/apps _output/ccm/vendor/k8s.io/api/authentication _output/ccm/vendor/k8s.io/api/authorization _output/ccm/vendor/k8s.io/api/autoscaling _output/ccm/vendor/k8s.io/api/batch _output/ccm/vendor/k8s.io/api/certificates _output/ccm/vendor/k8s.io/api/coordination _output/ccm/vendor/k8s.io/api/events _output/ccm/vendor/k8s.io/api/extensions _output/ccm/vendor/k8s.io/api/networking _output/ccm/vendor/k8s.io/api/policy _output/ccm/vendor/k8s.io/api/rbac _output/ccm/vendor/k8s.io/api/scheduling _output/ccm/vendor/k8s.io/api/settings _output/ccm/vendor/k8s.io/api/storage
	cp -rf build/src/staging/src/k8s.io/api/core/v1 _output/ccm/vendor/k8s.io/api/core
	cp -rf build/src/staging/src/k8s.io/api/admission/v1beta1 _output/ccm/vendor/k8s.io/api/admission
	cp -rf build/src/staging/src/k8s.io/api/admissionregistration/v1alpha1 _output/ccm/vendor/k8s.io/api/admissionregistration
	cp -rf build/src/staging/src/k8s.io/api/admissionregistration/v1beta1 _output/ccm/vendor/k8s.io/api/admissionregistration
	cp -rf build/src/staging/src/k8s.io/api/apps/v1 _output/ccm/vendor/k8s.io/api/apps
	cp -rf build/src/staging/src/k8s.io/api/apps/v1beta1 _output/ccm/vendor/k8s.io/api/apps
	cp -rf build/src/staging/src/k8s.io/api/apps/v1beta2 _output/ccm/vendor/k8s.io/api/apps
	cp -rf build/src/staging/src/k8s.io/api/authentication/v1 _output/ccm/vendor/k8s.io/api/authentication
	cp -rf build/src/staging/src/k8s.io/api/authentication/v1beta1 _output/ccm/vendor/k8s.io/api/authentication
	cp -rf build/src/staging/src/k8s.io/api/authorization/v1 _output/ccm/vendor/k8s.io/api/authorization
	cp -rf build/src/staging/src/k8s.io/api/authorization/v1beta1 _output/ccm/vendor/k8s.io/api/authorization
	cp -rf build/src/staging/src/k8s.io/api/autoscaling/v1 _output/ccm/vendor/k8s.io/api/autoscaling
	cp -rf build/src/staging/src/k8s.io/api/autoscaling/v2beta1 _output/ccm/vendor/k8s.io/api/autoscaling
	cp -rf build/src/staging/src/k8s.io/api/autoscaling/v2beta2 _output/ccm/vendor/k8s.io/api/autoscaling
	cp -rf build/src/staging/src/k8s.io/api/batch/v1 _output/ccm/vendor/k8s.io/api/batch
	cp -rf build/src/staging/src/k8s.io/api/batch/v1beta1 _output/ccm/vendor/k8s.io/api/batch
	cp -rf build/src/staging/src/k8s.io/api/batch/v2alpha1 _output/ccm/vendor/k8s.io/api/batch
	cp -rf build/src/staging/src/k8s.io/api/certificates/v1beta1 _output/ccm/vendor/k8s.io/api/certificates
	cp -rf build/src/staging/src/k8s.io/api/coordination/v1beta1 _output/ccm/vendor/k8s.io/api/coordination
	cp -rf build/src/staging/src/k8s.io/api/events/v1beta1 _output/ccm/vendor/k8s.io/api/events
	cp -rf build/src/staging/src/k8s.io/api/extensions/v1beta1 _output/ccm/vendor/k8s.io/api/extensions
	cp -rf build/src/staging/src/k8s.io/api/networking/v1 _output/ccm/vendor/k8s.io/api/networking
	cp -rf build/src/staging/src/k8s.io/api/policy/v1beta1 _output/ccm/vendor/k8s.io/api/policy
	cp -rf build/src/staging/src/k8s.io/api/rbac/v1 _output/ccm/vendor/k8s.io/api/rbac
	cp -rf build/src/staging/src/k8s.io/api/rbac/v1alpha1 _output/ccm/vendor/k8s.io/api/rbac
	cp -rf build/src/staging/src/k8s.io/api/rbac/v1beta1 _output/ccm/vendor/k8s.io/api/rbac
	cp -rf build/src/staging/src/k8s.io/api/scheduling/v1alpha1 _output/ccm/vendor/k8s.io/api/scheduling
	cp -rf build/src/staging/src/k8s.io/api/scheduling/v1beta1 _output/ccm/vendor/k8s.io/api/scheduling
	cp -rf build/src/staging/src/k8s.io/api/settings/v1alpha1 _output/ccm/vendor/k8s.io/api/settings
	cp -rf build/src/staging/src/k8s.io/api/storage/v1 _output/ccm/vendor/k8s.io/api/storage
	cp -rf build/src/staging/src/k8s.io/api/storage/v1alpha1 _output/ccm/vendor/k8s.io/api/storage
	cp -rf build/src/staging/src/k8s.io/api/storage/v1beta1 _output/ccm/vendor/k8s.io/api/storage
	mkdir -p _output/ccm/vendor/k8s.io/apiextensions-apiserver/pkg
	cp -rf build/src/staging/src/k8s.io/apiextensions-apiserver/pkg/features _output/ccm/vendor/k8s.io/apiextensions-apiserver/pkg
	mkdir -p _output/ccm/vendor/k8s.io/apimachinery/pkg/api
	mkdir -p _output/ccm/vendor/k8s.io/apimachinery/pkg/apis/meta
	mkdir -p _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	mkdir -p _output/ccm/vendor/k8s.io/apimachinery/third_party/forked/golang
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/api/equality _output/ccm/vendor/k8s.io/apimachinery/pkg/api
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/api/errors _output/ccm/vendor/k8s.io/apimachinery/pkg/api
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/api/meta _output/ccm/vendor/k8s.io/apimachinery/pkg/api
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/api/resource _output/ccm/vendor/k8s.io/apimachinery/pkg/api
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/api/validation _output/ccm/vendor/k8s.io/apimachinery/pkg/api
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1 _output/ccm/vendor/k8s.io/apimachinery/pkg/apis/meta
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1beta1 _output/ccm/vendor/k8s.io/apimachinery/pkg/apis/meta
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/apis/meta/internalversion _output/ccm/vendor/k8s.io/apimachinery/pkg/apis/meta
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/apis/config _output/ccm/vendor/k8s.io/apimachinery/pkg/apis
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/conversion _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/fields _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/labels _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/runtime _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/selection _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/types _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/cache _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/clock _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/diff _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/errors _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/framer _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/intstr _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/json _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/mergepatch _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/naming _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/net _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/rand _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/runtime _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/sets _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/strategicpatch _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/uuid _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/validation _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/wait _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/waitgroup _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/util/yaml _output/ccm/vendor/k8s.io/apimachinery/pkg/util
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/version _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/pkg/watch _output/ccm/vendor/k8s.io/apimachinery/pkg
	cp -rf build/src/staging/src/k8s.io/apimachinery/third_party/forked/golang/reflect _output/ccm/vendor/k8s.io/apimachinery/third_party/forked/golang
	cp -rf build/src/staging/src/k8s.io/apimachinery/third_party/forked/golang/json _output/ccm/vendor/k8s.io/apimachinery/third_party/forked/golang
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/admission
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/apis/apiserver
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/token _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/request
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/authorization
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/admission/plugin/namespace/ _output/ccm/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/audit
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints/handlers
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/registry
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/admission _output/ccm/vendor/k8s.io/apiserver/pkg
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/admission/initializer _output/ccm/vendor/k8s.io/apiserver/pkg/admission
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/admission/metrics _output/ccm/vendor/k8s.io/apiserver/pkg/admission
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/apis/apiserver _output/ccm/vendor/k8s.io/apiserver/pkg/apis
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/apis/apiserver/install _output/ccm/vendor/k8s.io/apiserver/pkg/apis/apiserver
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/apis/apiserver/v1alpha1 _output/ccm/vendor/k8s.io/apiserver/pkg/apis/apiserver
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/apis/audit _output/ccm/vendor/k8s.io/apiserver/pkg/apis
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/apis/audit/v1alpha1 _output/ccm/vendor/k8s.io/apiserver/pkg/apis/audit
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/apis/audit/v1beta1 _output/ccm/vendor/k8s.io/apiserver/pkg/apis/audit
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/apis/config _output/ccm/vendor/k8s.io/apiserver/pkg/apis
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/authenticator _output/ccm/vendor/k8s.io/apiserver/pkg/authentication
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/authenticatorfactory _output/ccm/vendor/k8s.io/apiserver/pkg/authentication
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/group _output/ccm/vendor/k8s.io/apiserver/pkg/authentication
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/request/anonymous _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/request
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/request/bearertoken _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/request
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/request/headerrequest _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/request
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/request/union _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/request
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/request/websocket _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/request
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/request/x509 _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/request
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/serviceaccount _output/ccm/vendor/k8s.io/apiserver/pkg/authentication
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/token/tokenfile _output/ccm/vendor/k8s.io/apiserver/pkg/authentication/token
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authentication/user _output/ccm/vendor/k8s.io/apiserver/pkg/authentication
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authorization/authorizer _output/ccm/vendor/k8s.io/apiserver/pkg/authorization
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authorization/authorizerfactory _output/ccm/vendor/k8s.io/apiserver/pkg/authorization
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authorization/path _output/ccm/vendor/k8s.io/apiserver/pkg/authorization
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/authorization/union _output/ccm/vendor/k8s.io/apiserver/pkg/authorization
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/admission/plugin/initialization _output/ccm/vendor/k8s.io/apiserver/pkg/admission/plugin
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle _output/ccm/vendor/k8s.io/apiserver/pkg/admission/plugin/namespace
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/admission/plugin/webhook/mutating _output/ccm/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/admission/plugin/webhook/validating _output/ccm/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/audit _output/ccm/vendor/k8s.io/apiserver/pkg
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/audit/policy _output/ccm/vendor/k8s.io/apiserver/pkg/audit
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/endpoints _output/ccm/vendor/k8s.io/apiserver/pkg
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/endpoints/discovery _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/endpoints/filters _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/endpoints/handlers/responsewriters _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints/handlers
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/endpoints/metrics _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/endpoints/openapi _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/endpoints/request _output/ccm/vendor/k8s.io/apiserver/pkg/endpoints
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/features _output/ccm/vendor/k8s.io/apiserver/pkg
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/registry/generic _output/ccm/vendor/k8s.io/apiserver/pkg/registry
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/registry/generic/registry _output/ccm/vendor/k8s.io/apiserver/pkg/registry/generic
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/registry/rest _output/ccm/vendor/k8s.io/apiserver/pkg/registry
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/server _output/ccm/vendor/k8s.io/apiserver/pkg
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/storage _output/ccm/vendor/k8s.io/apiserver/pkg
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/dryrun _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/feature _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/flag _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/flushwriter _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/logs _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/openapi _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/trace _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/webhook _output/ccm/vendor/k8s.io/apiserver/pkg/util
	cp -rf build/src/staging/src/k8s.io/apiserver/pkg/util/wsstream _output/ccm/vendor/k8s.io/apiserver/pkg/util
	mkdir -p _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/audit _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/authenticator/token _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/authorizer
	cp -rf build/src/staging/src/k8s.io/apiserver/plugin/pkg/audit/buffered _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/audit
	cp -rf build/src/staging/src/k8s.io/apiserver/plugin/pkg/audit/log _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/audit
	cp -rf build/src/staging/src/k8s.io/apiserver/plugin/pkg/audit/truncate _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/audit
	cp -rf build/src/staging/src/k8s.io/apiserver/plugin/pkg/audit/webhook _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/audit
	cp -rf build/src/staging/src/k8s.io/apiserver/plugin/pkg/authenticator/token/webhook _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/authenticator/token
	cp -rf build/src/staging/src/k8s.io/apiserver/plugin/pkg/authorizer/webhook _output/ccm/vendor/k8s.io/apiserver/plugin/pkg/authorizer
	mkdir -p _output/ccm/vendor/k8s.io/client-go/listers/admissionregistration _output/ccm/vendor/k8s.io/client-go/listers/apps _output/ccm/vendor/k8s.io/client-go/listers/autoscaling _output/ccm/vendor/k8s.io/client-go/listers/batch _output/ccm/vendor/k8s.io/client-go/listers/certificates _output/ccm/vendor/k8s.io/client-go/listers/coordination _output/ccm/vendor/k8s.io/client-go/listers/core _output/ccm/vendor/k8s.io/client-go/listers/events _output/ccm/vendor/k8s.io/client-go/listers/extensions _output/ccm/vendor/k8s.io/client-go/listers/networking _output/ccm/vendor/k8s.io/client-go/listers/policy _output/ccm/vendor/k8s.io/client-go/listers/rbac _output/ccm/vendor/k8s.io/client-go/listers/scheduling _output/ccm/vendor/k8s.io/client-go/listers/settings _output/ccm/vendor/k8s.io/client-go/listers/storage
	mkdir -p _output/ccm/vendor/k8s.io/client-go/pkg/apis
	mkdir -p _output/ccm/vendor/k8s.io/client-go/plugin/pkg/client/auth
	mkdir -p _output/ccm/vendor/k8s.io/client-go/tools
	mkdir -p _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/discovery _output/ccm/vendor/k8s.io/client-go
	cp -rf build/src/staging/src/k8s.io/client-go/informers _output/ccm/vendor/k8s.io/client-go
	cp -rf build/src/staging/src/k8s.io/client-go/kubernetes _output/ccm/vendor/k8s.io/client-go
	cp -rf build/src/staging/src/k8s.io/client-go/listers/admissionregistration/v1alpha1 _output/ccm/vendor/k8s.io/client-go/listers/admissionregistration
	cp -rf build/src/staging/src/k8s.io/client-go/listers/admissionregistration/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/admissionregistration
	cp -rf build/src/staging/src/k8s.io/client-go/listers/apps/v1 _output/ccm/vendor/k8s.io/client-go/listers/apps
	cp -rf build/src/staging/src/k8s.io/client-go/listers/apps/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/apps
	cp -rf build/src/staging/src/k8s.io/client-go/listers/apps/v1beta2 _output/ccm/vendor/k8s.io/client-go/listers/apps
	cp -rf build/src/staging/src/k8s.io/client-go/listers/autoscaling/v1 _output/ccm/vendor/k8s.io/client-go/listers/autoscaling
	cp -rf build/src/staging/src/k8s.io/client-go/listers/autoscaling/v2beta1 _output/ccm/vendor/k8s.io/client-go/listers/autoscaling
	cp -rf build/src/staging/src/k8s.io/client-go/listers/autoscaling/v2beta2 _output/ccm/vendor/k8s.io/client-go/listers/autoscaling
	cp -rf build/src/staging/src/k8s.io/client-go/listers/batch _output/ccm/vendor/k8s.io/client-go/listers
	cp -rf build/src/staging/src/k8s.io/client-go/listers/certificates/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/certificates
	cp -rf build/src/staging/src/k8s.io/client-go/listers/coordination/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/coordination
	cp -rf build/src/staging/src/k8s.io/client-go/listers/core/v1 _output/ccm/vendor/k8s.io/client-go/listers/core
	cp -rf build/src/staging/src/k8s.io/client-go/listers/events/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/events
	cp -rf build/src/staging/src/k8s.io/client-go/listers/extensions/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/extensions
	cp -rf build/src/staging/src/k8s.io/client-go/listers/networking/v1 _output/ccm/vendor/k8s.io/client-go/listers/networking
	cp -rf build/src/staging/src/k8s.io/client-go/listers/policy/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/policy
	cp -rf build/src/staging/src/k8s.io/client-go/listers/rbac/v1 _output/ccm/vendor/k8s.io/client-go/listers/rbac
	cp -rf build/src/staging/src/k8s.io/client-go/listers/rbac/v1alpha1 _output/ccm/vendor/k8s.io/client-go/listers/rbac
	cp -rf build/src/staging/src/k8s.io/client-go/listers/rbac/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/rbac
	cp -rf build/src/staging/src/k8s.io/client-go/listers/scheduling/v1alpha1 _output/ccm/vendor/k8s.io/client-go/listers/scheduling
	cp -rf build/src/staging/src/k8s.io/client-go/listers/scheduling/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/scheduling
	cp -rf build/src/staging/src/k8s.io/client-go/listers/settings/v1alpha1 _output/ccm/vendor/k8s.io/client-go/listers/settings
	cp -rf build/src/staging/src/k8s.io/client-go/listers/storage/v1 _output/ccm/vendor/k8s.io/client-go/listers/storage
	cp -rf build/src/staging/src/k8s.io/client-go/listers/storage/v1alpha1 _output/ccm/vendor/k8s.io/client-go/listers/storage
	cp -rf build/src/staging/src/k8s.io/client-go/listers/storage/v1beta1 _output/ccm/vendor/k8s.io/client-go/listers/storage
	cp -rf build/src/staging/src/k8s.io/client-go/pkg/apis/clientauthentication _output/ccm/vendor/k8s.io/client-go/pkg/apis
	cp -rf build/src/staging/src/k8s.io/client-go/pkg/version _output/ccm/vendor/k8s.io/client-go/pkg
	cp -rf build/src/staging/src/k8s.io/client-go/plugin/pkg/client/auth/exec _output/ccm/vendor/k8s.io/client-go/plugin/pkg/client/auth
	cp -rf build/src/staging/src/k8s.io/client-go/rest _output/ccm/vendor/k8s.io/client-go
	cp -rf build/src/staging/src/k8s.io/client-go/tools/auth _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/cache _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/clientcmd _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/leaderelection _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/metrics _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/pager _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/record _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/reference _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/tools/watch _output/ccm/vendor/k8s.io/client-go/tools
	cp -rf build/src/staging/src/k8s.io/client-go/transport _output/ccm/vendor/k8s.io/client-go
	cp -rf build/src/staging/src/k8s.io/client-go/util/buffer _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/util/cert _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/util/connrotation _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/util/flowcontrol _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/util/homedir _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/util/integer _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/util/retry _output/ccm/vendor/k8s.io/client-go/util
	cp -rf build/src/staging/src/k8s.io/client-go/util/workqueue _output/ccm/vendor/k8s.io/client-go/util
	mkdir -p _output/ccm/vendor/k8s.io/csi-api/pkg/apis/csi _output/ccm/vendor/k8s.io/csi-api/pkg/client/clientset
	cp -rf build/src/staging/src/k8s.io/csi-api/pkg/apis/csi/v1alpha1 _output/ccm/vendor/k8s.io/csi-api/pkg/apis/csi
	cp -rf build/src/staging/src/k8s.io/csi-api/pkg/client/clientset/versioned _output/ccm/vendor/k8s.io/csi-api/pkg/client/clientset
	mkdir -p _output/ccm/vendor/k8s.io/kube-openapi/pkg
	cp -rf build/src/vendor/k8s.io/kube-openapi/pkg/builder _output/ccm/vendor/k8s.io/kube-openapi/pkg
	cp -rf build/src/vendor/k8s.io/kube-openapi/pkg/common _output/ccm/vendor/k8s.io/kube-openapi/pkg
	cp -rf build/src/vendor/k8s.io/kube-openapi/pkg/handler _output/ccm/vendor/k8s.io/kube-openapi/pkg
	cp -rf build/src/vendor/k8s.io/kube-openapi/pkg/util _output/ccm/vendor/k8s.io/kube-openapi/pkg
	mkdir -p _output/ccm/vendor/k8s.io/kube-controller-manager/config
	cp -rf build/src/staging/src/k8s.io/kube-controller-manager/config/v1alpha1 _output/ccm/vendor/k8s.io/kube-controller-manager/config
	mkdir -p _output/ccm/vendor/k8s.io/kubernetes/cmd/controller-manager
	cp -rf build/src/cmd/controller-manager/app _output/ccm/vendor/k8s.io/kubernetes/cmd/controller-manager
	mkdir -p _output/ccm/vendor/k8s.io/kubernetes/pkg/api/v1
	cp -rf build/src/pkg/api/v1/node _output/ccm/vendor/k8s.io/kubernetes/pkg/api/v1
	cp -rf build/src/pkg/api/v1/pod _output/ccm/vendor/k8s.io/kubernetes/pkg/api/v1
	cp -rf build/src/pkg/api/v1/service _output/ccm/vendor/k8s.io/kubernetes/pkg/api/v1
	cp -rf build/src/pkg/api/service _output/ccm/vendor/k8s.io/kubernetes/pkg/api
	mkdir -p _output/ccm/vendor/k8s.io/kubernetes/pkg/apis _output/ccm/vendor/k8s.io/kubernetes/pkg/client _output/ccm/vendor/k8s.io/kubernetes/pkg/kubelet/util _output/ccm/vendor/k8s.io/kubernetes/pkg/scheduler _output/ccm/vendor/k8s.io/kubernetes/pkg/security _output/ccm/vendor/k8s.io/kubernetes/pkg/util/net _output/ccm/vendor/k8s.io/kubernetes/pkg/volume
	cp -rf build/src/pkg/apis/core _output/ccm/vendor/k8s.io/kubernetes/pkg/apis
	cp -rf build/src/pkg/apis/autoscaling _output/ccm/vendor/k8s.io/kubernetes/pkg/apis
	cp -rf build/src/pkg/apis/extensions _output/ccm/vendor/k8s.io/kubernetes/pkg/apis
	cp -rf build/src/pkg/apis/networking _output/ccm/vendor/k8s.io/kubernetes/pkg/apis
	cp -rf build/src/pkg/apis/policy _output/ccm/vendor/k8s.io/kubernetes/pkg/apis
	cp -rf build/src/pkg/apis/scheduling _output/ccm/vendor/k8s.io/kubernetes/pkg/apis
	cp -rf build/src/pkg/capabilities _output/ccm/vendor/k8s.io/kubernetes/pkg
	cp -rf build/src/pkg/client/leaderelectionconfig _output/ccm/vendor/k8s.io/kubernetes/pkg/client
	cp -rf build/src/pkg/fieldpath _output/ccm/vendor/k8s.io/kubernetes/pkg
	cp -rf build/src/pkg/kubelet/apis _output/ccm/vendor/k8s.io/kubernetes/pkg/kubelet
	cp -rf build/src/pkg/kubelet/types _output/ccm/vendor/k8s.io/kubernetes/pkg/kubelet
	cp -rf build/src/pkg/kubelet/util/format _output/ccm/vendor/k8s.io/kubernetes/pkg/kubelet/util
	cp -rf build/src/pkg/scheduler/algorithm _output/ccm/vendor/k8s.io/kubernetes/pkg/scheduler
	cp -rf build/src/pkg/scheduler/api _output/ccm/vendor/k8s.io/kubernetes/pkg/scheduler
	cp -rf build/src/pkg/scheduler/cache _output/ccm/vendor/k8s.io/kubernetes/pkg/scheduler
	cp -rf build/src/pkg/scheduler/util _output/ccm/vendor/k8s.io/kubernetes/pkg/scheduler
	cp -rf build/src/pkg/security/apparmor _output/ccm/vendor/k8s.io/kubernetes/pkg/security
	cp -rf build/src/pkg/serviceaccount _output/ccm/vendor/k8s.io/kubernetes/pkg
	cp -rf build/src/pkg/util/file _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/hash _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/io _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/metrics _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/mount _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/net/sets _output/ccm/vendor/k8s.io/kubernetes/pkg/util/net
	cp -rf build/src/pkg/util/node _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/nsenter _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/parsers _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/strings _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/taints _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/util/version _output/ccm/vendor/k8s.io/kubernetes/pkg/util
	cp -rf build/src/pkg/volume _output/ccm/vendor/k8s.io/kubernetes/pkg
	mkdir -p _output/ccm/pkg/api _output/ccm/pkg/client/metrics _output/ccm/pkg/master _output/ccm/pkg/util
	cp -rf build/src/pkg/api/legacyscheme _output/ccm/pkg/api
	cp -rf build/src/pkg/client/metrics/prometheus _output/ccm/pkg/client/metrics
	cp -rf build/src/pkg/controller _output/ccm/pkg
	cp -rf build/src/pkg/features _output/ccm/pkg
	cp -rf build/src/pkg/master/ports _output/ccm/pkg/master
	cp -rf build/src/pkg/util/configz _output/ccm/pkg/util
	cp -rf build/src/pkg/util/flag _output/ccm/pkg/util
	cp -rf build/src/pkg/version _output/ccm/pkg
	mkdir -p _output/ccm/vendor/k8s.io/utils
	cp -rf build/src/vendor/k8s.io/utils/exec _output/ccm/vendor/k8s.io/utils
	cp -rf build/src/vendor/k8s.io/utils/pointer _output/ccm/vendor/k8s.io/utils
	mkdir -p _output/ccm/vendor/bitbucket.org/ww
	cp -rf build/src/vendor/bitbucket.org/ww/goautoneg _output/ccm/vendor/bitbucket.org/ww
	mkdir -p _output/ccm/vendor/github.com/NYTimes
	cp -rf build/src/vendor/github.com/NYTimes/gziphandler _output/ccm/vendor/github.com/NYTimes
	mkdir -p _output/ccm/vendor/github.com/beorn7/perks
	cp -rf build/src/vendor/github.com/beorn7/perks/quantile _output/ccm/vendor/github.com/beorn7/perks
	mkdir -p _output/ccm/vendor/github.com/coreos/etcd/etcdserver/api/v3rpc _output/ccm/vendor/github.com/coreos/etcd/auth _output/ccm/vendor/github.com/coreos/etcd/mvcc _output/ccm/vendor/github.com/coreos/etcd/pkg _output/ccm/vendor/github.com/coreos/go-semver _output/ccm/vendor/github.com/coreos/go-systemd
	cp -rf build/src/vendor/github.com/coreos/etcd/auth/authpb _output/ccm/vendor/github.com/coreos/etcd/auth
	cp -rf build/src/vendor/github.com/coreos/etcd/client _output/ccm/vendor/github.com/coreos/etcd
	cp -rf build/src/vendor/github.com/coreos/etcd/clientv3 _output/ccm/vendor/github.com/coreos/etcd
	cp -rf build/src/vendor/github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes _output/ccm/vendor/github.com/coreos/etcd/etcdserver/api/v3rpc
	cp -rf build/src/vendor/github.com/coreos/etcd/etcdserver/etcdserverpb _output/ccm/vendor/github.com/coreos/etcd/etcdserver
	cp -rf build/src/vendor/github.com/coreos/etcd/mvcc/mvccpb _output/ccm/vendor/github.com/coreos/etcd/mvcc
	cp -rf build/src/vendor/github.com/coreos/etcd/pkg/pathutil _output/ccm/vendor/github.com/coreos/etcd/pkg
	cp -rf build/src/vendor/github.com/coreos/etcd/pkg/srv _output/ccm/vendor/github.com/coreos/etcd/pkg
	cp -rf build/src/vendor/github.com/coreos/etcd/pkg/tlsutil _output/ccm/vendor/github.com/coreos/etcd/pkg
	cp -rf build/src/vendor/github.com/coreos/etcd/pkg/transport _output/ccm/vendor/github.com/coreos/etcd/pkg
	cp -rf build/src/vendor/github.com/coreos/etcd/pkg/types _output/ccm/vendor/github.com/coreos/etcd/pkg
	cp -rf build/src/vendor/github.com/coreos/etcd/version _output/ccm/vendor/github.com/coreos/etcd
	cp -rf build/src/vendor/github.com/coreos/go-semver/semver _output/ccm/vendor/github.com/coreos/go-semver
	cp -rf build/src/vendor/github.com/coreos/go-systemd/daemon _output/ccm/vendor/github.com/coreos/go-systemd
	mkdir -p _output/ccm/vendor/github.com/davecgh/go-spew
	cp -rf build/src/vendor/github.com/davecgh/go-spew/spew _output/ccm/vendor/github.com/davecgh/go-spew
	mkdir -p _output/ccm/vendor/github.com/docker/distribution _output/ccm/vendor/github.com/docker/docker/pkg
	cp -rf build/src/vendor/github.com/docker/distribution/digestset _output/ccm/vendor/github.com/docker/distribution
	cp -rf build/src/vendor/github.com/docker/distribution/reference _output/ccm/vendor/github.com/docker/distribution
	cp -rf build/src/vendor/github.com/docker/docker/pkg/term _output/ccm/vendor/github.com/docker/docker/pkg
	mkdir -p _output/ccm/vendor/github.com/elazarl
	cp -rf build/src/vendor/github.com/elazarl/go-bindata-assetfs _output/ccm/vendor/github.com/elazarl
	mkdir -p _output/ccm/vendor/github.com/emicklei
	cp -rf build/src/vendor/github.com/emicklei/go-restful _output/ccm/vendor/github.com/emicklei
	cp -rf build/src/vendor/github.com/emicklei/go-restful-swagger12 _output/ccm/vendor/github.com/emicklei
	mkdir -p _output/ccm/vendor/github.com/evanphx
	cp -rf build/src/vendor/github.com/evanphx/json-patch _output/ccm/vendor/github.com/evanphx
	mkdir -p _output/ccm/vendor/github.com/ghodss
	cp -rf build/src/vendor/github.com/ghodss/yaml _output/ccm/vendor/github.com/ghodss
	mkdir -p _output/ccm/vendor/github.com/go-openapi
	cp -rf build/src/vendor/github.com/go-openapi/jsonpointer _output/ccm/vendor/github.com/go-openapi
	cp -rf build/src/vendor/github.com/go-openapi/jsonreference _output/ccm/vendor/github.com/go-openapi
	cp -rf build/src/vendor/github.com/go-openapi/spec _output/ccm/vendor/github.com/go-openapi
	cp -rf build/src/vendor/github.com/go-openapi/swag _output/ccm/vendor/github.com/go-openapi
	mkdir -p _output/ccm/vendor/github.com/gogo/protobuf
	cp -rf build/src/vendor/github.com/gogo/protobuf/proto _output/ccm/vendor/github.com/gogo/protobuf
	cp -rf build/src/vendor/github.com/gogo/protobuf/proto _output/ccm/vendor/github.com/gogo/protobuf
	cp -rf build/src/vendor/github.com/gogo/protobuf/sortkeys _output/ccm/vendor/github.com/gogo/protobuf
	mkdir -p _output/ccm/vendor/github.com/golang/groupcache
	cp -rf build/src/vendor/github.com/golang/groupcache/lru _output/ccm/vendor/github.com/golang/groupcache
	mkdir -p _output/ccm/vendor/github.com/golang/protobuf _output/ccm/vendor/github.com/golang/protobuf/protoc-gen-go
	cp -rf build/src/vendor/github.com/golang/protobuf/proto _output/ccm/vendor/github.com/golang/protobuf
	cp -rf build/src/vendor/github.com/golang/protobuf/protoc-gen-go/descriptor _output/ccm/vendor/github.com/golang/protobuf/protoc-gen-go
	cp -rf build/src/vendor/github.com/golang/protobuf/ptypes _output/ccm/vendor/github.com/golang/protobuf
	mkdir -p _output/ccm/vendor/github.com/google
	cp -rf build/src/vendor/github.com/google/btree _output/ccm/vendor/github.com/google
	cp -rf build/src/vendor/github.com/google/gofuzz _output/ccm/vendor/github.com/google
	mkdir -p _output/ccm/vendor/github.com/googleapis/gnostic
	cp -rf build/src/vendor/github.com/googleapis/gnostic/OpenAPIv2 _output/ccm/vendor/github.com/googleapis/gnostic
	cp -rf build/src/vendor/github.com/googleapis/gnostic/compiler _output/ccm/vendor/github.com/googleapis/gnostic
	cp -rf build/src/vendor/github.com/googleapis/gnostic/extensions _output/ccm/vendor/github.com/googleapis/gnostic
	mkdir -p _output/ccm/vendor/github.com/gregjones
	cp -rf build/src/vendor/github.com/gregjones/httpcache _output/ccm/vendor/github.com/gregjones
	mkdir -p _output/ccm/vendor/github.com/grpc-ecosystem
	cp -rf build/src/vendor/github.com/grpc-ecosystem/go-grpc-prometheus _output/ccm/vendor/github.com/grpc-ecosystem
	mkdir -p _output/ccm/vendor/github.com/hashicorp
	cp -rf build/src/vendor/github.com/hashicorp/golang-lru _output/ccm/vendor/github.com/hashicorp
	mkdir -p _output/ccm/vendor/github.com/imdario
	cp -rf build/src/vendor/github.com/imdario/mergo _output/ccm/vendor/github.com/imdario
	mkdir -p _output/ccm/vendor/github.com/json-iterator
	cp -rf build/src/vendor/github.com/json-iterator/go _output/ccm/vendor/github.com/json-iterator
	mkdir -p _output/ccm/vendor/github.com/mailru/easyjson
	cp -rf build/src/vendor/github.com/mailru/easyjson/buffer _output/ccm/vendor/github.com/mailru/easyjson
	cp -rf build/src/vendor/github.com/mailru/easyjson/jlexer _output/ccm/vendor/github.com/mailru/easyjson
	cp -rf build/src/vendor/github.com/mailru/easyjson/jwriter _output/ccm/vendor/github.com/mailru/easyjson
	mkdir -p _output/ccm/vendor/github.com/matttproud/golang_protobuf_extensions
	cp -rf build/src/vendor/github.com/matttproud/golang_protobuf_extensions/pbutil _output/ccm/vendor/github.com/matttproud/golang_protobuf_extensions
	mkdir -p _output/ccm/vendor/github.com/modern-go
	cp -rf build/src/vendor/github.com/modern-go/concurrent _output/ccm/vendor/github.com/modern-go
	cp -rf build/src/vendor/github.com/modern-go/reflect2 _output/ccm/vendor/github.com/modern-go
	mkdir -p _output/ccm/vendor/github.com/opencontainers
	cp -rf build/src/vendor/github.com/opencontainers/go-digest _output/ccm/vendor/github.com/opencontainers
	mkdir -p _output/ccm/vendor/github.com/pborman
	cp -rf build/src/vendor/github.com/pborman/uuid _output/ccm/vendor/github.com/pborman
	mkdir -p _output/ccm/vendor/github.com/peterbourgon
	cp -rf build/src/vendor/github.com/peterbourgon/diskv _output/ccm/vendor/github.com/peterbourgon
	mkdir -p _output/ccm/vendor/github.com/prometheus/client_model _output/ccm/vendor/github.com/prometheus/common/expfmt _output/ccm/vendor/github.com/prometheus/common/model _output/ccm/vendor/github.com/prometheus/common/internal/bitbucket.org/ww
	cp -rf build/src/vendor/github.com/prometheus/client_model/go _output/ccm/vendor/github.com/prometheus/client_model
	cp -rf build/src/vendor/github.com/prometheus/common/expfmt _output/ccm/vendor/github.com/prometheus/common
	cp -rf build/src/vendor/github.com/prometheus/common/internal/bitbucket.org/ww/goautoneg _output/ccm/vendor/github.com/prometheus/common/internal/bitbucket.org/ww
	cp -rf build/src/vendor/github.com/prometheus/common/model _output/ccm/vendor/github.com/prometheus/common
	cp -rf build/src/vendor/github.com/prometheus/procfs _output/ccm/vendor/github.com/prometheus/
	mkdir -p _output/ccm/vendor/github.com/PuerkitoBio
	cp -rf build/src/vendor/github.com/PuerkitoBio/purell _output/ccm/vendor/github.com/PuerkitoBio
	cp -rf build/src/vendor/github.com/PuerkitoBio/urlesc _output/ccm/vendor/github.com/PuerkitoBio
	mkdir -p _output/ccm/vendor/github.com/ugorji/go
	cp -rf build/src/vendor/github.com/ugorji/go/codec _output/ccm/vendor/github.com/ugorji/go
	mkdir -p _output/ccm/vendor/golang.org/x/crypto _output/ccm/vendor/golang.org/x/net/internal _output/ccm/vendor/golang.org/x/net/lex
	cp -rf build/src/vendor/golang.org/x/crypto/ed25519 _output/ccm/vendor/golang.org/x/crypto
	cp -rf build/src/vendor/golang.org/x/net/context _output/ccm/vendor/golang.org/x/net
	cp -rf build/src/vendor/golang.org/x/net/idna _output/ccm/vendor/golang.org/x/net
	cp -rf build/src/vendor/golang.org/x/net/internal/timeseries _output/ccm/vendor/golang.org/x/net/internal
	cp -rf build/src/vendor/golang.org/x/net/lex/httplex _output/ccm/vendor/golang.org/x/net/lex
	cp -rf build/src/vendor/golang.org/x/net/trace _output/ccm/vendor/golang.org/x/net
	mkdir -p _output/ccm/vendor/golang.org/x/sys
	cp -rf build/src/vendor/golang.org/x/sys/unix _output/ccm/vendor/golang.org/x/sys
	mkdir -p _output/ccm/vendor/golang.org/x/text/secure _output/ccm/vendor/golang.org/x/text/unicode
	cp -rf build/src/vendor/golang.org/x/text/cases _output/ccm/vendor/golang.org/x/text
	cp -rf build/src/vendor/golang.org/x/text/internal _output/ccm/vendor/golang.org/x/text
	cp -rf build/src/vendor/golang.org/x/text/language _output/ccm/vendor/golang.org/x/text
	cp -rf build/src/vendor/golang.org/x/text/runes _output/ccm/vendor/golang.org/x/text
	cp -rf build/src/vendor/golang.org/x/text/secure/bidirule _output/ccm/vendor/golang.org/x/text/secure
	cp -rf build/src/vendor/golang.org/x/text/secure/precis _output/ccm/vendor/golang.org/x/text/secure
	cp -rf build/src/vendor/golang.org/x/text/transform _output/ccm/vendor/golang.org/x/text
	cp -rf build/src/vendor/golang.org/x/text/unicode/bidi _output/ccm/vendor/golang.org/x/text/unicode
	cp -rf build/src/vendor/golang.org/x/text/unicode/norm _output/ccm/vendor/golang.org/x/text/unicode
	cp -rf build/src/vendor/golang.org/x/text/width _output/ccm/vendor/golang.org/x/text
	mkdir -p _output/ccm/vendor/golang.org/x/time
	cp -rf build/src/vendor/golang.org/x/time/rate _output/ccm/vendor/golang.org/x/time
	mkdir -p _output/ccm/vendor/google.golang.org/api _output/ccm/vendor/google.golang.org/genproto/googleapis/api _output/ccm/vendor/google.golang.org/genproto/googleapis/rpc
	cp -rf build/src/vendor/google.golang.org/api/gensupport _output/ccm/vendor/google.golang.org/api
	cp -rf build/src/vendor/google.golang.org/genproto/googleapis/api/annotations _output/ccm/vendor/google.golang.org/genproto/googleapis/api
	cp -rf build/src/vendor/google.golang.org/genproto/googleapis/rpc/status _output/ccm/vendor/google.golang.org/genproto/googleapis/rpc
	cp -rf build/src/vendor/google.golang.org/grpc _output/ccm/vendor/google.golang.org
	mkdir -p _output/ccm/vendor/github.com/prometheus/client_golang
	cp -rf build/src/vendor/github.com/prometheus/client_golang/prometheus _output/ccm/vendor/github.com/prometheus/client_golang
	mkdir -p _output/ccm/vendor/golang.org/x/crypto/ssh
	cp -rf build/src/vendor/golang.org/x/crypto/ssh/terminal _output/ccm/vendor/golang.org/x/crypto/ssh
	mkdir -p _output/ccm/vendor/golang.org/x/net
	cp -rf build/src/vendor/golang.org/x/net/http2 _output/ccm/vendor/golang.org/x/net
	mkdir -p _output/ccm/vendor/golang.org/x/net
	cp -rf build/src/vendor/golang.org/x/net/websocket _output/ccm/vendor/golang.org/x/net
	mkdir -p _output/ccm/vendor/golang.org/x
	cp -rf build/src/vendor/golang.org/x/oauth2 _output/ccm/vendor/golang.org/x
	mkdir -p _output/ccm/vendor/cloud.google.com/go/compute
	cp -rf build/src/vendor/cloud.google.com/go/compute/metadata _output/ccm/vendor/cloud.google.com/go/compute
	cp -rf build/src/vendor/cloud.google.com/go/internal _output/ccm/vendor/cloud.google.com/go
	mkdir -p _output/ccm/vendor/google.golang.org
	cp -rf build/src/vendor/google.golang.org/grpc _output/ccm/vendor/google.golang.org
	mkdir -p _output/ccm/vendor/google.golang.org/api/compute _output/ccm/vendor/google.golang.org/api/container _output/ccm/vendor/google.golang.org/api _output/ccm/vendor/google.golang.org/api/tpu/v1
	cp -rf build/src/vendor/google.golang.org/api/compute/v0.alpha _output/ccm/vendor/google.golang.org/api/compute
	cp -rf build/src/vendor/google.golang.org/api/compute/v0.beta _output/ccm/vendor/google.golang.org/api/compute
	cp -rf build/src/vendor/google.golang.org/api/compute/v1 _output/ccm/vendor/google.golang.org/api/compute
	cp -rf build/src/vendor/google.golang.org/api/container/v1 _output/ccm/vendor/google.golang.org/api/container
	cp -rf build/src/vendor/google.golang.org/api/googleapi _output/ccm/vendor/google.golang.org/api
	cp -rf build/src/vendor/google.golang.org/api/tpu/v1 _output/ccm/vendor/google.golang.org/api/tpu
	mkdir -p _output/ccm/vendor/gopkg.in/natefinch _output/ccm/vendor/gopkg.in/square
	cp -rf build/src/vendor/gopkg.in/gcfg.v1 _output/ccm/vendor/gopkg.in
	cp -rf build/src/vendor/gopkg.in/inf.v0 _output/ccm/vendor/gopkg.in
	cp -rf build/src/vendor/gopkg.in/natefinch/lumberjack.v2 _output/ccm/vendor/gopkg.in/natefinch
	cp -rf build/src/vendor/gopkg.in/square/go-jose.v2 _output/ccm/vendor/gopkg.in/square
	cp -rf build/src/vendor/gopkg.in/yaml.v2 _output/ccm/vendor/gopkg.in
	cp -rf build/src/vendor/gopkg.in/warnings.v0 _output/ccm/vendor/gopkg.in
	# Need to deal with the move from pkg/cloudprovider to staging/src/k8s.io/cloudprovider
	# Currently we expressly remove other cloud providers from the import list.
	mkdir -p _output/ccm/pkg/cloudprovider/providers
	cp -rf build/src/pkg/cloudprovider/providers/gce _output/ccm/pkg/cloudprovider/providers
	cp build/src/pkg/cloudprovider/providers/*.go _output/ccm/pkg/cloudprovider/providers; \
	sed -i '/\t_ "k8s.io\/kubernetes\/pkg\/cloudprovider\/providers\/aws"/d' _output/ccm/pkg/cloudprovider/providers/providers.go; \
	sed -i '/\t_ "k8s.io\/kubernetes\/pkg\/cloudprovider\/providers\/azure"/d' _output/ccm/pkg/cloudprovider/providers/providers.go; \
	sed -i '/\t_ "k8s.io\/kubernetes\/pkg\/cloudprovider\/providers\/cloudstack"/d' _output/ccm/pkg/cloudprovider/providers/providers.go; \
	sed -i '/\t_ "k8s.io\/kubernetes\/pkg\/cloudprovider\/providers\/openstack"/d' _output/ccm/pkg/cloudprovider/providers/providers.go; \
	sed -i '/\t_ "k8s.io\/kubernetes\/pkg\/cloudprovider\/providers\/ovirt"/d' _output/ccm/pkg/cloudprovider/providers/providers.go; \
	sed -i '/\t_ "k8s.io\/kubernetes\/pkg\/cloudprovider\/providers\/photon"/d' _output/ccm/pkg/cloudprovider/providers/providers.go; \
	sed -i '/\t_ "k8s.io\/kubernetes\/pkg\/cloudprovider\/providers\/vsphere"/d' _output/ccm/pkg/cloudprovider/providers/providers.go
	cp build/src/pkg/cloudprovider/*.go _output/ccm/pkg/cloudprovider
	#mkdir -p _output/vendor/k8s.io
	#cp -rf vendor/k8s.io/cloud-provider _output/ccm/vendor/k8s.io
	cd _output/ccm; \
	tar cvfz ../ccm-vendor.tar.gz vendor pkg

controller-manager: build/src artifacts/ccm-build.dockerfile _output/ccm-vendor.tar.gz
	@echo Building the Cloud Controller Manager
	docker build -t wrf:test -f artifacts/ccm-build.dockerfile .
	docker run -ti --name ccm-build wrf:test
	docker cp ccm-build:/go/src/k8s.io/kubernetes/controller-manager .

_output/kubernetes-server-linux-amd64.tar.gz: _output build/kubernetes-server-linux-amd64.tar.gz controller-manager
	tar xvfz build/kubernetes-server-linux-amd64.tar.gz -C build/server
	cp build/server/kubernetes/LICENSES _output/server/kubernetes
	cp -rf build/server/kubernetes/server _output/server/kubernetes
	cp -rf build/server/kubernetes/addons _output/server/kubernetes
	# TODO: WRF Determine if we should be propogating src tgz
	# Add GCP specific binaries (CCM) to server tgz
	mv controller-manager _output/server/kubernetes
	# TODO: WRF Pull is CSI and package that with both server and kubelet.
	cd _output/server; \
	tar cvfz ../kubernetes-server-linux-amd64.tar.gz kubernetes

_output/kubernetes-manifests.tar.gz: build _output build/kubernetes-manifests.tar.gz cluster/gce/manifests/cloud-controller-manager.manifest
	tar xvfz build/kubernetes-manifests.tar.gz -C build/manifests
	mkdir build/manifests/kubernetes/gci-trusty/cloud-controller-manager
	cp cluster/gce/manifests/cloud-controller-manager.manifest build/manifests/kubernetes/gci-trusty/cloud-controller-manager
	cp cluster/addons/cloud-controller-manager/*.yaml build/manifests/kubernetes/gci-trusty/cloud-controller-manager
	cp cluster/gce/gci/configure-helper.sh build/manifests/kubernetes/gci-trusty/gci-configure-helper.sh
	cd build/manifests; \
	tar cvfz ../../_output/kubernetes-manifests.tar.gz kubernetes

_output/%.tar.gz.sha512: _output/%.tar.gz
	# Do I need to remove the binary name from the sha file?
	sha512sum $< > $@

build-artifacts: _output/kubernetes-server-linux-amd64.tar.gz _output/kubernetes-manifests.tar.gz build/kubernetes-node-linux-amd64.tar.gz build/kubernetes.tar.gz

deploy-artifacts: _output/kubernetes-server-linux-amd64.tar.gz _output/kubernetes-server-linux-amd64.tar.gz.sha512 _output/kubernetes-manifests.tar.gz _output/kubernetes-manifests.tar.gz.sha512

clean:
	rm -rf build _output vendor/github.com/docker/docker controller-manager
	docker rmi -f wrf:test | true
	docker rm ccm-build | true

.PHONY: build-artifacts deploy-artifacts clean ccm-build-image
