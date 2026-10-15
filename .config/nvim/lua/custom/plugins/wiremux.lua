return {
	"MuhametSmaili/wiremux.nvim",
	lazy = false,
	---@wiremux.config.UserOptions
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
		picker = {
			adapter = "fzf-lua",
		},
	},
	keys = {
		{
			"<leader>aa",
			function()
				require("wiremux").send("{this}", {
					compose = {
						close_behavior = "hide",
						on_new_payload = "append",
					},
				})
			end,
			desc = "Send a message with compose",
		},
		{
			"<leader>at",
			function()
				require("wiremux").send("{this}", { focus = true, behavior = "last" })
			end,
			mode = { "x", "n" },
			desc = "Send This",
		},
		{
			"<leader>af",
			function()
				require("wiremux").send("{file}", { behavior = "pick", focus = true })
			end,
			desc = "Send File",
		},
		{
			"<leader>av",
			function()
				require("wiremux").send("{selection}", { focus = true })
			end,
			mode = { "x" },
			desc = "Send Visual Selection",
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
			desc = "Create",
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
				require("wiremux").send("{diagnostics}", { focus = true, behavior = "pick" })
			end,
			desc = "Wiremux send diagonstic current line",
			mode = { "n", "x" },
		},
		{
			"<leader>aD",
			function()
				require("wiremux").send("{diagnostics_all}", { focus = true, behavior = "pick" })
			end,
			desc = "Wiremux send all diagonstics",
			mode = { "x", "n" },
		},
		{
			"ga",
			function()
				return require("wiremux").send_motion()
			end,
			desc = "Wiremux send motion",
			mode = { "x", "n" },
			expr = true,
		},
		{
			"<leader>ap",
			function()
				require("wiremux").send({
					{ value = "Can you review my changes?\n{changes}", label = "Review changes" },
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
				})
			end,
			mode = { "n", "x" },
			desc = "Select prompt",
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
