-- =============================================================================
-- Rex config — ported from ~/.config/tmux/tmux.conf (+ Ghostty keybinds)
--
-- Reload:   rex config reload   (or prefix + r)
-- Validate: rex config check
-- Inspect:  rex keymap
-- =============================================================================

local TERMINAL = "com.superlogical.terminal"
local SHELL = "com.superlogical.terminal.shell"

-- tmux: set -g prefix C-a
local PREFIX = "ctrl+a"

--- Build a prefix sequence, e.g. pre("h") -> "ctrl+a>h"
local function pre(key)
  return PREFIX .. ">" .. key
end

-- =============================================================================
-- Helpers
-- =============================================================================

--- Working directory the focused terminal last reported (OSC 7), or $HOME.
local function focused_cwd(ctx)
  local r = rex.block.call(TERMINAL, "pwd", { block_id = ctx.block_id })
  local pwd = r and r.pwd
  if not pwd or pwd == "" then
    return os.getenv("HOME")
  end
  -- OSC 7 reports file://host/path with percent-encoding
  pwd = pwd:gsub("^file://[^/]*", "")
  pwd = pwd:gsub("%%(%x%x)", function(hex)
    return string.char(tonumber(hex, 16))
  end)
  return pwd
end

-- =============================================================================
-- Custom actions
-- =============================================================================

-- tmux: display-popup -E -w W -h H -d "#{pane_current_path}" CMD
-- Opens CMD in a centered floating layer; it closes when CMD exits.
rex.action{
  name = "popup",
  title = "Open Popup",
  category = "Pane",
  args = { command = "array", w = "number?", h = "number?" },
  run = function(ctx, args)
    local w, h = args.w or 0.8, args.h or 0.8
    local _, err = rex.session.new_layer{
      bounds = { (1 - w) / 2, (1 - h) / 2, w, h },
      focus = true,
      layout = rex.layout.block{
        flavor = SHELL,
        options = { command = args.command, cwd = focused_cwd(ctx) },
      },
    }
    if err ~= nil then error(err, 0) end
  end,
}

-- tmux: bind -r H resize-pane -L 5
-- Resizes once, then stays in "resize" mode so H/J/K/L keep repeating
-- without the prefix (like tmux's repeat-time). Esc / Enter / q to leave.
rex.action{
  name = "resize_repeat",
  title = "Resize Pane (repeat)",
  category = "Pane",
  args = { direction = "string" },
  run = function(_, args)
    rex.client.queue("pane.resize", { direction = args.direction })
    rex.client.queue("client.mode.enter", { name = "resize" })
  end,
}

-- =============================================================================
-- Prefix
-- =============================================================================

-- tmux: bind C-a send-prefix
rex.bind(pre("ctrl+a"), "pane.send_key", { key = "ctrl+a" })

-- =============================================================================
-- Panes
-- =============================================================================

-- Navigation (vim)
rex.bind(pre("h"), "pane.focus.left")
rex.bind(pre("j"), "pane.focus.down")
rex.bind(pre("k"), "pane.focus.up")
rex.bind(pre("l"), "pane.focus.right")

-- Resize (repeatable)
rex.bind(pre("shift+h"), "resize_repeat", { direction = "left" })
rex.bind(pre("shift+j"), "resize_repeat", { direction = "down" })
rex.bind(pre("shift+k"), "resize_repeat", { direction = "up" })
rex.bind(pre("shift+l"), "resize_repeat", { direction = "right" })

rex.mode("resize", { exclusive = true })
for key, dir in pairs({ h = "left", j = "down", k = "up", l = "right" }) do
  rex.bind("resize/" .. key, "pane.resize", { direction = dir })
  rex.bind("resize/shift+" .. key, "pane.resize", { direction = dir })
end
rex.bind("resize/=", "pane.balance")
rex.bind("resize/escape", "client.mode.exit")
rex.bind("resize/enter", "client.mode.exit")
rex.bind("resize/q", "client.mode.exit")

-- Splits (tmux: '-' = split below, '%' = split right)
rex.bind(pre("-"), "pane.split", { direction = "down" })
rex.bind(pre("shift+5"), "pane.split", { direction = "right" })

rex.bind(pre("="), "pane.balance")             -- select-layout -E
rex.bind(pre("z"), "pane.zoom")                -- tmux default
rex.bind(pre("x"), "pane.close")               -- tmux default
rex.bind(pre("shift+1"), "pane.move_to_new_tab") -- '!' break-pane

-- =============================================================================
-- Tabs (tmux windows)
-- =============================================================================

rex.bind(pre("c"), "client.tab.new")
rex.bind(pre("n"), "client.tab.next")
rex.bind(pre("p"), "client.tab.previous")
rex.bind(pre("tab"), "client.tab.previous")    -- closest to last-window
rex.bind(pre("ctrl+left"), "client.tab.move.backward")
rex.bind(pre("ctrl+right"), "client.tab.move.forward")

-- base-index 1: Rex tabs also count from 1
for i = 1, 9 do
  rex.bind(pre(tostring(i)), "client.tab.goto", { index = i })
end

-- =============================================================================
-- Sessions
-- =============================================================================

rex.bind(pre("o"), "session.switch")           -- was: hetki popup
rex.bind(pre("shift+tab"), "session.previous") -- closest to switch-client -l
rex.bind(pre("shift+s"), "session.new")
rex.bind(pre("shift+x"), "session.close")

-- =============================================================================
-- Popups
-- =============================================================================

rex.bind(pre("g"), "popup", { command = { "zsh", "-i", "-c", "gdiff" }, w = 1, h = 1 })
rex.bind(pre("y"), "popup", { command = { "yazi" }, w = 0.8, h = 0.8 })
rex.bind(pre("f"), "pane.insert_path")         -- Rex-native file picker (was fzp)

-- =============================================================================
-- Misc
-- =============================================================================

rex.bind(pre("r"), "client.config.reload")
rex.bind(pre("ctrl+s"), "client.sidebar.toggle") -- closest to toggling status bar
rex.bind("alt+c", "client.sidebar.toggle")       -- collapse/expand the vertical tabs
rex.bind("alt+shift+c", "client.tab_style.toggle") -- switch tabs: top bar <-> sidebar

-- From Ghostty: cmd+p sent "prefix o" to tmux -> go straight to the switcher
rex.bind("cmd+p", "session.switch")

-- Ghostty: theme = light:Catppuccin Latte,dark:Catppuccin Mocha
-- Imports both themes from Ghostty's bundle (Mocha last, so it ends up selected).
local GHOSTTY_THEMES = "/Applications/Ghostty.app/Contents/Resources/ghostty/themes/"
rex.action{
  name = "import_ghostty_themes",
  title = "Import Ghostty Catppuccin Themes",
  category = "Appearance",
  run = function()
    rex.client.queue("client.theme.import", { path = GHOSTTY_THEMES .. "Catppuccin Latte" })
    rex.client.queue("client.theme.import", { path = GHOSTTY_THEMES .. "Catppuccin Mocha" })
  end,
}
rex.bind(pre("shift+t"), "import_ghostty_themes")
