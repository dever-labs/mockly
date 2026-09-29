# Changelog

## [0.14.0](https://github.com/dever-labs/mockly/compare/v0.13.1...v0.14.0) (2026-09-29)


### Features

* add `mockly config validate` command ([#225](https://github.com/dever-labs/mockly/issues/225)) ([b4854a8](https://github.com/dever-labs/mockly/commit/b4854a858431a973e8be5cefd86465d0ec3d4a38))
* add generic outbound webhook support + Nets/Nexi preset ([#200](https://github.com/dever-labs/mockly/issues/200)) ([fa03453](https://github.com/dever-labs/mockly/commit/fa0345392a2e4f75452e21f63979c829a29818c5))
* opt-in near-miss diagnostics for unmatched HTTP requests ([#228](https://github.com/dever-labs/mockly/issues/228)) ([361aaba](https://github.com/dever-labs/mockly/commit/361aaba3ef717241b2fa4bf5c0af42a250637088))
* query param matching supports regex, absence, and repeated values ([#227](https://github.com/dever-labs/mockly/issues/227)) ([e084485](https://github.com/dever-labs/mockly/commit/e0844855afc0940f57621d361b62c7c820da0de6)), closes [#206](https://github.com/dever-labs/mockly/issues/206)
* support ${VAR}/${VAR:-default} env var substitution in config ([#226](https://github.com/dever-labs/mockly/issues/226)) ([ea7a925](https://github.com/dever-labs/mockly/commit/ea7a925e82a9ceb2a80776e033e6fb80f7c82c5c)), closes [#214](https://github.com/dever-labs/mockly/issues/214)


### Bug Fixes

* register missing NTLM preset, document undocumented endpoints/presets, add Scenarios and Fault Injection UI pages ([#194](https://github.com/dever-labs/mockly/issues/194)) ([1237bb8](https://github.com/dever-labs/mockly/commit/1237bb89c0eeba0be106fd94bfbbbac138fa68f1))
* remove dotnet run-file cache and add dotnet/ to .gitignore ([#139](https://github.com/dever-labs/mockly/issues/139)) ([326fa05](https://github.com/dever-labs/mockly/commit/326fa0509882fc94ba6823f37646995ef0eb7043))

## [0.13.1](https://github.com/dever-labs/mockly/compare/v0.13.0...v0.13.1) (2026-07-13)


### Bug Fixes

* add missing XML doc comments to MocklyContainer public API ([#128](https://github.com/dever-labs/mockly/issues/128)) ([37d9dfb](https://github.com/dever-labs/mockly/commit/37d9dfb0590636287acbc0ef7428bb2bea6f46db))
* update .NET SDK version and clean up init script ([#130](https://github.com/dever-labs/mockly/issues/130)) ([2cd9124](https://github.com/dever-labs/mockly/commit/2cd9124d1c7fe67b0e02f617af6a3b03c5d32ecc))

## [0.13.0](https://github.com/dever-labs/mockly/compare/v0.12.4...v0.13.0) (2026-07-08)


### Features

* full API parity for all testcontainers clients ([#112](https://github.com/dever-labs/mockly/issues/112)) ([65dec0d](https://github.com/dever-labs/mockly/commit/65dec0d7af0bf8039775ade9519a1950e88dcd05))


### Bug Fixes

* **ci:** add missing release-please manifest at v0.12.4 ([6008d70](https://github.com/dever-labs/mockly/commit/6008d70da39eb5a3f60cf309cbf227d942da215f))
* **ci:** pin release-please-action to SHA for sha_pinning_required policy ([3486218](https://github.com/dever-labs/mockly/commit/348621817953aa0db09ecd89fcdc81badd67e93c))
* **ci:** use PAT token for release-please PR creation ([ebe57c3](https://github.com/dever-labs/mockly/commit/ebe57c37df878550c35911af968ffb9082a9dd0d))
* **devcontainer:** add git configuration path to container environment ([fcb09b6](https://github.com/dever-labs/mockly/commit/fcb09b68a679cb275d407632ce6fcd43dda51a5f))
* install mockly-driver locally before building java testcontainers in CI ([#115](https://github.com/dever-labs/mockly/issues/115)) ([f9f4864](https://github.com/dever-labs/mockly/commit/f9f48649db4ced322872f3f258a8b89cf2f52cfa))
* patch security vulnerabilities in go/testcontainers and java/testcontainers ([#127](https://github.com/dever-labs/mockly/issues/127)) ([eba8dcb](https://github.com/dever-labs/mockly/commit/eba8dcbd622eb04dbeb5cf3cef9692e7e2197498))
* subscribe before fast-path check in WaitFor to avoid race ([#126](https://github.com/dever-labs/mockly/issues/126)) ([f944ef6](https://github.com/dever-labs/mockly/commit/f944ef655a52492f90416773d97c3cabc374796b))
* update .NET SDK version to 10 and bump Go module dependencies ([2a2511e](https://github.com/dever-labs/mockly/commit/2a2511efd2e298c69aa79c006ac231710faf11fa))
