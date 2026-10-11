# Repository bootstrap prompt

Copy this prompt into a coding agent when starting a new Go repository:

```text
Bootstrap this repository as a deterministic, CI-ready Go project. Inspect the existing code first and preserve intentional project-specific configuration.

Use mise for all developer and CI tools. Add or update mise.toml with the pinned Go version and these tools: hk, golangci-lint, actionlint, pinact, zizmor, gosec, and govulncheck. Do not rely on globally installed tools.

Use hk as the canonical quality entry point. Add hk.pkl with fast checks for Go formatting, go vet, golangci-lint, `go test ./...`, go mod tidy validation, actionlint, pinact, and zizmor. Add a pre-push suite that extends the fast checks with gosec and govulncheck. Add a commit-msg hook requiring conventional commits. Keep checks deterministic and scoped to relevant files where possible.

Add AGENTS.md containing these project rules:
- Run `hk check` after every meaningful change and before handoff.
- Use `hk check --all` for whole-repository validation and `mise exec -- hk run pre-push` for the full security suite.
- Do not bypass failing checks; fix the cause or document an intentional exception.
- Use cmd/<program> for executable entry points, internal/<area> for private reusable code, and adjacent testdata for fixtures.
- Prefer TDD: write a focused failing test, implement the smallest fix, then refactor.
- Keep tests isolated and deterministic; avoid network, wall-clock, and machine-specific dependencies.
- For substantial rendered or serialized output, use strict byte-for-byte approval tests. Never rewrite approvals during normal tests; require explicit opt-in such as UPDATE_APPROVALS=1 to update approvals, review the diff, and rerun without the flag.
- Keep generated output deterministic by sorting collections, normalizing paths, and injecting clocks or IDs.

Add a minimal GitHub Actions CI workflow that runs hk check --all on pushes and pull requests. Pin every third-party action to a full commit SHA and include the version in a comment. Use least-privilege permissions and concurrency cancellation. Add Dependabot configuration for GitHub Actions.

Add .editorconfig, a suitable .gitignore, and an MIT license. Update README.md with setup instructions (`mise install`, `hk install`), the canonical check command, the full-suite command, and a short description of the tooling. If the repository is intended as a GitHub template, include a reusable copy of this prompt and configure the repository as a template through the GitHub API or repository settings.

Use the repository's actual module path and Go version. Run `mise install`, then `hk check --all`; fix all failures rather than weakening checks. Review all generated or modified files before finishing.
```
