# SPDX-License-Identifier: Apache-2.0
"""The one place where rules_fab starts an action that runs a tool.

Every rule runs its tools through `run_tool`; the hermeticity lint fails on
`ctx.actions.run` or `ctx.actions.run_shell` anywhere else (spec section
"Toolchains", requirement 3).
"""

HELPERS_TOOLCHAIN_TYPE = Label("//fab:helpers_toolchain_type")

# A rule that calls run_tool lists these in its `toolchains`, together with
# the toolchain types of the tools it runs.
RUN_TOOL_TOOLCHAINS = [HELPERS_TOOLCHAIN_TYPE]

def run_tool(
        ctx,
        toolchain_type,
        tool,
        arguments = [],
        inputs = [],
        outputs = [],
        mnemonic = None,
        progress_message = None,
        env = {},
        home_env = [],
        tools = []):
    """Runs a tool from a toolchain, with a private home, as one action.

    The tool comes from `ctx.toolchains[toolchain_type].tools[tool]`. It runs
    under the launcher, which points HOME, TMPDIR and the XDG directories,
    and each variable named in `home_env`, at a fresh directory inside the
    action, so the tool reads and writes no host configuration (spec H3).

    Args:
      ctx: the rule context. The rule's `toolchains` must include
        `toolchain_type` and `RUN_TOOL_TOOLCHAINS`.
      toolchain_type: the toolchain type that provides the tool.
      tool: the tool's name in that toolchain's `tools` dict.
      arguments: the tool's arguments: strings or `Args` objects.
      inputs: the action's inputs.
      outputs: the action's outputs. The launcher also passes them to the
        tool in RUN_TOOL_OUTPUTS, which the swap test's stub uses.
      mnemonic: the action's mnemonic.
      progress_message: the action's progress message.
      env: fixed environment values for the tool.
      home_env: names of variables to point at directories of their own in
        the private home, such as `KICAD_CONFIG_HOME`.
      tools: more executables the action needs, from toolchains.
    """
    info = ctx.toolchains[toolchain_type]
    if info == None:
        fail("no toolchain for %s; add it to the rule's toolchains" % toolchain_type)
    if tool not in info.tools:
        fail("toolchain for %s has no tool %r; it has %s" % (toolchain_type, tool, sorted(info.tools.keys())))
    exe = info.tools[tool]
    launcher = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers.launcher

    pre = ctx.actions.args()
    for name in home_env:
        pre.add("--home-env", name)
    for k in sorted(env.keys()):
        pre.add("--env", "%s=%s" % (k, env[k]))
    for out in outputs:
        # The path, so a directory output (a tree artifact) is one value.
        pre.add("--output", out.path)
    pre.add("--")
    pre.add(exe.executable)

    ctx.actions.run(
        executable = launcher,
        arguments = [pre] + list(arguments),
        inputs = inputs,
        outputs = outputs,
        tools = [exe] + list(tools),
        mnemonic = mnemonic,
        progress_message = progress_message,
    )
