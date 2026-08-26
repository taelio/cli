# tael CLI

Command-line interface for [tael](https://tael.io), the AI DevOps platform.
Connect a repository, trigger deploys, follow logs, and check what is live —
without leaving the terminal.

## Install

With Go 1.26 or newer:

```sh
go install tael.io/cli@latest
```

Or build from source:

```sh
git clone https://github.com/tael-io/cli
cd cli
make build   # produces ./tael
```

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
| `tael status [app]` | Show an app's live status and health checks                    |
| `tael deploy [app]` | Trigger a deploy and print the deploy id                       |
| `tael deploys [app]`| List an app's deploy history                                   |
| `tael logs [app]`   | Print recent logs; `-f` streams new lines live                 |
| `tael open [app]`   | Open the app's live URL in the browser                         |
| `tael incidents`    | List incidents in the workspace                                |
| `tael init`         | Connect a repository (currently guides to the web app)         |
| `tael version`      | Show the CLI version and commit                                |

Commands taking `[app]` accept an app name or id. When the workspace has
exactly one app the argument can be omitted.

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
