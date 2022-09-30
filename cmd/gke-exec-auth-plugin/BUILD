load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "gke-exec-auth-plugin_lib",
    srcs = [
        "alt_token_source.go",
        "cache.go",
        "main.go",
        "request.go",
        "tpm.go",
        "tpm_other.go",
        "tpm_windows.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gke-exec-auth-plugin",
    visibility = ["//visibility:private"],
    deps = [
        "//pkg/nodeidentity",
        "//pkg/tpmattest",
        "//vendor/cloud.google.com/go/compute/metadata",
        "//vendor/github.com/gofrs/flock",
        "//vendor/github.com/google/go-tpm/tpm2",
        "//vendor/github.com/google/go-tpm/tpmutil",
        "//vendor/golang.org/x/oauth2",
        "//vendor/golang.org/x/oauth2/google",
        "//vendor/google.golang.org/api/googleapi",
        "//vendor/k8s.io/api/certificates/v1:certificates",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/util/cert",
        "//vendor/k8s.io/client-go/util/certificate/csr",
        "//vendor/k8s.io/client-go/util/keyutil",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_binary(
    name = "gke-exec-auth-plugin",
    embed = [":gke-exec-auth-plugin_lib"],
    pure = "on",
    visibility = ["//visibility:public"],
)

go_test(
    name = "gke-exec-auth-plugin_test",
    srcs = [
        "cache_test.go",
        "request_test.go",
        "tpm_test.go",
    ],
    embed = [":gke-exec-auth-plugin_lib"],
    deps = [
        "//pkg/nodeidentity",
        "//vendor/github.com/google/go-tpm/tpm2",
        "//vendor/github.com/google/go-tpm/tpmutil",
        "//vendor/k8s.io/client-go/rest",
    ],
)
