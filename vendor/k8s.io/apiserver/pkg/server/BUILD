load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "server",
    srcs = [
        "config.go",
        "config_selfclient.go",
        "deleted_kinds.go",
        "deprecated_insecure_serving.go",
        "doc.go",
        "genericapiserver.go",
        "handler.go",
        "healthz.go",
        "hooks.go",
        "lifecycle_signals.go",
        "plugins.go",
        "secure_serving.go",
        "signal.go",
        "signal_posix.go",
        "signal_windows.go",
        "storage_readiness_hook.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server",
    importpath = "k8s.io/apiserver/pkg/server",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/coreos/go-systemd/v22/daemon",
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/github.com/google/uuid",
        "//vendor/golang.org/x/crypto/cryptobyte",
        "//vendor/golang.org/x/net/http2",
        "//vendor/golang.org/x/time/rate",
        "//vendor/gopkg.in/evanphx/json-patch.v4:json-patch_v4",
        "//vendor/k8s.io/api/apidiscovery/v2:apidiscovery",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/managedfields",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apimachinery/pkg/util/waitgroup",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/policy/validating",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/mutating",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/validating",
        "//vendor/k8s.io/apiserver/pkg/apis/apiserver/install",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticatorfactory",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/union",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/apiserver/pkg/endpoints",
        "//vendor/k8s.io/apiserver/pkg/endpoints/discovery",
        "//vendor/k8s.io/apiserver/pkg/endpoints/discovery/aggregated",
        "//vendor/k8s.io/apiserver/pkg/endpoints/filterlatency",
        "//vendor/k8s.io/apiserver/pkg/endpoints/filters",
        "//vendor/k8s.io/apiserver/pkg/endpoints/handlers/responsewriters",
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/apiserver/pkg/endpoints/openapi",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/registry/generic",
        "//vendor/k8s.io/apiserver/pkg/registry/rest",
        "//vendor/k8s.io/apiserver/pkg/server/dynamiccertificates",
        "//vendor/k8s.io/apiserver/pkg/server/egressselector",
        "//vendor/k8s.io/apiserver/pkg/server/filters",
        "//vendor/k8s.io/apiserver/pkg/server/healthz",
        "//vendor/k8s.io/apiserver/pkg/server/mux",
        "//vendor/k8s.io/apiserver/pkg/server/routes",
        "//vendor/k8s.io/apiserver/pkg/server/routine",
        "//vendor/k8s.io/apiserver/pkg/server/storage",
        "//vendor/k8s.io/apiserver/pkg/storage/value",
        "//vendor/k8s.io/apiserver/pkg/storageversion",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/request",
        "//vendor/k8s.io/apiserver/pkg/util/version",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/component-base/cli/flag",
        "//vendor/k8s.io/component-base/featuregate",
        "//vendor/k8s.io/component-base/logs",
        "//vendor/k8s.io/component-base/metrics/features",
        "//vendor/k8s.io/component-base/metrics/prometheus/slis",
        "//vendor/k8s.io/component-base/tracing",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kube-openapi/pkg/builder3",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/handler",
        "//vendor/k8s.io/kube-openapi/pkg/handler3",
        "//vendor/k8s.io/kube-openapi/pkg/spec3",
        "//vendor/k8s.io/kube-openapi/pkg/util",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
        "//vendor/k8s.io/utils/clock",
        "//vendor/k8s.io/utils/net",
    ],
)
