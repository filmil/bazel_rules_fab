# SPDX-License-Identifier: Apache-2.0
"""Unit tests of the assembly swatch's vendor limits."""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":assembly.bzl", "ASSEMBLY_PARTS", "ASSEMBLY_VENDORS", "assembly_limit", "assembly_refusal", "assembly_rungs")

def _part(rung):
    return [p for p in ASSEMBLY_PARTS if p["rung"] == rung][0]

# A vendor that places nothing finer than 0.65 mm: no real vendor in the
# table is this coarse, so it tests the pitch limit.
_COARSE = {
    "name": "Coarse",
    "tier": "Test",
    "smallest_passive": "0603",
    "min_pitch_mm": 0.65,
    "sources": ["https://example.com/limits"],
}

def _limits_impl(ctx):
    env = unittest.begin(ctx)
    jlcpcb = ASSEMBLY_VENDORS["jlcpcb"]
    asserts.equals(env, "smallest package 0402", assembly_limit(jlcpcb, _part("0201")))
    asserts.equals(env, "", assembly_limit(jlcpcb, _part("0402")))
    asserts.equals(env, "", assembly_limit(jlcpcb, _part("LQFP-48")))
    asserts.equals(env, "smallest package 0603", assembly_limit(_COARSE, _part("0402")))
    asserts.equals(env, "", assembly_limit(_COARSE, _part("TSSOP-16")))
    asserts.equals(env, "finest pitch 0.65 mm", assembly_limit(_COARSE, _part("QFN-16")))
    asserts.equals(env, "finest pitch 0.65 mm", assembly_limit(_COARSE, _part("LQFP-48")))
    asserts.equals(
        env,
        "Coarse Test cannot place LQFP-48: below its finest pitch 0.65 mm (https://example.com/limits)",
        assembly_refusal(_COARSE, _part("LQFP-48")),
    )
    return unittest.end(env)

limits_test = unittest.make(_limits_impl)

def _rungs_impl(ctx):
    env = unittest.begin(ctx)
    names = lambda ps: [p["rung"] for p in ps]
    asserts.equals(env, [p["rung"] for p in ASSEMBLY_PARTS], names(assembly_rungs("pcbway", "")))
    asserts.equals(env, [p["rung"] for p in ASSEMBLY_PARTS[1:]], names(assembly_rungs("jlcpcb", "")))
    asserts.equals(env, ["1206", "SOT-23"], names(assembly_rungs("jlcpcb", "1206"))[:2])
    return unittest.end(env)

rungs_test = unittest.make(_rungs_impl)
