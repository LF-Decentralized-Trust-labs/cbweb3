#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""
CI gate: every mock and test-vector path must resolve to a real path AND method in one of
the published interface contracts.

WHY THIS EXISTS
---------------
Before the 2026-08 realignment, nothing in CI cross-checked a fixture against the
specification it claimed to implement. The published PvP contract described seven endpoints
that no CBWeb3 gateway has ever served; every mock, vector and conformance test was built on
top of them, and the whole set shipped green for four sessions. Spectral validated the
contract's *syntax*, ajv validated the fixtures' *shape*, and nothing validated the one
relationship that mattered.

This gate closes that hole. It answers exactly one question, for every fixture:

    Does `<METHOD> <path>` exist on a gateway the Toolbox describes?

WHAT IT CHECKS
--------------
1. Every `Toolbox/mocks/**/*.json` `request.{method,path}` resolves.
2. Every vector in `Toolbox/test-vectors/*/*.json` has an `input.{method,path}` that
   resolves.
3. The path resolves to a contract path *template*, and that template declares the method.

Concrete identifiers are matched against templated segments, so
`/api/v1/payments/fx/agreements/aaeca49d-.../accept` resolves against
`/api/v1/payments/fx/agreements/{tradeId}/accept`. Query strings are ignored: they are
parameters, not paths.

A fixture may legitimately target any of the three contracts — a Scenario A mock resolves
against `pvp`, a discovery mock against `amm`, a login mock against `auth`. The union is the
surface the Toolbox publishes, so the union is what is checked.

Exit status: 0 if every fixture resolves, 1 otherwise.
"""

from __future__ import annotations

import glob
import json
import os
import re
import sys

try:
    import yaml
except ImportError:  # pragma: no cover
    sys.exit("PyYAML is required: pip install pyyaml")

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
CONTRACT_GLOB = os.path.join(REPO_ROOT, "Toolbox", "contracts", "*", "openapi_*.yaml")
MOCK_GLOB = os.path.join(REPO_ROOT, "Toolbox", "mocks", "**", "*.json")
VECTOR_GLOB = os.path.join(REPO_ROOT, "Toolbox", "test-vectors", "*", "*.json")

HTTP_METHODS = {
    "get", "put", "post", "delete", "options", "head", "patch", "trace",
}


def rel(path: str) -> str:
    return os.path.relpath(path, REPO_ROOT)


# ---------------------------------------------------------------------------
# Contract surface
# ---------------------------------------------------------------------------
class Surface:
    """The union of paths and methods published by the three contracts."""

    def __init__(self) -> None:
        # exact path template -> {METHOD: [contract names]}
        self.templates: dict[str, dict[str, list[str]]] = {}
        # compiled regex for templates that carry {parameters}
        self.patterns: list[tuple[re.Pattern[str], str]] = []
        self.contracts: list[str] = []

    def load(self, spec_path: str) -> None:
        with open(spec_path, encoding="utf-8") as handle:
            doc = yaml.safe_load(handle)
        name = os.path.basename(os.path.dirname(spec_path))
        self.contracts.append(name)
        for path, item in (doc.get("paths") or {}).items():
            if not isinstance(item, dict):
                continue
            methods = {
                method.upper()
                for method in item
                if method.lower() in HTTP_METHODS
            }
            if not methods:
                continue
            entry = self.templates.setdefault(path, {})
            for method in methods:
                entry.setdefault(method, []).append(name)
            if "{" in path:
                # A templated segment matches exactly one non-empty path segment.
                regex = "^" + re.sub(r"\{[^/}]+\}", r"[^/]+", re.escape(path)
                                     .replace(r"\{", "{").replace(r"\}", "}")
                                     .replace(r"\-", "-")) + "$"
                regex = re.sub(r"\{[^/}]+\}", r"[^/]+", regex)
                self.patterns.append((re.compile(regex), path))

    def resolve(self, path: str) -> list[str]:
        """Return every contract template the path resolves to, exact match first."""
        if path in self.templates:
            return [path]
        return [template for pattern, template in self.patterns if pattern.match(path)]


# ---------------------------------------------------------------------------
# Fixture checking
# ---------------------------------------------------------------------------
def check(surface: Surface, source: str, label: str, method: str, path: str,
          failures: list[str]) -> None:
    if not method or not path:
        failures.append(f"{source}: {label} has no method or no path")
        return

    method = method.upper()
    bare = path.split("?", 1)[0].split("#", 1)[0]

    if not bare.startswith("/"):
        failures.append(f"{source}: {label} path {path!r} is not absolute")
        return

    matches = surface.resolve(bare)
    if not matches:
        failures.append(
            f"{source}: {label} — no contract declares the path {bare!r}.\n"
            f"        {method} {path}\n"
            f"        Checked {len(surface.templates)} paths across "
            f"{', '.join(sorted(set(surface.contracts)))}."
        )
        return

    allowed: set[str] = set()
    for template in matches:
        allowed |= set(surface.templates[template])
    if method not in allowed:
        failures.append(
            f"{source}: {label} — the path exists but does not declare {method}.\n"
            f"        {method} {path}\n"
            f"        Resolved to {matches[0]!r}, which declares "
            f"{', '.join(sorted(allowed))}."
        )


def main() -> int:
    surface = Surface()
    specs = sorted(glob.glob(CONTRACT_GLOB))
    if not specs:
        print(f"No contracts found at {rel(CONTRACT_GLOB)} — the glob is stale.")
        return 1
    for spec in specs:
        surface.load(spec)

    print(
        f"Loaded {len(surface.templates)} path templates from {len(specs)} contract(s): "
        f"{', '.join(os.path.basename(s) for s in specs)}"
    )

    failures: list[str] = []
    mocks = 0
    vectors = 0

    for mock_path in sorted(glob.glob(MOCK_GLOB, recursive=True)):
        with open(mock_path, encoding="utf-8") as handle:
            try:
                doc = json.load(handle)
            except json.JSONDecodeError as exc:
                failures.append(f"{rel(mock_path)}: not valid JSON — {exc}")
                continue
        request = doc.get("request") or {}
        check(
            surface,
            rel(mock_path),
            f"mock {doc.get('id', '<no id>')}",
            request.get("method", ""),
            request.get("path", ""),
            failures,
        )
        mocks += 1

    for vector_path in sorted(glob.glob(VECTOR_GLOB)):
        if os.path.basename(vector_path).lower().startswith("readme"):
            continue
        with open(vector_path, encoding="utf-8") as handle:
            try:
                doc = json.load(handle)
            except json.JSONDecodeError as exc:
                failures.append(f"{rel(vector_path)}: not valid JSON — {exc}")
                continue
        for vector in doc.get("vectors") or []:
            request = vector.get("input") or {}
            check(
                surface,
                rel(vector_path),
                f"vector {vector.get('id', '<no id>')}",
                request.get("method", ""),
                request.get("path", ""),
                failures,
            )
            vectors += 1

    print(f"Checked {mocks} mock fixture(s) and {vectors} test vector(s).")

    if failures:
        print(f"\n{len(failures)} artifact path(s) do not resolve against any contract:\n")
        for failure in failures:
            print(f"  - {failure}")
        print(
            "\nEvery mock and vector must target an endpoint that actually exists on a "
            "CBWeb3 gateway.\nIf the endpoint is real, publish it in the contract first. "
            "Never invent one."
        )
        return 1

    print("OK — every mock and vector path resolves to a real contract path and method.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
