# The e2e design

hdlf_rules_kicad's e2e design (its `integration/e2e`, as of commit adb54e1),
copied as the board the fab rules run on (issue #8). See hdlf_rules_kicad's
README of it for what the design holds: a 22 mm by 12 mm, two-layer board
with a header, a resistor and an LED.

* `main.kicad_sch`, `power.kicad_sch`: the schematic.
* `main.kicad_pcb`: the board, with one change: D1's reference text is
  to the right of D1, not above it, where it lay 0.015 mm from R1's pads
  (below D1 it would sit on D1's value text, on F.Fab). JLCPCB's
  rules want 0.15 mm from pad to silkscreen; KiCad's defaults do not
  check it.
* `main.kicad_pro`: the project file.

Each check of the design is in a package of its own: `fab_rules/` is the
board against a fab's design rules.
