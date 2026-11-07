local CapsMeh = require("capsmeh")
local M = {}
local active

local function pandoc_path()
	for _, path in ipairs({ "/opt/homebrew/bin/pandoc", "/usr/local/bin/pandoc" }) do
		if hs.fs.attributes(path, "mode") == "file" then
			return path
		end
	end
end

function M.paste()
	if active then
		return hs.alert.show("Rich paste is already running")
	end
	local version = hs.pasteboard.changeCount()
	local markdown = hs.pasteboard.getContents()
	if not markdown or not markdown:find("%S") then
		return hs.alert.show("Copy some Markdown first")
	end
	if #markdown > 1024 * 1024 then
		return hs.alert.show("Rich paste supports up to 1 MiB of Markdown")
	end
	local pandoc = pandoc_path()
	if not pandoc then
		return hs.alert.show("Pandoc was not found in Homebrew's bin folder")
	end
	local app, window = hs.application.frontmostApplication(), hs.window.focusedWindow()
	if not app or not window then
		return hs.alert.show("Focus a text editor before using rich paste")
	end
	local pid, window_id = app:pid(), window:id()
	local task, timer, result
	local function finish(message)
		if active ~= finish then
			return
		end
		active = nil
		if timer then
			timer:stop()
		end
		if task then
			task:setCallback(nil)
			if task:isRunning() then
				task:terminate()
			end
		end
		if message then
			hs.alert.show(message)
		end
	end
	active = finish
	task = hs.task.new(pandoc, function(code, output)
		if active ~= finish then
			return
		end
		task = nil
		if code ~= 0 then
			return finish("Pandoc could not convert the Markdown")
		end
		local ok, converted = pcall(hs.json.decode, output)
		if
			not ok
			or type(converted) ~= "table"
			or type(converted.html) ~= "string"
			or type(converted.text) ~= "string"
			or not converted.text:find("%S")
		then
			return finish("Pandoc returned no usable rich text")
		end
		result = converted
	end, {
		"--from=gfm",
		"--to=" .. hs.configdir .. "/markdown-paste-writer.lua",
		"--wrap=none",
		"--syntax-highlighting=none",
		"--email-obfuscation=none",
		"--sandbox",
	})
	if not task or not task:setInput(markdown):start() then
		return finish("Could not start Pandoc")
	end
	local deadline = hs.timer.secondsSinceEpoch() + 10
	timer = hs.timer.doEvery(0.02, function()
		if active ~= finish then
			return
		end
		local current_app, current_window = hs.application.frontmostApplication(), hs.window.focusedWindow()
		if not current_app or current_app:pid() ~= pid or not current_window or current_window:id() ~= window_id then
			return finish("Rich paste cancelled: focus changed")
		end
		if hs.pasteboard.changeCount() ~= version then
			return finish("Rich paste cancelled: clipboard changed")
		end
		if hs.timer.secondsSinceEpoch() >= deadline then
			return finish("Rich paste timed out; clipboard left unchanged")
		end
		local modifiers = hs.eventtap.checkKeyboardModifiers()
		if
			not result
			or CapsMeh.isHeld()
			or modifiers.ctrl
			or modifiers.alt
			or modifiers.shift
			or modifiers.cmd
			or modifiers.fn
		then
			return
		end
		if
			not hs.pasteboard.writeAllData({ ["public.html"] = result.html, ["public.utf8-plain-text"] = result.text })
		then
			return finish("Could not copy rich text")
		end
		finish()
		hs.eventtap.keyStroke({ "cmd" }, "v", 0, app)
	end)
end

return M
