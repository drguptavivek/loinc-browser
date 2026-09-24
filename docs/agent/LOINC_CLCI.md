# Common Lab Codes for India (CLCI)

## CLCI

Common Lab Codes for India (CLCI) is a national subset of LOINC for Indian laboratories, published by the National Resource Centre for EHR Standards (NRCeS) at C-DAC Pune. The release of 29 June 2026 has 1,473 lab tests, each with one "General Name" (the name Indian labs use, e.g. "Amylase, Blood") and one LOINC code from LOINC 2.82. All 1,473 codes are ACTIVE in 2.82 and no code has two names.

CLCI is lab-only: no radiology, documents, or clinical observations. The largest classes are chemistry (422), microbiology (351), allergy (150), drug/toxicology (107), hematology (92), and serology (80). For imaging use the radiology playbook filters (`radModality`, `radRegion`, `radContrast`, ...).

NRCeS calls the mapping "suggestive": a curated default for a test as Indian labs commonly name it, not a rule. The release file is copyright C-DAC (all rights reserved) and contains LOINC content, so it is never committed here; users download it themselves.

Source: [NRCeS national releases: Common Lab Codes for India](https://www.nrces.in/services/national-releases#lab_codes).

## CLCI In This App

When the CLCI CSV is in the data directory (`<data dir>/common-lab-codes-for-india-YYYYMMDD/common-lab-codes-for-india.csv`, the newest folder wins, or `<data dir>/common_codes/common-lab-codes-for-india.csv`, or the path in `LOINC_CLCI_CSV`), startup loads it and word search uses it four ways:

- **Prior.** Word-search relevance boosts CLCI codes by as much as the popularity boost for the most-used US terms. CLCI is India-curated and `COMMON_TEST_RANK` comes from US usage, so neither overrides the other.
- **Name match.** When the query shares at least 75% of its words with a General Name (shared words over all words of both, ignoring order, punctuation, and stop words), that name's codes come first on page one, still subject to every other filter, and the response lists them in `clciMatches`. "creatinine urine" matches "Creatinine, Urine"; "urine" alone does not. Names are not unique ("Urea Nitrogen (BUN), Urine" has three codes), so several codes can be pinned. This is how long names such as "Band form neutrophils per 100 white blood cells, Blood", which no term matches word for word, still find their code.
- **Filter.** `clci=true` (HTTP `/api/v1/terms/search`, `/api/search`, MCP `loinc_search_terms`) keeps only CLCI codes. Without the CSV loaded, `clci=true` returns 400.
- **Label.** Term results, term detail, and MCP candidates carry `clciName`, the CLCI General Name, so a reviewer sees the national default beside each pick.

Meaning search (`mode=semantic`) does not use the prior yet; `mode=hybrid` gets it through the word half.

C-DAC's own toolkit (CLNtk, Java/Lucene) does the same with a `GENERAL_NAME` search field and an `enableClci` filter. Source: [C-DAC Toolkit for LOINC](https://cdac.gov.in/index.aspx?id=hi_hs_medinfo_loinc_download).

## Blood Means Serum Or Plasma

Indian lab names say "Blood" for tests run on serum or plasma. Of the 758 CLCI names ending ", Blood", only 149 map to a LOINC blood term (`Bld`, `BldV`) and 5 to `Ser/Plas/Bld`; 578 map to `Ser`, `Plas`, or `Ser/Plas` and 26 to platelet-poor plasma (`PPP`). "Amylase, Blood" is 1798-8, Amylase in Serum or Plasma.

Word search therefore reads "blood" as `blood OR serum OR plasma` and ranks terms whose system is not blood 4 bm25 points lower. A true blood term wins a close race, while a serum or plasma term is still found when no blood term matches. Responses list the rewrite in `synonyms`.

Source: [NRCeS national releases: Common Lab Codes for India](https://www.nrces.in/services/national-releases#lab_codes), checked against LOINC 2.82.

## CLCI Evaluation

Each CLCI General Name was searched with `classType=lab` in word mode, counting the CLCI code at #1 / in the top 3 / in the top 10:

| Server | #1 | Top 3 | Top 10 |
|---|---|---|---|
| rc6 | 26% | 41% | 50% |
| rc6 + blood rule, CLCI not loaded | 38% | 64% | 85% |
| rc6 + blood rule + CLCI prior | 72% | 89% | 92% |
| rc6 + blood rule + CLCI prior + name match | 85% | 98% | 99% |

The last two rows are measured on the names CLCI itself curated, so they show the prior and name match working, not how well they generalize. For held-out numbers use a lab compendium that CLCI did not see (such as the external mapper's reviewed rows). Hybrid mode went from 30/52/77 (rc6) to 84/97/98 with all three. The five names still missed are not lab terms (height, body surface area, donor name), which `classType=lab` excludes.

## Updating CLCI

NRCeS updates CLCI with Indian lab practice and new LOINC releases. To update, download the new zip from the NRCeS page, extract it into the data directory as `common-lab-codes-for-india-YYYYMMDD/`, and restart; the newest folder is used. Then rerun the evaluation above. A CLCI code that a newer LOINC release deprecates is still boosted, but it stays hidden by default like any other deprecated term.

Source: [NRCeS national releases: Common Lab Codes for India](https://www.nrces.in/services/national-releases#lab_codes).
