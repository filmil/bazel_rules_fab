# SPDX-License-Identifier: Apache-2.0
"""fab_package: the ZIP a fab house takes for a board (issue #9)."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

# The vendors fab_package knows; the packager has a layout for each.
FAB_PACKAGE_VENDORS = ["jlcpcb", "pcbway"]

def _single_csv(ctx, attr):
    files = getattr(ctx.files, attr)
    if len(files) != 1:
        fail("%s: %s must name one CSV file, not %d" % (ctx.label, attr, len(files)))
    return files[0]

def _fab_package_impl(ctx):
    # A bare board has no parts: no BOM and no positions.
    bare = not ctx.attr.bom and not ctx.attr.pos
    if not bare and not (ctx.attr.bom and ctx.attr.pos):
        fail("%s: bom goes with pos: give both, or neither for a bare board" % ctx.label)
    if bare and ctx.attr.rotations:
        fail("%s: rotations correct the placement list; a bare board has none" % ctx.label)
    bom = None if bare else _single_csv(ctx, "bom")
    pos = None if bare else _single_csv(ctx, "pos")
    for attr in ["gerbers", "drill"]:
        if not getattr(ctx.files, attr):
            fail("%s: %s gives no files" % (ctx.label, attr))
    fab = ctx.files.gerbers + ctx.files.drill
    board = ctx.attr.board_name or ctx.label.name.split("/")[-1]
    if "/" in board or "\\" in board:
        fail("%s: board_name %r must be a plain file name" % (ctx.label, board))
    out = ctx.actions.declare_file(ctx.label.name + ".zip")
    args = ctx.actions.args()
    args.add("--vendor", ctx.attr.vendor)
    args.add("--board", board)
    if not bare:
        args.add("--bom", bom)
        args.add("--pos", pos)
    args.add("--out", out)
    for f in fab:
        args.add("--fab", f.path)

    # In the order written: the first match counts.
    for pattern, degrees in ctx.attr.rotations.items():
        args.add("--rotation", "%s=%s" % (pattern, degrees))
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "packager",
        arguments = [args],
        inputs = fab + ([] if bare else [bom, pos]),
        outputs = [out],
        mnemonic = "FabPackage",
        progress_message = "Packaging %{label} for " + ctx.attr.vendor,
    )
    return [DefaultInfo(files = depset([out]))]

fab_package = rule(
    implementation = _fab_package_impl,
    doc = """A fab house's ZIP for a board: `<name>.zip`.

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
""",
    attrs = {
        "vendor": attr.string(
            doc = "The fab house: " + ", ".join(FAB_PACKAGE_VENDORS) + ".",
            mandatory = True,
            values = FAB_PACKAGE_VENDORS,
        ),
        "gerbers": attr.label(
            doc = "The Gerber layers (`kicad_gerbers`).",
            mandatory = True,
            allow_files = True,
        ),
        "drill": attr.label(
            doc = "The drill files (`kicad_drill`).",
            mandatory = True,
            allow_files = True,
        ),
        "bom": attr.label(
            doc = "The bill of materials (`kicad_bom`): a CSV with the columns Refs, Value, " +
                  "Footprint, and DNP and LCSC when present. Leave it and `pos` out for a " +
                  "bare board.",
            allow_files = [".csv"],
        ),
        "pos": attr.label(
            doc = "The positions (`kicad_pos`): a CSV with the columns Ref, Val, Package, " +
                  "PosX, PosY, Rot and Side, in mm. Leave it and `bom` out for a bare board.",
            allow_files = [".csv"],
        ),
        "board_name": attr.string(
            doc = "The board's name in the CSV files' names; the target's name if empty. " +
                  "A bare board's ZIP has no CSV files, so it has no effect there.",
        ),
        "rotations": attr.string_dict(
            doc = "Rotation corrections: footprint regular expression to degrees. Empty by " +
                  "default.",
        ),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)
