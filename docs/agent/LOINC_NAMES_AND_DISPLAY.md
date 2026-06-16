# LOINC Names And Display

## Names

LOINC publishes multiple human-readable names for a code. Use the code as the stable identifier, but choose the name according to context:

- Fully-Specified Name: the formal colon-separated name built from the major parts. Best for mapping, disambiguation, and understanding term structure.
- Long Common Name: a more readable display name. Often the best official LOINC name for data dictionaries, exchanges, implementation guides, and general human display when a local display name is not preferred.
- Short Name: a compact label intended for constrained displays such as report columns. It is not available for every term, can contain duplicates, and should not be used as a database key.

Use an official LOINC name with a LOINC code. Use a local name with a local code. In exchanges, sending both the local code/name and the LOINC code/name supports validation, debugging, and regulatory or operational traceability.

Sources: [LOINC Term Basics](https://loinc.org/get-started/loinc-term-basics/) and [Daniel J. Vreeman, "Which LOINC name should I use for what?"](https://danielvreeman.com/blog/2016/06/21/loinc-name-use/).

## Display Names

For clinical front ends, local interface names are often better than LOINC names because clinicians already know local conventions and because some formal LOINC dimensions are not natural display text. The Long Common Name is usually suitable when an official LOINC display is needed, especially for documents and technical specifications.

Do not put a local test name into a field that is supposed to contain an official LOINC name. Do not send only the Component as the LOINC display. If character limits prevent use of the Long Common Name, the Short Name is an acceptable fallback.

Source: [Daniel J. Vreeman, "Which LOINC name should I use for what?"](https://danielvreeman.com/blog/2016/06/21/loinc-name-use/).
