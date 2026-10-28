# SPDX-License-Identifier: Apache-2.0
"""swap_test: proves that targets use only tools from toolchains."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")

_USE_STUB = str(Label("//fab/toolchains:use_stub"))

def _stub_transition_impl(_settings, _attr):
    return {_USE_STUB: True}

_stub_transition = transition(
    implementation = _stub_transition_impl,
    inputs = [],
    outputs = [_USE_STUB],
)

def _swap_test_impl(ctx):
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    files = depset(transitive = [t[DefaultInfo].files for t in ctx.attr.targets]).to_list()
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = helpers.swapcheck.files_to_run.executable,
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = files).merge(helpers.swapcheck.default_runfiles)
    return [
        DefaultInfo(executable = exe, runfiles = runfiles),
        RunEnvironmentInfo(environment = {
            "SWAP_FILES": " ".join([f.short_path for f in files]),
        }),
    ]

swap_test = rule(
    implementation = _swap_test_impl,
    test = True,
    doc = """Builds targets with every tool swapped for a stub, and checks them.

The targets are built with `--@rules_fab//fab/toolchains:use_stub`, so
each tool toolchain that honors the flag resolves to the stub tool. The stub
writes a record into every output. The test fails if any output file of the
targets lacks the record: some action made it without a toolchain tool.

Example:

```starlark
swap_test(
    name = "swap_test",
    targets = [":gerbers", ":bom"],
)
```
""",
    attrs = {
        "targets": attr.label_list(
            doc = "Targets whose outputs must all come from the stub.",
            cfg = _stub_transition,
            mandatory = True,
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)
