#!/usr/bin/env bash
# Capture golden response exemplars from the official LOINC services.
# Credentials come from loinc.env (username=..., password=...) and are never echoed.
# Output: docs/exemplars/fhir.loinc.org/*.json|.headers and docs/exemplars/searchapi/*.json|.headers
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_FILE="${LOINC_ENV_FILE:-loinc.env}"
user="$(sed -n 's/^username=//p' "$ENV_FILE" | head -1)"
pass="$(sed -n 's/^password=//p' "$ENV_FILE" | head -1)"
[ -n "$user" ] && [ -n "$pass" ] || { echo "missing username/password in $ENV_FILE" >&2; exit 1; }
netrc="$(mktemp)"; trap 'rm -f "$netrc"' EXIT
chmod 600 "$netrc"
printf 'machine fhir.loinc.org login %s password %s\nmachine loinc.regenstrief.org login %s password %s\n' "$user" "$pass" "$user" "$pass" >"$netrc"

FHIR="${LOINC_FHIR_BASE:-https://fhir.loinc.org}"
SEARCH="${LOINC_SEARCH_BASE:-https://loinc.regenstrief.org/searchapi}"

get() { # dir name url
  mkdir -p "docs/exemplars/$1"
  local code
  code="$(curl -sS --netrc-file "$netrc" -H 'Accept: application/fhir+json, application/json' \
    -D "docs/exemplars/$1/$2.headers" -o "docs/exemplars/$1/$2.json" -w '%{http_code}' "$3" || echo 000)"
  echo "$code $1/$2"
}
post() { # dir name url body
  mkdir -p "docs/exemplars/$1"
  local code
  code="$(curl -sS --netrc-file "$netrc" -H 'Accept: application/fhir+json' -H 'Content-Type: application/fhir+json' \
    -D "docs/exemplars/$1/$2.headers" -o "docs/exemplars/$1/$2.json" -w '%{http_code}' --data "$4" "$3" || echo 000)"
  echo "$code $1/$2"
}

F=fhir.loinc.org
get $F metadata "$FHIR/metadata"
get $F metadata-terminology "$FHIR/metadata?mode=terminology"
get $F codesystem-search-url "$FHIR/CodeSystem?url=http://loinc.org"
get $F codesystem-read-loinc "$FHIR/CodeSystem/loinc"
get $F codesystem-lookup-718-7 "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=718-7"
get $F codesystem-lookup-4544-3 "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=4544-3"
get $F codesystem-lookup-4544-3-props "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=4544-3&property=METHOD_TYP&property=VersionFirstReleased"
get $F codesystem-lookup-LP31755-9 "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=LP31755-9"
get $F codesystem-lookup-LP14542-2-parent "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=LP14542-2&property=parent"
get $F codesystem-lookup-LP31448-1-child "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=LP31448-1&property=child"
get $F codesystem-lookup-30064-0-parent "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=30064-0&property=parent"
get $F codesystem-lookup-LL1162-8 "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=LL1162-8"
get $F codesystem-lookup-LA6751-7 "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=LA6751-7"
get $F codesystem-lookup-718-7-de "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=718-7&displayLanguage=de-DE"
get $F codesystem-lookup-unknown "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=99999-9"
get $F codesystem-lookup-version-mismatch "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=99463-2&version=2.71"
get $F codesystem-validate-code-718-7 "$FHIR/CodeSystem/\$validate-code?url=http://loinc.org&code=718-7"
get $F codesystem-validate-code-718-7-baddisplay "$FHIR/CodeSystem/\$validate-code?url=http://loinc.org&code=718-7&display=Wrong%20name"
get $F codesystem-validate-code-unknown "$FHIR/CodeSystem/\$validate-code?url=http://loinc.org&code=99999-9"
get $F codesystem-subsumes-LP31755-9_44022-2 "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=LP31755-9&codeB=44022-2"
get $F codesystem-subsumes-44022-2_LP31755-9 "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=44022-2&codeB=LP31755-9"
get $F codesystem-subsumes-718-7_718-7 "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=718-7&codeB=718-7"
get $F codesystem-subsumes-718-7_2345-7 "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=718-7&codeB=2345-7"
get $F valueset-read-LL1162-8 "$FHIR/ValueSet/LL1162-8"
get $F valueset-search-url-LL1162-8 "$FHIR/ValueSet?url=http://loinc.org/vs/LL1162-8"
get $F valueset-search-name-yes "$FHIR/ValueSet?name:in=Yes&_count=2"
get $F valueset-read-LG9568-9 "$FHIR/ValueSet/LG9568-9"
get $F valueset-read-loinc-top-ranked "$FHIR/ValueSet?url=http://loinc.org/vs/loinc-top-ranked"
get $F valueset-expand-LL1162-8 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LL1162-8"
get $F valueset-expand-LL1000-0 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LL1000-0"
get $F valueset-expand-LG9568-9-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LG9568-9&count=3"
get $F valueset-expand-loinc-vs-count2 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs&count=2"
get $F valueset-expand-loinc-vs-count2-offset2 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs&count=2&offset=2"
get $F valueset-expand-loinc-vs-filter "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs&filter=hemoglobin&count=3"
get $F valueset-expand-LP31755-9-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LP31755-9&count=3"
get $F valueset-expand-document-ontology-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/loinc-document-ontology&count=3"
get $F valueset-expand-deprecated-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/deprecated-loinc-terms&count=3"
get $F valueset-expand-top-lab-orders-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/top-lab-orders&count=3"
get $F valueset-expand-top-ranked-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/loinc-top-ranked&count=3"
get $F valueset-expand-rsna-playbook-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/loinc-rsna-radiology-playbook&count=3"
get $F valueset-expand-attachment-requests-count3 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/valid-hl7-attachment-requests&count=3"
get $F valueset-expand-unknown "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LL0000-0"
post $F valueset-expand-inline-component "$FHIR/ValueSet/\$expand" '{"resourceType":"Parameters","parameter":[{"name":"count","valueInteger":3},{"name":"valueSet","resource":{"resourceType":"ValueSet","status":"active","compose":{"include":[{"system":"http://loinc.org","filter":[{"property":"COMPONENT","op":"=","value":"LP14449-0"}]}]}}}]}'
get $F valueset-validate-code-LL1162-8 "$FHIR/ValueSet/LL1162-8/\$validate-code?system=http://loinc.org&code=LA15679-6"
get $F valueset-validate-code-LG9568-9 "$FHIR/ValueSet/LG9568-9/\$validate-code?system=http://loinc.org&code=6785-0"
get $F valueset-validate-code-LL1162-8-miss "$FHIR/ValueSet/LL1162-8/\$validate-code?system=http://loinc.org&code=718-7"
get $F conceptmap-search-loinc-to-ieee "$FHIR/ConceptMap?url=http://loinc.org/cm/loinc-to-ieee-11073-10101&_summary=true"
get $F conceptmap-translate-11556-8 "$FHIR/ConceptMap/\$translate?system=http://loinc.org&code=11556-8"
get $F conceptmap-translate-30657-1 "$FHIR/ConceptMap/\$translate?system=http://loinc.org&code=30657-1"
get $F conceptmap-translate-parts-snomed-LP100006-8 "$FHIR/ConceptMap/\$translate?url=http://loinc.org/cm/loinc-parts-to-snomed-ct&system=http://loinc.org&code=LP100006-8"
get $F conceptmap-translate-snomed-reverse "$FHIR/ConceptMap/\$translate?url=http://loinc.org/cm/snomed-ct-to-loinc-parts&system=http://snomed.info/sct&code=708299006"
get $F conceptmap-translate-none "$FHIR/ConceptMap/\$translate?system=http://loinc.org&code=718-7"
get $F questionnaire-89689-4 "$FHIR/Questionnaire/89689-4"
get $F questionnaire-search-url-89689-4 "$FHIR/Questionnaire?url=http://loinc.org/q/89689-4"

S=searchapi
get $S loincs-glucose-rows2 "$SEARCH/loincs?query=glucose&rows=2"
get $S loincs-glucose-rows2-offset2-sort "$SEARCH/loincs?query=glucose&rows=2&offset=2&sortorder=loinc_num"
get $S loincs-glucose-filtercounts "$SEARCH/loincs?query=glucose&rows=1&includefiltercounts=true"
get $S loincs-hemoglobin-lang15 "$SEARCH/loincs?query=718-7&rows=1&language=15"
get $S loincs-fielded "$SEARCH/loincs?query=Component:glucose%20System:bld&rows=2"
get $S loincs-nomatch "$SEARCH/loincs?query=zzqqxxnomatch&rows=2"
get $S parts-glucose-rows2 "$SEARCH/parts?query=glucose&rows=2"
get $S answerlists-yes-rows2 "$SEARCH/answerlists?query=yes&rows=2"
get $S groups-glucose-rows2 "$SEARCH/groups?query=glucose&rows=2"

# Behaviour probes (status of deprecated terms, subsumption over the hierarchy, paging)
get $F codesystem-lookup-deprecated-6796-7 "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=6796-7"
get $F codesystem-lookup-discouraged "$FHIR/CodeSystem/\$lookup?system=http://loinc.org&code=2571-8&property=STATUS"
get $F codesystem-validate-code-deprecated-6796-7 "$FHIR/CodeSystem/\$validate-code?url=http://loinc.org&code=6796-7"
get $F codesystem-subsumes-LP384441-4_30064-0 "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=LP384441-4&codeB=30064-0"
get $F codesystem-subsumes-30064-0_LP384441-4 "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=30064-0&codeB=LP384441-4"
get $F codesystem-subsumes-LP7846-1_LP14542-2 "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=LP7846-1&codeB=LP14542-2"
get $F codesystem-subsumes-unknown "$FHIR/CodeSystem/\$subsumes?system=http://loinc.org&codeA=99999-9&codeB=718-7"
get $F valueset-expand-LL1162-8-count2-offset1 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LL1162-8&count=2&offset=1"
get $F valueset-expand-LL1162-8-filter "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LL1162-8&filter=ra"
get $F valueset-expand-LG9568-9 "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LG9568-9"
get $F valueset-expand-LL1162-8-de "$FHIR/ValueSet/\$expand?url=http://loinc.org/vs/LL1162-8&displayLanguage=de-DE&includeDesignations=true"
get $F valueset-read-id-LG9568-9-search "$FHIR/ValueSet?url=http://loinc.org/vs/LG9568-9"
get $F valueset-search-url-deprecated "$FHIR/ValueSet?url=http://loinc.org/vs/deprecated-loinc-terms&_elements=url,name"
get $F conceptmap-search-url-loinc-to-ieee "$FHIR/ConceptMap?url=http://loinc.org/cm/loinc-to-ieee-11073-10101&_count=1"
get $F conceptmap-translate-reverse-ieee "$FHIR/ConceptMap/\$translate?url=http://loinc.org/cm/ieee-11073-10101-to-loinc&system=urn:iso:std:iso:11073:10101&code=160116"
get $F conceptmap-translate-reverse-flag "$FHIR/ConceptMap/\$translate?url=http://loinc.org/cm/loinc-to-ieee-11073-10101&system=urn:iso:std:iso:11073:10101&code=160116&reverse=true"
get $F conceptmap-translate-mapto-deprecated "$FHIR/ConceptMap/\$translate?system=http://loinc.org&code=1009-0"
