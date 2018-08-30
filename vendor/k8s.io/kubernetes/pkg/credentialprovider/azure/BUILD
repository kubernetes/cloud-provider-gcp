package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "azure_acr_helper.go",
        "azure_credentials.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/credentialprovider/azure",
    importpath = "k8s.io/kubernetes/pkg/credentialprovider/azure",
    deps = [
        "//vendor/github.com/ghodss/yaml:go_default_library",
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider/providers/azure/auth:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/credentialprovider:go_default_library",
        "@com_github_azure_azure_sdk_for_go//services/containerregistry/mgmt/2017-10-01/containerregistry:go_default_library",
        "@com_github_azure_go_autorest//autorest:go_default_library",
        "@com_github_azure_go_autorest//autorest/adal:go_default_library",
        "@com_github_azure_go_autorest//autorest/azure:go_default_library",
        "@com_github_dgrijalva_jwt_go//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["azure_credentials_test.go"],
    embed = [":go_default_library"],
    deps = [
        "@com_github_azure_azure_sdk_for_go//services/containerregistry/mgmt/2017-10-01/containerregistry:go_default_library",
        "@com_github_azure_go_autorest//autorest/to:go_default_library",
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
