# gascity-irc-pack

An IRC bridge to Gas City, built as a deterministic, CI-ready Go project.

## Development

Install the pinned toolchain and tools, then install the hk hooks:

```sh
mise install
hk install
```

Run the canonical local checks:

```sh
hk check
```

Run the full suite, including security checks:

```sh
mise exec -- hk run pre-push
```

The repository uses mise to pin developer tools and hk to provide formatting, vetting, linting, tests, module-tidy validation, and GitHub Actions checks. Commits use conventional commit messages.

<!-- delivery workflow smoke test -->
