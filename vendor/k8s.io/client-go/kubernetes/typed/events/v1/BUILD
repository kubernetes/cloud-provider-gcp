load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "events",
    srcs = [
        "doc.go",
        "event.go",
        "events_client.go",
        "generated_expansion.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/events/v1",
    importpath = "k8s.io/client-go/kubernetes/typed/events/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/events/v1:events",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/applyconfigurations/events/v1:events",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/scheme",
        "//vendor/k8s.io/client-go/rest",
    ],
)
