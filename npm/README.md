# @tollbit/tollbit-cli

A CLI client for searching and grounding content for agents, llows access to all content in the [TollBit](https://tollbit.com) network.

Installing this package downloads the native `tollbit` binary for your OS from [GitHub Releases](https://github.com/tollbit/cli/releases).

## Install

```bash
npm install -g @tollbit/tollbit-cli
```

Requires Node.js 18+.

Then:

```bash
tollbit search "climate policy"
tollbit content pricing https://example.com/article
tollbit content fetch https://example.com/article --confirm
```

Primary workflow: install → `search` → `content pricing` → `content fetch`. Authentication is triggered automatically by the CLI when needed.

## Update

```bash
npm update -g @tollbit/tollbit-cli
```

## Docs

Full install options (curl / PowerShell / manual), configuration, and command reference:

**https://github.com/tollbit/cli**
