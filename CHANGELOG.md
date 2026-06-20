
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
