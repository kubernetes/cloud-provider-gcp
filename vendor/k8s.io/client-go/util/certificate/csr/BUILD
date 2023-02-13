load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "csr",
    srcs = ["csr.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/util/certificate/csr",
    importpath = "k8s.io/client-go/util/certificate/csr",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/certificates/v1:certificates",
        "//vendor/k8s.io/api/certificates/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/fields",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/watch",
        "//vendor/k8s.io/client-go/util/cert",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/pointer",
    ],
)
