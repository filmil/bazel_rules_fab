# SPDX-License-Identifier: Apache-2.0
"""Toy rules for the swap test template."""

load("//fab/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

ECHO_TOOLCHAIN_TYPE = Label("//fab/private/swapdemo:echo_toolchain_type")

def _echo_rule_impl(ctx):
    if ctx.attr.directory:
        out = ctx.actions.declare_directory(ctx.label.name)
    else:
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
    doc = "Runs the echo tool. With directory = True the output is a tree artifact.",
    attrs = {"directory": attr.bool(default = False)},
    toolchains = [ECHO_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)

def _no_tool_rule_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    ctx.actions.write(out, "written without a toolchain tool\n")
    return [DefaultInfo(files = depset([out]))]

no_tool_rule = rule(
    implementation = _no_tool_rule_impl,
    doc = "Writes an output without any tool: swap_test's known-bad input.",
)
