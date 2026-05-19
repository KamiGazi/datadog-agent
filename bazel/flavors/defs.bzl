# Flavor unit-test tag sets for the Datadog Agent, mirroring tasks/build_tags.py.
#
# Use flavor_gotags(flavor_name) to get a gotags-ready value for a go_test
# rule; it handles the platform-specific select() automatically.
#
# To verify this file stays in sync with tasks/build_tags.py:
#   bazel test //bazel/flavors:verify_flavor_tags

# Tags compute_build_tags_for_flavor() drops on non-Linux platforms.
LINUX_ONLY_TAGS = [
    "crio",
    "jetson",
    "linux_bpf",
    "netcgo",
    "nvml",
    "pcap",
    "podman",
    "systemd",
    "trivy",
]

# Tags added on top of a flavor's set when targeting Windows.
WINDOWS_INCLUDE_TAGS = ["wmi"]

# Tags dropped from a flavor's set when targeting Windows.
WINDOWS_EXCLUDE_TAGS = ["requirefips"]

# Tags dropped from a flavor's set when targeting macOS.
DARWIN_EXCLUDE_TAGS = ["containerd", "cri", "docker"]

# Tags added unconditionally to every flavor's tag set.
COMMON_TAGS = [
    "grpcnotrace",
    "no_dynamic_plugins",
    "retrynotrace",
    "trivy_no_javadb",
]

# Tag added on top of a flavor's set when running unit tests.
UNIT_TEST_TAGS = ["test"]

# Per-flavor tags, with COMMON_TAGS and UNIT_TEST_TAGS factored out (composed
# back below). Keep each list sorted alphabetically.
_FLAVOR_SPECIFIC_TAGS = {
    "base": [
        "cel",
        "clusterchecks",
        "consul",
        "containerd",
        "cri",
        "crio",
        "docker",
        "ec2",
        "etcd",
        "fargateprocess",
        "jetson",
        "jmx",
        "kubeapiserver",
        "kubelet",
        "ncm",
        "netcgo",
        "nvml",
        "oracle",
        "orchestrator",
        "otlp",
        "podman",
        "python",
        "sharedlibrarycheck",
        "systemd",
        "systemprobechecks",
        "trivy",
        "zk",
        "zlib",
        "zstd",
    ],
    "fips": [
        "cel",
        "consul",
        "containerd",
        "cri",
        "crio",
        "docker",
        "ec2",
        "etcd",
        "fargateprocess",
        "goexperiment.systemcrypto",
        "jetson",
        "jmx",
        "kubeapiserver",
        "kubelet",
        "ncm",
        "netcgo",
        "nvml",
        "oracle",
        "orchestrator",
        "otlp",
        "podman",
        "python",
        "requirefips",
        "sharedlibrarycheck",
        "systemd",
        "systemprobechecks",
        "trivy",
        "zk",
        "zlib",
        "zstd",
    ],
    "heroku": [
        "bundle_installer",
        "consul",
        "etcd",
        "jmx",
        "ncm",
        "netcgo",
        "otlp",
        "python",
        "sharedlibrarycheck",
        "systemprobechecks",
        "zk",
        "zlib",
        "zstd",
    ],
    "iot": [
        "jetson",
        "systemd",
        "zlib",
        "zstd",
    ],
    "dogstatsd": [
        "containerd",
        "docker",
        "kubelet",
        "podman",
        "zlib",
        "zstd",
    ],
}

FLAVOR_UNIT_TEST_TAGS = {
    flavor: _FLAVOR_SPECIFIC_TAGS[flavor] + COMMON_TAGS + UNIT_TEST_TAGS
    for flavor in _FLAVOR_SPECIFIC_TAGS
}

def _without(tags, excluded):
    return [t for t in tags if t not in excluded]

def flavor_gotags(flavor_name):
    """Returns the platform-aware gotags select() for a go_test rule.

    Mirrors compute_build_tags_for_flavor() in tasks/build_tags.py:
    LINUX_ONLY_TAGS are dropped off-Linux, Windows additionally adds
    WINDOWS_INCLUDE_TAGS and drops WINDOWS_EXCLUDE_TAGS, macOS drops
    DARWIN_EXCLUDE_TAGS.

    Args:
        flavor_name: key of FLAVOR_UNIT_TEST_TAGS.

    Returns:
        select() yielding the per-platform build-tag list for the flavor.
    """
    tags = FLAVOR_UNIT_TEST_TAGS[flavor_name]
    return select(
        {
            "@platforms//os:linux": tags,
            "@platforms//os:windows": _without(tags, LINUX_ONLY_TAGS + WINDOWS_EXCLUDE_TAGS) + WINDOWS_INCLUDE_TAGS,
            "@platforms//os:macos": _without(tags, LINUX_ONLY_TAGS + DARWIN_EXCLUDE_TAGS),
        },
        no_match_error = "flavor_gotags: only linux/macos/windows are supported",
    )
