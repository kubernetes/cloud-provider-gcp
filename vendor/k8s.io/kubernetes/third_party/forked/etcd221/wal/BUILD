package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "decoder.go",
        "doc.go",
        "encoder.go",
        "metrics.go",
        "multi_readcloser.go",
        "repair.go",
        "util.go",
        "wal.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/third_party/forked/etcd221/wal",
    importpath = "k8s.io/kubernetes/third_party/forked/etcd221/wal",
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
        "//vendor/k8s.io/kubernetes/third_party/forked/etcd221/pkg/fileutil:go_default_library",
        "@com_github_coreos_etcd//pkg/crc:go_default_library",
        "@com_github_coreos_etcd//pkg/pbutil:go_default_library",
        "@com_github_coreos_etcd//raft:go_default_library",
        "@com_github_coreos_etcd//raft/raftpb:go_default_library",
        "@com_github_coreos_etcd//wal/walpb:go_default_library",
        "@com_github_coreos_pkg//capnslog:go_default_library",
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
        "//third_party/forked/etcd221/wal/walpb:all-srcs",
    ],
    tags = ["automanaged"],
)
