local M = {}

local event_types = hs.eventtap.event.types
local event_properties = hs.eventtap.event.properties
local escape_keycode = hs.keycodes.map.escape
local meh_flags = {
	ctrl = true,
	alt = true,
	shift = true,
}

local tap = nil
local escape_held = false
local chord_used = false
local synthetic_escape_events = 0

local function merge_meh_flags(flags)
	local merged = {}
	for key, value in pairs(flags) do
		if type(key) == "string" and value == true then
			merged[key] = true
		end
	end
	for key in pairs(meh_flags) do
		merged[key] = true
	end
	return merged
end

local function reset_state()
	escape_held = false
	chord_used = false
end

local function post_escape()
	-- The real Escape key is swallowed so tap behavior is decided on release.
	synthetic_escape_events = synthetic_escape_events + 2
	hs.eventtap.event.newKeyEvent({}, "escape", true):post()
	hs.eventtap.event.newKeyEvent({}, "escape", false):post()
end

local function handle_escape(event)
	if synthetic_escape_events > 0 then
		synthetic_escape_events = synthetic_escape_events - 1
		return false
	end

	if event:getType() == event_types.keyDown then
		if event:getProperty(event_properties.keyboardEventAutorepeat) ~= 0 then
			return true
		end

		escape_held = true
		chord_used = false
		return true
	end

	if escape_held then
		local should_send_escape = not chord_used
		reset_state()
		if should_send_escape then
			post_escape()
		end
	end

	return true
end

local function rewrite_as_meh(event)
	if event:getType() == event_types.keyDown then
		chord_used = true
	end

	local replacement = event:copy():setFlags(merge_meh_flags(event:getFlags()))
	return true, { replacement }
end

function M.isHeld()
	return escape_held
end

function M.start()
	if tap then
		return tap
	end

	tap = hs.eventtap.new({ event_types.keyDown, event_types.keyUp }, function(event)
		if event:getKeyCode() == escape_keycode then
			return handle_escape(event)
		end

		if escape_held then
			return rewrite_as_meh(event)
		end

		return false
	end)

	return tap:start()
end

function M.stop()
	if not tap then
		return
	end

	tap:stop()
	tap = nil
	reset_state()
end

return M
