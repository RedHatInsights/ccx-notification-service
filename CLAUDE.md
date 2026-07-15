@AGENTS.md

## Claude workflow policy

When a workflow fails (e.g., `/go-implement` failing because the sandbox is not enabled), report the error and tell the user how to fix it. Do not fall back to doing the work yourself.

When a workflow completes, display the **entire** `displaySummary` from the workflow result **verbatim** — never truncate, summarize, or omit any section. This includes the summary table, the full verification report (every acceptance criterion, design alignment, BDD alignment, test quality, bug check, make before_commit result, test count verification, files modified, verdict), all verification deviations, all agent feedback on workflow instructions, and the estimated costs table. Copy the content as-is; do not paraphrase or shorten it. Then wait for the user's next instruction — do not autonomously fix issues, re-apply changes, or take any follow-up action.
