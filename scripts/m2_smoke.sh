#!/usr/bin/env bash
set -euo pipefail

api_base="${QUICKEVAL_API_BASE:-http://127.0.0.1:8080}"
admin_username="${QUICKEVAL_SMOKE_ADMIN:-admin}"
: "${QUICKEVAL_SMOKE_PASSWORD:?QUICKEVAL_SMOKE_PASSWORD is required}"

smoke_dir="/private/tmp/quickeval-m2-smoke"
mkdir -p "${smoke_dir}"
cookie="${smoke_dir}/admin.cookie"
timestamp="$(date +%s)"

request() {
  local method="$1"
  local path="$2"
  local input="${3:-}"
  local output="$4"
  local status
  local args=(-sS -o "${output}" -w "%{http_code}" -X "${method}" -b "${cookie}")
  if [[ -n "${csrf:-}" ]]; then
    args+=(-H "X-CSRF-Token: ${csrf}")
  fi
  if [[ -n "${input}" ]]; then
    args+=(-H "Content-Type: application/json" --data-binary "@${input}")
  fi
  status="$(curl "${args[@]}" "${api_base}${path}")"
  printf "%s" "${status}"
}

jq -n --arg username "${admin_username}" --arg password "${QUICKEVAL_SMOKE_PASSWORD}" \
  '{username: $username, password: $password}' > "${smoke_dir}/login.json"
test "$(curl -sS -o "${smoke_dir}/session.json" -w "%{http_code}" -c "${cookie}" \
  -H "Content-Type: application/json" --data-binary "@${smoke_dir}/login.json" \
  "${api_base}/api/v1/auth/login")" = "200"
csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/session.json")"

jq -n --arg name "M2 评测对象 ${timestamp}" \
  '{name: $name, description: "M2 smoke"}' > "${smoke_dir}/target-input.json"
test "$(request POST /api/v1/evaluation-targets "${smoke_dir}/target-input.json" \
  "${smoke_dir}/target.json")" = "201"
target_id="$(jq -er '.data.id' "${smoke_dir}/target.json")"

jq -n --arg id "${target_id}" \
  '{evaluation_target_id: $id, name: "智能采购询价", description: "M2 smoke"}' \
  > "${smoke_dir}/scenario-input.json"
test "$(request POST /api/v1/scenarios "${smoke_dir}/scenario-input.json" \
  "${smoke_dir}/scenario.json")" = "201"
scenario_id="$(jq -er '.data.id' "${smoke_dir}/scenario.json")"

jq -n --arg name "事实准确性 ${timestamp}" \
  '{scope: "global", scenario_id: null, name: $name, description: "M2 smoke"}' \
  > "${smoke_dir}/tag-input.json"
test "$(request POST "/api/v1/case-tags" \
  "${smoke_dir}/tag-input.json" "${smoke_dir}/tag.json")" = "201"
tag_id="$(jq -er '.data.id' "${smoke_dir}/tag.json")"
tag_name="$(jq -er '.data.name' "${smoke_dir}/tag.json")"

jq -n --arg id "${target_id}" --arg name "M2 采购助手评测集 ${timestamp}" \
  '{evaluation_target_id: $id, name: $name, description: "M2 smoke"}' > "${smoke_dir}/dataset-input.json"
test "$(request POST /api/v1/datasets "${smoke_dir}/dataset-input.json" \
  "${smoke_dir}/dataset.json")" = "201"
dataset_id="$(jq -er '.data.dataset.id' "${smoke_dir}/dataset.json")"
draft_v1_id="$(jq -er '.data.draft.id' "${smoke_dir}/dataset.json")"

test "$(request GET "/api/v1/datasets?evaluation_target_id=${target_id}&page_size=100" "" \
  "${smoke_dir}/datasets.json")" = "200"
jq -e --arg id "${dataset_id}" --arg draft "${draft_v1_id}" \
  '.data.items | any(.id == $id and .draft_version_id == $draft and .draft_case_count == 0)' \
  "${smoke_dir}/datasets.json" >/dev/null

jq -n --arg scenario "${scenario_id}" --arg tag "${tag_id}" '{
  scenario_id: $scenario,
  name: "预算追问",
  user_prompt: "预算 10 万元，请推荐采购方案",
  precondition: null,
  expected_result: null,
  judging_guide: "不得虚构商品参数",
  is_enabled: true,
  tag_ids: [$tag]
}' > "${smoke_dir}/case-input.json"
test "$(request POST "/api/v1/dataset-versions/${draft_v1_id}/cases" \
  "${smoke_dir}/case-input.json" "${smoke_dir}/case.json")" = "201"
case_v1_id="$(jq -er '.data.id' "${smoke_dir}/case.json")"
case_key="$(jq -er '.data.case_key' "${smoke_dir}/case.json")"
test "$(request GET "/api/v1/datasets?scenario_id=${scenario_id}&page_size=100" "" \
  "${smoke_dir}/datasets-by-scenario.json")" = "200"
jq -e --arg id "${dataset_id}" '.data.items | any(.id == $id)' \
  "${smoke_dir}/datasets-by-scenario.json" >/dev/null

printf '\357\273\277用例名称,用户问题,前置条件,期望结果,评判要点,用例标签,是否启用\n错误行,,,,,,是\n' \
  > "${smoke_dir}/invalid.csv"
test "$(curl -sS -o "${smoke_dir}/invalid-preview.json" -w "%{http_code}" -b "${cookie}" \
  -H "X-CSRF-Token: ${csrf}" -F "file=@${smoke_dir}/invalid.csv;type=text/csv" \
  "${api_base}/api/v1/dataset-versions/${draft_v1_id}/case-imports/preview")" = "200"
jq -e '.data.has_errors and .data.error_row_count == 1' \
  "${smoke_dir}/invalid-preview.json" >/dev/null

printf '\357\273\277用例名称,用户问题,前置条件,期望结果,评判要点,用例标签,是否启用\n交付周期,\"预算包含逗号, 交付期多久\",已选择商品,,\"回答需说明\n预计交付周期\",%s,是\n' \
  "${tag_name}" > "${smoke_dir}/valid.csv"
test "$(curl -sS -o "${smoke_dir}/preview.json" -w "%{http_code}" -b "${cookie}" \
  -H "X-CSRF-Token: ${csrf}" -F "file=@${smoke_dir}/valid.csv;type=text/csv" \
  "${api_base}/api/v1/dataset-versions/${draft_v1_id}/case-imports/preview")" = "200"
jq -e '.data.has_errors == false and .data.valid_row_count == 1 and
  (.data.rows[0].judging_guide | contains("\n"))' "${smoke_dir}/preview.json" >/dev/null
import_token="$(jq -er '.data.import_token' "${smoke_dir}/preview.json")"
jq -n --arg token "${import_token}" '{import_token: $token}' > "${smoke_dir}/commit-input.json"
test "$(request POST "/api/v1/dataset-versions/${draft_v1_id}/case-imports/commit" \
  "${smoke_dir}/commit-input.json" "${smoke_dir}/commit.json")" = "201"
jq -e '.data.created_count == 1' "${smoke_dir}/commit.json" >/dev/null
test "$(request GET "/api/v1/dataset-versions/${draft_v1_id}/cases?page_size=100" "" \
  "${smoke_dir}/mixed-cases.json")" = "200"
jq -e --arg scenario "${scenario_id}" \
  '(.data.items | any(.scenario_id == $scenario and .scenario_assignment_status == "confirmed")) and
   (.data.items | any(.scenario_id == null and .scenario_assignment_status == "unclassified"))' \
  "${smoke_dir}/mixed-cases.json" >/dev/null
test "$(request POST "/api/v1/dataset-versions/${draft_v1_id}/case-imports/commit" \
  "${smoke_dir}/commit-input.json" "${smoke_dir}/commit-reused.json")" = "409"
jq -e '.error.code == "IMPORT_PREVIEW_EXPIRED"' "${smoke_dir}/commit-reused.json" >/dev/null

test "$(request GET "/api/v1/dataset-versions/${draft_v1_id}" "" \
  "${smoke_dir}/draft-v1.json")" = "200"
draft_v1_lock="$(jq -er '.data.lock_version' "${smoke_dir}/draft-v1.json")"
jq -n --argjson lock "${draft_v1_lock}" \
  '{release_note: "M2 smoke V1", expected_lock_version: $lock}' > "${smoke_dir}/publish-v1-input.json"
test "$(request POST "/api/v1/dataset-versions/${draft_v1_id}/publish" \
  "${smoke_dir}/publish-v1-input.json" "${smoke_dir}/v1.json")" = "200"
jq -e '.data.status == "published" and .data.version_no == 1 and .data.case_count == 2' \
  "${smoke_dir}/v1.json" >/dev/null

jq -n --arg id "${target_id}" \
  '{evaluation_target_id: $id, name: "不可切换场景", description: "M2 smoke"}' \
  > "${smoke_dir}/second-scenario-input.json"
test "$(request POST /api/v1/scenarios "${smoke_dir}/second-scenario-input.json" \
  "${smoke_dir}/second-scenario.json")" = "201"
second_scenario_id="$(jq -er '.data.id' "${smoke_dir}/second-scenario.json")"
test "$(request GET "/api/v1/scenarios/${second_scenario_id}/available-case-tags" "" \
  "${smoke_dir}/second-scenario-tags.json")" = "200"
jq -e --arg id "${tag_id}" '.data.global | any(.id == $id)' \
  "${smoke_dir}/second-scenario-tags.json" >/dev/null
test "$(request GET "/api/v1/datasets/${dataset_id}" "" "${smoke_dir}/detail-before-update.json")" = "200"
dataset_lock="$(jq -er '.data.dataset.lock_version' "${smoke_dir}/detail-before-update.json")"
jq -n --arg name "M2 第二评测对象 ${timestamp}" \
  '{name: $name, description: "M2 target switch"}' > "${smoke_dir}/second-target-input.json"
test "$(request POST /api/v1/evaluation-targets "${smoke_dir}/second-target-input.json" \
  "${smoke_dir}/second-target.json")" = "201"
second_target_id="$(jq -er '.data.id' "${smoke_dir}/second-target.json")"
jq -n --arg target "${second_target_id}" --arg name "禁止切换" --argjson lock "${dataset_lock}" \
  '{evaluation_target_id: $target, name: $name, description: null, expected_lock_version: $lock}' \
  > "${smoke_dir}/scenario-switch-input.json"
test "$(request PATCH "/api/v1/datasets/${dataset_id}" \
  "${smoke_dir}/scenario-switch-input.json" "${smoke_dir}/scenario-switch.json")" = "409"
jq -e '.error.code == "DATASET_TARGET_LOCKED"' "${smoke_dir}/scenario-switch.json" >/dev/null

jq '.expected_lock_version = 0 | .user_prompt = "不可修改"' \
  "${smoke_dir}/case-input.json" > "${smoke_dir}/immutable-input.json"
test "$(request PATCH "/api/v1/version-cases/${case_v1_id}" \
  "${smoke_dir}/immutable-input.json" "${smoke_dir}/immutable.json")" = "409"
jq -e '.error.code == "VERSION_IMMUTABLE"' "${smoke_dir}/immutable.json" >/dev/null

test "$(request GET "/api/v1/datasets/${dataset_id}" "" "${smoke_dir}/detail-v1.json")" = "200"
dataset_lock="$(jq -er '.data.dataset.lock_version' "${smoke_dir}/detail-v1.json")"
jq -n --arg base "${draft_v1_id}" --argjson lock "${dataset_lock}" \
  '{base_version_id: $base, expected_dataset_lock_version: $lock}' > "${smoke_dir}/draft-v2-input.json"
test "$(request POST "/api/v1/datasets/${dataset_id}/drafts" \
  "${smoke_dir}/draft-v2-input.json" "${smoke_dir}/draft-v2.json")" = "201"
draft_v2_id="$(jq -er '.data.id' "${smoke_dir}/draft-v2.json")"

test "$(request POST "/api/v1/datasets/${dataset_id}/drafts" \
  "${smoke_dir}/draft-v2-input.json" "${smoke_dir}/duplicate-draft.json")" = "409"
jq -e '.error.code == "LOCK_VERSION_CONFLICT" or .error.code == "DRAFT_EXISTS"' \
  "${smoke_dir}/duplicate-draft.json" >/dev/null

test "$(request GET "/api/v1/dataset-versions/${draft_v2_id}/cases?page_size=100" "" \
  "${smoke_dir}/v2-cases.json")" = "200"
jq -e --arg key "${case_key}" --arg id "${case_v1_id}" \
  '.data.items | any(.case_key == $key and .id != $id)' "${smoke_dir}/v2-cases.json" >/dev/null

draft_v2_lock="$(jq -er '.data.lock_version' "${smoke_dir}/draft-v2.json")"
jq -n --argjson lock "${draft_v2_lock}" \
  '{release_note: "M2 smoke V2", expected_lock_version: $lock}' > "${smoke_dir}/publish-v2-input.json"
test "$(request POST "/api/v1/dataset-versions/${draft_v2_id}/publish" \
  "${smoke_dir}/publish-v2-input.json" "${smoke_dir}/v2.json")" = "200"
jq -e '.data.status == "published" and .data.version_no == 2' "${smoke_dir}/v2.json" >/dev/null

test "$(curl -sS -o "${smoke_dir}/export.csv" -w "%{http_code}" -b "${cookie}" \
  "${api_base}/api/v1/dataset-versions/${draft_v2_id}/cases.csv")" = "200"
test "$(head -c 3 "${smoke_dir}/export.csv" | od -An -t x1 | tr -d ' ')" = "efbbbf"

test "$(request GET "/api/v1/datasets/${dataset_id}" "" "${smoke_dir}/detail-v2.json")" = "200"
dataset_lock="$(jq -er '.data.dataset.lock_version' "${smoke_dir}/detail-v2.json")"
jq -n --argjson lock "${dataset_lock}" \
  '{base_version_id: null, expected_dataset_lock_version: $lock}' > "${smoke_dir}/draft-delete-input.json"
test "$(request POST "/api/v1/datasets/${dataset_id}/drafts" \
  "${smoke_dir}/draft-delete-input.json" "${smoke_dir}/draft-delete.json")" = "201"
draft_delete_id="$(jq -er '.data.id' "${smoke_dir}/draft-delete.json")"
draft_delete_lock="$(jq -er '.data.lock_version' "${smoke_dir}/draft-delete.json")"
test "$(request DELETE "/api/v1/dataset-versions/${draft_delete_id}?expected_lock_version=${draft_delete_lock}" \
  "" "${smoke_dir}/draft-delete-response.json")" = "204"

test "$(request GET "/api/v1/datasets/${dataset_id}" "" "${smoke_dir}/detail-final.json")" = "200"
dataset_lock="$(jq -er '.data.dataset.lock_version' "${smoke_dir}/detail-final.json")"
jq -n --argjson lock "${dataset_lock}" '{expected_lock_version: $lock}' \
  > "${smoke_dir}/archive-input.json"
test "$(request POST "/api/v1/datasets/${dataset_id}/archive" \
  "${smoke_dir}/archive-input.json" "${smoke_dir}/archived.json")" = "200"
jq -e '.data.status == "archived"' "${smoke_dir}/archived.json" >/dev/null
archived_lock="$(jq -er '.data.lock_version' "${smoke_dir}/archived.json")"
jq -n --argjson lock "${archived_lock}" '{expected_lock_version: $lock}' \
  > "${smoke_dir}/restore-input.json"
test "$(request POST "/api/v1/datasets/${dataset_id}/restore" \
  "${smoke_dir}/restore-input.json" "${smoke_dir}/restored.json")" = "200"
jq -e '.data.status == "active"' "${smoke_dir}/restored.json" >/dev/null

echo "M2 smoke passed: dataset, draft uniqueness, cases, CSV preview/commit/export, snapshots, publish and archive"
