# tael CLI

Command-line interface for [tael](https://tael.io), the AI DevOps platform.
Connect a repository, trigger deploys, follow logs, and check what is live —
without leaving the terminal.

## Install

Homebrew (macOS and Linux):

```sh
brew install taelio/tap/tael
```

Or with the install script, which resolves the latest release, verifies its
SHA-256 checksum, and installs to `/usr/local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/taelio/cli/main/install.sh | bash
```

Pin a version or change the destination:

```sh
TAEL_VERSION=v0.2.0 INSTALL_DIR="$HOME/.local/bin" \
  bash <(curl -fsSL https://raw.githubusercontent.com/taelio/cli/main/install.sh)
```

Windows binaries are published on the
[releases page](https://github.com/taelio/cli/releases).

Build from source with Go 1.26 or newer:

```sh
git clone https://github.com/taelio/cli
cd cli
make build   # produces ./tael
```

### In a cluster

Tael runs the CLI as an operations toolbox pod so you can execute commands
against the platform from inside a cluster:

```sh
kubectl exec -it deploy/tael-cli -- tael status
```

The pod idles and binds no port, so it has no Service and no Ingress.

## Login

```sh
tael login
```

This opens your browser to the tael sign-in page. Approve the login there and
the CLI saves your credentials to `~/.tael.yaml` (created with `0600`
permissions). For CI or scripts, skip the browser flow and pass a token
directly with `--token` or the `TAEL_API_TOKEN` environment variable.

## Configuration

Every setting resolves with **flag > environment > config file > default**
precedence.

| Setting   | Flag          | Environment      | `~/.tael.yaml` key | Default               |
| --------- | ------------- | ---------------- | ------------------ | --------------------- |
| API token | `--token`     | `TAEL_API_TOKEN` | `token`            | —                     |
| Base URL  | `--base-url`  | `TAEL_BASE_URL`  | `base_url`         | `https://api.tael.io` |
| Workspace | `--workspace` | `TAEL_WORKSPACE` | `workspace`        | —                     |

All commands accept `-o json` to print the raw API response instead of the
text rendering (default `-o text`).

## Commands

| Command             | Description                                                    |
| ------------------- | -------------------------------------------------------------- |
| `tael login`        | Authenticate in the browser and save credentials               |
| `tael logout`       | Remove the saved token from `~/.tael.yaml`                     |
| `tael whoami`       | Show the authenticated user and workspace                      |
| `tael apps` (`ps`)  | List apps: name, status, URL, last update                      |
| `tael app [app]`    | One app: status, address, repository, framework, last deploy and checks |
| `tael status [app]` | Show an app's live status and health checks                    |
| `tael deploy [app]` | Trigger a deploy and print the deploy id                       |
| `tael deploys [app]`| List an app's deploy history                                   |
| `tael logs [app]`   | Print recent logs; `-f` streams new lines live                 |
| `tael open [app]`   | Open the app's live URL in the browser                         |
| `tael domains`      | Every app's web address, with the live ones marked             |
| `tael setup [app]`  | Where an app's setup stands: what Tael read and wrote, the setup pull request |
| `tael go-live <app>` | Merge the setup pull request so the first deploy starts       |
| `tael retry <app>`  | Run a failed setup again from the step that failed             |
| `tael remove <app> --yes` | Take an app out of Tael; the repository is untouched     |
| `tael pipeline [app] [--set step.setting=value]` | Show the pipeline, or change a step's setting (Tael opens a pull request with it) |
| `tael incidents`    | List incidents in the workspace                                |
| `tael tasks [--done] [app]` | What Tael is doing, has done, or needs you for; `--done` lists finished tasks |
| `tael task <id>`    | One task: its plan, change, evidence, outcome and comments     |
| `tael approve <id> [--note]` | Say yes to what a task is waiting on (or to a proposal) |
| `tael decline <id> [--note]` | Say no                                                |
| `tael why [app]`    | Why the last deploy failed; asks Tael and follows the investigation when nothing failed yet |
| `tael pause` / `tael resume` | Stop Tael starting or carrying out anything, and let it work again |
| `tael solutions list` | List the Tael Managed solutions installed in the workspace   |
| `tael solutions add <key> [--for <app>] [--size small\|medium\|large]` | Add one from the catalog: `postgres`, `monitoring`, `object-storage`, `backups`, `secrets` |
| `tael solutions status <name>` | Show a solution's live status and checks             |
| `tael solutions connect <name> --app <app>` | Connect a solution to an app; it reads the values on its next deploy |
| `tael solutions remove <name> [--force]` | Remove a solution; stored data is deleted and volumes are released |
| `tael repos`        | The repositories Tael can see, ready for `tael new`            |
| `tael new --repo owner/name [--branch] [--name] [--database] [--go-live] [--no-follow]` | Put a repository live: Tael reads it, sets it up and follows the setup until the pull request is ready |
| `tael init`         | How to connect a repository (installing the GitHub App is a browser step) |
| `tael version`      | Show the CLI version and commit                                |

Commands taking `[app]` accept an app name or id. When the workspace has
exactly one app the argument can be omitted. A solution's `<name>` is its
display name, its instance name or its id.

## Exit codes

| Code | Meaning                                        |
| ---- | ---------------------------------------------- |
| 0    | Success                                        |
| 1    | API or runtime failure                         |
| 2    | Usage error (bad flags, arguments, subcommand) |
| 3    | Missing, expired, or rejected credentials      |

Errors are written to stderr; command output stays on stdout.

## Development

```sh
make build   # build ./tael with version/commit ldflags
make test    # go test -race ./...
make vet     # go vet ./...
make lint    # golangci-lint, skipped when not installed
make all     # all of the above
```
