-------------------------------------------------
-- name : fzf-lua
-- url  : https://github.com/ibhagwan/fzf-lua
---------------------------------------------------
return {
	"ibhagwan/fzf-lua",
	cmd = "FzfLua",
	keys = {
		{
			"<C-p>",
			":FzfLua files<cr>",
			desc = "Find files",
		},
		{ "<leader><space>", "<cmd>FzfLua resume<cr>", desc = "Resume last command" },
		{ "<leader>sw", "<cmd>FzfLua live_grep<cr>", desc = "Find word (grep project)" },
		{
			"<leader>sW",
			':lua require"fzf-lua".live_grep({ cmd = "git grep --line-number --files-with-matches --column --color=always" })<cr>',
			desc = "Find word (grep project)",
		},
		{
			"<leader>,",
			function()
				require("fzf-lua").oldfiles({
					cwd_only = true,
					stat_file = true, -- verify files exist on disk
				})
			end,
			desc = "Find word in buffer",
		},
		{ "<leader>sb", "<cmd>FzfLua lgrep_curbuf<cr>", desc = "Find word in buffer" },
		{
			"<leader>sg",
			function()
				require("fzf-lua").git_status({
					winopts = {
						fullscreen = true,
						preview = {
							layout = "vertical",
							vertical = "down:95%",
						},
					},
					fzf_opts = {
						["--layout"] = "reverse",
						["--info"] = "inline",
					},
					header = false,
				})
			end,
			desc = "Git status with large preview",
		},
		{ "<leader>sw", "<cmd>FzfLua grep_visual<cr>", desc = "Grep", mode = "x" },
		-- {
		--   "<leader>sW",
		--   ":lua require'fzf-lua'.grep_project({ multiprocess=true })<CR>",
		--   desc = "Find word (grep project)",
		-- },
		{ "<leader><tab>", ":FzfLua buffers<cr>", desc = "Find buffers" },
		{ "<leader>sR", ":FzfLua registers <cr>", desc = "Search registers" },
		{
			"<leader>sc",
			function()
				require("fzf-lua").colorschemes({
					winopts = { height = 0.45, width = 0.20, row = 0, col = 1 },
					actions = {
						["default"] = function(selected)
							local theme = selected[1]
							Custom.theme.apply(theme, Custom.colorscheme.transparent)
						end,
					},
				})
			end,
			desc = "Search colorschemes",
		},
		{
			"q:",
			function()
				require("fzf-lua").command_history({
					winopts = { height = 0.30, width = 0.12, row = 1, col = 0, layout = "default" },
					fzf_opts = {
						["--layout"] = false,
					},
				})
			end,
			desc = "Search commands",
		},
		{
			"q/",
			function()
				require("fzf-lua").search_history({
					winopts = { height = 0.30, width = 0.12, row = 1, col = 0, layout = "default" },
					fzf_opts = {
						["--layout"] = false,
					},
				})
			end,
			desc = "Search commands",
		},
		{ "<leader>sh", ":FzfLua help_tags<cr>", desc = "Search help tags" },
		{ "<leader>sH", ":FzfLua highlights<cr>", desc = "Search highlights" },
		{ "<leader>sm", ":FzfLua marks <cr>", desc = "Search marks" },
		{ "<leader>sM", ":FzfLua man_pages <cr>", desc = "Search manual pages" },
		{ "<leader>sk", ":FzfLua keymaps <cr>", desc = "Search key mappings" },
		{
			"<leader>ss",
			":FzfLua spell_suggest winopts={height=0.33,width=0.12} winopts.relative=cursor  <cr>",
			desc = "Show spell suggestions",
		},
		{ "<leader>sJ", ":FzfLua jumps <cr>", desc = "Search jumps" },
		{
			"<leader>gb",
			function()
				require("custom.plugins.fzf.utils").buffer_commits()
			end,
			desc = "Git buffer commits (CodeDiff)",
		},
		{
			"<leader>gB",
			function()
				require("custom.plugins.fzf.utils").buffer_commits_compare()
			end,
			desc = "Git buffer commits (CodeDiff two commits)",
		},
		{
			"<leader>gc",
			function()
				require("custom.plugins.fzf.utils").commits()
			end,
			desc = "Git commits (CodeDiff)",
		},
		{
			"<leader>gA",
			function()
				require("fzf-lua").fzf_live(
					"git rev-list --all | xargs git grep --line-number --column --color=always <query>",
					{
						fzf_opts = {
							["--delimiter"] = ":",
							-- ["--preview-window"] = "nohidden,down,60%,border-top,+{3}+3/3,~3",
						},
						preview = "git show {1}:{2} | "
							.. "bat --style=default --color=always --file-name={2} --highlight-line={3}",
					}
				)
			end,
			desc = "Git grap all GIT history",
		},
		{
			"<c-x><c-f>",
			"<esc>:FzfLua complete_path winopts={height=1.20,width=0.12} winopts.relative=cursor  <cr>",
			mode = "i",
		},
		{
			"<leader>st",
			"<cmd>lua require('fzf-lua').grep({search='TODO|HACK|PERF|NOTE|FIX', no_esc=true})<CR>",
			desc = "Search tags TODO|FIX|PERF|FIX...",
		},
		{ "z=", "<cmd>FzfLua spell_suggest<cr>", desc = "Spelling suggestions" },
	},
	init = function()
		---@diagnostic disable-next-line: duplicate-set-field
		vim.ui.select = function(items, opts, on_choice)
			local ui_select = require("fzf-lua.providers.ui_select")

			-- Register the fzf-lua picker the first time we call select.
			if not ui_select.is_registered() then
				ui_select.register(function(ui_opts)
					if ui_opts.kind == "luasnip" then
						ui_opts.prompt = "Snippet choice: "
						ui_opts.winopts = {
							relative = "cursor",
							height = 0.35,
							width = 0.3,
						}
					elseif ui_opts.kind == "lsp_message" then
						ui_opts.winopts = { height = 0.4, width = 0.4 }
					else
						ui_opts.winopts = { height = 0.5, width = 0.4 }
					end

					return ui_opts
				end)
			end

			-- Don't show the picker if there's nothing to pick.
			if #items > 0 then
				return vim.ui.select(items, opts, on_choice)
			end
		end
	end,
	opts = function()
		return {
			{ "fzf-native", "hide" },
			oldfiles = {
				include_current_session = true,
				winopts = {
					preview = { hidden = true },
				},
			},
			grep = {
				hidden = true,
			},
			git = {
				bcommits = { follow = true },
			},
			previewers = {
				codeaction = { toggle_behavior = "extend" },
			},
			winopts = {
				height = 0.7,
				width = 0.60,
				preview = {
					scrollbar = false,
					layout = "horizontal",
				},
				winopts = {
					relativenumber = true,
				},
			},
			keymap = {
				builtin = {
					["<Esc>"] = "hide",
					["<a-m>"] = "toggle-fullscreen",
					["<C-i>"] = "toggle-preview",
					["<C-f>"] = "preview-page-down",
					["<C-b>"] = "preview-page-up",
					-- ["<C-y>"] = "select",
					-- ["<C-n>"] = "down",
					["<C-/>"] = "toggle-help",
				},
				fzf = {
					["ctrl-f"] = "preview-page-down",
					["ctrl-b"] = "preview-page-up",
					["ctrl-q"] = "select-all+accept",
					["alt-p"] = "toggle-preview",
				},
			},
			files = {
				git_icons = true,
				multiprocess = true,
				winopts = {
					preview = { hidden = true },
				},
			},
			lsp = {
				code_actions = {
					winopts = {
						width = 70,
						height = 20,
						relative = "cursor",
						preview = {
							hidden = true,
							vertical = "down:50%",
						},
					},
				},
			},
		}
	end,
}
