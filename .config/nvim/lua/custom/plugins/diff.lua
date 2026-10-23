return {
	"esmuellert/codediff.nvim",
	dependencies = { "MunifTanjim/nui.nvim" },
	cmd = "CodeDiff",
	keys = {
		{ "<leader>gD", "<cmd>CodeDiff<cr>", desc = "Git diff 3-way-split" },
	},
	opts = {
		explorer = {
			position = "left",
			-- hidden = true,
			view_mode = "tree",
			width = 30,
		},
		keymaps = {
			view = {
				next_hunk = "]h",
				prev_hunk = "[h",
			},
		},
	},
}
