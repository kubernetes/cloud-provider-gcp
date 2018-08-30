load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "appengine.go",
        "default.go",
        "doc_go19.go",
        "doc_not_go19.go",
        "go19.go",
        "google.go",
        "jwt.go",
        "not_go19.go",
        "sdk.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/oauth2/google",
    importpath = "golang.org/x/oauth2/google",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/compute/metadata:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/internal:go_default_library",
        "//vendor/golang.org/x/oauth2/jws:go_default_library",
        "//vendor/golang.org/x/oauth2/jwt:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "example_test.go",
        "google_test.go",
        "jwt_test.go",
        "sdk_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/jws:go_default_library",
        "//vendor/golang.org/x/oauth2/jwt:go_default_library",
        "//vendor/google.golang.org/appengine:go_default_library",
        "//vendor/google.golang.org/appengine/urlfetch:go_default_library",
    ],
)
