package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["utils.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/storage/etcd/testing",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/storage/etcd/testing",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd/etcdtest:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd/testing/testingcert:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/storagebackend:go_default_library",
        "@com_github_coreos_etcd//client:go_default_library",
        "@com_github_coreos_etcd//clientv3:go_default_library",
        "@com_github_coreos_etcd//etcdserver:go_default_library",
        "@com_github_coreos_etcd//etcdserver/api/etcdhttp:go_default_library",
        "@com_github_coreos_etcd//etcdserver/api/v2http:go_default_library",
        "@com_github_coreos_etcd//integration:go_default_library",
        "@com_github_coreos_etcd//pkg/testutil:go_default_library",
        "@com_github_coreos_etcd//pkg/transport:go_default_library",
        "@com_github_coreos_etcd//pkg/types:go_default_library",
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
        "//staging/src/k8s.io/apiserver/pkg/storage/etcd/testing/testingcert:all-srcs",
    ],
    tags = ["automanaged"],
)
