local review_changes = "Review the changes in {file} for bugs and regressions."
	.. "\nReport findings with file/line references."
	.. "\nDo not modify files."
	.. "\n\n{changes}"

local prompts = {
	{ value = review_changes, label = "Review file changes" },
	{
		value = "Can you help me fix the diagnostics in {file}?\n{diagnostics_all}",
		label = "Fix diagnostics (file)",
	},
	{ value = "Can you help me fix this diagnostic?\n{diagnostics}", label = "Fix diagnostics (line)" },
	{ value = "Add documentation to {this}", label = "Add docs" },
	{ value = "Explain {this}", label = "Explain" },
	{ value = "Can you fix {this}?", label = "Fix" },
	{ value = "How can {this} be optimized?", label = "Optimize" },
	{ value = "Can you review {file} for any issues?", label = "Review file" },
	{ value = "Can you write tests for {this}?", label = "Write tests" },
	{ value = "Can you help me fix these issues?\n{quickfix}", label = "Fix quickfix" },
}

return {
	"MuhametSmaili/wiremux.nvim",
	lazy = false,
	---@type wiremux.config.UserOptions
	opts = {
		-- log_level = "debug",
		targets = {
			definitions = {
				opencode = { cmd = "opencode", kind = { "pane", "window" }, split = "horizontal", shell = false },
				claudecode = { cmd = "claude", kind = { "pane", "window" }, split = "horizontal", shell = false },
				kiro = { kind = { "pane", "window" }, split = "horizontal", cmd = "kiro-cli", shell = false },
				pi = { kind = { "pane", "window" }, split = "horizontal", cmd = "pi", shell = false },
				shell = { kind = { "pane", "window" }, split = "horizontal" },
				quick = { kind = { "pane", "window" }, split = "horizontal", shell = false },
			},
		},
		actions = {
			send = {
				compose = true,
				behavior = "pick",
				focus = true,
				submit = false,
			},
		},
		ui = {
			compose = {
				close_behavior = "hide",
				on_new_payload = "append",
				history_limit = 3,
			},
		},
		picker = {
			adapter = "fzf-lua",
		},
	},
	-- Message mappings: lowercase composes; uppercase sends directly.
	keys = {
		{
			"<leader>aa",
			function()
				require("wiremux").send("{this}")
			end,
			mode = { "n", "x" },
			desc = "Add context to draft",
		},
		{
			"<leader>ae",
			function()
				require("wiremux").send()
			end,
			desc = "Open / resume draft",
		},
		{
			"<leader>ar",
			function()
				require("wiremux").send(review_changes)
			end,
			desc = "Compose review of file changes",
		},
		{
			"<leader>aR",
			function()
				require("wiremux").send(review_changes, { compose = false, behavior = "last", focus = true })
			end,
			desc = "Send review of file changes directly",
		},
		{
			"<leader>ah",
			"<cmd>Wiremux history<cr>",
			desc = "Compose history",
		},
		{
			"<leader>aA",
			function()
				require("wiremux").send("{this}", { compose = false, behavior = "last", focus = true })
			end,
			mode = { "x", "n" },
			desc = "Send context directly",
		},
		{
			"<leader>af",
			function()
				require("wiremux").send("{file}")
			end,
			desc = "Add file reference to draft",
		},
		{
			"<leader>aF",
			function()
				require("wiremux").send("{file}", { compose = false, behavior = "last", focus = true })
			end,
			desc = "Send file reference directly",
		},
		{
			"<leader>av",
			function()
				require("wiremux").send("{selection}")
			end,
			mode = { "x" },
			desc = "Add selection to draft",
		},
		{
			"<leader>aV",
			function()
				require("wiremux").send("{selection}", { compose = false, behavior = "last", focus = true })
			end,
			mode = { "x" },
			desc = "Send selection directly",
		},
		{
			"<leader>ac",
			function()
				require("wiremux").create()
			end,
			desc = "Create",
		},
		{
			"<leader>ax",
			function()
				require("wiremux").close()
			end,
			desc = "Close target",
		},
		{
			"<leader>ao",
			function()
				require("wiremux").focus({ behavior = "pick" })
			end,
			desc = "Focus target",
		},
		{
			"<leader>ad",
			function()
				require("wiremux").send("{diagnostics}")
			end,
			desc = "Compose line diagnostics",
			mode = { "n", "x" },
		},
		{
			"<leader>aD",
			function()
				require("wiremux").send("{diagnostics}", { compose = false, behavior = "last", focus = true })
			end,
			desc = "Send line diagnostics directly",
			mode = { "n", "x" },
		},
		{
			"<leader>ag",
			function()
				require("wiremux").send("{diagnostics_all}")
			end,
			desc = "Compose file diagnostics",
			mode = { "n", "x" },
		},
		{
			"<leader>aG",
			function()
				require("wiremux").send("{diagnostics_all}", { compose = false, behavior = "last", focus = true })
			end,
			desc = "Send file diagnostics directly",
			mode = { "n", "x" },
		},
		{
			"ga",
			function()
				return require("wiremux").send_motion({ compose = false })
			end,
			desc = "Send motion directly",
			mode = { "x", "n" },
			expr = true,
		},
		{
			"<leader>ap",
			function()
				require("wiremux").send(prompts)
			end,
			mode = { "n", "x" },
			desc = "Choose prompt for draft",
		},
		{
			"<leader>aP",
			function()
				require("wiremux").send(prompts, { compose = false, behavior = "last", focus = true })
			end,
			mode = { "n", "x" },
			desc = "Choose prompt to send directly",
		},
		{
			"<leader>tr",
			function()
				require("wiremux").send({
					{
						label = "npm test",
						value = "npm test; exec $SHELL",
						submit = true,
						visible = function()
							return vim.fn.filereadable("package.json") == 1
						end,
					},
					{
						label = "npm run build",
						value = "npm run build",
						submit = true,
						visible = function()
							return vim.fn.filereadable("package.json") == 1
						end,
					},
					{
						label = "npm run start",
						value = "npm run start",
						submit = true,
						visible = function()
							return vim.fn.filereadable("package.json") == 1
						end,
					},
					{
						label = "go build",
						value = "go build",
						submit = true,
						visible = function()
							return vim.bo.filetype == "go"
						end,
					},
					{
						label = "go test (all)",
						value = "go test ./...; exec $SHELL",
						submit = true,
						visible = function()
							return vim.bo.filetype == "go"
						end,
					},
					{
						label = "go test (selection)",
						value = "go test -run '{selection}'",
						submit = true,
						visible = function()
							return vim.bo.filetype == "go" and require("wiremux.context").is_available("selection")
						end,
					},
				}, {
					compose = false,
					mode = "definitions",
					filter = {
						definitions = function(name)
							return name == "quick"
						end,
					},
					behavior = "pick",
				})
			end,
			desc = "Run project command",
		},
	},
}
