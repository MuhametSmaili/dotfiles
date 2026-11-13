"""Offline checks: no macOS calls, window writes, or permission prompts."""

import copy
import json
from pathlib import Path
import subprocess
import sys
import unittest

from layout import render


def desktop(numbers=(1, 2, 3), state=None, focused=0, command=None, size=(1440, 900)):
    return {
        "version": 1,
        "display": {"index": 1, "visible": {"x": 0, "y": 25, "width": size[0], "height": size[1]}},
        "space": 1,
        "gap": 10,
        "windows": [{"number": n, "order": i} for i, n in enumerate(numbers)],
        "focused": focused,
        "event": {"kind": "command", "name": command[0], "args": list(command[1:])} if command else {"kind": "relayout"},
        "state": copy.deepcopy(state),
    }


def frames(out):
    return {f["number"]: f["frame"] for f in out["frames"]}


class LayoutTests(unittest.TestCase):
    def test_empty_space(self):
        out = render(desktop((), focused=-1))
        self.assertEqual(out["frames"], [])
        self.assertNotIn("focus", out)

    def test_single_window_gaps(self):
        out = render(desktop((1,)))
        self.assertEqual(frames(out)[1], {"x": 10, "y": 35, "width": 1420, "height": 880})

    def test_tree_survives_window_server_reordering(self):
        first = render(desktop())
        second = render(desktop((3, 2, 1), state=first["state"], focused=2))
        self.assertEqual(frames(first), frames(second))

    def test_close_collapses_tree(self):
        initial = render(desktop())
        out = render(desktop((1,), state=initial["state"]))
        self.assertEqual(frames(out)[1]["width"], 1420)

    def test_new_window_gets_a_split(self):
        first = render(desktop((1, 2)))
        out = render(desktop(state=first["state"]))
        self.assertEqual(set(frames(out)), {1, 2, 3})
        self.assertNotEqual(frames(first)[1], frames(out)[1])

    def test_swap_and_split(self):
        first = render(desktop((1, 2)))
        swapped = render(desktop((1, 2), state=first["state"], command=("swap", "right")))
        self.assertEqual(frames(swapped)[1], frames(first)[2])
        flipped = render(desktop((1, 2), state=first["state"], command=("togglesplit",)))
        self.assertEqual(frames(flipped)[1]["width"], 1420)
        self.assertLess(frames(flipped)[1]["height"], 880)

    def test_focus_neighbor(self):
        first = render(desktop((1, 2)))
        out = render(desktop((1, 2), state=first["state"], command=("focus", "right")))
        self.assertEqual(out["focus"], 2)

    def test_whole_space_stack_and_return_preserve_bsp(self):
        first = render(desktop())
        stack = render(desktop(state=first["state"], command=("layout", "stack")))
        for rect in frames(stack).values():
            self.assertEqual(rect, {"x": 10, "y": 35, "width": 1420, "height": 880})
        back = render(desktop(state=stack["state"], command=("layout", "bsp")))
        self.assertEqual(frames(back), frames(first))

    def test_whole_space_stack_is_idempotent_and_cycles(self):
        stack = render(desktop(command=("layout", "stack")))
        out = render(desktop(state=stack["state"], command=("layout", "stack")))
        self.assertEqual(frames(out), frames(stack))
        self.assertEqual(out["state"], stack["state"])
        for direction, target in (("left", 3), ("up", 3), ("right", 2), ("down", 2)):
            cycle = render(desktop(state=out["state"], command=("focus", direction)))
            self.assertEqual(cycle["focus"], target)
            self.assertEqual(frames(cycle), frames(stack))
        portrait = render(desktop(command=("layout", "stack"), size=(800, 1400)))
        for rect in frames(portrait).values():
            self.assertEqual(rect, {"x": 10, "y": 35, "width": 780, "height": 1380})

    def test_whole_space_stack_handles_window_changes_and_floating(self):
        stack = render(desktop(command=("layout", "stack")))
        reordered = render(desktop((3, 2, 1), state=stack["state"], focused=2, command=("focus", "down")))
        self.assertEqual(reordered["focus"], 2)
        changed = render(desktop((1, 3, 4), state=stack["state"]))
        self.assertEqual(set(frames(changed)), {1, 3, 4})
        self.assertTrue(all(rect == frames(stack)[1] for rect in frames(changed).values()))
        floated = render(desktop(state=stack["state"], command=("togglefloat",)))
        self.assertEqual(floated["unmanaged"], [1])
        self.assertEqual(frames(floated)[1], {"x": 365, "y": 255, "width": 710, "height": 440})
        cycle = render(desktop(state=floated["state"], focused=1, command=("focus", "down")))
        self.assertEqual(cycle["focus"], 3)
        empty = render(desktop((), state=stack["state"], focused=-1, command=("focus", "down")))
        self.assertEqual(empty["frames"], [])
        self.assertNotIn("focus", empty)

    def test_float_and_retile(self):
        first = render(desktop())
        floated = render(desktop(state=first["state"], command=("togglefloat",)))
        self.assertEqual(floated["unmanaged"], [1])
        self.assertEqual(frames(floated)[1], {"x": 365, "y": 255, "width": 710, "height": 440})
        settled = render(desktop(state=floated["state"]))
        self.assertNotIn(1, frames(settled))
        tiled = render(desktop(state=floated["state"], command=("togglefloat",)))
        self.assertEqual(tiled["unmanaged"], [])
        self.assertIn(1, frames(tiled))

    def test_centered_float_honors_minimum_size_and_display_offset(self):
        inp = desktop()
        inp["event"] = {"kind": "command", "name": "togglefloat", "args": []}
        inp["display"]["visible"]["x"] = -1440
        inp["windows"][0]["minSize"] = {"width": 1000, "height": 600}
        out = render(inp)
        self.assertEqual(frames(out)[1], {"x": -1220, "y": 175, "width": 1000, "height": 600})
        inp["windows"][0]["minSize"] = {"width": 2000, "height": 2000}
        out = render(inp)
        self.assertEqual(frames(out)[1], {"x": -1430, "y": 35, "width": 1420, "height": 880})

    def test_maximise_exits_when_focus_changes(self):
        out = render(desktop(command=("togglemax",)))
        self.assertEqual(frames(out)[1]["width"], 1420)
        inp = desktop(state=out["state"], focused=1)
        inp["event"] = {"kind": "window_focus"}
        restored = render(inp)
        self.assertIsNone(restored["state"]["maximised"])
        self.assertLess(frames(restored)[1]["width"], 1420)

    def test_maximise_round_trip_preserves_layout_and_other_windows(self):
        first = render(desktop((1, 2)))
        split = render(desktop((1, 2), state=first["state"], command=("togglesplit",)))
        for _ in range(3):
            zoomed = render(desktop((1, 2), state=split["state"], command=("togglemax",)))
            self.assertEqual(frames(zoomed)[1], {"x": 10, "y": 35, "width": 1420, "height": 880})
            self.assertEqual(frames(zoomed)[2], frames(split)[2])
            self.assertEqual(zoomed["state"]["tree"], split["state"]["tree"])
            settled = render(desktop((1, 2), state=zoomed["state"]))
            self.assertEqual(frames(settled), frames(zoomed))
            restored = render(desktop((1, 2), state=settled["state"], command=("togglemax",)))
            self.assertEqual(frames(restored), frames(split))
            self.assertEqual(restored["state"], split["state"])

    def test_maximise_native_tab_replacement_and_close(self):
        zoomed = render(desktop(command=("togglemax",)))
        inp = desktop((4, 2, 3), state=zoomed["state"])
        inp["replaced"] = [{"was": 1, "window": 4}]
        replaced = render(inp)
        self.assertEqual(replaced["state"]["maximised"], 4)
        self.assertEqual(frames(replaced)[4], frames(zoomed)[1])
        closed = render(desktop((2, 3), state=replaced["state"]))
        self.assertIsNone(closed["state"]["maximised"])
        self.assertEqual(set(frames(closed)), {2, 3})

    def test_maximise_stack_does_not_shift_windows(self):
        stack = render(desktop(command=("layout", "stack")))
        zoomed = render(desktop(state=stack["state"], command=("togglemax",)))
        restored = render(desktop(state=zoomed["state"], command=("togglemax",)))
        self.assertEqual(frames(zoomed), frames(stack))
        self.assertEqual(frames(restored), frames(stack))
        self.assertEqual(restored["state"], stack["state"])

    def test_native_tab_replacement_keeps_position(self):
        first = render(desktop())
        inp = desktop((4, 2, 3), state=first["state"])
        inp["replaced"] = [{"was": 1, "window": 4}]
        out = render(inp)
        self.assertEqual(frames(out)[4], frames(first)[1])
        self.assertNotIn(1, frames(out))

    def test_learned_minimum(self):
        inp = desktop((1, 2))
        inp["windows"][0]["minSize"] = {"width": 900, "height": 500}
        out = render(inp)
        self.assertGreaterEqual(frames(out)[1]["width"], 900)

    def test_many_windows_stay_inside_display(self):
        for mode in ("bsp", "stack"):
            out = render(desktop(tuple(range(25)), command=("layout", mode)))
            self.assertEqual(len(out["frames"]), 25)
            for rect in frames(out).values():
                self.assertGreaterEqual(rect["x"], 10)
                self.assertGreaterEqual(rect["y"], 35)
                self.assertLessEqual(rect["x"] + rect["width"], 1430)
                self.assertLessEqual(rect["y"] + rect["height"], 915)
                self.assertGreater(rect["width"], 0)
                self.assertGreater(rect["height"], 0)

    def test_resident_protocol(self):
        payload = "\n".join(json.dumps(desktop(command=("layout", mode))) for mode in ("bsp", "stack"))
        process = subprocess.run(
            [sys.executable, "-B", str(Path(__file__).with_name("layout.py"))],
            input=payload + "\n", text=True, capture_output=True, check=True,
        )
        replies = [json.loads(line) for line in process.stdout.splitlines()]
        self.assertEqual([r["state"]["mode"] for r in replies], ["bsp", "stack"])
        self.assertEqual(process.stderr, "")


if __name__ == "__main__":
    unittest.main()
