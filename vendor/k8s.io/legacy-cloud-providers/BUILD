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
        "//vendor/k8s.io/legacy-cloud-providers/aws:all-srcs",
        "//vendor/k8s.io/legacy-cloud-providers/azure:all-srcs",
        "//vendor/k8s.io/legacy-cloud-providers/gce:all-srcs",
        "//vendor/k8s.io/legacy-cloud-providers/openstack:all-srcs",
        "//vendor/k8s.io/legacy-cloud-providers/vsphere:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
