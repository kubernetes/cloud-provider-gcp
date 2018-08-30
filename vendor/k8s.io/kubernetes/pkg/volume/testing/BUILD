package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "mock_volume.go",
        "testing.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/volume/testing",
    importpath = "k8s.io/kubernetes/pkg/volume/testing",
    deps = [
        "//vendor/k8s.io/api/authentication/v1:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/client-go/util/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/io:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/mount:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/strings:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume/util/recyclerclient:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume/util/volumepathhandler:go_default_library",
        "@com_github_stretchr_testify//mock:go_default_library",
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
