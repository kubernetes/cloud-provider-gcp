load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_batch_client.go",
        "fake_cronjob.go",
        "fake_job.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/batch/v1/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/batch/v1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/batch/v1:batch",
        "//vendor/k8s.io/client-go/applyconfigurations/batch/v1:batch",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/typed/batch/v1:batch",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
