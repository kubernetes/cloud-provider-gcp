load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1alpha2",
    srcs = [
        "leasecandidate.go",
        "leasecandidatespec.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/applyconfigurations/coordination/v1alpha2",
    importpath = "k8s.io/client-go/applyconfigurations/coordination/v1alpha2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/coordination/v1:coordination",
        "//vendor/k8s.io/api/coordination/v1alpha2",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/managedfields",
        "//vendor/k8s.io/client-go/applyconfigurations/internal",
        "//vendor/k8s.io/client-go/applyconfigurations/meta/v1:meta",
    ],
)
