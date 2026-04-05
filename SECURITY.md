# Security Policy

## Supported Versions

Only the latest release receives security fixes. Once a new version is published,
the previous version is considered end-of-life.

| Version | Supported |
| ------- | --------- |
| latest  | ✅        |
| < latest | ❌       |

## Scope

Mockly is a **developer tool** intended for use in local development and CI
environments. It is explicitly **not designed to be exposed to the public internet**
or used as a production service. Please keep this in mind when evaluating the
severity of reported issues.

Issues considered in scope:

- Vulnerabilities that allow escaping the mock server's intended isolation (e.g.
  path traversal when serving static assets, arbitrary file read/write)
- Remote code execution through config parsing or API inputs
- Denial-of-service vectors that could disrupt CI pipelines (unbounded memory
  growth, panic on malformed input)
- Dependency vulnerabilities with a CVSS score ≥ 7.0

Issues considered out of scope:

- Lack of authentication/authorisation on the management API (by design —
  Mockly runs on localhost or in an isolated CI network)
- TLS not being enabled by default (same rationale)
- Issues that require physical access to the machine running Mockly
- Theoretical vulnerabilities with no practical exploit path

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report privately via GitHub's built-in private vulnerability reporting:

1. Go to the [Security tab](https://github.com/dever-labs/mockly/security)
2. Click **"Report a vulnerability"**
3. Fill in the details described below

Alternatively, email **security@dever-labs.io** with the subject line
`[mockly] Security Vulnerability Report`.

### What to include

- A clear description of the vulnerability and its impact
- Steps to reproduce (config snippets, curl commands, or a minimal Go test)
- The version(s) of Mockly affected
- Any suggested mitigations or patches (optional but appreciated)

### Response timeline

| Step | Target |
| ---- | ------ |
| Acknowledgement | Within 2 business days |
| Initial triage & severity assessment | Within 5 business days |
| Fix or workaround published | Within 30 days for high/critical; 90 days for medium/low |

We follow responsible disclosure: we will coordinate a release and notify you
before any public disclosure. If you would like credit in the release notes,
let us know when you report.

## Dependency scanning

Mockly uses [Dependabot](https://docs.github.com/en/code-security/dependabot)
for automated dependency updates. Go module and npm package vulnerabilities are
reviewed and patched on a rolling basis.

## Security best practices when running Mockly

- Run Mockly on `localhost` or a private CI network — never bind it to a
  publicly routable interface
- Use environment-specific config files; avoid committing secrets into
  `mockly.yaml`
- Pin the Mockly binary version in CI rather than using `@latest` to ensure
  reproducible, auditable builds
