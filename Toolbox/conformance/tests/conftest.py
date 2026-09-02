"""
Shared fixtures for the CBWeb3 Toolbox conformance suite.

AUTHENTICATION — read this before changing anything here.

The CBWeb3 API Gateway v2.3.0 does NOT use `Authorization: Bearer` on its
`/api/v1` surface. It uses an **HttpOnly cookie named `access_token`**, declared
in all three contracts as:

    CookieAuth: { type: apiKey, in: cookie, name: access_token }
    Attributes: HttpOnly, SameSite=Strict, Path=/;
                Secure follows the gateway's COOKIE_SECURE env var.

`BearerAuth` exists **only** in the Scenario B (`amm`) contract, and only as an
ALTERNATIVE to the cookie on the 12 `/api/v2` operations guarded by
RequireAnyAuth. The two cross-currency swap endpoints are cookie-only.

There is **NO CSRF mechanism**: no `X-CSRF-Token`, no double-submit cookie, no
synchroniser token, no Origin/Referer check anywhere in either platform spec or
in the gateway source. `SameSite=Strict` is the sole mitigation. This suite
therefore sends no CSRF header — inventing one would misrepresent the platform.
The absence is recorded as a finding in spec/security_checklist.md.

Because the credential is a cookie, every request must go through a single
`requests.Session` so the jar persists. Bare `requests.post(...)` calls with a
static header dict cannot hold a session.

CONFIGURATION

  CBWEB3_BASE_URL        Bare origin, no /api/v1 suffix.  Default http://localhost:4010
  CBWEB3_AUTH_MODE       mock | direct | pki | bearer     Default mock
  CBWEB3_PROFILE         mock | scenario-a-bank | scenario-a-cb
                             | scenario-b-bank | scenario-b-cb   Default mock
  CBWEB3_CLIENT_ID       direct + pki modes
  CBWEB3_CLIENT_SECRET   direct + pki modes
  CBWEB3_USER_ID         pki mode  (Keycloak UUID)
  CBWEB3_KEY_PEM         pki mode  (path to the participant P-256 private key)
  CBWEB3_CERT_PEM        pki mode  (path to the participant certificate)
  CBWEB3_NONCE_ENCODING  pki mode  utf8 | hex   Default utf8  (see sign_nonce)
  CBWEB3_BEARER_TOKEN    bearer mode
  CBWEB3_TIMEOUT         per-request timeout in seconds.  Default 15
  CBWEB3_REFUND_WAIT_SECONDS  live HTLC expiry wait.      Default 20

  CBWEB3_AUTH_BASE_URL / CBWEB3_PVP_BASE_URL / CBWEB3_AMM_BASE_URL
      Optional per-domain overrides. Against a real gateway all three default to
      CBWEB3_BASE_URL, because one gateway serves the whole surface. In `mock`
      mode, when CBWEB3_BASE_URL is left at the default Prism port 4010, they
      default to the three-instance Prism convention: 4010 pvp / 4011 amm /
      4012 auth.

`CBWEB3_AUTH_TOKEN` no longer exists anywhere in this repository.

THE FOUR MODES

  direct  POST /api/v1/auth/login {clientId, clientSecret} -> AuthResponse plus
          Set-Cookie. The roles routed here are ROLE_GOVERNANCE, ROLE_SUPERVISOR,
          ROLE_NOC.
  pki     POST /api/v1/auth/login {clientId, clientSecret} -> {"nonce": "<hex>"}
          with NO tokens and NO cookies; sign the nonce with the participant key
          and POST /api/v1/auth/wallet/bind {user_id, nonce_signature_hex,
          cert_pem}, which sets the cookies. This is the flow commercial banks
          (ROLE_COMMERCIAL_BANK, ROLE_TREASURY) actually perform.
  bearer  Sets `Authorization: Bearer`. Valid ONLY on the Scenario B /api/v2
          RequireAnyAuth routes. Tests that may run in this mode carry the
          `bearer_ok` marker; every other test is skipped.
  mock    THE CI ESCAPE HATCH. Prism is stateless and serves no login flow, so
          no login is attempted: a synthetic cookie is injected straight into
          the jar. This works because Prism validates only the PRESENCE of an
          apiKey-in-cookie credential, never its value — and a request without
          the cookie still yields 401, which is a useful negative assertion.
          Prism cannot persist state, so mock-mode runs assert status code and
          response shape only; every multi-step test carries `live_only` and is
          skipped automatically.
"""

import os
from urllib.parse import urlparse

import pytest
import requests

AUTH_MODES = ("mock", "direct", "pki", "bearer")
PROFILES = (
    "mock",
    "scenario-a-bank",
    "scenario-a-cb",
    "scenario-b-bank",
    "scenario-b-cb",
)

DEFAULT_BASE_URL = "http://localhost:4010"
# Three-instance Prism convention, matching the `servers` block of each contract.
PRISM_PORTS = {"pvp": 4010, "amm": 4011, "auth": 4012}

# Prism validates the presence of the cookie credential, never its value.
SYNTHETIC_COOKIE = "SYNTHETIC_COOKIE_CI"


# ---------------------------------------------------------------------------
# CLI options
# ---------------------------------------------------------------------------
def pytest_addoption(parser):
    parser.addoption(
        "--base-url",
        default=None,
        help="Base origin of the CBWeb3 gateway (overrides CBWEB3_BASE_URL). "
        "No /api/v1 suffix — tests use full contract paths.",
    )
    parser.addoption(
        "--auth-mode",
        default=None,
        choices=list(AUTH_MODES),
        help="Authentication mode (overrides CBWEB3_AUTH_MODE). Default: mock.",
    )
    parser.addoption(
        "--profile",
        default=None,
        choices=list(PROFILES),
        help="Gateway profile (overrides CBWEB3_PROFILE). Default: mock. "
        "Route groups are registered conditionally per deployment, so a 404 may "
        "mean 'this gateway is not that kind of node' rather than a defect.",
    )


# ---------------------------------------------------------------------------
# Configuration fixtures
# ---------------------------------------------------------------------------
@pytest.fixture(scope="session")
def auth_mode(request):
    mode = request.config.getoption("--auth-mode") or os.environ.get(
        "CBWEB3_AUTH_MODE", "mock"
    )
    if mode not in AUTH_MODES:
        raise pytest.UsageError(
            f"CBWEB3_AUTH_MODE={mode!r} is not one of {AUTH_MODES}"
        )
    return mode


@pytest.fixture(scope="session")
def profile(request):
    value = request.config.getoption("--profile") or os.environ.get(
        "CBWEB3_PROFILE", "mock"
    )
    if value not in PROFILES:
        raise pytest.UsageError(f"CBWEB3_PROFILE={value!r} is not one of {PROFILES}")
    return value


@pytest.fixture(scope="session")
def timeout():
    return float(os.environ.get("CBWEB3_TIMEOUT", "15"))


@pytest.fixture(scope="session")
def base_url(request):
    url = request.config.getoption("--base-url") or os.environ.get(
        "CBWEB3_BASE_URL", DEFAULT_BASE_URL
    )
    return url.rstrip("/")


def _domain_url(base, domain, auth_mode):
    """
    Resolve the base URL for one contract domain.

    A real gateway serves all three domains, so every domain resolves to the
    single base URL. Only in `mock` mode, and only when the base URL is still on
    the default Prism port, do we fan out to the 4010/4011/4012 convention.
    """
    override = os.environ.get(f"CBWEB3_{domain.upper()}_BASE_URL")
    if override:
        return override.rstrip("/")
    if auth_mode == "mock":
        parsed = urlparse(base)
        if parsed.port == PRISM_PORTS["pvp"]:
            return f"{parsed.scheme}://{parsed.hostname}:{PRISM_PORTS[domain]}"
    return base


@pytest.fixture(scope="session")
def auth_url(base_url, auth_mode):
    return _domain_url(base_url, "auth", auth_mode)


@pytest.fixture(scope="session")
def pvp_url(base_url, auth_mode):
    return _domain_url(base_url, "pvp", auth_mode)


@pytest.fixture(scope="session")
def amm_url(base_url, auth_mode):
    return _domain_url(base_url, "amm", auth_mode)


def _env(name):
    value = os.environ.get(name)
    if not value:
        pytest.fail(
            f"{name} is required for CBWEB3_AUTH_MODE={os.environ.get('CBWEB3_AUTH_MODE')}. "
            "See Toolbox/conformance/README.md."
        )
    return value


def _read_pem(path_env):
    path = _env(path_env)
    if os.path.exists(path):
        with open(path, "r", encoding="utf-8") as handle:
            return handle.read()
    # The value may itself be the PEM text (useful for CI secrets).
    if "-----BEGIN" in path:
        return path
    pytest.fail(f"{path_env}={path!r} is neither an existing file nor PEM text.")


def sign_nonce(key_pem, nonce_hex):
    """
    Produce the `nonce_signature_hex` for POST /api/v1/auth/wallet/bind.

    The contract documents the field as "DER-encoded ECDSA signature in hex
    format" over the nonce returned by /auth/login. What it does NOT document is
    whether the bytes signed are the nonce's hex TEXT or its DECODED bytes.
    Both platform specs are silent. Rather than guess, this helper signs the hex
    text by default and exposes CBWEB3_NONCE_ENCODING=hex to sign the decoded
    bytes instead. If wallet/bind returns 401 NONCE_SIGNATURE_MISMATCH in one
    encoding, try the other — the ambiguity is upstream, not here.
    """
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import ec

    key = serialization.load_pem_private_key(key_pem.encode("utf-8"), password=None)
    encoding = os.environ.get("CBWEB3_NONCE_ENCODING", "utf8")
    payload = bytes.fromhex(nonce_hex) if encoding == "hex" else nonce_hex.encode("utf-8")
    return key.sign(payload, ec.ECDSA(hashes.SHA256())).hex()


# ---------------------------------------------------------------------------
# Sessions
# ---------------------------------------------------------------------------
def _new_session():
    session = requests.Session()
    session.headers.update({"Accept": "application/json"})
    # No CSRF header: the platform implements none.
    return session


def _inject_synthetic_cookie(session):
    """
    Install the CI cookie with NO domain, deliberately.

    A domain-scoped cookie is silently dropped against a `localhost` origin:
    http.cookiejar's eff_request_host() rewrites `localhost` to
    `localhost.local`, so a cookie stored for domain `localhost` never matches
    and every request arrives unauthenticated — a 401 that looks like a gateway
    defect and is not. A domain-less cookie is returned to every host, which is
    exactly what a test fixture wants. The same trap catches any cookie-jar
    client pointed at a localhost sandbox.
    """
    session.cookies.set("access_token", SYNTHETIC_COOKIE)


@pytest.fixture(scope="session")
def anon_session():
    """
    A session that carries NO credential at all.

    Used for the public operations (`security: []`) and for the negative
    assertion that a protected operation answers 401 without the cookie. That
    assertion holds against Prism as well as against a live gateway.
    """
    session = _new_session()
    yield session
    session.close()


@pytest.fixture(scope="session")
def session(auth_mode, auth_url, timeout):
    """
    The authenticated session. Cookie-jar backed, shared for the whole run.

    A login failure is a hard failure, never a skip: a conformance suite that
    goes green because it silently could not authenticate is exactly the failure
    mode this suite exists to prevent.
    """
    http = _new_session()

    if auth_mode == "mock":
        _inject_synthetic_cookie(http)

    elif auth_mode == "bearer":
        http.headers["Authorization"] = f"Bearer {_env('CBWEB3_BEARER_TOKEN')}"

    else:
        login = http.post(
            f"{auth_url}/api/v1/auth/login",
            json={
                "clientId": _env("CBWEB3_CLIENT_ID"),
                "clientSecret": _env("CBWEB3_CLIENT_SECRET"),
            },
            timeout=timeout,
        )
        if login.status_code != 200:
            pytest.fail(
                f"POST /api/v1/auth/login -> {login.status_code}: {login.text[:400]}"
            )
        body = login.json()

        if auth_mode == "direct":
            if "nonce" in body and "accessToken" not in body:
                pytest.fail(
                    "This participant was routed to the PKI nonce flow "
                    "(the 200 body carries `nonce`, no tokens and no cookies). "
                    "Re-run with CBWEB3_AUTH_MODE=pki."
                )
            # Cookies were installed by Set-Cookie; capture the refresh token
            # from the BODY because the refresh_token cookie is HttpOnly and
            # POST /api/v1/auth/refresh reads the token from the JSON body.
            http.cbweb3_refresh_token = body.get("refreshToken")
        else:  # pki
            if "nonce" not in body:
                pytest.fail(
                    "Expected the PKI nonce flow: a 200 carrying {'nonce': '<hex>'} "
                    "and no cookies. Got: " + str(sorted(body))
                )
            bind = http.post(
                f"{auth_url}/api/v1/auth/wallet/bind",
                json={
                    "user_id": _env("CBWEB3_USER_ID"),
                    "nonce_signature_hex": sign_nonce(
                        _read_pem("CBWEB3_KEY_PEM"), body["nonce"]
                    ),
                    "cert_pem": _read_pem("CBWEB3_CERT_PEM"),
                },
                timeout=timeout,
            )
            if bind.status_code != 200:
                pytest.fail(
                    f"POST /api/v1/auth/wallet/bind -> {bind.status_code}: "
                    f"{bind.text[:400]}"
                )
            http.cbweb3_refresh_token = bind.json().get("refreshToken")

        if "access_token" not in http.cookies.get_dict():
            pytest.fail(
                "The login flow completed with 200 but no `access_token` cookie "
                "reached the jar. Every protected operation will 401."
            )

    yield http
    http.close()


@pytest.fixture(scope="session")
def roles(session, auth_mode, auth_url, timeout):
    """
    The roles carried by the session, read from GET /api/v1/auth/me.

    `subject`, `issuer` and `roles` are the three required fields of MeResponse.
    Returns None in mock mode: Prism echoes a static example, so the value says
    nothing about the run.
    """
    if auth_mode == "mock":
        return None
    resp = session.get(f"{auth_url}/api/v1/auth/me", timeout=timeout)
    if resp.status_code != 200:
        pytest.fail(f"GET /api/v1/auth/me -> {resp.status_code}: {resp.text[:300]}")
    return resp.json().get("roles", [])


@pytest.fixture(scope="session")
def require_profile(profile):
    """
    Skip a test whose route the configured deployment does not register.

    Scenario A registers HTLC/Token/FX only when the payment orchestrator is
    wired; the escrow *approve* half only on Central Bank gateways
    (PaymentProxyHandler == nil) and the *create* half only on commercial-bank
    gateways; Scenario B registers each /api/v2 group only when its backing
    service is injected. A 404 there means "not this kind of node", not
    "non-conformant". The `mock` profile never skips: Prism serves every
    operation in the contract.
    """

    def _require(*allowed):
        if profile == "mock":
            return
        if profile not in allowed:
            pytest.skip(
                f"requires gateway profile {' or '.join(allowed)}; "
                f"CBWEB3_PROFILE={profile}"
            )

    return _require


@pytest.fixture(scope="session")
def require_role(roles):
    """Skip when the session does not carry one of the named roles."""

    def _require(*allowed):
        if roles is None:
            return  # mock mode: role is unknowable
        if not set(allowed) & set(roles):
            pytest.skip(
                f"session roles {roles} do not include any of {list(allowed)}"
            )

    return _require


@pytest.fixture(scope="session")
def expect_error(auth_mode):
    """
    Assert an error response.

    The status code is always asserted. The `{"error": "..."}` body is asserted
    only against a live gateway: Prism emits its own problem+json envelope for
    security failures, which is a property of the mock, not of the contract.

    The `error` STRING is never asserted on. The /api/v1 error model has no
    machine-readable code — `ErrorCodeResponse {error, code}` is declared in both
    platform specs and referenced by zero operations — and the same `error` field
    carries sometimes a human sentence and sometimes a code
    (INVALID_CERTIFICATE_CHAIN, NONCE_SIGNATURE_MISMATCH).
    """

    def _expect(resp, *allowed_status):
        assert resp.status_code in allowed_status, (
            f"{resp.request.method} {resp.request.url} -> {resp.status_code}, "
            f"expected one of {allowed_status}. Body: {resp.text[:400]}"
        )
        if auth_mode != "mock":
            body = resp.json()
            assert "error" in body, f"error body missing `error`: {body}"

    return _expect


@pytest.fixture(scope="session")
def skip_if_unavailable():
    """
    501 and 502/503 are first-class deployment outcomes, not conformance failures.

    501 = feature not configured in this deployment ("PKI authentication not
    configured", "Swap history is not available on this gateway").
    503 = capability disabled ("Fiat token adapter unavailable").
    502 = an upstream dependency (Central Bank API, compliance service, hub RPC)
    is unreachable — that is an estate problem, not a contract violation.
    """

    def _skip(resp):
        if resp.status_code in (501, 502, 503):
            pytest.skip(
                f"{resp.request.method} {resp.request.url} -> {resp.status_code}: "
                "feature not configured / capability disabled / upstream "
                "unreachable in this deployment"
            )

    return _skip


# ---------------------------------------------------------------------------
# Marker-driven collection rules
# ---------------------------------------------------------------------------
def pytest_collection_modifyitems(config, items):
    mode = config.getoption("--auth-mode") or os.environ.get("CBWEB3_AUTH_MODE", "mock")
    prof = config.getoption("--profile") or os.environ.get("CBWEB3_PROFILE", "mock")

    skip_live = pytest.mark.skip(
        reason="live_only: Prism is stateless and cannot serve a multi-step "
        "state transition (CBWEB3_AUTH_MODE=mock)"
    )
    skip_not_bearer = pytest.mark.skip(
        reason="CBWEB3_AUTH_MODE=bearer holds no cookie; only `bearer_ok` tests "
        "can run (BearerAuth is accepted on 12 Scenario B /api/v2 routes only)"
    )
    skip_scenario_a = pytest.mark.skip(
        reason="scenario_a test against a Scenario B profile: Scenario B has no "
        "FX agreement surface and no HTLC endpoints at all"
    )
    skip_scenario_b = pytest.mark.skip(
        reason="scenario_b test against a Scenario A profile: Scenario A has no "
        "AMM, bridge or hub surface"
    )

    for item in items:
        keywords = item.keywords
        if mode == "mock" and "live_only" in keywords:
            item.add_marker(skip_live)
        if mode == "bearer" and "bearer_ok" not in keywords:
            item.add_marker(skip_not_bearer)
        if prof.startswith("scenario-b") and "scenario_a" in keywords:
            item.add_marker(skip_scenario_a)
        if prof.startswith("scenario-a") and "scenario_b" in keywords:
            item.add_marker(skip_scenario_b)
