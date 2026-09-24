# Find Mode — Design Plan

Status: §2 and §3 implemented 2026-09-24 (Find, the term card, the basket, Map a list, `POST /api/v1/terms/match`). §4 ("My setup") and §5 steps 1 and 4 are deferred.

**Goal:** make people's job easier. A clinician, lab manager or HMIS/EHR implementer should get
from a local test name to the right LOINC code, copied into their system, in seconds. The
existing relationship/hierarchy explorer stays, as an **Advanced / Terminology** view for people
building or auditing LOINC content.

## 1. What LOINC is like, from a user's side

1. **Nobody browses LOINC top-down.** Users arrive with a local name ("S. Sodium", "HbA1c",
   "CT chest with contrast") and want the one right code. LOINC's formal trees (multiaxial
   hierarchy, component hierarchy) are organised around how terms are modelled, not how a lab
   or clinic thinks; the chemistry class does not line up with lab departments. Deep trees help
   terminologists, not end users.
2. **The hard part is choosing between near-duplicates, not finding a candidate.** "Glucose"
   returns hundreds of terms. What separates them is the 6 axes:
   - **System:** serum/plasma vs blood vs urine
   - **Property:** mass vs moles per volume
   - **Time:** a single point in time vs a 24-hour collection
   - **Scale:** quantitative vs ordinal
   - **Method:** named method vs no method

   A good browser makes those differences obvious and quick to choose between.
3. **Each domain needs different filters.** The 6 axes are the same everywhere; the useful
   filters are not:

   | Domain | Filters that matter | Source |
   |---|---|---|
   | Lab tests | specimen, property/units, scale, method, order vs observation (`ORDER_OBS`) | term axes |
   | Lab panels / orders | the panel's member tree, answer lists | `PanelsAndForms` |
   | Radiology | modality, body region, contrast, view | `LoincRsnaRadiologyPlaybook` |
   | Documents & clinical summaries | kind, setting, specialty, role, type of service | `DocumentOntology` |
   | Forms & assessments (PHQ-9 etc.) | the form's items and answer lists | `PanelsAndForms`, `AnswerFile` |
   | Vitals & measurements | small, curated common set | common-test rank |

## 2. A simple "Find" mode as the default

1. **Choose a domain first.** Lab tests · Lab panels/orders · Radiology · Documents & summaries ·
   Forms & assessments · Vitals & measurements. Each one sets the class/type filter and shows
   only the filters from §1.3 that fit it.
2. **One search box, ranked by real-world use.** Rank by `COMMON_TEST_RANK`,
   `COMMON_ORDER_RANK`, the user's common-code list (§4) and the Universal Lab Orders value set.
   Deprecated terms stay hidden by default, as they are today. Show the Long Common Name first,
   with the 6 axes as small chips underneath.
3. **Browse by what you already know, not by the formal tree.** Keep it to 2–3 shallow levels:
   lab = analyte → specimen → property; radiology = modality → body region. Build these from
   parts and the Radiology Playbook, not the multiaxial hierarchy.
4. **A term card with a copy menu:**
   - code only: `2951-2`
   - code | display name
   - FHIR `Coding` JSON (system `http://loinc.org`, code, display, version), also usable in
     ABDM profiles
   - HL7 v2 `CWE`: `2951-2^Sodium [Moles/volume] in Serum or Plasma^LN`
   - example UCUM units, plus the LOINC version
   - when a term is DEPRECATED or DISCOURAGED: a clear warning and a one-click jump to its
     `MAP_TO` replacement
   - the required LOINC licence notice
5. **"Similar terms" is a variants table, not a graph.** It lists terms with the same
   component, one row each, with the axes as columns and only the differing cells highlighted
   (LOINC Groups, the LG codes, formalise this). Below it: "panels that contain this term" and,
   for a panel, "its members". That covers most of what "related" means to a user.

## 3. Three jobs to make easy

1. **Find one code fast:** type a local name, get the right code at the top, copy it in one
   click. Keyboard first: type, press arrow keys, press Enter to copy.
2. **Map a whole test master:** an HMIS/LIS keeps a test master of hundreds to thousands of
   local names. The flow is: upload it → matches are suggested → results are grouped as
   *Auto-matched (confident)* / *Needs a look* / *No match* → the user reviews only the
   uncertain ones. Order items (CBC, LFT) show their member result codes, so mapping one order
   maps its parameters too.
3. **Put the codes into their system:** a persistent basket across searches; export as CSV in
   the user's own column order, a FHIR ConceptMap, or HL7 v2. Deprecated codes raise a warning,
   never a silent gap.

## 4. International: a "My setup" choice, picked once and remembered

| Setting | What it changes | Data |
|---|---|---|
| Common-code list | ranking prior, filter, local-name matching ("S. Creatinine", "LFT") | India: CLCI (already built, `internal/loinc/clci.go`). Everywhere: Universal Lab Orders value set and common-test ranks, both in the release. Other countries/hospitals: the user drops their own CSV (code + local name) into `data/common_codes/`; no third-party lists are shipped |
| Language | search and display in the user's language; copy still gives the official English name | `AccessoryFiles/LinguisticVariants` (23 locales, already read by the FHIR layer) |
| Units | flags when a local unit such as mg/dL points to a per-mole variant such as mmol/L | term property + example units |
| Target system | the default copy/export format (ABDM FHIR, FHIR, HL7 v2, HMIS CSV) | the copy menu in §2.4 |

## 5. Order of work

1. Generalise the CLCI loader to any number of labelled lists in `data/common_codes/*.csv`,
   and add the Universal Lab Orders list.
2. Find mode: domain entry, the term card with its copy menu, the variants table.
3. Bulk test-master mapping flow (check what `data/uploads` already supports first).
4. Language setting using the linguistic variants.
