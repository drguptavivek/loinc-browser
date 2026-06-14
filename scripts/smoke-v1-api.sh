#!/usr/bin/env bash
set -u

BASE_URL="${BASE_URL:-http://localhost:18080}"
DB="${DB:-./data/loinc-normalized.sqlite}"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 2
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required" >&2
  exit 2
fi

if [ ! -f "$DB" ]; then
  echo "database not found: $DB" >&2
  exit 2
fi

sql_one() {
  sqlite3 -noheader -batch "$DB" "$1" | head -n 1
}

term_active="$(sql_one "select loinc_num from loinc_terms where status = 'ACTIVE' limit 1")"
term_glucose="$(sql_one "select loinc_num from loinc_terms where long_common_name like '%glucose%' and status = 'ACTIVE' limit 1")"
term_deprecated="$(sql_one "select loinc_num from loinc_map_to limit 1")"
term_reported="$(sql_one "select loinc_num from loinc_terms where loinc_num = '2352-3' union all select '$term_glucose' limit 1")"
panel_parent="$(sql_one "select parent_loinc_num from panel_items limit 1")"
panel_child_null="$(sql_one "select child_loinc_num from panel_items where answer_list_id_override is null limit 1")"
answer_list="$(sql_one "select answer_list_id from answer_list_answers limit 1")"
part_number="$(sql_one "select part_number from loinc_part_links limit 1")"
group_id="$(sql_one "select group_id from group_loinc_terms limit 1")"
source_id="$(sql_one "select id from source_organizations limit 1")"
hier_root="$(sql_one "select node_id from hierarchy_occurrences where parent_node_id is null limit 1")"
hier_child="$(sql_one "select child_node_id from hierarchy_edges limit 1")"

fail=0
total=0

check() {
  local name="$1"
  local path="$2"
  local tmp
  local code
  local body

  total=$((total + 1))
  tmp="$(mktemp)"
  code="$(curl -sS -o "$tmp" -w "%{http_code}" "$BASE_URL$path" || printf "curl_failed")"
  body="$(tr "\n" " " < "$tmp" | cut -c1-180)"
  rm -f "$tmp"

  if [ "$code" != "200" ]; then
    fail=$((fail + 1))
    printf "FAIL %-38s %s %s\n" "$name" "$code" "$body"
  else
    printf "OK   %-38s %s %s\n" "$name" "$code" "$body"
  fi
}

check health "/api/v1/health"
check terms_search_glucose "/api/v1/terms/search?q=glucose&usageType=observation&rankMode=observation&sort=relevance&limit=2"
check terms_search_exact_active "/api/v1/terms/search?q=$term_active&limit=2"
check terms_search_status_all "/api/v1/terms/search?status=*&limit=2"
check terms_search_deprecated "/api/v1/terms/search?status=DEPRECATED&limit=2"
check terms_search_usage_order "/api/v1/terms/search?usageType=order&rankMode=order&sort=usage&rankedOnly=true&limit=2"
check terms_top_observation "/api/v1/terms/top?rankMode=observation&limit=2"
check terms_top_order "/api/v1/terms/top?rankMode=order&limit=2"
check term_detail_active "/api/v1/terms/$term_active"
check term_detail_glucose "/api/v1/terms/$term_glucose"
check term_detail_deprecated "/api/v1/terms/$term_deprecated"
check term_fit "/api/v1/terms/$term_reported/fit"
check term_relationships "/api/v1/terms/$term_reported/relationships"
check term_relationships_null_panel_child "/api/v1/terms/$panel_child_null/relationships"
check term_answer_lists "/api/v1/terms/$term_reported/answer-lists"
check term_panel_memberships "/api/v1/terms/$term_reported/panel-memberships"
check term_panel_memberships_null "/api/v1/terms/$panel_child_null/panel-memberships"
check term_copyright "/api/v1/terms/$term_reported/copyright"
check hierarchy_roots "/api/v1/hierarchy/roots"
check hierarchy_node_root "/api/v1/hierarchy/nodes/$hier_root"
check hierarchy_node_child "/api/v1/hierarchy/nodes/$hier_child"
check hierarchy_parents_child "/api/v1/hierarchy/nodes/$hier_child/parents"
check hierarchy_children_root "/api/v1/hierarchy/nodes/$hier_root/children"
check hierarchy_terms_root "/api/v1/hierarchy/nodes/$hier_root/terms?limit=2"
check panels_search "/api/v1/panels/search?q=glucose&limit=2"
check panel_detail "/api/v1/panels/$panel_parent"
check panel_items "/api/v1/panels/$panel_parent/items?limit=2"
check answer_lists_search "/api/v1/answer-lists/search?q=positive&limit=2"
check answer_list_detail "/api/v1/answer-lists/$answer_list"
check answer_list_answers "/api/v1/answer-lists/$answer_list/answers?limit=2"
check answer_list_terms "/api/v1/answer-lists/$answer_list/terms?limit=2"
check parts_search "/api/v1/parts/search?q=glucose&limit=2"
check part_detail "/api/v1/parts/$part_number"
check part_terms_all "/api/v1/parts/$part_number/terms?status=*&limit=2"
check part_terms_primary "/api/v1/parts/$part_number/terms?status=*&linkSet=primary&limit=2"
check groups_search "/api/v1/groups/search?limit=2"
check group_detail "/api/v1/groups/$group_id"
check group_terms "/api/v1/groups/$group_id/terms?status=*&limit=2"
check source_orgs "/api/v1/source-organizations"
check source_org_detail "/api/v1/source-organizations/$source_id"
check accessories_part "/api/v1/accessories?kind=part-primary&q=glucose&limit=2"
check accessories_answer "/api/v1/accessories?kind=answer-list&q=positive&limit=2"
check accessories_group "/api/v1/accessories?kind=group&limit=2"
check accessories_panel_membership "/api/v1/accessories?kind=panel-membership&limit=2"
check accessories_panel_child "/api/v1/accessories?kind=panel-child&limit=2"
check accessories_hierarchy "/api/v1/accessories?kind=hierarchy&q=glucose&limit=2"

printf "SUMMARY total=%d fail=%d\n" "$total" "$fail"

if [ "$fail" -ne 0 ]; then
  exit 1
fi
