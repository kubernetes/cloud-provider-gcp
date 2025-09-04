load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_coordination_client.go",
        "fake_leasecandidate.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/coordination/v1alpha2/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/coordination/v1alpha2/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/coordination/v1alpha2",
        "//vendor/k8s.io/client-go/applyconfigurations/coordination/v1alpha2",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/typed/coordination/v1alpha2",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
