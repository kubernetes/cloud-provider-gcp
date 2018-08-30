package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "checks.go",
        "checks_unix.go",
        "checks_windows.go",
        "utils.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubeadm/app/preflight",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/preflight",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/constants:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/images:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/registry/core/service/ipallocator:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/initsystem:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipvs:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/version:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/version:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e_node/system:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
        "@com_github_blang_semver//:go_default_library",
        "@com_github_puerkitobio_purell//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "checks_test.go",
        "utils_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
        "//vendor/k8s.io/utils/exec/testing:go_default_library",
        "@com_github_renstrom_dedent//:go_default_library",
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
