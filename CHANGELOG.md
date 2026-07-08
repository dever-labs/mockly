# Changelog

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
