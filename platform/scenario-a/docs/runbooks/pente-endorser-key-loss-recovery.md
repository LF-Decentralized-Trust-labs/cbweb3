<!-- SPDX-License-Identifier: Apache-2.0 -->

# Recovering from Pente endorser key loss

For environments deployed **before** the Paladin key store was moved off `tmpfs`.
If your stack was provisioned from a tree that still carries
`- /data/keystore:mode=0777` in a Paladin compose, every Pente privacy group it
created is already dead or will die at the next container recreation, and the
repair is not a restart — the groups have to be recreated.

## Symptom

An FX propose fails and the payment-orchestrator logs:

```
rpc error: code = Internal desc = on-chain FX propose: pgroup transaction
<uuid> reverted: PD012214: Unable to decode revert data (no revert data available)
```

Everything upstream looks healthy: the private assembly succeeds, both members
endorse, the transition is dispatched. Only the public transaction on the group
contract reverts.

## Why it happens

The Paladin config declares two key stores
(`provisioning/templates/*/paladin-config/*/config.yaml.tmpl`):

| `keySelector` | `keyStore` | `keyDerivation` |
| --- | --- | --- |
| `^funded_operator$` | `static`, hex key inline in the config | `direct` |
| `.*` (`hd_wallet`) | `filesystem`, `/data/keystore` | `bip32` |

Pente endorsement identities are **salted with the group id** —
`funded_operator.<groupId>@<node>`. They do not match the anchored
`^funded_operator$` selector, so they fall through to `hd_wallet` and are derived
by bip32 from the seed in `/data/keystore`.

With that directory on `tmpfs` the seed lives in RAM. **Every container start
generates a new one** and all endorsement keys change with it. A plain
`docker restart` is enough — it does not take a recreation, which is where the
original post-mortem was too optimistic: it read `restarts=0` as evidence the
keys had been stable. Reproduced locally on 2026-09-03, `docker restart` alone
rewrote `-seed.key` (sha256 `43c7489e…` → `601303110…`) and the next FX propose
on the pre-existing group failed with `PD012214`.

The plain `funded_operator` identity keeps working throughout — it comes from the
static inline key — so nothing appears broken and no error is logged.

A Pente privacy group writes its endorser set to the base ledger when it is
created, and that set is **immutable**. After the recreation the contract rejects
every state transition with `PenteInvalidEndorser(address)`, and because the nodes
run without `--revert-reason-enabled` Paladin only sees an empty revert body and
reports `PD012214`.

Two consequences worth stating plainly:

- The gap between cause and symptom is unbounded. In the LNET incident the
  containers were recreated on 2026-08-17 and the first propose was on
  2026-09-03; the environment had been "up" for eleven days.
- No group created before the fix is recoverable. There is no way to update an
  endorser set on-chain, and the seed that would reproduce the old keys is gone.

## Repair

**The order matters.** Recreating the groups before the key store is persisted is
wasted work: step 2 recreates the containers, which regenerates the seed and
kills the groups you just made.

### 1. Persist the key store

Deploy from a tree that contains this fix. It has **two** parts, and the first
without the second does not boot:

1. No `/data/keystore` entry in any Paladin compose `tmpfs` block.
2. The data-init service `mkdir`s the key store directory on the volume
   (`mkdir -p /data/cb/keystore`, `/data/bank/keystore`).

Part 2 is easy to miss, and the way it fails is misleading. The tmpfs mount was
doing two jobs: making the path writable *and* creating it. Paladin's filesystem
key store does neither — drop the tmpfs alone and it exits with

```
PD010507: Initialization of embedded signer for wallet 'hd_wallet' failed:
PD020800: Path 'keystore' does not exist, or it is not a directory:
stat /data/keystore: no such file or directory
```

which surfaces five minutes later as `[FAIL] start-paladin: paladin health check
timed out after 5m0s`. The path it names is right there in `config.yaml`, so it
reads like a config error rather than a missing directory. This is not
hypothetical: it broke the verification deploy on 2026-09-03.

The init service also restores `0600` on the key store files after its recursive
`chmod 777` — `-seed.key` is private key material, and the recursive chmod now
reaches it because the key store lives inside that tree.

The guards for both parts are in `toolkit/engine/orchestrator`. Run them before
deploying:

```bash
cd scenario-a/toolkit && go test ./engine/orchestrator/ -run 'TestPaladinKeyStore|TestTmpfs|TestCreatesKeyStore'
```

### 2. Recreate the Paladin containers

A mount change does not apply to a running container.

```bash
docker compose -f <entity>/paladin-compose.yaml up -d --force-recreate paladin-cb
# commercial bank: --force-recreate paladin-bank
```

Do **not** delete the data volume: the indexed domain state (Zeto instances,
group index) lives there and is meant to survive.

### 3. Prove the seed now survives a restart

This is the check that tells you the fix took, and it is worth doing before
spending the group recreation. **Compare the seed file itself** across a restart:

```bash
docker exec <paladin> sha256sum /data/keystore/-seed.key
docker restart <paladin> && sleep 25
docker exec <paladin> sha256sum /data/keystore/-seed.key   # must be identical
```

`mount | grep keystore` inside the container is the other half: it must show
`/data/keystore` on ext4/overlay, not `tmpfs`.

**Do not use `ptx_resolveVerifier` for this.** It is the check the post-mortem
originally prescribed and it is a false green. Paladin caches the identity →
verifier mapping in its own database, which lives on the persistent `/data`
volume, so the resolve is a lookup and not a derivation. Measured on a broken
stack (2026-09-03): the seed sha256 went `43c7489e…` → `601303110…` across two
restarts while the salted identity kept resolving to `0xae3027f2…`. Signing
re-derives from the seed, which is why the endorsement is produced by a key that
no longer matches the on-chain endorser — the resolve never notices.

The plain `funded_operator` identity is useless here for a second, independent
reason: it is served by the static inline key and is stable even with the key
store in RAM. That is the stability the original incident hid behind.

### 4. Recreate the FX contexts

Once — and only once — step 3 passes. Both `create-pente-context` and
`deploy-fxa-pente` gate on keys in `<dataDir>/.deployed-addrs.env`
(`PENTE_CONTEXT_GROUP_ID` and `FX_AGREEMENT_DEPLOYED_AT` respectively), so
removing those lines is what makes `apply` plan them again:

```bash
cd scenario-a/samples
for f in cbweb3-data/*/.deployed-addrs.env; do
  sed -i -E '/^(PENTE_CONTEXT_GROUP_ID|PENTE_CONTEXT_ADDRESS|FX_PENTE_REGISTRY_ADDRESS|FX_AGREEMENT_ADDRESS|FX_AGREEMENT_DEPLOYED_AT)=/d' "$f"
done
```

Then re-run `apply` for each entity. `cbweb3 apply` has no per-step flag: it
plans what its `Check` reports unsatisfied, which after the edit above is exactly
the two Pente steps.

```bash
cbweb3 apply -f <manifest>.yaml --repo-root <repo>   # run from scenario-a/samples
```

`deploy-fxa-pente` overwrites `fx-contexts.json` in the
`${SPOKE_ID}_${BANK_ID}_fx_contexts` volume, so the backend picks up the new
group and in-group FXAgreement address without a manual edit.

Repeat for **every** group in the environment — each country's central bank and
each of its commercial banks, not just the entity that surfaced the error.

### 5. Verify

Propose an FX agreement end to end. There is no cheaper check: a group with a
stale endorser set is indistinguishable from a healthy one until a state
transition is attempted.

## Related

- `--revert-reason-enabled` is now set in the Besu entrypoints
  (`provisioning/templates/*/scripts/entry.sh`). On a node deployed before that,
  a reverting private transition still reports only `PD012214`; recover the real
  error by replaying the public transaction with `eth_call` at `latest` and
  decoding the 4-byte selector in the returned `data`.
- [container-log-disk-recovery.md](container-log-disk-recovery.md) — the other
  half of the same deploy's fallout.
