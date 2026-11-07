"""JSON providers and commands for Hetki's local worktree actions."""

import hashlib
import json
import re
import subprocess
import sys
import uuid
from datetime import datetime
from pathlib import Path


def run(*args):
    result = subprocess.run(args, text=True, capture_output=True, check=False)
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip() or f"{args[0]} failed")
    return result.stdout.rstrip("\n")


def tmux(request, *args):
    return run("tmux", "-S", request["context"]["server"]["socket"], *args)


def repository(request):
    pane = request["context"]["subject"]["pane_id"]
    cwd = tmux(request, "display-message", "-p", "-t", pane, "#{pane_current_path}")
    return run("git", "-C", cwd, "rev-parse", "--show-toplevel")


def worktrees(repo):
    output = run("git", "-C", repo, "worktree", "list", "--porcelain", "-z")
    entries = []
    current = {}
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


def emit(identifier, label, description, value):
    print(json.dumps({"id": identifier, "label": label, "description": description, "value": value}))


def list_worktrees(repo):
    for entry in worktrees(repo):
        if "bare" in entry or "prunable" in entry:
            continue
        path = entry["worktree"]
        branch = entry.get("branch", "detached HEAD").removeprefix("refs/heads/")
        emit(hashlib.sha256(path.encode()).hexdigest()[:16], branch, path, {"path": path})


def list_branches(repo):
    name = "try/" + datetime.now().strftime("%Y%m%d-%H%M%S") + "-" + uuid.uuid4().hex[:4]
    emit("scratch", "New scratch branch: " + name, "Create from this pane's current HEAD", {"branch": name, "create": True})
    occupied = {entry.get("branch") for entry in worktrees(repo)}
    branches = run("git", "-C", repo, "for-each-ref", "--format=%(refname)", "refs/heads/")
    for ref in branches.splitlines():
        if ref in occupied:
            continue
        branch = ref.removeprefix("refs/heads/")
        identifier = hashlib.sha256(ref.encode()).hexdigest()[:16]
        emit(identifier, branch, "Create a worktree for this existing local branch", {"branch": branch, "create": False})


def create_worktree(repo, choice):
    branch = choice["branch"]
    run("git", "check-ref-format", "--branch", branch)
    common = Path(run("git", "-C", repo, "rev-parse", "--path-format=absolute", "--git-common-dir"))
    main = common.parent if common.name == ".git" else common
    slug = re.sub(r"[^A-Za-z0-9._-]+", "-", branch).strip(".-")[:60] or "branch"
    suffix = hashlib.sha256(branch.encode()).hexdigest()[:8]
    path = main.parent / (main.name + "-worktrees") / (slug + "-" + suffix)
    for entry in worktrees(repo):
        if entry.get("worktree") == str(path) and entry.get("branch") == "refs/heads/" + branch:
            if path.is_dir():
                return str(path)
    if path.exists():
        raise RuntimeError(f"Destination already exists: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    args = ["git", "-C", repo, "worktree", "add"]
    if choice["create"]:
        args += ["-b", branch, str(path), "HEAD"]
    else:
        args += [str(path), branch]
    run(*args)
    return str(path)


def open_terminal(request, path):
    pane = request["context"]["subject"]["pane_id"]
    choice = request["steps"]["placement"]
    options = choice.get("value", {})
    placement = options.get("kind", choice["id"])
    if placement == "window":
        session = tmux(request, "display-message", "-p", "-t", pane, "#{session_id}")
        created = tmux(request, "new-window", "-d", "-P", "-F", "#{pane_id}", "-t", session + ":", "-c", path, "-n", Path(path).name)
    elif placement in ("horizontal", "vertical"):
        flag = "-h" if placement == "horizontal" else "-v"
        created = tmux(request, "split-window", "-d", "-P", "-F", "#{pane_id}", flag, "-t", pane, "-c", path)
    else:
        raise RuntimeError("Unknown terminal placement")
    if options.get("focus", False):
        tmux(request, "select-window", "-t", created)
        tmux(request, "select-pane", "-t", created)


def main():
    request = json.load(sys.stdin)
    repo = repository(request)
    mode = sys.argv[1]
    if mode == "worktrees":
        list_worktrees(repo)
    elif mode == "branches":
        list_branches(repo)
    elif mode == "open":
        path = request["steps"]["worktree"]["value"]["path"]
        if not any(entry.get("worktree") == path and "prunable" not in entry for entry in worktrees(repo)):
            raise RuntimeError("Selected worktree is no longer available")
        open_terminal(request, path)
    elif mode == "create":
        path = create_worktree(repo, request["steps"]["branch"]["value"])
        open_terminal(request, path)
    else:
        raise RuntimeError("Unknown worktree action")


if __name__ == "__main__":
    try:
        main()
    except (KeyError, ValueError, OSError, RuntimeError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
