---
name: vikunja
description: Find, discuss, create, or update Vikunja tasks, projects, and assignments when the user requests board work.
compatibility: Board reads and writes require a connected Vikunja MCP server.
---

# vikunja

Use the connected `vikunja` MCP unless the user requests another tracker.

## Agree on the work

Change the board only when the user asks or approves a proposal. An implementation
request is not permission to create tasks or publish progress updates.

When shaping new tasks, discuss the result, why it matters, and what counts as
complete. Propose the smallest useful set and wait for approval. If the user asks
to create tasks directly, do so; ask only about gaps that change scope or destination.

## Write for humans

- Use ASD-STE100 Simplified Technical English principles: short sentences,
  active voice, one idea per sentence, and consistent terms.
- Give each task a clear action title. Describe the result, reason, and completion
  condition in only as much detail as the reader needs. No fixed template.
- Keep related work together. Split tasks only when separate tracking is useful.
- Keep agent instructions and execution logs out of task descriptions. Link to
  supporting designs or research when needed; use `vault-policy` for those files.

Example: **Remember the selected theme** — Keep the last selected theme after
restart so users do not need to select it again.

## Save requested changes

1. Find the existing project and check for a matching task before creating one.
   Ask if the destination or match is unclear. Create projects only when approved.
2. Change only the requested scope, including assignments, dates, and access.
   Confirm the intended user and project before assignment or sharing.
   Use `format: "markdown"` for rich-text reads and writes.
3. Verify the saved result and return its link from the configured board instance.
   After a write timeout, read the current state before retrying. Report blockers.
   Mark work complete only when the agreed scope is verified.
