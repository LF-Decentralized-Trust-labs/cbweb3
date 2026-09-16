# CLAUDE.md

Instructions for Claude Code working in this repository.

**Read [AGENTS.md](AGENTS.md) first.** It is the authoritative, vendor-neutral set of rules
for AI agents here — project context, repository layout, disclosure, authorship, review and
accountability — and it applies to Claude in full. This file adds only what is specific to
running Claude Code.

## Commits

- **Always commit with `git commit -s`.** DCO sign-off is enforced by an organisation
  ruleset, so a commit without a `Signed-off-by` trailer cannot merge. This applies to
  every commit you create, and to the operations that rewrite one: `git commit --amend -s`,
  `git rebase --signoff`, `git cherry-pick -s`.

- **Sign off as the human who directs the commit, never as anyone else.** `-s` uses the
  local `user.name` and `user.email`, which is what you want: that person is submitting the
  work and is the one certifying the [DCO](https://developercertificate.org/). Writing a
  sign-off in a third party's name is forbidden — see AGENTS.md, "Disclosure and
  authorship". When you import or replay someone else's commits, their authorship stays in
  the `Author` field and the submitter signs off.

- **Disclose AI assistance** with an `Assisted-by:` trailer, and never list an AI as an
  author or co-author. AGENTS.md gives the format.

A commit you create therefore ends with both trailers:

```text
Assisted-by: anthropic:claude-opus-5
Signed-off-by: Jane Doe <jane@example.org>
```

## Scope

`platform/` carries its own [CLAUDE.md](platform/CLAUDE.md) and `.claude/` from the
upstream platform repository; those apply when working inside `platform/`. This file and
AGENTS.md apply repository-wide.
