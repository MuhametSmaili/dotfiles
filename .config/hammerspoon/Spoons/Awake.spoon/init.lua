--- === Awake ===
---
--- Keep macOS awake (prevent display/idle sleep), with an optional mouse
--- "jiggle" that nudges the cursor a few pixels every minute so you stay
--- present in apps that track idle time. Adds a menubar toggle (☕ / ☾).
---
--- Unlike a plain caffeinate toggle, Awake also performs the periodic jiggle,
--- but only when the machine has actually been idle, so it never fights your
--- own cursor movement.
---
--- Download: https://github.com/MuhametSmaili/dotfiles

local obj = {}
obj.__index = obj

-- Metadata
obj.name = "Awake"
obj.version = "1.0"
obj.author = "MuhametSmaili"
obj.homepage = "https://github.com/MuhametSmaili/dotfiles"
obj.license = "MIT - https://opensource.org/licenses/MIT"

--- Awake.jiggleDistance
--- Variable
--- Pixels to move the cursor on each jiggle (default 60).
obj.jiggleDistance = 60

--- Awake.jiggleInterval
--- Variable
--- Seconds between jiggles (default 60).
obj.jiggleInterval = 60

-- State ----------------------------------------------------------------------

local menu = nil
local jiggle_timer = nil
local jiggle_dir = 1

-- Jiggle ---------------------------------------------------------------------

local function jiggleOnce()
	local idle_threshold = obj.jiggleInterval - 5
	if hs.host.idleTime() < idle_threshold then
		return
	end
	local p = hs.mouse.absolutePosition()
	local newPos = { x = p.x + obj.jiggleDistance * jiggle_dir, y = p.y }
	hs.eventtap.event.newMouseEvent(hs.eventtap.event.types.mouseMoved, newPos):post()
	jiggle_dir = -jiggle_dir
end

local function stopJiggle()
	if jiggle_timer then
		jiggle_timer:stop()
		jiggle_timer = nil
	end
end

local function startJiggle()
	stopJiggle()
	jiggle_timer = hs.timer.doEvery(obj.jiggleInterval, jiggleOnce)
end

local function updateMenu(state)
	if not menu then
		return
	end
	menu:setTitle(state and "☕" or "☾")
	menu:setTooltip(state and "Awake: ACTIVE" or "Awake: INACTIVE")
end

local function applyState(state)
	hs.caffeinate.set("displayIdle", state)
	if state then
		startJiggle()
	else
		stopJiggle()
	end
	updateMenu(state)
	return state
end

-- Public ---------------------------------------------------------------------

--- Awake:init()
--- Method
--- Creates the menubar item and syncs to the current caffeinate state. Called
--- automatically by `hs.loadSpoon`.
---
--- Returns:
---  * The Awake object
function obj:init()
	if not menu then
		menu = hs.menubar.new()
		obj.menu = menu
	end
	if menu then
		menu:setClickCallback(function()
			obj.toggle()
		end)
	end
	applyState(hs.caffeinate.get("displayIdle"))
	return self
end

--- Awake:start()
--- Method
--- Turn Awake on (prevent idle sleep + start jiggle).
---
--- Returns:
---  * The Awake object
function obj:start()
	applyState(true)
	return self
end

--- Awake:stop()
--- Method
--- Turn Awake off.
---
--- Returns:
---  * The Awake object
function obj:stop()
	applyState(false)
	return self
end

--- Awake.toggle()
--- Method
--- Toggle Awake on/off, with an on-screen alert. Safe to use as a bare
--- function reference (does not require `self`).
function obj.toggle()
	local newState = applyState(not hs.caffeinate.get("displayIdle"))
	hs.alert.show(newState and "Awake ON ☕" or "Awake OFF ☾")
end

--- Awake:bindHotkeys(mapping)
--- Method
--- Binds hotkeys for Awake.
---
--- Parameters:
---  * mapping - A table containing hotkey details for the following items:
---    * toggle - Toggle Awake on/off
---    * start  - Turn Awake on (optional)
---    * stop   - Turn Awake off (optional)
---
--- Returns:
---  * The Awake object
function obj:bindHotkeys(mapping)
	hs.spoons.bindHotkeysToSpec({
		toggle = obj.toggle,
		start = hs.fnutils.partial(self.start, self),
		stop = hs.fnutils.partial(self.stop, self),
	}, mapping)
	return self
end

return obj
