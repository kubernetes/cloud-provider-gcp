load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "claims.go",
        "doc.go",
        "ecdsa.go",
        "ecdsa_utils.go",
        "errors.go",
        "hmac.go",
        "map_claims.go",
        "none.go",
        "parser.go",
        "rsa.go",
        "rsa_pss.go",
        "rsa_utils.go",
        "signing_method.go",
        "token.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/dgrijalva/jwt-go",
    importpath = "github.com/dgrijalva/jwt-go",
    visibility = ["//visibility:public"],
)
