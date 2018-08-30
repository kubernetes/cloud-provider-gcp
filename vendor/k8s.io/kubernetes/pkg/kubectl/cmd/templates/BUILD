load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "command_groups.go",
        "markdown.go",
        "normalizers.go",
        "templater.go",
        "templates.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubectl/cmd/templates",
    importpath = "k8s.io/kubernetes/pkg/kubectl/cmd/templates",
    visibility = [
        "//build/visible_to:pkg_kubectl_cmd_templates_CONSUMERS",
    ],
    deps = [
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubectl/util/term:go_default_library",
        "@com_github_makenowjust_heredoc//:go_default_library",
        "@com_github_russross_blackfriday//:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = [
        "//build/visible_to:pkg_kubectl_cmd_templates_CONSUMERS",
    ],
)
