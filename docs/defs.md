<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of rules_fab.

Load every rule from this file:

```starlark
load("@rules_fab//fab:defs.bzl", "determinism_test", "swap_test", "tool_toolchain")
```

The rules arrive in milestone M3 (see the issues of
https://github.com/filmil/bazel_rules_fab). This file is the only public
entry point; everything under `//fab/private` may change without notice.

<a id="fab_rules"></a>

## fab_rules

<pre>
load("@rules_fab//fab:defs.bzl", "fab_rules")

fab_rules(<a href="#fab_rules-name">name</a>, <a href="#fab_rules-rule_set">rule_set</a>, <a href="#fab_rules-stackup">stackup</a>, <a href="#fab_rules-vendor">vendor</a>)
</pre>

A fab house's design rules, as `<name>.kicad_dru`.

```starlark
fab_rules(
    name = "jlcpcb_rules",
    stackup = "2L-1oz",
    vendor = "jlcpcb",
)

kicad_pcb(
    name = "pcb",
    board = "main.kicad_pcb",
    design_rules = ":jlcpcb_rules",
    schematic = ":sch",
)
```

The rules come from kicad-druid 1.2.0 (MIT), pinned by its archive's
sha256: they follow the vendor's published capabilities for drill sizes,
annular rings, track widths and clearances. `kicad_drc_test` on the board
then checks it against them; rules_kicad's canary makes sure KiCad read
the file.

AGENTS.md, rule 7: changing which rules a board uses, or the rule files,
needs a person.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="fab_rules-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="fab_rules-rule_set"></a>rule_set |  The rule files to pick from, named as kicad-druid names them (`JLCPCB.kicad_dru`, `JLCPCB-2L-1oz.kicad_dru`, ...). The default is the pinned kicad-druid 1.2.0; another release of it can stand in.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `"@kicad_druid//:rules"`  |
| <a id="fab_rules-stackup"></a>stackup |  Layers and outer copper weight: 2L-1oz, 4L-1oz, 4L-2oz, 6L-1oz.   | String | optional |  `"4L-1oz"`  |
| <a id="fab_rules-vendor"></a>vendor |  The fab house: generic, jlcpcb, pcbway.   | String | required |  |


<a id="tool_toolchain"></a>

## tool_toolchain

<pre>
load("@rules_fab//fab:defs.bzl", "tool_toolchain")

tool_toolchain(<a href="#tool_toolchain-name">name</a>, <a href="#tool_toolchain-tools">tools</a>)
</pre>

A toolchain that offers tools by name, for `run_tool`.

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

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="tool_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="tool_toolchain-tools"></a>tools |  Executable targets, each mapped to the name rules use for it.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: Label -> String</a> | required |  |


<a id="determinism_test"></a>

## determinism_test

<pre>
load("@rules_fab//fab:defs.bzl", "determinism_test")

determinism_test(<a href="#determinism_test-name">name</a>, <a href="#determinism_test-target">target</a>, <a href="#determinism_test-kwargs">**kwargs</a>)
</pre>

Checks that two builds of `target` give byte-identical outputs.

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


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="determinism_test-name"></a>name |  the test's name.   |  none |
| <a id="determinism_test-target"></a>target |  the target whose outputs must not change between builds.   |  none |
| <a id="determinism_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


<a id="swap_test"></a>

## swap_test

<pre>
load("@rules_fab//fab:defs.bzl", "swap_test")

swap_test(<a href="#swap_test-name">name</a>, <a href="#swap_test-targets">targets</a>, <a href="#swap_test-kwargs">**kwargs</a>)
</pre>

Builds targets with every tool swapped for a stub, and checks them.

The targets are built with `--@rules_fab//fab/toolchains:use_stub`,
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


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="swap_test-name"></a>name |  the test's name.   |  none |
| <a id="swap_test-targets"></a>targets |  targets whose outputs must all come from the stub.   |  none |
| <a id="swap_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


