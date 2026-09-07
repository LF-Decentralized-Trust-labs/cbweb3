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
4. Every `{placeholder}` a fixture path carries is answered by an `input.pathParams` entry,
   and every `pathParams` entry answers a placeholder that is actually in the path.

Concrete identifiers are matched against templated segments, so
`/api/v1/payments/fx/agreements/aaeca49d-.../accept` resolves against
`/api/v1/payments/fx/agreements/{tradeId}/accept`. Query strings are ignored: they are
parameters, not paths.

HOW A TEMPLATED FIXTURE PATH IS HANDLED
---------------------------------------
`Toolbox/test-vectors/README.md` fixes the convention: `input.path` is the contract's
templated path *verbatim*, and the concrete values live in `input.pathParams`. So a vector
legitimately reads

    "path": "/api/v1/payments/fx/agreements/{tradeId}/accept",
    "pathParams": {"tradeId": "{{TRADE_ID}}"}

and this gate expands the one into the other before resolving. Three kinds of `pathParams`
value are recognised:

  * a `{{RUNTIME}}` substitution — an identifier the runner only learns at execution time,
    such as the `trade_id` a previous vector returned. It stands for exactly one path
    segment, so it is matched as one and its content is not inspected.
  * a literal that contains `/` — `pair` identifiers really do look like
    `W-tCeBM_BRL/W-tCeBM_ARS` (see the contract's own example). It is percent-encoded, as
    a client must encode it, and therefore still occupies a single segment.
  * any other literal — substituted as written.

A placeholder with NO `pathParams` entry is the copy-paste this gate exists to catch, and
is reported as exactly that rather than as a missing endpoint.

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
import urllib.parse

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

# A single `{name}` placeholder in a path. It cannot span a `/`, so a contract template
# never nests one, and `{{RUNTIME}}` markers only ever appear as pathParams *values*.
PLACEHOLDER = re.compile(r"\{([^/{}]+)\}")

# Stands in for a value the runner supplies at execution time. Any single non-empty
# segment would do; this one cannot collide with a real identifier.
RUNTIME_SEGMENT = "__runtime_substitution__"


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
        """Return every contract template the path resolves to, exact match first.

        The path must already be concrete: `expand_path` has substituted `pathParams` and
        rejected anything still templated. The brace guard below is the backstop for that,
        because a path still carrying `{placeholder}` would compare equal to the contract
        template it was copied from, and that equality is exactly what would let an
        unsubstituted copy-paste ship green.
        """
        if "{" in path or "}" in path:
            return []
        if path in self.templates:
            return [path]
        return [template for pattern, template in self.patterns if pattern.match(path)]


# ---------------------------------------------------------------------------
# Path expansion
# ---------------------------------------------------------------------------
def expand_path(path: str, path_params: dict[str, str]) -> tuple[str, list[str]]:
    """Substitute `pathParams` into a templated fixture path.

    Returns the concrete path and a list of problems. When there are problems the returned
    path is not worth resolving, so the caller reports and stops.
    """
    problems: list[str] = []
    used: set[str] = set()

    def substitute(match: re.Match[str]) -> str:
        name = match.group(1)
        used.add(name)
        if name not in path_params:
            problems.append(
                f"the path carries the placeholder {match.group(0)!r} but "
                f"input.pathParams declares no value for {name!r}. A fixture path is the "
                f"contract template verbatim; the concrete value belongs in pathParams."
            )
            return match.group(0)
        value = path_params[name]
        if not isinstance(value, str) or not value:
            problems.append(
                f"input.pathParams[{name!r}] is {value!r}; it must be a non-empty string."
            )
            return match.group(0)
        if "{" in value or "}" in value:
            # A {{RUNTIME}} substitution: one opaque segment, content not our business.
            return RUNTIME_SEGMENT
        # A literal. Percent-encode it exactly as a client must, so that a value which
        # legitimately contains "/" still occupies the single segment the template allows.
        return urllib.parse.quote(value, safe="")

    expanded = PLACEHOLDER.sub(substitute, path)

    for name in sorted(set(path_params) - used):
        problems.append(
            f"input.pathParams declares {name!r}, which the path does not contain. "
            f"Either the path or the parameter was renamed and the other was not."
        )

    return expanded, problems


# ---------------------------------------------------------------------------
# Fixture checking
# ---------------------------------------------------------------------------
def check(surface: Surface, source: str, label: str, method: str, path: str,
          failures: list[str], path_params: dict[str, str] | None = None) -> None:
    if not method or not path:
        failures.append(f"{source}: {label} has no method or no path")
        return

    method = method.upper()
    bare = path.split("?", 1)[0].split("#", 1)[0]

    if not bare.startswith("/"):
        failures.append(f"{source}: {label} path {path!r} is not absolute")
        return

    concrete, problems = expand_path(bare, path_params or {})
    if problems:
        for problem in problems:
            failures.append(f"{source}: {label} — {problem}\n        {method} {path}")
        return

    matches = surface.resolve(concrete)
    if not matches:
        expansion = "" if concrete == bare else f"        Expanded to {concrete!r}.\n"
        failures.append(
            f"{source}: {label} — no contract declares the path {bare!r}.\n"
            f"        {method} {path}\n"
            f"{expansion}"
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
                request.get("pathParams") or {},
            )
            vectors += 1

    print(f"Checked {mocks} mock fixture(s) and {vectors} test vector(s).")

    if failures:
        print(f"\n{len(failures)} artifact path(s) did not check out:\n")
        for failure in failures:
            print(f"  - {failure}")
        print(
            "\nEvery mock and vector must target an endpoint that actually exists on a "
            "CBWeb3 gateway.\nIf the endpoint is real, publish it in the contract first. "
            "Never invent one.\nIf the path is right but a placeholder went unanswered, "
            "give it a value in input.pathParams\nrather than writing the concrete "
            "identifier into input.path."
        )
        return 1

    print("OK — every mock and vector path resolves to a real contract path and method.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
