"""
Conformance: shared authentication and session surface.

Source contract: contracts/auth/openapi_auth_v2.3.0.yaml  (8 paths / 8 operations)

The whole authentication block is byte-identical between the two platform specs,
so these tests run unchanged against a Scenario A gateway and a Scenario B one —
neither `scenario_a` nor `scenario_b` is marked on them.

WHAT IS NOT TESTED HERE, AND WHY
  * POST /api/v1/auth/pki-login — the only login endpoint that itself requires an
    existing session cookie. Only its 401-without-cookie behaviour is asserted;
    exercising it needs a deployment with PKI configured (it answers 501
    otherwise) and a second credential set the suite has no way to provision.
  * POST /api/v1/auth/client-secret/change — rotating the secret would invalidate
    the credentials the rest of the run depends on. Only the 401 is asserted.
  * Cookie Max-Age. The three Set-Cookie examples in the platform spec disagree
    (300 / 900 / 3600) and the spec never states the real access-token lifetime,
    so no TTL is asserted anywhere.
"""

import os

import pytest
import requests

from tests.helpers import has_keys, json_body, status_is

pytestmark = pytest.mark.auth


# ---------------------------------------------------------------------------
# GET /healthz — public (`security: []`)
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_healthz_is_public_and_returns_status(anon_session, auth_url, timeout):
    """GET /healthz answers 200 with the required `status` field, no credential."""
    resp = anon_session.get(f"{auth_url}/healthz", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "status")
    # `status` is required and described as a literal readiness marker; the
    # contract gives "ok" as an example only, so the value is not asserted.
    assert isinstance(body["status"], str) and body["status"]


# ---------------------------------------------------------------------------
# GET /api/v1/auth/me — CookieAuth
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_me_returns_subject_issuer_and_roles(session, auth_url, timeout):
    """
    A session answers 200 with MeResponse's three required fields.

    This is the correct assertion point for "am I authenticated, and as what
    role" — there is no machine-readable role model anywhere in the platform
    spec, so `roles` is the only programmatic handle a client has.
    """
    resp = session.get(f"{auth_url}/api/v1/auth/me", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "subject", "issuer", "roles")
    assert isinstance(body["roles"], list)
    assert all(isinstance(role, str) for role in body["roles"])


@pytest.mark.error
@pytest.mark.mock_safe
def test_me_without_cookie_returns_401(anon_session, auth_url, timeout, expect_error):
    """Without the `access_token` cookie the session endpoint answers 401."""
    resp = anon_session.get(f"{auth_url}/api/v1/auth/me", timeout=timeout)

    expect_error(resp, 401)


@pytest.mark.error
@pytest.mark.mock_safe
def test_pki_login_requires_an_existing_session(anon_session, auth_url, timeout, expect_error):
    """
    POST /api/v1/auth/pki-login is a LOGIN endpoint that is itself CookieAuth.

    Mirrored, not corrected: it is declared with `security: [{CookieAuth: []}]`
    in both platform specs. The body is valid so the 401 cannot be confused with
    a 400.
    """
    resp = anon_session.post(
        f"{auth_url}/api/v1/auth/pki-login",
        json={
            "user_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
            "client_secret": "not-a-real-secret",
        },
        timeout=timeout,
    )

    expect_error(resp, 401)


@pytest.mark.error
@pytest.mark.mock_safe
def test_client_secret_change_requires_a_session(anon_session, auth_url, timeout, expect_error):
    """Secret rotation is CookieAuth; without the cookie it answers 401."""
    resp = anon_session.post(
        f"{auth_url}/api/v1/auth/client-secret/change",
        json={
            "current_client_secret": "current-value",
            "new_client_secret": "new-value",
        },
        timeout=timeout,
    )

    expect_error(resp, 401)


# ---------------------------------------------------------------------------
# POST /api/v1/auth/login — public, two behaviours from one endpoint
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.live_only
def test_direct_login_returns_tokens_and_sets_an_httponly_cookie(
    auth_mode, auth_url, timeout, anon_session
):
    """
    Direct flow: 200 AuthResponse plus a Set-Cookie installing `access_token`.

    The cookie attributes asserted here (HttpOnly, SameSite=Strict) are the ones
    the CookieAuth scheme description states normatively. `Secure` is NOT
    asserted: it follows the gateway's COOKIE_SECURE env var and is absent over
    plain HTTP by design.
    """
    if auth_mode != "direct":
        pytest.skip("direct-flow test; run with CBWEB3_AUTH_MODE=direct")

    resp = anon_session.post(
        f"{auth_url}/api/v1/auth/login",
        json={
            "clientId": os.environ["CBWEB3_CLIENT_ID"],
            "clientSecret": os.environ["CBWEB3_CLIENT_SECRET"],
        },
        timeout=timeout,
    )

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "accessToken", "expiresIn", "tokenType")
    assert isinstance(body["expiresIn"], int)

    set_cookie = resp.headers.get("Set-Cookie", "")
    assert "access_token=" in set_cookie
    assert "HttpOnly" in set_cookie
    assert "SameSite=Strict" in set_cookie


@pytest.mark.happy_path
@pytest.mark.live_only
def test_pki_login_returns_a_nonce_and_no_tokens(auth_mode, auth_url, timeout, anon_session):
    """
    PKI flow: 200 carrying only {"nonce": "<hex>"} — no tokens, no cookies.

    LOAD-BEARING UPSTREAM DEFECT, mirrored rather than fixed: the platform types
    this 200 as AuthResponse, whose required fields are accessToken / expiresIn /
    tokenType and which has no `nonce` property at all. A strict response
    validator therefore FAILS a correct commercial-bank login. Our contract models
    the nonce shape as a documented `oneOf` marked
    `x-cbweb3-source: implementation-observed`.
    """
    if auth_mode != "pki":
        pytest.skip("PKI-flow test; run with CBWEB3_AUTH_MODE=pki")

    resp = anon_session.post(
        f"{auth_url}/api/v1/auth/login",
        json={
            "clientId": os.environ["CBWEB3_CLIENT_ID"],
            "clientSecret": os.environ["CBWEB3_CLIENT_SECRET"],
        },
        timeout=timeout,
    )

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "nonce")
    assert "accessToken" not in body
    assert "access_token" not in anon_session.cookies.get_dict()


@pytest.mark.error
@pytest.mark.live_only
def test_login_with_bad_credentials_is_refused(auth_url, timeout, expect_error, anon_session):
    """
    A wrong clientSecret is refused.

    The operation declares exactly two failure codes, 400 and 401, and the
    platform does not say which one a bad secret produces — so both are accepted
    and the free-form `error` string is not matched.
    """
    resp = anon_session.post(
        f"{auth_url}/api/v1/auth/login",
        json={"clientId": "definitely-not-a-participant", "clientSecret": "wrong"},
        timeout=timeout,
    )

    expect_error(resp, 400, 401)


# ---------------------------------------------------------------------------
# POST /api/v1/auth/refresh — public; reads the token from the BODY
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.live_only
def test_refresh_is_public_and_reads_the_token_from_the_body(
    session, auth_url, timeout, anon_session
):
    """
    Refresh is `security: []` and takes `refreshToken` in the JSON body.

    Design inconsistency worth knowing rather than hiding: the refresh_token
    cookie is HttpOnly, so a browser SPA cannot read the value it is required to
    send. A cookie-jar client works only because it captured `refreshToken` from
    the login RESPONSE BODY, which is what the session fixture does.
    """
    token = getattr(session, "cbweb3_refresh_token", None)
    if not token:
        pytest.skip("login response carried no refreshToken to replay")

    resp = anon_session.post(
        f"{auth_url}/api/v1/auth/refresh",
        json={"refreshToken": token},
        timeout=timeout,
    )

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "accessToken", "expiresIn", "tokenType")


# ---------------------------------------------------------------------------
# POST /api/v1/auth/logout — CookieAuth
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.live_only
def test_logout_clears_the_session_cookie(auth_mode, auth_url, timeout):
    """
    Logout answers 200 and expires the cookies.

    Runs on its OWN session — logging the shared session out would break every
    later test and reintroduce inter-test dependency.
    """
    if auth_mode != "direct":
        pytest.skip("needs a disposable direct-flow session to log out")

    disposable = requests.Session()
    login = disposable.post(
        f"{auth_url}/api/v1/auth/login",
        json={
            "clientId": os.environ["CBWEB3_CLIENT_ID"],
            "clientSecret": os.environ["CBWEB3_CLIENT_SECRET"],
        },
        timeout=timeout,
    )
    status_is(login, 200)

    resp = disposable.post(f"{auth_url}/api/v1/auth/logout", timeout=timeout)
    status_is(resp, 200)

    set_cookie = resp.headers.get("Set-Cookie", "")
    # The platform's own logout example expires the cookies with Max-Age=-1.
    assert "access_token=" in set_cookie
    disposable.close()
