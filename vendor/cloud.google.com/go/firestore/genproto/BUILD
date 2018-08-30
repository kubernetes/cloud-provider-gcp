load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["test.pb.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/firestore/genproto",
    importpath = "cloud.google.com/go/firestore/genproto",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "@io_bazel_rules_go//proto/wkt:timestamp_go_proto",
        "@org_golang_google_genproto//googleapis/firestore/v1beta1:go_default_library",
    ],
)
