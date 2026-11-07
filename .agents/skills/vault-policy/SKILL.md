---
name: vault-policy
description: Read supporting project designs, decisions, and research before substantial planning or implementation. Use vikunja for requested task tracking.
---

# Project Vault Policy

Use the `vikunja` skill when the user requests task tracking.
The board holds tracked work. Use this directory as the source of truth
for supporting project designs, decisions, research and historical context:

```text
~/.vaults/personal/ai/Projects/<project-name>/
```

Determine `<project-name>` from the Git repository root directory. Before creating a directory, match an existing project directory case-insensitively so differently capitalized duplicates are not created.

## Before Substantial Work

Before substantial planning or implementation work:

1. Read the project's `README.md` and `plan.md` when present.
2. Read `status.md` when present.
3. Treat existing `plan.md` and `status.md` files as historical context, not proof
   of current task status. When work refers to a board task, read it for live status.
4. Read only the relevant decisions, research, or archived context needed for the task.

## What Belongs in the Vault

Store supporting project material in the project vault instead of the application repository:

- Detailed designs and long-form plans, linked from the relevant board work.
- Architecture and product decisions.
- Dependency and technical research.
- Historical or superseded proposals.

Do not routinely store conversations. Create a conversation summary only when the user explicitly requests one or when it contains an important decision that is not captured in the plan or decision records.

Application source code, tests, package manifests, and normal code documentation remain in the application repository.

## After Substantial Work

After completing or changing planned work:

1. Follow the `vikunja` skill for requested board updates.
2. Update vault documents when designs, decisions or research change; link them
   from the board rather than duplicating its active task/status checklists.
3. Update `Last reviewed` when supporting documents change.
4. Preserve existing plans and status files. Migrate or archive historical
   backlogs only when the user requests it, retaining links to replacement work.

Never store credentials, tokens, provider URLs, authorization headers, personal secrets, or other sensitive values in the vault.

## Supporting Documents

```text
Projects/<project-name>/
  README.md
  plan.md
  decisions/
  research/
  archive/
```

Keep `plan.md` for detailed design when needed, not as a parallel task tracker.
Create only the files and directories the project actually needs. Do not create empty structure for its own sake.
