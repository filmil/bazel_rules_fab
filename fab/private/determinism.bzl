# SPDX-License-Identifier: Apache-2.0
"""determinism_test: two builds of a target must give the same bytes."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")

_SALT = str(Label("//fab/private:determinism_salt"))

def _salt_a_impl(_settings, _attr):
    return {_SALT: "a"}

def _salt_b_impl(_settings, _attr):
    return {_SALT: "b"}

_salt_a = transition(implementation = _salt_a_impl, inputs = [], outputs = [_SALT])
_salt_b = transition(implementation = _salt_b_impl, inputs = [], outputs = [_SALT])

def _one(target):
    # A 1:1 transition gives a Target; older Bazel versions give a list.
    return target[0] if type(target) == "list" else target

def _stage(ctx, target, tag):
    """Links each output of target under <name>.<tag>/ for the runfiles."""
    links = []
    for f in _one(target)[DefaultInfo].files.to_list():
        path = "%s.%s/%s" % (ctx.label.name, tag, f.short_path)
        if f.is_directory:
            link = ctx.actions.declare_directory(path)
        else:
            link = ctx.actions.declare_file(path)
        ctx.actions.symlink(output = link, target_file = f)
        links.append(link)
    return links

def _determinism_test_impl(ctx):
    compare = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers.compare
    a = _stage(ctx, ctx.attr.target_a, "a")
    b = _stage(ctx, ctx.attr.target_b, "b")
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = compare.files_to_run.executable,
        is_executable = True,
    )
    return [
        DefaultInfo(
            executable = exe,
            runfiles = ctx.runfiles(files = a + b).merge(compare.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            # One path per line: a path may hold spaces.
            "DET_A": "\n".join([f.short_path for f in a]),
            "DET_B": "\n".join([f.short_path for f in b]),
            "DET_EXPECT": ctx.attr.expect,
        }),
    ]

raw_determinism_test = rule(
    implementation = _determinism_test_impl,
    test = True,
    doc = "The rule behind determinism_test. Use the macro.",
    attrs = {
        "target_a": attr.label(mandatory = True, cfg = _salt_a),
        "target_b": attr.label(mandatory = True, cfg = _salt_b),
        "expect": attr.string(
            doc = "\"same\", or \"different\" for this rule's own known-bad test.",
            default = "same",
            values = ["same", "different"],
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)

def determinism_test(name, target, **kwargs):
    """Checks that two builds of `target` give byte-identical outputs.

    The target is built twice in one build, under two configurations that
    differ only in a setting no rule reads, so every action runs twice.
    The test compares the SHA-256 of every output file, directories walked,
    and fails on any difference (spec D1).

    Two runs in the same second can hide a time stamp with a one-second
    resolution, so pair this with a header scan where a format has one. CI
    also builds every determinism_test target in two fresh output bases and
    compares the results.

    Example:

    ```starlark
    determinism_test(
        name = "fab_jlc_determinism_test",
        target = ":fab_jlc",
    )
    ```

    Args:
      name: the test's name.
      target: the target whose outputs must not change between builds.
      **kwargs: common test attributes, such as `size` and `tags`.
    """
    raw_determinism_test(
        name = name,
        target_a = target,
        target_b = target,
        **kwargs
    )
