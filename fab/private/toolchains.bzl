# SPDX-License-Identifier: Apache-2.0
"""Providers and rules for the toolchains of rules_fab.

Every tool an action or a test runs comes from a toolchain (spec section
"Toolchains"). A tool toolchain returns `platform_common.ToolchainInfo`
with a `tools` dict, from tool name to `FilesToRunProvider`; `run_tool`
looks a tool up there by name. The helpers toolchain brings this module's
own Go helpers: the launcher every action runs through, the checker of the
swap test, and the comparer of determinism_test. The stub tool is used directly by stub tool toolchains.
"""

HelpersInfo = provider(
    doc = "The Go helpers that run_tool and swap_test use.",
    fields = {
        "launcher": "FilesToRunProvider: runs a tool with a private home.",
        "swapcheck": "DefaultInfo of the swap test's checker (a target binary).",
        "compare": "DefaultInfo of determinism_test's comparer (a target binary).",
        "pkgcheck": "DefaultInfo of fab_package_test's checker (a target binary).",
    },
)

def _helpers_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo(helpers = HelpersInfo(
        launcher = ctx.attr.launcher[DefaultInfo].files_to_run,
        swapcheck = ctx.attr.swapcheck[DefaultInfo],
        compare = ctx.attr.compare[DefaultInfo],
        pkgcheck = ctx.attr.pkgcheck[DefaultInfo],
    ), tools = {
        # Tools that rules run in actions through run_tool, so built for the
        # execution platform: fab_package's packager (issue #9).
        "packager": ctx.attr.packager[DefaultInfo].files_to_run,
    })]

helpers_toolchain = rule(
    implementation = _helpers_toolchain_impl,
    doc = "The helpers toolchain: the launcher, the swap checker, the determinism comparer, " +
          "fab_package's packager and fab_package_test's checker.",
    attrs = {
        "launcher": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "swapcheck": attr.label(mandatory = True, executable = True, cfg = "target"),
        "compare": attr.label(mandatory = True, executable = True, cfg = "target"),
        "packager": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "pkgcheck": attr.label(mandatory = True, executable = True, cfg = "target"),
    },
)

UNSUPPORTED_PLATFORM_MESSAGE = (
    "rules_fab v1 supports only Linux x86_64 hosts (@platforms//os:linux, " +
    "@platforms//cpu:x86_64). This build's execution platform is another one. " +
    "See https://github.com/filmil/bazel_rules_fab#using-it"
)

def _unsupported_platform_toolchain_impl(_ctx):
    fail(UNSUPPORTED_PLATFORM_MESSAGE)

unsupported_platform_toolchain = rule(
    implementation = _unsupported_platform_toolchain_impl,
    doc = """A toolchain that fails analysis with a clear message (spec P1).

It is registered after the real toolchains, with no constraints, so Bazel
picks it only when no real toolchain matches the execution platform. A
toolchain resolution error alone does not say which platforms work.
""",
)

def _tool_toolchain_impl(ctx):
    tools = {}
    for target, name in ctx.attr.tools.items():
        if name in tools:
            fail("tool name %r appears twice" % name)
        files_to_run = target[DefaultInfo].files_to_run
        if files_to_run == None or files_to_run.executable == None:
            fail("tool %r: %s is not executable" % (name, target.label))
        tools[name] = files_to_run
    return [platform_common.ToolchainInfo(tools = tools)]

tool_toolchain = rule(
    implementation = _tool_toolchain_impl,
    doc = """A toolchain that offers tools by name, for `run_tool`.

Example, a toolchain for another build of the fab packager (the toolchain
type arrives with this module's toolchain issue):

```starlark
tool_toolchain(
    name = "my_packager_impl",
    tools = {"//third_party/packager:packager": "packager"},
)

toolchain(
    name = "my_packager",
    toolchain = ":my_packager_impl",
    toolchain_type = "@rules_fab//fab:toolchain_type",
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
