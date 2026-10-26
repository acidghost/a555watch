# a555watch

A 555 modern `watch` replacement with an interactive output history and line/character diffs.

## Installation

Download a binary for macOS or Linux (amd64/arm64) from
[Releases](https://github.com/acidghost/a555watch/releases), or install from source:

```sh
go install github.com/acidghost/a555watch@latest
```

## Usage

```sh
a555watch -n 1s -- git status --short
a555watch --no-tui --errexit -- sh -c 'date; uptime'
```

The command and its arguments are executed directly, not through a shell. Use `sh -c` explicitly for
pipes, redirects, or other shell syntax. Options for `a555watch` must precede the watched command.

In the TUI, press `?` for help, `tab` to switch between output and history, `d` for line/character
diffs, `p` to pause, `f` to follow the latest output, and `q` to quit.

### Options

<!--[[[cog
import cog
import subprocess
out = subprocess.run(
    ["./a555watch", "--help"],
    cwd="build",
    check=True,
    capture_output=True,
    text=True,
)
lines = [line.rstrip() for line in out.stdout.splitlines()]
lines = lines[1:-1]
help = "\n".join(lines)
cog.out(f"```\n{help}\n```")
]]]-->
```
    ┌───────────────────────────────────────────┐
    │                                           │
    │                                           │
    │                                           │
    │                             .      .      │
    │   ,-. .-- .-- .-- . , , ,-. |- ,-. |-.    │
    │   ,-| `-. `-. `-. |/|/  ,-| |  |   | |    │
    │   `-^ `-' `-' `-' ' '   `-^ `' `-' ' '    │
    │                                           │
    │                                           │
    │                                           │
    │                                           │
    └───────────────────────────────────────────┘

 ./a555watch [options] command

   -n, --interval duration   time to wait between updates (default 2s)
   -e, --errexit             exit if command has a non-zero exit
   -g, --chgexit             exit when the output of command changes
       --no-tui              do not use the TUI
       --no-alt              do not start the TUI in alt screen
       --log string          write debug logs to file
       --debug               enable tracing logs
   -h, --help                display this help and exit
   -V, --version             show binary version
```
<!--[[[end]]]-->

## Development

The toolchain is pinned in `mise.toml` and `mise.lock`, matching
[go-start](https://github.com/acidghost/go-start). Mise replaces the old Nix flake and Nix-based
devcontainer setup.

```sh
mise trust
mise install
mise exec -- just lint
mise exec -- just build-all
```

With mise activated in your shell, run `just` directly:

- `just build` / `just b`: build the native binary with version metadata.
- `just build-all`: build Darwin/arm64, Linux/arm64, and Linux/amd64 binaries.
- `just run` / `just r`: build and run; `just run-local` remains an alias.
- `just install`: install to `go env GOBIN` (must be configured and exist).
- `just fmt`, `just lint`: format and lint Go code.
- `just vendor`: tidy modules and regenerate the checked-in `vendor/` tree.
- `just update-readme`: refresh the options above (requires `uv`; uses pinned cogapp).
- `just clean`, `just help`: remove build output or list recipes.

Keep `go.mod`, `go.sum`, and `vendor/` in sync when updating dependencies. Renovate manages
dependencies, GitHub Actions, and tool versions; the LCM workflow approves Renovate PRs carrying the
shared config's `lcm/approve` label. Enable the Renovate GitHub App for this repository if it is not
already enabled.

## CI and releases

CI checks formatting, lint, all three `just build-all` targets, and CLI smoke tests. Workflows use
the same mise toolchain as local development, SHA-pinned actions, and scoped token permissions.

- Pushing a `v*` tag publishes a GoReleaser release with `SHA256SUMS` and GitHub artifact
  attestations.
- Release builds do not restore toolchain or Go caches.

Verify downloaded assets from the directory containing the release files:

```sh
sha256sum -c SHA256SUMS # macOS: shasum -a 256 -c SHA256SUMS
gh attestation verify <archive>.tar.gz --repo acidghost/a555watch
```

Unlike the server-oriented template, this project does not ship a scratch container: the watched
commands must be available in its runtime environment.
