#!/usr/bin/env python3
"""Resident mimi layout: dwindle BSP or workspace-wide overlapping stack.

JSON lines in/out; standard library only. State belongs to mimi, per Space and
screen. No subprocesses or disk writes per request. Stack windows overlap and
rely on macOS raising the focused window. No drag interpretation or insertion
hints are added.

Read from render() first, then follow the state, command, geometry and tree
sections. Command and Geometry hold request-local inputs, not persistent state.
Tree nodes remain the original JSON dictionaries; no object conversion is needed.

render() owns and mutates the supplied state. Inputs must contain unique window
IDs and a valid, acyclic tree. This refactor preserves valid-input behavior, not
an untrusted-input boundary: malformed input still raises, and the JSON codec
still has its own nesting limits despite the iterative tree operations.
"""

from __future__ import annotations

import json
import sys
from typing import NamedTuple


DIRECTIONS = ("left", "right", "up", "down")


class Command(NamedTuple):
    """Command arguments plus the event kind needed by focus handling."""

    kind: str | None
    name: str | None
    args: list

    @classmethod
    def from_event(cls, event: dict) -> Command:
        kind = event.get("kind")
        name = event.get("name") if kind == "command" else None
        return cls(kind, name, event.get("args", []))

    @property
    def direction(self) -> str:
        return self.args[0] if self.args else ""


class Geometry(NamedTuple):
    """Display constraints for one request; never included in returned state."""

    box: dict[str, float]
    gap: float
    minimums: dict[int, tuple[float, float]]

    @classmethod
    def from_request(cls, request: dict) -> Geometry:
        gap = float(request.get("gap", 10))
        box = inset_frame(request["display"]["visible"], gap)
        minimums = {
            window["number"]: (
                window.get("minSize", {}).get("width", 0),
                window.get("minSize", {}).get("height", 0),
            )
            for window in request["windows"]
        }
        return cls(box, gap, minimums)

    def frames(self, tree: dict | None) -> dict[int, dict]:
        return rectangles(tree, self.box, self.gap, self.minimums)


def render(request: dict) -> dict:
    """Render one request, mutating its state in place; no cross-request caches."""
    state = request.get("state") or {}
    state.setdefault("mode", "bsp")
    command = Command.from_event(request.get("event", {}))
    windows = request["windows"]
    focused = focused_window(windows, request.get("focused", -1))
    geometry = Geometry.from_request(request)

    reconcile_windows(state, windows, request.get("replaced", []))
    floating, order = update_floating(state, focused, command)
    update_layout_mode(state, command)
    tree, by_window = sync_tree(state.get("tree"), order, focused, geometry)

    if state["mode"] == "stack":
        frames, focus = render_stack(state, order, focused, command, geometry)
    else:
        tree, frames, focus = render_bsp(tree, by_window, focused, command, geometry)
    state["tree"] = tree

    apply_maximise(state, frames, focused, command, geometry.box)
    if command.name == "togglefloat" and focused in floating:
        minimum = geometry.minimums.get(focused, (0, 0))
        frames[focused] = floating_frame(geometry.box, minimum)

    return build_response(state, frames, focus)


# -- State and window reconciliation -------------------------------------------


def focused_window(windows: list[dict], index: int) -> int | None:
    """The protocol supplies a list index, not a window ID."""
    if 0 <= index < len(windows):
        return windows[index]["number"]
    return None


def reconcile_windows(state: dict, windows: list[dict], replaced: list[dict]) -> None:
    """Remap saved IDs, drop closed windows, and append newly observed windows."""
    renames = {item["was"]: item["window"] for item in replaced}
    present = {window["number"] for window in windows}
    for leaf in leaves(state.get("tree")):
        leaf["windows"] = [renames.get(n, n) for n in leaf["windows"]]
        leaf["active"] = renames.get(leaf["active"], leaf["active"])

    order = []
    for number in state.get("order", []):
        number = renames.get(number, number)
        if number in present:
            order.append(number)
    seen = set(order)
    for window in windows:
        number = window["number"]
        if number not in seen:
            order.append(number)
            seen.add(number)
    state["order"] = order

    state["floating"] = sorted(
        {renames.get(n, n) for n in state.get("floating", [])} & present
    )
    maximised = state.get("maximised")
    state["maximised"] = renames.get(maximised, maximised)


def update_floating(state: dict, focused: int | None,
                    command: Command) -> tuple[set[int], list[int]]:
    """Persist the floating set and return it with the ordered tiled windows."""
    floating = set(state["floating"])
    if command.name == "togglefloat" and focused is not None:
        floating.symmetric_difference_update((focused,))
    state["floating"] = sorted(floating)
    order = [number for number in state["order"] if number not in floating]
    return floating, order


def update_layout_mode(state: dict, command: Command) -> None:
    if command.name != "layout" or not command.args:
        return
    if command.args[0] in ("bsp", "stack"):
        state["mode"] = command.args[0]


def sync_tree(tree: dict | None, order: list[int], focused: int | None,
              geometry: Geometry) -> tuple[dict | None, dict[int, dict]]:
    """Reconcile the tree and build an index valid until a swap."""
    tree = prune_tree(tree, set(order))
    by_window = {number: leaf for leaf in leaves(tree) for number in leaf["windows"]}
    for number in order:
        if number in by_window:
            continue
        target = insertion_target(tree, focused)
        tree, inserted = insert_window(tree, target, number, geometry.frames(tree))
        if inserted is not None:
            by_window[number] = inserted

    held = by_window.get(focused)
    if held is not None:
        held["active"] = focused
    return tree, by_window


def insertion_target(tree: dict | None, focused: int | None) -> int | None:
    """Prefer the focused active window; otherwise split the last leaf."""
    last_active = None
    for leaf in leaves(tree):
        last_active = leaf["active"]
        if last_active == focused:
            return focused
    return last_active


# -- Layout commands ----------------------------------------------------------


def render_stack(state: dict, order: list[int], focused: int | None,
                 command: Command, geometry: Geometry) -> tuple[dict[int, dict], int | None]:
    """Cycle or swap workspace-wide stack order, then overlap all tiled windows."""
    focus = None
    directional = command.name in ("focus", "swap") and command.direction in DIRECTIONS
    if focused in order and directional:
        step = -1 if command.direction in ("left", "up") else 1
        other = cycle_window(order, focused, step)
        if command.name == "swap":
            swap_positions(state["order"], focused, other)
            swap_positions(order, focused, other)
        else:
            focus = other
    frames = {number: dict(geometry.box) for number in order}
    return frames, focus


def render_bsp(tree: dict | None, by_window: dict, focused: int | None,
               command: Command,
               geometry: Geometry) -> tuple[dict | None, dict[int, dict], int | None]:
    """Apply one BSP command and calculate final geometry only when needed."""
    frames = None
    focus = None
    if command.name == "togglesplit":
        toggle_split(tree, focused)
    elif command.name in ("focus", "swap") and command.args and focused in by_window:
        frames, focus = apply_directional_command(
            tree, by_window, focused, command, geometry
        )
    if frames is None:
        frames = geometry.frames(tree)
    return tree, frames, focus


def toggle_split(tree: dict | None, focused: int | None) -> None:
    """Flip the focused leaf's parent split, leaving a root leaf unchanged."""
    location = locate_leaf(tree, focused)
    if location is None or location[1] is None:
        return
    parent = location[1]
    parent["axis"] = "vertical" if parent["axis"] == "horizontal" else "horizontal"


def apply_directional_command(tree: dict, by_window: dict, focused: int,
                              command: Command,
                              geometry: Geometry) -> tuple[dict | None, int | None]:
    """Return (reusable frames, focus); None frames mean geometry changed."""
    frames = geometry.frames(tree)
    other = neighbour(tree, frames, focused, command.direction)
    target = by_window.get(other)
    if target is None:
        return frames, None
    if command.name == "focus":
        return frames, target["active"]
    swap_stacks(by_window[focused], target)
    # Swapping can change minimum sizes, even though the splits are unchanged.
    return None, None


def cycle_window(members: list[int], focused: int, step: int) -> int:
    """Wrap within a nonempty list that contains the focused window."""
    index = members.index(focused)
    return members[(index + step) % len(members)]


def swap_positions(order: list[int], first: int, second: int) -> None:
    first_index, second_index = order.index(first), order.index(second)
    order[first_index], order[second_index] = order[second_index], order[first_index]


def swap_stacks(source: dict, target: dict) -> None:
    """BSP swap moves entire leaf stacks, not only their active windows."""
    source["windows"], target["windows"] = target["windows"], source["windows"]
    source["active"], target["active"] = target["active"], source["active"]


def apply_maximise(state: dict, frames: dict, focused: int | None,
                   command: Command, box: dict) -> None:
    """Toggle maximisation or clear it when focus moves to a different window."""
    maximised = state.get("maximised")
    if command.name == "togglemax" and focused in frames:
        maximised = None if maximised == focused else focused
    elif maximised not in frames or (
        command.kind == "window_focus" and focused not in (None, maximised)
    ):
        maximised = None
    state["maximised"] = maximised
    if maximised is not None:
        frames[maximised] = dict(box)


# -- Geometry: measure each subtree once, then place it once --------------------


def inset_frame(frame: dict, gap: float) -> dict:
    box = dict(frame)
    box["x"] += gap
    box["y"] += gap
    box["width"] = max(1, box["width"] - 2 * gap)
    box["height"] = max(1, box["height"] - 2 * gap)
    return box


def floating_frame(box: dict, minimum: tuple[float, float]) -> dict:
    """Match the existing centered, half-size floating placement policy."""
    width = min(box["width"], max(box["width"] / 2, minimum[0]))
    height = min(box["height"], max(box["height"] / 2, minimum[1]))
    return {
        "x": box["x"] + (box["width"] - width) / 2,
        "y": box["y"] + (box["height"] - height) / 2,
        "width": width,
        "height": height,
    }


def rectangles(tree: dict | None, box: dict, gap: float, minimums: dict) -> dict:
    """Assign independent window frames in O(nodes + windows), without recursion."""
    if tree is None:
        return {}
    required = measure_minimums(tree, minimums, gap)
    frames = {}
    pending = [(tree, box)]
    while pending:
        node, frame = pending.pop()
        if "windows" in node:
            for number in node["windows"]:
                frames[number] = dict(frame)
            continue
        first, second = split_frame(node, frame, gap, required)
        # LIFO: visit a first to preserve frame output order.
        pending.append((node["b"], second))
        pending.append((node["a"], first))
    return frames


def measure_minimums(tree: dict | None, minimums: dict, gap: float) -> dict:
    """Build a request-local table; children must be measured before their parent."""
    required = {}
    pending = [(tree, False)] if tree is not None else []
    while pending:
        node, children_measured = pending.pop()
        if "windows" in node:
            required[id(node)] = leaf_minimum(node, minimums)
        elif not children_measured:
            pending.append((node, True))
            pending.append((node["b"], False))
            pending.append((node["a"], False))
        else:
            first = required[id(node["a"])]
            second = required[id(node["b"])]
            if node["axis"] == "horizontal":
                required[id(node)] = (first[0] + second[0] + gap, max(first[1], second[1]))
            else:
                required[id(node)] = (max(first[0], second[0]), first[1] + second[1] + gap)
    return required


def leaf_minimum(leaf: dict, minimums: dict) -> tuple[float, float]:
    """A stack must accommodate the largest width and height among its members."""
    members = iter(leaf["windows"])
    width, height = minimums.get(next(members), (0, 0))
    for number in members:
        member_width, member_height = minimums.get(number, (0, 0))
        width = max(width, member_width)
        height = max(height, member_height)
    return width, height


def split_frame(node: dict, frame: dict, gap: float, required: dict) -> tuple[dict, dict]:
    """Split a frame; overlap inside the display when minimum sizes cannot fit."""
    if node["axis"] == "horizontal":
        size_key, position_key, dimension = "width", "x", 0
    else:
        size_key, position_key, dimension = "height", "y", 1
    length = frame[size_key]
    inner = max(2, length - gap)
    first_minimum = required[id(node["a"])][dimension]
    second_minimum = required[id(node["b"])][dimension]
    first_size = max(1, round(inner * node["ratio"]))
    if first_minimum + second_minimum <= inner:
        first_size = max(first_minimum, min(first_size, inner - second_minimum))
        second_size = inner - first_size
    else:
        first_size = min(length, max(first_size, first_minimum))
        second_size = min(length, max(inner - first_size, second_minimum))
    first, second = dict(frame), dict(frame)
    first[size_key] = max(1, first_size)
    second[size_key] = max(1, second_size)
    second[position_key] = frame[position_key] + length - second[size_key]
    return first, second


def neighbour(tree: dict | None, frames: dict, number: int, direction: str) -> int | None:
    """Choose the nearest eligible active window, breaking distance ties by ID."""
    if number not in frames or direction not in DIRECTIONS:
        return None
    current = frames[number]
    best = None
    for leaf in leaves(tree):
        if number in leaf["windows"]:
            continue
        other = leaf["active"]
        frame = frames[other]
        # Keep the original evaluation order for fractional coordinates.
        dx = frame["x"] + frame["width"] / 2 - current["x"] - current["width"] / 2
        dy = frame["y"] + frame["height"] / 2 - current["y"] - current["height"] / 2
        if is_in_direction(dx, dy, direction):
            candidate = (dx * dx + dy * dy, other)
            if best is None or candidate < best:
                best = candidate
    return best[1] if best is not None else None


def is_in_direction(dx: float, dy: float, direction: str) -> bool:
    if direction == "left":
        return dx < 0 and abs(dy) <= abs(dx)
    if direction == "right":
        return dx > 0 and abs(dy) <= abs(dx)
    if direction == "up":
        return dy < 0 and abs(dx) <= abs(dy)
    if direction == "down":
        return dy > 0 and abs(dx) <= abs(dy)
    return False


# -- Tree operations: iterative walks; original JSON node representation --------
# A leaf stores {"windows": [IDs...], "active": ID}.
# A branch stores {"axis": "horizontal" or "vertical", "ratio": fraction,
#                  "a": child_node, "b": child_node}.


def leaves(tree: dict | None):
    """Yield leaves in a-before-b order without constructing a leaf list."""
    pending = [tree] if tree is not None else []
    while pending:
        node = pending.pop()
        if "windows" in node:
            yield node
        else:
            pending.append(node["b"])
            pending.append(node["a"])


def locate_leaf(tree: dict | None,
                number: int | None) -> tuple[dict, dict | None, str | None] | None:
    """Return (leaf, parent, side); a root leaf has no parent or side."""
    pending = [(tree, None, None)] if tree is not None else []
    while pending:
        node, parent, side = pending.pop()
        if "windows" in node:
            if number in node["windows"]:
                return node, parent, side
        else:
            pending.append((node["b"], node, "b"))
            pending.append((node["a"], node, "a"))
    return None


def prune_tree(tree: dict | None, present: set[int]) -> dict | None:
    """Remove absent windows, then collapse branches whose children disappeared."""
    root = {"a": tree}
    pending = [(root, "a", False)]
    while pending:
        parent, side, children_pruned = pending.pop()
        node = parent[side]
        if node is None:
            continue
        if "windows" in node:
            parent[side] = prune_leaf(node, present)
        elif not children_pruned:
            pending.append((parent, side, True))
            pending.append((node, "b", False))
            pending.append((node, "a", False))
        elif node["a"] is None:
            parent[side] = node["b"]
        elif node["b"] is None:
            parent[side] = node["a"]
    return root["a"]


def prune_leaf(leaf: dict, present: set[int]) -> dict | None:
    leaf["windows"] = [number for number in leaf["windows"] if number in present]
    if not leaf["windows"]:
        return None
    if leaf.get("active") not in leaf["windows"]:
        leaf["active"] = leaf["windows"][0]
    return leaf


def insert_window(tree: dict | None, target: int | None,
                  number: int, frames: dict) -> tuple[dict | None, dict | None]:
    """Split a target leaf; return (new root, inserted leaf) without copying nodes."""
    if tree is None:
        leaf = {"windows": [number], "active": number}
        return leaf, leaf
    location = locate_leaf(tree, target)
    if location is None:
        return tree, None
    target_leaf, parent, side = location
    frame = frames[target]
    leaf = {"windows": [number], "active": number}
    branch = {
        "axis": "horizontal" if frame["width"] >= frame["height"] else "vertical",
        "ratio": 0.5,
        "a": target_leaf,
        "b": leaf,
    }
    if parent is None:
        return branch, leaf
    parent[side] = branch
    return tree, leaf


# -- Output and resident JSON-lines entry point --------------------------------


def build_response(state: dict, frames: dict, focus: int | None) -> dict:
    response = {
        "frames": [
            {"number": number, "frame": {key: round(value) for key, value in frame.items()}}
            for number, frame in frames.items()
        ],
        "state": state,
        "unmanaged": list(state["floating"]),
    }
    if focus is not None:
        response["focus"] = focus
    return response


def main() -> None:
    for line in sys.stdin:
        if line.strip():
            print(json.dumps(render(json.loads(line))), flush=True)


if __name__ == "__main__":
    main()
