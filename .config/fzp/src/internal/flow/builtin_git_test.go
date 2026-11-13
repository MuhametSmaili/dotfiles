package flow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"fzp/internal/tmux"
)

// gitRepo creates a repository with two commits on main, a local "feature"
// branch checked out in a linked worktree, a local "idle" branch and a
// remote-tracking ref for each of "feature" and "remote-only".
func gitRepo(t *testing.T) (repo, worktree string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	repo = filepath.Join(root, "repo")
	worktree = filepath.Join(root, "feature")

	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Ada")
	t.Setenv("GIT_AUTHOR_EMAIL", "ada@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Ada")
	t.Setenv("GIT_COMMITTER_EMAIL", "ada@example.com")

	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git("init", "--quiet", "--initial-branch=main")
	git("commit", "--quiet", "--allow-empty", "-m", "first")
	git("commit", "--quiet", "--allow-empty", "-m", "second")
	git("branch", "idle")
	git("worktree", "add", "--quiet", "-b", "feature", worktree)
	git("update-ref", "refs/remotes/origin/feature", "HEAD")
	git("update-ref", "refs/remotes/origin/remote-only", "HEAD")
	git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/feature")
	return repo, worktree
}

func listIDs(t *testing.T, source source, env *env) []string {
	t.Helper()

	items, err := source.list(context.Background(), env)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

func TestGitCommitsListsNewestFirstInTheTargetDirectory(t *testing.T) {
	repo, _ := gitRepo(t)
	env := newEnv(os.Environ(), tmux.Target{Cwd: repo})

	items, err := gitCommits{}.list(context.Background(), env)

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 || items[0].Fields["subject"] != "second" || items[1].Fields["subject"] != "first" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].ID != items[0].Fields["hash"] || len(items[0].ID) != 40 || items[0].Fields["author"] != "Ada" {
		t.Errorf("first item = %+v", items[0])
	}
}

func TestGitBranchesFilters(t *testing.T) {
	repo, _ := gitRepo(t)
	env := newEnv(os.Environ(), tmux.Target{Cwd: repo})
	no := false

	tests := []struct {
		name   string
		source gitBranches
		want   []string
	}{
		{
			name:   "all branches without remote HEAD",
			source: gitBranches{},
			want: []string{
				"refs/heads/feature", "refs/heads/idle", "refs/heads/main",
				"refs/remotes/origin/feature", "refs/remotes/origin/remote-only",
			},
		},
		{
			name:   "unused local branches and new remote branches",
			source: gitBranches{ExcludeCheckedOut: true, ExcludeShadowedRemotes: true},
			want:   []string{"refs/heads/idle", "refs/remotes/origin/remote-only"},
		},
		{
			name:   "remote only, shadowed by local names",
			source: gitBranches{Local: &no, ExcludeShadowedRemotes: true},
			want:   []string{"refs/remotes/origin/remote-only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := listIDs(t, tt.source, env)

			if !slices.Equal(got, tt.want) {
				t.Errorf("branches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitBranchesFields(t *testing.T) {
	repo, worktree := gitRepo(t)
	env := newEnv(os.Environ(), tmux.Target{Cwd: repo})

	items, err := gitBranches{}.list(context.Background(), env)

	if err != nil {
		t.Fatalf("list: %v", err)
	}

	byID := map[string]item{}
	for _, it := range items {
		byID[it.ID] = it
	}

	feature := byID["refs/heads/feature"].Fields
	if feature["branch"] != "feature" || feature["remote"] != "" || canonicalPath(feature["worktree"]) != canonicalPath(worktree) {
		t.Errorf("feature fields = %v", feature)
	}

	remote := byID["refs/remotes/origin/remote-only"]
	if remote.Label != "origin/remote-only" || remote.Fields["branch"] != "remote-only" || remote.Fields["remote"] != "origin" {
		t.Errorf("remote item = %+v", remote)
	}
}

func TestGitWorktreesFilters(t *testing.T) {
	repo, worktree := gitRepo(t)

	tests := []struct {
		name   string
		cwd    string
		source gitWorktrees
		want   []string
	}{
		{name: "all", cwd: repo, source: gitWorktrees{}, want: []string{repo, worktree}},
		{name: "others from the main worktree", cwd: repo, source: gitWorktrees{ExcludeCurrent: true}, want: []string{worktree}},
		{name: "others from a linked worktree", cwd: worktree, source: gitWorktrees{ExcludeCurrent: true}, want: []string{repo}},
		{name: "removable", cwd: worktree, source: gitWorktrees{ExcludeCurrent: true, ExcludeMain: true}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv(os.Environ(), tmux.Target{Cwd: tt.cwd})

			got := listIDs(t, tt.source, env)

			canonical := func(paths []string) []string {
				out := make([]string, 0, len(paths))
				for _, p := range paths {
					out = append(out, canonicalPath(p))
				}
				return out
			}
			if !slices.Equal(canonical(got), canonical(tt.want)) {
				t.Errorf("worktrees = %v, want %v", got, tt.want)
			}
		})
	}
}
