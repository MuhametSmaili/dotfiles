--- === GridMouse ===
---
--- Keyboard-driven mouse. A two-stage grid lets you place the cursor anywhere
--- without touching the trackpad, then nudge and click from the home row.
---
--- 10x10 main grid: two home-row keys pick a cell. Then a 2x5 sub-grid inside
--- that cell: one home-row key picks a sub-cell. Then `ikjl` to nudge,
--- space/enter to click (shift for right-click), `d` to double-click,
--- esc/backspace to exit.
---
--- Download: https://github.com/MuhametSmaili/dotfiles

local obj = {}
obj.__index = obj

-- Metadata
obj.name = "GridMouse"
obj.version = "1.0"
obj.author = "MuhametSmaili"
obj.homepage = "https://github.com/MuhametSmaili/dotfiles"
obj.license = "MIT - https://opensource.org/licenses/MIT"

--- GridMouse.freeMove
--- Variable
--- Controls what happens when you select a sub-cell. If `true` (the default),
--- GridMouse enters a free-move stage. In this stage the overlay disappears,
--- you move the cursor with the movement keys, and then you click. If `false`,
--- GridMouse clicks at once and exits. Hold `shift` for a right click.
obj.freeMove = true

--- GridMouse.movementKeys
--- Variable
--- The keys that move the cursor in the free-move stage. This is a table with
--- `up`, `down`, `left`, and `right` fields. The default is `i`/`k`/`j`/`l`.
--- For vim-style keys, set `{ up = "k", down = "j", left = "h", right = "l" }`.
obj.movementKeys = { up = "i", down = "k", left = "j", right = "l" }

-- Types ----------------------------------------------------------------------

---@alias State "idle" | "grid" | "subgrid" | "nudge"
---@alias Rect { x: number, y: number, w: number, h: number }
---@alias Point { x: number, y: number }
---@alias Size { w: number, h: number }

---@class GridSpec
---@field origin Point                               -- top-left, canvas-local coords
---@field size   Size                                -- total grid size in pixels
---@field cols   integer
---@field rows   integer
---@field label  fun(col: integer, row: integer): string?  -- nil skips the label
---@field focus? Rect                                -- optional rect to outline

-- Config ---------------------------------------------------------------------

local KEYS = { "a", "s", "d", "f", "g", "h", "j", "k", "l", ";" }
local COLS, ROWS = #KEYS, #KEYS

-- Sub-grid: 2 rows x 5 cols, one key per cell. Top row asdfg, bottom hjkl;.
local SUB_COLS, SUB_ROWS = 5, 2
local SUB_KEYS = {
	a = { 0, 0 },
	s = { 1, 0 },
	d = { 2, 0 },
	f = { 3, 0 },
	g = { 4, 0 },
	h = { 0, 1 },
	j = { 1, 1 },
	k = { 2, 1 },
	l = { 3, 1 },
	[";"] = { 4, 1 },
}
local SUB_LABELS = {
	{ "a", "s", "d", "f", "g" },
	{ "h", "j", "k", "l", ";" },
}

local NUDGE = 10
local CLICK_GAP_US = 20000 -- 20ms between mouseDown and mouseUp

-- Click keys: each entry is { right = bool, double = bool }.
-- For right click, hold shift with any single-click key.
local CLICK_KEYS = {
	space = { right = false, double = false },
	["return"] = { right = false, double = false },
	d = { right = false, double = true },
}

local UI = {
	overlay_alpha = 0.25,
	stroke_alpha = 0.5,
	label_alpha = 0.95,
	focus_alpha = 0.9,
	focus_stroke_width = 2,
	min_font = 12,
	font_scale = 0.28,
}

local key_index = {}
for i, k in ipairs(KEYS) do
	key_index[k] = i - 1
end

-- State ----------------------------------------------------------------------

local canvas, tap
---@type State
local state = "idle"
---@type Rect
local screen_frame
---@type string?
local first_key = nil
---@type Rect?
local sub_cell = nil -- bounds of the selected main cell

local function is_shift()
	return hs.eventtap.checkKeyboardModifiers().shift == true
end

local function cleanup()
	state = "idle"
	first_key = nil
	sub_cell = nil
	if tap then
		tap:stop()
		tap = nil
	end
	if canvas then
		canvas:hide()
	end
end

-- Rendering ------------------------------------------------------------------

local function ensure_canvas()
	if not canvas then
		canvas = hs.canvas.new({ x = 0, y = 0, w = 0, h = 0 })
		canvas:level("overlay")
		canvas:behavior({ "canJoinAllSpaces", "fullScreenAuxiliary" })
		canvas:clickActivating(false)
	end
end

-- Draw a grid described by `spec`. This is the single rendering primitive;
-- both the main grid and sub-grid are thin wrappers over it.
---@param spec GridSpec
local function draw_grid_spec(spec)
	ensure_canvas()
	canvas:frame(screen_frame)
	local sf = screen_frame
	local ox, oy = spec.origin.x, spec.origin.y
	local cw = spec.size.w / spec.cols
	local ch = spec.size.h / spec.rows
	local font_size = math.max(UI.min_font, math.floor(math.min(cw, ch) * UI.font_scale))

	local els = {
		{
			type = "rectangle",
			action = "fill",
			fillColor = { white = 0, alpha = UI.overlay_alpha },
			frame = { x = 0, y = 0, w = sf.w, h = sf.h },
		},
	}

	if spec.focus then
		els[#els + 1] = {
			type = "rectangle",
			action = "stroke",
			strokeColor = { white = 1, alpha = UI.focus_alpha },
			strokeWidth = UI.focus_stroke_width,
			frame = spec.focus,
		}
	end

	for col = 0, spec.cols - 1 do
		for row = 0, spec.rows - 1 do
			local x, y = ox + col * cw, oy + row * ch
			els[#els + 1] = {
				type = "rectangle",
				action = "stroke",
				strokeColor = { white = 1, alpha = UI.stroke_alpha },
				strokeWidth = 1,
				frame = { x = x, y = y, w = cw, h = ch },
			}
			local text = spec.label(col, row)
			if text then
				els[#els + 1] = {
					type = "text",
					text = text:upper(),
					textSize = font_size,
					textColor = { white = 1, alpha = UI.label_alpha },
					textAlignment = "center",
					frame = { x = x, y = y + (ch - font_size) / 2, w = cw, h = font_size + 4 },
				}
			end
		end
	end

	canvas:replaceElements(els):show()
end

local function draw_grid()
	draw_grid_spec({
		origin = { x = 0, y = 0 },
		size = { w = screen_frame.w, h = screen_frame.h },
		cols = COLS,
		rows = ROWS,
		label = function(col, row)
			local c1, c2 = KEYS[col + 1], KEYS[row + 1]
			if first_key == nil or first_key == c1 then
				return c1 .. c2
			end
		end,
	})
end

local function draw_subgrid()
	assert(sub_cell, "draw_subgrid called without a selected cell")
	local bx = sub_cell.x - screen_frame.x
	local by = sub_cell.y - screen_frame.y
	draw_grid_spec({
		origin = { x = bx, y = by },
		size = { w = sub_cell.w, h = sub_cell.h },
		cols = SUB_COLS,
		rows = SUB_ROWS,
		label = function(col, row)
			return SUB_LABELS[row + 1][col + 1]
		end,
		focus = { x = bx, y = by, w = sub_cell.w, h = sub_cell.h },
	})
end

-- Mouse actions --------------------------------------------------------------

-- Perform a mouse click at `p`. Pure action; no lifecycle side-effects.
---@param p Point
---@param right boolean
---@param double boolean
local function do_click(p, right, double)
	local down = right and hs.eventtap.event.types.rightMouseDown or hs.eventtap.event.types.leftMouseDown
	local up = right and hs.eventtap.event.types.rightMouseUp or hs.eventtap.event.types.leftMouseUp
	local clickStateProp = hs.eventtap.event.properties.mouseEventClickState
	hs.mouse.absolutePosition(p)
	hs.eventtap.event.newMouseEvent(down, p):post()
	hs.eventtap.event.newMouseEvent(up, p):post()
	if double then
		hs.timer.usleep(CLICK_GAP_US)
		hs.eventtap.event.newMouseEvent(down, p):setProperty(clickStateProp, 2):post()
		hs.eventtap.event.newMouseEvent(up, p):setProperty(clickStateProp, 2):post()
	end
end

-- Tear down the overlay, then click after a short delay so the overlay has
-- time to disappear before the click is delivered to the underlying app.
---@param p Point
---@param right boolean
---@param double boolean
local function click_at(p, right, double)
	cleanup()
	hs.timer.doAfter(0.01, function()
		do_click(p, right, double)
	end)
end

---@param dx number
---@param dy number
local function nudge(dx, dy)
	local p = hs.mouse.absolutePosition()
	local newPos = { x = p.x + dx, y = p.y + dy }
	hs.mouse.absolutePosition(newPos)
	hs.eventtap.event.newMouseEvent(hs.eventtap.event.types.mouseMoved, newPos):post()
end

---@param col integer  -- 0-indexed
---@param row integer  -- 0-indexed
local function jump_to_cell(col, row)
	local cw = screen_frame.w / COLS
	local ch = screen_frame.h / ROWS
	sub_cell = {
		x = screen_frame.x + col * cw,
		y = screen_frame.y + row * ch,
		w = cw,
		h = ch,
	}
	hs.mouse.absolutePosition({
		x = sub_cell.x + cw / 2,
		y = sub_cell.y + ch / 2,
	})
	state = "subgrid"
	first_key = nil
	draw_subgrid()
end

---@param col integer  -- 0-indexed
---@param row integer  -- 0-indexed
local function jump_to_subcell(col, row)
	assert(sub_cell, "jump_to_subcell called without a selected cell")
	local cw = sub_cell.w / SUB_COLS
	local ch = sub_cell.h / SUB_ROWS
	local p = {
		x = sub_cell.x + col * cw + cw / 2,
		y = sub_cell.y + row * ch + ch / 2,
	}
	hs.mouse.absolutePosition(p)
	if obj.freeMove then
		state = "nudge"
		canvas:hide()
	else
		-- Skip the free-move stage: click at once. Shift gives a right click.
		click_at(p, is_shift(), false)
	end
end

-- Key handlers ---------------------------------------------------------------

---@param key string
local function handle_grid_key(key)
	if key == "delete" and first_key then
		first_key = nil
		draw_grid()
		return
	end
	local idx = key_index[key]
	if idx == nil then
		return
	end
	if not first_key then
		first_key = key
		draw_grid()
	else
		jump_to_cell(key_index[first_key], idx)
	end
end

---@param key string
local function handle_subgrid_key(key)
	local pos = SUB_KEYS[key]
	if pos then
		jump_to_subcell(pos[1], pos[2])
	end
end

---@param key string
---@return { [1]: number, [2]: number }?  -- {dx, dy} in pixels, or nil
local function movement_delta(key)
	local mk = obj.movementKeys
	if key == mk.up then
		return { 0, -NUDGE }
	elseif key == mk.down then
		return { 0, NUDGE }
	elseif key == mk.left then
		return { -NUDGE, 0 }
	elseif key == mk.right then
		return { NUDGE, 0 }
	end
	return nil
end

---@param key string
local function handle_nudge_key(key)
	local click = CLICK_KEYS[key]
	if click then
		click_at(hs.mouse.absolutePosition(), click.right or is_shift(), click.double)
		return
	end
	local delta = movement_delta(key)
	if delta then
		nudge(delta[1], delta[2])
	end
end

local function handle_key(event)
	local key = hs.keycodes.map[event:getKeyCode()]
	if not key then
		return true
	end

	if key == "escape" or (key == "delete" and state == "nudge") then
		cleanup()
	elseif state == "grid" then
		handle_grid_key(key)
	elseif state == "subgrid" then
		handle_subgrid_key(key)
	elseif state == "nudge" then
		handle_nudge_key(key)
	end

	return true
end

-- Public ---------------------------------------------------------------------

--- GridMouse:start()
--- Method
--- Show the grid overlay and begin keyboard mouse control.
---
--- Parameters:
---  * None
---
--- Returns:
---  * The GridMouse object
function obj:start()
	cleanup()
	local scr = hs.mouse.getCurrentScreen() or hs.screen.mainScreen()
	screen_frame = scr:fullFrame()
	state = "grid"
	draw_grid()

	tap = hs.eventtap
		.new({ hs.eventtap.event.types.keyDown }, function(event)
			if state == "idle" then
				return false
			end
			return handle_key(event)
		end)
		:start()
	return self
end

--- GridMouse:stop()
--- Method
--- Tear down the overlay and stop keyboard mouse control.
---
--- Parameters:
---  * None
---
--- Returns:
---  * The GridMouse object
function obj:stop()
	cleanup()
	return self
end

--- GridMouse:bindHotkeys(mapping)
--- Method
--- Binds hotkeys for GridMouse.
---
--- Parameters:
---  * mapping - A table containing hotkey modifier/key details for the following items:
---    * start - Show the grid overlay and begin keyboard mouse control
---    * stop  - Tear down the overlay (optional)
---
--- Returns:
---  * The GridMouse object
function obj:bindHotkeys(mapping)
	hs.spoons.bindHotkeysToSpec({
		start = hs.fnutils.partial(self.start, self),
		stop = hs.fnutils.partial(self.stop, self),
	}, mapping)
	return self
end

return obj
