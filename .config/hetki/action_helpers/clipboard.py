"""Copy a selected value using the current desktop's clipboard tool."""

import os
import shutil
import subprocess
import sys


def clipboard_command():
    if sys.platform == "darwin" and shutil.which("pbcopy"):
        return ["pbcopy"]
    if os.environ.get("WAYLAND_DISPLAY") and shutil.which("wl-copy"):
        return ["wl-copy"]
    if os.environ.get("DISPLAY"):
        if shutil.which("xclip"):
            return ["xclip", "-selection", "clipboard"]
        if shutil.which("xsel"):
            return ["xsel", "--clipboard", "--input"]
    raise RuntimeError("No clipboard tool for this desktop: use pbcopy on macOS, wl-copy on Wayland, or xclip/xsel on X11")


def copy(text):
    subprocess.run(clipboard_command(), input=text, text=True, check=True, timeout=5)


if __name__ == "__main__":
    try:
        if len(sys.argv) != 2:
            raise ValueError("Expected one value to copy")
        copy(sys.argv[1])
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
