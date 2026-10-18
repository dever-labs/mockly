# Changelog

## [0.17.0](https://github.com/dever-labs/mockly/compare/v0.16.0...v0.17.0) (2026-10-18)


### Features

* **clients:** add Kotlin DSL wrapper for the Java client ([#382](https://github.com/dever-labs/mockly/issues/382)) ([9b1a294](https://github.com/dever-labs/mockly/commit/9b1a294de6de886e4a31d79c699b1e5f4a359e23))
* **node:** publish an npm CLI wrapper for npx mockly start ([#379](https://github.com/dever-labs/mockly/issues/379)) ([e340a8e](https://github.com/dever-labs/mockly/commit/e340a8eb9d66c2108264482d35f9e42a033b7f99))
* **release:** adopt goreleaser to unify the binary/checksum/Docker pipeline ([#376](https://github.com/dever-labs/mockly/issues/376)) ([65e302f](https://github.com/dever-labs/mockly/commit/65e302f223bb39e2422b1fdc3e28cc8ce7a11b58))
* **release:** publish .deb/.rpm packages via nfpm ([#377](https://github.com/dever-labs/mockly/issues/377)) ([6a88297](https://github.com/dever-labs/mockly/commit/6a88297b2f6df1d84ebaf384837d79face66cf18)), closes [#367](https://github.com/dever-labs/mockly/issues/367)
* **release:** publish a Homebrew cask to dever-labs/homebrew-tap ([#378](https://github.com/dever-labs/mockly/issues/378)) ([4bbbff0](https://github.com/dever-labs/mockly/commit/4bbbff041e520e94b767142437b1c3ffdef59bcf)), closes [#363](https://github.com/dever-labs/mockly/issues/363)
* **release:** publish a Scoop bucket manifest for Windows installs ([#380](https://github.com/dever-labs/mockly/issues/380)) ([973a8de](https://github.com/dever-labs/mockly/commit/973a8deab3fbcf8f671240e4839e12ea6c66280e))


### Bug Fixes

* **actions:** make setup-mockly a real node20 action with post-job teardown ([#375](https://github.com/dever-labs/mockly/issues/375)) ([40dc603](https://github.com/dever-labs/mockly/commit/40dc603704b5efbd074783e85ad966aa4efa6403))
* **http:** add nosniff header and universal Content-Type guard ([#373](https://github.com/dever-labs/mockly/issues/373)) ([acf1906](https://github.com/dever-labs/mockly/commit/acf1906a44ac7128c12804296a25e8ba4cc0f808))

## [0.16.0](https://github.com/dever-labs/mockly/compare/v0.15.0...v0.16.0) (2026-10-14)


### Features

* **amqp:** add configurable exchange/binding topology ([#219](https://github.com/dever-labs/mockly/issues/219)) ([#334](https://github.com/dever-labs/mockly/issues/334)) ([f2716db](https://github.com/dever-labs/mockly/commit/f2716db932fe10499c69d2a11a261385837c588e))
* **config:** support multi-file config via include and directory mode ([#339](https://github.com/dever-labs/mockly/issues/339)) ([f8ff1c2](https://github.com/dever-labs/mockly/commit/f8ff1c266ebf8f2df4607633e968634cf183a5df))
* **graphql:** add graphql-transport-ws subscription support ([#346](https://github.com/dever-labs/mockly/issues/346)) ([2a0b931](https://github.com/dever-labs/mockly/commit/2a0b9319c13512c33f1f9e4a303cc965ccc90eab))
* **grpc:** add server/client/bidirectional streaming support ([#211](https://github.com/dever-labs/mockly/issues/211)) ([#345](https://github.com/dever-labs/mockly/issues/345)) ([e794894](https://github.com/dever-labs/mockly/commit/e794894caf308fd3b01115c0e96ba6724d622180))
* **http:** add easier HTML page mocking ([#202](https://github.com/dever-labs/mockly/issues/202)) ([#347](https://github.com/dever-labs/mockly/issues/347)) ([23eec43](https://github.com/dever-labs/mockly/commit/23eec435dbce20eb72955ab6bdf0db3fb963350c))
* **http:** add per-listener TLS scoping ([#204](https://github.com/dever-labs/mockly/issues/204)) ([#342](https://github.com/dever-labs/mockly/issues/342)) ([87a338d](https://github.com/dever-labs/mockly/commit/87a338dcc11d9b43c41c0a193301f7da68af5960))
* inline schema references in mockly.yaml + e2e protocol coverage ([#281](https://github.com/dever-labs/mockly/issues/281)) ([6ca93f8](https://github.com/dever-labs/mockly/commit/6ca93f8da70c1020741dd24c34f30b4d7701ac2c))
* **protocols:** add optional protocolVersion pin per protocol ([#237](https://github.com/dever-labs/mockly/issues/237)) ([#332](https://github.com/dever-labs/mockly/issues/332)) ([7d39ed8](https://github.com/dever-labs/mockly/commit/7d39ed8ae254dda29e36178603cb62d18a481888))
* **start:** add --watch flag for config hot-reload ([#340](https://github.com/dever-labs/mockly/issues/340)) ([e27cea5](https://github.com/dever-labs/mockly/commit/e27cea55939adba4b245b3b66eaad351b6660851))
* **tlsutil:** auto-generate self-signed TLS certs when none configured ([#341](https://github.com/dever-labs/mockly/issues/341)) ([261a7f0](https://github.com/dever-labs/mockly/commit/261a7f01623b11a37e21db6dd229b846f3b85f29))


### Bug Fixes

* **deps:** bump rustls to 0.23.45 to resolve moderate vulnerabilities ([b684e94](https://github.com/dever-labs/mockly/commit/b684e94e6dedb4cfa95cbcda1d508cbae0eaa2d3))
* **docker,ci:** run container as non-root, add govulncheck + coverage gate ([#285](https://github.com/dever-labs/mockly/issues/285)) ([64ff87c](https://github.com/dever-labs/mockly/commit/64ff87c2eaf1933c8b21d1c984ab9a7470e7aae8))
* **http,tls:** close leaked listeners, scope record mode to listener, lock cert-gen ([#343](https://github.com/dever-labs/mockly/issues/343)) ([f302ca6](https://github.com/dever-labs/mockly/commit/f302ca68fcbcf2ea37a10952f946e7301cbd3ae2))
* **http:** HTML-escape reflected request data in templated responses ([#361](https://github.com/dever-labs/mockly/issues/361)) ([3aa1f90](https://github.com/dever-labs/mockly/commit/3aa1f9040f4c7d6152d3916a2bfd63d90b755e41))
* **node:** add explicit types:node to tsconfig for TypeScript 7 compat ([#330](https://github.com/dever-labs/mockly/issues/330)) ([0a30af6](https://github.com/dever-labs/mockly/commit/0a30af67e44291ab137cf34c5743df2f051bcad3))
* **node:** resync package-lock.json with optional driver deps ([#321](https://github.com/dever-labs/mockly/issues/321)) ([f5bf230](https://github.com/dever-labs/mockly/commit/f5bf230765a2cb0bc4367bb65687b2402e6cd01e))
* **snmp:** replace unmaintained GoSNMPServer with native agent ([#344](https://github.com/dever-labs/mockly/issues/344)) ([905250b](https://github.com/dever-labs/mockly/commit/905250b5beafbf7442eb5f441f1c15804ba4d432))

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
