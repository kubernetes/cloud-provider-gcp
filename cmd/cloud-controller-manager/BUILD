package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
    "go_cross_binary",
)
load("@rules_pkg//:pkg.bzl", "pkg_tar")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_image_index", "oci_load", "oci_push")
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "cloud-controller-manager_bin",
    embed = [":cloud-controller-manager_lib"],
    pure = "on",
    x_defs = version_x_defs(),
    visibility = ["//visibility:private"],
)

go_library(
    name = "cloud-controller-manager_lib",
    srcs = [
        "gkenetworkparamsetcontroller.go",
        "gkeservicecontroller.go",
        "main.go",
        "nodeipamcontroller.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager",
    deps = [
        "//cmd/cloud-controller-manager/options",
        "//pkg/controller/gkenetworkparamset",
        "//pkg/controller/nodeipam",
        "//pkg/controller/nodeipam/config",
        "//pkg/controller/nodeipam/ipam",
        "//pkg/controller/service",
        "//providers/gce",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned",
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/app",
        "//vendor/k8s.io/cloud-provider/app/config",
        "//vendor/k8s.io/cloud-provider/names",
        "//vendor/k8s.io/cloud-provider/options",
        "//vendor/k8s.io/component-base/cli/flag",
        "//vendor/k8s.io/component-base/logs",
        "//vendor/k8s.io/component-base/metrics/prometheus/clientgo",
        "//vendor/k8s.io/component-base/metrics/prometheus/version",
        "//vendor/k8s.io/controller-manager/app",
        "//vendor/k8s.io/controller-manager/controller",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kubernetes/cmd/kube-controller-manager/names",
        "//vendor/k8s.io/utils/net",
    ],
)

go_test(
    name = "cloud-controller-manager_test",
    srcs = [
        "gkeservicecontroller_test.go",
        "nodeipamcontroller_test.go",
    ],
    embed = [":cloud-controller-manager_lib"],
    deps = [
        "//pkg/controller/nodeipam/config",
        "//pkg/controller/service",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/app/config",
        "//vendor/k8s.io/cloud-provider/config",
        "//vendor/k8s.io/controller-manager/app",
    ],
)

go_cross_binary(
    name = "bin_amd64",
    platform = "@io_bazel_rules_go//go/toolchain:linux_amd64",
    target = ":cloud-controller-manager_bin",
)

go_cross_binary(
    name = "bin_arm64",
    platform = "@io_bazel_rules_go//go/toolchain:linux_arm64",
    target = ":cloud-controller-manager_bin",
)

pkg_tar(
    name = "layer_amd64",
    srcs = [":bin_amd64"],
    package_dir = "/usr/local/bin",
)

pkg_tar(
    name = "layer_arm64",
    srcs = [":bin_arm64"],
    package_dir = "/usr/local/bin",
)

oci_image(
    name = "image_amd64",
    entrypoint = ["/usr/local/bin/bin_amd64"],
    tars = [":layer_amd64"],
    os = "linux",
    architecture = "amd64",
)

oci_image(
    name = "image_arm64",
    entrypoint = ["/usr/local/bin/bin_arm64"],
    tars = [":layer_arm64"],
    os = "linux",
    architecture = "arm64",
)

oci_load(
    name = "load_amd64",
    image = ":image_amd64",
    repo_tags = ["my-repo/cloud-controller-manager:local"],
)

oci_load(
    name = "load_arm64",
    image = ":image_arm64",
    repo_tags = ["my-repo/cloud-controller-manager:local"],
)

oci_image_index(
    name = "cloud-controller-manager_index",
    images = [
        ":image_amd64",
        ":image_arm64",
    ],
)

oci_push(
    name = "push_multiplatform_to_gcr",
    image = ":cloud-controller-manager_index",
    repository = "{repository_placeholder}",
)

# Legacy single platform image
# Left to keep compatibility with existing Google pipeline 
# To be removed once updated the pipeline to use multi platform image
load("//defs:container.bzl", "image")
image(binary = ":cloud-controller-manager_bin")
