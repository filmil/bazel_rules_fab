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
            "SWAP_EXPECT": ctx.attr.expect,
            # One path per line: a path may hold spaces.
            "SWAP_FILES": "\n".join([f.short_path for f in files]),
        }),
    ]

raw_swap_test = rule(
    implementation = _swap_test_impl,
    test = True,
    doc = "The rule behind swap_test. Use the macro.",
    attrs = {
        "targets": attr.label_list(
            doc = "Targets whose outputs must all come from the stub.",
            cfg = _stub_transition,
            mandatory = True,
        ),
        "expect": attr.string(
            doc = "\"stub\", or \"foreign\" for this rule's own known-bad test.",
            default = "stub",
            values = ["stub", "foreign"],
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)

def swap_test(name, targets, **kwargs):
    """Builds targets with every tool swapped for a stub, and checks them.

    The targets are built with `--@hdlf_rules_fab//fab/toolchains:use_stub`,
    so each tool toolchain that honors the flag resolves to the stub tool.
    The stub writes a record into every output, files and directories
    alike. The test fails if any output file of the targets lacks the
    record: some action made it without a toolchain tool.

    Example:

    ```starlark
    swap_test(
        name = "swap_test",
        targets = [":gerbers", ":bom"],
    )
    ```

    Args:
      name: the test's name.
      targets: targets whose outputs must all come from the stub.
      **kwargs: common test attributes, such as `size` and `tags`.
    """
    raw_swap_test(name = name, targets = targets, **kwargs)
