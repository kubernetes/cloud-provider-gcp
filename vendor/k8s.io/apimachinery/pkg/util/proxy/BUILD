load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "proxy",
    srcs = [
        "dial.go",
        "doc.go",
        "transport.go",
        "upgradeaware.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/proxy",
    importpath = "k8s.io/apimachinery/pkg/util/proxy",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/mxk/go-flowrate/flowrate",
        "//vendor/golang.org/x/net/html",
        "//vendor/golang.org/x/net/html/atom",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/httpstream",
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/third_party/forked/golang/netutil",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
