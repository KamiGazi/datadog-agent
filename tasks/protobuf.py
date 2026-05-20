import os
import re

from invoke import Exit, task

from tasks.libs.build.bazel import bazel
from tasks.libs.common.color import Color, color_message
from tasks.libs.common.git import get_unstaged_files, get_untracked_files


@task
def generate(ctx, pre_commit=False):
    """
    Generates protobuf definitions in pkg/proto

    We must build the packages one at a time due to protoc-gen-go limitations
    """
    proto_file = re.compile(r"pkg/proto/pbgo/.*\.pb\.go$")
    old_unstaged_proto_files = set(get_unstaged_files(ctx, re_filter=proto_file, include_deleted_files=True))
    old_untracked_proto_files = set(get_untracked_files(ctx, re_filter=proto_file))
    base = os.path.dirname(os.path.abspath(__file__))
    proto_root = os.path.join(os.path.abspath(os.path.join(base, "..")), "pkg", "proto")

    print(f"generating protobuf code from: {proto_root}")
    bazel(ctx, "run", "//pkg/proto/pbgo/core:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/dogstatsdhttp:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/languagedetection:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/mocks/core:api_mockgen")
    bazel(ctx, "run", "//pkg/proto/pbgo/privateactionrunner/actionsclient:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/privateactionrunner/errorcode:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/privateactionrunner/privateactions:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/process:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/sbom:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/trace:write_pb_go")
    bazel(ctx, "run", "//pkg/proto/pbgo/trace/idx:write_pb_go")

    # Generate messagepack marshallers
    bazel(ctx, "run", "//pkg/proto/pbgo/core:remoteconfig_gen")
    bazel(ctx, "run", "//pkg/proto/pbgo/trace:agent_payload_gen")
    bazel(ctx, "run", "//pkg/proto/pbgo/trace:span_gen")
    bazel(ctx, "run", "//pkg/proto/pbgo/trace:stats_gen")
    bazel(ctx, "run", "//pkg/proto/pbgo/trace:trace_gen")
    bazel(ctx, "run", "//pkg/proto/pbgo/trace:tracer_payload_gen")

    # Check the generated files were properly committed
    current_unstaged_proto_files = set(get_unstaged_files(ctx, re_filter=proto_file, include_deleted_files=True))
    current_untracked_proto_files = set(get_untracked_files(ctx, re_filter=proto_file))
    if (
        old_unstaged_proto_files != current_unstaged_proto_files
        or old_untracked_proto_files != current_untracked_proto_files
    ):
        if pre_commit:
            updated_files = [f"- {file}\n" for file in current_unstaged_proto_files - old_unstaged_proto_files]
            updated_files += [f"- {file}\n" for file in current_untracked_proto_files - old_untracked_proto_files]
            raise Exit(f"Files modified\n{''.join(updated_files)}", code=1)
        else:
            print("Generation complete and new files were updated, don't forget to commit and push")
    else:
        print(f"[{color_message('WARN', Color.ORANGE)}] Generation complete and no new files were updated")
