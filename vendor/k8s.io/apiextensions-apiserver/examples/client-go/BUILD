filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//vendor/k8s.io/apiextensions-apiserver/examples/client-go/hack:all-srcs",
        "//vendor/k8s.io/apiextensions-apiserver/examples/client-go/pkg/apis/cr:all-srcs",
        "//vendor/k8s.io/apiextensions-apiserver/examples/client-go/pkg/client/clientset/versioned:all-srcs",
        "//vendor/k8s.io/apiextensions-apiserver/examples/client-go/pkg/client/informers/externalversions:all-srcs",
        "//vendor/k8s.io/apiextensions-apiserver/examples/client-go/pkg/client/listers/cr/v1:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
