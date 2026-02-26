load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "providerconfig",
    srcs = [
        "doc.go",
        "groupversion_info.go",
        "register.go",
        "types.go",
        "zz_generated.deepcopy.go",
        "zz_generated.openapi.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1",
    importpath = "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
        "//vendor/sigs.k8s.io/controller-runtime/pkg/scheme",
    ],
)
