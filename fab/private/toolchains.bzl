# SPDX-License-Identifier: Apache-2.0
"""Providers and rules for the toolchains of rules_fab.

Every tool an action or a test runs comes from a toolchain (spec section
"Toolchains"). A tool toolchain returns `platform_common.ToolchainInfo`
with a `tools` dict, from tool name to `FilesToRunProvider`; `run_tool`
looks a tool up there by name. The helpers toolchain brings this module's
own Go helpers: the launcher every action runs through, and the stub and
checker of the swap test.
"""

HelpersInfo = provider(
    doc = "The Go helpers that run_tool and swap_test use.",
    fields = {
        "launcher": "FilesToRunProvider: runs a tool with a private home.",
        "stub": "FilesToRunProvider: stands in for any tool in swap tests.",
        "swapcheck": "DefaultInfo of the swap test's checker (a target binary).",
    },
)

def _helpers_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo(helpers = HelpersInfo(
        launcher = ctx.attr.launcher[DefaultInfo].files_to_run,
        stub = ctx.attr.stub[DefaultInfo].files_to_run,
        swapcheck = ctx.attr.swapcheck[DefaultInfo],
    ))]

helpers_toolchain = rule(
    implementation = _helpers_toolchain_impl,
    doc = "The helpers toolchain: launcher, stub and swap checker.",
    attrs = {
        "launcher": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "stub": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "swapcheck": attr.label(mandatory = True, executable = True, cfg = "target"),
    },
)

def _tool_toolchain_impl(ctx):
    tools = {}
    for target, name in ctx.attr.tools.items():
        if name in tools:
            fail("tool name %r appears twice" % name)
        tools[name] = target[DefaultInfo].files_to_run
    return [platform_common.ToolchainInfo(tools = tools)]

tool_toolchain = rule(
    implementation = _tool_toolchain_impl,
    doc = """A toolchain that offers tools by name, for `run_tool`.

Example, a stub toolchain for the swap test:

```starlark
tool_toolchain(
    name = "stub_impl",
    tools = {"//fab/private/stub": "packager"},
)
```
""",
    attrs = {
        "tools": attr.label_keyed_string_dict(
            doc = "Executable targets, each mapped to the name rules use for it.",
            cfg = "exec",
            mandatory = True,
        ),
    },
)
