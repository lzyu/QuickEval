#!/usr/bin/env bash
set -euo pipefail

api_base="${QUICKEVAL_API_BASE:-http://127.0.0.1:8080}"
admin_username="${QUICKEVAL_SMOKE_ADMIN:-admin}"
: "${QUICKEVAL_SMOKE_PASSWORD:?QUICKEVAL_SMOKE_PASSWORD is required}"

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
smoke_dir="/private/tmp/quickeval-m4-smoke"
mkdir -p "${smoke_dir}"
timestamp="$(date +%s)"
admin_cookie="${smoke_dir}/admin.cookie"
member_a_cookie="${smoke_dir}/member-a.cookie"
member_b_cookie="${smoke_dir}/member-b.cookie"
member_password="MemberPass123!"
valid_image="${repo_dir}/page_refer/人工评测工作台.png"
invalid_image="${repo_dir}/scripts/fixtures/not-an-image.png"

request() {
  local method="$1"
  local path="$2"
  local cookie="$3"
  local csrf="$4"
  local input="${5:-}"
  local output="$6"
  local idempotency_key="${7:-}"
  local args=(-sS -o "${output}" -w "%{http_code}" -X "${method}" -b "${cookie}")
  [[ -n "${csrf}" ]] && args+=(-H "X-CSRF-Token: ${csrf}")
  [[ -n "${idempotency_key}" ]] && args+=(-H "Idempotency-Key: ${idempotency_key}")
  [[ -n "${input}" ]] && args+=(-H "Content-Type: application/json" --data-binary "@${input}")
  curl "${args[@]}" "${api_base}${path}"
}

upload() {
  local path="$1"
  local cookie="$2"
  local csrf="$3"
  local lock="$4"
  local image="$5"
  local output="$6"
  local key="$7"
  curl -sS -o "${output}" -w "%{http_code}" -X POST -b "${cookie}" \
    -H "X-CSRF-Token: ${csrf}" -H "Idempotency-Key: ${key}" \
    -F "expected_owner_lock_version=${lock}" -F "files=@${image}" \
    "${api_base}${path}"
}

login() {
  local username="$1"
  local password="$2"
  local cookie="$3"
  local prefix="$4"
  jq -n --arg username "${username}" --arg password "${password}" \
    '{username: $username, password: $password}' > "${prefix}-input.json"
  test "$(curl -sS -o "${prefix}.json" -w "%{http_code}" -c "${cookie}" \
    -H "Content-Type: application/json" --data-binary "@${prefix}-input.json" \
    "${api_base}/api/v1/auth/login")" = "200"
}

login "${admin_username}" "${QUICKEVAL_SMOKE_PASSWORD}" "${admin_cookie}" "${smoke_dir}/admin-session"
admin_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/admin-session.json")"

for suffix in a b; do
  jq -n --arg username "m4_member_${suffix}_${timestamp}" --arg password "${member_password}" \
    --arg name "M4 验收成员 ${suffix}" \
    '{username: $username, display_name: $name, email: null, role: "member", password: $password}' \
    > "${smoke_dir}/member-${suffix}-input.json"
  test "$(request POST /api/v1/users "${admin_cookie}" "${admin_csrf}" \
    "${smoke_dir}/member-${suffix}-input.json" "${smoke_dir}/member-${suffix}.json")" = "201"
done
member_a_username="$(jq -er '.data.username' "${smoke_dir}/member-a.json")"
member_b_username="$(jq -er '.data.username' "${smoke_dir}/member-b.json")"

jq -n --arg name "M4 评测对象 ${timestamp}" \
  '{name: $name, description: "M4 smoke"}' > "${smoke_dir}/target-input.json"
test "$(request POST /api/v1/evaluation-targets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/target-input.json" "${smoke_dir}/target.json")" = "201"
target_id="$(jq -er '.data.id' "${smoke_dir}/target.json")"

jq -n --arg id "${target_id}" --arg name "M4 截图场景 ${timestamp}" \
  '{evaluation_target_id: $id, name: $name, description: "M4 smoke"}' \
  > "${smoke_dir}/scenario-input.json"
test "$(request POST /api/v1/scenarios "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/scenario-input.json" "${smoke_dir}/scenario.json")" = "201"
scenario_id="$(jq -er '.data.id' "${smoke_dir}/scenario.json")"

jq -n --arg name "回答错误 ${timestamp}" \
  '{name: $name, description: "M4 smoke issue tag"}' > "${smoke_dir}/tag-input.json"
test "$(request POST /api/v1/issue-tags "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/tag-input.json" "${smoke_dir}/tag.json")" = "201"
tag_id="$(jq -er '.data.id' "${smoke_dir}/tag.json")"

jq -n --arg id "${scenario_id}" --arg name "M4 评测集 ${timestamp}" \
  '{scenario_id: $id, name: $name, description: "M4 smoke"}' > "${smoke_dir}/dataset-input.json"
test "$(request POST /api/v1/datasets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/dataset-input.json" "${smoke_dir}/dataset.json")" = "201"
draft_id="$(jq -er '.data.draft.id' "${smoke_dir}/dataset.json")"

jq -n '{name: "截图证据用例", user_prompt: "请给出采购建议", precondition: null,
  expected_result: null, judging_guide: "证据完整", is_enabled: true, tag_ids: []}' \
  > "${smoke_dir}/case-input.json"
test "$(request POST "/api/v1/dataset-versions/${draft_id}/cases" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/case-input.json" "${smoke_dir}/case.json")" = "201"
test "$(request GET "/api/v1/dataset-versions/${draft_id}" "${admin_cookie}" "" "" \
  "${smoke_dir}/draft.json")" = "200"
draft_lock="$(jq -er '.data.lock_version' "${smoke_dir}/draft.json")"
jq -n --argjson lock "${draft_lock}" \
  '{release_note: "M4 smoke", expected_lock_version: $lock}' > "${smoke_dir}/publish-input.json"
test "$(request POST "/api/v1/dataset-versions/${draft_id}/publish" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/publish-input.json" "${smoke_dir}/version.json")" = "200"

login "${member_a_username}" "${member_password}" "${member_a_cookie}" "${smoke_dir}/member-a-session"
member_a_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/member-a-session.json")"
login "${member_b_username}" "${member_password}" "${member_b_cookie}" "${smoke_dir}/member-b-session"
member_b_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/member-b-session.json")"

jq -n --arg version "${draft_id}" \
  '{dataset_version_id: $version, agent_version: "2026.07.28",
    environment: "staging", purpose_note: "M4 smoke", config_note: null}' \
  > "${smoke_dir}/run-input.json"
test "$(request POST /api/v1/evaluation-runs "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/run-input.json" "${smoke_dir}/run.json" "m4-run-${timestamp}")" = "201"
run_id="$(jq -er '.data.id' "${smoke_dir}/run.json")"
test "$(request GET "/api/v1/pages/evaluation-runs/${run_id}/workbench?page_size=100" \
  "${member_a_cookie}" "" "" "${smoke_dir}/workbench.json")" = "200"
result_id="$(jq -er '.data.results.items[0].id' "${smoke_dir}/workbench.json")"

test "$(upload "/api/v1/case-results/${result_id}/attachments" "${member_a_cookie}" \
  "${member_a_csrf}" 0 "${invalid_image}" "${smoke_dir}/invalid-upload.json" \
  "m4-invalid-${timestamp}")" = "415"

upload_key="m4-upload-${timestamp}"
test "$(upload "/api/v1/case-results/${result_id}/attachments" "${member_a_cookie}" \
  "${member_a_csrf}" 0 "${valid_image}" "${smoke_dir}/upload.json" "${upload_key}")" = "201"
attachment_id="$(jq -er '.data.items[0].id' "${smoke_dir}/upload.json")"
test "$(upload "/api/v1/case-results/${result_id}/attachments" "${member_a_cookie}" \
  "${member_a_csrf}" 0 "${valid_image}" "${smoke_dir}/upload-replay.json" "${upload_key}")" = "200"
jq -e --arg id "${attachment_id}" \
  '.data.items | length == 1 and .[0].id == $id' "${smoke_dir}/upload-replay.json" >/dev/null
test "$(request GET "/api/v1/attachments/${attachment_id}/content" \
  "${member_b_cookie}" "" "" "${smoke_dir}/member-b-before-badcase.json")" = "403"

jq -n '{status: "evaluated", answer_text: null, score: 2, comment: "截图展示回答错误",
  skip_reason: null, expected_lock_version: 1}' > "${smoke_dir}/evaluate-input.json"
test "$(request PATCH "/api/v1/case-results/${result_id}" "${member_a_cookie}" \
  "${member_a_csrf}" "${smoke_dir}/evaluate-input.json" "${smoke_dir}/evaluated.json")" = "200"
jq -e '.data.result.status == "evaluated" and .data.result.answer_text == null and
  (.data.result.attachments | length) == 1' "${smoke_dir}/evaluated.json" >/dev/null

test "$(request DELETE "/api/v1/attachments/${attachment_id}?expected_owner_lock_version=2" \
  "${member_a_cookie}" "${member_a_csrf}" "" "${smoke_dir}/delete-evidence.json")" = "409"
jq -e '.error.code == "EVIDENCE_REQUIRED"' "${smoke_dir}/delete-evidence.json" >/dev/null

jq -n --arg tag "${tag_id}" \
  '{expected_result_lock_version: 2,
    result_patch: {status: "evaluated", answer_text: null, score: 2, comment: null},
    badcase: {title: "Agent 给出错误采购建议", description: "截图中建议与约束冲突",
      issue_tag_ids: [$tag]}}' > "${smoke_dir}/mark-missing-comment.json"
test "$(request POST "/api/v1/case-results/${result_id}/mark-badcase" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/mark-missing-comment.json" \
  "${smoke_dir}/mark-missing-comment-response.json" "m4-mark-invalid-${timestamp}")" = "422"

jq -n --arg tag "${tag_id}" \
  '{expected_result_lock_version: 2,
    badcase: {title: "Agent 给出错误采购建议", description: "截图中建议与约束冲突",
      issue_tag_ids: [$tag]}}' > "${smoke_dir}/mark-input.json"
mark_key="m4-mark-${timestamp}"
test "$(request POST "/api/v1/case-results/${result_id}/mark-badcase" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/mark-input.json" \
  "${smoke_dir}/marked.json" "${mark_key}")" = "201"
badcase_id="$(jq -er '.data.badcase.id' "${smoke_dir}/marked.json")"
run_lock="$(jq -er '.data.run_lock_version' "${smoke_dir}/marked.json")"
jq -e '.data.result.has_badcase == true and .data.progress.badcase_count == 1 and
  (.data.badcase.issue_tags | length) == 1' "${smoke_dir}/marked.json" >/dev/null

test "$(request POST "/api/v1/case-results/${result_id}/mark-badcase" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/mark-input.json" \
  "${smoke_dir}/marked-replay.json" "${mark_key}")" = "200"
jq -e --arg id "${badcase_id}" '.data.badcase.id == $id' "${smoke_dir}/marked-replay.json" >/dev/null

test "$(request GET "/api/v1/badcases?issue_tag_id=${tag_id}" "${member_a_cookie}" "" "" \
  "${smoke_dir}/badcases.json")" = "200"
jq -e --arg id "${badcase_id}" '.data.items | any(.id == $id)' "${smoke_dir}/badcases.json" >/dev/null
test "$(request GET "/api/v1/pages/badcases/${badcase_id}" "${member_a_cookie}" "" "" \
  "${smoke_dir}/badcase-detail.json")" = "200"
jq -e --arg result "${result_id}" \
  '.data.evaluation.case_result_id == $result and (.data.attachments | length) == 1 and
   (.data.activities | length) == 1' "${smoke_dir}/badcase-detail.json" >/dev/null

test "$(request GET "/api/v1/attachments/${attachment_id}/content" \
  "${member_b_cookie}" "" "" "${smoke_dir}/member-b-content.png")" = "200"
test "$(curl -sS -o "${smoke_dir}/anonymous-content.json" -w "%{http_code}" \
  "${api_base}/api/v1/attachments/${attachment_id}/content")" = "401"
test "$(request DELETE "/api/v1/attachments/${attachment_id}?expected_owner_lock_version=3" \
  "${member_b_cookie}" "${member_b_csrf}" "" "${smoke_dir}/member-b-delete.json")" = "403"

jq -n --argjson lock "${run_lock}" '{expected_lock_version: $lock}' \
  > "${smoke_dir}/complete-input.json"
test "$(request POST "/api/v1/evaluation-runs/${run_id}/complete" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/complete-input.json" \
  "${smoke_dir}/completed.json")" = "200"
test "$(request GET "/api/v1/attachments/${attachment_id}/content" \
  "${member_a_cookie}" "" "" "${smoke_dir}/completed-content.png")" = "200"

storage_file="${repo_dir}/uploads/evaluations/${run_id}/${result_id}/${attachment_id}.png"
test -f "${storage_file}"
cmp -s "${valid_image}" "${storage_file}"

echo "M4 smoke passed: validated private screenshots, idempotency, screenshot-only evidence, deletion guard, atomic Badcase, traceability and authorization"
