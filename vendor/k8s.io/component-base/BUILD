filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//vendor/k8s.io/component-base/cli/flag:all-srcs",
        "//vendor/k8s.io/component-base/cli/globalflag:all-srcs",
        "//vendor/k8s.io/component-base/codec:all-srcs",
        "//vendor/k8s.io/component-base/config:all-srcs",
        "//vendor/k8s.io/component-base/featuregate:all-srcs",
        "//vendor/k8s.io/component-base/logs:all-srcs",
        "//vendor/k8s.io/component-base/metrics:all-srcs",
        "//vendor/k8s.io/component-base/version:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
