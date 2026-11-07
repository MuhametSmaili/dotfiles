"""List recent commits from the selected pane's repository as Hetki choices."""

import json
import subprocess
import sys


LIMIT = 1000


def run(*args):
    result = subprocess.run(
        args, text=True, encoding="utf-8", errors="replace",
        capture_output=True, timeout=10, check=False,
    )
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or f"{args[0]} failed")
    return result.stdout


def main():
    request = json.load(sys.stdin)
    context = request["context"]
    cwd = run(
        "tmux", "-S", context["server"]["socket"], "display-message",
        "-p", "-t", context["subject"]["pane_id"], "#{pane_current_path}",
    ).rstrip("\n")
    output = run(
        "git", "--no-pager", "-C", cwd, "log", f"--max-count={LIMIT}",
        "--no-color", "--no-decorate", "--no-show-signature", "--no-notes",
        "--format=%H%x09%h%x09%cs%x09%<(200,trunc)%s", "HEAD", "--",
    )
    for line in output.split("\n"):
        if not line:
            continue
        commit, short, date, subject = line.split("\t", 3)
        print(json.dumps({
            "id": commit,
            "label": f"{short}  {date}  {subject.rstrip()}",
            "value": {"hash": commit},
        }))


if __name__ == "__main__":
    try:
        main()
    except (KeyError, ValueError, OSError, RuntimeError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
