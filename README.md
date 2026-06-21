# 🎛️ YouTrack Terminal User Interface (TUI)

A sleek, keyboard-driven terminal dashboard for JetBrains YouTrack, written in Go using the **Bubble Tea** framework. It communicates directly with the YouTrack REST API to provide high-performance, asynchronous, and interactive issue management without requiring any external Python CLI.

Styled out-of-the-box with a vibrant **Catppuccin Mocha** color palette, `yt-tui` keeps you in your terminal flow while tracking your project tasks.

---

## ✨ Features

- **🎛️ Interactive Dashboard (Home)**: A split-pane interface showing **My Open Issues** (unresolved issues assigned to you) alongside your **Projects**. Seamlessly toggle focus with `Tab`.
- **📂 Global Project Browser**: View and browse all accessible YouTrack projects in a clean tabular view.
- **⚡ Asynchronous Paginated Loading**: Loads issues in the background without blocking the UI, giving you an instantly responsive view even on large codebases.
- **🔍 Filter & Search**: Instantly query and filter issues in lists by readable ID or summary text using local filtering.
- **📝 Comprehensive Issue Detail**: Renders full markdown descriptions, assignees, priorities, states, and custom fields. Supports dedicated scrollable viewports for description and comments.
- **🔄 Complete Issue Lifecycle**:
  - **Create & Clone**: Instantly spawn new issues or clone existing ones (pre-populating description, type, priority, and assignee details).
  - **State Transitions**: Transition states (e.g. `Open` ➔ `In Progress` ➔ `Fixed`) dynamically.
  - **Assigning**: Quickly assign tickets to other team members or self-assign with `me`.
  - **Commenting**: Write and submit markdown comments directly from the detail view.
- **🛡️ Native REST API Integration**: Directly interacts with YouTrack REST API endpoints, completely avoiding Python keyring lockouts, credential corruption, or subprocess TTY issues.

---

## 🚀 Prerequisites

If you choose to build the project from source, you will need:
1. **Go Compiler**: Go 1.18+ installed on your system.

---

## 🛠️ Installation & Building

### 📦 Option 1: Download Pre-built Binaries (Recommended)

You can download the latest pre-compiled binary for your platform directly from the [GitHub Releases](https://github.com/nospor/yt-tui/releases) page.

1. Go to [Releases](https://github.com/nospor/yt-tui/releases) and download the archive matching your operating system and architecture (Linux, macOS, Windows; AMD64/ARM64).
2. Extract the downloaded archive.
3. (Optional) Copy the `yt-tui` binary to a directory in your `PATH` (e.g., `/usr/local/bin/` on Linux/macOS) to run it from anywhere.

### 🛠️ Option 2: Build from Source

If you have Go installed, you can build the application manually:

```bash
git clone https://github.com/nospor/yt-tui.git
cd yt-tui

# to quickly build
go build -o yt-tui .

# or (builds slower, but binary is smaller)
go build -trimpath -ldflags="-s -w" -o yt-tui .

# then run
./yt-tui

# you may also want to copy the binary to your PATH (and run it from any place), e.g.:
sudo cp yt-tui /usr/local/bin/
```

---

## ⚙️ Configuration

`yt-tui` loads its user settings and credentials from a JSON file located at `~/.config/yt-tui/config.json`. The file and its parent directories are automatically created with default settings on the first run.

### Environment Variables & `.env` Files (Optional)
You can resolve values in `config.json` dynamically from environment variables by prefixing the value with `$`. For example:

```json
{
  "url": "youtrack url",
  "token": "$SOMETOKEN"
}
```

When `yt-tui` encounters a value starting with `$`, it resolves it using:
1. **System Environment Variables**: (e.g., `os.Getenv("SOMETOKEN")`).
2. **Local/Config `.env` Files**: It checks for the variable definition in a local `.env` file in the current working directory, falling back to `~/.config/yt-tui/.env`.

If you do not specify a `url` or `token` in `config.json` at all, `yt-tui` will check standard fallback variables: `YOUTRACK_BASE_URL` and `YOUTRACK_TOKEN`.

If you have legacy credentials configured in `~/.config/youtrack-cli/.env`, they will be automatically migrated to your `config.json` on the first launch.

### Multi-Server Support (Optional)

If you manage issues across multiple YouTrack instances, you can configure an array of servers in your `config.json`. When the `servers` array is set, the application will display a list of these instances on startup for you to choose from.

```json
{
  "servers": [
    {
      "name": "Work YouTrack",
      "url": "https://work.youtrack.cloud",
      "token": "$WORK_TOKEN"
    },
    {
      "name": "Personal YouTrack",
      "url": "https://personal.youtrack.cloud",
      "token": "perm:abc123yourtokenhere"
    }
  ]
}
```

If you select one of the configured servers, `yt-tui` will authenticate using those credentials. You can also select the `➕ Connect to another YouTrack...` option to manually input a new server's URL and token.

### Default Config Structure

```json
{
  "url": "",
  "token": "",
  "page_size": 20,
  "max_issues": 500,
  "fields": ["ID", "Summary", "State", "Priority", "Assignee"],
  "custom_types": ["Bug", "Task", "Ops", "Initiative", "Epic"],
  "custom_priorities": ["0 - Immediate action", "1 - Interrupt current sprint", "2 - Must have", "3 - Should have", "4 - Nice to have"],
  "custom_states": ["Open", "In Progress", "Verified", "Done", "Duplicate", "Won't fix", "Incomplete"],
  "sort_column": "ID",
  "sort_direction": "asc"
}
```

### Configuration Options

| Option                | Type             | Default                                              | Description                                                                                                                                                                                 |
| --------------------- | ---------------- | ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `url`                 | String           | `""`                                                 | The base URL of your YouTrack instance (e.g. `https://company.youtrack.cloud`).                                                                                                             |
| `token`               | String           | `""`                                                 | Your permanent YouTrack API token.                                                                                                                                                          |
| `servers`             | Array of Objects | `[]` (empty)                                         | List of YouTrack server configurations (each containing `name`, `url`, and `token`) to choose from on startup. |
| `page_size`           | Integer          | `20`                                                 | The number of issues requested per query page. Larger page sizes fetch more records at once, while smaller sizes load background pages in quicker increments.                               |
| `max_issues`          | Integer          | `500`                                                | The maximum number of issues to load for a project list. Once this limit is reached, the app will stop fetching new pages of issues to prevent performance degradation on massive projects. |
| `fields`              | Array of Strings | `["ID", "Summary", "State", "Priority", "Assignee"]` | The list of columns/fields to display on the tasks list. Supports standard fields (`ID`, `Summary`, `State`, `Priority`, `Assignee`, `Type`) as well as custom field names.                 |
| `custom_types`        | Array of Strings | `[]` (empty)                                         | Custom list of issue type options to populate the creation dropdown instead of the standard default list (Bug, Feature, Task, etc.).                                                        |
| `custom_priorities`   | Array of Strings | `[]` (empty)                                         | Custom list of issue priority options to populate the creation dropdown instead of the standard default list (Minor, Normal, Major, etc.).                                                  |
| `custom_states`       | Array of Strings | `[]` (empty)                                         | Custom list of issue state transition options to populate the state selection modal instead of the standard default list (Open, In Progress, Verified, etc.).                               |
| `filtered_states`     | Array of Strings | `[]` (empty)                                         | List of selected issue states to display on the tasks list. States not in this list will be filtered out.                                                                                   |
| `filtered_priorities` | Array of Strings | `[]` (empty)                                         | List of selected issue priorities to display on the tasks list. Priorities not in this list will be filtered out.                                                                           |
| `sort_column`         | String           | `""`                                                 | The column by which the tasks list is sorted (e.g. `ID`, `Summary`, `State`, etc.).                                                                                                         |
| `sort_direction`      | String           | `""`                                                 | The sorting direction (`asc` or `desc`).                                                                                                                                                    |

---

## ⌨️ Keyboard Controls & Navigation

### 🌐 Global Controls
* `Ctrl+C`: Force quit the application at any time.

### 🚪 Welcome / Login View
* **Server Selection Mode** (when `servers` is configured):
  - `↑` / `↓` (or `k` / `j`): Navigate the server list.
  - `Enter`: Select and authenticate with the chosen YouTrack instance.
  - `q` / `Ctrl+C`: Force quit the application.
* **Credentials Entry Mode**:
  - `Tab` / `Shift+Tab`: Switch focus between YouTrack Base URL and API Token fields.
  - `Enter`: Save credentials and authenticate.
  - `Esc`: Return to Server Selection Mode (if `servers` array is set).
  - `Ctrl+C`: Force quit the application.

### 🏠 Dashboard View (Home Screen)
* `Tab` / `Shift+Tab`: Switch active focus panel (**My Open Issues** vs **Projects**).
* `↑` / `↓` (or `k` / `j`): Scroll items inside the active focus panel.
* `Enter`: Open the highlighted item:
  - Selecting an issue opens the **Issue Detail** view.
  - Selecting a project opens the **Issues List** filtered to that project.
* `n`: Create a new issue (pre-selects the highlighted project if focused).
* `p`: View the full **Projects List** screen.
* `b`: View the full **Agile Boards** screen.
* `r`: Reload and refresh all dashboard data.
* `q`: Exit application.

### 📂 Projects List View
* `↑` / `↓` (or `k` / `j`): Navigate projects table.
* `Enter`: View issues inside the selected project.
* `n`: Create a new issue in the selected project.
* `r`: Refresh projects list.
* `Esc` / `Backspace`: Go back to the dashboard.

### 🎛️ Agile Boards View
* `↑` / `↓` (or `k` / `j`): Navigate agile boards table.
* `Enter`: View issues belonging to the selected board.
* `r`: Refresh agile boards list.
* `Esc` / `Backspace`: Go back to the dashboard.

### 📋 Issues List View
* `↑` / `↓` (or `k` / `j`): Scroll issues table.
* `Enter`: View details of the selected issue.
* `/`: Activate search/filter mode. Type keywords to filter issues by summary/ID. Press `Esc` or `Enter` to close search mode.
* `f`: Open the State & Priority filter panel. Use arrow keys/hjkl to navigate, Space to toggle checkboxes, Enter to save, and Esc to cancel.
* `s`: Open the Column & Direction sort panel. Use arrow keys/hjkl to navigate, Space to select choices, Enter to save, and Esc to cancel.
* `n`: Create a new issue in the current project context.
* `r`: Clear cache and force reload issues list.
* `Esc` / `Backspace`: Go back to the dashboard/previous screen.

### 🔍 Issue Detail View
* `Tab`: Cycle focus between the Description, Comments, and Links (Parents/Children) viewports.
* `↑` / `↓` (or `k` / `j`): Scroll/navigate within the active viewport. In the Links viewport, this moves the selection cursor.
* `Enter` (in Links viewport): Jump directly to the highlighted parent/child task.
* `c`: Add a comment. Type your comment and press `Enter` to submit, or `Esc` to cancel.
* `s`: Transition issue state (opens state input prompt; e.g. type `In Progress` or `Fixed` and hit `Enter`).
* `a`: Assign issue (opens assignee input prompt; type username or `me` and hit `Enter`).
* `e`: Edit/update this issue's details (Summary, Description, Priority, Type, Assignee).
* `C`: Clone this issue. Pre-populates the new issue form with this ticket's details.
* `r`: Force refresh issue details, comments, and links.
* `Esc` / `Backspace`: Go back to the issues list (or the previous issue if navigated via links).

### 📝 Issue Form (Create / Clone / Edit)
* `Tab` / `Shift+Tab` / `↑` / `↓`: Move focus between form fields (Project, Summary, Description, Type, Priority, Assignee).
* `←` / `→` (or `h` / `l`): Cycle options in dropdown fields (Project, Type, Priority).
* `a`-`z` (on dropdown fields): Pressing the first letter of an option jumps directly to that choice.
* `Ctrl+g` (on Description field): Open preferred external editor (using the `$EDITOR` environment variable) to write/edit the description.
* `Ctrl+s` (or `Enter` on text inputs): Submit the form and save/create/clone the issue.
* `Esc`: Cancel and discard changes.

## License

TeamsTUI is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Thanks For Visiting
Hope you liked it. Wanna **[buy Me a coffee](https://www.buymeacoffee.com/nospor)**?

