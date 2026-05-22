"""Build-tags library shared between tasks/build_tags.py and the Bazel codegen.

This is import-pure: no `invoke` and no side effects on import. It owns
the canonical sets of build tags, the per-flavor `build_tags` mapping,
and `build_tags_codegen_payload()` — the structured view consumed both
by the dda inv `codegen-to-json` task and by the Bazel codegen entry
script (//bazel/build_tags_codegen:codegen.py). The .bzl/.go rendering
itself lives next to the Bazel rule that needs it.

tasks/build_tags.py re-exports every name defined here so existing call
sites (`from tasks.build_tags import ALL_TAGS`, ...) keep working.
"""

from __future__ import annotations

from tasks.flavor import AgentFlavor

# Common build tags, added on all builds
COMMON_TAGS = {
    # removes the import to golang.org/x/net/trace in google.golang.org/grpc,
    # which prevents dead code elimination, see https://github.com/golang/go/issues/62024
    "grpcnotrace",
    # removes the import to golang.org/x/net/trace in github.com/grpc-ecosystem/go-grpc-middleware
    # which prevents dead code elimination, see https://github.com/golang/go/issues/62024
    "retrynotrace",
    # Disables dynamic plugins in containerd v1, which removes the import to std "plugin" package on Linux amd64,
    # which makes the agent significantly smaller.
    # This can be removed when we start using containerd v2.1 or later.
    "no_dynamic_plugins",
    # Remove some dependencies from Trivy to reduce binary size.
    "trivy_no_javadb",
}

# ALL_TAGS lists all available build tags.
# Used to remove unknown tags from provided tag lists.
ALL_TAGS = {
    "bundle_installer",
    "clusterchecks",
    "consul",
    "containerd",
    "cri",
    "crio",
    # Opt out of the ASM build requirements of dd-trace-go
    "datadog.no_waf",
    "docker",
    "ec2",
    "etcd",
    "fargateprocess",
    "goexperiment.systemcrypto",  # used for FIPS mode
    "jetson",
    "jmx",
    "kubeapiserver",
    "kubelet",
    "linux_bpf",
    "ncm",
    "netcgo",  # Force the use of the CGO resolver. This will also have the effect of making the binary non-static
    "netgo",
    "npm",
    "nvml",  # used for the nvidia go-nvml library
    "oracle",
    "orchestrator",
    "osusergo",
    "otlp",
    "pcap",  # used by system-probe to compile packet filters using google/gopacket/pcap, which requires cgo to link libpcap
    "podman",
    "python",
    "requirefips",  # used for Linux FIPS mode to avoid having to set GOFIPS
    "seclmax",  # used for security agent/system-probe to compile the full feature set of secl
    "serverless",
    "sharedlibrarycheck",
    "systemd",
    "systemprobechecks",  # used to include system-probe based checks in the agent build
    "test",  # used for unit-tests
    "trivy",
    "wmi",
    "zk",
    "zlib",
    "zstd",
    "cel",
    "cws_instrumentation_injector_only",  # used for building cws-instrumentation with only the injector code
    "remove_all_sd",  # remove all discovery provider from prometheusreceiver components
}.union(COMMON_TAGS)

# Tags Gazelle needs to see in addition to ALL_TAGS so it can analyse test-only
# files gated by them. Kept separate because they're test-only and don't belong
# in ALL_TAGS (which is also used to validate user-provided tag lists).
GAZELLE_EXTRA_TAGS = {
    "e2ecoverage",
    "e2eunit",
    "functionaltests",
    "manualtest",
    "private_runner_experimental",
}

# Tags in ALL_TAGS that we deliberately keep out of Gazelle's set, typically
# because they require cgo/native deps that Gazelle's static analysis can't
# resolve cleanly.
GAZELLE_OMIT_TAGS = {"pcap", "remove_all_sd"}

# Build tags Gazelle considers when analysing tag-gated .go files. Consumed by
# //tasks:build_tags_codegen which writes the matching .bzl file loaded by
# the root BUILD.bazel.
GAZELLE_BUILD_TAGS = (ALL_TAGS - GAZELLE_OMIT_TAGS) | GAZELLE_EXTRA_TAGS

### Tag inclusion lists

# AGENT_TAGS lists the tags needed when building the agent.
AGENT_TAGS = {
    "consul",
    "containerd",
    "cri",
    "datadog.no_waf",
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
    "cel",
}

# AGENT_HEROKU_TAGS lists the tags for Heroku agent build
AGENT_HEROKU_TAGS = AGENT_TAGS.difference(
    {
        "containerd",
        "cri",
        "crio",
        "docker",
        "ec2",
        "fargateprocess",
        "jetson",
        "kubeapiserver",
        "kubelet",
        "nvml",
        "oracle",
        "orchestrator",
        "podman",
        "systemd",
        "trivy",
        "cel",
    }
).union(
    {
        "bundle_installer",
    }
)

FIPS_TAGS = {"goexperiment.systemcrypto", "requirefips"}

# CLUSTER_AGENT_TAGS lists the tags needed when building the cluster-agent
CLUSTER_AGENT_TAGS = {
    "clusterchecks",
    "datadog.no_waf",
    "kubeapiserver",
    "orchestrator",
    "zlib",
    "zstd",
    "ec2",
    "cel",
}

# CLUSTER_AGENT_CLOUDFOUNDRY_TAGS lists the tags needed when building the cloudfoundry cluster-agent
CLUSTER_AGENT_CLOUDFOUNDRY_TAGS = {"clusterchecks", "cel"}

# DOGSTATSD_TAGS lists the tags needed when building dogstatsd
DOGSTATSD_TAGS = {"containerd", "docker", "kubelet", "podman", "zlib", "zstd"}

# IOT_AGENT_TAGS lists the tags needed when building the IoT agent
IOT_AGENT_TAGS = {"jetson", "systemd", "zlib", "zstd"}

# INSTALLER_TAGS lists the tags needed when building the installer
INSTALLER_TAGS = {"ec2"}

# PROCESS_AGENT_TAGS lists the tags necessary to build the process-agent
PROCESS_AGENT_TAGS = {
    "containerd",
    "cri",
    "crio",
    "datadog.no_waf",
    "ec2",
    "docker",
    "fargateprocess",
    "kubelet",
    "netcgo",
    "podman",
    "zlib",
    "zstd",
}

# PROCESS_AGENT_HEROKU_TAGS lists the tags necessary to build the process-agent for Heroku
PROCESS_AGENT_HEROKU_TAGS = {
    "datadog.no_waf",
    "fargateprocess",
    "netcgo",
    "zlib",
    "zstd",
}

# SECURITY_AGENT_TAGS lists the tags necessary to build the security agent
SECURITY_AGENT_TAGS = {
    "netcgo",
    "datadog.no_waf",
    "docker",
    "zlib",
    "zstd",
    "ec2",
}

# SBOMGEN_TAGS lists the tags necessary to build sbomgen
SBOMGEN_TAGS = {
    "trivy",
    "containerd",
    "docker",
    "crio",
}

# SERVERLESS_TAGS lists the tags necessary to build serverless
SERVERLESS_TAGS = {"serverless", "otlp"}

# SYSTEM_PROBE_TAGS lists the tags necessary to build system-probe
SYSTEM_PROBE_TAGS = {
    "datadog.no_waf",
    "ec2",
    "linux_bpf",
    "netcgo",
    "npm",
    "nvml",
    "pcap",
    "zlib",
    "zstd",
    "seclmax",
}

# TRACE_AGENT_TAGS lists the tags that have to be added when the trace-agent
TRACE_AGENT_TAGS = {
    "docker",
    "containerd",
    "datadog.no_waf",
    "kubelet",
    "otlp",
    "netcgo",
    "podman",
}

# TRACE_AGENT_HEROKU_TAGS lists the tags necessary to build the trace-agent for Heroku
TRACE_AGENT_HEROKU_TAGS = TRACE_AGENT_TAGS.difference(
    {
        "containerd",
        "docker",
        "kubeapiserver",
        "kubelet",
        "podman",
    }
)

CWS_INSTRUMENTATION_TAGS = {"netgo", "osusergo"}

OTEL_AGENT_TAGS = {"otlp", "zlib", "zstd", "kubelet"}

LOADER_TAGS = set()

# We need to remove all discovery provider from prometheusreceiver components to avoid loading too many dependencies in the host-profiler binary.
# imported by https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/f963ab53ee55aeb56d58617ed12c840e8b07cc53/receiver/prometheusreceiver/factory.go#L10
HOST_PROFILER_TAGS = {"remove_all_sd", "docker", "kubelet"}

PRIVATEACTIONRUNNER_TAGS = set()

SECRET_GENERIC_CONNECTOR_TAGS = set()

# AGENT_TEST_TAGS lists the tags that have to be added to run tests
AGENT_TEST_TAGS = AGENT_TAGS.union({"clusterchecks"})


### Tag exclusion lists

# List of tags to always remove when not building on Linux
LINUX_ONLY_TAGS = {"netcgo", "systemd", "jetson", "linux_bpf", "nvml", "pcap", "podman", "trivy", "crio"}

# List of tags to always remove when building on AIX
AIX_EXCLUDED_TAGS = {
    "cel",
    "clusterchecks",
    "containerd",
    "cri",
    "crio",
    "docker",
    "fargateprocess",
    "jetson",
    "jmx",
    "kubeapiserver",
    "kubelet",
    "linux_bpf",
    "netcgo",
    "npm",
    "nvml",
    "orchestrator",
    "pcap",
    "podman",
    "systemd",
    "systemprobechecks",
    "trivy",
}

# List of tags to always add when building on Windows
WINDOWS_INCLUDED_TAGS = {"wmi"}

# List of tags to always remove when building on Windows
WINDOWS_EXCLUDED_TAGS = {
    "requirefips",
}

# List of tags to always remove when building on Darwin/macOS
DARWIN_EXCLUDED_TAGS = {"docker", "containerd", "cri"}

# Unit test build tags
UNIT_TEST_TAGS = {"test"}

# List of tags to always remove when running unit tests
UNIT_TEST_EXCLUDED_TAGS = {"datadog.no_waf", "pcap"}

# Build type: maps flavor to build tags map
build_tags = {
    AgentFlavor.base: {
        # Build setups
        "agent": AGENT_TAGS,
        "cluster-agent": CLUSTER_AGENT_TAGS,
        "cluster-agent-cloudfoundry": CLUSTER_AGENT_CLOUDFOUNDRY_TAGS,
        "dogstatsd": DOGSTATSD_TAGS,
        "installer": INSTALLER_TAGS,
        "process-agent": PROCESS_AGENT_TAGS,
        "security-agent": SECURITY_AGENT_TAGS,
        "serverless": SERVERLESS_TAGS,
        "system-probe": SYSTEM_PROBE_TAGS,
        "system-probe-unit-tests": SYSTEM_PROBE_TAGS.union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
        "trace-agent": TRACE_AGENT_TAGS,
        "cws-instrumentation": CWS_INSTRUMENTATION_TAGS,
        "sbomgen": SBOMGEN_TAGS,
        "otel-agent": OTEL_AGENT_TAGS,
        "loader": LOADER_TAGS,
        "host-profiler": HOST_PROFILER_TAGS,
        "privateactionrunner": PRIVATEACTIONRUNNER_TAGS,
        "secret-generic-connector": SECRET_GENERIC_CONNECTOR_TAGS,
        # Test setups
        "test": AGENT_TEST_TAGS.union(PROCESS_AGENT_TAGS)
        .union(CLUSTER_AGENT_TAGS)
        .union(UNIT_TEST_TAGS)
        .difference(UNIT_TEST_EXCLUDED_TAGS),
        "lint": AGENT_TEST_TAGS.union(PROCESS_AGENT_TAGS)
        .union(CLUSTER_AGENT_TAGS)
        .union(UNIT_TEST_TAGS)
        .difference(UNIT_TEST_EXCLUDED_TAGS),
        "unit-tests": AGENT_TEST_TAGS.union(PROCESS_AGENT_TAGS)
        .union(CLUSTER_AGENT_TAGS)
        .union(UNIT_TEST_TAGS)
        .difference(UNIT_TEST_EXCLUDED_TAGS),
    },
    AgentFlavor.fips: {
        "agent": AGENT_TAGS.union(FIPS_TAGS),
        "dogstatsd": DOGSTATSD_TAGS.union(FIPS_TAGS),
        "process-agent": PROCESS_AGENT_TAGS.union(FIPS_TAGS),
        "security-agent": SECURITY_AGENT_TAGS.union(FIPS_TAGS),
        "serverless": SERVERLESS_TAGS.union(FIPS_TAGS),
        "system-probe": SYSTEM_PROBE_TAGS.union(FIPS_TAGS),
        "system-probe-unit-tests": SYSTEM_PROBE_TAGS.union(FIPS_TAGS)
        .union(UNIT_TEST_TAGS)
        .difference(UNIT_TEST_EXCLUDED_TAGS),
        "trace-agent": TRACE_AGENT_TAGS.union(FIPS_TAGS),
        "cws-instrumentation": CWS_INSTRUMENTATION_TAGS.union(FIPS_TAGS),
        "sbomgen": SBOMGEN_TAGS.union(FIPS_TAGS),
        "installer": INSTALLER_TAGS.union(FIPS_TAGS),
        "privateactionrunner": PRIVATEACTIONRUNNER_TAGS.union(FIPS_TAGS),
        "secret-generic-connector": SECRET_GENERIC_CONNECTOR_TAGS.union(FIPS_TAGS),
        # Test setups
        "lint": AGENT_TAGS.union(FIPS_TAGS).union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
        "unit-tests": AGENT_TAGS.union(FIPS_TAGS).union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
        "otel-agent": OTEL_AGENT_TAGS.union(FIPS_TAGS),
    },
    AgentFlavor.heroku: {
        "agent": AGENT_HEROKU_TAGS,
        "process-agent": PROCESS_AGENT_HEROKU_TAGS,
        "trace-agent": TRACE_AGENT_HEROKU_TAGS,
        "lint": AGENT_HEROKU_TAGS.union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
        "unit-tests": AGENT_HEROKU_TAGS.union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
    },
    AgentFlavor.iot: {
        "agent": IOT_AGENT_TAGS,
        "lint": IOT_AGENT_TAGS.union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
        "unit-tests": IOT_AGENT_TAGS.union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
    },
    AgentFlavor.dogstatsd: {
        "dogstatsd": DOGSTATSD_TAGS,
        "lint": DOGSTATSD_TAGS.union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
        "unit-tests": DOGSTATSD_TAGS.union(UNIT_TEST_TAGS).difference(UNIT_TEST_EXCLUDED_TAGS),
    },
}


def build_tags_codegen_payload() -> dict[str, object]:
    """Structured view of the tag data consumed by the Bazel codegen.

    All list values are sorted and deduplicated so the generated .bzl / .go
    files are byte-stable.
    """
    return {
        "common_tags": sorted(COMMON_TAGS),
        "unit_test_tags": sorted(UNIT_TEST_TAGS),
        "linux_only_tags": sorted(LINUX_ONLY_TAGS),
        "windows_included_tags": sorted(WINDOWS_INCLUDED_TAGS),
        "windows_excluded_tags": sorted(WINDOWS_EXCLUDED_TAGS),
        "darwin_excluded_tags": sorted(DARWIN_EXCLUDED_TAGS),
        "flavor_specific_tags": {
            flavor.name: sorted(build_tags[flavor]["unit-tests"] - COMMON_TAGS - UNIT_TEST_TAGS)
            for flavor in AgentFlavor
            if "unit-tests" in build_tags.get(flavor, {})
        },
        "gazelle_build_tags": sorted(GAZELLE_BUILD_TAGS),
    }
