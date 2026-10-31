# SPDX-License-Identifier: Apache-2.0
"""fab_package_test: checks a fab_package ZIP against its vendor (issue #11)."""

load(":package.bzl", "FAB_PACKAGE_VENDORS")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")

def _fab_package_test_impl(ctx):
    pkgcheck = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers.pkgcheck
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = pkgcheck.files_to_run.executable,
        is_executable = True,
    )
    return [
        DefaultInfo(
            executable = exe,
            runfiles = ctx.runfiles(files = [ctx.file.package]).merge(pkgcheck.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            "PKG_BARE": "1" if ctx.attr.bare else "",
            "PKG_COPPER_LAYERS": str(ctx.attr.copper_layers) if ctx.attr.copper_layers else "",
            "PKG_VENDOR": ctx.attr.vendor,
            "PKG_WANT_ERROR": ctx.attr.want_error,
            "PKG_ZIP": ctx.file.package.short_path,
        }),
    ]

fab_package_test = rule(
    implementation = _fab_package_test_impl,
    test = True,
    doc = """Checks a fab house's ZIP against what the vendor takes.

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
""",
    attrs = {
        "package": attr.label(
            doc = "The ZIP (`fab_package`).",
            mandatory = True,
            allow_single_file = [".zip"],
        ),
        "vendor": attr.string(
            doc = "The fab house: " + ", ".join(FAB_PACKAGE_VENDORS) + ".",
            mandatory = True,
            values = FAB_PACKAGE_VENDORS,
        ),
        "bare": attr.bool(
            doc = "The ZIP is a bare board's: it must have no BOM and no placement list. " +
                  "False by default: it must have one of each, so a package that lost them " +
                  "fails.",
        ),
        "copper_layers": attr.int(
            doc = "The board's number of copper layers, such as 6: the ZIP must then hold " +
                  "exactly its inner layers' Gerbers, `In1.Cu` to `In<copper_layers - 2>.Cu` " +
                  "(none for 2). 0, the default, does not check them.",
            values = [0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32],
        ),
        "want_error": attr.string(
            doc = "For a known-bad ZIP: text that one of the problems must contain. The " +
                  "test then passes only if the check finds it. Empty by default: the ZIP " +
                  "must be good.",
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)
