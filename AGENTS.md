# YouTrack TUI - Agent Guide & Instructions

Welcome! This workspace contains the **YouTrack TUI (Terminal User Interface)**, an interactive, terminal-based dashboard for managing JetBrains YouTrack issues and projects. It is built in Go using the **Bubble Tea** TUI framework and wraps JetBrains' Python-based command-line interface `youtrack-cli` (executable name `yt`).

This document provides a technical guide, architecture map, behavioral rules, and guidelines for AI agents working on this codebase.

---

## 🛠 Project Architecture

The application is structured into the following key packages:

### 1. Root Application
* [main.go](main.go) - Entry point. Verifies that the `youtrack-cli` command `yt` is present in the system `$PATH` (or fallback path `~/.local/bin/yt`), and initializes the Bubble Tea program.

### 2. Configuration Package
* [config.go](internal/config/config.go) - Handles loading and initializing user configuration from `~/.config/yt-tui/config.json`. Managed properties include default page sizes.

### 3. YouTrack CLI Wrapper (`ytcli`)
This package wraps execution of the python `yt` tool and parses its JSON output:
* [client.go](internal/ytcli/client.go) - Contains the [Client](internal/ytcli/client.go#L15) struct which manages command execution, authentication checks, and formatting errors.
* [models.go](internal/ytcli/models.go) - Declares Go structures matching YouTrack's schema (e.g. `Issue`, `Project`, `Comment`, `User`, `CustomField`).

### 4. User Interface Package (`ui`)
Built using Bubble Tea (`bubbletea`), Lip Gloss (`lipgloss`), and Bubbles components:
* [app.go](internal/ui/app.go) - Defines [AppModel](internal/ui/app.go#L43) which coordinates the global navigation, state transitions, sub-models, and window resizing.
* [welcome.go](internal/ui/welcome.go) - The login and connection configuration page.
* [dashboard.go](internal/ui/dashboard.go) - The homepage listing "My Open Issues" and available "Projects".
* [projects.go](internal/ui/projects.go) - A panel rendering a list of YouTrack projects.
* [issues.go](internal/ui/issues.go) - The issues list view with filtering and paginated results.
* [issue_detail.go](internal/ui/issue_detail.go) - Renders descriptions, fields, and comments for a single ticket.
* [issue_form.go](internal/ui/issue_form.go) - Form inputs for creating and cloning issues.
* [styles.go](internal/ui/styles.go) - Catppuccin Mocha color scheme definitions and style helper functions.
* [keys.go](internal/ui/keys.go) - Unified hotkey configurations.

---

## ⚠️ Critical Implementation Rules for Agents

When editing or extending this codebase, you **must** adhere to the following constraints:

### 1. Keyring/Authentication Decryption Bypass
* **Context**: When CLI wrapper commands run in background subprocesses without a TTY, python `keyring` may fail to read credentials, generate a *new* key, and overwrite the config, corrupting the stored token.
* **Rule**: Maintain the plaintext configuration bypass recommendation inside `~/.config/youtrack-cli/.env`. Do NOT force keyring utilization. If authentication errors occur, check for the presence of the plaintext fallback first.

### 2. JSON Output Sanitization
* **Context**: The `youtrack-cli` python library uses `rich.console` formatting which sometimes writes styling/non-JSON noise (or backslash escapes like `\[`) to standard output.
* **Rule**: When adding or updating calls in `client.go`, you **must** pass command output bytes through [sanitizeJSON](internal/ytcli/client.go#L98) before passing it to `json.Unmarshal`.

### 3. UI State Management and Propagating Events
* **Context**: Screen navigation and view state is centralized in [AppModel.Update](internal/ui/app.go#L82).
* **Rule**: Do not manage screen switches directly in sub-models. Instead, emit messages like `pushStateMsg`, `switchStateMsg`, or `popStateMsg` so the parent `AppModel` can coordinate screen history and window sizing correctly.

### 4. Code Aesthetics and Formatting
* Keep user interface components aligned with the Catppuccin Mocha theme defined in [styles.go](internal/ui/styles.go).
* Maintain Bubble Tea model initialization procedures. When implementing new states, add their setup steps under the switch statement in `switchState` inside `app.go`.
* **Rule**: You **must** run Go formatting (`gofmt -s -w .`) on the workspace root after modifying any Go source files to ensure code format checks pass.

### 5. README / Documentation Maintenance
* **Context**: The [README.md](README.md) acts as the single source of truth for users on keyboard mappings, config files (`config.json`), and available features.
* **Rule**: When adding new features, command-line arguments, config settings, or keyboard mappings, you **must** update [README.md](README.md) to keep documentation synchronized and styled nicely.

---

## 🛠 Build & Run Reference

* To build: `go build -o yt-tui`
* To run: `./yt-tui`
* Dependencies are listed in [go.mod](go.mod).
