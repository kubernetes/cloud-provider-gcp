load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_certificates_client.go",
        "fake_clustertrustbundle.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/certificates/v1alpha1/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/certificates/v1alpha1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/certificates/v1alpha1",
        "//vendor/k8s.io/client-go/applyconfigurations/certificates/v1alpha1",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/typed/certificates/v1alpha1",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
