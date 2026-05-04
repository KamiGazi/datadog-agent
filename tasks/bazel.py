from __future__ import annotations

import json
import re
import subprocess
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import urlparse

from invoke import task

from tasks.build_tags import compute_build_tags_for_flavor
from tasks.flavor import AgentFlavor
from tasks.libs.common.gomodules import AGENT_MODULE_PATH_PREFIX

REPO_ROOT = Path(__file__).parent.parent
TEST_FUNC_RE = re.compile(r'^func (Test\w+)\(', re.MULTILINE)
# `go test` actually runs functions of four shapes: TestX, FuzzX (seed corpus
# under `go test`, full fuzz under `go test -fuzz`), ExampleX, and BenchmarkX
# (under `-bench`). For "does this package have anything to run?" any of them
# counts; matches Bazel's behaviour, since rules_go embeds all four.
_RUNNABLE_FUNC_RE = re.compile(r'^func (?:Test|Fuzz|Example|Benchmark)\w*\(', re.MULTILINE)
_IMPORT_PREFIX = AGENT_MODULE_PATH_PREFIX.rstrip("/")
_FLAVOR_TAG_PREFIX = "flavor_"
# Tag the dd_go_test macro stamps on every variant it emits (defs.bzl). Used to
# distinguish dd_go_test-generated go_test rules from other custom wrappers
# (rtloader_go_test, ...).
_DD_GO_TEST_TAG = "go_tests"


def _load_root_gazelle_excludes() -> set[str]:
    """Parse `# gazelle:exclude <path>` directives from the root BUILD.bazel.

    These mark paths the migration session has deliberately held back from
    Gazelle generation — typically stub `BUILD.bazel` files committed by mass
    migrations (cf. #49305) whose real source files are gated by build tags and
    haven't been wired up yet. The exclude entry is the canonical "not migrated
    yet" marker in this repo; treating it as such here keeps parity in sync
    with the gazelle session.
    """
    excludes: set[str] = set()
    try:
        text = (REPO_ROOT / "BUILD.bazel").read_text()
    except OSError:
        return excludes
    prefix = "# gazelle:exclude "
    for line in text.splitlines():
        s = line.strip()
        if s.startswith(prefix):
            excludes.add(s[len(prefix) :].strip())
    return excludes


def _path_under_excludes(rel: str, excludes: set[str]) -> bool:
    """True if rel is in excludes or sits under a directory listed in excludes.
    Both file-level entries (path/to/foo_test.go) and directory entries
    (path/to/pkg) are matched uniformly.
    """
    parts = rel.split("/")
    for i in range(1, len(parts) + 1):
        if "/".join(parts[:i]) in excludes:
            return True
    return False


def _go_test_packages(tags: list[str]) -> dict[str, list[str]]:
    """Return {import_path: [abs_test_file_paths]} for in-repo packages that
    have test files compiled under the given tags and a BUILD.bazel.

    Uses 'go list ... all' from the workspace root. With go.work, the 'all'
    meta-pattern covers every package in every workspace module — including
    sub-modules with their own go.mod — in a single invocation, where the
    directory-pattern '<root>/...' would otherwise stop at go.mod boundaries
    and miss them.

    Packages held back from Gazelle generation are skipped, whether the
    directive is at package scope (`# gazelle:ignore` in the package's own
    BUILD.bazel) or at workspace scope (`# gazelle:exclude` in the root
    BUILD.bazel). A workspace-scoped exclude on a single test file removes
    only that file; the package stays in scope if other test files remain.
    """
    tag_flag = f"-tags={' '.join(sorted(tags))}"
    result = subprocess.run(
        ["go", "list", "-json", "-e", tag_flag, "all"],
        capture_output=True,
        text=True,
        cwd=REPO_ROOT,
    )
    root_excludes = _load_root_gazelle_excludes()
    pkgs: dict[str, list[str]] = {}
    decoder = json.JSONDecoder()
    text, pos = result.stdout, 0
    while pos < len(text):
        try:
            obj, end = decoder.raw_decode(text, pos)
        except json.JSONDecodeError:
            break
        pos = end
        while pos < len(text) and text[pos].isspace():
            pos += 1
        import_path = obj.get("ImportPath", "")
        if not import_path.startswith(_IMPORT_PREFIX):
            continue
        pkg_dir = Path(obj.get("Dir", ""))
        try:
            pkg_rel = pkg_dir.relative_to(REPO_ROOT).as_posix()
        except ValueError:
            continue
        if _path_under_excludes(pkg_rel, root_excludes):
            continue
        build_file = pkg_dir / "BUILD.bazel"
        if not build_file.is_file():
            continue
        if "# gazelle:ignore" in build_file.read_text():
            continue
        test_files = obj.get("TestGoFiles", []) + obj.get("XTestGoFiles", [])
        test_files = [f for f in test_files if not _path_under_excludes(f"{pkg_rel}/{f}", root_excludes)]
        if not test_files:
            continue
        abs_test_files = [str(pkg_dir / f) for f in test_files]
        # A *_test.go file can exist without anything `go test` would actually
        # run (e.g. dummy sentinels kept just to ship a testdata directory, or
        # files with only helpers). Both `go test` and `bazel test` emit
        # "no tests to run" for those; treat them the same as having no test on
        # the Go side so the comparison stays symmetric with the BEP-derived
        # Bazel set.
        if not _has_runnable_tests(abs_test_files):
            continue
        pkgs[import_path] = abs_test_files
    return pkgs


def _label_to_import_path(label: str) -> str:
    """Convert a Bazel label like '//pkg/util/kernel:kernel_test_iot' into the
    Go import path of the package the test lives in."""
    pkg_part = label.lstrip("/").split(":", 1)[0]
    return _IMPORT_PREFIX if not pkg_part else f"{_IMPORT_PREFIX}/{pkg_part}"


# Emitted by the stdlib testing package when a *_test.go file compiled but
# defined no TestX functions, or when all TestX functions were filtered out by
# -test.run. Identifies a no-op test binary run.
_NO_TESTS_MARKER = "testing: warning: no tests to run"


def _test_log_candidates(
    label: str,
    uri: str,
    cfg_id: str,
    local_exec_root: str | None,
    config_testlogs: dict[str, str],
) -> list[str]:
    """Candidate absolute paths where Bazel may have materialized test.log,
    in priority order.

    BEP's `testActionOutput.uri` is file:// for locally-executed actions but
    bytestream:// for remote-cache hits (BuildBarn on Datadog CI). For
    bytestream://, Bazel still materializes test.log on disk
    (`--remote_download_outputs=toplevel` is the Bazel 7+ default), just not
    at the URI we get from BEP. The convenience symlink `bazel-testlogs`
    isn't an option on CI either (it disables `--noexperimental_convenience_symlinks`).
    The reliable fallback is `<localExecRoot>/<testlogs-dir>/<label>/test.log`,
    where `testlogs-dir` comes from the testResult's configuration BINDIR
    (BINDIR's `bin` sibling).
    """
    paths: list[str] = []
    if uri.startswith("file://"):
        paths.append(urlparse(uri).path)
    testlogs_rel = config_testlogs.get(cfg_id)
    if local_exec_root and testlogs_rel:
        # Label "//pkg/foo:bar_test" -> "pkg/foo/bar_test/test.log".
        label_rel = label.lstrip("/").replace(":", "/")
        paths.append(f"{local_exec_root}/{testlogs_rel}/{label_rel}/test.log")
    return paths


def _test_log_status(paths: list[str]) -> tuple[bool, str]:
    """Return (had_cases, reason) for the first readable path in `paths`.

    had_cases is True iff the Go testing framework actually ran at least one
    TestX. A no-op binary (every *_test.go gated out by //go:build) emits a
    specific warning we grep for. When no path can be read or the log contains
    the marker, reason explains which — surfaced inline with forward failures.

    The caller passes candidate paths in priority order. Bazel also produces a
    test.xml, but rules_go's default go_test action writes only an empty
    <testsuites></testsuites> placeholder unless an external runner like
    gotestsum is wired in. The plain stdout log is the only signal that works
    against the default rules_go configuration.
    """
    errors: list[str] = []
    for path in paths:
        try:
            with open(path) as fh:
                content = fh.read()
        except OSError as e:
            errors.append(f"{type(e).__name__}: {path}")
            continue
        if _NO_TESTS_MARKER in content:
            return False, "test.log contains 'no tests to run' marker"
        return True, ""
    return False, f"test.log unreadable ({'; '.join(errors)})"


@dataclass
class BazelCoverage:
    """Test coverage Bazel reports for the current host, derived from BEP.

    The parity check compares Go-side test discovery against this to decide
    whether each Go test package has a matching Bazel run.
    """

    # Import paths covered by dd_go_test variants Bazel actually exercised,
    # keyed by flavor name.
    dd_covered: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    # Import paths from plain go_test rules (no flavor tag, e.g. rtloader's
    # custom wrapper). Flavor-agnostic; the caller intersects with each
    # flavor's Go-side test set to attribute them.
    plain_paths: set[str] = field(default_factory=set)
    # For each (flavor, import_path) where a dd_go_test variant was *not*
    # counted as covered, why — one entry per rejected variant. Surfaced
    # inline with forward-failure messages so the diagnosis lives in the job
    # log.
    dd_rejections: dict[tuple[str, str], list[str]] = field(default_factory=lambda: defaultdict(list))


def _bazel_covered_packages_from_bep(bep_path: Path) -> BazelCoverage:
    """Parse a Build Event Protocol JSON stream and return two coverage sets:

    1. `dd_covered` — {flavor_name: {import_path}} for dd_go_test variants
       Bazel actually executed with at least one TestX function. Discriminated
       by three orthogonal BEP signals:
         * `targetKind == "go_test rule"`
         * `"go_tests" in tags` (the dd_go_test macro stamps this)
         * `"flavor_<X>" in tags` (the specific flavor)
       and gated on a testResult test.log that doesn't carry the
       "no tests to run" marker — filtering incompatible targets and no-op
       binaries.

    2. `plain_paths` — {import_path} for plain go_test rules (no `go_tests`
       tag), used by custom wrappers (e.g. rtloader_go_test) that don't go
       through dd_go_test. These carry no flavor tag so are flavor-agnostic.
       Under `bazel test --config=<flavor>`, `--test_tag_filters=flavor_<X>`
       excludes them and no testResult fires; targetConfigured is the
       existence signal. Tcompat-incompatible plain go_tests surface as
       `aborted.reason="SKIPPED"` on the corresponding targetCompleted and
       are dropped here. The caller intersects these with each flavor's
       Go-side test set so that labels whose package directory doesn't map
       cleanly to a Go test package (e.g. tests living in a subdirectory
       like `embedded:tmpl_test` actually covering `.../embedded/tmpl`, or
       Bazel-only rule tests outside the Go workspace) are silently ignored
       rather than producing spurious reverse failures.
    """
    target_flavor: dict[str, str] = {}
    plain_go_test_labels: set[str] = set()
    # (uri, config_id) per label so we can recover test.log even when Bazel
    # writes only a bytestream:// URI to the BEP. The convenience symlink
    # `bazel-testlogs` doesn't exist on CI (--noexperimental_convenience_symlinks),
    # so we reconstruct the absolute path from `localExecRoot` and the
    # configuration's BINDIR.
    test_action: dict[str, tuple[str, str]] = {}
    skipped_labels: set[str] = set()
    local_exec_root: str | None = None
    config_testlogs: dict[str, str] = {}

    with open(bep_path) as fh:
        for line in fh:
            if not line.strip():
                continue
            event = json.loads(line)
            eid = event.get("id", {})
            if "workspace" in eid:
                local_exec_root = event.get("workspaceInfo", {}).get("localExecRoot")
            elif "configuration" in eid:
                cfg_id = eid["configuration"].get("id", "")
                bindir = event.get("configuration", {}).get("makeVariable", {}).get("BINDIR", "")
                # BINDIR is "bazel-out/<config-mnemonic>/bin"; testlogs lives
                # alongside as "bazel-out/<config-mnemonic>/testlogs".
                if bindir.endswith("/bin"):
                    config_testlogs[cfg_id] = bindir[: -len("/bin")] + "/testlogs"
            elif "targetConfigured" in eid:
                label = eid["targetConfigured"].get("label", "")
                cfg = event.get("configured", {})
                if cfg.get("targetKind") != "go_test rule":
                    continue
                tags = set(cfg.get("tag") or [])
                if _DD_GO_TEST_TAG in tags:
                    flavor = next(
                        (t[len(_FLAVOR_TAG_PREFIX) :] for t in tags if t.startswith(_FLAVOR_TAG_PREFIX)),
                        None,
                    )
                    if flavor is not None:
                        target_flavor[label] = flavor
                else:
                    plain_go_test_labels.add(label)
            elif "testResult" in eid:
                label = eid["testResult"].get("label", "")
                cfg_id = eid["testResult"].get("configuration", {}).get("id", "")
                for out in event["testResult"].get("testActionOutput") or []:
                    if out.get("name") == "test.log":
                        test_action[label] = (out.get("uri", ""), cfg_id)
                        break
            elif "targetCompleted" in eid:
                if event.get("aborted", {}).get("reason") == "SKIPPED":
                    skipped_labels.add(eid["targetCompleted"].get("label", ""))

    coverage = BazelCoverage()
    for label, flavor in target_flavor.items():
        import_path = _label_to_import_path(label)
        action = test_action.get(label)
        if action is None:
            # No testResult event. Either tag-filtered out by --config=<flavor>
            # (analysis ran but execution didn't) or target_compatible_with
            # rejected the target. Both mean "not covered" for parity.
            reason = (
                "skipped by target_compatible_with"
                if label in skipped_labels
                else "no testResult (likely filtered by --test_tag_filters)"
            )
            coverage.dd_rejections[(flavor, import_path)].append(f"{label}: {reason}")
            continue
        uri, cfg_id = action
        had_cases, reason = _test_log_status(_test_log_candidates(label, uri, cfg_id, local_exec_root, config_testlogs))
        if not had_cases:
            coverage.dd_rejections[(flavor, import_path)].append(f"{label}: {reason}")
            continue
        coverage.dd_covered[flavor].add(import_path)

    coverage.plain_paths = {
        _label_to_import_path(label) for label in plain_go_test_labels if label not in skipped_labels
    }
    return coverage


def _test_funcs(file_paths: list[str]) -> set[str]:
    funcs: set[str] = set()
    for p in file_paths:
        try:
            funcs.update(TEST_FUNC_RE.findall(Path(p).read_text()))
        except OSError:
            pass
    return funcs


def _has_runnable_tests(file_paths: list[str]) -> bool:
    """Report whether any of the given *_test.go files defines a function the
    Go test toolchain would discover (TestX/FuzzX/ExampleX/BenchmarkX)."""
    for p in file_paths:
        try:
            if _RUNNABLE_FUNC_RE.search(Path(p).read_text()):
                return True
        except OSError:
            pass
    return False


@task(
    help={
        "flavor_name": f"Agent flavor ({', '.join(f.name for f in AgentFlavor)}). Default: all.",
        "bep": "Path to the build_event_json_file produced by the preceding "
        "'bazel test ...' invocation. The same file covers every flavor "
        "variant the test run included.",
        "verbose": "Print passing packages.",
    },
)
def ensure_test_parity(ctx, bep, flavor_name=None, verbose=False):
    """
    Verify every Go test visible to 'dda inv test --flavor=<f>' has a
    matching Bazel go_test target that actually executed at least one test
    case for the same flavor.

    Reads test execution outcomes from a Bazel Build Event Protocol JSON
    stream (--build_event_json_file output). Targets that Bazel skipped
    (target_compatible_with) or that compiled zero TestX functions (every
    *_test.go filtered out by //go:build) are correctly omitted from the
    coverage set without any extra platform reasoning here.

    Packages with no BUILD.bazel or carrying '# gazelle:ignore' are silently
    skipped (not yet migrated). Exits 1 if any gap is found.
    """
    bep_path = Path(bep) if bep else None
    if bep_path is None or not bep_path.is_file():
        print(f"error: BEP file not found: {bep}", file=sys.stderr)
        sys.exit(2)

    flavors = list(AgentFlavor)
    if flavor_name:
        try:
            flavors = [AgentFlavor[flavor_name]]
        except KeyError:
            print(f"Unknown flavor '{flavor_name}'. Options: {[f.name for f in AgentFlavor]}", file=sys.stderr)
            sys.exit(1)

    coverage = _bazel_covered_packages_from_bep(bep_path)

    failed = False
    for flavor in flavors:
        tags = compute_build_tags_for_flavor("unit-tests", None, None, flavor)
        test_pkgs = _go_test_packages(tags)
        # Plain go_tests (no flavor tag) are flavor-agnostic; only count those
        # whose import_path is a real Go test package for this flavor, so labels
        # with mismatched dirs (subdir-located tests, Bazel-only rule tests
        # outside the Go workspace) don't produce spurious reverse failures.
        bazel_pkgs = coverage.dd_covered.get(flavor.name, set()) | (coverage.plain_paths & test_pkgs.keys())
        for import_path, test_files in sorted(test_pkgs.items()):
            if import_path in bazel_pkgs:
                bazel_pkgs.discard(import_path)
                if verbose:
                    funcs = _test_funcs(test_files)
                    print(f"[PASS] {import_path} [{flavor.name}] ({len(funcs)} tests)")
            else:
                funcs = _test_funcs(test_files)
                sample = ", ".join(sorted(funcs)[:3])
                suffix = ", ..." if len(funcs) > 3 else ""
                print(f"[FAIL] {import_path} [{flavor.name}] -- no Bazel target ({len(funcs)}: {sample}{suffix})")
                for reason in coverage.dd_rejections.get((flavor.name, import_path), []):
                    print(f"       Bazel: {reason}")
                failed = True
        for import_path in sorted(bazel_pkgs):
            print(f"[FAIL] {import_path} [{flavor.name}] -- Bazel target exists but no matching dda inv test package")
            failed = True

    if failed:
        sys.exit(1)
