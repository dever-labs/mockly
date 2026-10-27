#!/usr/bin/env python3
"""Mint a GitHub App installation token with the App key held sign-only in OpenBao.

The private key lives in OpenBao's transit engine (non-exportable); this script
only asks OpenBao to sign the App JWT, so the PEM never reaches the runner.
Replaces actions/create-github-app-token, which needs the PEM itself.

Env: VAULT_ADDR, VAULT_TOKEN, APP_ID, OWNER (org), optional TRANSIT_KEY,
     REPOSITORIES (comma-separated repo names to scope the token to),
     GITHUB_OUTPUT (writes `token=` there and masks it).
"""
import base64
import json
import os
import sys
import time
import urllib.error
import urllib.request

ADDR = os.environ["VAULT_ADDR"].rstrip("/")
VTOKEN = os.environ["VAULT_TOKEN"]
APP_ID = os.environ["APP_ID"]
OWNER = os.environ["OWNER"]
KEY = os.environ.get("TRANSIT_KEY", "github-automation-app")
REPOS = [r.strip() for r in os.environ.get("REPOSITORIES", "").split(",") if r.strip()]


def b64u(b):
    return base64.urlsafe_b64encode(b).rstrip(b"=").decode()


def call(method, url, headers, body=None):
    req = urllib.request.Request(url, method=method, headers=headers,
                                 data=json.dumps(body).encode() if body is not None else None)
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"{method} {url.split('?')[0]} -> {e.code} {e.read()[:200].decode(errors='replace')}") from None


def app_jwt():
    now = int(time.time())
    head = b64u(json.dumps({"alg": "RS256", "typ": "JWT"}).encode())
    claims = b64u(json.dumps({"iat": now - 60, "exp": now + 540, "iss": str(APP_ID)}).encode())
    signing_input = f"{head}.{claims}".encode()
    out = call("POST", f"{ADDR}/v1/transit/sign/{KEY}/sha2-256", {"X-Vault-Token": VTOKEN},
               {"input": base64.b64encode(signing_input).decode(), "signature_algorithm": "pkcs1v15"})
    sig = out["data"]["signature"].split(":", 2)[2]
    return f"{head}.{claims}.{b64u(base64.b64decode(sig))}"


def main():
    gh = {"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28",
          "User-Agent": "gh-app-token", "Authorization": "Bearer " + app_jwt()}
    inst = call("GET", f"https://api.github.com/orgs/{OWNER}/installation", gh)["id"]
    token = call("POST", f"https://api.github.com/app/installations/{inst}/access_tokens", gh,
                 {"repositories": REPOS} if REPOS else {})["token"]
    print(f"::add-mask::{token}")
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a") as f:
            f.write(f"token={token}\n")
    else:
        print(token)


if __name__ == "__main__":
    try:
        main()
    except Exception as e:  # noqa: BLE001
        print(f"::error::gh-app-token failed: {e}", file=sys.stderr)
        sys.exit(1)
