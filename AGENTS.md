# YouTrack TUI - Agent Guide & Instructions

Welcome! This workspace contains the **YouTrack TUI (Terminal User Interface)**, an interactive, terminal-based dashboard for managing JetBrains YouTrack issues and projects. It is built in Go using the **Bubble Tea** TUI framework and communicates directly with YouTrack's REST API.

This document provides a technical guide, architecture map, behavioral rules, and guidelines for AI agents working on this codebase.

---

## 🛠 Project Architecture

The application is structured into the following key packages:

### 1. Root Application
* [main.go](main.go) - Entry point. Initializes the Bubble Tea program.

### 2. Configuration Package
* [config.go](internal/config/config.go) - Handles loading, parsing, and saving user configuration/credentials from `~/.config/yt-tui/config.json`. Managed properties include URL, API Token, and page sizes.

### 3. YouTrack REST API Client (`ytcli`)
This package handles native HTTP communication with YouTrack's REST API:
* [client.go](internal/ytcli/client.go) - Contains the [Client](internal/ytcli/client.go#L15) struct which manages API requests, authentication, and HTTP response handling.
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

### 1. Credentials Management
* **Rule**: Store all API connections and token credentials within the local `config.json` via the [config](internal/config/config.go) package. Do not hardcode credentials or log sensitive tokens. Maintain backwards-compatible legacy migration from `~/.config/youtrack-cli/.env` when reading credentials.

### 2. UI State Management and Propagating Events
* **Context**: Screen navigation and view state is centralized in [AppModel.Update](internal/ui/app.go#L82).
* **Rule**: Do not manage screen switches directly in sub-models. Instead, emit messages like `pushStateMsg`, `switchStateMsg`, or `popStateMsg` so the parent `AppModel` can coordinate screen history and window sizing correctly.

### 3. Code Aesthetics and Formatting
* Keep user interface components aligned with the Catppuccin Mocha theme defined in [styles.go](internal/ui/styles.go).
* Maintain Bubble Tea model initialization procedures. When implementing new states, add their setup steps under the switch statement in `switchState` inside `app.go`.
* **Rule**: You **must** run Go formatting (`gofmt -s -w .`) on the workspace root after modifying any Go source files to ensure code format checks pass.

### 4. README / Documentation Maintenance
* **Context**: The [README.md](README.md) acts as the single source of truth for users on keyboard mappings, config files (`config.json`), and available features.
* **Rule**: When adding new features, command-line arguments, config settings, or keyboard mappings, you **must** update [README.md](README.md) to keep documentation synchronized and styled nicely.

---

## 🛠 Build & Run Reference

* To build: `go build -o yt-tui`
* To run: `./yt-tui`
* Dependencies are listed in [go.mod](go.mod).
