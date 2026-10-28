<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of rules_fab.

Load every rule from this file:

```starlark
load("@rules_fab//fab:defs.bzl", "swap_test", "tool_toolchain")
```

The rules arrive in milestone M3 (see the issues of
https://github.com/filmil/bazel_rules_fab). This file is the only public
entry point; everything under `//fab/private` may change without notice.

<a id="swap_test"></a>

## swap_test

<pre>
load("@rules_fab//fab:defs.bzl", "swap_test")

swap_test(<a href="#swap_test-name">name</a>, <a href="#swap_test-targets">targets</a>)
</pre>

Builds targets with every tool swapped for a stub, and checks them.

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

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="swap_test-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="swap_test-targets"></a>targets |  Targets whose outputs must all come from the stub.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | required |  |


<a id="tool_toolchain"></a>

## tool_toolchain

<pre>
load("@rules_fab//fab:defs.bzl", "tool_toolchain")

tool_toolchain(<a href="#tool_toolchain-name">name</a>, <a href="#tool_toolchain-tools">tools</a>)
</pre>

A toolchain that offers tools by name, for `run_tool`.

Example, a stub toolchain for the swap test:

```starlark
tool_toolchain(
    name = "stub_impl",
    tools = {"//fab/private/stub": "packager"},
)
```

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="tool_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="tool_toolchain-tools"></a>tools |  Executable targets, each mapped to the name rules use for it.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: Label -> String</a> | required |  |


