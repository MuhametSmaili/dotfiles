--- === Leader ===
---
--- A leader-key modal with a HUD, in the spirit of which-key. Define a menu
--- tree, press the leader hotkey, then keys to navigate groups or trigger
--- actions. Auto-exits on timeout or unknown key.
---
--- The menu tree is supplied by you via `:create()`. Each entry is either an
--- action (`{ label = "...", action = fn }`) or a group
--- (`{ label = "...", group = { ... } }`). Groups nest arbitrarily.
---
--- Example:
--- ```lua
--- hs.loadSpoon("Leader")
--- spoon.Leader.create({ "ctrl", "alt", "shift" }, "space", {
---   r = { label = "Reload", action = hs.reload },
---   o = { label = "Open", group = {
---     g = { label = "Ghostty", action = spoon.Leader.app("Ghostty") },
---   } },
--- }, 2.5)
--- ```
---
--- Download: https://github.com/MuhametSmaili/dotfiles

local obj = {}
obj.__index = obj

-- Metadata
obj.name = "Leader"
obj.version = "1.0"
obj.author = "MuhametSmaili"
obj.homepage = "https://github.com/MuhametSmaili/dotfiles"
obj.license = "MIT - https://opensource.org/licenses/MIT"

-- Style ----------------------------------------------------------------------

local style = {
	bg = { hex = "#111827", alpha = 0.88 },
	key_bg = { hex = "#60A5FA", alpha = 0.15 },
	key_color = { hex = "#93C5FD", alpha = 1 },
	label = { hex = "#E2E8F0", alpha = 1 },
	dim = { hex = "#64748B", alpha = 1 },
	font = "SF Pro Text",
	font_bold = "SF Pro Text Bold",
	size = 16,
	key_size = 15,
	row_h = 32,
	pad_x = 28,
	pad_y = 20,
	key_w = 28,
	key_gap = 14,
	radius = 14,
	min_w = 220,
}

--- Leader.style
--- Variable
--- Table of HUD style options (colors, fonts, sizing). Override fields after
--- loading the Spoon to customize the appearance.
obj.style = style

-- HUD ------------------------------------------------------------------------

local hud = nil
local measure_canvas = nil

local function destroy_hud()
	if hud then
		hud:delete()
		hud = nil
	end
end

local function measure_width(lines)
	if not measure_canvas then
		measure_canvas = hs.canvas.new({ x = 0, y = 0, w = 8, h = 8 })
	end
	local w = style.min_w
	for _, l in ipairs(lines) do
		local text = l.label .. (l.group and "  →" or "")
		local st = hs.styledtext.new(text, { font = { name = style.font, size = style.size } })
		local sz = measure_canvas:minimumTextSize(st)
		if sz then
			local lw = style.pad_x * 2 + style.key_w + style.key_gap + math.ceil(sz.w)
			if lw > w then
				w = lw
			end
		end
	end
	return w
end

local function build_row(line, y, w)
	local label_x = style.pad_x + style.key_w + style.key_gap
	return {
		{
			type = "rectangle",
			action = "fill",
			fillColor = style.key_bg,
			roundedRectRadii = { xRadius = 6, yRadius = 6 },
			frame = { x = style.pad_x - 2, y = y + 3, w = style.key_w, h = style.row_h - 6 },
		},
		{
			type = "text",
			text = hs.styledtext.new(line.key, {
				font = { name = style.font_bold, size = style.key_size },
				color = style.key_color,
				paragraphStyle = { alignment = "center" },
			}),
			frame = {
				x = style.pad_x - 2,
				y = y + (style.row_h - style.key_size) / 2 - 1,
				w = style.key_w,
				h = style.key_size + 4,
			},
		},
		{
			type = "text",
			text = hs.styledtext.new(line.label .. (line.group and "  →" or ""), {
				font = { name = style.font, size = style.size },
				color = line.group and style.dim or style.label,
			}),
			frame = {
				x = label_x,
				y = y + (style.row_h - style.size) / 2 - 1,
				w = w - label_x - style.pad_x,
				h = style.size + 4,
			},
		},
	}
end

local function collect_lines(menu)
	local lines = {}
	for k, v in pairs(menu) do
		lines[#lines + 1] = { key = k:upper(), label = v.label or "", group = v.group }
	end
	-- actions before groups, alphabetical within each
	table.sort(lines, function(a, b)
		if (a.group ~= nil) ~= (b.group ~= nil) then
			return a.group == nil
		end
		return a.key < b.key
	end)
	return lines
end

local function show_hud(menu)
	destroy_hud()

	local lines = collect_lines(menu)
	local w = measure_width(lines)
	local h = #lines * style.row_h + style.pad_y * 2
	local screen = hs.screen.mainScreen():frame()

	hud = hs.canvas.new({
		x = math.floor(screen.x + (screen.w - w) / 2),
		y = math.floor(screen.y + (screen.h - h) / 2),
		w = w,
		h = h,
	})
	hud:level(hs.canvas.windowLevels.overlay)
	hud:behavior({ "canJoinAllSpaces" })

	local els = {
		{
			type = "rectangle",
			action = "fill",
			fillColor = style.bg,
			roundedRectRadii = { xRadius = style.radius, yRadius = style.radius },
			frame = { x = 0, y = 0, w = w, h = h },
		},
	}
	for i, line in ipairs(lines) do
		for _, el in ipairs(build_row(line, style.pad_y + (i - 1) * style.row_h, w)) do
			els[#els + 1] = el
		end
	end

	hud:replaceElements(els):show()
end

-- Menu walking ---------------------------------------------------------------

local function collect_keys(menu, set)
	for k, v in pairs(menu) do
		set[k] = true
		if v.group then
			collect_keys(v.group, set)
		end
	end
	return set
end

-- Modal ----------------------------------------------------------------------

--- Leader.create(mods, key, root, timeout)
--- Function
--- Creates the leader modal bound to `mods`+`key`, driving the menu tree `root`.
---
--- Parameters:
---  * mods - A table of modifier keys (e.g. `{ "ctrl", "alt", "shift" }`)
---  * key - The leader key (e.g. `"space"`)
---  * root - The menu tree. Each value is `{ label, action }` or `{ label, group }`
---  * timeout - Optional seconds of inactivity before auto-exit (default 2)
---
--- Returns:
---  * The created `hs.hotkey.modal` object
function obj.create(mods, key, root, timeout)
	timeout = timeout or 2
	local modal = hs.hotkey.modal.new(mods, key)
	local timer, blocker
	local current = root

	local function exit()
		modal:exit()
	end

	local function reset_timer()
		if timer then
			timer:stop()
		end
		timer = hs.timer.doAfter(timeout, exit)
	end

	local function open(menu)
		current = menu
		show_hud(menu)
		reset_timer()
	end

	local function handle(k)
		local entry = current[k]
		if not entry then
			return exit()
		end
		if entry.group then
			open(entry.group)
		else
			exit()
			entry.action()
		end
	end

	function modal:entered()
		-- swallow unknown keys so they don't reach the focused app
		blocker = hs.eventtap
			.new({ hs.eventtap.event.types.keyDown }, function(e)
				local name = hs.keycodes.map[e:getKeyCode()]
				if not name then
					return true
				end
				if name == "escape" or current[name] then
					return false -- let the modal binding handle it
				end
				exit()
				return true
			end)
			:start()
		open(root)
	end

	function modal:exited()
		if timer then
			timer:stop()
			timer = nil
		end
		if blocker then
			blocker:stop()
			blocker = nil
		end
		destroy_hud()
		current = root
	end

	for k in pairs(collect_keys(root, {})) do
		modal:bind({}, k, function()
			handle(k)
		end)
	end
	modal:bind({}, "escape", exit)

	return modal
end

-- Action factories -----------------------------------------------------------

--- Leader.app(name)
--- Function
--- Returns an action that launches or focuses the app named `name`.
function obj.app(name)
	return function()
		hs.application.launchOrFocus(name)
	end
end

--- Leader.open(target, app)
--- Function
--- Returns an action that opens `target` (a URL/path), optionally with `app`.
function obj.open(target, app)
	local args = app and { "-a", app, target } or { target }
	return function()
		local t = hs.task.new("/usr/bin/open", nil, args)
		if t then
			t:start()
		else
			hs.alert.show("Failed to open: " .. target)
		end
	end
end

--- Leader.task(bin, args)
--- Function
--- Returns an action that runs the executable `bin` with the list `args`.
function obj.task(bin, args)
	local argv = {}
	for i, a in ipairs(args or {}) do
		argv[i] = tostring(a)
	end
	return function()
		local t = hs.task.new(bin, nil, argv)
		if t then
			t:start()
		else
			hs.alert.show("Failed to start: " .. bin)
		end
	end
end

return obj
