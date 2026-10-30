.PHONY: test build build-release alias dev-install dev-uninstall bump third-party-notices plugin

test:
	go run gotest.tools/gotestsum@latest

build:
	go build -tags dev -o tollbit ./cmd/tollbit

# Untagged build matching release/GoReleaser (pinned production endpoints).
build-release:
	go build -o tollbit ./cmd/tollbit

# Regenerate THIRD-PARTY-NOTICES.md from the linked Go dependencies' licenses.
third-party-notices:
	bash scripts/gen-third-party-notices.sh

# Regenerate the Claude plugin's skills and the CLI version in its hand-written files.
plugin:
	go run ./cmd/gen-plugin

# Prints an alias for the repo-local binary; use: eval "$(make alias)"
alias:
	@echo 'alias tollbit="$(CURDIR)/tollbit"'

# Build and swap in local binary at installed command path.
# Existing installed binary is preserved as tollbit-original.
dev-install: build
	@INSTALL_PATH="$$(command -v tollbit 2>/dev/null || true)"; \
	if [ -z "$$INSTALL_PATH" ]; then \
	  INSTALL_PATH="$${HOME}/.local/bin/tollbit"; \
	  echo "tollbit not found in PATH; using default install path: $$INSTALL_PATH"; \
	fi; \
	INSTALL_DIR="$$(dirname "$$INSTALL_PATH")"; \
	ORIGINAL_PATH="$$INSTALL_DIR/tollbit-original"; \
	mkdir -p "$$INSTALL_DIR"; \
	echo "using installed path: $$INSTALL_PATH"; \
	if [ -f "$$INSTALL_PATH" ]; then \
	  mv -f "$$INSTALL_PATH" "$$ORIGINAL_PATH"; \
	  echo "moved installed binary to $$ORIGINAL_PATH"; \
	fi; \
	install -m 0755 "$(CURDIR)/tollbit" "$$INSTALL_PATH"; \
	echo "installed dev build at $$INSTALL_PATH"

# Restore original installed binary if present and remove dev-installed tollbit.
dev-uninstall:
	@INSTALL_PATH="$$(command -v tollbit 2>/dev/null || true)"; \
	if [ -z "$$INSTALL_PATH" ]; then \
	  INSTALL_PATH="$${HOME}/.local/bin/tollbit"; \
	fi; \
	INSTALL_DIR="$$(dirname "$$INSTALL_PATH")"; \
	ORIGINAL_PATH="$$INSTALL_DIR/tollbit-original"; \
	DEV_PATH="$$INSTALL_DIR/tollbit"; \
	if [ ! -f "$$ORIGINAL_PATH" ]; then \
	  echo "no $$ORIGINAL_PATH found; nothing to restore"; \
	  exit 0; \
	fi; \
	if [ -f "$$DEV_PATH" ]; then \
	  rm -f "$$DEV_PATH"; \
	  echo "removed dev binary at $$DEV_PATH"; \
	fi; \
	mv -f "$$ORIGINAL_PATH" "$$DEV_PATH"; \
	echo "restored original binary to $$DEV_PATH"

# Bump CLI + skill version (semver), then regenerate the plugin. Example: make bump VERSION=0.2.0
bump:
	@test -n "$(VERSION)" || (echo "usage: make bump VERSION=0.2.0  (or v0.2.0)"; exit 1)
	@python3 "$(CURDIR)/scripts/bump-version.py" "$(VERSION)"
	@$(MAKE) --no-print-directory plugin

# Private eval targets (make agent-eval, …); absent from the public tree.
-include evals/agent-eval/agent-eval.mk
