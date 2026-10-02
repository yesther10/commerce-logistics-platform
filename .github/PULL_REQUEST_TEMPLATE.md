## Linked issue

Closes #N

The linked issue must be approved before merge.

## Type — select exactly one

- [ ] `type:bug` — Bug fix
- [ ] `type:feature` — New feature
- [ ] `type:docs` — Documentation only
- [ ] `type:refactor` — Code refactoring
- [ ] `type:chore` — Maintenance or tooling
- [ ] `type:breaking-change` — Breaking change

## Summary

-

## Changes

| File or area | Change |
| --- | --- |
|  |  |

## Test plan

- [ ] `docker compose --env-file .env.example config --quiet`
- [ ] `docker compose --env-file .env.example --profile app config --quiet`
- [ ] Focused tests and checks are listed here with their exact results.
- [ ] Shellcheck was run if this PR changes shell scripts; otherwise, this is `N/A`.
- [ ] Manual or integration verification is listed here, or `N/A` with a reason.

## Chain context

| Field | Value |
| --- | --- |
| Chain |  |
| Tracker PR |  |
| Position |  /  |
| Base | `develop` or immediate parent branch |
| Depends on |  |
| Follow-up |  |
| Review budget |  / 400 changed lines |
| Starts at |  |
| Ends with |  |

### Chain overview

```text
develop
  └── #NNN Previous PR
       └── 📍 #NNN This PR
            └── #NNN Next PR
```

### Scope

- Includes:
- Excludes:

## Rollback boundary

State exactly which files or behavior can be reverted without removing unrelated work. If rollback is not applicable, explain why.

## Checklist

- [ ] The linked issue is approved and referenced with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- [ ] Exactly one `type:*` label is assigned to this PR.
- [ ] CI is green or the failing check is explained before review.
- [ ] The PR contains one deliverable work unit.
- [ ] The changed-line budget is at or below 400, or a maintainer approved a documented exception.
- [ ] Tests and relevant documentation are included with this work unit.
- [ ] The commit messages follow Conventional Commits.
- [ ] No AI commits or `Co-Authored-By` trailers are included.
