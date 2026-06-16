# LOINC Special Cases

## Special Cases

LOINC special cases are mapping patterns where ordinary name-part matching is not enough. They often involve deciding whether a concept is a variable or a result value, whether a collection should be represented as a panel, whether the organism or antigen belongs in the Component or result value, and whether method or scale changes the clinical meaning.

Use this guide when candidate terms look plausible by name but differ in reporting model, scale, system, method, or result-value semantics.

Source: [LOINC Users' Guide, "3 - Special cases"](https://loinc.org/kb/users-guide/special-cases/).

## Binary Vs Multiple Answer

Some findings can be represented either as many binary observations or as one observation with multiple possible answers. In the binary-panel model, each finding is its own observation and the result is present/absent or yes/no. In the multiple-answer model, the observation names a broader variable and the positive findings are reported as values.

Mapping implication: binary findings usually use an ordinal Scale and presence-style Property. Multiple-choice or multiple-answer observations usually use a nominal Scale and a presence-or-identity style Property. Check the local reporting workflow before choosing.

Source: [LOINC Users' Guide, "3.1 Findings viewed as variables or as values"](https://loinc.org/kb/users-guide/special-cases/).

## Blood Bank

Blood bank content often needs to distinguish patient, donor, and blood product unit contexts. LOINC represents this with the System field and, when needed, a super-system such as donor or blood product unit. Additional details such as donor name or relationship should be transmitted in other message fields, not embedded in the LOINC term.

LOINC supports both binary panel-style reporting for individual antigens or antibodies and multiple-answer reporting for sets of antibodies or antigens identified or absent. Choose based on how the laboratory reports the result.

Source: [LOINC Users' Guide, "3.2 Blood bank"](https://loinc.org/kb/users-guide/special-cases/).

## Flow Cytometry

Flow cytometry terms commonly describe CD markers or combinations of markers. Single-marker Components generally imply marker-positive cells; negative signs become explicit when multiple markers are part of the cell phenotype. Two common result types are absolute cell number and percent/fraction of cells.

Mapping implication: distinguish number concentration from number fraction. When reporting a percentage, make sure the denominator or parent cell population is clear; newer terms may encode the parent population in the divisor, while other workflows may report the parent population separately.

Source: [LOINC Users' Guide, "3.3 Immunocompetence studies (flow cytometry)"](https://loinc.org/kb/users-guide/special-cases/).

## Microbiology

Microbiology culture reporting is often a chain of workflow steps, but LOINC terms are intended to identify the observation being reported. Result status belongs in message status fields, and detailed specimen collection descriptions usually belong in accompanying observations or comments.

For cultures, LOINC commonly identifies the observation as bacteria, virus, fungus, or organism identified by a culture method. Organism names are usually result values, not separate LOINC observation identifiers. SNOMED CT or another organism terminology is often the right code system for organism result values.

Presence/identity matters: use a presence-style Property and ordinal Scale for positive/negative organism tests, but use presence-or-identity and nominal Scale when the result can identify one or more alternative organisms.

Source: [LOINC Users' Guide, "3.4 Microbiology"](https://loinc.org/kb/users-guide/special-cases/).

## Antimicrobial Susceptibility

Antimicrobial susceptibility terms are named around the generic drug tested, susceptibility Property, isolate System, and the method when relevant, such as MIC, MLC, serum bactericidal titer, gradient strip, or agar diffusion. Methodless susceptibility codes also exist.

Breakpoints can depend on infection type or route in some organism-drug combinations. When a breakpoint-specific context is clinically meaningful, LOINC may encode that context in the System, such as isolate for suspected meningitis, rather than treating it as part of the drug Component.

Genotypic resistance testing is different from phenotypic susceptibility testing. Susceptibility testing measures whether growth is inhibited; resistance testing detects genes or mutations that predict resistance.

Source: [LOINC Users' Guide, "3.5 Antimicrobial susceptibilities"](https://loinc.org/kb/users-guide/special-cases/).

## Molecular Genetics

Molecular genetics terms should be as specific as needed about the analyte, target, gene, mutation, or variant. LOINC generally follows established gene and variant nomenclature and uses disease names only when the gene or defect is not sufficiently specified; disease names may still appear as related names for searching.

Specimen distinctions are often less important for molecular pathology than for ordinary lab tests; blood/tissue style Systems may be used when the result meaning is not specimen-specific. Method is often the broad molecular genetics method unless a technique produces meaningfully different results.

Narrative or document-level genetic reports are common but are less useful for automated analysis than discrete coded results. When structured reporting is possible, prefer discrete observations and answer values over bulk narrative.

Source: [LOINC Users' Guide, "3.9 Molecular genetics"](https://loinc.org/kb/users-guide/special-cases/).

## Allergy

Allergy Components are based on the biological source of the allergen. Formal Components use taxonomic names when applicable, while Long Common Names can use common names. For highly specified allergens, Components may distinguish native versus recombinant sources and individual antigen sequence numbers.

RAST class and similar categorized allergy results should not be treated like continuous quantitative measurements. If the result is an ordered class, check whether Scale and Property reflect ordinal or semi-quantitative reporting rather than raw concentration.

Source: [LOINC Users' Guide, "3.11 Allergy testing"](https://loinc.org/kb/users-guide/special-cases/).

## Urinalysis Strips

Urinalysis test strips may be interpreted as concentration-like ranges or as ordinal categories such as negative, 1+, 2+, and 3+. A facility usually adopts one interpretation path for a given strip result, and the local normal range or result format helps identify the intended mapping.

Mapping implication: concentration-style reporting should map to concentration/property terms, while plus-scale or negative/positive reporting should map to presence/threshold and ordinal Scale terms. Do not choose based only on the analyte name.

Source: [LOINC Users' Guide, "3.12 Urinalysis by Test Strips"](https://loinc.org/kb/users-guide/special-cases/).
