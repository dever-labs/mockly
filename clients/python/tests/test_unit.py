"""Unit tests for mockly-driver (no real binary required)."""

import os
import tempfile

import pytest

from mockly_driver._install import get_binary_path, install
from mockly_driver._server import _get_free_port


# ---------------------------------------------------------------------------
# _get_free_port
# ---------------------------------------------------------------------------

def test_get_free_port_returns_valid_port():
    port = _get_free_port()
    assert 1025 <= port <= 65535


# ---------------------------------------------------------------------------
# get_binary_path
# ---------------------------------------------------------------------------

def test_get_binary_path_returns_none_when_missing(tmp_path, monkeypatch):
    monkeypatch.delenv("MOCKLY_BINARY_PATH", raising=False)
    result = get_binary_path(bin_dir=str(tmp_path / "nonexistent"))
    assert result is None


def test_get_binary_path_respects_env_var(tmp_path, monkeypatch):
    fake_binary = tmp_path / "mockly"
    fake_binary.write_bytes(b"fake")
    monkeypatch.setenv("MOCKLY_BINARY_PATH", str(fake_binary))
    result = get_binary_path()
    assert result == str(fake_binary)


def test_get_binary_path_ignores_missing_env_var(tmp_path, monkeypatch):
    monkeypatch.setenv("MOCKLY_BINARY_PATH", str(tmp_path / "does_not_exist"))
    result = get_binary_path()
    assert result is None


# ---------------------------------------------------------------------------
# install
# ---------------------------------------------------------------------------

def test_install_raises_with_mockly_no_install(monkeypatch):
    monkeypatch.setenv("MOCKLY_NO_INSTALL", "1")
    monkeypatch.delenv("MOCKLY_BINARY_PATH", raising=False)
    with pytest.raises(RuntimeError, match="MOCKLY_NO_INSTALL"):
        install()


def test_install_returns_existing_binary_via_env(tmp_path, monkeypatch):
    fake_binary = tmp_path / "mockly"
    fake_binary.write_bytes(b"fake")
    monkeypatch.setenv("MOCKLY_BINARY_PATH", str(fake_binary))
    monkeypatch.delenv("MOCKLY_NO_INSTALL", raising=False)
    result = install()
    assert result == str(fake_binary)
