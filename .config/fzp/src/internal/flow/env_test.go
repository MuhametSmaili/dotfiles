package flow

import (
	"slices"
	"strings"
	"testing"

	"fzp/internal/tmux"
)

func TestFlowEnvExportsTargetAndSelections(t *testing.T) {
	env := newEnv([]string{"HOME=/home/me", "FZP_STEP_OLD_ID=stale"}, tmux.Target{PaneID: "%7", Cwd: "/work"})
	env.push("worktree", item{ID: "/w", Label: "main", Fields: map[string]string{"branch": "main", "path": "/w"}})

	got := env.environ()

	want := []string{
		"HOME=/home/me",
		"FZP_PANE_ID=%7",
		"FZP_PANE_CWD=/work",
		"FZP_STEP_WORKTREE_ID=/w",
		"FZP_STEP_WORKTREE_LABEL=main",
		"FZP_STEP_WORKTREE_BRANCH=main",
		"FZP_STEP_WORKTREE_PATH=/w",
	}
	for _, entry := range want {
		if !slices.Contains(got, entry) {
			t.Errorf("environment is missing %q", entry)
		}
	}

	if slices.Contains(got, "FZP_STEP_OLD_ID=stale") {
		t.Error("an inherited FZP_STEP_ variable leaked into the flow")
	}
}

func TestFlowEnvTruncateForgetsLaterSelections(t *testing.T) {
	env := newEnv(nil, tmux.Target{})
	env.push("first", item{ID: "a"})
	env.push("second", item{ID: "b"})

	env.truncate(1)

	if _, ok := env.lookup("FZP_STEP_SECOND_ID"); ok {
		t.Error("FZP_STEP_SECOND_ID survived truncation")
	}
	if value, _ := env.lookup("FZP_STEP_FIRST_ID"); value != "a" {
		t.Errorf("FZP_STEP_FIRST_ID = %q, want a", value)
	}
}

func TestFlowEnvExpand(t *testing.T) {
	env := newEnv(nil, tmux.Target{SessionID: "$3"})
	env.push("branch", item{ID: "refs/heads/x", Label: "x"})

	got, err := env.expand("Remove ${FZP_STEP_BRANCH_LABEL} from $FZP_SESSION_ID?")

	if err != nil || got != "Remove x from $3?" {
		t.Errorf("expand = %q, %v", got, err)
	}

	_, err = env.expand("$FZP_STEP_BRANCH_TYPO")

	if err == nil || !strings.Contains(err.Error(), "unset variable FZP_STEP_BRANCH_TYPO") {
		t.Errorf("error = %v, want an unset variable error", err)
	}
}
