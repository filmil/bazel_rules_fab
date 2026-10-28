# SPDX-License-Identifier: Apache-2.0
"""A rule that runs the clock tool: a known-bad, non-deterministic input."""

load("//fab/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

CLOCK_TOOLCHAIN_TYPE = Label("//fab/private/detdemo:clock_toolchain_type")

def _clock_rule_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    run_tool(ctx, CLOCK_TOOLCHAIN_TYPE, "clock", arguments = [out.path], outputs = [out], mnemonic = "Clock")
    return [DefaultInfo(files = depset([out]))]

clock_rule = rule(
    implementation = _clock_rule_impl,
    toolchains = [CLOCK_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
