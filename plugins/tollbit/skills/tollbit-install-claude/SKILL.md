---
name: tollbit-install-claude
description: Install and set up the tollbit CLI in Claude's sandbox (Claude in the desktop or web app, chat or Cowork). Use before the first TollBit command in a new session, and when a `tollbit` command fails with "command not found".
---

# Install and set up the TollBit CLI in Claude

These steps are for Claude in the desktop or web app, in chat or Cowork. There, commands run in a temporary sandbox whose network goes through a proxy. Each new session starts without the CLI, so do both steps in every new session, before any other `tollbit` command.

If you run on the user's own computer instead (for example, Claude Code in a terminal), these steps don't apply. Install the CLI as https://github.com/tollbit/cli#install describes.

## 1. Install the CLI

If `tollbit version` fails, install the CLI. Tell the user you are installing it before you start.

```bash
npm install -g @tollbit/tollbit-cli
tollbit version
```

The npm package downloads the CLI release from GitHub through the sandbox's proxy (`HTTPS_PROXY`) and checks it against the release's SHA-256 checksums.

- If `tollbit` is not found afterwards, run `hash -r` and try again.
- If the install fails with `no data for 60s`, the download is not getting through the proxy. Treat it like a blocked host (below).
- If npm is not available, use the installer script instead:

  ```bash
  installer="$(mktemp)"
  curl -fsSL -o "$installer" "https://raw.githubusercontent.com/tollbit/cli/v0.4.3/scripts/install.sh"
  bash "$installer" --no-modify-path
  ~/.local/bin/tollbit version
  ```

  Then run the CLI as `~/.local/bin/tollbit`.

If a download or a `tollbit` command fails with a 403 or a blocked-host error, the organization's network settings are blocking it. Tell the user that an admin needs to change **Organization settings → Capabilities → Code execution** to allow package managers and GitHub, plus `oauth.tollbit.com` and `gateway.tollbit.com` under **Additional allowed domains** (or allow all domains), and then start a new session. Don't install from any other source.

## 2. Choose end-user proximity

The CLI picks how the user authorizes it from one setting, end-user proximity: whether the CLI runs on the same machine as the user's browser.

- If you know it doesn't (Claude's sandbox usually doesn't), set `remote`:

  ```bash
  tollbit runtime set --end-user-proximity remote
  ```

- If you know it does, set `local`.
- If you aren't sure, set nothing. The CLI makes its best guess.

`tollbit runtime status` shows the value in use.

If `tollbit auth login` fails with `authorization timed out`, ask the user what happened:

- They didn't get to it in time: run `tollbit auth login` again.
- No page opened for them, or they approved and their browser then couldn't connect: their browser isn't on this machine. Run `tollbit runtime set --end-user-proximity remote` and log in again.

With `remote`, `tollbit auth login` prints a link. Give it to the user, and ask them to read back the three icons the page shows. Then run `tollbit auth complete` with those names, as the tollbit-cli skill describes.
