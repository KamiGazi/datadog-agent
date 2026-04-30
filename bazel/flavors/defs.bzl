# Flavor unit-test tag sets for the Datadog Agent, mirroring tasks/build_tags.py.
#
# FLAVOR_UNIT_TEST_TAGS maps each AgentFlavor name to the tag set used when
# running unit tests for that flavor:
#
#   build_tags[flavor]["unit-tests"].union(COMMON_TAGS)
#
# Use flavor_gotags(flavor_name) to get a gotags-ready value for a go_test rule.
# It handles the platform-specific select() automatically.
#
# To verify this file is in sync with tasks/build_tags.py:
#   bazel test //bazel/flavors:verify_flavor_tags

# LINUX_ONLY_TAGS mirrors LINUX_ONLY_TAGS from tasks/build_tags.py.
# dda inv test never passes these tags on non-Linux platforms.
# Prefer flavor_gotags() over consulting this list directly.
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

# Tags compute_build_tags_for_flavor() adds when targeting Windows.
# Mirrors `if platform == "win32": include.union([...])` in tasks/build_tags.py.
WINDOWS_INCLUDE_TAGS = ["wmi"]

# Tags compute_build_tags_for_flavor() drops when targeting Windows.
# Mirrors WINDOWS_EXCLUDE_TAGS in tasks/build_tags.py.
WINDOWS_EXCLUDE_TAGS = ["requirefips"]

# Tags compute_build_tags_for_flavor() drops when targeting macOS.
# Mirrors DARWIN_EXCLUDED_TAGS in tasks/build_tags.py.
DARWIN_EXCLUDE_TAGS = ["containerd", "cri", "docker"]

# FLAVOR_UNIT_TEST_TAGS maps each AgentFlavor name to its unit-test tag set.
# Each list includes COMMON_TAGS and the "test" tag.
# Tags from UNIT_TEST_EXCLUDE_TAGS (datadog.no_waf, pcap) are absent.
# Tags are sorted alphabetically.
FLAVOR_UNIT_TEST_TAGS = {
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
        "grpcnotrace",
        "jetson",
        "jmx",
        "kubeapiserver",
        "kubelet",
        "ncm",
        "netcgo",
        "no_dynamic_plugins",
        "nvml",
        "oracle",
        "orchestrator",
        "otlp",
        "podman",
        "python",
        "retrynotrace",
        "sharedlibrarycheck",
        "systemd",
        "systemprobechecks",
        "test",
        "trivy",
        "trivy_no_javadb",
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
        "grpcnotrace",
        "jetson",
        "jmx",
        "kubeapiserver",
        "kubelet",
        "ncm",
        "netcgo",
        "no_dynamic_plugins",
        "nvml",
        "oracle",
        "orchestrator",
        "otlp",
        "podman",
        "python",
        "requirefips",
        "retrynotrace",
        "sharedlibrarycheck",
        "systemd",
        "systemprobechecks",
        "test",
        "trivy",
        "trivy_no_javadb",
        "zk",
        "zlib",
        "zstd",
    ],
    "heroku": [
        "bundle_installer",
        "consul",
        "etcd",
        "grpcnotrace",
        "jmx",
        "ncm",
        "netcgo",
        "no_dynamic_plugins",
        "otlp",
        "python",
        "retrynotrace",
        "sharedlibrarycheck",
        "systemprobechecks",
        "test",
        "trivy_no_javadb",
        "zk",
        "zlib",
        "zstd",
    ],
    "iot": [
        "grpcnotrace",
        "jetson",
        "no_dynamic_plugins",
        "retrynotrace",
        "systemd",
        "test",
        "trivy_no_javadb",
        "zlib",
        "zstd",
    ],
    "dogstatsd": [
        "containerd",
        "docker",
        "grpcnotrace",
        "kubelet",
        "no_dynamic_plugins",
        "podman",
        "retrynotrace",
        "test",
        "trivy_no_javadb",
        "zlib",
        "zstd",
    ],
}

def flavor_gotags(flavor_name):
    """Returns the gotags value for a go_test rule for the given flavor.

    Each platform branch matches what compute_build_tags_for_flavor() in
    tasks/build_tags.py produces for that platform: LINUX_ONLY_TAGS are
    dropped off-Linux, Windows additionally adds WINDOWS_INCLUDE_TAGS and
    drops WINDOWS_EXCLUDE_TAGS, macOS drops DARWIN_EXCLUDE_TAGS.

    Args:
        flavor_name: the flavor name, must be a key of FLAVOR_UNIT_TEST_TAGS.

    Returns:
        A select() yielding the per-platform build-tag list for the flavor.
    """
    tags = FLAVOR_UNIT_TEST_TAGS[flavor_name]
    non_linux_only = [t for t in tags if t not in LINUX_ONLY_TAGS]
    windows = [t for t in non_linux_only if t not in WINDOWS_EXCLUDE_TAGS] + WINDOWS_INCLUDE_TAGS
    darwin = [t for t in non_linux_only if t not in DARWIN_EXCLUDE_TAGS]
    return select({
        "@platforms//os:linux": tags,
        "@platforms//os:windows": windows,
        "@platforms//os:macos": darwin,
        "//conditions:default": non_linux_only,
    })
