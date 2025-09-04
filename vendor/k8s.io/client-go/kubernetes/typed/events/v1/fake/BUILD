load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_event.go",
        "fake_events_client.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/events/v1/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/events/v1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/events/v1:events",
        "//vendor/k8s.io/client-go/applyconfigurations/events/v1:events",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/typed/events/v1:events",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
