# Issue tracker: GitHub

Issues and product requests for this repository live in GitHub Issues. Use the
`gh` CLI for issue operations.

## Conventions

- Create an issue: `gh issue create --title "..." --body "..."`.
- Read an issue: `gh issue view <number> --comments`.
- List issues: `gh issue list --state open` with appropriate filters.
- Comment, label, or close: use `gh issue comment`, `gh issue edit`, and
  `gh issue close`.

Infer the repository from `git remote -v`; `gh` does this automatically when
run inside this clone.

## Pull requests as a triage surface

PRs as a request surface: no.

## Skill mapping

When a skill says to publish to the issue tracker, create a GitHub issue. When
it says to fetch a ticket, run `gh issue view <number> --comments`.
