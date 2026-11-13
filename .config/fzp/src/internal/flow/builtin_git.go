package flow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"fzp/internal/proc"
)

const (
	defaultCommitLimit = 1000
	maxCommitLimit     = 100000
)

// Git builtins run in with.dir, which defaults to the flow's working
// directory: the target pane's directory when known.

type gitCommits struct {
	Dir   string `yaml:"dir"`
	Ref   string `yaml:"ref"`
	Limit *int   `yaml:"limit"`
}

type gitBranches struct {
	Dir                    string `yaml:"dir"`
	Local                  *bool  `yaml:"local"`
	Remote                 *bool  `yaml:"remote"`
	ExcludeCheckedOut      bool   `yaml:"exclude_checked_out"`
	ExcludeShadowedRemotes bool   `yaml:"exclude_shadowed_remotes"`
}

type gitWorktrees struct {
	Dir            string `yaml:"dir"`
	ExcludeCurrent bool   `yaml:"exclude_current"`
	ExcludeMain    bool   `yaml:"exclude_main"`
	ExcludeLocked  bool   `yaml:"exclude_locked"`
}

func newGitCommits(with *yaml.Node) (source, error) {
	var s gitCommits
	if err := decodeParams(with, &s); err != nil {
		return nil, err
	}

	if s.Limit != nil && (*s.Limit < 1 || *s.Limit > maxCommitLimit) {
		return nil, fmt.Errorf("limit must be between 1 and %d", maxCommitLimit)
	}
	return s, nil
}

func newGitBranches(with *yaml.Node) (source, error) {
	var s gitBranches
	if err := decodeParams(with, &s); err != nil {
		return nil, err
	}

	if !boolOr(s.Local, true) && !boolOr(s.Remote, true) {
		return nil, errors.New("local and remote cannot both be false")
	}
	return s, nil
}

func newGitWorktrees(with *yaml.Node) (source, error) {
	var s gitWorktrees
	return s, decodeParams(with, &s)
}

func boolOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (s gitCommits) list(ctx context.Context, env *env) ([]item, error) {
	ref, err := expandParam(env, "ref", firstNonEmpty(s.Ref, "HEAD"))
	if err != nil {
		return nil, err
	}

	limit := defaultCommitLimit
	if s.Limit != nil {
		limit = *s.Limit
	}

	out, err := runGit(ctx, env, s.Dir,
		"log", "--max-count="+strconv.Itoa(limit), "-z", "--no-color", "--no-decorate",
		"--no-show-signature", "--no-notes", "--format=%H%x1f%h%x1f%cs%x1f%an%x1f%s", ref, "--")
	if err != nil {
		return nil, err
	}

	var items []item
	for _, record := range splitRecords(out, "\x00") {
		values := strings.SplitN(record, "\x1f", 5)
		if len(values) != 5 {
			return nil, fmt.Errorf("git log: unexpected record %q", record)
		}

		fields := map[string]string{
			"hash":    values[0],
			"short":   values[1],
			"date":    values[2],
			"author":  values[3],
			"subject": values[4],
		}
		items = append(items, item{
			ID:          fields["hash"],
			Label:       fields["short"] + "  " + fields["date"] + "  " + fields["subject"],
			Description: fields["author"],
			Fields:      fields,
		})
	}
	return items, nil
}

func (s gitBranches) list(ctx context.Context, env *env) ([]item, error) {
	var patterns []string
	if boolOr(s.Local, true) || s.ExcludeShadowedRemotes {
		patterns = append(patterns, "refs/heads/")
	}
	if boolOr(s.Remote, true) {
		patterns = append(patterns, "refs/remotes/")
	}

	args := append([]string{"for-each-ref",
		"--format=%(refname)%1f%(refname:short)%1f%(upstream:short)%1f%(worktreepath)%1f%(committerdate:relative)%1f%(subject)",
	}, patterns...)
	out, err := runGit(ctx, env, s.Dir, args...)
	if err != nil {
		return nil, err
	}

	branches, err := parseBranches(out)
	if err != nil {
		return nil, err
	}
	return s.filter(branches), nil
}

func parseBranches(out string) ([]item, error) {
	var branches []item
	for _, record := range splitRecords(out, "\n") {
		values := strings.SplitN(record, "\x1f", 6)
		if len(values) != 6 {
			return nil, fmt.Errorf("git for-each-ref: unexpected record %q", record)
		}

		ref := values[0]
		if strings.HasPrefix(ref, "refs/remotes/") && strings.HasSuffix(ref, "/HEAD") {
			continue
		}

		fields := map[string]string{
			"ref":      ref,
			"name":     values[1],
			"upstream": values[2],
			"worktree": values[3],
			"date":     values[4],
			"subject":  values[5],
		}
		fields["remote"], fields["branch"] = splitBranchRef(ref)

		branches = append(branches, item{
			ID:          ref,
			Label:       fields["name"],
			Description: branchDescription(fields),
			Fields:      fields,
		})
	}
	return branches, nil
}

// splitBranchRef returns the remote name, empty for a local branch, and the
// branch name without the remote. A remote name is assumed to have no '/'.
func splitBranchRef(ref string) (remote, branch string) {
	if local, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return "", local
	}

	remoteBranch := strings.TrimPrefix(ref, "refs/remotes/")
	remote, branch, _ = strings.Cut(remoteBranch, "/")
	return remote, branch
}

func branchDescription(fields map[string]string) string {
	description := fields["date"] + "  " + fields["subject"]
	if fields["worktree"] != "" {
		description = "checked out in " + shortenHome(fields["worktree"]) + "  " + description
	}
	return description
}

func (s gitBranches) filter(branches []item) []item {
	local := map[string]bool{}
	for _, b := range branches {
		if b.Fields["remote"] == "" {
			local[b.Fields["branch"]] = true
		}
	}

	kept := make([]item, 0, len(branches))
	for _, b := range branches {
		isLocal := b.Fields["remote"] == ""

		switch {
		case isLocal && !boolOr(s.Local, true):
		case isLocal && s.ExcludeCheckedOut && b.Fields["worktree"] != "":
		case !isLocal && s.ExcludeShadowedRemotes && local[b.Fields["branch"]]:
		default:
			kept = append(kept, b)
		}
	}
	return kept
}

func (s gitWorktrees) list(ctx context.Context, env *env) ([]item, error) {
	out, err := runGit(ctx, env, s.Dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}

	current := ""
	if s.ExcludeCurrent {
		top, err := runGit(ctx, env, s.Dir, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, err
		}
		current = canonicalPath(strings.TrimSpace(top))
	}

	var items []item
	for index, entry := range parseWorktrees(out) {
		path := entry["worktree"]
		if s.skip(index, entry, current) {
			continue
		}

		fields := map[string]string{
			"path":   path,
			"branch": strings.TrimPrefix(entry["branch"], "refs/heads/"),
			"head":   entry["HEAD"],
			"main":   strconv.FormatBool(index == 0),
			"locked": strconv.FormatBool(hasAttr(entry, "locked")),
		}

		label := fields["branch"]
		if label == "" {
			label = "(detached " + shortHash(fields["head"]) + ")"
		}

		items = append(items, item{
			ID:          path,
			Label:       label,
			Description: shortenHome(path),
			Fields:      fields,
		})
	}
	return items, nil
}

func (s gitWorktrees) skip(index int, entry map[string]string, current string) bool {
	switch {
	case hasAttr(entry, "bare"), hasAttr(entry, "prunable"):
		return true
	case s.ExcludeMain && index == 0:
		return true
	case s.ExcludeLocked && hasAttr(entry, "locked"):
		return true
	case current != "" && canonicalPath(entry["worktree"]) == current:
		return true
	default:
		return false
	}
}

// parseWorktrees reads `git worktree list --porcelain -z`: NUL-terminated
// "key value" attributes, with an empty attribute between worktrees.
func parseWorktrees(out string) []map[string]string {
	var entries []map[string]string
	current := map[string]string{}

	for _, attribute := range strings.Split(out, "\x00") {
		if attribute == "" {
			if len(current) > 0 {
				entries = append(entries, current)
				current = map[string]string{}
			}
			continue
		}

		key, value, _ := strings.Cut(attribute, " ")
		current[key] = value
	}

	if len(current) > 0 {
		entries = append(entries, current)
	}
	return entries
}

func hasAttr(entry map[string]string, key string) bool {
	_, ok := entry[key]
	return ok
}

func runGit(ctx context.Context, env *env, dirParam string, args ...string) (string, error) {
	dir, err := expandParam(env, "dir", dirParam)
	if err != nil {
		return "", err
	}

	argv := []string{"git", "--no-pager"}
	if dir != "" {
		argv = append(argv, "-C", dir)
	}

	out, err := proc.Command{
		Argv:        append(argv, args...),
		Dir:         env.target.Dir(),
		Env:         env.environ(),
		Timeout:     builtinTimeout,
		StdoutLimit: maxSourceOutput,
	}.Output(ctx)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func splitRecords(out, separator string) []string {
	var records []string
	for _, record := range strings.Split(out, separator) {
		record = strings.TrimPrefix(record, "\n")
		if record != "" {
			records = append(records, record)
		}
	}
	return records
}

func canonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}

func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
