# SPDX-License-Identifier: Apache-2.0
#
# kicad-druid 1.2.0 (https://github.com/Cimos/kicad-druid, MIT): KiCad
# design rules for fab houses, one .kicad_dru per build variant, and a test
# board for JLCPCB and for PCBWay (issue #8).

# The rule files, for fab_rules.
filegroup(
    name = "rules",
    srcs = glob(["*/*.kicad_dru"]),
    visibility = ["//visibility:public"],
)

# The test boards, as fixtures. Each fails DRC on purpose, in several ways:
# it has a footprint that passes and one that fails each rule.
exports_files(
    glob([
        "JLCPCB/JLCPCB.kicad_*",
        "PCBWay/PCBWay.kicad_*",
    ]) + ["LICENSE"],
    visibility = ["//visibility:public"],
)
