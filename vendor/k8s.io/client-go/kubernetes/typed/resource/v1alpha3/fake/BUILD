load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_deviceclass.go",
        "fake_podschedulingcontext.go",
        "fake_resource_client.go",
        "fake_resourceclaim.go",
        "fake_resourceclaimtemplate.go",
        "fake_resourceslice.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/resource/v1alpha3/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/resource/v1alpha3/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/resource/v1alpha3",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/applyconfigurations/resource/v1alpha3",
        "//vendor/k8s.io/client-go/kubernetes/typed/resource/v1alpha3",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
