<!-- SPDX-License-Identifier: Apache-2.0 -->
[![Test](https://github.com/filmil/bazel_rules_fab/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/filmil/bazel_rules_fab/actions/workflows/test.yml)
[![Tag and Release](https://github.com/filmil/bazel_rules_fab/actions/workflows/tag-and-release.yml/badge.svg)](https://github.com/filmil/bazel_rules_fab/releases)
[![Publish to my Bazel registry](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish.yml/badge.svg)](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish.yml)
[![Publish on Bazel Central Registry](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish-bcr.yml/badge.svg)](https://github.com/filmil/bazel_rules_fab/actions/workflows/publish-bcr.yml)

# rules_fab

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
bazel_dep(name = "rules_fab", version = "0.1.0")
```

rules_fab builds on rules_kicad, which is in no registry yet. Until it is,
pin the same commit as rules_fab does, in your `MODULE.bazel` (an override
counts only in the root module). It is GitHub's archive of that commit,
checked by its hash, so the build needs no `git` on the host:

```starlark
bazel_dep(name = "rules_kicad", version = "0.0.0")
archive_override(
    module_name = "rules_kicad",
    integrity = "sha256-0wW65aG/D8xPUFUlBxzLyxXOy4EilPYYa3A5mOfttTY=",
    strip_prefix = "bazel_rules_kicad-4b31ab05e198af1596832652a920b5c619af4936",
    urls = ["https://github.com/filmil/bazel_rules_kicad/archive/4b31ab05e198af1596832652a920b5c619af4936.tar.gz"],
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
load("@rules_fab//fab:defs.bzl", "fab_rules")
load("@rules_kicad//kicad:defs.bzl", "kicad_drc_test", "kicad_pcb")

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

A JLCPCB package of the board, from rules_kicad's outputs:

```starlark
load("@rules_fab//fab:defs.bzl", "fab_package")

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
BOM and centroid file instead.

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
Prebuilt tools and libraries that the build downloads are listed here, with
their versions, sources and licenses, as they are added.

* kicad-druid 1.2.0 (https://github.com/Cimos/kicad-druid), the design
  rules of `fab_rules`: MIT. The source archive of its tag, pinned by
  sha256 in `MODULE.bazel`, fetched from this repository's release
  `mirror-kicad-druid-1.2.0` (`tools/mirror`) or from GitHub.
