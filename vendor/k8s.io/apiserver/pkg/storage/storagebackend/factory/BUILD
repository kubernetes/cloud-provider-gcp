load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "etcd2.go",
        "etcd3.go",
        "factory.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/storage/storagebackend/factory",
    importpath = "k8s.io/apiserver/pkg/storage/storagebackend/factory",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd3:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/storagebackend:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value:go_default_library",
        "@com_github_coreos_etcd//client:go_default_library",
        "@com_github_coreos_etcd//clientv3:go_default_library",
        "@com_github_coreos_etcd//pkg/transport:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["tls_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/testing:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/example:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/example/v1:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd/testing/testingcert:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/storagebackend:go_default_library",
        "@com_github_coreos_etcd//integration:go_default_library",
        "@com_github_coreos_etcd//pkg/transport:go_default_library",
    ],
)
