Install the TollBit CLI. These instructions install the TollBit CLI (or tell you how to connect to TollBit another way) and show how to use it. The right steps depend on which AI product and surface you are running in, so first work out your platform, as described below. Don't install or change anything until you have.

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

3. Compare what you found with these hints. They are what each platform shows, or is expected to show. Products change them, so they are hints, not rules: weigh them with your instructions. The last column names the install section to follow.

   | Platform | What to look for | Install |
   |---|---|---|
   | Antigravity (desktop app and IDE) | `ANTIGRAVITY_*` variables, `__CFBundleIdentifier=com.google.antigravity`, no `TERM_PROGRAM` | Install: script, then Install: MCP instead |
   | Antigravity CLI | `ANTIGRAVITY_*` variables, and `TERM_PROGRAM` set | Install: script, then Install: MCP instead |
   | ChatGPT | your instructions name ChatGPT | Install: MCP |
   | ChatGPT Work | your instructions name ChatGPT Work | Install: MCP |
   | Claude chat (desktop app) or Claude chat (web) | `CLAUDE_CODE_ENTRYPOINT=remote_cowork`, in a cloud Linux container | Install: MCP |
   | Claude Code (desktop app) | `CLAUDE_CODE_ENTRYPOINT=claude-desktop` | Install: script, then Install: MCP instead |
   | Claude Code (terminal) | `CLAUDE_CODE_ENTRYPOINT=cli` | Install: script, then Install: MCP instead |
   | Claude Code on the web | `CLAUDE_CODE_ENTRYPOINT=remote`, in a cloud Linux container | Install: script, then Install: MCP instead |
   | Claude Duet | your instructions name Claude Duet | Install: MCP |
   | Claude mobile apps | your instructions name the Claude iOS or Android app | Install: MCP |
   | Codex (ChatGPT desktop app) | `CODEX_SANDBOX`, plus `CODEX_APP_RUNTIME_CLI_PATH` and `CODEX_APP_TOOLS_PIPE_PATH` | Install: Codex, then Install: MCP instead |
   | Codex CLI | `CODEX_SANDBOX`, without any `CODEX_APP_*` variables | Install: Codex, then Install: MCP instead |
   | Cursor | `CURSOR_AGENT=1`, plus `CURSOR_LAYOUT` and `VSCODE_*` variables | Install: script, then Install: MCP instead |
   | Cursor CLI | `CURSOR_AGENT=1`, without `CURSOR_LAYOUT` or `VSCODE_*` variables | Install: script, then Install: MCP instead |
   | Gemini app | no shell; your instructions say "You are Gemini" | Install: MCP |
   | Gemini Enterprise | your instructions name Gemini Enterprise | Install: MCP through an admin |
   | Gemini Spark | your instructions name Gemini Spark | Install: MCP |
   | GitHub Copilot CLI | `COPILOT_CLI` | Install: script, then Install: MCP instead |
   | Grok | your instructions name Grok, the chat app (not Grok Bot) | Install: MCP |
   | Microsoft 365 Copilot | your instructions name Microsoft 365 Copilot | Install: MCP through an admin |
   | Microsoft Copilot Studio | your instructions name Microsoft Copilot Studio | Install: MCP through an admin |
   | Mistral Vibe | your instructions name Mistral Vibe | Install: MCP |
   | Perplexity | your instructions name Perplexity | Install: MCP |
   | Perplexity Computer | your instructions name Perplexity Computer | Install: MCP |
   | Slackbot | your instructions name Slackbot | Install: MCP through an admin |
   | Any other agent with no shell | only your instructions | Install: MCP if the user can add a custom MCP server there, otherwise Install: not supported |

4. Decide. If your instructions and environment clearly point to one platform, that's your answer, even if the user said something else. If they don't, decide what you can:
   - Can you run shell commands at all?
   - Do they run on the user's own computer, or in a sandbox in the cloud? A Linux container where you are `root`, often behind an `HTTPS_PROXY`, is a cloud sandbox.
   - Which product are you, even if not which surface?
5. Tell the user in one line what you found and what you'll do: "This looks like <platform> because <evidence>, so I'll <follow your install section>." If the user said they were on a different platform, say that plainly. If your evidence can't confirm the platform they named, don't agree with it: say what you can confirm, and go on from that.

## Install

Follow the one section the hints table names for your platform, and skip the others. If your platform isn't in the table and you can run shell commands, use "Install: script". If you can't, use "Install: MCP" if the user can add a custom MCP server or connector in this product, and "Install: not supported" if they can't. If you can run shell commands but can't get a working CLI, follow "Install: MCP instead", which connects TollBit to you as an MCP server.

### Install: script

Tell the user you're installing the TollBit CLI, then run the installer. The download gives up after two minutes, so a blocked network can't hang the install, and the installer runs only once it has downloaded completely. The command removes the downloaded script afterwards and exits with the installer's status.

On macOS or Linux:

```bash
installer="$(mktemp)" && curl -fsSL --connect-timeout 20 --max-time 120 -o "$installer" "https://tollbit.com/.agents/cli/install.sh" && bash "$installer" --version v0.5.4; rc=$?; rm -f "$installer"; (exit $rc)
```

On Windows (PowerShell):

```powershell
& ([scriptblock]::Create((irm -TimeoutSec 120 "https://tollbit.com/.agents/cli/install.ps1"))) -Version v0.5.4
```

- **Approvals.** Many agents ask the user before running a command. If yours does, tell the user what the command does and ask them to approve it.
- **Network.** The installer downloads from `tollbit.com`, or from GitHub if that fails, and `tollbit` commands need network access too. Some agents run commands without network access. If yours does, ask the user to allow it. The fallbacks below need network access too, so don't try them. If you can't get access, give the user the command to run in their own terminal. In a cloud sandbox with an allowlist, allow `tollbit.com` and `*.tollbit.com`: the installer needs `tollbit.com`, and `tollbit` commands need its subdomains.
- **A hang is a failure.** If the installer times out, or prints nothing for two minutes, stop it and go to the fallbacks below. Don't wait, and don't retry the same command.
- **If the installer fails,** use the first of these that works:
  1. **npm, but only if it's already installed** (`npm --version` works): `npm install -g @tollbit/tollbit-cli@0.5.4`. Don't install Node.js or Git to get it: on a managed work computer, those installers can wait for an administrator prompt the user never sees.
  2. **The release, downloaded directly.** Download `tollbit_0.5.4_<os>_<arch>.tar.gz` (`.zip` on Windows; `<os>` is `darwin`, `linux` or `windows`, and `<arch>` is `amd64` or `arm64`) and `tollbit_0.5.4_checksums.txt` from `https://tollbit.com/.agents/cli/v0.5.4/`, or from `https://github.com/tollbit/cli/releases/download/v0.5.4/` if that fails, with the same two-minute limit. Take both files from the same place. Check the archive's SHA-256 against the checksums file, unpack it, and put `tollbit` (`tollbit.exe` on Windows) in a directory on the user's PATH, such as `~/.local/bin`.
- **Once the installer or a fallback has installed it, check it** with `tollbit doctor`, which exits 1 if a check fails. If `tollbit` isn't found, the installer put it in `~/.local/bin`: run it from there, or start a new shell.
- **If nothing installed,** don't try other sources. Follow "Install: MCP instead".
- **If `tollbit` installed but `tollbit doctor` fails,** follow "Troubleshoot a failed install" at the end of these instructions: it finds where the install breaks and writes a report for TollBit support. Once the user has the report, follow "Install: MCP instead".

### Install: Codex

Codex runs your commands in a sandbox with no network access, where only your workspace is writable. The install and every `tollbit` command need network access and write to `~/.local/bin` or `~/.config/tollbit`, so allowing only the install isn't enough.

Don't run the installer to see what happens, and don't try npm, a release download or "Troubleshoot a failed install". They all need the same access.

1. **Choose how to install.** If you have a `request_permissions` tool, and `ls -ld ~/.local ~/.local/bin ~/.config ~/.config/tollbit` shows no symlinks (a missing path is fine), go to step 2. Otherwise go to step 3.
2. **Install from here.** Make one `request_permissions` call for network access and write access to the full paths of `~/.local/bin` and `~/.config/tollbit`. Ask for nothing else, and never for a shell profile such as `~/.zshrc`: Codex can't grant a symlink, and asking for one can break your shell tool until the user starts a new chat. Ask the user to approve it for the whole session if Codex offers that. Once it's granted, run this in your shell tool, not a Node REPL or any other tool, because their sandboxes differ:

   ```bash
   installer="$(mktemp)" && curl -fsSL --connect-timeout 20 --max-time 120 -o "$installer" "https://tollbit.com/.agents/cli/install.sh" && bash "$installer" --version v0.5.4 --no-modify-path; rc=$?; rm -f "$installer"; (exit $rc)
   ```

   Then run `tollbit doctor` and follow "After installing" straight away, because the access may end with this turn. If `tollbit` isn't found, the installer put it in `~/.local/bin`: run it from there, and give the user the PATH line the installer printed. If anything here fails, go to step 3.
3. **Hand it to the user.** Tell them the Codex sandbox stops you doing this, and show them both of these commands in full, to run in their own terminal, not in Codex. The first installs TollBit. The second adds a Codex rule that lets `tollbit` commands, and nothing else, run outside the sandbox without asking. Tell them that includes paid commands such as `tollbit content`.

   ```bash
   installer="$(mktemp)" && curl -fsSL --connect-timeout 20 --max-time 120 -o "$installer" "https://tollbit.com/.agents/cli/install.sh" && bash "$installer" --version v0.5.4; rc=$?; rm -f "$installer"; (exit $rc)
   ```

   ```bash
   mkdir -p ~/.codex/rules && r='prefix_rule(pattern = ["tollbit"], decision = "allow")' && { grep -qsxF "$r" ~/.codex/rules/default.rules || printf '\n%s\n' "$r" >> ~/.codex/rules/default.rules; }
   ```

   On Windows, the first command is the PowerShell one in "Install: script", and the second is adding `prefix_rule(pattern = ["tollbit"], decision = "allow")` as a line in `%USERPROFILE%\.codex\rules\default.rules`.

   Then they restart Codex and ask you to carry on. In the CLI, `codex resume --last` brings this conversation back. When they do, run `tollbit doctor` and follow "After installing".
4. **Keep TollBit working.** If you installed in step 2, finish by telling the user that the access they approved ends with this turn or this session, so TollBit will need their approval again after that. Do this even if they approved it for the session. To avoid that, offer them the second command in step 3, say what it does, including that it covers paid commands such as `tollbit content`, and tell them to restart Codex after running it. If they'd rather not add the rule, they can switch Codex to Full access, or run `tollbit` in their own terminal.

Once the rule is in place, run each `tollbit` command on its own. Don't chain it with other commands, because the rule covers only `tollbit`.

If the user can't get `tollbit` installed in their own terminal either, or `tollbit doctor` fails, follow "Install: MCP instead".

### Install: MCP

On this platform, TollBit connects as an MCP server, which these products call a custom connector, app or plugin. You can't add it yourself, so walk the user through it. For any product:

- **Name:** TollBit
- **URL:** `https://mcp.tollbit.com/mcp`
- **Authentication:** OAuth, if the product asks. Leave any client ID and secret empty: TollBit registers the product itself.

Where to add it, with the product's own guide. Show the user the steps and the guide link for their product, so they can check the guide if a menu has moved:

- **Claude chat, the Claude mobile apps, Claude Cowork and Claude Duet:** on claude.ai or in the Claude desktop app, go to **Customize → Connectors → + Add → Add custom connector**, enter the name and URL, and click **Add**. Turn it on in a chat from the **+** menu. On a Team or Enterprise plan, an owner adds it under **Organization settings → Connectors**, and each member then clicks **Connect**. The Claude mobile apps and Claude Duet use the connectors on the user's Claude account, so add it on claude.ai or in the desktop app, and it then works there too. Guide: https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp
- **ChatGPT and ChatGPT Work:** on the web, go to **Plugins** (`https://chatgpt.com/plugins`) → **+** → **Add custom MCP server**. Enter the name and URL, choose OAuth, accept the warning, click **Create as a plugin**, then install it. In a chat or a Work task, type `@` and pick TollBit; it then works on desktop and mobile too. Not every ChatGPT plan can add custom MCP servers. In a ChatGPT workspace, an admin may have to allow plugins with custom MCP servers first. Guide: https://learn.chatgpt.com/docs/plugins (for admins: https://learn.chatgpt.com/docs/enterprise/apps-and-connectors)
- **Gemini app and Gemini Spark:** on a computer, open `https://gemini.google.com` and go to **Settings → Connected Apps → Custom apps → Add a custom app**. Enter the URL and click **Next**. Custom apps can only be added on the web, but then work in the mobile app too. They need a personal Google account in the US, and a plan that includes custom apps. Guide: https://support.google.com/gemini/answer/17209137
- **Perplexity and Perplexity Computer:** go to **Customize → Connectors → Add custom → Add MCP connector → Remote**. Enter the name and URL, choose OAuth under the advanced options, tick the acknowledgement, and click **Add**. Then click the connector's card to connect. In Perplexity Computer, use it in a task from **/ → Plugins**. Custom connectors need a Pro, Max or Enterprise plan: on Free, the user can add one, but connecting fails with "Failed to enable connector". On an Enterprise plan, an admin may have to allow members to add custom connectors. Perplexity saves a later sign-in as a second account and keeps using the first by default, so for buying content or analytics the user should choose **Sign in** the first time they connect. If they already connected as a guest, they sign in again from the connector, then remove the guest account. Guide: https://www.perplexity.ai/help-center/en/articles/13915507-adding-custom-remote-connectors
- **Grok:** go to **Connectors** (`https://grok.com/connectors`) → **New Connector** → **Custom**. Enter the URL and complete the sign-in. On Grok Business or Enterprise, a team admin adds it first in the xAI console under **Grok Business → Connectors → + Add Connector → Other**, and members then connect it on `https://grok.com/connectors`. Guide: https://docs.x.ai/grok/connectors (for admins: https://docs.x.ai/grok/connector-management)
- **Mistral Vibe:** open the **Connectors** page → **+ Add Connector** → **Custom MCP Connector**. Enter the name and URL, click **Connect**, and complete the sign-in. Only an admin can add connectors; on the Free, Pro and Student plans, the account owner is the admin. Guide: https://docs.mistral.ai/vibe/work/connectors/mcp-connectors

When the user connects, TollBit's sign-in page offers **Sign in** or **Continue as guest**. A guest can search and check prices. Buying content and analytics need a TollBit account: when a tool needs one, the product asks the user to sign in again, and they choose **Sign in**.

If the option to add it isn't there, the user's plan, country or an admin setting is blocking it. Tell them which, from the notes above, and that their plan or their admin can turn it on. Don't suggest other ways to install.

Once TollBit is connected, skip "After installing", which is for the CLI. Instead, look at TollBit's tools, tell the user what they offer, leading with what fits what you know about them and their work, and suggest one thing to try first.

### Install: MCP instead

Follow this section when you can run commands but couldn't get a working TollBit CLI, and "Install: script", "Install: Codex" or "Troubleshoot a failed install" sends you here. Tell the user the CLI isn't working here, that TollBit can still connect to you as an MCP server, and set that up. For any agent:

- **Name:** `TollBit`
- **URL:** `https://mcp.tollbit.com/mcp`, Streamable HTTP.
- **Authentication:** OAuth. Leave any client ID and secret empty: TollBit registers the agent itself.

How to add it, with the agent's own guide. Show the user the steps and the guide link for their agent, so they can check the guide if a command has changed. When you edit a config file that already exists, add the `TollBit` entry and keep the others. If a command fails because you have no network access or can't write to the user's home directory, give it to the user to run in their own terminal.

- **Claude Code in a terminal:** run `claude mcp add --transport http --scope user TollBit https://mcp.tollbit.com/mcp`. `--scope user` makes it available in all the user's projects. Then ask the user to run `/mcp`, select `TollBit` and sign in in the browser. If `TollBit` isn't listed, they restart Claude Code; `claude --continue` brings this conversation back. Guide: https://code.claude.com/docs/en/mcp
- **Claude Code in the desktop app and on the web:** these sessions use the connectors on the user's Claude account. The user adds TollBit as a custom connector on claude.ai, as for Claude chat in "Install: MCP", then starts a new session. Guide: https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp
- **Codex CLI and Codex in the ChatGPT desktop app:** your sandbox can't write to `~/.codex` or reach the network, so show the user these commands to run in their own terminal. The first adds the server to `~/.codex/config.toml`, which the CLI, the desktop app and the IDE extension share. The second opens a browser to sign in.

  ```bash
  codex mcp add TollBit --url https://mcp.tollbit.com/mcp
  codex mcp login TollBit
  ```

  Then they restart Codex. In the CLI, `codex resume --last` brings this conversation back. Guide: https://learn.chatgpt.com/docs/extend/mcp
- **Cursor and Cursor CLI:** both read `~/.cursor/mcp.json`. Add this to it:

  ```json
  { "mcpServers": { "TollBit": { "url": "https://mcp.tollbit.com/mcp" } } }
  ```

  In Cursor, the user turns TollBit on and signs in from **Customize** in the sidebar, and restarts Cursor if it isn't listed. In Cursor CLI, they run `agent mcp login TollBit`. Guides: https://cursor.com/docs/mcp and, for the CLI, https://cursor.com/docs/cli/mcp
- **Antigravity (desktop app and IDE) and Antigravity CLI:** the IDE and the CLI read `~/.gemini/config/mcp_config.json`. Add this to it:

  ```json
  { "mcpServers": { "TollBit": { "serverUrl": "https://mcp.tollbit.com/mcp" } } }
  ```

  The desktop app shows its own file under **Customizations → Installed → MCP Servers → Copy Config File Path**; if that's a different file, ask the user for the path and add the entry there too. Antigravity signs in with OAuth itself. In the desktop app, the user clicks **Authenticate** next to TollBit under **Customizations → Installed**, signs in in the browser, and pastes the code back. In the CLI, `/mcp` reloads the servers. Guide: https://antigravity.google/docs/mcp
- **GitHub Copilot CLI:** run `copilot mcp add --transport http TollBit https://mcp.tollbit.com/mcp`, which adds it to `~/.copilot/mcp-config.json`. If `/mcp` doesn't list `TollBit`, the user restarts Copilot CLI. If it shows as needing sign-in, they run `/mcp auth TollBit`, which opens a browser. Guide: https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers
- **Any other agent:** add a remote MCP server with the name and URL above, the way your product's own documentation describes, and sign in with OAuth when it asks. Show the user the documentation page you used. If your product can't add a remote MCP server, or you can't find how, tell the user and stop.

When the user signs in, TollBit's sign-in page offers **Sign in** or **Continue as guest**. A guest can search and check prices. Buying content and analytics need a TollBit account: when a tool needs one, the agent asks the user to sign in again, and they choose **Sign in**.

If TollBit's tools still don't appear, stop and tell the user. Don't try other ways to install.

Once TollBit is connected, skip "After installing", which is for the CLI. Instead, look at TollBit's tools, tell the user what they offer, leading with what fits what you know about them and their work, and suggest one thing to try first.

### Install: MCP through an admin

On this platform, only an admin or the agent's builder can add an MCP server. Tell the user that, and show them these details and the steps for their product, with its guide link, to pass on:

- **URL:** `https://mcp.tollbit.com/mcp`, Streamable HTTP.
- **Authentication:** OAuth with dynamic client registration. Leave any client ID and secret empty.
- **Microsoft Copilot Studio:** the agent's maker opens the agent's **Tools** page → **Add a tool** → **New tool** → **Model Context Protocol**, enters the name and URL, chooses **OAuth 2.0** → **Dynamic discovery**, and clicks **Create**. Then they create a connection and click **Add to agent**. Guide: https://learn.microsoft.com/en-us/microsoft-copilot-studio/mcp-add-existing-server-to-agent
- **Slackbot:** a developer adds the server to a Slack app under **Features → MCP Servers**, with dynamic client registration, and an admin installs the app. Each user then turns it on from **Apps** in the Slackbot toolbar and signs in. Guides: https://docs.slack.dev/ai/slackbot-mcp-client/ and, for users, https://slack.com/help/articles/52414744085139-Connect-Slackbot-to-other-apps-with-MCP

Gemini Enterprise and Microsoft 365 Copilot don't register themselves, so the admin first registers their product with TollBit with one command, then adds TollBit with the values it returns. The admin runs the command in a terminal, with their company's name and their product's redirect URL below, and keeps the `client_id` and `client_secret` it returns. The secret is like a password: they don't email or share it.

```bash
curl -s https://oauth.tollbit.com/oauth2/register -H 'Content-Type: application/json' -d '{"client_name": "<product> - <company>", "redirect_uris": ["<redirect URL>"], "token_endpoint_auth_method": "client_secret_basic"}'
```

Then they add TollBit in their product with these values:

- **Authorization URL:** `https://oauth.tollbit.com/oauth2/auth`
- **Token URL:** `https://oauth.tollbit.com/oauth2/token`
- **Client ID** and **Client Secret:** from the command.
- **Scopes:** `openid offline_access tollbit:discover tollbit:analytics tollbit:spend`

If signing in fails with `invalid_client`, they run the command again with `client_secret_post` in place of `client_secret_basic`, and use the new values.

- **Gemini Enterprise:** the redirect URL is `https://vertexaisearch.cloud.google.com/oauth-redirect`. At `https://business.gemini.google`, the admin goes to **Settings & help → their team → Manage team → Connected apps → Add MCP Server**, enters the URL above and the name TollBit, and chooses **OAuth 2.0** with the values above. They click **Verify Auth**, then **Continue**, choose a location and click **Add**, and then turn the connection on: it's off by default. Guide: https://support.google.com/g/answer/17106276
- **Microsoft 365 Copilot:** the redirect URL is `https://teams.microsoft.com/api/platform/v1.0/oAuthRedirect`. In the Teams Developer Portal, the admin goes to **Tools → OAuth client registration → New**, enters the URL above as the base URL and the values above, with the token URL as the refresh URL too, leaves PKCE on, and allows **Any Teams app**. Then a Global Administrator or AI Administrator goes to the Microsoft 365 admin center, **Copilot → Connectors → Connect to MCP server**, enters the URL above and the registration ID, and turns it on for their users. Each user then connects TollBit from **Sources** in Copilot Chat's settings. Users need a Microsoft 365 Copilot license. Guide: https://learn.microsoft.com/en-us/microsoft-365/copilot/connectors/set-up-custom-federated-connectors

### Install: not supported

You can't run shell commands here, so you can't install the TollBit CLI, and the user can't add TollBit as an MCP server in this product. Tell the user that, and that TollBit works in agents that can run commands, such as Claude Code, Codex, Cursor and GitHub Copilot CLI, and connects to Claude, ChatGPT, Gemini, Perplexity, Grok and Mistral Vibe as an MCP server.

## After installing

Configure yourself. Name this agent with `tollbit auth set --name "<name>"`. The name is for the user, so choose one that means something to them, such as "Alex's Claude Code" or "My Codex". Tell them the name you chose, and that you can change it if they'd like a different one. Then say where you run with `tollbit runtime set --end-user-proximity local` (the user's own computer) or `remote` (a cloud sandbox), using what you found in "Work out your platform". `tollbit runtime help` explains the setting.

Many operations don't need sign-in, so don't sign in up front. The user can sign in if they wish, and the CLI prompts when a command needs it.

Then find out what TollBit can do, from your TollBit skills if you have them, or by exploring `tollbit --help`. Tell the user what it offers, leading with the features that fit what you know about them and their work, and suggest one thing to try first. If you want to install TollBit's skills, ask the user first.

If you install them with `tollbit guide --install <dir>`, then run `tollbit doctor --skills-dir <dir>` and read the `Skills` lines. A missing or out-of-date skill only warns, so `doctor` still exits 0.

## Troubleshoot a failed install

Follow this section only when "Install: script" sends you here. Otherwise skip it.

Work out where installing the tollbit CLI breaks on this machine, then write a short report the user sends to TollBit support. On a managed work computer some fixes need the user's own IT administrators; TollBit support works with them when that's the case. This covers installing only. If `tollbit version` already prints a version, the install worked: say so, and stop. If `tollbit` isn't found but step 5 finds a binary that runs, an earlier install worked and only PATH is missing: run every step, and report it as a PATH problem.

Rules:

- **Diagnose, don't work around.** Don't install from any source other than those listed below, turn off TLS verification (except for the read-only issuer lookup in step 3), change system or security settings, change npm or shell configuration, or ask for an administrator password. Suggest such changes in the report instead.
- **Change nothing on the machine.** The checks only read, except for test files they remove at once (in the install folder on Windows, and in a temporary folder) and step 5's download into a temporary folder it deletes.
- **Run every step**, even after one fails. The steps check different things, and the report needs all of them.
- Keep the exact error text of every failing command. Before you show anything, replace passwords and tokens (for example the `user:pass` in a proxy URL) with `***`.
- Use the commands for the shell you have: bash or zsh on macOS and Linux, PowerShell on Windows. In Windows PowerShell 5.1, `curl` means `Invoke-WebRequest`; write `curl.exe` for real curl.

### 1. Environment

Find out, and note for the report:

- **Where you run.** One of these:
  - Claude in the desktop or web app, chat or Cowork: a temporary Linux sandbox whose network goes through a proxy (`HTTPS_PROXY` is set), limited by the organization's allowlist.
  - Claude Code, on the user's computer.
  - Codex, whose sandbox often has no network at all.
  - A terminal the user runs commands in themselves.
- **What failed.** Which install method was tried (script, npm, the tollbit-install-claude skill) and its exact error. Find it earlier in the conversation, or ask the user to paste it. If they can't, and the user agrees, rerun that install once to capture it: for npm, `npm install -g @tollbit/tollbit-cli --foreground-scripts` shows the install step's own error. If what failed is adding the TollBit plugin itself in the Claude app (for example, no option to add a marketplace or upload a plugin), that comes from the organization's Claude settings, not this machine or the plugin's files: user-created skills turned off, Skills or Cowork turned off, or a limit on which repositories can be added. Note it in the report for an organization admin, and check the CLI install anyway.
- **OS, CPU architecture and shell.**

  ```bash
  uname -srm; echo "shell: $SHELL"
  for c in curl tar shasum sha256sum node npm; do printf '%s: ' "$c"; command -v "$c" || echo missing; done
  node --version; npm --version
  ```

  ```powershell
  [Environment]::OSVersion.VersionString
  [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture
  "PowerShell $($PSVersionTable.PSVersion) $($PSVersionTable.PSEdition)"
  "Language mode: $($ExecutionContext.SessionState.LanguageMode)"
  Get-ExecutionPolicy -List
  foreach ($c in 'node', 'npm', 'curl.exe') { "${c}: $((Get-Command $c -ErrorAction SilentlyContinue).Source)" }
  node --version; npm --version
  ```

What the results mean:

- Only `amd64`/`x86_64` and `arm64`/`aarch64` are supported, on macOS, Linux and Windows. The npm package needs Node 18 or later.
- **Language mode `ConstrainedLanguage`** (set by AppLocker or WDAC) breaks `irm ... | iex`, the PowerShell installer.
- **Execution policy** applies only to `.ps1` files saved to disk. `Restricted` or `AllSigned` blocks running a saved `install.ps1`, but not `irm ... | iex`. It also blocks `npm.ps1`, so `npm` can fail in PowerShell while `npm.cmd` works.

### 2. Proxy settings

```bash
env | grep -i -E '^(https?|all|no)_proxy=' | sed -E 's#//[^@/]*@#//***@#'
echo "NODE_EXTRA_CA_CERTS=$NODE_EXTRA_CA_CERTS NODE_USE_ENV_PROXY=$NODE_USE_ENV_PROXY"
for k in proxy https-proxy noproxy cafile strict-ssl ignore-scripts registry prefix; do echo "npm $k = $(npm config get $k 2>&1)"; done | sed -E 's#//[^@/]*@#//***@#'
[ "$(uname)" = Darwin ] && { scutil --proxy | grep -E 'HTTPS(Enable|Proxy|Port)|ProxyAutoConfig(Enable|URLString)|ProxyAutoDiscoveryEnable' || echo "macOS system proxy: none"; }
```

```powershell
Get-ChildItem Env: | Where-Object Name -Match '^(https?|all|no)_proxy$|^NODE_EXTRA_CA_CERTS$|^NODE_USE_ENV_PROXY$' | ForEach-Object { "$($_.Name)=$($_.Value -replace '//[^@/]*@', '//***@')" }
foreach ($k in 'proxy', 'https-proxy', 'noproxy', 'cafile', 'strict-ssl', 'ignore-scripts', 'registry', 'prefix') { "npm $k = $(npm.cmd config get $k 2>&1)" -replace '//[^@/]*@', '//***@' }
netsh winhttp show proxy
Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' | Select-Object ProxyEnable, ProxyServer, AutoConfigURL
```

What the results mean:

- Each install method finds the proxy in a different place:
  - `install.sh` (curl) reads the `HTTPS_PROXY` environment variable.
  - `install.ps1` (`Invoke-WebRequest`) uses the Windows proxy in Internet Settings, and also `HTTPS_PROXY` in PowerShell 7.
  - npm fetches the package with its own `proxy`/`https-proxy` settings. The package's install step (`npm/install.js`) then downloads the binary from `tollbit.com`, or from GitHub if that fails, using only the `HTTPS_PROXY`/`NO_PROXY` environment variables.
- **A proxy set in npm but not in `HTTPS_PROXY`:** npm gets the package, then the binary download goes direct and fails with `no data for 60s` or a connection error.
- **A system proxy is set but `HTTPS_PROXY` isn't** (macOS `HTTPSEnable : 1` from `scutil --proxy`, or Windows `ProxyEnable` 1): browsers use it, but curl, Node and the CLI don't. Setting `HTTPS_PROXY` to the same address fixes the install.
- **Only a PAC file is set** (Windows `AutoConfigURL`, macOS `ProxyAutoConfigEnable : 1` or `ProxyAutoDiscoveryEnable : 1`): curl and Node can't use a PAC file. The user needs the actual proxy address from IT.
- **`ignore-scripts = true`:** npm never runs the package's install step, so the binary is never downloaded.
- Behind a proxy that inspects TLS (step 3), Node needs `NODE_EXTRA_CA_CERTS` pointing at the corporate root certificate. npm's `cafile` doesn't apply to the install step.

### 3. Hosts

These are the hosts installing needs:

| Host | Needed by | Probe URL |
|---|---|---|
| `tollbit.com` | every method: `install.sh`, `install.ps1` and release downloads. When it fails, they fall back to the GitHub hosts below. | https://tollbit.com/.agents/cli/latest |
| `raw.githubusercontent.com` | older install commands that fetch `install.sh` or `install.ps1` from GitHub | https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh |
| `api.github.com` | the scripts, when `tollbit.com` fails, to find the latest version (not needed when a version is given) | https://api.github.com/repos/tollbit/cli/releases/latest |
| `github.com` | release downloads when `tollbit.com` fails, and Claude Code's `/plugin marketplace add` | https://github.com/tollbit/cli/releases |
| `release-assets.githubusercontent.com` | release downloads when `tollbit.com` fails: GitHub redirects them here | https://github.com/tollbit/cli/releases/download/v0.5.4/tollbit_0.5.4_checksums.txt (follows the redirect) |
| `registry.npmjs.org` | npm | https://registry.npmjs.org/@tollbit%2ftollbit-cli/latest |

`release-assets.githubusercontent.com` has no probe of its own: rate it by the release download line, after the redirect. If `github.com` passes and the release download fails, `release-assets.githubusercontent.com` is the blocked host.

```bash
for url in \
  https://tollbit.com/.agents/cli/latest \
  https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh \
  https://api.github.com/repos/tollbit/cli/releases/latest \
  https://github.com/tollbit/cli/releases \
  https://github.com/tollbit/cli/releases/download/v0.5.4/tollbit_0.5.4_checksums.txt \
  https://registry.npmjs.org/@tollbit%2ftollbit-cli/latest
do
  echo "== $url"
  curl -sSL -o /dev/null --max-time 20 -v -w 'HTTP %{http_code} in %{time_total}s\n' "$url" 2>&1 \
    | grep -E '^(HTTP |curl: |\* +(issuer|subject):|< HTTP/|< (x-deny-reason|x-squid-error|x-ratelimit-remaining):)|CONNECT tunnel'
done
```

```powershell
$urls = @(
  'https://tollbit.com/.agents/cli/latest',
  'https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh',
  'https://api.github.com/repos/tollbit/cli/releases/latest',
  'https://github.com/tollbit/cli/releases',
  'https://github.com/tollbit/cli/releases/download/v0.5.4/tollbit_0.5.4_checksums.txt',
  'https://registry.npmjs.org/@tollbit%2ftollbit-cli/latest'
)
foreach ($u in $urls) {
  try {
    $r = Invoke-WebRequest -Uri $u -UseBasicParsing -TimeoutSec 20 -ErrorAction Stop
    "OK   $($r.StatusCode) $u"
  } catch {
    $code = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { '-' }
    "FAIL $code $u`n     $($_.Exception.GetType().Name): $($_.Exception.Message) $($_.Exception.InnerException.Message)"
  }
}
```

On Windows, also run `curl.exe -sSIv --max-time 20 <url>` for each failing URL to see the certificate issuer. curl.exe ignores the Windows proxy, so add `--proxy <ProxyServer from step 2>` when one is set.

When a host fails with a certificate error, read the certificate's issuer with `curl -sSkLv -o /dev/null --max-time 20 <url> 2>&1 | grep -E '^\* +(subject|issuer):'`. `-k` skips verification so the issuer can be printed; the response is discarded, and nothing downloaded this way is used. Each `subject:` line names the host (`CN=...`), and the `issuer:` line after it is who signed that host's certificate.

Name each host's result with one of these (`tollbit doctor` uses the same names):

| Result | What you see |
|---|---|
| **ok** | HTTP 200 (`raw.githubusercontent.com` may be 200 or 304). |
| **dns** | `curl: (6) Could not resolve host`; PowerShell: `No such host is known` or `The remote name could not be resolved`. The network has no DNS for the host, or blocks it there. |
| **timeout** | `curl: (28)`; PowerShell: `The operation has timed out`. Usually a firewall dropping traffic, or a proxy that's needed but not configured. |
| **connect** | `curl: (7) Failed to connect`, or a connection reset (`curl: (35)` or `(56)`) right after connecting. |
| **tls_intercepted** | `curl: (60) SSL certificate problem` (`unable to get local issuer certificate`, `self-signed certificate in certificate chain`); PowerShell: `Could not establish trust relationship`; Node: `UNABLE_TO_GET_ISSUER_CERT_LOCALLY` or `SELF_SIGNED_CERT_IN_CHAIN`. Or the request passes but the `issuer:` line names a corporate CA or a security product (Zscaler, Netskope, Palo Alto, Forcepoint, Blue Coat, or the company's name) instead of a public CA such as DigiCert or Sectigo. Windows curl.exe behind such a proxy often fails with `CRYPT_E_NO_REVOCATION_CHECK`. Write down the issuer. |
| **tls_other** | Any other TLS failure, such as an expired certificate or a failed handshake. |
| **proxy_rejected** | `CONNECT tunnel failed, response 403` (blocked) or `407` (the proxy wants a login), or a 403 or "blocked" page that names the proxy or the company. In Claude's app sandbox this means the organization's allowlist blocks the host. |
| **`http_<code>`** | Any other HTTP error from the host itself, for example `http_404`. On `api.github.com`, a 403 or 429 with `x-ratelimit-remaining: 0` is GitHub's rate limit (many users share one office IP address), not a block: write `http_403 (rate limit)`. Giving the scripts a version (`--version` or `-Version`) avoids the call. |

If every host fails the same way and you are in an agent sandbox (Codex, or Claude Code with sandboxing turned on), your sandbox may be what blocks the network. Ask the user to allow network access, then run this step's probes again before blaming their network. If they pass then, your sandbox was the cause: carry on with the remaining steps, and name it as the likely cause in the report.

### 4. Write access and PATH, without admin

```bash
d="$HOME/.local/bin"
e="$d"; while [ ! -e "$e" ]; do e="$(dirname "$e")"; done  # the installer creates what's missing
if [ -w "$e" ]; then echo "install dir writable: $d (checked $e)"; else echo "install dir NOT writable: $d (checked $e)"; fi
case ":$PATH:" in *":$d:"*) echo "$d is on PATH";; *) echo "$d is NOT on PATH";; esac
for f in ~/.zshrc ~/.bashrc ~/.profile; do [ -e "$f" ] && { [ -w "$f" ] && echo "$f writable" || echo "$f NOT writable"; }; done
t="$(mktemp -d)" && echo "temp dir writable: $t" && rmdir "$t"
echo "elevated (root): $([ "$(id -u)" = 0 ] && echo yes || echo no)"
p="$(npm prefix -g 2>/dev/null)"
if [ -z "$p" ]; then echo "npm not available"
elif [ -w "$p/lib/node_modules" ] || { [ ! -e "$p/lib/node_modules" ] && [ -w "$p" ]; }; then echo "npm global prefix writable: $p"
else echo "npm global prefix NOT writable: $p (npm -g needs sudo)"; fi
```

```powershell
$d = "$env:LOCALAPPDATA\Programs\tollbit\bin"
$e = $d; while (-not (Test-Path $e)) { $e = Split-Path $e }  # the installer creates what's missing
try { Set-Content "$e\.tollbit-write-test" x -ErrorAction Stop; Remove-Item "$e\.tollbit-write-test"; "install dir writable: $d (checked $e)" } catch { "install dir NOT writable: $d (checked $e: $($_.Exception.Message))" }
"on user PATH: $(([Environment]::GetEnvironmentVariable('Path', 'User') -split ';') -contains $d)"
# Opens the key with write access without writing to it.
try { $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true); $k.Close(); 'user PATH writable' } catch { "user PATH NOT writable: $($_.Exception.Message)" }
"temp dir: $env:TEMP"
$p = npm.cmd prefix -g 2>$null
if (-not $p) { 'npm not available' }
else { try { Set-Content "$p\.tollbit-write-test" x -ErrorAction Stop; Remove-Item "$p\.tollbit-write-test"; "npm global prefix writable: $p" } catch { "npm global prefix NOT writable: $p (npm -g needs admin)" } }
"elevated: $(([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole('Administrators'))"
```

What the results mean:

- The scripts install to `~/.local/bin` or `%LOCALAPPDATA%\Programs\tollbit\bin` and add it to the user's PATH. Neither needs admin.
- **npm prefix not writable** (for example under `C:\Program Files\nodejs` or `/usr/local`): `npm install -g` needs admin. On Windows it may wait on an admin prompt the agent can't see, which looks like a hang. The fix is the script installer, or a user-owned npm prefix. Suggest these; don't apply them.
- **Install directory not on PATH:** the install may have worked. Open a new shell (or run `hash -r`) and check again, or run `tollbit` by its full path.

### 5. Does the binary run

Look for a `tollbit` binary from earlier attempts:

```bash
command -v tollbit
for f in ~/.local/bin/tollbit "$(npm prefix -g 2>/dev/null)/lib/node_modules/@tollbit/tollbit-cli/tollbit"; do [ ! -e "$f" ] || ls -l "$f"; done
```

```powershell
Get-Command tollbit -All -ErrorAction SilentlyContinue | Select-Object Source
$p = npm.cmd prefix -g
Get-Item "$env:LOCALAPPDATA\Programs\tollbit\bin\tollbit.exe", "$p\node_modules\@tollbit\tollbit-cli\tollbit.exe" -ErrorAction SilentlyContinue
```

Run each binary you find by its full path, with `version`.

If none exists and `tollbit.com` or the GitHub release download was **ok** in step 3, test a fresh copy in a temporary directory. If only GitHub was ok, set `base` to `https://github.com/tollbit/cli/releases/download/v0.5.4` instead. This checks the binary on its own, apart from any installer. The `<os>_<arch>` part is `darwin_arm64`, `linux_amd64`, `windows_amd64` and so on.

```bash
t="$(mktemp -d)"; cd "$t"
base=https://tollbit.com/.agents/cli/v0.5.4
curl -fsSLO "$base/tollbit_0.5.4_checksums.txt"
a="$(grep -o 'tollbit_[^ ]*_<os>_<arch>.tar.gz' tollbit_*_checksums.txt)"
curl -fsSLO "$base/$a" && grep " $a\$" tollbit_*_checksums.txt | (shasum -a 256 -c 2>/dev/null || sha256sum -c) \
  && tar -xzf "$a" && ./tollbit version
cd - >/dev/null; rm -rf "$t"
```

```powershell
$t = Join-Path $env:TEMP "tollbit-test-$([guid]::NewGuid().ToString('N'))"; New-Item -ItemType Directory $t | Out-Null
$base = 'https://tollbit.com/.agents/cli/v0.5.4'
Invoke-WebRequest "$base/tollbit_0.5.4_checksums.txt" -OutFile "$t\sums.txt" -UseBasicParsing
$line = Select-String -Path "$t\sums.txt" -Pattern '_windows_<arch>\.zip$' | Select-Object -First 1
$sum, $a = $line.Line -split '\s+'
Invoke-WebRequest "$base/$a" -OutFile "$t\$a" -UseBasicParsing
if ((Get-FileHash "$t\$a" -Algorithm SHA256).Hash -eq $sum) { Expand-Archive "$t\$a" "$t\x"; & "$t\x\tollbit.exe" version } else { "checksum mismatch for ${a}: not running it" }
Remove-Item -Recurse -Force $t
```

What the results mean:

- **It prints a version:** the binary works. A failed install is then about the download, write access or PATH. If it isn't on PATH, an earlier install worked and only PATH is missing. An older version is fine for this check; note it in the report.
- **`tollbit` exits with code 1 and prints nothing, after an npm install:** the package is there but its binary isn't, so the install step was skipped (`ignore-scripts`) or failed. Rerun `npm install -g @tollbit/tollbit-cli --foreground-scripts` and keep its output.
- **`exec format error` or `cannot execute binary file`:** the binary is for the wrong CPU (`file <path>` shows which). On Windows on ARM, x64 PowerShell or Node picks the amd64 build.
- **macOS, `killed: 9`, or "cannot be opened because the developer cannot be verified":** Gatekeeper. `xattr -l <path>` shows whether `com.apple.quarantine` is set.
- **Linux, `Permission denied` although the file is executable:** the directory is mounted `noexec`. `findmnt -T <path> -o OPTIONS` shows it.
- **Windows, `This program is blocked by group policy` or `Access is denied`:** AppLocker, WDAC or a software restriction policy blocks programs in user folders.
- **Windows, `Operation did not complete successfully because the file contains a virus`, or the file disappears after the download:** antivirus quarantined it.
- **Windows SmartScreen prompt:** the file has a `Zone.Identifier` stream (`Get-Item <path> -Stream Zone.Identifier`), usually because it came through a browser.

### 6. Report

Give the user this report in a code block. Fill in every line, writing `not checked` where a step didn't apply. Ask them to send it to TollBit support, and to look it over first: it contains folder paths, which may include their username.

```text
TollBit CLI install diagnostic
Date:
Run by: <Claude app sandbox | Claude Code | Codex | terminal>, <agent and model if known>
OS / arch / shell:
PowerShell language mode / execution policy:      (Windows only)
Install tried: <method>; error: <exact text>

Proxy:  env <HTTPS_PROXY / none>; npm <proxy / none>; system <macOS or Windows proxy / PAC / none>   (credentials removed)
TLS inspection: <none seen | issuer "...">; NODE_EXTRA_CA_CERTS <set / not set>

Hosts:
  tollbit.com                           <ok | dns | connect | timeout | proxy_rejected | tls_intercepted | tls_other | http_<code>>  <detail>
  raw.githubusercontent.com             <...>
  api.github.com                        <...>
  github.com                            <...>
  release-assets.githubusercontent.com  <...>
  registry.npmjs.org                    <...>

Write access: install dir <yes/no>; on PATH <yes/no>; npm prefix <path>, writable <yes/no>; elevated <yes/no>
Binary: <found at ... | none>; runs <yes: version | no: error>

Likely cause:
Suggested next step:
Fix likely needs: <you | your IT administrators | your Claude organization admin | TollBit>
```

For **Likely cause**, name the first thing that breaks the install method the user tried. Common ones:

| Finding | Suggested next step | Who |
|---|---|---|
| `tollbit.com` and the GitHub release hosts (`github.com`, `release-assets.githubusercontent.com`) all blocked | Every install method downloads from `tollbit.com`, then GitHub Releases. Allow `tollbit.com` and `*.tollbit.com`, which `tollbit` commands need anyway. | IT administrators |
| npm registry OK, the download fails in npm's install step | Set `HTTPS_PROXY` to the same proxy npm uses, then install again; or allow `tollbit.com`. | you, or IT administrators |
| TLS inspection | IT provides the corporate root certificate (a PEM file); for npm, set `NODE_EXTRA_CA_CERTS` to it. Don't turn off verification. | IT administrators, then you |
| npm prefix needs admin | Use the script installer, or set a user-owned npm prefix. If Node is installed and managed by the company, IT may need to do this. | you, or IT administrators |
| PowerShell `ConstrainedLanguage` | The PowerShell installer can't run. Install manually from GitHub Releases, if policy allows running it. | IT administrators |
| Binary blocked by AppLocker, WDAC or antivirus | Allow `tollbit.exe` from https://github.com/tollbit/cli/releases. Each release can be verified with `gh attestation verify <archive> --repo tollbit/cli`. | IT administrators |
| Claude app sandbox, proxy_rejected on `tollbit.com` or npm | Allow package managers, plus `tollbit.com` and `*.tollbit.com` under **Additional allowed domains**, in **Organization settings → Capabilities → Code execution**; then start a new session. | Claude organization admin |
| Codex or sandboxed Claude Code, every host fails the same way | Allow network access for the agent, then try again. | you |
| Checksum mismatch, a binary that crashes, or no cause found | TollBit support looks into it. | TollBit |

Fill in **Fix likely needs** from the **Who** column. On a managed work computer that is often the user's IT administrators, since only they can change the network, the proxy or what programs may run. Tell the user so, and that TollBit support will bring in their IT administrators where needed. The user sends the report to TollBit support only, not to their IT administrators.

After the report, stop. Don't try the install again, and don't carry on with any other setup steps, unless the user or the instructions that sent you here ask you to.

Once the user has the report, follow "Install: MCP instead".
