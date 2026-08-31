# SPDX-License-Identifier: Apache-2.0

"""Build one PDF per scenario from the per-portal user manuals.

Each scenario's Markdown manuals are concatenated into a single document and
rendered with pandoc + xelatex. The script performs the rewrites a combined
document needs:

- per-manual "Table of Contents" sections are removed in favour of one global
  table of contents (their anchors are file-local and would collide);
- every heading gets an explicit, chapter-prefixed identifier;
- in-manual anchor links and cross-manual links (``./bank.md#41-dashboard``)
  are rewritten to the prefixed identifiers so they resolve inside the PDF;
- image paths are made absolute so the concatenated file can live anywhere.

Requirements: pandoc and a LaTeX distribution providing xelatex.

Usage (from the repository root or anywhere):

    python3 docs/user-manuals/tools/build-pdf.py [--scenario a|b] [--outdir DIR]
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
import tempfile
from datetime import date
from pathlib import Path

MANUALS_DIR = Path(__file__).resolve().parent.parent
IMG_DIR = MANUALS_DIR / "img"

# Order follows docs/user-manuals/README.md: portals first, walkthrough last.
CHAPTER_ORDER = [
    "bank",
    "governance",
    "treasury",
    "supervisor",
    "noc",
    "end-to-end-walkthrough",
]

SCENARIOS = {
    "a": {
        "dir": "scenario-a",
        "title": "CBWeb3 Platform — Scenario A User Manual",
        "name": "Scenario A — Enhanced Correspondent Banking",
        "subtitle": "Enhanced Correspondent Banking (dual-layer HTLC PvP settlement)",
        "output": "CBWeb3-Scenario-A-User-Manual.pdf",
    },
    "b": {
        "dir": "scenario-b",
        "title": "CBWeb3 Platform — Scenario B User Manual",
        "name": "Scenario B — International Hub",
        "subtitle": "International Hub (FXAgreement + AMM + Bridge, with relay and circuit breaker)",
        "output": "CBWeb3-Scenario-B-User-Manual.pdf",
    },
}

LATEX_HEADER = r"""
\usepackage{float}
\floatplacement{figure}{H}
\usepackage[export]{adjustbox}
\usepackage{fancyhdr}
\usepackage{longtable}
\pagestyle{fancy}
\fancyhf{}
\fancyhead[L]{\small\nouppercase{\leftmark}}
\fancyhead[R]{\small\thepage}
\renewcommand{\headrulewidth}{0.4pt}
\fancypagestyle{plain}{\fancyhf{}\fancyhead[R]{\small\thepage}\renewcommand{\headrulewidth}{0pt}}
% Frame screenshots so their edges are visible against the page.
\let\cbwOldIncludegraphics\includegraphics
\renewcommand{\includegraphics}[2][]{%
  \cbwOldIncludegraphics[#1,cfbox=black!25 0.4pt 0pt]{#2}}
% Keep long verbatim blocks from running into the margin.
\usepackage{fvextra}
\fvset{breaklines=true,breakanywhere=true}
% Glyphs the manuals use that Latin Modern does not provide.
\usepackage{amssymb}
\usepackage{newunicodechar}
\newunicodechar{≥}{\ensuremath{\geq}}
\newunicodechar{≤}{\ensuremath{\leq}}
\newunicodechar{⇄}{\ensuremath{\rightleftarrows}}
% Zero-width space: inserted by the build script into long inline code spans
% (API paths, identifiers) so they can break instead of running off the page.
\newunicodechar{​}{\allowbreak}
"""

ZERO_WIDTH_SPACE = "​"
CODE_SPAN = re.compile(r"`([^`\n]+)`")


def slugify(heading_text: str) -> str:
    """Reproduce the GitHub heading-anchor algorithm used by the manuals."""
    text = heading_text.strip()
    text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", text)  # links -> label
    text = text.replace("`", "").replace("*", "").replace("_", "")
    text = text.lower()
    text = re.sub(r"[^a-z0-9 \-]", "", text)
    return text.replace(" ", "-")


def breakable_code_spans(line: str, threshold: int = 14) -> str:
    """Let long inline code spans break after path separators.

    LaTeX cannot break inside ``\\texttt``, so a span such as
    ``GET /api/v1/compliance/zk-pointer/verify`` runs past the right margin.
    A zero-width space (mapped to ``\\allowbreak`` in the preamble) adds break
    opportunities without changing the rendered or copied text.
    """

    def rewrite(match: re.Match[str]) -> str:
        body = match.group(1)
        if len(body) <= threshold:
            return match.group(0)
        return "`" + re.sub(r"([/_])", r"\1" + ZERO_WIDTH_SPACE, body) + "`"

    return CODE_SPAN.sub(rewrite, line)


def strip_toc(lines: list[str]) -> list[str]:
    """Drop the "## Table of Contents" block, up to the next level-2 heading."""
    start = None
    for i, line in enumerate(lines):
        if start is None:
            if re.match(r"^##\s+Table of Contents\s*$", line):
                start = i
        elif line.startswith("## "):
            return lines[:start] + lines[i:]
    return lines if start is None else lines[:start]


class Chapter:
    """A single manual, prepared for inclusion in the combined document."""

    def __init__(self, key: str, path: Path):
        self.key = key
        self.path = path
        self.lines = strip_toc(path.read_text(encoding="utf-8").split("\n"))
        self.slug_map: dict[str, str] = {}
        # Lookup that tolerates the trailing/duplicated hyphens produced by
        # headings ending in punctuation (e.g. "Dashboard (`/`)" -> "dashboard-").
        # A few hand-written links in the manuals omit them.
        self.loose_map: dict[str, str] = {}
        self.top_id = ""
        self._collect_headings()

    def resolve(self, anchor: str) -> str | None:
        return self.slug_map.get(anchor) or self.loose_map.get(anchor.strip("-"))

    def _collect_headings(self) -> None:
        seen: dict[str, int] = {}
        in_fence = False
        for line in self.lines:
            if line.startswith("```"):
                in_fence = not in_fence
                continue
            if in_fence:
                continue
            match = re.match(r"^(#{1,6})\s+(.*?)\s*$", line)
            if not match:
                continue
            slug = slugify(match.group(2))
            count = seen.get(slug, 0)
            seen[slug] = count + 1
            unique = slug if count == 0 else f"{slug}-{count}"
            prefixed = f"{self.key}--{unique}"
            self.slug_map[unique] = prefixed
            self.loose_map.setdefault(unique.strip("-"), prefixed)
            if len(match.group(1)) == 1 and not self.top_id:
                self.top_id = prefixed

    def render(self, chapters: dict[str, "Chapter"], problems: list[str]) -> str:
        out: list[str] = []
        seen: dict[str, int] = {}
        in_fence = False
        for line in self.lines:
            if line.startswith("```"):
                in_fence = not in_fence
                out.append(line)
                continue
            if in_fence:
                out.append(line)
                continue
            heading = re.match(r"^(#{1,6})\s+(.*?)\s*$", line)
            if heading:
                slug = slugify(heading.group(2))
                count = seen.get(slug, 0)
                seen[slug] = count + 1
                unique = slug if count == 0 else f"{slug}-{count}"
                line = f"{heading.group(1)} {heading.group(2)} {{#{self.key}--{unique}}}"
                out.append(line)
                continue
            line = self._rewrite_links(line, chapters, problems)
            out.append(breakable_code_spans(line))
        return "\n".join(out)

    def _rewrite_links(
        self, line: str, chapters: dict[str, "Chapter"], problems: list[str]
    ) -> str:
        # Images: ../img/... -> absolute path.
        line = line.replace("](../img/", f"]({IMG_DIR}/")

        def same_file(match: re.Match[str]) -> str:
            anchor = match.group(1)
            target = self.resolve(anchor)
            if target is None:
                problems.append(f"{self.path.name}: unresolved anchor #{anchor}")
                return f"](#{self.top_id})"
            return f"](#{target})"

        line = re.sub(r"\]\(#([^)]+)\)", same_file, line)

        def cross_file(match: re.Match[str]) -> str:
            key, anchor = match.group(1), match.group(2)
            chapter = chapters.get(key)
            if chapter is None:
                problems.append(f"{self.path.name}: link to unknown manual {key}.md")
                return match.group(0)
            if anchor:
                target = chapter.resolve(anchor)
                if target is None:
                    problems.append(
                        f"{self.path.name}: unresolved anchor {key}.md#{anchor}"
                    )
                    return f"](#{chapter.top_id})"
                return f"](#{target})"
            return f"](#{chapter.top_id})"

        return re.sub(r"\]\(\./([A-Za-z0-9\-]+)\.md(?:#([^)]+))?\)", cross_file, line)


def readme_section(title_pattern: str) -> str:
    """Extract one level-2 section verbatim from the manuals README."""
    lines = (MANUALS_DIR / "README.md").read_text(encoding="utf-8").split("\n")
    start = None
    for i, line in enumerate(lines):
        if start is None:
            if re.match(rf"^##\s+{title_pattern}", line):
                start = i
        elif line.startswith("## ") or line.strip() == "---":
            return "\n".join(lines[start:i]).rstrip()
    return "\n".join(lines[start:]).rstrip() if start is not None else ""


def front_matter(scenario: dict, built_on: str) -> str:
    status = readme_section("Implementation status")
    conventions = readme_section("Conventions used in every manual")
    return "\n".join(
        [
            "# About this document {#about}",
            "",
            "This manual compiles the per-portal user manuals for "
            f"**{scenario['name']}** into a single document, in the order used "
            "by the portal index: Bank, Governance, Treasury, Supervisor, NOC, "
            "and finally the cross-portal end-to-end walkthrough.",
            "",
            "The chapters are the Markdown manuals kept in the repository under "
            "`docs/user-manuals/`. Each manual's own table of contents has been "
            "replaced by the single table of contents at the front of this "
            "document; the text is otherwise unchanged. Cross-references between "
            "manuals resolve to the corresponding chapter of this PDF.",
            "",
            f"Generated on {built_on}.",
            "",
            "---",
            "",
            status,
            "",
            "---",
            "",
            conventions,
            "",
        ]
    )


def build(scenario_key: str, outdir: Path, keep_source: bool) -> Path:
    scenario = SCENARIOS[scenario_key]
    scenario_dir = MANUALS_DIR / scenario["dir"]

    chapters: dict[str, Chapter] = {}
    for key in CHAPTER_ORDER:
        path = scenario_dir / f"{key}.md"
        if not path.exists():
            sys.exit(f"missing manual: {path}")
        chapters[key] = Chapter(key, path)

    built_on = date.today().isoformat()
    problems: list[str] = []
    parts = [front_matter(scenario, built_on)]
    parts += [chapters[key].render(chapters, problems) for key in CHAPTER_ORDER]
    body = "\n\n\\newpage\n\n".join(parts)

    for problem in sorted(set(problems)):
        print(f"warning: {problem}", file=sys.stderr)

    outdir.mkdir(parents=True, exist_ok=True)
    output = outdir / scenario["output"]

    with tempfile.TemporaryDirectory() as tmp:
        source = Path(tmp) / f"{scenario['dir']}-combined.md"
        source.write_text(body, encoding="utf-8")
        header = Path(tmp) / "header.tex"
        header.write_text(LATEX_HEADER, encoding="utf-8")
        if keep_source:
            kept = outdir / source.name
            kept.write_text(body, encoding="utf-8")
            print(f"combined markdown: {kept}")

        cmd = [
            "pandoc",
            str(source),
            "-o",
            str(output),
            # The manuals are written for GitHub, so match GFM behaviour where
            # pandoc's markdown reader differs: lists may interrupt a
            # paragraph, "- [ ]" is a checkbox, bare URLs are links, and
            # placeholders such as <Institution> are literal text, not HTML.
            "--from=markdown+lists_without_preceding_blankline+task_lists"
            "+autolink_bare_uris-raw_html",
            "--pdf-engine=xelatex",
            "--toc",
            "--toc-depth=3",
            "--top-level-division=chapter",
            f"--include-in-header={header}",
            "--metadata=title:" + scenario["title"],
            "--metadata=subtitle:" + scenario["subtitle"],
            "--metadata=date:" + built_on,
            "--metadata=lang:en",
            "-V",
            "documentclass=report",
            "-V",
            "fontsize=11pt",
            "-V",
            "geometry:a4paper,margin=2.2cm",
            "-V",
            "colorlinks=true",
            "-V",
            "linkcolor=RoyalBlue",
            "-V",
            "urlcolor=RoyalBlue",
            "-V",
            "toccolor=black",
            "-V",
            "graphics=true",
        ]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            sys.stderr.write(result.stdout)
            sys.stderr.write(result.stderr)
            sys.exit(f"pandoc failed for scenario {scenario_key}")
        if result.stderr.strip():
            sys.stderr.write(result.stderr)

    return output


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--scenario",
        choices=sorted(SCENARIOS),
        action="append",
        help="scenario to build (default: both)",
    )
    parser.add_argument(
        "--outdir",
        type=Path,
        default=MANUALS_DIR / "pdf",
        help="output directory (default: docs/user-manuals/pdf)",
    )
    parser.add_argument(
        "--keep-source",
        action="store_true",
        help="also write the combined Markdown next to the PDF",
    )
    args = parser.parse_args()

    for key in args.scenario or sorted(SCENARIOS):
        output = build(key, args.outdir.resolve(), args.keep_source)
        size_mb = output.stat().st_size / (1024 * 1024)
        print(f"wrote {output} ({size_mb:.1f} MB)")


if __name__ == "__main__":
    main()
