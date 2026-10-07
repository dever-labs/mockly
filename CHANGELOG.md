# Changelog

## [0.15.0](https://github.com/dever-labs/mockly/compare/v0.14.0...v0.15.0) (2026-10-07)


### Features

* add install.ps1 for Windows installs ([#270](https://github.com/dever-labs/mockly/issues/270)) ([5439297](https://github.com/dever-labs/mockly/commit/5439297e53facdca21ec88b525f1d6a797034928))
* **cli:** generate a ready-to-run config from an OpenAPI spec ([#277](https://github.com/dever-labs/mockly/issues/277)) ([a87e463](https://github.com/dever-labs/mockly/commit/a87e463632dff2308ca2c71b2aae94cd048edd55))
* **cli:** generate mock configs from AsyncAPI 2.x/3.x specs ([#278](https://github.com/dever-labs/mockly/issues/278)) ([c98a2f1](https://github.com/dever-labs/mockly/commit/c98a2f1051f8bc583d3dba33ed42bba916d0d638))
* **config:** parameterize configs with a vars: map (and apply it to the Keycloak preset) ([#273](https://github.com/dever-labs/mockly/issues/273)) ([73c5c52](https://github.com/dever-labs/mockly/commit/73c5c5278ea18d6b0ad3da1ff87b8db59e356d47))
* **dotnet:** expand TFM support testing and improve NuGet READMEs ([#267](https://github.com/dever-labs/mockly/issues/267)) ([4059060](https://github.com/dever-labs/mockly/commit/40590605f85ca91abe0018ddd46f5007b2de6ef5))
* **http:** add record mode to bootstrap mocks from a real backend ([#275](https://github.com/dever-labs/mockly/issues/275)) ([844da5c](https://github.com/dever-labs/mockly/commit/844da5c77b73149d1e62bcee885dc391b1f59434))
* **protoidl:** generate gRPC mocks from Protobuf (.proto) service definitions ([#279](https://github.com/dever-labs/mockly/issues/279)) ([0f1cc69](https://github.com/dever-labs/mockly/commit/0f1cc6980ea7f39b1d9acded5ef3d979441baaf8))
* verify SHA256 checksums in install scripts ([#271](https://github.com/dever-labs/mockly/issues/271)) ([be7e4c0](https://github.com/dever-labs/mockly/commit/be7e4c0c8dbec7d2dd767a274f216fbb07aeeb82))


### Bug Fixes

* **build:** make Makefile portable across Linux/macOS/Windows ([#276](https://github.com/dever-labs/mockly/issues/276)) ([733a6ac](https://github.com/dever-labs/mockly/commit/733a6ac86894f652ba9fb48afa38153809c26087))
* correct documented/default management port from 9090 to real default 9091 ([#274](https://github.com/dever-labs/mockly/issues/274)) ([35522b4](https://github.com/dever-labs/mockly/commit/35522b4e4f8c5f237c127431f662b7a003efffef))
* **presets:** make Keycloak preset's RSA key material and JWTs real ([#272](https://github.com/dever-labs/mockly/issues/272)) ([e2b0949](https://github.com/dever-labs/mockly/commit/e2b09491c7e8f9a80972528bb7f088475254b4cc))
* **webhook:** skip webhook dispatch when URL template fails to render ([#268](https://github.com/dever-labs/mockly/issues/268)) ([ba57c0f](https://github.com/dever-labs/mockly/commit/ba57c0fd727a000fe6118cecfa3d2997579f959d))

## [0.14.0](https://github.com/dever-labs/mockly/compare/v0.13.1...v0.14.0) (2026-10-06)


### Features

* add `mockly config validate` command ([#225](https://github.com/dever-labs/mockly/issues/225)) ([b4854a8](https://github.com/dever-labs/mockly/commit/b4854a858431a973e8be5cefd86465d0ec3d4a38))
* add generic outbound webhook support + Nets/Nexi preset ([#200](https://github.com/dever-labs/mockly/issues/200)) ([fa03453](https://github.com/dever-labs/mockly/commit/fa0345392a2e4f75452e21f63979c829a29818c5))
* fault injection delay jitter (delay_range) and rate_limit mode ([#230](https://github.com/dever-labs/mockly/issues/230)) ([b7c11af](https://github.com/dever-labs/mockly/commit/b7c11afc78e46a1e1c72e926ee099884aa497ec2))
* **http:** add streaming/SSE response support ([#236](https://github.com/dever-labs/mockly/issues/236)) ([231a50c](https://github.com/dever-labs/mockly/commit/231a50c1f389ccb94a29d6935b16d8f797b0d71b))
* **nats:** add embedded NATS + JetStream protocol support ([#243](https://github.com/dever-labs/mockly/issues/243)) ([f0d8736](https://github.com/dever-labs/mockly/commit/f0d87369f571f6139540c863e2d7ec2130d6d23b)), closes [#221](https://github.com/dever-labs/mockly/issues/221)
* opt-in near-miss diagnostics for unmatched HTTP requests ([#228](https://github.com/dever-labs/mockly/issues/228)) ([361aaba](https://github.com/dever-labs/mockly/commit/361aaba3ef717241b2fa4bf5c0af42a250637088))
* opt-in Prometheus /metrics endpoint for observability ([#215](https://github.com/dever-labs/mockly/issues/215)) ([#231](https://github.com/dever-labs/mockly/issues/231)) ([f079471](https://github.com/dever-labs/mockly/commit/f079471a7969036d675e87669df0984c2342c5f1))
* query param matching supports regex, absence, and repeated values ([#227](https://github.com/dever-labs/mockly/issues/227)) ([e084485](https://github.com/dever-labs/mockly/commit/e0844855afc0940f57621d361b62c7c820da0de6)), closes [#206](https://github.com/dever-labs/mockly/issues/206)
* **redis:** add opt-in stateful mode for real key/value round-tripping ([#234](https://github.com/dever-labs/mockly/issues/234)) ([9713150](https://github.com/dever-labs/mockly/commit/9713150454b644ac00fee438aee7191adba036fc))
* state store TTL expiry and prefix-scoped reset ([#233](https://github.com/dever-labs/mockly/issues/233)) ([eb104a9](https://github.com/dever-labs/mockly/commit/eb104a9a3d87b1443760b0d9715c3db869f5d2b1))
* structured body_multipart and body_xml request matchers ([#232](https://github.com/dever-labs/mockly/issues/232)) ([ddf990d](https://github.com/dever-labs/mockly/commit/ddf990dc76530fbbe70e7ecb91388487d96dbc68))
* support ${VAR}/${VAR:-default} env var substitution in config ([#226](https://github.com/dever-labs/mockly/issues/226)) ([ea7a925](https://github.com/dever-labs/mockly/commit/ea7a925e82a9ceb2a80776e033e6fb80f7c82c5c)), closes [#214](https://github.com/dever-labs/mockly/issues/214)
* **websocket:** add binary frame matching and response support ([#235](https://github.com/dever-labs/mockly/issues/235)) ([12119e0](https://github.com/dever-labs/mockly/commit/12119e0d2b87ba32c31168ad97d9faf27bb64dfb))


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
