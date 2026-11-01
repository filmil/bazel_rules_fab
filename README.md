<!-- SPDX-License-Identifier: Apache-2.0 -->
[![Test](https://github.com/filmil/bazel_rules_fab/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/filmil/bazel_rules_fab/actions/workflows/test.yml)
[![Tag and Release](https://github.com/filmil/bazel_rules_fab/actions/workflows/tag-and-release.yml/badge.svg)](https://github.com/filmil/bazel_rules_fab/releases)
[![Publish to my Bazel registry](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish.yml/badge.svg)](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish.yml)
[![Publish on Bazel Central Registry](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish-bcr.yml/badge.svg)](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish-bcr.yml)

# hdlf_rules_fab

Hermetic Bazel rules for PCB fab packages.
The build turns the Gerbers, drill files, BOM and position files of a
KiCad board into the ZIP a vendor (JLCPCB, PCBWay) expects, with its file
names and column names, and checks the package. It also provides the
vendor design rules for the KiCad DRC.
Every tool comes from a Bazel toolchain, so nothing needs to be installed
on the machine, and two builds give byte-identical outputs.

This repository is new. The rules arrive in milestones; see the
[issues](https://github.com/filmil/bazel_rules_fab/issues) and the
[milestones](https://github.com/filmil/bazel_rules_fab/milestones).
Only Linux x86_64 hosts are supported.

## Using it

```starlark
bazel_dep(name = "hdlf_rules_fab", version = "0.1.0")
```

That is for a release. hdlf_rules_fab builds on hdlf_rules_kicad, which is in no
registry yet, and no release of hdlf_rules_fab is made until it is (issue #7).
To use hdlf_rules_fab meanwhile, pin a commit of it with an
`archive_override(module_name = "hdlf_rules_fab", ...)` of GitHub's archive of
that commit, and pin hdlf_rules_kicad as hdlf_rules_fab does, in your `MODULE.bazel`
(an override counts only in the root module). Each is an archive checked
by its hash, so the build needs no `git` on the host:

```starlark
bazel_dep(name = "hdlf_rules_kicad", version = "0.0.0")
archive_override(
    module_name = "hdlf_rules_kicad",
    integrity = "sha256-46jre2ndNDzCLSCl8vCHchOngJMCRWuO2zdDKuFB/Ew=",
    strip_prefix = "bazel_rules_kicad-9acaa9cbdcdf4b99a1e9b0ee0af79f1fc682f414",
    urls = ["https://github.com/filmil/bazel_rules_kicad/archive/9acaa9cbdcdf4b99a1e9b0ee0af79f1fc682f414.tar.gz"],
)
```

`.bazelrc`, while the module is only in the filmil registry:

```
common --registry=https://raw.githubusercontent.com/filmil/bazel-registry/main
common --registry=https://bcr.bazel.build
```

The rules are documented in [docs/defs.md](docs/defs.md). A board checked
against a fab house's design rules:

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "fab_rules")
load("@hdlf_rules_kicad//kicad:defs.bzl", "kicad_drc_test", "kicad_pcb")

fab_rules(
    name = "jlcpcb_rules",
    stackup = "2L-1oz",  # also 4L-1oz (the default), 4L-2oz, 6L-1oz
    vendor = "jlcpcb",   # also pcbway, generic
)

kicad_pcb(
    name = "pcb",
    board = "main.kicad_pcb",
    design_rules = ":jlcpcb_rules",
    schematic = ":sch",
)

kicad_drc_test(
    name = "drc_test",
    pcb = ":pcb",
)
```

A JLCPCB package of the board, from hdlf_rules_kicad's outputs:

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "fab_package")

fab_package(
    name = "jlcpcb",
    bom = ":bom",          # kicad_bom
    drill = ":drill",      # kicad_drill
    gerbers = ":gerbers",  # kicad_gerbers
    pos = ":pos",          # kicad_pos
    vendor = "jlcpcb",
)
```

`bazel build :jlcpcb` writes `jlcpcb.zip`: the Gerbers and drill file, and
JLCPCB's BOM and placement list (CPL). `vendor = "pcbway"` writes PCBWay's
BOM and centroid file instead. PCBWay's sample BOM is a spreadsheet; this
writes it as CSV, which PCBWay has not confirmed it takes, so check with
PCBWay before you order.

`fab_package_test` checks such a ZIP against what the vendor takes: its
layers, file names and columns.

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "fab_package_test")

fab_package_test(
    name = "jlcpcb_test",
    size = "small",
    package = ":jlcpcb",
    vendor = "jlcpcb",
)
```

### Capability swatch

`fab_swatch` is a credit-card board of a fab house's smallest features,
each labelled with its size: track widths and spacings, vias, plated and
non-plated holes, SMD pad gaps, silkscreen text and lines, copper to the
edge, and, on a 4- or 6-layer stackup, the inner tracks. Its numbers come
from the vendor's `fab_rules`, so it passes DRC with them; ordering one
shows what the vendor makes.

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "fab_swatch")
load("@hdlf_rules_kicad//kicad:defs.bzl", "kicad_drc_test")

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

`jlcpcb_swatch` is a `kicad_pcb` that carries the vendor's rules
(`jlcpcb_swatch_rules`, a `fab_rules`), so its DRC uses them.

`below` draws one feature a step under the vendor's minimum: a known-bad
board, whose DRC must fail on that feature's rule and no other.
`SWATCH_BELOW` maps each feature `below` takes to the DRC violation it
gives:

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "SWATCH_BELOW", "fab_swatch")

fab_swatch(
    name = "jlcpcb_thin_track",
    below = "track_width",
    vendor = "jlcpcb",
)

kicad_drc_test(
    name = "jlcpcb_thin_track_drc_test",
    expect = SWATCH_BELOW["track_width"],  # "track_width"
    pcb = ":jlcpcb_thin_track",
)
```

### Assembly swatch

`fab_assembly_swatch` is a ladder of real parts that a vendor's assembly
service places, smallest first, each between two test points so that a
multimeter shows whether it was soldered (issue #32):

| Rung | Part | Reads |
|---|---|---|
| 0201, 0402, 0603, 0805, 1206 | 1 kΩ 1 % resistor (UNI-ROYAL) | 1.00 kΩ ± 1 %, either way |
| SOT-23 | BAV99 (Nexperia) | about 0.6 V, pin 1 (+) to pin 3 |
| SOIC-8 | NE5532 (Texas Instruments) | about 0.6 V, either way between pins 3 and 2 |
| TSSOP-16, 0.65 mm | 74HC595PW (Nexperia) | about 0.6 V, pin 8 (+, GND) to pin 14 |
| QFN-16, 0.5 mm | TLV9064IRTER (Texas Instruments) | about 0.6 V, pin 10 (+, IN3+) to pin 2 (V+) |
| LQFP-48, 0.5 mm | STM32F030C8T6 (ST) | about 0.6 V, pin 23 (+, VSS) to pin 10 |

Each part is a catalog part, with its LCSC number, MPN, manufacturer and
datasheet in `ASSEMBLY_PARTS`; each IC reading is across a diode its
datasheet documents. Each vendor's assembly limits, with their sources,
are in `ASSEMBLY_VENDORS`, and leave out what the vendor cannot place:

* **JLCPCB**, Economic PCBA: from 0402 and 0.4 mm pitch
  ([capabilities](https://jlcpcb.com/capabilities/pcb-assembly-capabilities)).
  Economic takes the credit-card board as it is; Standard places 0201, but
  needs boards of 70 × 70 mm with rails, so it would need a panel.
* **PCBWay**: from 01005 and 0.25 mm pitch
  ([capabilities](https://www.pcbway.com/assembly-capabilities.html)), so
  every rung. The card is not panelized; PCBWay may panelize it on order,
  as its [machine assembly page](https://www.pcbway.com/helpcenter/pcb_assembly_ordering/What_are_the_requests_on_pcb_for_assembly_by_machine_.html)
  asks of boards with fewer than 200 solder points.

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "fab_assembly_swatch")
load("@hdlf_rules_kicad//kicad:defs.bzl", "kicad_drc_test", "kicad_erc_test")

fab_assembly_swatch(
    name = "jlcpcb_assembly",
    vendor = "jlcpcb",
)

kicad_drc_test(
    name = "jlcpcb_assembly_drc_test",
    pcb = ":jlcpcb_assembly",
)

kicad_erc_test(
    name = "jlcpcb_assembly_erc_test",
    schematic = ":jlcpcb_assembly_schematic",
)
```

`jlcpcb_assembly` is the board, a card like `fab_swatch`'s: each part's
footprint is KiCad's own, as it is, and its two test points sit straight
out from the measured pads, each on one short track. The silkscreen
names each rung and its reading, and marks the meter's + and - pads.
The board passes DRC with the vendor's rules and schematic parity, and
makes a package with `fab_package` (the BOM with LCSC numbers, and the
placement list) that passes `fab_package_test`.
`ASSEMBLY_MODEL_LIBRARIES` lists the 3D model libraries of its parts,
for `kicad_render(models = ...)`.

KiCad's footprints draw their silkscreen 0.10 mm (an 0402) to 0.14 mm
(a test point) from their own pads, under the vendors' 0.15 mm
(kicad-druid's rule "Pad to Silkscreen"); the fab clips silkscreen off
pads. So the board's rules are the vendor's, as they are, and one rule
after them: a footprint's silkscreen may come 0.09 mm from its own pads.
Everything else, the generator's labels among it, keeps the vendor's
0.15 mm; the tests show that a label on a pad, and a silkscreen line
0.12 mm from one, still fail.

## Building

```sh
bazel build //...
bazel test //...
cd integration && bazel test //...
```

After changing a `.bzl` file, regenerate the documentation with
`bazel run //docs:update_docs`.
Format with `bazel run //tools:buildifier`.

## Releasing

The operator runs the `Tag and Release` workflow
(`.github/workflows/tag-and-release.yml`, `workflow_dispatch`).
It picks the next version from the conventional commits since the last
tag, makes the tag, the release and its source archive, and opens one
pull request on the filmil registry and one on the Bazel Central Registry.
It needs the repository secret `BCR_PUBLISH_TOKEN`.

None of the modules published here are published with any sort of guarantee
whatsoever. Use at your own risk.

See the overall description at the [project page][pp].

[pp]: https://hdlfactory.com/post/2025/09/29/getting-ready-for-the-brave-new-bazel-modules-world/

## Licenses

This repository is under the Apache License 2.0 (`LICENSE`).
The prebuilt tools and libraries that the build downloads, with their
versions, sources and licenses:

* kicad-druid 1.2.0 (https://github.com/Cimos/kicad-druid), the design
  rules of `fab_rules`: MIT. The source archive of its tag, pinned by
  sha256 in `MODULE.bazel`, fetched from this repository's release
  `mirror-kicad-druid-1.2.0` (`tools/mirror`) or from GitHub.
* The Go SDK (go1.26.7 with rules_go 0.64.2, https://go.dev):
  BSD-3-Clause. rules_go downloads it to build this module's packager,
  checkers and swatch generators from source.
* Through hdlf_rules_kicad, which runs KiCad for these rules: KiCad 9.0.9
  (GPL-3.0-or-later), KiCad's symbol and footprint libraries 9.0.9
  (CC-BY-SA 4.0 with the KiCad libraries exception), and the DejaVu
  fonts 2.37 (the Bitstream Vera fonts license, the DejaVu changes in the
  public domain); and, only for a build that renders or exports 3D
  models, KiCad's 3D models 9.0.9 (the same license as the libraries)
  and rain 2.2.1
  (MIT), which fetches them. hdlf_rules_kicad's README lists each with
  its source:
  https://github.com/filmil/bazel_rules_kicad#licenses.
