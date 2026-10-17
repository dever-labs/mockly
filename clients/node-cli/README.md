# @dever-labs/mockly-cli

Run the [Mockly](https://github.com/dever-labs/mockly) mock server via `npx` — no local install, Docker, or shell script required.

```sh
npx @dever-labs/mockly-cli start --config mockly.yaml
```

The first run downloads the matching platform binary (same mechanism as
[`@dever-labs/mockly-driver`](https://www.npmjs.com/package/@dever-labs/mockly-driver)); subsequent runs reuse it.

## Installation

Use directly via `npx` (no install step needed):

```sh
npx @dever-labs/mockly-cli start
```

Or install it as a dev dependency so `mockly` is available as a local script/alias:

```sh
npm install --save-dev @dever-labs/mockly-cli
```

```json
{
  "scripts": {
    "mocks": "mockly start --config mockly.yaml"
  }
}
```

## Usage

All arguments are forwarded as-is to the underlying `mockly` binary — see the
[CLI reference](https://github.com/dever-labs/mockly/blob/main/docs) for the full command list:

```sh
npx @dever-labs/mockly-cli start
npx @dever-labs/mockly-cli preset use keycloak
npx @dever-labs/mockly-cli --help
```

## Environment variables

Same resolution/override mechanism as `@dever-labs/mockly-driver`:

- `MOCKLY_BINARY_PATH` — use this exact binary; skips all download logic
- `MOCKLY_NO_INSTALL` — fail fast instead of downloading (for air-gapped environments where the binary is staged externally)
- `MOCKLY_DOWNLOAD_BASE_URL` — base URL override for Artifactory / internal mirrors
- `MOCKLY_VERSION` — binary version override
- `HTTPS_PROXY` / `HTTP_PROXY` — route the download through an HTTP proxy

## Alternatives

Already use Mockly in a Node.js/TypeScript test suite? Use
[`@dever-labs/mockly-driver`](https://www.npmjs.com/package/@dever-labs/mockly-driver) instead —
it embeds the same binary-resolution logic behind a programmatic `MocklyServer` API designed for
tests, rather than a standalone CLI.

## License

MIT
