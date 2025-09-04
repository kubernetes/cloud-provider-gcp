load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_priorityclass.go",
        "fake_scheduling_client.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/scheduling/v1/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/scheduling/v1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/scheduling/v1:scheduling",
        "//vendor/k8s.io/client-go/applyconfigurations/scheduling/v1:scheduling",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/typed/scheduling/v1:scheduling",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
