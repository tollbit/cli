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

{{hints_table}}

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
installer="$(mktemp)" && curl -fsSL --connect-timeout 20 --max-time 120 -o "$installer" "https://tollbit.com/.agents/cli/install.sh" && bash "$installer" --version v{{version}}; rc=$?; rm -f "$installer"; (exit $rc)
```

On Windows (PowerShell):

```powershell
& ([scriptblock]::Create((irm -TimeoutSec 120 "https://tollbit.com/.agents/cli/install.ps1"))) -Version v{{version}}
```

- **Approvals.** Many agents ask the user before running a command. If yours does, tell the user what the command does and ask them to approve it.
- **Network.** The installer downloads from `tollbit.com`, or from GitHub if that fails, and `tollbit` commands need network access too. Some agents run commands without network access. If yours does, ask the user to allow it. The fallbacks below need network access too, so don't try them. If you can't get access, give the user the command to run in their own terminal. In a cloud sandbox with an allowlist, allow `tollbit.com` and `*.tollbit.com`: the installer needs `tollbit.com`, and `tollbit` commands need its subdomains.
- **A hang is a failure.** If the installer times out, or prints nothing for two minutes, stop it and go to the fallbacks below. Don't wait, and don't retry the same command.
- **If the installer fails,** use the first of these that works:
  1. **npm, but only if it's already installed** (`npm --version` works): `npm install -g @tollbit/tollbit-cli@{{version}}`. Don't install Node.js or Git to get it: on a managed work computer, those installers can wait for an administrator prompt the user never sees.
  2. **The release, downloaded directly.** Download `tollbit_{{version}}_<os>_<arch>.tar.gz` (`.zip` on Windows; `<os>` is `darwin`, `linux` or `windows`, and `<arch>` is `amd64` or `arm64`) and `tollbit_{{version}}_checksums.txt` from `https://tollbit.com/.agents/cli/v{{version}}/`, or from `https://github.com/tollbit/cli/releases/download/v{{version}}/` if that fails, with the same two-minute limit. Take both files from the same place. Check the archive's SHA-256 against the checksums file, unpack it, and put `tollbit` (`tollbit.exe` on Windows) in a directory on the user's PATH, such as `~/.local/bin`.
- **Once the installer or a fallback has installed it, check it** with `tollbit doctor`, which exits 1 if a check fails. If `tollbit` isn't found, the installer put it in `~/.local/bin`: run it from there, or start a new shell.
- **If nothing installed,** don't try other sources. Follow "Install: MCP instead".
- **If `tollbit` installed but `tollbit doctor` fails,** follow "Troubleshoot a failed install" at the end of these instructions: it finds where the install breaks and writes a report for TollBit support. Once the user has the report, follow "Install: MCP instead".

### Install: Codex

Codex runs your commands in a sandbox with no network access, where only your workspace is writable. The install and every `tollbit` command need network access and write to `~/.local/bin` or `~/.config/tollbit`, so allowing only the install isn't enough.

Don't run the installer to see what happens, and don't try npm, a release download or "Troubleshoot a failed install". They all need the same access.

1. **Choose how to install.** If you have a `request_permissions` tool, and `ls -ld ~/.local ~/.local/bin ~/.config ~/.config/tollbit` shows no symlinks (a missing path is fine), go to step 2. Otherwise go to step 3.
2. **Install from here.** Make one `request_permissions` call for network access and write access to the full paths of `~/.local/bin` and `~/.config/tollbit`. Ask for nothing else, and never for a shell profile such as `~/.zshrc`: Codex can't grant a symlink, and asking for one can break your shell tool until the user starts a new chat. Ask the user to approve it for the whole session if Codex offers that. Once it's granted, run this in your shell tool, not a Node REPL or any other tool, because their sandboxes differ:

   ```bash
   installer="$(mktemp)" && curl -fsSL --connect-timeout 20 --max-time 120 -o "$installer" "https://tollbit.com/.agents/cli/install.sh" && bash "$installer" --version v{{version}} --no-modify-path; rc=$?; rm -f "$installer"; (exit $rc)
   ```

   Then run `tollbit doctor` and follow "After installing" straight away, because the access may end with this turn. If `tollbit` isn't found, the installer put it in `~/.local/bin`: run it from there, and give the user the PATH line the installer printed. If anything here fails, go to step 3.
3. **Hand it to the user.** Tell them the Codex sandbox stops you doing this, and show them both of these commands in full, to run in their own terminal, not in Codex. The first installs TollBit. The second adds a Codex rule that lets `tollbit` commands, and nothing else, run outside the sandbox without asking. Tell them that includes paid commands such as `tollbit content`.

   ```bash
   installer="$(mktemp)" && curl -fsSL --connect-timeout 20 --max-time 120 -o "$installer" "https://tollbit.com/.agents/cli/install.sh" && bash "$installer" --version v{{version}}; rc=$?; rm -f "$installer"; (exit $rc)
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

{{troubleshooting}}

Once the user has the report, follow "Install: MCP instead".
