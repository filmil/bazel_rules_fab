# SPDX-License-Identifier: Apache-2.0
"""A toy rule for the swap test template: runs the "echo" tool."""

load("//fab/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

ECHO_TOOLCHAIN_TYPE = Label("//fab/private/swapdemo:echo_toolchain_type")

def _echo_rule_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    run_tool(
        ctx,
        ECHO_TOOLCHAIN_TYPE,
        "echo",
        arguments = [out.path],
        outputs = [out],
        mnemonic = "Echo",
    )
    return [DefaultInfo(files = depset([out]))]

echo_rule = rule(
    implementation = _echo_rule_impl,
    toolchains = [ECHO_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
