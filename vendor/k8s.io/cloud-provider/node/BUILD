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
        "//vendor/k8s.io/cloud-provider/node/helpers:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
