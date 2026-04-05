"""Binary download and location helpers for mockly-driver."""

import os
import platform
import shutil
import stat
import sys
import urllib.error
import urllib.request

_DEFAULT_VERSION = "v0.1.0"
_DEFAULT_BASE_URL = "https://github.com/dever-labs/mockly/releases/download"


def _exe_suffix() -> str:
    return ".exe" if platform.system() == "Windows" else ""


def _asset_name() -> str:
    system = platform.system().lower()
    machine = platform.machine().lower()

    if machine in ("x86_64", "amd64"):
        arch = "amd64"
    elif machine in ("arm64", "aarch64"):
        arch = "arm64"
    else:
        raise RuntimeError(f"Unsupported architecture: {machine}")

    if system == "linux":
        os_name = "linux"
    elif system == "darwin":
        os_name = "darwin"
    elif system == "windows":
        os_name = "windows"
    else:
        raise RuntimeError(f"Unsupported OS: {system}")

    suffix = ".exe" if os_name == "windows" else ""
    return f"mockly-{os_name}-{arch}{suffix}"


def get_binary_path(bin_dir: str | None = None) -> str | None:
    """Return the path to an existing mockly binary, or None if not found.

    Search order:
    1. MOCKLY_BINARY_PATH environment variable (if set and file exists)
    2. <bin_dir>/mockly[.exe] (if bin_dir provided and file exists)
    3. <cwd>/bin/mockly[.exe]
    """
    env_path = os.environ.get("MOCKLY_BINARY_PATH")
    if env_path:
        return env_path if os.path.isfile(env_path) else None

    suffix = _exe_suffix()
    name = f"mockly{suffix}"

    if bin_dir:
        candidate = os.path.join(bin_dir, name)
        if os.path.isfile(candidate):
            return candidate

    cwd_candidate = os.path.join(os.getcwd(), "bin", name)
    if os.path.isfile(cwd_candidate):
        return cwd_candidate

    return None


def install(
    version: str | None = None,
    base_url: str | None = None,
    bin_dir: str | None = None,
    force: bool = False,
) -> str:
    """Download the mockly binary and return its path.

    Respects:
    - MOCKLY_NO_INSTALL  — raise RuntimeError instead of downloading
    - MOCKLY_BINARY_PATH — return immediately if the file exists
    - MOCKLY_VERSION     — version override
    - MOCKLY_DOWNLOAD_BASE_URL — base URL override
    - HTTPS_PROXY / HTTP_PROXY — proxy for downloads
    """
    if os.environ.get("MOCKLY_NO_INSTALL"):
        raise RuntimeError(
            "MOCKLY_NO_INSTALL is set; refusing to download the mockly binary. "
            "Provide a pre-staged binary via MOCKLY_BINARY_PATH or disable MOCKLY_NO_INSTALL."
        )

    env_path = os.environ.get("MOCKLY_BINARY_PATH")
    if env_path and os.path.isfile(env_path):
        return env_path

    version = version or os.environ.get("MOCKLY_VERSION") or _DEFAULT_VERSION
    base_url = base_url or os.environ.get("MOCKLY_DOWNLOAD_BASE_URL") or _DEFAULT_BASE_URL

    suffix = _exe_suffix()
    name = f"mockly{suffix}"
    dest_dir = bin_dir or os.path.join(os.getcwd(), "bin")
    dest_path = os.path.join(dest_dir, name)

    if not force and os.path.isfile(dest_path):
        return dest_path

    asset = _asset_name()
    url = f"{base_url}/{version}/{asset}"

    os.makedirs(dest_dir, exist_ok=True)

    proxy_url = os.environ.get("HTTPS_PROXY") or os.environ.get("HTTP_PROXY")
    if proxy_url:
        proxy_handler = urllib.request.ProxyHandler({"https": proxy_url, "http": proxy_url})
        opener = urllib.request.build_opener(proxy_handler)
    else:
        opener = urllib.request.build_opener()

    print(f"Downloading mockly {version} from {url} …", file=sys.stderr)
    try:
        with opener.open(url) as resp, open(dest_path, "wb") as fh:
            shutil.copyfileobj(resp, fh)
    except urllib.error.URLError as exc:
        raise RuntimeError(f"Failed to download mockly binary from {url}: {exc}") from exc

    # Make executable on Unix
    if platform.system() != "Windows":
        current = os.stat(dest_path).st_mode
        os.chmod(dest_path, current | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

    return dest_path


def main() -> None:  # pragma: no cover
    """Entry point for the ``mockly-install`` CLI script."""
    import argparse

    parser = argparse.ArgumentParser(description="Download the mockly binary")
    parser.add_argument("--version", default=None, help="Version to download (default: v0.1.0)")
    parser.add_argument("--base-url", default=None, help="Override download base URL")
    parser.add_argument("--bin-dir", default=None, help="Directory to place binary (default: <cwd>/bin)")
    parser.add_argument("--force", action="store_true", help="Re-download even if already installed")
    args = parser.parse_args()

    path = install(version=args.version, base_url=args.base_url, bin_dir=args.bin_dir, force=args.force)
    print(path)
