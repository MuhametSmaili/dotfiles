"""Shared plumbing for Hetki action helpers.

Helpers are run as ``python3 <dir>/<helper>.py``, so this sibling module is
importable without packaging. Each helper reads one JSON request from stdin and
either prints JSON choice lines or performs a command.
"""

from __future__ import annotations

import json
import subprocess
import sys
from collections.abc import Callable, Mapping
from typing import Any, NoReturn, TypeVar

Request = dict[str, Any]
Operation = TypeVar("Operation", bound=Callable[..., Any])

TIMEOUT = 10  # seconds; generous for local git/tmux queries


def run(*args: str, timeout: float | None = TIMEOUT, errors: str = "strict") -> str:
    """Run a command and return its stdout without the trailing newline.

    ``timeout=None`` leaves the bound to the action's own Hetki timeout, for
    commands whose duration scales with repository size.
    """
    result = subprocess.run(
        args, text=True, encoding="utf-8", errors=errors,
        capture_output=True, timeout=timeout, check=False,
    )
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip() or f"{args[0]} failed")
    return result.stdout.rstrip("\n")


def tmux(request: Request, *args: str) -> str:
    return run("tmux", "-S", request["context"]["server"]["socket"], *args)


def subject_cwd(request: Request) -> str:
    """Return the pane directory Hetki snapshotted when the flow opened."""
    cwd = request["context"]["subject"].get("cwd")
    if not cwd:
        raise RuntimeError("The pane's directory is unknown")
    return cwd


def emit(identifier: str, label: str, value: Any, description: str | None = None) -> None:
    """Print one Hetki choice as a JSON line."""
    choice = {"id": identifier, "label": label, "value": value}
    if description is not None:
        choice["description"] = description
    print(json.dumps(choice))


def read_request() -> Request:
    return json.load(sys.stdin)


def parse_invocation(operations: Mapping[str, Operation]) -> tuple[Operation, Request]:
    """Validate the mode argument before reading the request from stdin."""
    if len(sys.argv) != 2 or sys.argv[1] not in operations:
        raise ValueError("Expected one argument: " + ", ".join(operations))
    return operations[sys.argv[1]], read_request()


def fail(message: str) -> NoReturn:
    print(message, file=sys.stderr)
    sys.exit(1)


def entrypoint(main: Callable[[], None]) -> None:
    """Run ``main`` and turn expected failures into a message and exit code 1."""
    try:
        main()
    except KeyError as error:
        fail(f"Missing field: {error}")
    except subprocess.TimeoutExpired as error:
        fail(f"Timed out after {error.timeout:g}s: {' '.join(map(str, error.cmd))}")
    except (ValueError, OSError, RuntimeError) as error:
        fail(str(error))
