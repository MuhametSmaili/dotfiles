# Hammerspoon

This is my [Hammerspoon](https://www.hammerspoon.org/) setup. I package the
reusable parts as **Spoons**, the Hammerspoon plugin format, so you can install
only the parts you want. The other files are personal modules. Use the ones you
find useful.

## Spoons

Each Spoon is self-contained and documented. You can install each one on its
own.

| Spoon         | What it does                                                                                                  | Docs                                       |
| ------------- | ------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| **GridMouse** | Controls the mouse from the keyboard. A grid places the cursor, and the home row keys move it and click it.   | [README](Spoons/GridMouse.spoon/README.md) |
| **Leader**    | Shows a leader-key menu in a heads-up display (HUD), similar to which-key. You define the groups and actions. | [README](Spoons/Leader.spoon/README.md)    |
| **Awake**     | Keeps macOS awake, and moves the cursor a short distance when the computer is idle. Adds a menubar toggle.    | [README](Spoons/Awake.spoon/README.md)     |
| **CleanMode** | Blocks keyboard and pointer input behind a full-screen overlay so you can safely clean your devices.          | [README](Spoons/CleanMode.spoon/README.md) |

## Install one Spoon

You do not need the rest of this configuration to use one Spoon.

If you are new to Hammerspoon, install it first and give it permissions. See the
[Hammerspoon Getting Started guide](https://www.hammerspoon.org/go/) and the
[Hammerspoon project on GitHub](https://github.com/Hammerspoon/hammerspoon).

To install a Spoon:

1. Find your Spoons folder. The default folder is `~/.hammerspoon/Spoons/`. If
   you use a custom configuration path, use the `Spoons/` folder next to your
   `init.lua`.
2. Copy the `<Name>.spoon` folder into that Spoons folder.
3. Add the load and bind lines to your `init.lua`.
4. Reload your configuration. Open the Hammerspoon menubar, then select Reload
   Config.

This example loads and binds the Spoons:

```lua
hs.loadSpoon("GridMouse")
spoon.GridMouse:bindHotkeys({ start = { { "ctrl", "alt", "shift" }, "g" } })

hs.loadSpoon("Awake")
spoon.Awake:bindHotkeys({ toggle = { { "ctrl", "alt", "shift" }, "c" } })

hs.loadSpoon("CleanMode")
spoon.CleanMode:bindHotkeys({ toggle = { { "ctrl", "alt", "shift" }, "n" } })

hs.loadSpoon("Leader")
spoon.Leader.create({ "ctrl", "alt", "shift" }, "space", {
  r = { label = "Reload", action = hs.reload },
  o = { label = "Open", group = {
    g = { label = "Ghostty", action = spoon.Leader.app("Ghostty") },
  } },
})
```

For the full options of each Spoon, read its README. See the links in the table
above.

## Other modules

The other `.lua` files in this folder are my personal modules. The
[`init.lua`](init.lua) file connects them. I did not package them as Spoons,
because they depend on my apps and my workflow. They are small, and you can copy
them:

- `appdeck.lua` — an app launcher grid (Meh+Tab)
- `bing.lua` — a daily Bing wallpaper, with a region picker
- `capsmeh.lua` — Caps Lock acts as Meh when you hold it, and as Escape when you tap it
- `ghostty.lua` — opens a Ghostty window in a directory
- `key-caster.lua` — shows each keystroke on screen, which helps for screen recordings
- `markdown-paste.lua` — converts copied Markdown to rich text with Pandoc and pastes it (Meh+V)
- `mic.lua` — mutes and unmutes the microphone
- `ssh.lua` — lists the hosts in `~/.ssh/config`, and opens one in a terminal

The Leader menu (Meh+Space) starts most of these. "Meh" means
`ctrl + alt + shift`.

### Paste Markdown as rich text

Copy Markdown, focus the destination editor, then press **Meh+V** (hold Caps Lock
and press V). Release the keys to paste formatted text. Normal **Cmd+V** is unchanged.

Requires Pandoc 3.13 or newer in `/opt/homebrew/bin` or `/usr/local/bin`. The
`markdown-paste-writer.lua` companion uses Pandoc's GitHub-flavored Markdown
parser to produce HTML and a Markdown-free plain-text fallback in one local
process. Bold, italics, headings, lists, links, tables and code blocks are
converted; the receiving app decides which formatting it preserves. Teams
formatting needs an in-app check, particularly tables and code blocks.

The converted rich text stays on the clipboard. Images become their alt text;
raw HTML is treated as text, and only HTTP, HTTPS and mailto links are retained.
Conversion does not fetch resources, use the network or write clipboard content
to temporary files. It never presses Enter or sends a message.

Conversion is asynchronous, accepts up to 1 MiB of text and times out after ten
seconds, including waiting for modifier keys to be released. Changing the
clipboard, foreground app or focused window cancels the operation before it
writes or pastes. Moving between editors within the same window is not detected.
Errors leave the clipboard unchanged unless the clipboard write itself fails.

## Notes

- These Spoons are part of this dotfiles repository.
  [GNU Stow](https://www.gnu.org/software/stow/) links them into the live
  configuration, so `install.sh` installs them on every machine.
- Each Spoon uses the MIT license. See each README.

## Disclaimer

I built these modules with the help of AI. Read the code before you use it, and
use it at your own risk.
