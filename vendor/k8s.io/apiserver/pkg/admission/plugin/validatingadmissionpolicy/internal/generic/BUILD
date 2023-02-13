load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generic",
    srcs = [
        "controller.go",
        "doc.go",
        "informer.go",
        "interface.go",
        "lister.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy/internal/generic",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy/internal/generic",
    visibility = ["//vendor/k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy:__subpackages__"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
