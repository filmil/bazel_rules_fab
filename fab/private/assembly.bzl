# SPDX-License-Identifier: Apache-2.0
"""fab_assembly_swatch: a ladder of placed parts that a meter can check (issue #32)."""

load("@hdlf_rules_kicad//kicad:defs.bzl", "kicad_schematic")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

# Passive (chip) sizes, smallest first.
PASSIVE_SIZES = ["01005", "0201", "0402", "0603", "0805", "1206"]

# What each vendor's assembly service places, from its own pages (read on
# 2026-10-31). kicad-druid has no assembly numbers, so they live here, each
# with its source. `smallest_passive` is the smallest chip size;
# `min_pitch_mm` the finest IC lead pitch.
ASSEMBLY_VENDORS = {
    "jlcpcb": {
        "name": "JLCPCB",
        # Economic PCBA takes a credit-card board as it is; Standard needs
        # 70 x 70 mm with 5 mm rails, so the card would need a panel.
        "tier": "Economic PCBA",
        "smallest_passive": "0402",
        "min_pitch_mm": 0.4,
        "sources": [
            # "Minimum Package: 0402 / 01005"; "Minimum IC Pin Spacing:
            # 0.4mm / 0.35mm" (Economic / Standard).
            "https://jlcpcb.com/capabilities/pcb-assembly-capabilities",
            # Part to part spacing, for the board (issue #32 (b)).
            "https://jlcpcb.com/help/article/minimum-spacing-for-smd-components",
        ],
    },
    "pcbway": {
        "name": "PCBWay",
        "tier": "PCB assembly",
        # "we can accept components as small as 01005, 0201,0402."
        "smallest_passive": "01005",
        # "Fine Pitch Parts: we can assemble 0.25mm fine pitch parts."
        "min_pitch_mm": 0.25,
        "sources": [
            "https://www.pcbway.com/assembly-capabilities.html",
            # The card is not panelized; PCBWay may panelize it on order.
            "https://www.pcbway.com/helpcenter/pcb_assembly_ordering/What_are_the_requests_on_pcb_for_assembly_by_machine_.html",
        ],
    },
}

# The ladder, smallest first. Every part is a real catalog part; each LCSC
# number was checked on its jlcpcb.com/partdetail page (2026-10-31), and
# each reading is from the part's datasheet, with its pins as the
# datasheet's pinout prints them. `measure` is the meter's + pin, then its
# - pin; `silk` is what the board prints next to the test pads.
_R = "Device:R"
_R_DATASHEET = "https://www.lcsc.com/datasheet/lcsc_datasheet_%s.pdf"

def _resistor(size, metric, mpn, lcsc, sheet):
    return {
        "rung": size,
        "kind": "passive",
        "ref": "R",
        "symbol": _R,
        "footprint": "Resistor_SMD:R_%s_%sMetric" % (size, metric),
        "value": "1k",
        "measure": ["1", "2"],
        "reading": "1.00 kOhm +-1 % either way (pin 1, pin 2)",
        "silk": "1.00k 1%",
        "fields": {
            "Datasheet": _R_DATASHEET % sheet,
            "LCSC": lcsc,
            "MPN": mpn,
            "Manufacturer": "UNI-ROYAL",
            "Tolerance": "1%",
        },
    }

ASSEMBLY_PARTS = [
    _resistor("0201", "0603", "0201WMF1001TEE", "C270365", "2205311945_UNI-ROYAL-Uniroyal-Elec-0201WMF1001TEE_C270365"),
    _resistor("0402", "1005", "0402WGF1001TCE", "C11702", "2206010216_UNI-ROYAL-Uniroyal-Elec-0402WGF1001TCE_C11702"),
    _resistor("0603", "1608", "0603WAF1001T5E", "C21190", "2206010130_UNI-ROYAL-Uniroyal-Elec-0603WAF1001T5E_C21190"),
    _resistor("0805", "2012", "0805W8F1001T5E", "C17513", "2205311800_UNI-ROYAL-Uniroyal-Elec-0805W8F1001T5E_C17513"),
    _resistor("1206", "3216", "1206W4F1001T5E", "C4410", "2206010216_UNI-ROYAL-Uniroyal-Elec-1206W4F1001T5E_C4410"),
    {
        "rung": "SOT-23",
        "kind": "ic",
        "pitch_mm": 0.95,
        "ref": "D",
        "symbol": "Diode:BAV99",
        "footprint": "Package_TO_SOT_SMD:SOT-23",
        "value": "BAV99",
        # Datasheet table 3, pinning: 1 anode (diode 1), 2 cathode (diode
        # 2), 3 cathode (diode 1) and anode (diode 2).
        "measure": ["1", "3"],
        "reading": "diode, about 0.6 V from pin 1 (anode, diode 1) to pin 3 (cathode, diode 1); open the other way",
        "silk": "0.6V",
        "fields": {
            "Datasheet": "https://www.lcsc.com/datasheet/lcsc_datasheet_2407250951_Nexperia-BAV99-215_C2500.pdf",
            "LCSC": "C2500",
            "MPN": "BAV99,215",
            "Manufacturer": "Nexperia",
        },
    },
    {
        "rung": "SOIC-8",
        "kind": "ic",
        "pitch_mm": 1.27,
        "ref": "U",
        "symbol": "Amplifier_Operational:NE5532",
        "footprint": "Package_SO:SOIC-8_3.9x4.9mm_P1.27mm",
        "value": "NE5532",
        # §5.1, note 4: input protection diodes, back to back, between the
        # inputs.
        "measure": ["3", "2"],
        "reading": "diode, about 0.6 V either way between pin 3 (1IN+) and pin 2 (1IN-)",
        "silk": "0.6V",
        "fields": {
            "Datasheet": "https://www.ti.com/lit/ds/symlink/ne5532.pdf",
            "LCSC": "C7426",
            "MPN": "NE5532DR",
            "Manufacturer": "Texas Instruments",
        },
    },
    {
        "rung": "TSSOP-16",
        "kind": "ic",
        "pitch_mm": 0.65,
        "ref": "U",
        "symbol": "74xx:74HC595",
        "footprint": "Package_SO:TSSOP-16_4.4x5mm_P0.65mm",
        "value": "74HC595",
        # "Inputs include clamp diodes": pin 8 (GND) to pin 14 (DS).
        "measure": ["8", "14"],
        "reading": "diode, about 0.6 V from pin 8 (GND) to pin 14 (DS); open the other way",
        "silk": "0.6V",
        "fields": {
            "Datasheet": "https://www.lcsc.com/datasheet/lcsc_datasheet_2407241044_Nexperia-74HC595PW-118_C5948.pdf",
            "LCSC": "C5948",
            "MPN": "74HC595PW,118",
            "Manufacturer": "Nexperia",
        },
    },
    {
        "rung": "QFN-16",
        "kind": "ic",
        "pitch_mm": 0.5,
        "ref": "U",
        "symbol": "74xx:74HC595",
        "footprint": "Package_DFN_QFN:DHVQFN-16-1EP_2.5x3.5mm_P0.5mm_EP1x2mm",
        "value": "74HC595",
        # The same die and pin numbers as the TSSOP part.
        "measure": ["8", "14"],
        "reading": "diode, about 0.6 V from pin 8 (GND) to pin 14 (DS); open the other way",
        "silk": "0.6V",
        "fields": {
            "Datasheet": "https://www.lcsc.com/datasheet/lcsc_datasheet_2407261712_Nexperia-74HC595BQ-115_C730243.pdf",
            "LCSC": "C730243",
            "MPN": "74HC595BQ,115",
            "Manufacturer": "Nexperia",
        },
    },
    {
        "rung": "LQFP-48",
        "kind": "ic",
        "pitch_mm": 0.5,
        "ref": "U",
        "symbol": "MCU_ST_STM32F0:STM32F030C8Tx",
        "footprint": "Package_QFP:LQFP-48_7x7mm_P0.5mm",
        "value": "STM32F030C8T6",
        # Table 16: injection current; VIN < VSS conducts, so pin 23 (VSS)
        # to pin 10 (PA0).
        "measure": ["23", "10"],
        "reading": "diode, about 0.6 V from pin 23 (VSS) to pin 10 (PA0); open the other way",
        "silk": "0.6V",
        "fields": {
            "Datasheet": "https://www.lcsc.com/datasheet/lcsc_datasheet_2304140030_STMicroelectronics-STM32F030C8T6_C23922.pdf",
            "LCSC": "C23922",
            "MPN": "STM32F030C8T6",
            "Manufacturer": "STMicroelectronics",
        },
    },
]

def assembly_rungs(vendor, start = ""):
    """Returns the rungs the vendor places: ASSEMBLY_PARTS from `start`.

    A passive smaller than the vendor's smallest, or an IC finer than its
    finest pitch, is left out.

    Args:
      vendor: a key of ASSEMBLY_VENDORS.
      start: the first rung's name, or empty for the vendor's smallest.
        Fails when the vendor cannot place it (the known-bad case).

    Returns:
      The entries of ASSEMBLY_PARTS that the swatch has, smallest first.
    """
    v = ASSEMBLY_VENDORS[vendor]
    limits = [assembly_limit(v, p) for p in ASSEMBLY_PARTS]
    if start:
        names = [p["rung"] for p in ASSEMBLY_PARTS]
        if start not in names:
            fail("start %r: want one of %s" % (start, names))
        i = names.index(start)
        if limits[i]:
            fail(assembly_refusal(v, ASSEMBLY_PARTS[i]))
        return [p for j, p in enumerate(ASSEMBLY_PARTS) if j >= i and not limits[j]]
    return [p for j, p in enumerate(ASSEMBLY_PARTS) if not limits[j]]

def assembly_limit(v, p):
    """Returns the vendor's limit that a part is below, or "" if it fits.

    Args:
      v: an entry of ASSEMBLY_VENDORS.
      p: an entry of ASSEMBLY_PARTS.

    Returns:
      "smallest package 0402", "finest pitch 0.4 mm", or "".
    """
    if p["kind"] == "passive":
        if PASSIVE_SIZES.index(p["rung"]) < PASSIVE_SIZES.index(v["smallest_passive"]):
            return "smallest package %s" % v["smallest_passive"]
    elif p["pitch_mm"] < v["min_pitch_mm"]:
        return "finest pitch %s mm" % v["min_pitch_mm"]
    return ""

def assembly_refusal(v, p):
    """Returns the message for a part the vendor cannot place.

    Args:
      v: an entry of ASSEMBLY_VENDORS.
      p: an entry of ASSEMBLY_PARTS that is below one of its limits.

    Returns:
      The message, with the limit's source.
    """
    return "%s %s cannot place %s: below its %s (%s)" % (v["name"], v["tier"], p["rung"], assembly_limit(v, p), v["sources"][0])

# The symbol libraries the parts and the test points use.
_LIBS = ["74xx", "Amplifier_Operational", "Connector", "Device", "Diode", "MCU_ST_STM32F0", "power"]

def _spec(name, vendor, start):
    v = ASSEMBLY_VENDORS[vendor]
    counts = {}
    rungs = []
    for p in assembly_rungs(vendor, start):
        counts[p["ref"]] = counts.get(p["ref"], 0) + 1
        rungs.append({
            "ref": "%s%d" % (p["ref"], counts[p["ref"]]),
            "symbol": p["symbol"],
            "footprint": p["footprint"],
            "value": p["value"],
            "label": p["rung"],
            "reading": p["reading"],
            "measure": p["measure"],
            "fields": p["fields"],
        })
    return json.encode({
        "project": name,
        "rungs": rungs,
        "title": "%s assembly swatch (%s)" % (v["name"], v["tier"]),
    })

def _assembly_files_impl(ctx):
    libs = {}
    for f in ctx.files.symbols:
        if f.extension == "kicad_sym":
            libs[f.basename[:-len(".kicad_sym")]] = f
    missing = [lib for lib in _LIBS if lib not in libs]
    if missing:
        fail("%s: the symbol libraries %s lack %s" % (ctx.label, ctx.attr.symbols.label, missing))
    spec = ctx.actions.declare_file(ctx.label.name + ".json")
    ctx.actions.write(spec, _spec(ctx.attr.project, ctx.attr.vendor, ctx.attr.start))
    args = ctx.actions.args()
    args.add("--spec", spec)
    args.add("--sch", ctx.outputs.schematic)
    for lib in _LIBS:
        args.add("--lib", "%s=%s" % (lib, libs[lib].path))
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "assembly",
        arguments = [args],
        inputs = [spec] + [libs[lib] for lib in _LIBS],
        outputs = [ctx.outputs.schematic],
        mnemonic = "FabAssemblySchematic",
        progress_message = "Drawing the assembly swatch's schematic %{label}",
    )
    return [DefaultInfo(files = depset([ctx.outputs.schematic]))]

assembly_files = rule(
    implementation = _assembly_files_impl,
    doc = "The rule behind fab_assembly_swatch: the schematic. Use the macro.",
    attrs = {
        "vendor": attr.string(doc = "The fab house.", mandatory = True, values = sorted(ASSEMBLY_VENDORS.keys())),
        "start": attr.string(doc = "The first rung; empty for the vendor's smallest."),
        "project": attr.string(doc = "The schematic's base name.", mandatory = True),
        "symbols": attr.label(
            doc = "KiCad's symbol libraries.",
            allow_files = True,
            default = Label("@kicad_libs//:symbols"),
        ),
        "schematic": attr.output(mandatory = True),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)

def fab_assembly_swatch(name, vendor, start = "", **kwargs):
    """An assembly swatch: a ladder of real parts, each between two test points.

    ```starlark
    fab_assembly_swatch(
        name = "jlcpcb_assembly",
        vendor = "jlcpcb",
    )

    kicad_erc_test(
        name = "jlcpcb_assembly_erc_test",
        schematic = ":jlcpcb_assembly",
    )
    ```

    The rungs, smallest first: 1 kOhm 1 % resistors in 0201, 0402, 0603,
    0805 and 1206, then SOT-23, SOIC-8, TSSOP-16 (0.65 mm), QFN-16
    (0.5 mm) and LQFP-48 (0.5 mm), each a real catalog part with its LCSC
    number, MPN, manufacturer and datasheet (`ASSEMBLY_PARTS`). The
    vendor's assembly limits (`ASSEMBLY_VENDORS`, each with its source)
    leave out what it cannot place: JLCPCB's Economic PCBA starts at 0402.

    Each part's two measured pins go to two test points, so a meter shows
    whether it was soldered: 1.00 kOhm on a resistor, a diode drop of about
    0.6 V on an IC, across a diode its datasheet documents. Its other pins
    carry no-connect flags, and a net with a power input pin has a
    PWR_FLAG, so ERC passes. The fields carry LCSC, MPN and Manufacturer
    for the BOM.

    Each `ASSEMBLY_PARTS` entry has `rung` (the package, such as "0402"
    or "SOT-23"), `kind` ("passive" or "ic"), `pitch_mm` for an IC,
    `symbol` and `footprint` (from @kicad_libs), `value`, `ref` (the
    reference prefix), `measure` (the two pins the test points go to),
    `reading` (what a meter shows across them), `silk` (what the board
    prints by the test pads), and `fields` (Datasheet, LCSC, MPN,
    Manufacturer, for the schematic and the BOM). Each `ASSEMBLY_VENDORS` entry has `name`,
    `tier` (the assembly service), `smallest_passive`, `min_pitch_mm`, and
    `sources` (the URLs the limits come from).

    `name` is a `kicad_schematic`, for `kicad_erc_test` and `kicad_bom`.
    The board and its package follow in issue #32 (b).

    Args:
      name: the schematic's name.
      vendor: the fab house: jlcpcb or pcbway.
      start: the first rung (such as "0402"); by default the smallest the
        vendor places. A rung the vendor cannot place fails: a known-bad
        swatch.
      **kwargs: common attributes, such as `visibility` and `tags`.
    """
    if vendor not in ASSEMBLY_VENDORS:
        fail("fab_assembly_swatch %s: vendor %r: want one of %s" % (name, vendor, sorted(ASSEMBLY_VENDORS.keys())))

    # The rungs are picked at analysis, so a known-bad `start` fails there,
    # where a test can see it, not when the BUILD file loads.
    common = {k: kwargs[k] for k in ("tags", "testonly", "target_compatible_with") if k in kwargs}
    assembly_files(
        name = name + "_files",
        vendor = vendor,
        start = start,
        project = name,
        schematic = name + ".kicad_sch",
        visibility = ["//visibility:private"],
        **common
    )
    kicad_schematic(
        name = name,
        root = name + ".kicad_sch",
        deps = ["@kicad_libs//:standard"],
        **kwargs
    )
