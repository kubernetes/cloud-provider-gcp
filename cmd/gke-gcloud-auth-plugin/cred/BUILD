package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)
load("//defs:version.bzl", "version_x_defs")

go_library(
    name = "go_default_library",
    srcs = ["cred.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin/cred",
    deps = [
        "//vendor/github.com/natefinch/atomic:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd/api:go_default_library",
        "//vendor/k8s.io/client-go/util/retry:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["cred_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/google/go-cmp/cmp:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd/api:go_default_library",
    ],
)
