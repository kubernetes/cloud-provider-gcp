load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "scale",
    srcs = [
        "client.go",
        "doc.go",
        "interfaces.go",
        "util.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/scale",
    importpath = "k8s.io/client-go/scale",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/autoscaling/v1:autoscaling",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/dynamic",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/scale/scheme",
        "//vendor/k8s.io/client-go/scale/scheme/appsint",
        "//vendor/k8s.io/client-go/scale/scheme/appsv1beta1",
        "//vendor/k8s.io/client-go/scale/scheme/appsv1beta2",
        "//vendor/k8s.io/client-go/scale/scheme/autoscalingv1",
        "//vendor/k8s.io/client-go/scale/scheme/extensionsint",
        "//vendor/k8s.io/client-go/scale/scheme/extensionsv1beta1",
    ],
)
