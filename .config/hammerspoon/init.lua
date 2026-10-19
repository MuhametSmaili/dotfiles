hs = hs
local meh = { "ctrl", "alt", "shift" }

hs.alert.defaultStyle.radius = 6
hs.alert.show("Config reloaded")

local KeyCaster = require("key-caster")
local Mic = require("mic")
local CapsMeh = require("capsmeh")
local SSH = require("ssh")
local Bing = require("bing")
local Ghostty = require("ghostty")
local AppDeck = require("appdeck")

Bing.start()
CapsMeh.start()

hs.loadSpoon("GridMouse")
spoon.GridMouse:bindHotkeys({ start = { meh, "g" } })

hs.loadSpoon("Awake")
hs.loadSpoon("CleanMode")
hs.loadSpoon("Leader")

hs.hotkey.bind(meh, "tab", AppDeck.toggle)

spoon.Leader.create(meh, "space", {
	r = { label = "Reload", action = hs.reload },
	o = {
		label = "Open",
		group = {
			g = { label = "Ghostty", action = spoon.Leader.app("Ghostty") },
			b = { label = "Brave", action = spoon.Leader.app("Brave Browser") },
			t = { label = "Teams", action = spoon.Leader.app("Microsoft Teams") },
			m = {
				label = "Mail",
				action = function()
					hs.application.launchOrFocus(
						hs.application.pathForBundleID("com.microsoft.Outlook") and "Microsoft Outlook" or "Thunderbird"
					)
				end,
			},
			f = { label = "Finder", action = spoon.Leader.app("Finder") },
			o = { label = "Obsidian", action = spoon.Leader.app("Obsidian") },
			d = { label = "Discord", action = spoon.Leader.app("Discord") },
			w = { label = "WhatsApp", action = spoon.Leader.app("WhatsApp") },
		},
	},
	g = {
		label = "Go to",
		group = {
			d = { label = "Downloads", action = Ghostty.openHome("Downloads") },
			["."] = { label = "dotfiles", action = Ghostty.openHome("dotfiles") },
			v = { label = "Vault", action = Ghostty.openHome(".vaults") },
			p = { label = "Projects", action = Ghostty.openHome("Projects") },
			c = { label = "ChatGPT", action = spoon.Leader.open("https://chatgpt.com", "Brave Browser") },
		},
	},
	s = {
		label = "Screenshot",
		group = {
			a = { label = "Area(shottr)", action = spoon.Leader.open("shottr://grab/area") },
			-- A = {
			-- 	label = "Area",
			-- 	action = function()
			-- 		hs.eventtap.keyStroke({ "ctrl", "cmd", "shift" }, "4", 0)
			-- 	end,
			-- },
			o = { label = "OCR", action = spoon.Leader.open("shottr://ocr") },
			w = { label = "Window", action = spoon.Leader.open("shottr://grab/window") },
			f = { label = "Fullscreen", action = spoon.Leader.open("shottr://grab/fullscreen") },
			s = { label = "Scrolling", action = spoon.Leader.open("shottr://grab/scrolling") },
			r = {
				label = "Record (macOS)",
				action = function()
					hs.eventtap.keyStroke({ "shift", "cmd" }, "5", 0)
				end,
			},
		},
	},
	p = {
		label = "Pick",
		group = {
			e = {
				label = "Emoji",
				action = function()
					hs.eventtap.keyStroke({ "ctrl", "cmd" }, "space", 0)
				end,
			},
			p = {
				label = "1Password",
				action = function()
					hs.eventtap.keyStroke({ "shift", "cmd" }, "space", 0)
				end,
			},
			b = { label = "Color (bg)", action = spoon.Leader.open("pika://pick/background/hex") },
			f = { label = "Color (fg)", action = spoon.Leader.open("pika://pick/foreground/hex") },
			s = { label = "SSH host", action = SSH.pick },
		},
	},
	b = {
		label = "Bing",
		group = {
			i = { label = "Info (current)", action = Bing.info },
			r = { label = "Refresh now", action = Bing.refresh },
			p = { label = "Pick region", action = Bing.pickRegion },
			o = { label = "Open folder", action = spoon.Leader.open(os.getenv("HOME") .. "/Pictures/BingWallpapers") },
		},
	},
	t = {
		label = "Toggle",
		group = {
			m = { label = "Mic mute", action = Mic.toggle },
			k = { label = "Key Caster", action = KeyCaster.toggle },
			c = { label = "Awake", action = spoon.Awake.toggle },
			n = { label = "Clean Mode", action = spoon.CleanMode.toggle },
		},
	},
	q = {
		label = "Quit / Session",
		group = {
			l = {
				label = "Lock screen",
				action = function()
					hs.caffeinate.lockScreen()
				end,
			},
			s = {
				label = "Sleep display",
				action = function()
					hs.caffeinate.systemSleep()
				end,
			},
			o = {
				label = "Log out",
				action = function()
					hs.caffeinate.logOut()
				end,
			},
			e = {
				label = "Empty trash",
				action = function()
					hs.osascript.applescript('tell application "Finder" to empty trash')
				end,
			},
		},
	},
}, 2.5)
