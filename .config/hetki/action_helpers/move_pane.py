"""Move the selected pane within its session or into another session."""

from __future__ import annotations

import shlex

from _hetki import Request, emit, entrypoint, parse_invocation, tmux


def source_location(request: Request) -> tuple[str, str, str]:
    """Return the subject pane's (session_id, window_id, pane_id).

    Hetki resolves the subject when the flow opens and re-checks it before the
    final run, so the IDs in the request are authoritative.
    """
    subject = request["context"]["subject"]
    if not subject.get("session_id"):
        raise RuntimeError(
            "The pane's session is unknown (it may belong to multiple sessions); "
            "select it in Hetki's session tree"
        )
    return subject["session_id"], subject["window_id"], subject["pane_id"]


def named_rows(request: Request, command: str, *args: str, fields: int) -> list[list[str]]:
    tokens = shlex.split(tmux(request, command, *args))
    if len(tokens) % fields:
        raise RuntimeError("Unexpected tmux list response")
    return [tokens[i:i + fields] for i in range(0, len(tokens), fields)]


def list_sessions(request: Request) -> None:
    source, _, _ = source_location(request)
    for session, name in named_rows(
        request, "list-sessions", "-F", "#{session_id} name=#{q:session_name}", fields=2,
    ):
        if session != source:
            emit(session, name.removeprefix("name="), {"session_id": session})


def destination_session(request: Request, source: str) -> str:
    choice = request.get("steps", {}).get("session")
    return choice["value"]["session_id"] if choice else source


def list_windows(request: Request) -> None:
    source, source_window, _ = source_location(request)
    session = destination_session(request, source)
    windows = named_rows(
        request, "list-windows", "-t", session + ":", "-F",
        "#{window_id} #{pane_id} name=#{window_index}:#{q:window_name}", fields=3,
    )
    emit("new-window", "New window", {"session_id": session, "new_window": True})
    for window, pane, name in windows:
        if window != source_window:
            emit(window, name.removeprefix("name="), {
                "session_id": session, "window_id": window, "pane_id": pane,
            })


def invoking_client(request: Request, destination: str) -> str | None:
    invocation = request["context"].get("invocation")
    if not invocation or invocation["session_id"] == destination:
        return None

    clients = named_rows(
        request, "list-clients", "-t", invocation["session_id"], "-F",
        "#{q:client_name} #{pane_id} #{client_control_mode}", fields=3,
    )
    matches = [
        name for name, pane, control in clients
        if pane == invocation["pane_id"] and control == "0"
    ]
    if len(matches) != 1:
        raise RuntimeError("Cannot identify one invoking tmux client; the pane was not moved")
    return matches[0]


def preserve_client(request: Request, client: str | None, session: str, pane: str, destination: str) -> None:
    if not client or request["context"]["invocation"]["session_id"] != session:
        return

    panes = tmux(request, "list-panes", "-s", "-t", session, "-F", "#{pane_id}")
    if panes.splitlines() == [pane]:
        # Moving the last pane destroys its session and would detach this client.
        tmux(request, "switch-client", "-c", client, "-t", destination)


def move(request: Request) -> None:
    session, window, pane = source_location(request)
    choice = request["steps"]["window"]["value"]
    new_window = choice.get("new_window", False)
    destination = destination_session(request, session)
    if choice["session_id"] != destination:
        raise RuntimeError("The selected window belongs to a different destination session")
    if not new_window and choice["window_id"] == window:
        raise RuntimeError("Choose a different window")

    client = invoking_client(request, destination)
    preserve_client(request, client, session, pane, destination)
    source = f"{session}:{window}.{pane}"
    if new_window:
        window = tmux(
            request, "break-pane", "-d", "-P", "-F", "#{window_id}",
            "-s", source, "-t", destination + ":",
        )
    else:
        target = f"{destination}:{choice['window_id']}.{choice['pane_id']}"
        tmux(request, "join-pane", "-d", "-h", "-s", source, "-t", target)
        window = choice["window_id"]

    target = f"{destination}:{window}.{pane}"
    tmux(request, "select-pane", "-t", target)
    tmux(request, "select-window", "-t", target)
    if client:
        tmux(request, "switch-client", "-c", client, "-t", destination)


OPERATIONS = {"sessions": list_sessions, "windows": list_windows, "move": move}


def main() -> None:
    operation, request = parse_invocation(OPERATIONS)
    operation(request)


if __name__ == "__main__":
    entrypoint(main)
