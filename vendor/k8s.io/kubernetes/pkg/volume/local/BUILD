load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "local.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/volume/local",
    importpath = "k8s.io/kubernetes/pkg/volume/local",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/events:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/keymutex:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/mount:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/strings:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume/validation:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "local_linux_test.go",
        "local_test.go",
    ],
    embed = [":go_default_library"],
    deps = select({
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/k8s.io/api/core/v1:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/client-go/util/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/volume/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/api/core/v1:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/client-go/util/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/volume/testing:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/k8s.io/api/core/v1:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/client-go/util/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/volume/testing:go_default_library",
        ],
        "//conditions:default": [],
    }),
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
    visibility = ["//visibility:public"],
)
