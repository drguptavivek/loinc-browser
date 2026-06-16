#!/usr/bin/env python3
"""Build a local CSV lookup from the public LOINC abbreviations page.

Source: https://loinc.org/kb/abbreviations/

The generated CSV is intended as a local artifact under data/. The repository
is code-only, so generated LOINC-derived data should remain outside git.
"""

from __future__ import annotations

import argparse
import csv
import re
from datetime import datetime, timezone
from pathlib import Path
from urllib.request import Request, urlopen

from bs4 import BeautifulSoup


SOURCE_URL = "https://loinc.org/kb/abbreviations/"
DEFAULT_OUTPUT = Path("data/loinc-abbreviations.csv")


def fetch_html(url: str) -> str:
    request = Request(url, headers={"User-Agent": "loinc-browser-abbreviation-builder/1.0"})
    with urlopen(request, timeout=30) as response:
        return response.read().decode("utf-8")


def source_updated(soup: BeautifulSoup) -> str:
    text = soup.get_text("\n", strip=True)
    match = re.search(r"This section last updated:\s*([0-9]{4}-[0-9]{2}-[0-9]{2})", text)
    if not match:
        return ""
    return match.group(1)


def parse_rows(html: str, source_url: str) -> list[dict[str, str]]:
    soup = BeautifulSoup(html, "html.parser")
    table = soup.find("table")
    if table is None:
        raise ValueError("could not find abbreviations table")

    updated = source_updated(soup)
    retrieved_at = datetime.now(timezone.utc).replace(microsecond=0).isoformat()
    rows: list[dict[str, str]] = []
    for tr in table.find_all("tr"):
        cells = [cell.get_text(" ", strip=True) for cell in tr.find_all(["th", "td"])]
        if not cells or cells[0] == "Abbreviation/Acronym":
            continue
        if len(cells) != 3:
            raise ValueError(f"expected 3 cells, got {len(cells)}: {cells!r}")
        abbreviation, meaning, areas = cells
        if not abbreviation:
            continue
        rows.append(
            {
                "abbreviation": abbreviation,
                "meaning": meaning,
                "areas": areas,
                "source_url": source_url,
                "source_updated": updated,
                "retrieved_at_utc": retrieved_at,
            }
        )

    if not rows:
        raise ValueError("abbreviations table produced no data rows")
    return rows


def write_csv(path: Path, rows: list[dict[str, str]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fields = ["abbreviation", "meaning", "areas", "source_url", "source_updated", "retrieved_at_utc"]
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)


def main() -> None:
    parser = argparse.ArgumentParser(description="Build a local LOINC abbreviations CSV lookup.")
    parser.add_argument("--source-url", default=SOURCE_URL, help="LOINC abbreviations source page")
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT, help="CSV output path")
    args = parser.parse_args()

    html = fetch_html(args.source_url)
    rows = parse_rows(html, args.source_url)
    write_csv(args.output, rows)
    print(f"wrote {len(rows)} abbreviations to {args.output}")


if __name__ == "__main__":
    main()
