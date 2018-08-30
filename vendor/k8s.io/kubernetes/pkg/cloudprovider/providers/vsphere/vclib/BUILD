package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "connection.go",
        "constants.go",
        "custom_errors.go",
        "datacenter.go",
        "datastore.go",
        "folder.go",
        "pbm.go",
        "utils.go",
        "virtualmachine.go",
        "vmoptions.go",
        "volumeoptions.go",
        "vsphere_metrics.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib",
    importpath = "k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere/vclib",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
        "@com_github_vmware_govmomi//find:go_default_library",
        "@com_github_vmware_govmomi//object:go_default_library",
        "@com_github_vmware_govmomi//pbm:go_default_library",
        "@com_github_vmware_govmomi//pbm/types:go_default_library",
        "@com_github_vmware_govmomi//property:go_default_library",
        "@com_github_vmware_govmomi//session:go_default_library",
        "@com_github_vmware_govmomi//sts:go_default_library",
        "@com_github_vmware_govmomi//vim25:go_default_library",
        "@com_github_vmware_govmomi//vim25/mo:go_default_library",
        "@com_github_vmware_govmomi//vim25/soap:go_default_library",
        "@com_github_vmware_govmomi//vim25/types:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//pkg/cloudprovider/providers/vsphere/vclib/diskmanagers:all-srcs",
    ],
    tags = ["automanaged"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "datacenter_test.go",
        "datastore_test.go",
        "folder_test.go",
        "utils_test.go",
        "virtualmachine_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "@com_github_vmware_govmomi//:go_default_library",
        "@com_github_vmware_govmomi//object:go_default_library",
        "@com_github_vmware_govmomi//simulator:go_default_library",
    ],
)
