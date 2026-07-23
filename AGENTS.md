# Repository delivery requirements

These instructions apply to the entire repository.

## CI is a hard delivery gate

- Work is not complete while any required GitHub Actions check is failing or pending.
- Before every commit or push, run `git diff --check` and fix all whitespace errors, including trailing whitespace.
- For frontend changes, run `npm run build` before pushing. Run any additional tests or checks relevant to the changed area.
- After pushing, inspect the pull request checks with `gh pr checks`. If a GitHub Actions check fails, read the failing job logs, fix the root cause, rerun the local checks, commit, and push again.
- Continue monitoring the latest commit until every required GitHub Actions check passes. Do not report the task as complete merely because code was pushed.
- Treat warnings separately from failures, but record meaningful warnings and address them when they threaten future CI compatibility.

## Change safety

- Stage only files that belong to the current task. Never include unrelated working-tree changes in a CI-fix commit.
- Do not weaken, skip, or remove quality checks to make CI pass unless the user explicitly requests a policy change.
- Prefer the smallest root-cause fix that preserves existing behavior.
