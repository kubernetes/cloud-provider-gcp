load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "mutex.go",
        "mutex_go17.go",
        "profiler.go",
        "symbolizer.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/profiler",
    importpath = "cloud.google.com/go/profiler",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/compute/metadata:go_default_library",
        "//vendor/cloud.google.com/go/internal/version:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/transport/grpc:go_default_library",
        "@com_github_google_pprof//profile:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_genproto//googleapis/devtools/cloudprofiler/v2:go_default_library",
        "@org_golang_google_genproto//googleapis/rpc/errdetails:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "profiler_example_test.go",
        "profiler_test.go",
        "symbolizer_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/cloud.google.com/go/profiler/mocks:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/transport/grpc:go_default_library",
        "@com_github_golang_mock//gomock:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@com_github_google_pprof//profile:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_genproto//googleapis/devtools/cloudprofiler/v2:go_default_library",
        "@org_golang_google_genproto//googleapis/rpc/errdetails:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
