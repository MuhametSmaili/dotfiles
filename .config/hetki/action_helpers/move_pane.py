"""Move the selected pane within its session or into another session."""

import json
import shlex
import subprocess
import sys


def tmux(request, *args):
    result = subprocess.run(
        ["tmux", "-S", request["context"]["server"]["socket"], *args],
        text=True, capture_output=True, timeout=5, check=False,
    )
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or "tmux command failed")
    return result.stdout.rstrip("\n")


def source_location(request):
    subject = request["context"]["subject"]
    rows = tmux(request, "list-panes", "-a", "-F", "#{session_id} #{window_id} #{pane_id}")
    matches = []
    for line in rows.splitlines():
        session, window, pane = line.split()
        if pane != subject["pane_id"]:
            continue
        if subject.get("session_id") and session != subject["session_id"]:
            continue
        if window != subject["window_id"]:
            continue
        matches.append((session, window, pane))
    if not matches:
        raise RuntimeError("The source pane moved or no longer exists; reopen the action")
    if len(matches) != 1:
        raise RuntimeError("The pane belongs to multiple sessions; select it in Hetki's session tree")
    return matches[0]


def named_rows(request, command, *args, fields):
    output = tmux(request, command, *args)
    tokens = shlex.split(output)
    if len(tokens) % fields:
        raise RuntimeError("Unexpected tmux list response")
    return [tokens[i:i + fields] for i in range(0, len(tokens), fields)]


def emit(identifier, label, value):
    print(json.dumps({"id": identifier, "label": label, "value": value}))


def list_sessions(request):
    source, _, _ = source_location(request)
    for session, name in named_rows(
        request, "list-sessions", "-F", "#{session_id} name=#{q:session_name}", fields=2,
    ):
        if session != source:
            emit(session, name.removeprefix("name="), {"session_id": session})


def destination_session(request, source):
    choice = request.get("steps", {}).get("session")
    return choice["value"]["session_id"] if choice else source


def list_windows(request):
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


def invoking_client(request, destination):
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


def preserve_client(request, client, session, pane, destination):
    if not client or request["context"]["invocation"]["session_id"] != session:
        return

    panes = tmux(request, "list-panes", "-s", "-t", session, "-F", "#{pane_id}")
    if panes.splitlines() == [pane]:
        # Moving the last pane destroys its session and would detach this client.
        tmux(request, "switch-client", "-c", client, "-t", destination)


def move(request):
    session, window, pane = source_location(request)
    choice = request["steps"]["window"]["value"]
    destination = destination_session(request, session)
    if choice["session_id"] != destination:
        raise RuntimeError("The selected window belongs to a different destination session")

    if not choice.get("new_window", False) and choice["window_id"] == window:
        raise RuntimeError("Choose a different window")

    client = invoking_client(request, destination)
    preserve_client(request, client, session, pane, destination)
    source = f"{session}:{window}.{pane}"
    if choice.get("new_window", False):
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


def main():
    operations = {"sessions": list_sessions, "windows": list_windows, "move": move}
    if len(sys.argv) != 2 or sys.argv[1] not in operations:
        raise ValueError("Expected sessions, windows, or move")
    operations[sys.argv[1]](json.load(sys.stdin))


if __name__ == "__main__":
    try:
        main()
    except (KeyError, ValueError, OSError, RuntimeError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
