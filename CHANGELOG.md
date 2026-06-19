
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
