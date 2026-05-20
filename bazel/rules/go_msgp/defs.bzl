"""`msgp` marshaller generation with optional in-pipeline patches, diff-tested against the source tree.

Each call runs `msgp` once on `src` to produce both `<out>` and `<stem>_test.go`. When `patches` is non-empty, they are
applied in order to the main output (the test file is left untouched), and the patched main is written back alongside
the raw test file via `write_source_files`.
"""

load("@bazel_lib//lib:write_source_files.bzl", "write_source_files")

def _msgp_run_impl(ctx):
    args = ctx.actions.args()
    args.add("-file", ctx.file.src)
    args.add("-o", ctx.outputs.out)
    args.add("-io={}".format("true" if ctx.attr.io else "false"))
    for d in ctx.attr.directives:
        args.add("-d", d)
    ctx.actions.run(
        executable = ctx.executable._msgp,
        outputs = [ctx.outputs.out, ctx.outputs.out_test],
        inputs = [ctx.file.src],
        arguments = [args],
        mnemonic = "MsgpGen",
        progress_message = "msgp %{input}",
    )

_msgp_run = rule(
    implementation = _msgp_run_impl,
    attrs = {
        "src": attr.label(allow_single_file = True),
        "out": attr.output(),
        "out_test": attr.output(),
        "io": attr.bool(),
        "directives": attr.string_list(),
        "_msgp": attr.label(default = "@com_github_tinylib_msgp//:msgp", executable = True, cfg = "exec"),
    },
)

def _apply_patch_impl(ctx):
    ctx.actions.run(
        executable = "patch",
        arguments = ["-p4", "-o", ctx.outputs.out.path, "-i", ctx.file.patch.path, ctx.file.src.path],
        inputs = [ctx.file.src, ctx.file.patch],
        outputs = [ctx.outputs.out],
        mnemonic = "MsgpPatch",
        use_default_shell_env = True,
        execution_requirements = {"no-sandbox": "1"},
    )

_apply_patch = rule(
    implementation = _apply_patch_impl,
    attrs = {
        "src": attr.label(allow_single_file = True),
        "patch": attr.label(allow_single_file = True),
        "out": attr.output(),
    },
)

def go_msgp(name, src, out, io = False, directives = [], patches = []):
    if not out.endswith(".go"):
        fail("go_msgp: `out` must end in .go, got {}".format(out))
    out_test = "{}_test.go".format(out[:-len(".go")])
    raw_main = "{}_raw/{}".format(name, out)
    raw_test = "{}_raw/{}".format(name, out_test)
    _msgp_run(
        name = "{}_raw".format(name),
        src = src,
        out = raw_main,
        out_test = raw_test,
        io = io,
        directives = directives,
    )
    main = raw_main
    for i, patch in enumerate(patches):
        patched = "{}_p{}/{}".format(name, i, out)
        _apply_patch(
            name = "{}_p{}".format(name, i),
            src = ":{}".format(main),
            patch = patch,
            out = patched,
        )
        main = patched
    write_source_files(
        name = name,
        files = {
            out: ":{}".format(main),
            out_test: ":{}".format(raw_test),
        },
    )
