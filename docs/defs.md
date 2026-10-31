<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of hdlf_rules_fab.

Load every rule from this file:

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "determinism_test", "swap_test", "tool_toolchain")
```

The rules arrive in milestone M3 (see the issues of
https://github.com/filmil/bazel_rules_fab). This file is the only public
entry point; everything under `//fab/private` may change without notice.

<a id="fab_package"></a>

## fab_package

<pre>
load("@hdlf_rules_fab//fab:defs.bzl", "fab_package")

fab_package(<a href="#fab_package-name">name</a>, <a href="#fab_package-board_name">board_name</a>, <a href="#fab_package-bom">bom</a>, <a href="#fab_package-drill">drill</a>, <a href="#fab_package-gerbers">gerbers</a>, <a href="#fab_package-pos">pos</a>, <a href="#fab_package-rotations">rotations</a>, <a href="#fab_package-vendor">vendor</a>)
</pre>

A fab house's ZIP for a board: `<name>.zip`.

```starlark
fab_package(
    name = "jlcpcb",
    bom = ":bom",          # kicad_bom
    drill = ":drill",      # kicad_drill
    gerbers = ":gerbers",  # kicad_gerbers
    pos = ":pos",          # kicad_pos
    vendor = "jlcpcb",
)
```

The ZIP holds, at its root, for every vendor: the Gerber layers and drill
files, named as KiCad names them (Protel extensions such as `.gtl` and
`.gbl`). KiCad's job file, reports and drill maps (`.gbrjob`, `.rpt`,
`.pdf`, `.svg`) are left out. It also holds a BOM and a placement list,
which differ by vendor. In both, footprints are without their library,
parts marked DNP are left out of the BOM, `X` and `Y` read like
`115.0000mm`, and rotations are degrees, at least 0 and below 360, with
no unit.

`jlcpcb`:

* `<board>_bom.csv`: `Comment` (the value), `Designator`, `Footprint`,
  `LCSC Part #` (the schematic field `LCSC`, if the BOM has that column);
* `<board>_cpl.csv`: `Designator`, `Mid X`, `Mid Y`, `Layer` (`Top`,
  `Bottom`), `Rotation`.

`pcbway`, after PCBWay's sample files:

* `<board>_bom.csv`: `Item #`, `Designator`, `Qty`, `Manufacturer` and
  `Mfg Part #` (the schematic fields `Manufacturer` and `MPN`, if the BOM
  has those columns), `Description / Value`, `Package/Footprint`, and
  `Type` and `Your Instructions / Notes`, left empty;
* `<board>_centroid.csv`: `Designator`, `Footprint`, `Mid X`, `Mid Y`,
  `TB` (`T`, `B`), `Rotation`, `Comment` (the value). PCBWay's sample
  also has `Ref X/Y` and `Pad X/Y`; kicad_pos does not give them.

`<board>` is `board_name`, or the last part of the target's name; a bare
board has no CSV files for it to name.

A bare board, one with no parts to place, leaves out `bom` and `pos`
(both, or neither): its ZIP holds only the Gerber and drill files, which
is what both vendors take for a board without assembly. Check it with
`fab_package_test(bare = True)`.

Its entries are sorted by name, dated 1980-01-01 and mode 0644, so the
same board gives the same bytes.

A vendor's rotations are its own models', which are not always KiCad's,
so a board can come back with parts turned. `rotations` corrects them: each key
is a regular expression that must match a whole footprint name (the
position file's `Package`), each value the degrees to add. The rules are
tried in the order written, and the first match counts. It is empty by
default: hdlf_rules_fab ships no table yet.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="fab_package-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="fab_package-board_name"></a>board_name |  The board's name in the CSV files' names; the target's name if empty. A bare board's ZIP has no CSV files, so it has no effect there.   | String | optional |  `""`  |
| <a id="fab_package-bom"></a>bom |  The bill of materials (`kicad_bom`): a CSV with the columns Refs, Value, Footprint, and DNP and LCSC when present. Leave it and `pos` out for a bare board.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="fab_package-drill"></a>drill |  The drill files (`kicad_drill`).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="fab_package-gerbers"></a>gerbers |  The Gerber layers (`kicad_gerbers`).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="fab_package-pos"></a>pos |  The positions (`kicad_pos`): a CSV with the columns Ref, Val, Package, PosX, PosY, Rot and Side, in mm. Leave it and `bom` out for a bare board.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="fab_package-rotations"></a>rotations |  Rotation corrections: footprint regular expression to degrees. Empty by default.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: String -> String</a> | optional |  `{}`  |
| <a id="fab_package-vendor"></a>vendor |  The fab house: jlcpcb, pcbway.   | String | required |  |


<a id="fab_package_test"></a>

## fab_package_test

<pre>
load("@hdlf_rules_fab//fab:defs.bzl", "fab_package_test")

fab_package_test(<a href="#fab_package_test-name">name</a>, <a href="#fab_package_test-bare">bare</a>, <a href="#fab_package_test-copper_layers">copper_layers</a>, <a href="#fab_package_test-package">package</a>, <a href="#fab_package_test-vendor">vendor</a>, <a href="#fab_package_test-want_error">want_error</a>)
</pre>

Checks a fab house's ZIP against what the vendor takes.

```starlark
fab_package_test(
    name = "jlcpcb_test",
    size = "small",
    package = ":jlcpcb",   # fab_package
    vendor = "jlcpcb",
)
```

The checker has its own table of each vendor's files and columns, written
from the vendor's documents, so a bug in fab_package fails it. It checks:

* the layers: top and bottom copper, solder mask and silkscreen, the board
  outline and a drill file; paste and inner copper layers are allowed;
  nothing else, and nothing empty or below the ZIP's root;
* one BOM and one placement list, named for the same board, with the
  vendor's columns in the vendor's order, and at least one row each; or,
  with `bare = True` (a `fab_package` without `bom` and `pos`), neither;
* every row has a designator; in the placement list, X and Y are in mm,
  the rotation is at least 0 and below 360, and the layer is one the
  vendor names;
* with `copper_layers`, one Gerber of each inner copper layer the board
  has, from `In1.Cu` to `In<copper_layers - 2>.Cu`, and none it does not:
  a board of 2 layers has none.

It does not compare the BOM with the placement list: DNP parts are in the
one and not the other, and `kicad_pos(smd_only = True)` leaves out
through-hole parts.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="fab_package_test-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="fab_package_test-bare"></a>bare |  The ZIP is a bare board's: it must have no BOM and no placement list. False by default: it must have one of each, so a package that lost them fails.   | Boolean | optional |  `False`  |
| <a id="fab_package_test-copper_layers"></a>copper_layers |  The board's number of copper layers, such as 6: the ZIP must then hold exactly its inner layers' Gerbers, `In1.Cu` to `In<copper_layers - 2>.Cu` (none for 2). 0, the default, does not check them.   | Integer | optional |  `0`  |
| <a id="fab_package_test-package"></a>package |  The ZIP (`fab_package`).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="fab_package_test-vendor"></a>vendor |  The fab house: jlcpcb, pcbway.   | String | required |  |
| <a id="fab_package_test-want_error"></a>want_error |  For a known-bad ZIP: text that one of the problems must contain. The test then passes only if the check finds it. Empty by default: the ZIP must be good.   | String | optional |  `""`  |


<a id="fab_rules"></a>

## fab_rules

<pre>
load("@hdlf_rules_fab//fab:defs.bzl", "fab_rules")

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
then checks it against them; hdlf_rules_kicad's canary makes sure KiCad read
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
load("@hdlf_rules_fab//fab:defs.bzl", "tool_toolchain")

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
    toolchain_type = "@hdlf_rules_fab//fab:toolchain_type",
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
load("@hdlf_rules_fab//fab:defs.bzl", "determinism_test")

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


<a id="fab_assembly_swatch"></a>

## fab_assembly_swatch

<pre>
load("@hdlf_rules_fab//fab:defs.bzl", "fab_assembly_swatch")

fab_assembly_swatch(<a href="#fab_assembly_swatch-name">name</a>, <a href="#fab_assembly_swatch-vendor">vendor</a>, <a href="#fab_assembly_swatch-start">start</a>, <a href="#fab_assembly_swatch-kwargs">**kwargs</a>)
</pre>

An assembly swatch: a ladder of real parts, each between two test points.

```starlark
fab_assembly_swatch(
    name = "jlcpcb_assembly",
    vendor = "jlcpcb",
)

kicad_erc_test(
    name = "jlcpcb_assembly_erc_test",
    schematic = ":jlcpcb_assembly",
)
```

The rungs, smallest first: 1 kOhm 1 % resistors in 0201, 0402, 0603,
0805 and 1206, then SOT-23, SOIC-8, TSSOP-16 (0.65 mm), QFN-16
(0.5 mm) and LQFP-48 (0.5 mm), each a real catalog part with its LCSC
number, MPN, manufacturer and datasheet (`ASSEMBLY_PARTS`). The
vendor's assembly limits (`ASSEMBLY_VENDORS`, each with its source)
leave out what it cannot place: JLCPCB's Economic PCBA starts at 0402.

Each part's two measured pins go to two test points, so a meter shows
whether it was soldered: 1.00 kOhm on a resistor, a diode drop of about
0.6 V on an IC, across a diode its datasheet documents. Its other pins
carry no-connect flags, and a net with a power input pin has a
PWR_FLAG, so ERC passes. The fields carry LCSC, MPN and Manufacturer
for the BOM.

Each `ASSEMBLY_PARTS` entry has `rung` (the package, such as "0402"
or "SOT-23"), `kind` ("passive" or "ic"), `pitch_mm` for an IC,
`symbol` and `footprint` (from @kicad_libs), `value`, `ref` (the
reference prefix), `measure` (the two pins the test points go to),
`reading` (what a meter shows across them), `silk` (what the board
prints by the test pads), and `fields` (Datasheet, LCSC, MPN,
Manufacturer, for the schematic and the BOM). Each `ASSEMBLY_VENDORS` entry has `name`,
`tier` (the assembly service), `smallest_passive`, `min_pitch_mm`, and
`sources` (the URLs the limits come from).

`name` is a `kicad_schematic`, for `kicad_erc_test` and `kicad_bom`.
The board and its package follow in issue #32 (b).


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="fab_assembly_swatch-name"></a>name |  the schematic's name.   |  none |
| <a id="fab_assembly_swatch-vendor"></a>vendor |  the fab house: jlcpcb or pcbway.   |  none |
| <a id="fab_assembly_swatch-start"></a>start |  the first rung (such as "0402"); by default the smallest the vendor places. A rung the vendor cannot place fails: a known-bad swatch.   |  `""` |
| <a id="fab_assembly_swatch-kwargs"></a>kwargs |  common attributes, such as `visibility` and `tags`.   |  none |


<a id="fab_swatch"></a>

## fab_swatch

<pre>
load("@hdlf_rules_fab//fab:defs.bzl", "fab_swatch")

fab_swatch(<a href="#fab_swatch-name">name</a>, <a href="#fab_swatch-vendor">vendor</a>, <a href="#fab_swatch-stackup">stackup</a>, <a href="#fab_swatch-below">below</a>, <a href="#fab_swatch-kwargs">**kwargs</a>)
</pre>

A capability swatch: a credit-card board of a fab house's smallest features.

```starlark
fab_swatch(
    name = "jlcpcb_swatch",
    stackup = "2L-1oz",
    vendor = "jlcpcb",
)

kicad_drc_test(
    name = "jlcpcb_swatch_drc_test",
    pcb = ":jlcpcb_swatch",
)
```

The board is 85.60 x 53.98 mm with corners of 3.18 mm (ISO/IEC 7810
ID-1), with the stackup's copper layers: a 4L or 6L board has In1.Cu
to In2.Cu or In4.Cu, and ladders of the inner track width and spacing
on In1.Cu, labelled on the silkscreen above them (issue #35); the
vendors' inner rules can differ from the outer ones. Vias are through
vias: no vendor's rules list blind, buried or plugged ones. Its features start at the vendor's minimum and
go up, each labelled on the silkscreen with its size in mm: track
widths, track spacings, vias (drill and pad), plated holes (drill and
pad), non-plated holes, gaps between SMD pads (the solder mask web),
silkscreen text heights and line widths, and a track at the least
distance from the board edge. Ordering one shows what the vendor
makes.

The numbers come from the vendor's design rules (`fab_rules` of the
same vendor and stackup), so the board passes `kicad_drc_test` with
them. `name` is a `kicad_pcb` with those rules, ready for
`kicad_drc_test`, `kicad_gerbers`, `kicad_drill`, `kicad_render` and
`fab_package`. `<name>_rules` is the `fab_rules`; `<name>_files`, the
generated board, project and netlist, is private. The swatch has no
parts, so it follows an empty netlist, and its project turns off the
checks of dangling tracks and vias and of footprints that are in no
library: a swatch connects nothing, and its footprints are drawn by
the generator.

`below` draws one feature at 80 % of the vendor's minimum: a known-bad
board (spec T3) that fails DRC on that feature's rule and no other.
`SWATCH_BELOW` maps each feature to the violation type it gives. For
`via_annular`, the thin ring goes on the smallest via whose hole the
vendor's extra-cost rule (a bigger pad for a small hole) does not
cover, so that rule does not fire too.

The generator checks that each exposed pad keeps the vendor's
silkscreen clearance from its labels, and fails if a vendor's numbers
make it too big for its row.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="fab_swatch-name"></a>name |  the board's name.   |  none |
| <a id="fab_swatch-vendor"></a>vendor |  the fab house: generic, jlcpcb or pcbway.   |  none |
| <a id="fab_swatch-stackup"></a>stackup |  layers and outer copper weight: 2L-1oz, 4L-1oz, 4L-2oz or 6L-1oz. It picks the rules and the board's copper layers.   |  `"4L-1oz"` |
| <a id="fab_swatch-below"></a>below |  empty, or a feature to draw under its minimum: edge, inner_track_spacing or inner_track_width (4 layers or more), text_height, track_spacing, track_width, via_annular or via_hole.   |  `""` |
| <a id="fab_swatch-kwargs"></a>kwargs |  common attributes, such as `visibility` and `tags`.   |  none |


<a id="swap_test"></a>

## swap_test

<pre>
load("@hdlf_rules_fab//fab:defs.bzl", "swap_test")

swap_test(<a href="#swap_test-name">name</a>, <a href="#swap_test-targets">targets</a>, <a href="#swap_test-kwargs">**kwargs</a>)
</pre>

Builds targets with every tool swapped for a stub, and checks them.

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


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="swap_test-name"></a>name |  the test's name.   |  none |
| <a id="swap_test-targets"></a>targets |  targets whose outputs must all come from the stub.   |  none |
| <a id="swap_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


