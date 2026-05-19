"""Verify that bazel/flavors/defs.bzl is in sync with tasks/build_tags.py and
with the FlavorUnitTestTags map exported by bazel/rules/dd_agent_go_test.

Run with: bazel test //bazel/flavors:verify_flavor_tags
"""

import importlib.util
import subprocess
import sys
import types
import unittest

from python.runfiles import runfiles


def _load_tasks():
    """Load tasks/flavor.py and tasks/build_tags.py via importlib, bypassing tasks/__init__.py."""
    # Provide a minimal tasks package stub so build_tags.py can do
    # "from tasks.flavor import AgentFlavor" without executing __init__.py,
    # which would pull in dozens of unrelated modules.
    if "tasks" not in sys.modules:
        sys.modules["tasks"] = types.ModuleType("tasks")

    def _load(mod_name, rel_path):
        spec = importlib.util.spec_from_file_location(mod_name, rel_path)
        mod = importlib.util.module_from_spec(spec)
        sys.modules[mod_name] = mod
        spec.loader.exec_module(mod)
        return mod

    flavor = _load("tasks.flavor", "tasks/flavor.py")
    bt = _load("tasks.build_tags", "tasks/build_tags.py")
    return flavor, bt


def _expected_tags(flavor_mod, bt_mod):
    result = {}
    for f in flavor_mod.AgentFlavor:
        tags = bt_mod.build_tags[f].get("unit-tests", set())
        result[f.name] = sorted(tags | bt_mod.COMMON_TAGS)
    return result


def _load_defs_bzl():
    ns = {}
    with open("bazel/flavors/defs.bzl") as f:
        exec(f.read(), ns)  # noqa: S102
    return ns


def _load_go_extension_tags(dump_tags_rlocation):
    """Run //bazel/rules/dd_agent_go_test/dump_tags and parse its `flavor\\ttag` lines.

    Running the binary instead of parsing the Go source means this test stays
    blind to the extension's internal layout (single map, composed-at-init,
    multiple files, ...) — it only asserts the externally-visible tag sets.
    """
    bin_path = runfiles.Create().Rlocation(dump_tags_rlocation)
    out = subprocess.check_output([bin_path], text=True)
    result = {}
    for line in out.splitlines():
        if not line:
            continue
        flavor, tag = line.split("\t", 1)
        result.setdefault(flavor, []).append(tag)
    return {flavor: sorted(tags) for flavor, tags in result.items()}


class TestFlavorTagsSync(unittest.TestCase):
    # Set by __main__ from argv before unittest.main() dispatches setUpClass.
    dump_tags_rlocation: str = ""

    @classmethod
    def setUpClass(cls):
        cls.flavor_mod, cls.bt_mod = _load_tasks()
        cls.expected = _expected_tags(cls.flavor_mod, cls.bt_mod)
        cls.defs = _load_defs_bzl()
        cls.go_tags = _load_go_extension_tags(cls.dump_tags_rlocation)

    def test_all_flavors_present(self):
        self.assertEqual(set(self.expected), set(self.defs["FLAVOR_UNIT_TEST_TAGS"]))

    def test_flavor_tag_sets(self):
        for flavor_name, expected_tags in self.expected.items():
            with self.subTest(flavor=flavor_name):
                actual = sorted(self.defs["FLAVOR_UNIT_TEST_TAGS"][flavor_name])
                self.assertEqual(expected_tags, actual)

    def test_linux_only_tags(self):
        expected = sorted(self.bt_mod.LINUX_ONLY_TAGS)
        actual = sorted(self.defs["LINUX_ONLY_TAGS"])
        self.assertEqual(expected, actual)

    def test_windows_include_tags(self):
        # tasks/build_tags.py: `if platform == "win32": include.union(["wmi"])`.
        # Single tag today; mirror exactly so any future divergence (an extra
        # platform-conditional tag) fails this test loudly.
        self.assertEqual(["wmi"], sorted(self.defs["WINDOWS_INCLUDE_TAGS"]))

    def test_windows_exclude_tags(self):
        expected = sorted(self.bt_mod.WINDOWS_EXCLUDE_TAGS)
        actual = sorted(self.defs["WINDOWS_EXCLUDE_TAGS"])
        self.assertEqual(expected, actual)

    def test_darwin_exclude_tags(self):
        expected = sorted(self.bt_mod.DARWIN_EXCLUDED_TAGS)
        actual = sorted(self.defs["DARWIN_EXCLUDE_TAGS"])
        self.assertEqual(expected, actual)

    def test_go_extension_flavor_tag_sets(self):
        for flavor_name, expected_tags in self.expected.items():
            with self.subTest(flavor=flavor_name):
                self.assertIn(flavor_name, self.go_tags, f"{flavor_name} missing from Go FlavorUnitTestTags")
                self.assertEqual(expected_tags, self.go_tags[flavor_name])

    def test_go_extension_has_no_extra_flavors(self):
        self.assertEqual(set(self.expected), set(self.go_tags))


if __name__ == "__main__":
    # Pop the dump_tags rlocation off argv before unittest.main() sees it —
    # unittest treats positional args as test selectors.
    TestFlavorTagsSync.dump_tags_rlocation = sys.argv.pop(1)
    unittest.main()
