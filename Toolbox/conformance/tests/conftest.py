"""
Shared fixtures for CBWeb3 Toolbox conformance tests.

Configuration via environment variables:
  CBWEB3_BASE_URL   — Target implementation URL (default: http://localhost:4010)
  CBWEB3_AUTH_TOKEN  — Bearer token for authentication (default: synthetic token)
"""

import os
import pytest


def pytest_addoption(parser):
    parser.addoption(
        "--base-url",
        default=None,
        help="Base URL of the CBWeb3 API (overrides CBWEB3_BASE_URL env var)",
    )


@pytest.fixture(scope="session")
def base_url(request):
    """Base URL for the target CBWeb3 implementation."""
    url = request.config.getoption("--base-url")
    if url is None:
        url = os.environ.get("CBWEB3_BASE_URL", "http://localhost:4010")
    return url.rstrip("/")


@pytest.fixture(scope="session")
def auth_token():
    """Bearer token for authentication."""
    return os.environ.get(
        "CBWEB3_AUTH_TOKEN",
        "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.SYNTHETIC_TOKEN_CB001",
    )


@pytest.fixture(scope="session")
def auth_headers(auth_token):
    """Standard request headers with authentication."""
    return {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {auth_token}",
    }
