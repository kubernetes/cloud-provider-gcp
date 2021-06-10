load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["provider.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/kubectl-gke-exec-auth-plugin/provider",
    visibility = [
        "//visibility:public",
    ],
    deps = [
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1:go_default_library",
    ],
)
