
## [0.7.1] - 2026-07-04

### Features

- *(ui)* Add option to yank current comment in details view ([43cffcf](https://github.com/nospor/yt-tui/commit/43cffcfab3ddd830d918ed016dcd837bcb1d5578))
- *(ui)* Support editing comments in external editor with Ctrl+g ([45a6901](https://github.com/nospor/yt-tui/commit/45a6901773fa69c15aa2ee8909cdb10f2f10ea86))
- *(ui)* Support viewing issue description and comments in external editor via Ctrl+g ([0b49308](https://github.com/nospor/yt-tui/commit/0b4930822005d9be3089bb921db0d3c018283f08))

## [0.7.0] - 2026-06-29

### Features

- Support custom image viewer for image attachments ([b7dc71e](https://github.com/nospor/yt-tui/commit/b7dc71e5cdbc9c60f8b45f4feba3da8e04792d9d))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.9 [skip ci] ([b049ec3](https://github.com/nospor/yt-tui/commit/b049ec3fc3cc8a4c7e0e9c8a0d192274092ebadd))

## [0.6.9] - 2026-06-29

### Features

- *(ui)* Support multiline comments with Alt+Enter in issue detail view ([5427185](https://github.com/nospor/yt-tui/commit/5427185f04cad4ab6bd9d3ed765db5406f52ae97))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.8 [skip ci] ([3acebe2](https://github.com/nospor/yt-tui/commit/3acebe226a6fe91c4412536b70bff6b4b7e35b17))

## [0.6.8] - 2026-06-27

### Features

- Support launching yt-tui directly with a YouTrack issue URL ([15404ed](https://github.com/nospor/yt-tui/commit/15404edb6a891e09a0fc652ca6de63b8e5b8dbd5))

### Bug Fixes

- *(ytcli)* Add http client timeouts and vpn/proxy troubleshooting tips ([52eee9c](https://github.com/nospor/yt-tui/commit/52eee9c305ccb50f0798e11ff1a395dfc836469d))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.7 [skip ci] ([49ccbdc](https://github.com/nospor/yt-tui/commit/49ccbdc0f84e0c28362343b209928ef945b4200a))

## [0.6.7] - 2026-06-27

### Features

- *(ui)* Add dashboard panel scrolling and fix column alignment ([6bbe278](https://github.com/nospor/yt-tui/commit/6bbe27877ecada4c546694129f6cd121f8c9f6f7))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.6 [skip ci] ([69bf93f](https://github.com/nospor/yt-tui/commit/69bf93fa848285cce50797f055d43f71f958f3a0))

## [0.6.6] - 2026-06-25

### Features

- *(form)* Show errors as dismissable popup overlay ([7d82f79](https://github.com/nospor/yt-tui/commit/7d82f798d4a27c0c30eb6906828d7fbc04b471ba))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.5 [skip ci] ([a1d1d6e](https://github.com/nospor/yt-tui/commit/a1d1d6e4cc3584766f0022a34493b965f24fd218))

## [0.6.5] - 2026-06-25

### Features

- Make custom_states configurable per-project ([7db7958](https://github.com/nospor/yt-tui/commit/7db79586da66408b594c730e3dcd847b2212ac63))
- Storefeat: store and retrieve favorite view per connection/server ([68e2c13](https://github.com/nospor/yt-tui/commit/68e2c133b192a02e9923a2a5b3f0fe621835cf62))
- Make custom_priorities configurable per-project and merge list filters ([b5dcd70](https://github.com/nospor/yt-tui/commit/b5dcd70b88911a31970c008d1c8d2f47ab02f598))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.4 [skip ci] ([78b7e4e](https://github.com/nospor/yt-tui/commit/78b7e4e854dfb5b3f6722cb607c19bfb97705174))

## [0.6.4] - 2026-06-24

### Features

- Add global search popup with YouTrack URL parsing ([66a595b](https://github.com/nospor/yt-tui/commit/66a595b5fc19ecdb34ac26d29990f8a55b37cdbe))

            - Add a global search popup overlay accessible from any screen by
            pressing capital `S` (except when an input field/textarea is focused).
            - Implement a two-mode interaction: "Input Mode" to type/paste queries
            and "Results Mode" to navigate matches using arrows or j/k/Ctrl+N/Ctrl+P
            keys.
            - Add automatic task ID extraction (e.g. 'PRJ-21797') when pasting full
            YouTrack ticket URLs into the search input.
            - Optimize YouTrack API queries by querying `issue id` only for valid ID
            formats, removing invalid state query keywords, and using multi-word
            prefix wildcards.
- Add interactive help popup triggered by '?' ([014ee82](https://github.com/nospor/yt-tui/commit/014ee82f10d3931480fd277497333a030c81caf0))

### Other

- Fix popup background style leakage in overlay views ([7661ab0](https://github.com/nospor/yt-tui/commit/7661ab0edbfa3451bbf67940505da0db02d22bd5))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.3 [skip ci] ([859b458](https://github.com/nospor/yt-tui/commit/859b4586795eeb51fa1a8a78a277496c01519c18))

## [0.6.3] - 2026-06-24

### Features

- Add support for created, updated, updater, and reporter columns in issues list ([e2e1f22](https://github.com/nospor/yt-tui/commit/e2e1f22b57454ffa2f3862d0d8bade41a28a23ba))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.2 [skip ci] ([f56ddf4](https://github.com/nospor/yt-tui/commit/f56ddf4c11de28d40059e8726da8427698798e95))

## [0.6.2] - 2026-06-23

### Features

- *(ui)* Add custom quick action templates triggered by space ([46dabd9](https://github.com/nospor/yt-tui/commit/46dabd980b8219c4dff4421959392feec4de6bc1))

            - Implement spacebar shortcut in issues list and issue details views
            - Support configuring multi-command sequences in config.json under
            `actions`
            - Allow quick execution of actions via selection list or direct shortcut
            keys
            - Support update_field, comment, and assign command actions
- *(ui)* Add "Issues created by me" special project to dashboard ([d016d47](https://github.com/nospor/yt-tui/commit/d016d476a70f30e2c5b78c52a77bab54f28d5b80))

### Bug Fixes

- *(ui)* Resolve layout height overflow in task details view ([cf0266c](https://github.com/nospor/yt-tui/commit/cf0266cdc94c65372e56a1e14dc468725a48e049))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.1 [skip ci] ([dff30ce](https://github.com/nospor/yt-tui/commit/dff30ce6c99fa1e393b8b313d52c5630e610054a))

## [0.6.1] - 2026-06-22

### Features

- *(ui)* Add shortcut to delete issue links in details view ([d05e580](https://github.com/nospor/yt-tui/commit/d05e58047cfc2b2379df63a316451ada476bb15f))
- Link cloned issues and transition to detail view on clone ([b0f51dc](https://github.com/nospor/yt-tui/commit/b0f51dca21ab0c1f733c3cc7041988e734f4350f))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.6.0 [skip ci] ([1a18778](https://github.com/nospor/yt-tui/commit/1a18778a59f0138ca5be9fdda6fa1aa506f73c03))

## [0.6.0] - 2026-06-22

### Features

- *(ui)* Add clipboard image pasting for issues and comments ([6c9c6a2](https://github.com/nospor/yt-tui/commit/6c9c6a2ca445cca34d7a42416c41d437d1cb0178))
- *(ui)* Add local file picker ([8668e1d](https://github.com/nospor/yt-tui/commit/8668e1d390b0831ce295f9b1c680a090f5f5ef05))
- Attach file in task detail view ([ddebcb8](https://github.com/nospor/yt-tui/commit/ddebcb8a1996816cc8e054f3492a894dc8120d55))
- *(ui)* Support deleting issue attachments ([f15111d](https://github.com/nospor/yt-tui/commit/f15111d571f5b5b4d25895ca509865fa6db1bed6))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.5 [skip ci] ([fe6f201](https://github.com/nospor/yt-tui/commit/fe6f20170a119e393499e8af3f9838cd5fcc1eb7))

## [0.5.5] - 2026-06-22

### Features

- *(ui)* Display issue creation and last update info in task details ([6819b3a](https://github.com/nospor/yt-tui/commit/6819b3a553348302fdab9287e622b9380ba128e3))
- *(ui)* Display Repo custom field in issue detail view ([9256d3a](https://github.com/nospor/yt-tui/commit/9256d3aa12614b787dcf519b8dde79945f478efe))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.4 [skip ci] ([78fe2b8](https://github.com/nospor/yt-tui/commit/78fe2b8d33eba110e699b80fe41833a8fd49ae79))

## [0.5.4] - 2026-06-22

### Features

- *(ui)* Add 'Repo' custom field update with project-specific config mapping ([cbfc803](https://github.com/nospor/yt-tui/commit/cbfc80364a58d5035673b53687a50a6710d69497))

            - Implement capital 'R' shortcut in issue details view to select and
            update a task's 'Repo' custom field.
            - Implement map-based `repo_options` configuration
            (`map[string][]string`) to allow project-specific repository options as
            fallback definitions.
            - Pre-pend a "No repo" option in the selection menu to clear/empty the
            field in YouTrack.
            - Support fallback resolution using project ShortName or project
            database ID.

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.3 [skip ci] ([037ad26](https://github.com/nospor/yt-tui/commit/037ad263aab47d8067073763d3d14e0bf08f6b05))

## [0.5.3] - 2026-06-22

### Features

- *(ui)* Add comments and activity stream filtering in issue details ([b858ed2](https://github.com/nospor/yt-tui/commit/b858ed2f102115f16484aa7a3c2749914df586f8))
- *(ui)* Add 'yi' shortcut to yank issue ID to clipboard ([7a6f580](https://github.com/nospor/yt-tui/commit/7a6f5809edc4bd52f31ba2e5a100b9ea0a19b423))
- Add markdown rendering for task descriptions with 'm' toggle ([00386ad](https://github.com/nospor/yt-tui/commit/00386ad7f3b7ba236cc94f12611cf699111d8463))

### Bug Fixes

- *(ui)* Align box styles with viewport widths to fix double-wrapping and layout jumping ([ffb9041](https://github.com/nospor/yt-tui/commit/ffb9041e3db2ab3d62277213a8796ed150d562a9))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.5.2 [skip ci] ([029428a](https://github.com/nospor/yt-tui/commit/029428a2ef7804edddeca1b8e962ff04b0d87d90))

## [0.5.2] - 2026-06-21

### Bug Fixes

- *(ci)* Discard unstaged changes before rebase in release workflow ([3bb8103](https://github.com/nospor/yt-tui/commit/3bb81033d693225ceddb90393ab9c142408c1e33))

## [0.5.1] - 2026-06-21

### Features

- *(ui)* Add support for navigating and editing comments in issue detail view ([a38a651](https://github.com/nospor/yt-tui/commit/a38a65191a0e3c3f41f72cccb7f6c5a8a030c027))

### Testing

- *(ui)* Mock clipboard in yank tests for headless CI compatibility ([9fca8b3](https://github.com/nospor/yt-tui/commit/9fca8b3386d047f32f92e6911569ccc4350f21f5))

## [0.5.0] - 2026-06-21

### Features

- *(ui)* Add task yanking motion to details view ([2e428c3](https://github.com/nospor/yt-tui/commit/2e428c3a99c19d8d44b151e254d9a7679ffa636a))

            - Pressing 'y' enters a yank mode, showing a sleek options popup in the
            corner.
            - Pressing 's' copies the issue ID and summary (one line,
            space-separated) to the clipboard.
            - Pressing 'd' copies the issue description to the clipboard.
            - Copy success status messages temporarily replace the bottom help bar
            for 5 seconds (or until the next keystroke) to prevent any layout
            shifting or viewport jumping.
- Add 'yu' shortcut to yank issue URLs ([1a53fca](https://github.com/nospor/yt-tui/commit/1a53fcabc891da205d18a95f724e1ddedaffd729))
- *(ui)* Add "Issues created by me" special project to projects list ([99e2acb](https://github.com/nospor/yt-tui/commit/99e2acbc263cf4b6e03728eb9ffdb83786836e0f))
- *(detail)* Add creator information to task detail metadata ([17674d9](https://github.com/nospor/yt-tui/commit/17674d93c883d7d09642f2e9779736f1495bf2cc))
- *(ui)* Support unassigning tasks via "unassigned" keyword ([282d09e](https://github.com/nospor/yt-tui/commit/282d09e65fc13b10961afb70bf1d190465d16214))
- Add favorite tasks view navigation and toggle mapping ([69ad1cc](https://github.com/nospor/yt-tui/commit/69ad1cc93acb198803d545360de2bcccb7827afc))

            - Store favorite view configurations in `config.json` via a new
            `favorite_view` setting.
            - Use `f` on the dashboard to jump to the saved favorite tasks list.
            - Use `f` on a tasks list (project, board/sprint, or "created by me") to
            toggle favorite status.
            - Remap issues list filter hotkey to `F` (Shift+f).
            - Render a yellow star `★` indicator next to favorited views' headers.
- Add attachments support and dynamic download/open via xdg-open in issue detail view ([b818da8](https://github.com/nospor/yt-tui/commit/b818da8b3310820ff10c9c9e7503bdce7b005000))
- *(ui)* Add time tracking popup dialog to task details view ([594527d](https://github.com/nospor/yt-tui/commit/594527d0566d5d41edf897723c384e4b3d29a971))

            - Implement keyboard shortcut `t` in task details view to open a time
            tracking popup.
            - Integrate interactive monthly calendar grid with arrow-key navigation
            for date selection.
            - Implement regex parser to convert `1w 1d 1h 1m` duration strings to
            minutes (based on 5d workweeks and 8h workdays).
            - Add `work_types` configuration parameter to config.json with fallback
            defaults.
- *(ui)* Support shift+tab to cycle panels backwards in issue details ([19ce18d](https://github.com/nospor/yt-tui/commit/19ce18d5dcde7d2139b5980d988a501e122c5705))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.4.0 [skip ci] ([a874acb](https://github.com/nospor/yt-tui/commit/a874acb7bda797770dc10e01981407daf8230455))

## [0.4.0] - 2026-06-21

### Features

- *(config)* Support environment variable placeholders in config.json ([9f77c00](https://github.com/nospor/yt-tui/commit/9f77c007a96b52914b025d857b4a7b7831e61726))
- Add multi-server selection on startup ([1301d8e](https://github.com/nospor/yt-tui/commit/1301d8e6b81e648e940ba2f9c62c64280e51e099))
- *(ui)* Support editing issue description in external editor with Ctrl+g ([9ba9e5e](https://github.com/nospor/yt-tui/commit/9ba9e5e3f62c6258f9b6b671abc56c8804e52b3a))
- *(ui)* Add interactive linked issues pane in detail view ([0aa7f79](https://github.com/nospor/yt-tui/commit/0aa7f792a170e0bac19fdda768df3c750332bc4a))
- *(ui)* Normalize issue link relations and improve detail viewport scrolling ([d5c3b26](https://github.com/nospor/yt-tui/commit/d5c3b2639f64901a1eff6fc1eb7c0466545be434))

### Bug Fixes

- *(ui)* Fix alignment of text inputs and labels on welcome screen ([52e4a18](https://github.com/nospor/yt-tui/commit/52e4a18c92fa28f7844bf60c48961e49e2f381b9))
- *(ui)* Synchronize table heights in Update to prevent disappearing rows on scroll ([4cf8ee5](https://github.com/nospor/yt-tui/commit/4cf8ee590fc3a256276c476300f10cc8386fc677))

### Other

- Move links pane under description and fix navigation order ([c9c9298](https://github.com/nospor/yt-tui/commit/c9c929849fef7580a5106d9ffbdbe7f56dfc8239))
- Apply strikethrough to closed linked task IDs and sort them to the bottom ([689d61a](https://github.com/nospor/yt-tui/commit/689d61a0ffcff602d883525f88cf9d4811cab819))

### Styling

- Move issue details section titles to top borders and fix viewport alignment ([2109ccc](https://github.com/nospor/yt-tui/commit/2109ccca4e7f00b44987271b825b92a8422b1a1d))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.3.1 [skip ci] ([1e3efcc](https://github.com/nospor/yt-tui/commit/1e3efccc5ac0202d268a63ff1f6c8626a847c400))

## [0.3.1] - 2026-06-20

### Documentation

- *(readme)* Add installation instructions for pre-built release binaries ([8366968](https://github.com/nospor/yt-tui/commit/83669682238d12b172576b52b274e0259b94ed8b))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.3.0 [skip ci] ([546b7ef](https://github.com/nospor/yt-tui/commit/546b7ef52b70d8056fab927b9fcdd0fdf8f08e3c))

## [0.3.0] - 2026-06-20

### Features

- *(ui)* Support updating task details from detail view ([9f9cb9d](https://github.com/nospor/yt-tui/commit/9f9cb9d46a3d4968ab2f2dee0b03d28158b18f6f))
- *(ui)* Add agile boards view with expandable sprints subtree and direct issue querying ([d50b2d3](https://github.com/nospor/yt-tui/commit/d50b2d3852df3c7f6be0a7d2b0ddfd24b7974e80))

            - Implement full-screen Agile Boards view (navigated via 'b' from
            dashboard)
            - Add expandable subtree to load and display sprints under each agile
            board
            - Filter board sprints to show up to 5 previous and 5 next sprints
            relative to the current week
            - Fetch sprint issues directly using the YouTrack sprint API resource
            (bypassing search text parsers for robust compatibility)
- *(ui)* Add state and priority filtering to issues list view ([40d11fd](https://github.com/nospor/yt-tui/commit/40d11fd281e03e74166fca603548eecb33888101))

            - Implement local issue filtering by State and Priority
            - Add interactive checklist popup triggered by pressing 'f'
            - Save filter selections in configuration file config.json for
            persistence
- *(ui)* Support column sorting on issues list with config persistence ([091dda7](https://github.com/nospor/yt-tui/commit/091dda72eb4af7e8401c8ed7badbd332ede5ef5a))

            - Bind 's' key to open an interactive Sort popup modal
            - Support sorting by ID (natural sort), Priority/State (severity order),
            and other fields (alphabetic)
            - Add sort_column and sort_direction configuration options to persist
            preferences
            - Display ▲/▼ direction indicator arrow next to the sorted column header

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.2.0 [skip ci] ([7ce7336](https://github.com/nospor/yt-tui/commit/7ce7336c39b006e0307077fb20b97781abab2a16))

## [0.2.0] - 2026-06-19

### Features

- *(cli)* Implement native API state updates and support custom workflow states ([b7e4a27](https://github.com/nospor/yt-tui/commit/b7e4a27405b651d523bbccc2356c49b78b4eaedd))
- *(ui)* Auto-refresh tasks list after creation or state updates ([dae25af](https://github.com/nospor/yt-tui/commit/dae25af2ce7c15c37ae4824e143da11168fa158a))
- *(ui)* Preserve search filter and cursor when navigating back to issues list ([241238d](https://github.com/nospor/yt-tui/commit/241238d7f715e88d834b4f2ac00926bb4f07e9eb))

### Bug Fixes

- Use readable issue ID format for YouTrack CLI commands ([df31a23](https://github.com/nospor/yt-tui/commit/df31a23994625cf242e6dbfb8fbc2321e287f46f))
- *(ytcli)* Resolve 'me' keyword and normalize assignee usernames ([0017b82](https://github.com/nospor/yt-tui/commit/0017b823f084f750062c78ecf6b45e1d1115a70d))

### Refactor

- Migrate from YouTrack CLI wrapper to direct REST API client ([5f5c3df](https://github.com/nospor/yt-tui/commit/5f5c3dfac05a020bd9391c582ac2e31d1dd2aef7))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.1.2 [skip ci] ([e24cfbe](https://github.com/nospor/yt-tui/commit/e24cfbef6c0686405b3ebe815ac2e0ede8399812))
- Update CHANGELOG.md for v0.1.3 [skip ci] ([8e35fe2](https://github.com/nospor/yt-tui/commit/8e35fe2d286aff1d58d6575140b4d94b155c294f))

## [0.1.3] - 2026-06-19

### Styling

- Format Go files and update AGENTS.md formatting rule ([395e39a](https://github.com/nospor/yt-tui/commit/395e39ad932ec4991f4f3b14899c44b633d220b1))

## [0.1.2] - 2026-06-19

### Features

- Add configurable issue load limit to prevent performance degradation ([0dd01c7](https://github.com/nospor/yt-tui/commit/0dd01c76f4239c55db06f601c7c8462e5d130916))

            - Added `max_issues` configuration option (default: 500) in
            `internal/config/config.go`
            - Updated `issuesModel` in `internal/ui/issues.go` to cease background
            paging when the threshold is met
            - Ensured cache state correctly updates and treats limit exhaustion as
            fully loaded
- *(ui)* Improve issue creation form layout and add dropdown inputs ([f2cac29](https://github.com/nospor/yt-tui/commit/f2cac296d281e5818ba26060e19d6e86438ab3a3))
- *(config)* Support custom YouTrack types/priorities and improve issue creation ([283b6a5](https://github.com/nospor/yt-tui/commit/283b6a5fa5f67fc2ccd9ce3f34cdec061412345f))

### Bug Fixes

- Enable 'q' key to quit app on non-input screens ([98b0a98](https://github.com/nospor/yt-tui/commit/98b0a98c9fb278efae28cb9ec020b411de50cd1b))

### Documentation

- Update README with configuration details and add agent documentation rule ([669f5cc](https://github.com/nospor/yt-tui/commit/669f5cca7239ec8d00e29eb5a0e057c5bdcc013a))

### Styling

- Remove background color from state badges ([b514bfd](https://github.com/nospor/yt-tui/commit/b514bfdbfd485066ebdb588e6a7b50a568c23830))
- *(ui)* Modernize top header layout and design ([9d47cff](https://github.com/nospor/yt-tui/commit/9d47cffc4e65007f43aa066846fc3c19b95dacdf))

### Miscellaneous Tasks

- Update CHANGELOG.md for v0.1.1 [skip ci] ([3033184](https://github.com/nospor/yt-tui/commit/3033184167553901be973f8e649f59cf10566378))

## [0.1.1] - 2026-06-19

### Features

- Add versioning, goreleaser, and github release workflows ([c0bd57b](https://github.com/nospor/yt-tui/commit/c0bd57b3046b496d40ffe152b0f53d6c2b09ce45))

### Bug Fixes

- Ignore release notes and goreleaser outputs in gitignore ([479a33d](https://github.com/nospor/yt-tui/commit/479a33db4a7ee9b9e53da1d6109cee271532b011))

## [0.1.0] - 2026-06-19

### Features

- Page size and config ([478ff1e](https://github.com/nospor/yt-tui/commit/478ff1e99e3762e1f25d87d42238d7303b7cbd96))
- *(ui)* Implement progressive loading and caching for project tasks ([ca08ec6](https://github.com/nospor/yt-tui/commit/ca08ec6bfb75421bdd6e88976b41da946a9e282a))
- Make issue search a local filter matching summary and ID ([42a2db7](https://github.com/nospor/yt-tui/commit/42a2db7b851d807d383d8f3e04e121ec838dc5fe))
- *(ui)* Fix detail view scrolling, prevent layout overflow, and add J/K scrolling keys ([47b55c9](https://github.com/nospor/yt-tui/commit/47b55c907f252e2461c65ab441d640efc24c6c42))

### Bug Fixes

- Logging ([6a03614](https://github.com/nospor/yt-tui/commit/6a0361436b6e38c3879ae9f6aff6fedac0fde542))
- Now j/k works as should ([5466dff](https://github.com/nospor/yt-tui/commit/5466dff5699df858f3cf585bac3311242ab21ef7))
- Highligting tasks ([2828e94](https://github.com/nospor/yt-tui/commit/2828e946a9936ea01c6e2c5846dd3033ddf24e51))
- Tasks lists doesnt break now ([a53de19](https://github.com/nospor/yt-tui/commit/a53de1993f324b3cafa619e02072185ff86111bb))
- *(ytcli)* Use correct search query syntax for fetching issues by ID ([849dad9](https://github.com/nospor/yt-tui/commit/849dad9a734c47095859417d3efb5829cee36ba3))
