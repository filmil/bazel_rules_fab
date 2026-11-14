# SPDX-License-Identifier: Apache-2.0
"""fab_report: a fab package that people can read (issue #43)."""

load(":package.bzl", "FabPackageInfo")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

def _fab_report_impl(ctx):
    pkg = ctx.attr.package[FabPackageInfo]
    if ctx.attr.px_per_mm < 1 or ctx.attr.px_per_mm > 100:
        fail("%s: px_per_mm %d: want 1 to 100" % (ctx.label, ctx.attr.px_per_mm))
    out = ctx.actions.declare_directory(ctx.label.name)
    args = ctx.actions.args()
    args.add("--zip", pkg.zip)
    args.add("--vendor", pkg.vendor)
    if pkg.bare:
        args.add("--bare")
    args.add("--px_per_mm", str(ctx.attr.px_per_mm))
    args.add("--out", out.path)
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "report",
        arguments = [args],
        inputs = [pkg.zip],
        outputs = [out],
        mnemonic = "FabReport",
        progress_message = "Drawing and reporting %{label}",
    )
    return [DefaultInfo(files = depset([out]))]

fab_report = rule(
    implementation = _fab_report_impl,
    doc = """A fab package's human-readable form: a directory `<name>/`.

```starlark
fab_report(
    name = "jlcpcb_report",
    package = ":jlcpcb",  # fab_package
)
```

Everything in it is drawn from the package's ZIP, so it shows what is
sent to the fab house, not the KiCad source:

* `report.md`: the board's size (the outline's box) and copper layer
  count; the files, what each holds and its size; the drill table
  (diameter, plating, holes and slots per tool); the BOM as the vendor
  gets it, with its line and part counts; the parts placed on each side,
  and the designators that are placed but not in the BOM or the other
  way round; and the checks of `fab_package_test`'s checker on the ZIP,
  with their result. A problem the checker finds is listed, not an error:
  `fab_package_test` is the test that fails on it.
* `top.png` and `bottom.png`: the board as the fab sees it, the bottom
  mirrored, as seen from below. Copper in a solder mask opening is gold,
  copper under the mask light green, the silkscreen white (left out of
  mask openings), a hole dark grey.
* `<file>.png` for each Gerber and drill file, such as
  `main-F_Cu.gtl.png`: the layer as drawn, from the top, its board's area
  grey.

The Gerber reader draws what KiCad writes: apertures C, R, O and P,
aperture macros (primitives 1, 4, 5, 7, 20, 21), lines, multi-quadrant
arcs, regions and dark and clear polarity, in absolute coordinates. It
fails on what it would not draw right: incremental coordinates,
single-quadrant arcs, step and repeat, and aperture transformations. The
drill reader takes decimal coordinates and G85 slots, with the plating
from KiCad's `TA.AperFunction` comments.

The same package gives the same bytes: the images hold no time, and the
output is checked with `determinism_test`.
""",
    attrs = {
        "package": attr.label(
            doc = "The fab package (`fab_package`).",
            mandatory = True,
            providers = [FabPackageInfo],
        ),
        "px_per_mm": attr.int(
            doc = "The renders' resolution, in pixels per mm: 1 to 100. The default, 20, " +
                  "is about 500 dpi.",
            default = 20,
        ),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)
