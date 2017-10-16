package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["circonus.go"],
    visibility = ["//visibility:public"],
    deps = [
        "//adapter/circonus/config:go_default_library",
        "//pkg/adapter:go_default_library",
        "//template/metric:go_default_library",
        "@com_github_circonuslabs_circonus_gometrics//:go_default_library",
        "@com_github_hashicorp_go_multierror//:go_default_library",
        "@com_github_gogo_protobuf//types:go_default_library",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
    ],
)


load("@io_bazel_rules_go//go:def.bzl", "go_test")
go_test(
    name = "go_default_test",
    size = "small",
    srcs = ["circonus_test.go"],
    library = ":go_default_library",
    data = [
        "operatorconfig",
    ],
    deps = [
        "//pkg/adapter:go_default_library",
        "//pkg/adapter/test:go_default_library",
        "//pkg/template:go_default_library",
        "//test/testenv:go_default_library",
        "//template:go_default_library",
        "@io_istio_api//:mixer/v1",
        "@org_golang_x_net//context:go_default_library",
        "@io_istio_api//:mixer/v1/config/descriptor",
    ],
)
