"""List recent commits from the selected pane's repository as Hetki choices."""

from __future__ import annotations

from _hetki import emit, entrypoint, read_request, run, subject_cwd

LIMIT = 1000


def main() -> None:
    cwd = subject_cwd(read_request())
    output = run(
        "git", "--no-pager", "-C", cwd, "log", f"--max-count={LIMIT}",
        "--no-color", "--no-decorate", "--no-show-signature", "--no-notes",
        "--format=%H%x09%h%x09%cs%x09%<(200,trunc)%s", "HEAD", "--",
        errors="replace",  # commit subjects are not guaranteed to be UTF-8
    )
    # Split on "\n" only: str.splitlines() also breaks on control characters
    # that may appear inside a subject.
    for line in output.split("\n"):
        if not line:
            continue
        commit, short, date, subject = line.split("\t", 3)
        emit(commit, f"{short}  {date}  {subject.rstrip()}", {"hash": commit})


if __name__ == "__main__":
    entrypoint(main)
