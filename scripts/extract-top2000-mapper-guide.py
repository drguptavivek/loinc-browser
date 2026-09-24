#!/usr/bin/env python3
"""Extract per-term rows from LOINC's Mapper's Guide to Top 2000++ US Lab Tests (PDF) into CSV.

Usage: python scripts/extract-top2000-mapper-guide.py \
    data/common_codes/LOINC_1.6_Top2000CommonLabResultsUS.pdf data/common_codes/top2000_mapper_guide.csv

The PDF and CSV are LOINC-licensed content: keep them under data/, never in git. Needs pdfplumber.
Columns: loinc_num, long_common_name (as of 2017), class, rank (2017), example_ucum,
example_display, comment (LOINC's mapping note), system_adjusted.
"""
import csv, re, sys
import pdfplumber

src, out = sys.argv[1], sys.argv[2]
code_re = re.compile(r"^\d{1,6}-\d$")
fields = ["loinc_num", "long_common_name", "class", "rank", "example_ucum", "example_display", "comment", "system_adjusted"]
rows, cur, section = [], None, ""

def columns(words):
    # Column starts from the "B C E F G H I P" letter header; the row number sits left of B.
    head = {w["text"]: w["x0"] for w in words if abs(w["top"] - 45) < 4 and w["text"] in "BCEFGHIP"}
    if len(head) < 8:
        return None
    return [head["B"] - 22, head["C"] - 112, head["E"] - 26, head["F"] - 4, head["G"] - 20, head["H"] - 22, head["I"] - 82, head["P"] - 28]

with pdfplumber.open(src) as pdf:
    for page in pdf.pages:
        words = page.extract_words(x_tolerance=1)
        cols = columns(words)
        if not cols:
            continue
        lines = {}
        for w in words:
            if w["top"] < 90 or w["top"] > page.height - 50:
                continue
            lines.setdefault(round(w["top"]), []).append(w)
        for top in sorted(lines):
            ws = sorted(lines[top], key=lambda w: w["x0"])
            cells = [[] for _ in fields]
            rownum = False
            for w in ws:
                if w["x1"] <= cols[0] + 2 and w["text"].isdigit():
                    rownum = True
                    continue
                i = max(k for k in range(len(cols)) if w["x0"] >= cols[k] - 1) if w["x0"] >= cols[0] - 1 else 0
                cells[i].append(w["text"])
            text = [" ".join(c) for c in cells]
            if code_re.match(text[0]):
                cur = dict(zip(fields, text))
                rows.append(cur)
            elif text[0]:
                cur = None  # guidance prose or a section title starts in the code column
            elif cur is not None:  # a wrapped cell; the row number may sit on its last line
                for k, v in zip(fields, text):
                    if v and k not in ("loinc_num", "rank"):
                        cur[k] = (cur[k] + " " + v).strip()

# A long class name can push the right-aligned rank into the class column ("Chem 203").
for r in rows:
    m = re.match(r"^(.*\S)\s+(\d+)$", r["class"])
    if not r["rank"] and m:
        r["class"], r["rank"] = m.group(1), m.group(2)
# A few terms appear under two classes: keep one row, joining distinct comments.
merged = {}
for r in rows:
    if r["loinc_num"] not in merged:
        merged[r["loinc_num"]] = r
    elif r["comment"] and r["comment"] not in merged[r["loinc_num"]]["comment"]:
        m = merged[r["loinc_num"]]
        m["comment"] = (m["comment"] + " | " + r["comment"]).strip(" |")
rows = list(merged.values())

with open(out, "w", newline="") as f:
    w = csv.DictWriter(f, fieldnames=fields)
    w.writeheader()
    w.writerows(rows)
print(len(rows), "rows,", len({r["loinc_num"] for r in rows}), "unique;", sum(1 for r in rows if r["comment"]), "with comments;", sum(1 for r in rows if r["example_ucum"]), "with UCUM")
