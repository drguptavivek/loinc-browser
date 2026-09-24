# LOINC Mapping Guidance

## Mapping Guidance

Practical rules for mapping local lab tests to LOINC, summarized from LOINC's *Mapper's Guide to Top 2000++ US Lab Tests* (v1.6, June 2017) and checked against the 2.82 release. Use it after search has produced candidates that look alike by name but differ in property, method, timing, or specimen: the rules below say which one a typical lab report means.

The guide predates 2.82: its ranks are older than the release's `COMMON_TEST_RANK`, some names have changed, and a few of its citations are wrong or gone (it gives 718-7 as a hematocrit; PF4 600-2 is not in 2.82). Codes below were checked against 2.82 (all ACTIVE). The guide's per-test example units and comments can be extracted locally with `scripts/extract-top2000-mapper-guide.py` into `data/common_codes/top2000_mapper_guide.csv` (licensed content; never commit it).

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests](https://loinc.org/usage/).

## Mapping Workflow

- Ask instrument/kit manufacturers and reference labs for their LOINC mappings first; large vendors and US referral labs have mapped their routine tests.
- Search against the common tests first (`rankedOnly=true`, or sort by usage), and work one lab section at a time.
- Build the work list from real result messages: order name, units, and sample values. Units narrow the Property (see `units_and_property`); volumes decide what to map first.
- Some analyzer outputs are internal flags (e.g. counter "suspect blast" indicators) that are never reported to clinicians; they need no LOINC code.
- Ranks of 3000 in the guide are LOINC's placeholder for important tests added without usage data, not a measured rank.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, General Guidance](https://loinc.org/usage/).

## Units And Property

The reporting unit is the strongest signal for the Property axis:

- Mass units (mg/dL, ng/mL) → `MCnc`; molar units (mmol/L) → `SCnc`. US labs mostly report mass; many other countries report molar, and LOINC has a separate code for each (calcium 17861-6 mass vs 2000-8 molar).
- Enzyme activity (U/L) → `CCnc` "Enzymatic activity"; the same enzyme reported in ng/mL is a mass term (CK-MB activity 32673-6 vs CK-MB mass 13969-1). Labs usually add "mass" to the name for the mass version.
- Arbitrary or international units per volume → `ACnc`.
- Ratios: mass/mass or mol/mol → `MCrto` / `SCrto`; mixed units (mg/mmol) → `Ratio`.
- Per 24 hours → a rate (`MRat` / `SRat`) with a 24H timing, not a concentration.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Chem and Timed Urine](https://loinc.org/usage/).

## Calculated Vs Measured

Several analytes have a calculated, a directly measured, and a method-less code; the name often doesn't say which.

- LDL cholesterol: calculated (Friedewald, from a lipid panel) 13457-7 is the common US LDL; direct 18262-6; unspecified 2089-1 only when you cannot tell. An LDL reported without total cholesterol, HDL, and triglycerides is almost certainly direct, whatever it is called.
- Anion gap: without potassium ("gap 3") 10466-1, the common US form; with potassium ("gap 4") 1863-0, about 3-5 mmol/L higher. Names rarely say which; use the lab's reference range (about 8-16 for gap 3, 10-20 for gap 4).

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Chem](https://loinc.org/usage/).

## Detection Limit Variants

Some tests have routine and high-sensitivity codes that differ only by a detection limit in the method:

- TSH: the guide says to map current assays to 11579-0 (detection limit <= 0.05 mIU/L, 2nd generation) or 11580-8 (<= 0.005 mIU/L, "high sensitivity", "ultrasensitive", "3rd generation"); the method-less 3016-3 is meant for old tests of unknown sensitivity. In 2.82, 3016-3 still ranks higher (100 vs 140), so many compendia still use it.
- PSA: routine 2857-1 for screening; high-sensitivity 35741-8 only for post-prostatectomy follow-up.
- Testosterone: routine 2986-8; high-sensitivity 49041-7 (detection limit <= 1.0 ng/dL) for expected very low levels (women, post-orchiectomy).
- Free T4: the method-less code (3024-7 mass, 14920-3 molar) is right in most cases; "by dialysis" (6892-4) is costlier and only for special cases, such as interfering proteins.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Chem](https://loinc.org/usage/).

## Pregnancy And HCG

Qualitative HCG or beta-HCG is a pregnancy test (serum 2118-8 HCG, 2110-5 beta-HCG; LOINC has matching urine codes). Quantitative HCG (19080-1) and beta-HCG (2111-3) serve other purposes such as ectopic pregnancy or miscarriage follow-up. HCG as a tumor marker has its own codes naming "tumor marker".

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Chem](https://loinc.org/usage/).

## Timed Urine

A urine analyte can map to three codes: a random (spot) concentration with `Pt` timing, a 24-hour concentration, and a 24-hour excretion rate. The 24-hour report usually carries both a concentration and a rate; a rate reported "per total volume" is best mapped as 24-hour, since its reference range almost always is. Analyte/creatinine ratios exist for both spot and timed urines; pick the Property from the units (see `units_and_property`). Some labs reuse one internal code for random and 24-hour concentrations, which must be split.

Microalbumin is a separate, more sensitive urine albumin test for early diabetic kidney damage, not the routine albumin; albumin excretion reported in both mg/24h and ug/min gets two codes.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Chem](https://loinc.org/usage/).

## Blood Gas Specimens

Hemoglobin and chemistries on blood gas panels have arterial (`BldA`), venous (`BldV`), and plain blood (`Bld`) codes. Apart from the gases and lactate, arterial and venous values barely differ; the split exists so a panel can show one specimen for all its tests. If gases are reported with plain `Bld`, the message must say elsewhere whether the sample was arterial or venous. Pulse oximetry measures arterial saturation, not capillary.

Ionized calcium is usually reported in molar units even in the US, in serum/plasma (1995-0) or whole blood from blood gas analyzers (1994-3).

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Chem-Bld Gas](https://loinc.org/usage/).

## Coagulation Specimens

Coagulation tests are measured on platelet-poor plasma (`PPP`) even though names rarely say so and lab manuals may just say "plasma" (PT 5902-2, INR 6301-6). Point-of-care versions use `Bld` and usually carry "POC" or "blood" in the local name.

A coagulation factor can be measured three ways, each with its own code: antigen amount ("Ag", immune method), clotting activity ("Coag" method, seconds, % of normal, or INR), and chromogenic activity ("Chrom" method). Antigen tests show how much protein there is, not whether it works. D-dimer must name its unit basis: FEU (48065-7) and DDU (48066-5) differ roughly two-fold, so avoid D-dimer codes that don't specify it.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Coagulation](https://loinc.org/usage/).

## Heparin And Lupus Anticoagulant

Heparin-induced thrombocytopenia has three test kinds: heparin-PF4 antibody immunoassay (34701-3, "heparin induced platelet Ab"), heparin platelet aggregation, and serotonin release. A local name "PF4" may mean either the PF4 protein (platelet activation, not HIT) or the heparin-PF4 antibody; check before mapping.

For lupus anticoagulant, LOINC's consensus panels are 75881-3 (aPTT, dRVVT and PT) and 75515-7 (aPTT and dRVVT); map local LAC tests to members of these panels where possible.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Coagulation](https://loinc.org/usage/).

## CBC And Differential

In the US, nearly all CBC results come from automated counters: map them to the "Automated count" method codes (platelets 777-3, hematocrit 4544-3). Hemoglobin is the exception: counters measure it by a standard chemistry method, so it uses the same code as a chemistry analyzer (718-7). Spun hematocrit is separate (4545-0).

The automated "big five" differential cells map to automated codes. Cells that only a person can count (blasts, variant lymphocytes) should map to the manual-method code, even though a method-less code also exists. Image-assisted smear readers still count manually. Some labs report differentials with method-less codes plus 49024-3 (differential method) to say how they were done. Morphology can be one identifier variable (5909-7 blood smear finding) or separate graded findings.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Heme](https://loinc.org/usage/).

## Drug Screen Vs Confirm

Drug/Tox has thousands of codes; exclude it when mapping routine chemistry. Drugs of abuse have separate screen and confirm codes per specimen (urine, serum, meconium, hair, ...), distinguished by "screen" and "confirm", not by technology. Screen names usually say "screen" or "scr". Confirmations can be quantitative or qualitative, with different codes.

Therapeutic drug levels often have peak, trough, and untimed ("random") codes (vancomycin 4090-7 peak, 4092-3 trough, 20578-1 random); map the local timing precisely.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Drug/Tox](https://loinc.org/usage/).

## Viral Load And STI Testing

Viral loads come as copies per volume (`NCnc`), international units per volume (`ACnc`), their log10 forms, and occasionally mass; each is a different code, so the unit decides. For chlamydia and gonorrhea, nucleic acid amplification (NAAT) is the recommended method; antigen and antibody codes for them are being discouraged.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Micro](https://loinc.org/usage/).

## Occult Blood

Stool occult blood has guaiac (older and high-sensitivity) and immunochemical (FIT) tests, often on two or three samples. LOINC has panels for both: guaiac 50196-5 and FIT 57803-9, each with per-specimen members plus specimen counts.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Chem-Occult Bld](https://loinc.org/usage/).

## Allergen Synonyms

Allergen names vary: dog dander, epithelium, and hair are one allergen (use 6098-8 dog dander where possible), as are cat fur, hair, and dander. Results come as IgE concentration (kIU/L), RAST class (1-6), or percent of a control, each its own code. Component-resolved allergens use acronyms: "n" (native) or "r" (recombinant), three letters of the genus, and the species initial, e.g. `rCan f 1`. Not all allergy tests are IgE; some labs measure IgG or IgA.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Allergy](https://loinc.org/usage/).

## Cell Marker Names

Labs name the same cells differently: "T4 cells", "CD4 count", "T-cell CD4+", and "CD3+CD4+ lymphocytes" all mean 24467-3 CD3+CD4+ (T4 helper) cells. LOINC names cells by marker pattern, with the cell type as a synonym. Immunocompetence panels gate on lymphocytes, so fractions are per 100 lymphocytes unless the Component names another denominator.

Source: [LOINC Mapper's Guide to Top 2000++ US Lab Tests, Cell markers](https://loinc.org/usage/).
