"""
Utilities to manage build tags.

The canonical data (tag sets, per-flavor `build_tags` mapping, codegen
serialisers) lives in tasks.libs.common.build_tags so the Bazel codegen
py_binary can import it without pulling in `invoke`. This module keeps
the @task entry points and the pure helpers that operate on that data,
and re-exports every name from the data module so the existing
`from tasks.build_tags import ALL_TAGS` style imports keep working.
"""

from __future__ import annotations

import json
import os
import sys

from invoke import task

from tasks.flavor import AgentFlavor
from tasks.libs.common.build_tags import (
    AGENT_HEROKU_TAGS,
    AGENT_TAGS,
    AGENT_TEST_TAGS,
    AIX_EXCLUDED_TAGS,
    ALL_TAGS,
    CLUSTER_AGENT_CLOUDFOUNDRY_TAGS,
    CLUSTER_AGENT_TAGS,
    COMMON_TAGS,
    CWS_INSTRUMENTATION_TAGS,
    DARWIN_EXCLUDED_TAGS,
    DOGSTATSD_TAGS,
    FIPS_TAGS,
    GAZELLE_BUILD_TAGS,
    GAZELLE_EXTRA_TAGS,
    GAZELLE_OMIT_TAGS,
    HOST_PROFILER_TAGS,
    INSTALLER_TAGS,
    IOT_AGENT_TAGS,
    LINUX_ONLY_TAGS,
    LOADER_TAGS,
    OTEL_AGENT_TAGS,
    PRIVATEACTIONRUNNER_TAGS,
    PROCESS_AGENT_HEROKU_TAGS,
    PROCESS_AGENT_TAGS,
    SBOMGEN_TAGS,
    SECRET_GENERIC_CONNECTOR_TAGS,
    SECURITY_AGENT_TAGS,
    SERVERLESS_TAGS,
    SYSTEM_PROBE_TAGS,
    TRACE_AGENT_HEROKU_TAGS,
    TRACE_AGENT_TAGS,
    UNIT_TEST_EXCLUDED_TAGS,
    UNIT_TEST_TAGS,
    WINDOWS_EXCLUDED_TAGS,
    WINDOWS_INCLUDED_TAGS,
    _build_tags_codegen_payload,
    build_tags,
)

_GOOS_TO_SYS_PLATFORM = {
    "windows": "win32",
}


def _resolve_platform(platform=None):
    """Return the effective target platform as a sys.platform-style string.

    If platform is explicitly provided, normalize it from GOOS format to
    sys.platform format (e.g. "windows" -> "win32"). Otherwise fall back to
    the GOOS env var, then sys.platform.
    """
    if platform is None:
        platform = os.getenv("GOOS") or sys.platform
    return _GOOS_TO_SYS_PLATFORM.get(platform, platform)


def compute_build_tags_for_flavor(
    build: str,
    build_include: str | None,
    build_exclude: str | None,
    flavor: AgentFlavor = AgentFlavor.base,
    platform: str | None = None,
):
    """
    Given a flavor, an architecture, a list of tags to include and exclude, get the final list
    of tags that should be applied.
    If the list of build tags to include is empty, take the default list of build tags for
    the flavor or arch. Otherwise, use the list of build tags to include, minus incompatible tags
    for the given architecture.

    Then, remove from these the provided list of tags to exclude.
    """
    platform = _resolve_platform(platform)

    build_include = (
        get_default_build_tags(build=build, flavor=flavor, platform=platform)
        if build_include is None
        else filter_incompatible_tags(build_include.split(","), platform=platform)
    )

    build_exclude = [] if build_exclude is None else build_exclude.split(",")

    list = get_build_tags(build_include, build_exclude)

    return list


@task
def print_default_build_tags(_, build="agent", flavor=AgentFlavor.base.name, platform: str | None = None):
    """
    Build the default list of tags based on the build type and platform.
    Prints as comma separated list suitable for go tooling (eg, gopls, govulncheck)

    The container integrations are currently only supported on Linux, disabling on
    the Windows and Darwin builds.
    """

    try:
        flavor = AgentFlavor[flavor]
    except KeyError:
        flavorOptions = [flavor.name for flavor in AgentFlavor]
        print(f"'{flavor}' does not correspond to an agent flavor. Options: {flavorOptions}")
        exit(1)

    print(",".join(sorted(get_default_build_tags(build=build, flavor=flavor, platform=platform))))


def get_default_build_tags(build="agent", flavor=AgentFlavor.base, platform: str | None = None):
    """
    Build the default list of tags based on the build type and current platform.

    The container integrations are currently only supported on Linux, disabling on
    the Windows and Darwin builds.
    """
    platform = _resolve_platform(platform)
    include = build_tags[flavor].get(build)
    if include is None:
        print("Warning: unrecognized build type, no build tags included.", file=sys.stderr)
        include = set()

    include = include.union(COMMON_TAGS)
    return sorted(filter_incompatible_tags(include, platform=platform))


def filter_incompatible_tags(include, platform=None):
    """
    Filter out tags incompatible with the platform.
    include can be a list or a set.
    """
    platform = _resolve_platform(platform)
    exclude = set()
    if not platform.startswith("linux"):
        exclude = exclude.union(LINUX_ONLY_TAGS)

    if platform == "win32":
        include = include.union(WINDOWS_INCLUDED_TAGS)
        exclude = exclude.union(WINDOWS_EXCLUDED_TAGS)

    if platform == "darwin":
        exclude = exclude.union(DARWIN_EXCLUDED_TAGS)

    if platform == "aix":
        exclude = exclude.union(AIX_EXCLUDED_TAGS)

    return get_build_tags(include, exclude)


def get_build_tags(include, exclude):
    """
    Build the list of tags based on inclusions and exclusions passed through
    the command line
    include and exclude can be lists or sets.
    """
    # Convert parameters to sets
    include = set(include)
    exclude = set(exclude)

    # filter out unrecognised tags
    known_include = ALL_TAGS.intersection(include)
    unknown_include = include - known_include
    for tag in unknown_include:
        print(f"Warning: unknown build tag '{tag}' was filtered out from included tags list.", file=sys.stderr)

    known_exclude = ALL_TAGS.intersection(exclude)
    unknown_exclude = exclude - known_exclude
    for tag in unknown_exclude:
        print(f"Warning: unknown build tag '{tag}' was filtered out from excluded tags list.", file=sys.stderr)

    return list(known_include - known_exclude)


@task
def audit_tag_impact(ctx, build_exclude=None, csv=False):
    """
    Measure each tag's contribution to the binary size
    """
    build_exclude = [] if build_exclude is None else build_exclude.split(",")

    tags_to_audit = ALL_TAGS.difference(set(build_exclude)).difference(set(IOT_AGENT_TAGS))

    max_size = _compute_build_size(ctx, build_exclude=','.join(build_exclude))
    print(f"size with all tags is {max_size / 1000} kB")

    iot_agent_size = _compute_build_size(ctx, flavor=AgentFlavor.iot)
    print(f"iot agent size is {iot_agent_size / 1000} kB\n")

    report = {"unaccounted": max_size - iot_agent_size, "iot_agent": iot_agent_size}

    for tag in tags_to_audit:
        exclude_string = ','.join(build_exclude + [tag])
        size = _compute_build_size(ctx, build_exclude=exclude_string)
        delta = max_size - size
        print(f"tag {tag} adds {delta / 1000} kB (excludes: {exclude_string})")
        report[tag] = delta
        report["unaccounted"] -= delta

    if csv:
        print("\nCSV output in bytes:")
        for k, v in report.items():
            print(f"{k};{v}")


def _compute_build_size(ctx, build_exclude=None, flavor=AgentFlavor.base):
    import os

    from .agent import build as agent_build

    agent_build(ctx, build_exclude=build_exclude, skip_assets=True, flavor=flavor)

    statinfo = os.stat('bin/agent/agent')
    return statinfo.st_size


def compute_config_build_tags(
    targets="all", build_include=None, build_exclude=None, flavor=AgentFlavor.base.name, platform=None
):
    flavor = AgentFlavor[flavor]

    if targets == "all":
        targets = build_tags[flavor].keys()
    else:
        targets = targets.split(",")
        if not set(targets).issubset(build_tags[flavor]):
            print("Must choose valid targets. Valid targets are:")
            print(f'{", ".join(build_tags[flavor].keys())}')
            exit(1)

    if build_include is None:
        build_include = []
        for target in targets:
            build_include.extend(get_default_build_tags(build=target, flavor=flavor, platform=platform))
    else:
        build_include = filter_incompatible_tags(build_include.split(","), platform=platform)

    build_exclude = [] if build_exclude is None else build_exclude.split(",")
    use_tags = get_build_tags(build_include, build_exclude)
    return use_tags


@task
def codegen_to_json(_, output=""):
    """Emit build-tag data as JSON.

    Writes to --output= path if provided (so callers can sidestep stdout noise
    from dda/rich's console init on Windows), otherwise prints to stdout.
    """
    text = json.dumps(_build_tags_codegen_payload(), indent=2, sort_keys=True)
    if output:
        with open(output, "w") as f:
            f.write(text)
    else:
        print(text)


# Re-exports so existing `from tasks.build_tags import ...` call sites keep
# working without touching ~30 task modules. New code should import from
# tasks.libs.common.build_tags directly.
__all__ = [
    "AGENT_HEROKU_TAGS",
    "AGENT_TAGS",
    "AGENT_TEST_TAGS",
    "AIX_EXCLUDED_TAGS",
    "ALL_TAGS",
    "CLUSTER_AGENT_CLOUDFOUNDRY_TAGS",
    "CLUSTER_AGENT_TAGS",
    "COMMON_TAGS",
    "CWS_INSTRUMENTATION_TAGS",
    "DARWIN_EXCLUDED_TAGS",
    "DOGSTATSD_TAGS",
    "FIPS_TAGS",
    "GAZELLE_BUILD_TAGS",
    "GAZELLE_EXTRA_TAGS",
    "GAZELLE_OMIT_TAGS",
    "HOST_PROFILER_TAGS",
    "INSTALLER_TAGS",
    "IOT_AGENT_TAGS",
    "LINUX_ONLY_TAGS",
    "LOADER_TAGS",
    "OTEL_AGENT_TAGS",
    "PRIVATEACTIONRUNNER_TAGS",
    "PROCESS_AGENT_HEROKU_TAGS",
    "PROCESS_AGENT_TAGS",
    "SBOMGEN_TAGS",
    "SECRET_GENERIC_CONNECTOR_TAGS",
    "SECURITY_AGENT_TAGS",
    "SERVERLESS_TAGS",
    "SYSTEM_PROBE_TAGS",
    "TRACE_AGENT_HEROKU_TAGS",
    "TRACE_AGENT_TAGS",
    "UNIT_TEST_EXCLUDED_TAGS",
    "UNIT_TEST_TAGS",
    "WINDOWS_EXCLUDED_TAGS",
    "WINDOWS_INCLUDED_TAGS",
    "audit_tag_impact",
    "build_tags",
    "codegen_to_json",
    "compute_build_tags_for_flavor",
    "compute_config_build_tags",
    "filter_incompatible_tags",
    "get_build_tags",
    "get_default_build_tags",
    "print_default_build_tags",
]
