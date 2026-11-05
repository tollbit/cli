# Platforms

Data about the AI agent platforms (a product and surface, such as Claude Code in a terminal or the Codex CLI) that TollBit CLI and Services support.

[supported-platforms.yaml](supported-platforms.yaml) holds each platform's detection line and install section. `make prompts` builds the install prompt, [prompts/install-prompt.md](../prompts/install-prompt.md), from it and [the template](../prompts/install-prompt.template.md). After changing it, run `make prompts` and commit the result.
