# Copilot instructions for this repository

## Documentation site (MkDocs Material)

This repo's documentation lives as plain Markdown under `docs/`, built into a
static site with MkDocs Material and published to GitHub Pages
(`https://dever-labs.github.io/mockly/`, eventually `mockly.dever.dk`). The
`docs/` folder is the source of truth — always browsable directly on GitHub —
and the generated site is just a build artifact. Never treat the GitHub Wiki
or any other location as a parallel source of docs.

### When adding or editing docs pages

- Every page **must start with a single `#` (H1) heading**, not `##`. If the
  content's first heading isn't an H1, mkdocs-material injects its own `<h1>`
  from the nav title on top of it, producing a visible duplicate header. Keep
  the rest of the hierarchy consistent below that (`##` for top sections,
  `###` for subsections, etc.) — don't skip levels.
- New pages must be added to the `nav:` tree in `mkdocs.yml`, or they won't
  appear in navigation (MkDocs will still build them, just unreachable from
  the sidebar).
- Links to anything **outside `docs/`** (e.g. `configs/presets/*.yaml`,
  root-level `CONTRIBUTING.md`, `LICENSE`, source under `clients/`) must use
  full GitHub blob URLs
  (`https://github.com/dever-labs/mockly/blob/main/...`), not relative paths.
  MkDocs' strict-mode link checker only resolves paths inside `docs_dir` and
  will fail the build on anything else.
- Internal anchor links use **MkDocs/python-markdown's slugifier, not
  GitHub's** — they can disagree on headings containing punctuation (e.g. a
  `/` in a heading can produce a different hyphen count). After adding a
  cross-page anchor link, verify the generated `id=` in the built HTML rather
  than assuming GitHub's rendered slug is correct.
- Always validate before committing:
  ```bash
  pip install -r requirements-docs.txt   # or use a venv — see below
  mkdocs build --strict                  # must complete with zero warnings
  ```
- README.md is intentionally a short landing page (quickstart + links into
  `docs/`), not a full reference — don't re-grow it back into a monolith.
  Add substantial new content under `docs/` instead.

### Local environment notes

- `pip` is not preinstalled and the base image refuses system-wide installs
  ("externally managed"). Use a venv, e.g.:
  ```bash
  apt-get install -y python3.12-venv
  python3 -m venv /tmp/mkdocs-venv
  /tmp/mkdocs-venv/bin/pip install -r requirements-docs.txt
  source /tmp/mkdocs-venv/bin/activate && mkdocs build --strict
  ```

### CI/deploy workflow (`.github/workflows/docs.yml`)

- `build` job runs `mkdocs build --strict` on every PR touching
  `docs/**`/`mkdocs.yml`/`requirements-docs.txt` as a required check.
- `deploy` job runs only on `push` and publishes via
  `actions/upload-pages-artifact` + `actions/deploy-pages` straight to GitHub
  Pages' own storage through OIDC — **it never writes to any git branch or
  ref**, and in particular never touches `main`. There is deliberately no
  `gh-pages` branch in this repo; don't recreate one as a deploy target.
- This org's Actions policy only allows **SHA-pinned actions from a fixed
  allow-list** (see other workflows for the pattern: `uses: owner/repo@<full
  commit sha> # owner/repo@vX.Y.Z`). Before adding or upgrading any
  `uses:` step here, confirm the action is on the allow-list, and always pin
  to a full commit SHA, never a tag or branch.
- The `github-pages` deployment environment restricts which branches may
  deploy to it (`deployment-branch-policies` API) — currently `main` only.
  If a temporary branch preview is ever needed again, add that branch to the
  policy and remember to remove it afterward; don't leave stale entries.
