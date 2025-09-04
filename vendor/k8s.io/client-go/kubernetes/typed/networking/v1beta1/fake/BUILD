load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_ingress.go",
        "fake_ingressclass.go",
        "fake_ipaddress.go",
        "fake_networking_client.go",
        "fake_servicecidr.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/networking/v1beta1/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/networking/v1beta1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/networking/v1beta1",
        "//vendor/k8s.io/client-go/applyconfigurations/networking/v1beta1",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/typed/networking/v1beta1",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
