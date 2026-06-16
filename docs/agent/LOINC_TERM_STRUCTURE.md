# LOINC Term Structure

## Term Identity

A LOINC term is the combination of a unique, permanent LOINC code and its Fully-Specified Name. The code is the stable computer identifier and has no semantic structure except for its final mod-10 check digit. The term meaning is stored in the database fields and in the name parts, not in the numeric code itself. Prefer narrow lookups by LOINC number after search.

LOINC creates different codes when tests, measurements, or observations have clinically different meanings. Separate codes are usually warranted when results appear as separate reportable observations, have meaningfully different reference ranges, or differ in clinical interpretation.

Source: [LOINC Term Basics](https://loinc.org/get-started/loinc-term-basics/).

## Major Parts

The Fully-Specified Name is organized across five or six major parts separated by colons:

`<component>:<property>:<time>:<system>:<scale>:<method>`

The six dimensions are:

- Component: the analyte, substance, entity, question, document, or observation being measured or observed.
- Property: the attribute or kind of quantity, such as mass concentration, number concentration, presence, type, sequence, finding, or length.
- Time: the time aspect, such as a point in time or an interval.
- System: the specimen, body system, patient context, document subject, or other thing on which the observation was made.
- Scale: how the result is expressed, such as quantitative, semi-quantitative, ordinal, nominal, narrative, document, or other structured scale.
- Method: an optional high-level method class, included only when the method affects clinical interpretation, reference ranges, or other meaningful use.

Each active term should have valued primary attributes except Method, though panel and order-set terms may use dashes for attributes that do not apply cleanly.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Five Parts

The five required parts of a LOINC Fully-Specified Name are Component, Property, Time, System, and Scale. Method is an optional sixth part used only when the way the observation was made changes clinical interpretation, reference ranges, or other meaningful use.

- Component: what is measured or observed.
- Property: what kind of attribute is measured, such as concentration, count, presence, type, or length.
- Time: whether the observation is point-in-time or measured over an interval.
- System: the specimen, body system, patient context, or other thing observed.
- Scale: how the result is expressed, such as quantitative, ordinal, nominal, narrative, or document.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Abbreviations

LOINC uses abbreviations and acronyms across term parts, especially for timing, Property, System, Scale, Method, Class, challenge details, and related names. The public abbreviations list is a lookup aid, not a replacement for the structured term fields.

Common examples include timing abbreviations such as `Pt` for point in time and `24H` for 24 hours, system abbreviations such as `Ser/Plas`, `Urine`, `Bld`, and `CSF`, property abbreviations such as `MCnc`, `SCnc`, `NCnc`, `Find`, and `Len`, and scale abbreviations such as `Qn`, `Ord`, `Nom`, `Nar`, and `Doc`.

Do not expand abbreviations by string rules alone. Delimiters such as `/`, `+`, `.`, `^`, and `>` can carry part-specific meaning, and many combinations are intentionally not listed as standalone abbreviations. When precision matters, use term detail, part detail, and source fields rather than guessing from the display string.

Source: [LOINC Knowledge Base, "Abbreviations and acronyms used in LOINC"](https://loinc.org/kb/abbreviations/).

## Component

The Component is the first major part and names the principal analyte, measurement, question, document, or finding. It can include subparts for challenge or provocation and for adjustment or standardization. Component names generally avoid informal abbreviations, put the substance or entity first, prefer generic drug names over brand names, and use organism names rather than disease names for organism-specific tests.

For agents, Component alone is not enough to identify a term. The same component can have multiple clinically distinct LOINC terms once property, system, time, scale, or method differs.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Property

Property describes what kind of attribute is being measured or observed. It distinguishes clinically different quantities for the same component, such as mass concentration versus substance concentration, absolute count versus fraction, or presence/threshold versus type. Units can help infer Property, but Property is a semantic axis of the LOINC term and should not be guessed from a display name alone.

When mapping local terms, compare the local result units, result type, and clinical meaning against Property. For counts, concentrations, ratios, fractions, titers, and qualitative results, this axis often separates otherwise similar candidates.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Time

Time describes the interval over which the observation was made. `Pt` means a point or moment in time, while values like `24H` describe interval measurements. Time can include modifiers such as maximum, minimum, or mean when the observation is selected by that criterion.

Do not treat the default point-in-time value as unimportant during mapping. A spot urine result, a 24-hour urine result, and a timed maximum can be different observations even when the component and system are similar.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## System

System describes the specimen or thing observed, such as serum/plasma, urine, blood, cerebrospinal fluid, patient, abdomen, document, or another clinical context. The System part may include a super-system after a caret when the source is not the patient, such as donor, fetus, or blood product unit.

LOINC generally models specimen distinctions when conventional reporting treats them as clinically meaningful, for example serum sodium versus urine sodium. It does not try to encode every collection detail in the term name; those details should usually remain in source message fields.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Scale

Scale describes how the result value is expressed. Common values include `Qn` for continuous quantitative results, `SemiQn` for bucketed or range-based numeric results, `Ord` for ordered categories, `Nom` for unordered categories, `Nar` for narrative text, and `Doc` for document collections.

Scale affects form design, result validation, and answer-list handling. A nominal or ordinal term may need answer choices; a quantitative term needs unit and numeric handling; a document term identifies a collection of information independent of whether the payload is PDF, XML, text, image, or another format.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Method

Method is the sixth part and is optional. LOINC includes Method when the method type changes clinical interpretation, reference range, sensitivity, specificity, or other meaningful use. It is usually high level rather than instrument-specific.

For many chemistry and hematology tests, laboratories may use interchangeable instruments or techniques and LOINC intentionally avoids method distinctions. For immunochemistry, serology, microbiology, coagulation, and similar domains, Method is often more clinically meaningful and should be checked carefully.

Source: [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Status

Status matters for agent recommendations. Active terms are normally preferred. Deprecated, discouraged, and inactive terms require caution and should be selected only when the task explicitly needs them or when mapping legacy data.

Search results hide `STATUS=DEPRECATED` by default in this app. Selecting the `DEPRECATED` status facet should still allow explicitly browsing deprecated terms.

## Usage

`ORDER_OBS` describes whether a term is suitable as an order, an observation, or both. For form-builder work, match the intended workflow: observation terms for captured results, order terms for ordering workflows, and both only when appropriate.

## Rank

Common test and common order ranks are usage signals. Lower positive ranks are more common. Use `rankMode=observation` for result-capture workflows and `rankMode=order` for ordering workflows. `rankedOnly=true` narrows results to commonly used terms.

## Panels

Panels and forms are LOINC terms that contain authored child items. Use panel tools when building questionnaires, lab panels, order sets, batteries, or forms. Inspect panel items before assuming the parent term alone is enough.

Because individual panel elements often have different result scales, a panel or order-set term may use a dash for Scale or other parts that do not apply cleanly to the collection as a whole.

## Answer Lists

Answer lists define allowed answer choices for coded questions. A term may link to one or more answer lists, and a panel item may override the answer list. Inspect answer choices before using a term in a structured form.

LOINC Answer Codes identify qualitative or nominal result values and use `LA` identifiers. When including a LOINC Answer Code in a message or document, include the corresponding answer display text; sending local answer codes and names alongside standard answer codes is also useful.

## Hierarchy

Hierarchy browsing uses occurrence `nodeId` values. Do not use hierarchy concept codes as tree state because a concept can appear in multiple branches. Use node IDs for children, breadcrumbs, and subtree term calls.

## Parts

Parts are reusable pieces of LOINC term meaning, such as components, systems, methods, and other axes. Part searches help agents find related terms when direct term search is too broad. The Part File is also where active Property and Method values can be enumerated by part type.

## Groups

Groups collect related LOINC terms for clinical or domain-oriented browsing. Group membership can help compare candidates, but final selection should still use term detail and fit metadata.
