@AGENTS.md

## Workflow policy

When a workflow fails (e.g., `/go-implement` failing because the sandbox is not enabled), report the error and tell the user how to fix it. Do not fall back to doing the work yourself.

When a workflow completes, always display the full `displaySummary` from the workflow result (including the summary table, verification report, and estimated costs). Then wait for the user's next instruction — do not autonomously fix issues, re-apply changes, or take any follow-up action.
