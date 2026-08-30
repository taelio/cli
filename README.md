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

A token is made inside one workspace and acts there. `tael workspace use
<slug>` records a choice (`workspace_id` in `~/.tael.yaml`, sent as the
`X-Tael-Workspace-Id` header) and keeps it only when the API honours it for
your token; otherwise it says so and how to get a token for that workspace.

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
| `tael tasks new "<brief>" [--app] [--kind]` | Ask Tael to look into something, in your words     |
| `tael task <id>`    | One task: its plan, change, evidence, outcome and comments     |
| `tael task comment <id> "<text>"` | Leave a comment on a task, for Tael and the team |
| `tael needs-you`    | What is waiting on you: decisions Tael asked for, and proposals |
| `tael approve <id> [--note]` | Say yes to what a task is waiting on (or to a proposal) |
| `tael decline <id> [--note]` | Say no                                                |
| `tael why [app]`    | Why the last deploy failed; asks Tael and follows the investigation when nothing failed yet |
| `tael ask "<question>" [--app]` | Ask Tael a question; the answer streams, with what Tael looked at along the way |
| `tael feed [-f] [--last N] [--since <id>]` | What Tael is doing, one line each in its own words; `-f` keeps listening |
| `tael pause` / `tael resume` | Stop Tael starting or carrying out anything, and let it work again |
| `tael settings ai`  | How much Tael may do on its own: who approves, what runs unasked, quiet hours |
| `tael settings ai --approvers admins\|members --pre-approve category=N --quiet-hours HH:MM-HH:MM[@Zone] --clear-quiet-hours` | Change it (owners and admins only) |
| `tael plan`         | The workspace's plan, what it holds, the runtime, and any coupon in force (with a sentence, see Architecture below) |
| `tael coupon [code]` | Apply a coupon code; alone, the coupon in force ("TAEL-XXXX applied — Launch until 28 Feb 2027 · 5 apps · 20M AI tokens") |
| `tael usage`        | The meters this period: apps, seats, AI tokens (with "(coupon)" when the allowance comes from one, "part estimated" when some tokens were estimated), custom domains, and the deploys and builds so far |
| `tael tokens`       | Your API tokens for this workspace (never their secrets)       |
| `tael tokens create <name> [--expires 30d\|YYYY-MM-DD]` | Make a token; the secret is printed this once |
| `tael tokens revoke <id or name>` | Make a token stop working                          |
| `tael solutions list` | List the Tael Managed solutions installed in the workspace   |
| `tael solutions add <key> [--for <app>] [--size small\|medium\|large]` | Add one from the catalog: `postgres`, `monitoring`, `object-storage`, `backups`, `secrets` |
| `tael solutions status <name>` | Show a solution's live status and checks             |
| `tael solutions connect <name> --app <app>` | Connect a solution to an app; it reads the values on its next deploy |
| `tael solutions remove <name> [--force]` | Remove a solution; stored data is deleted and volumes are released |
| `tael solutions catalog` | What can be added, with the plan gate and sizes                |
| `tael solutions connection <name>` | The connection an app reads, secrets masked (revealing is a browser action) |
| `tael solutions upgrade <name>` | Apply the newer version Tael publishes                      |
| `tael solutions retry <name>` | Run a failed install again                                    |
| `tael digest [--days N]` | What happened over the last days in Tael's words, then the numbers; says when the reading is still being written |
| `tael suggestions [--all]` | What Tael noticed without being asked; `tael suggestions resolve <id>` marks one dealt with |
| `tael workspaces`   | Every workspace you are in, with the one the CLI acts in marked |
| `tael workspace use <slug>` | Act in another workspace, where the token can follow (see below) |
| `tael members`      | Who is in the workspace and how it admits people               |
| `tael members remove <user>` | Take a person out (by GitHub login, email, name or id); owners and admins only |
| `tael member role <user> <role>` | Change a person's role to owner, admin or member; owners and admins only |
| `tael invite link [--max-uses N]` | Make a join link anyone can use (`--role member\|admin`)   |
| `tael invite email <address>` / `tael invite github <login>` | Invite one person; the join link is shown once |
| `tael invites`      | List invitations; `tael invites revoke <id>` stops one working |
| `tael team join-policy [--github-org on\|off]` | Show or change how people join: by invitation only, or anyone with access to the GitHub repositories |
| `tael repos`        | The repositories Tael can see, ready for `tael new`            |
| `tael new --repo owner/name [--branch] [--name] [--database] [--go-live] [--no-follow]` | Put a repository live: Tael reads it, sets it up and follows the setup until the pull request is ready |
| `tael init`         | How to connect a repository (installing the GitHub App is a browser step) |
| `tael version`      | Show the CLI version and commit                                |

Commands taking `[app]` accept an app name or id. When the workspace has
exactly one app the argument can be omitted. A solution's `<name>` is its
display name, its instance name or its id.

### Architecture

The workspace as one picture, a change planned from a sentence, and the
build that carries it out. Nothing runs until you say `tael build`.

| Command             | Description                                                    |
| ------------------- | -------------------------------------------------------------- |
| `tael architecture [--app <app> \| --stack <stack>]` | The picture in text: addresses, stacks, apps, solutions and the runtime, each with what it connects to, then Tael's suggestions; `--app` narrows it to one app — its repository, addresses, solutions and the apps it calls — and `--stack` to one stack's apps |
| `tael plan "<what you want>" [--app <app> \| --stack <stack>] [--json]` | Ask Tael to plan a change in your words: the summary, the changes as numbered rows (with why one is blocked), any questions; kept as the last plan. `--app`/`--stack` scope the plan to that slice of the workspace |
| `tael plan "<what you want>" --build [--yes]` | Plan and build in one go, after the same question |
| `tael build [--plan <file>] [--app <app> \| --stack <stack>] [--yes]` | Carry out the last plan (or the file given): shows the changes, asks on a terminal unless `--yes`, then says what is happening for each; blocked changes are skipped, refusals are said in a sentence and exit 1 |

`tael plan` keeps the plan at `~/.tael/last-plan.json` (beside the config
file when `TAEL_CONFIG` names one); `tael build --plan` also reads what
`tael plan -o json` printed. Without a terminal and without `--yes`,
`tael build` refuses (exit 2) rather than guess. On a deployment with no
model to plan with, `tael plan` says so and exits 1.

#### Stacks and links

A stack is a named group of apps that ship together; an app belongs to at
most one. The workspace picture shows each stack as one row with its apps
indented beneath, and apps outside any stack exactly as before. A link
declares that one app calls another — a line the picture draws and the
planner reads; nothing runs because of it.

| Command             | Description                                                    |
| ------------------- | -------------------------------------------------------------- |
| `tael stacks`       | List the stacks: name, app count, members                      |
| `tael stack new <name> [--app <app> ...]` | Make a stack, with apps in it from the start (`--app` repeats) |
| `tael stack move <app> <stack>` | Move an app into a stack; `tael stack move <app> --none` puts it on its own again |
| `tael stack rename <stack> <name>` | Give a stack a new name                         |
| `tael stack remove <stack> [--yes]` | Remove a stack; its apps stay, ungrouped (asks on a terminal unless `--yes`) |
| `tael link <from-app> <to-app> [--label <text>]` | Say one app calls another, with how in a word (REST, gRPC, a queue) |
| `tael unlink <from-app> <to-app>` | Take that back                                   |

Stacks and apps are named the way `[app]` arguments are everywhere else:
by name or by id, and an unknown one answers with what there is.

### What stays in the browser

A few actions are deliberately only available to a signed-in browser
session, so the CLI does not offer them:

- **Installing the Tael GitHub App** (connecting GitHub, picking the
  repositories Tael may see) is a round trip through github.com. Do it once
  from the web app; `tael repos` then lists what Tael can see.
- **Revealing a solution's connection values.** `tael solutions connection`
  shows the names with the secrets masked; the values are only ever shown
  in the web app, and every reveal is on the record. Apps read the real
  values on deploy without anyone seeing them.
- **Creating a workspace and switching the browser's workspace.** A token
  is made inside one workspace; see `tael workspace use` above for how the
  CLI handles a choice the token cannot follow.
- **Signing in and out of the web app itself.** `tael login` has its own
  browser-approved flow and makes a token for the workspace you are in.

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
