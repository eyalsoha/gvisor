load("//tools/go_stateify:defs.bzl", "go_library")

package(licenses = ["notice"])

go_library(
    name = "fasync",
    srcs = ["fasync.go"],
    importpath = "gvisor.dev/gvisor/pkg/sentry/kernel/fasync",
    visibility = ["//:sandbox"],
    deps = [
        "//pkg/abi/linux",
        "//pkg/sentry/fs",
        "//pkg/sentry/kernel",
        "//pkg/sentry/kernel/auth",
        "//pkg/sync",
        "//pkg/waiter",
    ],
)
