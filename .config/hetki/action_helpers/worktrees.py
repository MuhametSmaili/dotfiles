"""JSON providers and commands for Hetki's local worktree actions.

Modes (first argument):
  worktrees  list worktrees to open (excludes the pane's own worktree)
  removable  list worktrees that can be removed (excludes main, own, locked)
  branches   list a new scratch branch, unused local branches and remote branches
  open       open a shell in the selected worktree
  create     create a worktree for the selected branch and open a shell
  remove     remove the selected worktree after confirmation (keeps the branch)
"""

from __future__ import annotations

import hashlib
import re
import uuid
from collections.abc import Iterator
from datetime import datetime
from pathlib import Path
from typing import Any

from _hetki import Request, emit, entrypoint, parse_invocation, run, subject_cwd, tmux

Entry = dict[str, str]

HEADS = "refs/heads/"
REMOTES = "refs/remotes/"


def repository(request: Request) -> str:
    return run("git", "-C", subject_cwd(request), "rev-parse", "--show-toplevel")


def worktrees(repo: str) -> list[Entry]:
    """Parse ``git worktree list --porcelain -z`` into one dict per worktree."""
    output = run("git", "-C", repo, "worktree", "list", "--porcelain", "-z")
    entries = []
    current: Entry = {}
    for field in output.split("\0"):
        if not field:
            if current:
                entries.append(current)
                current = {}
            continue
        key, _, value = field.partition(" ")
        current[key] = value
    if current:
        entries.append(current)
    return entries


def same_path(a: str | Path, b: str | Path) -> bool:
    return Path(a).resolve() == Path(b).resolve()


def short_branch(entry: Entry) -> str:
    return entry.get("branch", "detached HEAD").removeprefix(HEADS)


def digest(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()[:16]


def candidate_worktrees(repo: str, removable: bool) -> Iterator[Entry]:
    """Yield worktrees other than the pane's own, skipping bare and prunable ones.

    With ``removable``, also skip the main worktree (always the first porcelain
    entry) and locked ones.
    """
    for index, entry in enumerate(worktrees(repo)):
        if "bare" in entry or "prunable" in entry or same_path(entry["worktree"], repo):
            continue
        if removable and (index == 0 or "locked" in entry):
            continue
        yield entry


def is_candidate(repo: str, path: str, removable: bool) -> bool:
    return any(same_path(entry["worktree"], path) for entry in candidate_worktrees(repo, removable))


def list_worktrees(repo: str, removable: bool = False) -> None:
    for entry in candidate_worktrees(repo, removable):
        path = entry["worktree"]
        branch = short_branch(entry)
        emit(digest(path), branch, {"path": path, "branch": branch}, path)


def list_branches(repo: str) -> None:
    name = f"try/{datetime.now():%Y%m%d-%H%M%S}-{uuid.uuid4().hex[:4]}"
    emit("scratch", f"New scratch branch: {name}", {"branch": name, "create": True},
         "Create from this pane's current HEAD")

    occupied = {entry.get("branch") for entry in worktrees(repo)}
    # Full refnames throughout: a local branch named "origin/x" makes the short
    # name of refs/remotes/origin/x ambiguous, both for %(refname:short) and as
    # a start point. for-each-ref sorts by refname, so heads come first.
    refs = run("git", "-C", repo, "for-each-ref", "--format=%(refname)", HEADS, REMOTES).splitlines()
    local = {ref.removeprefix(HEADS) for ref in refs if ref.startswith(HEADS)}
    for ref in refs:
        if ref.startswith(HEADS):
            if ref not in occupied:
                branch = ref.removeprefix(HEADS)
                emit(digest(ref), branch, {"branch": branch, "create": False},
                     "Create a worktree for this existing local branch")
            continue
        remote_branch = ref.removeprefix(REMOTES)  # "<remote>/<branch>"
        _, _, branch = remote_branch.partition("/")
        if branch and branch != "HEAD" and branch not in local:
            emit(digest(ref), remote_branch, {"branch": branch, "create": True, "start": ref},
                 "Create a tracking branch and worktree")


def create_worktree(repo: str, choice: dict[str, Any]) -> str:
    branch = choice["branch"]
    run("git", "check-ref-format", "--branch", branch)
    common = Path(run("git", "-C", repo, "rev-parse", "--path-format=absolute", "--git-common-dir"))
    main = common.parent if common.name == ".git" else common
    slug = re.sub(r"[^A-Za-z0-9._-]+", "-", branch).strip(".-")[:60] or "branch"
    path = main.parent / f"{main.name}-worktrees" / f"{slug}-{digest(branch)[:8]}"

    # Re-running the action for a branch that already has this worktree just reopens it.
    for entry in worktrees(repo):
        if entry.get("branch") == HEADS + branch and same_path(entry["worktree"], path) and path.is_dir():
            return str(path)
    if path.exists():
        raise RuntimeError(f"Destination already exists: {path}")

    path.parent.mkdir(parents=True, exist_ok=True)
    args = ["git", "-C", repo, "worktree", "add"]
    if choice["create"]:
        start = choice.get("start", "HEAD")
        if start != "HEAD":
            args.append("--track")
        args += ["-b", branch, str(path), start]
    else:
        args += [str(path), branch]
    run(*args, timeout=None)  # checkout time scales with repo size
    return str(path)


def open_terminal(request: Request, path: str, name: str) -> None:
    pane = request["context"]["subject"]["pane_id"]
    choice = request["steps"]["placement"]
    options = choice.get("value", {})
    placement = options.get("kind", choice["id"])
    if placement == "window":
        session = tmux(request, "display-message", "-p", "-t", pane, "#{session_id}")
        created = tmux(request, "new-window", "-d", "-P", "-F", "#{pane_id}",
                       "-t", session + ":", "-c", path, "-n", name)
    elif placement in ("horizontal", "vertical"):
        flag = "-h" if placement == "horizontal" else "-v"
        created = tmux(request, "split-window", "-d", "-P", "-F", "#{pane_id}", flag, "-t", pane, "-c", path)
    else:
        raise RuntimeError("Unknown terminal placement")
    if options.get("focus", False):
        tmux(request, "select-window", "-t", created)
        tmux(request, "select-pane", "-t", created)


def panes_inside(request: Request, path: str) -> list[str]:
    root = Path(path).resolve()
    rows = tmux(request, "list-panes", "-a", "-F", "#{pane_id} #{pane_current_path}")
    inside = []
    for row in rows.splitlines():
        pane, _, cwd = row.partition(" ")
        # tmux reports an empty path for some panes; Path("") would resolve to our own cwd.
        if cwd and Path(cwd).resolve().is_relative_to(root):
            inside.append(pane)
    return inside


def open_worktree(request: Request, repo: str) -> None:
    choice = request["steps"]["worktree"]["value"]
    if not is_candidate(repo, choice["path"], removable=False):
        raise RuntimeError("Selected worktree is no longer available")
    open_terminal(request, choice["path"], choice["branch"])


def create_and_open(request: Request, repo: str) -> None:
    choice = request["steps"]["branch"]["value"]
    open_terminal(request, create_worktree(repo, choice), choice["branch"])


def remove_worktree(request: Request, repo: str) -> None:
    if request["steps"]["confirm"]["id"] != "remove":
        return  # Cancel: exit 0 so the flow closes without changes.
    path = request["steps"]["worktree"]["value"]["path"]
    if not is_candidate(repo, path, removable=True):
        raise RuntimeError("Selected worktree is no longer removable")
    inside = panes_inside(request, path)
    if inside:
        raise RuntimeError("Panes are still using this worktree: " + ", ".join(inside))
    # No --force: Git refuses worktrees with uncommitted or untracked changes.
    run("git", "-C", repo, "worktree", "remove", path, timeout=None)


OPERATIONS = {
    "worktrees": lambda _request, repo: list_worktrees(repo),
    "removable": lambda _request, repo: list_worktrees(repo, removable=True),
    "branches": lambda _request, repo: list_branches(repo),
    "open": open_worktree,
    "create": create_and_open,
    "remove": remove_worktree,
}


def main() -> None:
    operation, request = parse_invocation(OPERATIONS)
    operation(request, repository(request))


if __name__ == "__main__":
    entrypoint(main)
