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
  vendor's columns in the vendor's order, and at least one row each;
* every row has a designator; in the placement list, X and Y are in mm,
  the rotation is at least 0 and below 360, and the layer is one the
  vendor names.

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
        "want_error": attr.string(
            doc = "For a known-bad ZIP: text that one of the problems must contain. The " +
                  "test then passes only if the check finds it. Empty by default: the ZIP " +
                  "must be good.",
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)
