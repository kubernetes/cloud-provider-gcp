package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "daemonsets.go",
        "deployments.go",
        "job.go",
        "replicasets.go",
        "statefulset.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/e2e/upgrades/apps",
    importpath = "k8s.io/kubernetes/test/e2e/upgrades/apps",
    deps = [
        "//vendor/k8s.io/api/apps/v1:go_default_library",
        "//vendor/k8s.io/api/batch/v1:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/api/extensions/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller/deployment/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/version:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e/framework:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e/upgrades:go_default_library",
        "//vendor/k8s.io/kubernetes/test/utils/image:go_default_library",
        "@com_github_onsi_ginkgo//:go_default_library",
        "@com_github_onsi_gomega//:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
