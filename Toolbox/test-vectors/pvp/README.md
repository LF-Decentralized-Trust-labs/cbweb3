# PvP Settlement — Test Vectors

## Purpose

Deterministic test fixtures for verifying compatibility with the PvP settlement contract. Each vector defines an **input**, **expected output**, and **preconditions** so any implementation can validate its behavior against the canonical spec.

## Structure

Each JSON file contains an array of test vectors with:

| Field | Description |
|-------|-------------|
| `id` | Unique test vector identifier |
| `title` | Short human-readable description |
| `description` | Detailed explanation of what is being tested |
| `category` | `happy-path`, `error`, `edge-case` |
| `preconditions` | State that MUST exist before executing the test |
| `input` | Request to send (method, path, body) |
| `expectedOutput` | Expected response (status, key fields in body) |
| `notes` | Implementation notes or rationale |

## Files

| File | Description |
|------|-------------|
| `pvp_fx_agreement_vectors.json` | FX Agreement creation and acceptance |
| `pvp_htlc_vectors.json` | HTLC lock, settle, refund, and error cases |

## How to validate

1. Set up your implementation against the PvP contract (`contracts/pvp/openapi_pvp_v0.1.0.yaml`)
2. For each test vector:
   - Ensure preconditions are met
   - Send the `input` request
   - Assert that the response matches `expectedOutput`
3. All vectors MUST pass for an implementation to be considered conformant

## Synthetic data

All values (addresses, amounts, hashes, secrets) are synthetic. The SHA-256 hash values used in HTLC vectors are real cryptographic hashes of the stated secrets, so implementations can verify hash correctness.
