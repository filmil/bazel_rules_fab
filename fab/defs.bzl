# SPDX-License-Identifier: Apache-2.0
"""Public API of hdlf_rules_fab.

Load every rule from this file:

```starlark
load("@hdlf_rules_fab//fab:defs.bzl", "determinism_test", "swap_test", "tool_toolchain")
```

The rules arrive in milestone M3 (see the issues of
https://github.com/filmil/bazel_rules_fab). This file is the only public
entry point; everything under `//fab/private` may change without notice.
"""

load(
    "//fab/private:assembly.bzl",
    _ASSEMBLY_MODEL_LIBRARIES = "ASSEMBLY_MODEL_LIBRARIES",
    _ASSEMBLY_PARTS = "ASSEMBLY_PARTS",
    _ASSEMBLY_VENDORS = "ASSEMBLY_VENDORS",
    _fab_assembly_swatch = "fab_assembly_swatch",
)
load("//fab/private:determinism.bzl", _determinism_test = "determinism_test")
load("//fab/private:fab_rules.bzl", _FAB_STACKUPS = "FAB_STACKUPS", _fab_rules = "fab_rules")
load("//fab/private:package.bzl", _FAB_PACKAGE_VENDORS = "FAB_PACKAGE_VENDORS", _fab_package = "fab_package")
load("//fab/private:package_test.bzl", _fab_package_test = "fab_package_test")
load("//fab/private:swap_test.bzl", _swap_test = "swap_test")
load("//fab/private:swatch.bzl", _SWATCH_BELOW = "SWATCH_BELOW", _fab_swatch = "fab_swatch")
load("//fab/private:toolchains.bzl", _tool_toolchain = "tool_toolchain")

ASSEMBLY_MODEL_LIBRARIES = _ASSEMBLY_MODEL_LIBRARIES
ASSEMBLY_PARTS = _ASSEMBLY_PARTS
ASSEMBLY_VENDORS = _ASSEMBLY_VENDORS
fab_assembly_swatch = _fab_assembly_swatch
determinism_test = _determinism_test

# The vendors fab_package knows, and the stackups fab_rules knows.
FAB_PACKAGE_VENDORS = _FAB_PACKAGE_VENDORS
FAB_STACKUPS = _FAB_STACKUPS

fab_package = _fab_package
fab_package_test = _fab_package_test
fab_rules = _fab_rules
fab_swatch = _fab_swatch
SWATCH_BELOW = _SWATCH_BELOW
swap_test = _swap_test
tool_toolchain = _tool_toolchain

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_FAB_API_VERSION = "0"
