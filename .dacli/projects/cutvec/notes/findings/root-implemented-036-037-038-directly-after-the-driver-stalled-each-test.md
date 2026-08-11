---
id: f-root-implemented-036-037-038-directly-after-the-driver-stalled-each-test
kind: note
note_kind: finding
created: 2026-08-11T09:46:52Z
created_by: a-root
---
# Root implemented 036/037/038 directly after the driver stalled; each test verified to fail with its own fix reverted The serial driver stopped with 036/037/038 still open, and task 006's branch was 34 commits behind main (dacli#441), so an agent working there would have audited a tree missing the 029-035 fixes. Rather than restart into that, root implemented the three verified findings directly in a worktree cut from origin/main. PR #47.

036 Clock: rounding now happens once in whole milliseconds BEFORE the h/m split. The old code truncated with int() on the unrounded value while printing seconds with %06.3f, which rounds -- so a remainder rounding to 60 had nothing to carry into and emitted 00:00:60.000, which melt cannot parse. Carrying the rounding through the division makes the bad state unrepresentable rather than merely unlikely.

037 assets over MCP: callTool now routes an 'op' selector (index|search) exactly as it already did for timeline, and the catalog documents op with an enum so a client can discover it. Verified through the real binary, not just the test harness: a tools/call now reaches project.uninitialized (a genuine domain error from inside the handler) where it previously died at input.empty 'no assets subcommand given' before any worker ran.

038 timeline build: Validate now runs before anything is written, and deliberately AHEAD of the --plan branch -- a dry run should not answer 'I would write this' when the write produces something broken. The AC only asked for validation before Marshal/WriteFile; covering --plan too costs nothing and closes the more misleading half.

Every test was checked for teeth by reverting its own fix and confirming failure:
  036 -> Clock(59.9996) = 00:00:60.000, want 00:01:00.000 (and three more)
  037 -> op=search never reached its subcommand -- callTool did not route it
  038 -> duplicate producer id was accepted; result = {producers: 2}
This is the same check that exposed 027 as a false positive. A test that passes with and without its fix is decoration.

Auditor accuracy over the whole run: 10 true / 1 false.
