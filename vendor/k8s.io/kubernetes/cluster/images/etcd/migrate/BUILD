package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)

go_binary(
    name = "migrate",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = [
        "data_dir.go",
        "migrate.go",
        "migrate_client.go",
        "migrate_server.go",
        "migrator.go",
        "rollback_v2.go",
        "versions.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cluster/images/etcd/migrate",
    importpath = "k8s.io/kubernetes/cluster/images/etcd/migrate",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/k8s.io/kubernetes/third_party/forked/etcd221/wal:go_default_library",
        "@com_github_blang_semver//:go_default_library",
        "@com_github_coreos_etcd//client:go_default_library",
        "@com_github_coreos_etcd//clientv3:go_default_library",
        "@com_github_coreos_etcd//etcdserver:go_default_library",
        "@com_github_coreos_etcd//etcdserver/etcdserverpb:go_default_library",
        "@com_github_coreos_etcd//etcdserver/membership:go_default_library",
        "@com_github_coreos_etcd//mvcc/backend:go_default_library",
        "@com_github_coreos_etcd//mvcc/mvccpb:go_default_library",
        "@com_github_coreos_etcd//pkg/pbutil:go_default_library",
        "@com_github_coreos_etcd//pkg/types:go_default_library",
        "@com_github_coreos_etcd//raft/raftpb:go_default_library",
        "@com_github_coreos_etcd//snap:go_default_library",
        "@com_github_coreos_etcd//store:go_default_library",
        "@com_github_coreos_etcd//wal:go_default_library",
        "@com_github_coreos_etcd//wal/walpb:go_default_library",
        "@com_github_coreos_go_semver//semver:go_default_library",
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

go_test(
    name = "go_default_test",
    srcs = [
        "data_dir_test.go",
        "versions_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = ["@com_github_blang_semver//:go_default_library"],
)
