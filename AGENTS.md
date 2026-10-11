# AGENTS.md

## Project

This is a Go project. Keep behavior deterministic, testable, and suitable for CI.

## Tooling and validation

Use [mise](https://mise.jdx.dev/) for the toolchain and [hk](https://hk.jdx.dev/) for local quality checks.

After every meaningful change, run the canonical check:

```sh
hk check
```

`hk check` runs formatting, vetting, linting, tests, module-tidy validation, and applicable GitHub Actions checks. Use these broader scopes when appropriate:

```sh
hk check --all                 # check the whole repository
mise exec -- hk run pre-push  # full suite, including security checks
```

The pre-push suite includes `gosec` and `govulncheck`; do not bypass a failing check. Fix the cause or explain an intentional exception explicitly. Review files changed by any fixer before committing.

The commit-message hook requires conventional commits. Install hooks locally with:

```sh
mise install
hk install
```

## Go layout

Use the conventional Go layout:

```text
cmd/<program>/main.go       # executable entry points
internal/<area>/             # implementation packages private to this module
internal/<area>/*_test.go    # package tests
<package>/*_test.go          # tests for an intentional root package
testdata/                    # fixtures and approval files, adjacent to their package
```

- Keep application wiring in `cmd`; put reusable implementation in `internal`.
- Prefer `internal/` over a new public package. Add `pkg/` only when it is deliberately part of the external API.
- Keep generated files and test fixtures deterministic. Do not commit build artifacts.

## TDD

For behavior changes:

1. Add or update a focused test that fails for the missing behavior.
2. Implement the smallest change that makes it pass.
3. Refactor with tests still passing.
4. Run `hk check` before handing off the work.

Prefer table-driven tests and stable package APIs. Keep tests isolated, deterministic, and independent of network services, wall-clock time, and machine-specific paths.

## Strict approval testing

Use approval/golden tests for substantial rendered or serialized output:

- Compare complete canonical output byte-for-byte, including meaningful whitespace and trailing newlines.
- Store reviewed approvals in `testdata/` near the package under test.
- Never rewrite approvals or silently accept mismatches during a normal test run.
- Require explicit opt-in such as `UPDATE_APPROVALS=1` to update approvals, then review the diff and rerun without the flag.
- Sort maps and collections, normalize paths, and inject clocks or IDs before approving output.
- Fail on missing, unexpected, or stale approval files unless removal is intentional and reviewed.
