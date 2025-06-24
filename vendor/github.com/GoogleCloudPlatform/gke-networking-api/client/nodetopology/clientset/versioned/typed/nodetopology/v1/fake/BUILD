load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_nodetopology.go",
        "fake_nodetopology_client.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/typed/nodetopology/v1/fake",
    importpath = "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/typed/nodetopology/v1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1:nodetopology",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/typed/nodetopology/v1:nodetopology",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
