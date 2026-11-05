**Set up TollBit.** These instructions install the TollBit CLI (or tell you how to connect to TollBit another way) and show how to use it. The right steps depend on which AI product and surface you are running in, so first work out your platform, as described below. Don't install or change anything until you have.

Many products ask the user to approve each command. Before each step, tell the user in plain words what you're about to do and why, so they know what they're approving.

## Work out your platform

Users often get their platform wrong, so decide from your own evidence. If the user has said which platform they're on, treat that as a claim to check, not as evidence. Your evidence is your own instructions and, if you can run commands, your environment.

1. Read your own instructions. Many products name themselves and their surface there, for example "You are Claude Code", "You are running inside the Codex (desktop) app" or "You operate in Cursor". A product name alone ("You are Codex") doesn't say which surface you're on.
2. If you can run shell commands, first tell the user in plain words what you're about to do and why: you're checking which AI product you're running in, so you can pick the right install, and it only reads settings, no passwords or secrets. Then run the command below. It only reads: it lists the names of agent-related environment variables, and prints values only for a few variables that name the agent or app, so it shows no secrets.

   On macOS or Linux:

   ```bash
   uname -sm; echo "user=$(id -un)"
   echo "names: $(env | cut -d= -f1 | grep -E '^(AI_AGENT|CLAUDE|CODEX|CURSOR|COPILOT|GEMINI|ANTIGRAVITY|TERM_PROGRAM|VSCODE|__CFBundleIdentifier|HTTPS?_PROXY)' | sort | tr '\n' ' ')"
   for v in CLAUDE_CODE_ENTRYPOINT CURSOR_AGENT TERM_PROGRAM __CFBundleIdentifier; do if printenv "$v" >/dev/null; then echo "$v=$(printenv "$v")"; fi; done
   ```

   On Windows (PowerShell):

   ```powershell
   "$([Environment]::OSVersion.VersionString) user=$env:USERNAME"
   "names: " + ((Get-ChildItem Env: | Where-Object Name -Match '^(AI_AGENT|CLAUDE|CODEX|CURSOR|COPILOT|GEMINI|ANTIGRAVITY|TERM_PROGRAM|VSCODE|HTTPS?_PROXY)' | Sort-Object Name).Name -join ' ')
   foreach ($v in 'CLAUDE_CODE_ENTRYPOINT','CURSOR_AGENT','TERM_PROGRAM') { $x = [Environment]::GetEnvironmentVariable($v); if ($null -ne $x) { "$v=$x" } }
   ```

3. Compare what you found with these hints. They are what each platform showed when we tested it. Products change them, so they are hints, not rules: weigh them with your instructions. The last column names the install section to follow.

   | Platform | What we saw | Install |
   |---|---|---|
   | Antigravity (desktop app and IDE) | `ANTIGRAVITY_*` variables, `__CFBundleIdentifier=com.google.antigravity`, no `TERM_PROGRAM` | Install: script |
   | Antigravity CLI | `ANTIGRAVITY_*` variables, and `TERM_PROGRAM` set | Install: script |
   | ChatGPT Work | untested: your instructions name ChatGPT Work | Install: MCP (coming soon) |
   | Claude chat (desktop app) or Claude chat (web) | `CLAUDE_CODE_ENTRYPOINT=remote_cowork`, in a cloud Linux container | Install: Claude plugin |
   | Claude Code (desktop app) | `CLAUDE_CODE_ENTRYPOINT=claude-desktop` | Install: script |
   | Claude Code (terminal) | `CLAUDE_CODE_ENTRYPOINT=cli` | Install: script |
   | Claude Code on the web | `CLAUDE_CODE_ENTRYPOINT=remote`, in a cloud Linux container | Install: script |
   | Claude Duet | untested: your instructions name Claude Duet | Install: MCP (coming soon) |
   | Codex (ChatGPT desktop app) | `CODEX_SANDBOX`, plus `CODEX_APP_RUNTIME_CLI_PATH` and `CODEX_APP_TOOLS_PIPE_PATH` | Install: script |
   | Codex CLI | `CODEX_SANDBOX`, without any `CODEX_APP_*` variables | Install: script |
   | Cursor | `CURSOR_AGENT=1`, plus `CURSOR_LAYOUT` and `VSCODE_*` variables | Install: script |
   | Cursor CLI | `CURSOR_AGENT=1`, without `CURSOR_LAYOUT` or `VSCODE_*` variables | Install: script |
   | Gemini app | no shell; your instructions say "You are Gemini" | Install: MCP (coming soon) |
   | Gemini Enterprise | untested: your instructions name Gemini Enterprise | Install: MCP (coming soon) |
   | Gemini Spark | untested: your instructions name Gemini Spark | Install: MCP (coming soon) |
   | GitHub Copilot CLI | `COPILOT_CLI` | Install: script |
   | Microsoft 365 Copilot | untested: your instructions name Microsoft 365 Copilot | Install: MCP (coming soon) |
   | Perplexity | untested: your instructions name Perplexity | Install: MCP (coming soon) |
   | Perplexity Computer | untested: your instructions name Perplexity Computer | Install: MCP (coming soon) |
   | Any other agent with no shell | only your instructions | Install: not supported |

4. Decide. If your instructions and environment clearly point to one platform, that's your answer, even if the user said something else. If they don't, decide what you can:
   - Can you run shell commands at all?
   - Do they run on the user's own computer, or in a sandbox in the cloud? A Linux container where you are `root`, often behind an `HTTPS_PROXY`, is a cloud sandbox.
   - Which product are you, even if not which surface?
5. Tell the user in one line what you found and what you'll do: "This looks like <platform> because <evidence>, so I'll <follow your install section>." If the user said they were on a different platform, say that plainly. If your evidence can't confirm the platform they named, don't agree with it: say what you can confirm, and go on from that.

## Install

Follow the one section the hints table names for your platform, and skip the others. If your platform isn't in the table: use "Install: script" if you can run shell commands, and "Install: not supported" if you can't.

### Install: script

Tell the user you're installing the TollBit CLI, then run the installer. The download gives up after two minutes, so a blocked network can't hang the install, and the installer runs only once it has downloaded completely.

On macOS or Linux:

```bash
installer="$(mktemp)" && curl -fsSL --connect-timeout 20 --max-time 120 -o "$installer" "https://raw.githubusercontent.com/tollbit/cli/v0.5.0/scripts/install.sh" && bash "$installer" --version v0.5.0
```

On Windows (PowerShell):

```powershell
& ([scriptblock]::Create((irm -TimeoutSec 120 "https://raw.githubusercontent.com/tollbit/cli/v0.5.0/scripts/install.ps1"))) -Version v0.5.0
```

- **Approvals.** Many agents ask the user before running a command. If yours does, tell the user what the command does and ask them to approve it.
- **Network.** The installer downloads from GitHub. If your commands run without network access (Codex does by default), ask the user to allow network access for this one command. In a cloud sandbox with an allowlist, it needs `raw.githubusercontent.com`, `api.github.com`, `github.com` and `release-assets.githubusercontent.com`.
- **A hang is a failure.** If the installer times out, or prints nothing for two minutes, stop it and go to the fallbacks below. Don't wait, and don't retry the same command.
- **If the installer fails,** use the first of these that works:
  1. **npm, but only if it's already installed** (`npm --version` works): `npm install -g @tollbit/tollbit-cli@0.5.0`. Don't install Node.js or Git to get it: on a managed work computer, those installers can wait for an administrator prompt the user never sees.
  2. **The release, from github.com.** Download `tollbit_0.5.0_<os>_<arch>.tar.gz` (`.zip` on Windows; `<os>` is `darwin`, `linux` or `windows`, and `<arch>` is `amd64` or `arm64`) and `tollbit_0.5.0_checksums.txt` from `https://github.com/tollbit/cli/releases/download/v0.5.0/`, with the same two-minute limit. Check the archive's SHA-256 against the checksums file, unpack it, and put `tollbit` (`tollbit.exe` on Windows) in a directory on the user's PATH, such as `~/.local/bin`.
- **Check it** with `tollbit doctor`, which exits 1 if a check fails. If `tollbit` isn't found, the installer put it in `~/.local/bin`: run it from there, or start a new shell.
- **If it still fails,** don't try other sources. Get the troubleshooting prompt from https://assets.cdn.tollbit.com/cli/prompts/install-troubleshooting.md, or if that fails, from https://github.com/tollbit/cli/releases/download/v0.5.0/install-troubleshooting.md. Follow it: it finds where the install breaks and writes a report for TollBit support.

### Install: Claude plugin

In Claude chat, TollBit comes as a Claude plugin, and its `tollbit-install-claude` skill installs the CLI in your sandbox.

1. **Allow network access.** The CLI has to reach TollBit from your sandbox. Ask the user to open **Settings → Capabilities**, keep **Create and edit files** and **Use the internet while making files** on, then under **Allowed websites → Manage** add `oauth.tollbit.com` and `gateway.tollbit.com`. On a Team or Enterprise plan, an admin sets this instead, under **Organization settings → Capabilities → Code execution**.
2. **Add the plugin, if you don't have that skill.** You can't do this yourself. Ask the user to go to **Customize → Plugins → Add → Add marketplace**, enter `tollbit/cli`, then install the **tollbit** plugin. Then they start a new chat and ask it to set up TollBit, which runs the skill. They don't need to paste these instructions again. On a Team or Enterprise plan, if **Add marketplace** isn't there, an organization owner has to turn on Skills, User-created skills and Cowork first.
3. **Follow the `tollbit-install-claude` skill.**

### Install: MCP (coming soon)

On this platform, TollBit will connect through its MCP server, which isn't available yet. Tell the user it's coming soon, and that these instructions will be updated when it is. Until then, TollBit works in agents that can run commands, such as Claude Code, Codex, Cursor and GitHub Copilot CLI.

### Install: not supported

You can't run shell commands here, so you can't install the TollBit CLI. Tell the user that, and that TollBit works in agents that can run commands, such as Claude Code, Claude chat, Codex, Cursor and GitHub Copilot CLI.

## After installing

Configure yourself. Name this agent with `tollbit auth set --name "<name>"`. The name is for the user, so choose one that means something to them, such as "Alex's Claude Code" or "My Codex". Tell them the name you chose, and that you can change it if they'd like a different one. Then say where you run with `tollbit runtime set --end-user-proximity local` (the user's own computer) or `remote` (a cloud sandbox), using what you found in "Work out your platform". `tollbit runtime help` explains the setting.

Many operations don't need sign-in, so don't sign in up front. The user can sign in if they wish, and the CLI prompts when a command needs it.

Then find out what TollBit can do, from your TollBit skills if you have them, or by exploring `tollbit --help`. Tell the user what it offers, leading with the features that fit what you know about them and their work, and suggest one thing to try first. If you want to install TollBit's skills, ask the user first.
