# flavor_gotags() — platform-aware gotags select() for the dd_agent_go_test
# macro. Tag data is generated from tasks/build_tags.py via
# //bazel/repo:build_tags_codegen.bzl; only the platform composition lives here.

load(
    "@dd_build_tags_codegen//:flavors.bzl",
    "DARWIN_EXCLUDED_TAGS",
    "FLAVOR_UNIT_TEST_TAGS",
    "LINUX_ONLY_TAGS",
    "WINDOWS_EXCLUDED_TAGS",
    "WINDOWS_INCLUDED_TAGS",
)

def _without(tags, excluded):
    return [t for t in tags if t not in excluded]

def flavor_gotags(flavor_name):
    """Returns the platform-aware gotags select() for a go_test rule.

    Mirrors compute_build_tags_for_flavor() in tasks/build_tags.py:
    LINUX_ONLY_TAGS are dropped off-Linux, Windows additionally adds
    WINDOWS_INCLUDED_TAGS and drops WINDOWS_EXCLUDED_TAGS, macOS drops
    DARWIN_EXCLUDED_TAGS.

    Args:
        flavor_name: key of FLAVOR_UNIT_TEST_TAGS.

    Returns:
        select() yielding the per-platform build-tag list for the flavor.
    """
    tags = FLAVOR_UNIT_TEST_TAGS[flavor_name]
    return select(
        {
            "@platforms//os:linux": tags,
            "@platforms//os:windows": _without(tags, LINUX_ONLY_TAGS + WINDOWS_EXCLUDED_TAGS) + WINDOWS_INCLUDED_TAGS,
            "@platforms//os:macos": _without(tags, LINUX_ONLY_TAGS + DARWIN_EXCLUDED_TAGS),
        },
        no_match_error = "flavor_gotags: only linux/macos/windows are supported",
    )
