# @tollbit/tollbit-cli

A CLI client for searching and grounding content for agents, allows access to all content in the [TollBit](https://tollbit.com) network.

Installing this package downloads and verifies (SHA-256) the native `tollbit` binary for your OS from [GitHub Releases](https://github.com/tollbit/cli/releases).

## Install

```bash
npm install -g @tollbit/tollbit-cli
```

Requires Node.js 18+. Supported platforms: macOS, Linux, and Windows on x64 or arm64.

Then:

```bash
tollbit search "climate policy"
tollbit content pricing https://example.com/article
tollbit content fetch https://example.com/article --confirm
```

Primary workflow: install → `search` → `content pricing` → `content fetch`. Authentication is triggered automatically by the CLI when needed.

`content fetch` charges money — pricing is shown and confirmed before the request unless you pass `--confirm` (automation still incurs the cost).

## For agents

Run `tollbit guide` for orientation and the automation contract (exit codes, `--json` output, non-interactive fetch). Register the bundled skill with `tollbit guide --install <SKILLS_DIR>`.

## Update

```bash
npm update -g @tollbit/tollbit-cli
```

## Docs

Full install options (curl / PowerShell / manual), configuration, and command reference:

**https://github.com/tollbit/cli**
