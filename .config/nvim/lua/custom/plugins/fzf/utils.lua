-- Open fzf-lua commit pickers and review the selection in CodeDiff.
--
-- gb  (buffer commits)
--   enter: what the commit changed in this file   (<sha>^ vs <sha>)
--   alt-d: what the commit changed, all files     (<sha>^ vs <sha>)
--   alt-h: this file at the commit vs HEAD
-- gB  (buffer commits, pick two) -> compare two commits, older on the left
-- gc  (all commits)
--   enter: what the commit changed, all files
--   alt-h: the commit vs HEAD, all files

local M = {}

-- Run git and report whether it exited with 0
---@param args string[]
---@param cwd string?
---@return boolean
local function git_ok(args, cwd)
	return vim.system({ "git", unpack(args) }, { cwd = cwd }):wait().code == 0
end

-- Commit hashes from git_commits/git_bcommits entries
---@param selected string[]
---@return string[]
local function shas_of(selected)
	local shas = {}
	for _, line in ipairs(selected or {}) do
		local sha = line:match("^%s*(%x+)")
		if sha then
			table.insert(shas, sha)
		end
	end
	return shas
end

-- Parent of a commit, or nil (with a warning) for the root commit
---@return string?
local function parent_of(sha, cwd)
	if git_ok({ "rev-parse", "--verify", "--quiet", sha .. "^" }, cwd) then
		return sha .. "^"
	end
	vim.notify(sha .. " is the root commit (no parent to compare)", vim.log.levels.WARN)
end

-- Put the older commit first
---@return string, string
local function oldest_first(a, b, cwd)
	if git_ok({ "merge-base", "--is-ancestor", b, a }, cwd) then
		return b, a
	end
	return a, b
end

-- CodeDiff for the current file only, or the explorer with all changed files
local function codediff(a, b, all)
	vim.cmd.CodeDiff(all and { a, b } or { "file", a, b })
end

-- Action: what the selected commit changed
local function changes(all)
	return function(selected, opts)
		local sha = shas_of(selected)[1]
		local base = sha and parent_of(sha, opts.cwd)
		if base then
			codediff(base, sha, all)
		end
	end
end

-- Action: the selected commit vs HEAD
local function vs_head(all)
	return function(selected)
		local sha = shas_of(selected)[1]
		if sha then
			codediff(sha, "HEAD", all)
		end
	end
end

function M.buffer_commits()
	require("fzf-lua").git_bcommits({
		actions = {
			["enter"] = { fn = changes(false), header = "commit changes" },
			["alt-d"] = { fn = changes(true), header = "commit changes (all files)" },
			["alt-h"] = { fn = vs_head(false), header = "compare with HEAD" },
		},
	})
end

-- Pick commit A, then commit B (picker opens twice).
-- Marking two commits with <Tab> in the first picker skips the second one.
function M.buffer_commits_compare()
	local function compare(a, b, cwd, all)
		local old, new = oldest_first(a, b, cwd)
		codediff(old, new, all)
	end

	local function pick_second(first)
		local function run(all)
			return function(selected, opts)
				local sha = shas_of(selected)[1]
				if sha then
					compare(first, sha, opts.cwd, all)
				end
			end
		end
		require("fzf-lua").git_bcommits({
			prompt = ("Compare %s with> "):format(first),
			actions = {
				["enter"] = { fn = run(false), header = "compare this file" },
				["alt-d"] = { fn = run(true), header = "compare all files" },
			},
		})
	end

	local function run(all)
		return function(selected, opts)
			local shas = shas_of(selected)
			if #shas >= 2 then
				compare(shas[1], shas[2], opts.cwd, all)
			elseif #shas == 1 then
				-- Let the first picker close before opening the second
				vim.schedule(function()
					pick_second(shas[1])
				end)
			end
		end
	end

	require("fzf-lua").git_bcommits({
		prompt = "First commit (tab: mark 2)> ",
		fzf_opts = { ["--no-multi"] = false, ["--multi"] = true },
		actions = {
			["enter"] = { fn = run(false), header = "pick second / compare this file" },
			["alt-d"] = { fn = run(true), header = "compare all files (2 marked)" },
		},
	})
end

function M.commits()
	require("fzf-lua").git_commits({
		actions = {
			["enter"] = { fn = changes(true), header = "commit changes" },
			["alt-h"] = { fn = vs_head(true), header = "compare with HEAD" },
		},
	})
end

return M
