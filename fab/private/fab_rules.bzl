# SPDX-License-Identifier: Apache-2.0
"""fab_rules: a fab house's design rules for KiCad (issue #8)."""

# kicad-druid's file prefix per vendor.
FAB_VENDORS = {
    "generic": "Generic",
    "jlcpcb": "JLCPCB",
    "pcbway": "PCBWay",
}

# kicad-druid's build variants: layers and outer copper weight. The file
# without a suffix is the 4-layer, 1 oz one.
FAB_STACKUPS = ["2L-1oz", "4L-1oz", "4L-2oz", "6L-1oz"]

def _fab_rules_impl(ctx):
    prefix = FAB_VENDORS[ctx.attr.vendor]
    want = prefix + ("" if ctx.attr.stackup == "4L-1oz" else "-" + ctx.attr.stackup) + ".kicad_dru"
    found = [f for f in ctx.files.rule_set if f.basename == want]
    if len(found) != 1:
        fail("%s: the rule set %s has no %s" % (ctx.label, ctx.attr.rule_set.label, want))
    out = ctx.actions.declare_file(ctx.label.name + ".kicad_dru")
    ctx.actions.symlink(output = out, target_file = found[0])
    return [DefaultInfo(files = depset([out]))]

fab_rules = rule(
    implementation = _fab_rules_impl,
    doc = """A fab house's design rules, as `<name>.kicad_dru`.

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
""",
    attrs = {
        "vendor": attr.string(
            doc = "The fab house: " + ", ".join(sorted(FAB_VENDORS.keys())) + ".",
            mandatory = True,
            values = sorted(FAB_VENDORS.keys()),
        ),
        "stackup": attr.string(
            doc = "Layers and outer copper weight: " + ", ".join(FAB_STACKUPS) + ".",
            default = "4L-1oz",
            values = FAB_STACKUPS,
        ),
        "rule_set": attr.label(
            doc = "The rule files to pick from, named as kicad-druid names them " +
                  "(`JLCPCB.kicad_dru`, `JLCPCB-2L-1oz.kicad_dru`, ...). The default is the " +
                  "pinned kicad-druid 1.2.0; another release of it can stand in.",
            default = Label("@kicad_druid//:rules"),
            allow_files = [".kicad_dru"],
        ),
    },
)
