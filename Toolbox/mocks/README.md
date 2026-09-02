# CBWeb3 Toolbox — Reference Mocks

Canonical request/response pairs for the two settlement flows the CBWeb3 platform actually
serves, as delivered in **CBWeb3 API Gateway v2.3.0**.

Each file is one step of one flow. Read a directory in filename order and you have the whole
story: what the client sent, what the gateway answered, which gateway answered it, and what the
next step does with the value it received.

| Directory | Contract | Scenario | Files |
|---|---|---|---|
| [`auth/`](auth/) | `contracts/auth/openapi_auth_v2.3.0.yaml` | Shared (A and B) | 5 |
| [`pvp/`](pvp/) | `contracts/pvp/openapi_pvp_v2.3.0.yaml` | A — single-ledger, FX agreement + dual-layer HTLC | 23 |
| [`amm/`](amm/) | `contracts/amm/openapi_amm_v2.3.0.yaml` | B — hub-and-spoke, bridge + Hub AMM | 31 |
| | | | **59** |

Every file validates against [`../schemas/mock.schema.json`](../schemas/mock.schema.json), and
every `request.path` resolves to a real path + method in one of the three contracts.

## Authentication in every fixture

Cookie, never bearer. Protected requests carry

```
Cookie: access_token=SYNTHETIC.NOT-A-REAL-TOKEN.cbweb3-toolbox-fixture
```

and nothing else — there is no `Authorization` header anywhere in this directory, and **no CSRF
header, because the platform implements none**. `SameSite=Strict` is its only cross-site
mitigation; the absence is recorded as a finding, not papered over. Start at [`auth/`](auth/) to
see how a real session is obtained. Public endpoints (`security: []`) carry no cookie at all —
in `amm/happy-path` the first five fixtures are deliberately credential-free.

## Every value here is synthetic, and reproducibly so

No fixture contains a real key, credential, token, certificate or transaction. Identifiers are
derived so that a reviewer can regenerate them and confirm nothing was copied from a live
system:

| Value | Derivation |
|---|---|
| Addresses (`0x` + 40 hex) | first 20 bytes of `SHA-256("cbweb3-toolbox-mock:addr:<label>")` |
| Transaction hashes (`0x` + 64 hex) | `SHA-256("cbweb3-toolbox-mock:tx:<label>")` |
| UUIDs | `SHA-256("cbweb3-toolbox-mock:uuid:<label>")`, first 16 bytes, v4 bits set |
| HTLC contract ids | `SHA-256("cbweb3-toolbox-mock:htlc:<label>")` — the delivered gateway likewise returns a 64-char SHA-256 hex, not the `htlc-a1b2c3d4` form its examples show |
| HTLC secret (pre-image) | `SHA-256("cbweb3-toolbox-mock:secret:<label>")` — a 32-byte value, hex-encoded, exactly as the gateway generates |
| Session cookie, tokens, signature, certificate | literal `SYNTHETIC…DO-NOT-USE` placeholders |

## The hash lock, and how to check it yourself

Earlier revisions of this kit published
`SHA-256("cbweb3-test-secret-2026") = 0x7f83b165…9069` and told implementers to verify it.
**That claim was false twice over**, and both errors are corrected here:

1. The digest was simply wrong — the real one is `000cd63c…fdcf`, and `7f83b165…` could not be
   identified as any hash of any obvious candidate.
2. More seriously, the platform does not hash the ASCII of a secret at all. Verified in the
   delivered payment orchestrator (`SettleHTLC`): the `secret` field is **hex-decoded first**,
   and the hash lock is `SHA-256` over the resulting **32 bytes**. A secret that is not valid hex
   is rejected with `400 invalid secret hex`, so a plain-language pass-phrase can never be used.

The pair used throughout `pvp/` therefore is:

```
secret    d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c
hash_lock 93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2

$ printf 'd2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c' \
  | xxd -r -p | sha256sum
93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2  -
```

Note the hash lock is **bare 64-character hex with no `0x` prefix**. The responder endpoint
hex-decodes it and rejects anything that is not exactly 32 bytes, so neither the `0x`-prefixed
example nor the 66-character example in the platform's own document can be sent literally.

## Where these fixtures follow the delivered gateway rather than the contract

In eleven places the platform's own OpenAPI document does not describe the gateway it ships
with. These fixtures follow the **gateway**, because a mock that teaches a shape the server
never sends is the exact failure this realignment exists to undo.

All eleven are catalogued once, with source citations, in
[`../DIVERGENCES.md`](../DIVERGENCES.md) (rows **A1**–**A11**); each is also repeated in the
`notes` of the fixture that shows it. Do not maintain a second copy here — that is how the
mocks and the vectors came to publish two different catalogues of the same facts.

As of the realignment, `contracts/pvp/openapi_pvp_v2.3.0.yaml` declares the delivered shape for
every one of the eleven, so **all fifty-nine fixtures validate against their contract's declared
response schema.** Previously three did not (rows A1, A2 and A4), because validating would have
required publishing a value the gateway never emits.

## Reading a fixture

```jsonc
{
  "id":          "mock-pvp-hp-11",     // matches ^mock-<domain>-<category>-<n>$
  "title":       "…",                  // the step, in one line
  "description": "…",                  // what it demonstrates
  "notes":       "…",                  // which gateway serves it, traps, verified divergences
  "request":  { "method", "path", "headers", "body" },
  "response": { "status", "headers", "body" }
}
```

`notes` is the load-bearing field. It always names **which gateway** the call goes to, because
no single CBWeb3 host serves a whole flow: the create half of the reserve lifecycle is
registered only on commercial-bank gateways and the approve half only on Central Bank gateways,
and a cross-border flow spans two spokes. A `404` from a conformance run frequently means
"this gateway is not that kind of node", not "non-conformant".

## What these mocks are not

* **Not a stateful server.** Prism serves the contracts, not these files, and is stateless: it
  cannot remember an agreement created by one request. Use these fixtures as the canonical
  reference for shapes and values; use a live gateway for state transitions.
* **Not a complete API surface.** They cover the settlement critical path plus the errors an
  integrator will actually meet. The contracts cover 88 paths / 99 operations; these fixtures exercise 38 distinct operations.
* **Not a substitute for the divergence tables** in each contract's README.
