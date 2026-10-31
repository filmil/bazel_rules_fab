# SPDX-License-Identifier: Apache-2.0
"""fab_swatch: a credit-card board of a fab house's smallest features (issue #24)."""

load("@rules_kicad//kicad:defs.bzl", "kicad_pcb")
load(":fab_rules.bzl", "FAB_STACKUPS", "FAB_VENDORS", "fab_rules")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

# The features that `below` can draw under the vendor's minimum, and the
# DRC violation type each then gives (the generator's swatch.Features).
SWATCH_BELOW = {
    "edge": "copper_edge_clearance",
    "text_height": "text_height",
    "track_spacing": "clearance",
    "track_width": "track_width",
    "via_annular": "annular_width",
    "via_hole": "drill_out_of_range",
}

def _swatch_files_impl(ctx):
    args = ctx.actions.args()
    args.add("--rules", ctx.file.rules)
    args.add("--stackup", ctx.attr.stackup)
    args.add("--pcb", ctx.outputs.board)
    args.add("--pro", ctx.outputs.project)
    args.add("--net", ctx.outputs.netlist)
    if ctx.attr.below:
        args.add("--below", ctx.attr.below)
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "swatch",
        arguments = [args],
        inputs = [ctx.file.rules],
        outputs = [ctx.outputs.board, ctx.outputs.project, ctx.outputs.netlist],
        mnemonic = "FabSwatch",
        progress_message = "Drawing the capability swatch %{label}",
    )
    return [DefaultInfo(files = depset([ctx.outputs.board, ctx.outputs.project, ctx.outputs.netlist]))]

swatch_files = rule(
    implementation = _swatch_files_impl,
    doc = "The rule behind fab_swatch: the board, its project and an empty netlist. Use the macro.",
    attrs = {
        "rules": attr.label(
            doc = "The vendor's design rules (`fab_rules`).",
            allow_single_file = [".kicad_dru"],
            mandatory = True,
        ),
        "stackup": attr.string(doc = "The stackup, for the board's title."),
        "below": attr.string(
            doc = "A feature to draw under its minimum, for a known-bad board.",
            values = [""] + sorted(SWATCH_BELOW.keys()),
        ),
        "board": attr.output(mandatory = True),
        "project": attr.output(mandatory = True),
        "netlist": attr.output(mandatory = True),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)

def fab_swatch(name, vendor, stackup = "4L-1oz", below = "", **kwargs):
    """A capability swatch: a credit-card board of a fab house's smallest features.

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

    The board is two-layer, 85.60 x 53.98 mm with corners of 3.18 mm
    (ISO/IEC 7810 ID-1). Its features start at the vendor's minimum and
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

    Args:
      name: the board's name.
      vendor: the fab house: generic, jlcpcb or pcbway.
      stackup: layers and outer copper weight: 2L-1oz, 4L-1oz, 4L-2oz or
        6L-1oz. The board has two layers; the stackup picks the rules.
      below: empty, or a feature to draw under its minimum: edge,
        text_height, track_spacing, track_width, via_annular or via_hole.
      **kwargs: common attributes, such as `visibility` and `tags`.
    """
    if vendor not in FAB_VENDORS:
        fail("fab_swatch %s: vendor %r: want one of %s" % (name, vendor, sorted(FAB_VENDORS.keys())))
    if stackup not in FAB_STACKUPS:
        fail("fab_swatch %s: stackup %r: want one of %s" % (name, stackup, FAB_STACKUPS))

    # The generated files are an implementation detail: private, whatever
    # the board's visibility.
    inner = {k: v for k, v in kwargs.items() if k != "visibility"}
    fab_rules(
        name = name + "_rules",
        stackup = stackup,
        vendor = vendor,
        **kwargs
    )
    swatch_files(
        name = name + "_files",
        below = below,
        board = name + ".kicad_pcb",
        netlist = name + ".net",
        project = name + ".kicad_pro",
        rules = ":" + name + "_rules",
        stackup = stackup,
        visibility = ["//visibility:private"],
        **inner
    )
    kicad_pcb(
        name = name,
        board = ":" + name + ".kicad_pcb",
        design_rules = ":" + name + "_rules",
        netlist = ":" + name + ".net",
        project = ":" + name + ".kicad_pro",
        **kwargs
    )
