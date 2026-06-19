# YouTrack Terminal User Interface (TUI)

A terminal-based interface for JetBrains YouTrack, written in Go using the **Bubble Tea** framework. It wraps the Python-based YouTrack CLI (`youtrack-cli` / `yt`) to provide interactive issue browsing, project navigation, detail viewing, state updating, assigning, commenting, ticket creation, and ticket cloning.

---

## Prerequisites

1. **Go Compiler**: Go 1.18+ installed on your system.
2. **YouTrack CLI (`youtrack-cli`)**:
   - The TUI wraps the `yt` command-line utility.
   - Install the CLI via pip/pipx/uv (e.g. `pip install youtrack-cli` or `uv tool install youtrack-cli`).
   - Verify it is available in your standard system PATH (the TUI will search the PATH and fallback to `~/.local/bin/yt`).

---

## Building and Running

1. **Clone/Navigate** to the project directory:
   ```bash
   cd /home/robertn/projects/vag1/html/yt-tui
   ```

2. **Build** the application:
   ```bash
   go build -o yt-tui
   ```

3. **Run** the executable:
   ```bash
   ./yt-tui
   ```

---

## Keyboard Controls & Navigation

### Global Keys
* `Ctrl+C`: Force quit the application from any screen at any time.

### Welcome / Login View
* `Tab` / `Shift+Tab`: Switch focus between YouTrack Base URL and API Token fields.
* `Enter`: Save credentials and authenticate.
* `q`: Exit.

### Dashboard View (Home Screen)
* `Tab` / `Shift+Tab`: Switch between **My Open Issues** and **Projects** panels.
* `↑` / `↓` (or `j` / `k`): Navigate items within the active panel.
* `Enter`: Select the highlighted item.
  - Selecting an issue goes to the **Issue Detail** view.
  - Selecting a project opens the **Issues List** filtered by that project.
* `n`: Create a new issue in the highlighted project.
* `p`: Open the full **Projects List** table.
* `r`: Refresh the dashboard data.
* `q`: Quit.

### Issues List View
* `↑` / `↓` (or `j` / `k`): Scroll through issues.
* `Enter`: View details of the selected issue.
* `/`: Toggle search/filter mode. Type a keyword to search issues. Press `Esc` or `Enter` to close search mode.
* `n`: Create a new issue in this project.
* `Esc`: Go back to the previous screen.

### Issue Detail View
* `↑` / `↓` (or `j` / `k`): Scroll the issue description viewport.
* `PageUp` / `PageDown` (or `Ctrl+U` / `Ctrl+D`): Scroll the comments list viewport.
* `c`: Add a comment. Type your comment and press `Ctrl+S` to submit, or `Esc` to cancel.
* `s`: Transition issue state (opens state input, type a state like `Open`, `In Progress`, `Fixed`, and press `Enter`).
* `a`: Assign issue (opens assignee input, type username or `me` and press `Enter`).
* `C`: Clone this issue. Pre-populates the new issue form with this ticket's details.
* `r`: Refresh issue details.
* `Esc`: Go back to the previous screen.

---

## Troubleshooting: Keyring & Authentication Issues

### The Keyring Decryption Bug
When running YouTrack CLI wrapper commands in background processes without an active TTY session (as is necessary to fetch data in the background for a TUI), the Python `keyring` library may fail to read your secure password vault.

When this read failure occurs, `youtrack-cli` automatically generates a *new* encryption key and overwrites the existing one in the keyring. This immediately corrupts all previously saved credentials, resulting in:
* `Failed to decrypt credential` warnings logged to stderr.
* The CLI outputting raw Fernet-encrypted strings (starting with `gAAAAA...`) as the URL or Token, which causes subsequent network queries to fail.

### The Plaintext Config Bypass (Recommended Fix)
To bypass the system keyring and prevent decryption errors permanently, you can store your credentials in plaintext inside the YouTrack CLI configuration file.

1. **Clear corrupted keyring credentials** by running the logout command in your terminal:
   ```bash
   yt auth logout
   ```
   *(Press `y` and `Enter` when prompted for confirmation)*

2. **Configure plaintext fallback**:
   Create or edit the configuration file at `~/.config/youtrack-cli/.env` and write your connection variables in plaintext:
   ```env
   YOUTRACK_BASE_URL='https://youtrack.example.com/'
   YOUTRACK_TOKEN='perm:your-api-token-here'
   YOUTRACK_VERIFY_SSL='true'
   ```
   *(Be sure to replace the values with your actual YouTrack instance URL and personal API token)*

3. **Restricting File Permissions** (Security Best Practice):
   Because the token is saved in plaintext, restrict read/write access to your user account only:
   ```bash
   chmod 600 ~/.config/youtrack-cli/.env
   ```

When these environment variables are set in the `.env` file and the keyring entries are cleared, `youtrack-cli` will bypass the keyring entirely, running extremely fast and avoiding any future authentication errors.
