#!/usr/bin/env bash
set -euo pipefail

api_base="${QUICKEVAL_API_BASE:-http://127.0.0.1:8080}"
admin_username="${QUICKEVAL_SMOKE_ADMIN:-admin}"
: "${QUICKEVAL_SMOKE_PASSWORD:?QUICKEVAL_SMOKE_PASSWORD is required}"

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
smoke_dir="/private/tmp/quickeval-m5-smoke"
mkdir -p "${smoke_dir}"
timestamp="$(date +%s)"
admin_cookie="${smoke_dir}/admin.cookie"
member_a_cookie="${smoke_dir}/member-a.cookie"
member_b_cookie="${smoke_dir}/member-b.cookie"
member_password="MemberPass123!"
valid_image="${repo_dir}/page_refer/登记业务 Badcase.png"

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
  jq -n --arg username "m5_member_${suffix}_${timestamp}" --arg password "${member_password}" \
    --arg name "M5 验收成员 ${suffix}" \
    '{username: $username, display_name: $name, email: null, role: "member", password: $password}' \
    > "${smoke_dir}/member-${suffix}-input.json"
  test "$(request POST /api/v1/users "${admin_cookie}" "${admin_csrf}" \
    "${smoke_dir}/member-${suffix}-input.json" "${smoke_dir}/member-${suffix}.json")" = "201"
done
member_a_username="$(jq -er '.data.username' "${smoke_dir}/member-a.json")"
member_b_username="$(jq -er '.data.username' "${smoke_dir}/member-b.json")"
member_b_id="$(jq -er '.data.id' "${smoke_dir}/member-b.json")"

jq -n --arg name "M5 智慧助手 ${timestamp}" \
  '{name: $name, description: "M5 smoke"}' > "${smoke_dir}/target-input.json"
test "$(request POST /api/v1/evaluation-targets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/target-input.json" "${smoke_dir}/target.json")" = "201"
target_id="$(jq -er '.data.id' "${smoke_dir}/target.json")"

jq -n --arg id "${target_id}" --arg name "M5 真实业务 ${timestamp}" \
  '{evaluation_target_id: $id, name: $name, description: "M5 smoke"}' \
  > "${smoke_dir}/scenario-input.json"
test "$(request POST /api/v1/scenarios "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/scenario-input.json" "${smoke_dir}/scenario.json")" = "201"
scenario_id="$(jq -er '.data.id' "${smoke_dir}/scenario.json")"

for suffix in accuracy constraint; do
  jq -n --arg name "M5 ${suffix} ${timestamp}" \
    '{name: $name, description: "M5 smoke"}' > "${smoke_dir}/tag-${suffix}-input.json"
  test "$(request POST /api/v1/issue-tags "${admin_cookie}" "${admin_csrf}" \
    "${smoke_dir}/tag-${suffix}-input.json" "${smoke_dir}/tag-${suffix}.json")" = "201"
done
tag_accuracy_id="$(jq -er '.data.id' "${smoke_dir}/tag-accuracy.json")"
tag_constraint_id="$(jq -er '.data.id' "${smoke_dir}/tag-constraint.json")"

login "${member_a_username}" "${member_password}" "${member_a_cookie}" "${smoke_dir}/member-a-session"
member_a_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/member-a-session.json")"
login "${member_b_username}" "${member_password}" "${member_b_cookie}" "${smoke_dir}/member-b-session"
member_b_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/member-b-session.json")"

occurred_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
jq -n --arg target "${target_id}" --arg scenario "${scenario_id}" --arg tag "${tag_accuracy_id}" \
  --arg occurred "${occurred_at}" \
  '{evaluation_target_id: $target, scenario_id: $scenario, title: "采购助手未识别预算条件",
    agent_response_text: "建议采购高配服务器", agent_version: "2026.07.28",
    environment: "production", occurred_at: $occurred,
    business_reference: "ORDER-M5-001", session_id: "chat-m5-001",
    issue_tag_ids: [$tag]}' > "${smoke_dir}/create-input.json"
create_key="m5-create-${timestamp}"
test "$(request POST /api/v1/badcases "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/create-input.json" "${smoke_dir}/created.json" "${create_key}")" = "201"
badcase_id="$(jq -er '.data.id' "${smoke_dir}/created.json")"
jq -e '.data.source_type == "business" and .data.status == "pending" and
  .data.description == null and .data.lock_version == 0 and (.data.activities | length) == 1' \
  "${smoke_dir}/created.json" >/dev/null

test "$(request POST /api/v1/badcases "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/create-input.json" "${smoke_dir}/create-replay.json" "${create_key}")" = "200"
jq -e --arg id "${badcase_id}" '.data.id == $id' "${smoke_dir}/create-replay.json" >/dev/null
jq '.title = "不同请求"' "${smoke_dir}/create-input.json" > "${smoke_dir}/create-reused-input.json"
test "$(request POST /api/v1/badcases "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/create-reused-input.json" "${smoke_dir}/create-reused.json" "${create_key}")" = "409"
jq -e '.error.code == "IDEMPOTENCY_KEY_REUSED"' "${smoke_dir}/create-reused.json" >/dev/null

upload_key="m5-upload-${timestamp}"
test "$(curl -sS -o "${smoke_dir}/uploaded.json" -w "%{http_code}" -X POST \
  -b "${member_a_cookie}" -H "X-CSRF-Token: ${member_a_csrf}" \
  -H "Idempotency-Key: ${upload_key}" -F "expected_owner_lock_version=0" \
  -F "files=@${valid_image}" -F "files=@${valid_image}" \
  "${api_base}/api/v1/badcases/${badcase_id}/attachments")" = "201"
first_attachment_id="$(jq -er '.data.items[0].id' "${smoke_dir}/uploaded.json")"

test "$(request GET "/api/v1/pages/badcases/${badcase_id}" "${member_a_cookie}" "" "" \
  "${smoke_dir}/detail.json")" = "200"
jq -e '.data.lock_version == 1 and (.data.attachments | length) == 2 and
  (.data.original_attachments | length) == 0 and
  (.data.candidate_assignees | length) >= 2 and
  (.data.allowed_actions | index("edit")) != null' "${smoke_dir}/detail.json" >/dev/null

jq -n --arg occurred "${occurred_at}" \
  '{title: "越权修改", description: "不允许", agent_response_text: null,
    agent_version: null, environment: "production", occurred_at: $occurred,
    business_reference: null, session_id: null, expected_lock_version: 1}' \
  > "${smoke_dir}/forbidden-edit-input.json"
test "$(request PATCH "/api/v1/badcases/${badcase_id}" "${member_b_cookie}" "${member_b_csrf}" \
  "${smoke_dir}/forbidden-edit-input.json" "${smoke_dir}/forbidden-edit.json")" = "403"

jq -n --arg assignee "${member_b_id}" \
  '{assignee_id: $assignee, expected_lock_version: 1}' > "${smoke_dir}/assign-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/assign" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/assign-input.json" \
  "${smoke_dir}/assigned.json")" = "200"
jq -e --arg id "${member_b_id}" '.data.assignee_id == $id and .data.lock_version == 2' \
  "${smoke_dir}/assigned.json" >/dev/null

jq -n --arg tag "${tag_constraint_id}" \
  '{issue_tag_ids: [$tag], expected_lock_version: 2}' > "${smoke_dir}/tags-input.json"
test "$(request PUT "/api/v1/badcases/${badcase_id}/issue-tags" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/tags-input.json" \
  "${smoke_dir}/tagged.json")" = "200"
jq -e --arg tag "${tag_constraint_id}" \
  '.data.lock_version == 3 and .data.issue_tags[0].id == $tag' "${smoke_dir}/tagged.json" >/dev/null

jq -n '{reason: "", expected_lock_version: 3}' > "${smoke_dir}/missing-reason-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/start-processing" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/missing-reason-input.json" \
  "${smoke_dir}/missing-reason.json")" = "422"

jq -n '{reason: "已确认问题，开始定位", expected_lock_version: 3}' \
  > "${smoke_dir}/start-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/start-processing" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/start-input.json" \
  "${smoke_dir}/processing.json")" = "200"
jq -e '.data.status == "processing" and .data.lock_version == 4' \
  "${smoke_dir}/processing.json" >/dev/null

jq -n '{note: "已定位预算过滤条件未生效", expected_lock_version: 4}' \
  > "${smoke_dir}/note-input.json"
note_key="m5-note-${timestamp}"
test "$(request POST "/api/v1/badcases/${badcase_id}/add-note" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/note-input.json" \
  "${smoke_dir}/noted.json" "${note_key}")" = "200"
jq -e '.data.lock_version == 5 and
  ([.data.activities[] | select(.activity_type == "note_added")] | length) == 1' \
  "${smoke_dir}/noted.json" >/dev/null
test "$(request POST "/api/v1/badcases/${badcase_id}/add-note" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/note-input.json" \
  "${smoke_dir}/note-replay.json" "${note_key}")" = "200"
jq -e '([.data.activities[] | select(.activity_type == "note_added")] | length) == 1' \
  "${smoke_dir}/note-replay.json" >/dev/null
jq '.note = "不同的备注"' "${smoke_dir}/note-input.json" > "${smoke_dir}/note-reused-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/add-note" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/note-reused-input.json" \
  "${smoke_dir}/note-reused.json" "${note_key}")" = "409"

jq -n '{reason: "预算过滤已修复并验证", expected_lock_version: 5}' \
  > "${smoke_dir}/resolve-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/resolve" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/resolve-input.json" \
  "${smoke_dir}/resolved.json")" = "200"
jq -e '.data.status == "resolved" and .data.resolved_at != null and .data.lock_version == 6' \
  "${smoke_dir}/resolved.json" >/dev/null

jq -n '{reason: "非法迁移", expected_lock_version: 6}' > "${smoke_dir}/invalid-transition-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/start-processing" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/invalid-transition-input.json" \
  "${smoke_dir}/invalid-transition.json")" = "409"
jq -e '.error.code == "INVALID_STATE_TRANSITION"' "${smoke_dir}/invalid-transition.json" >/dev/null

jq -n '{reason: "问题复现，需要重新处理", expected_lock_version: 6}' \
  > "${smoke_dir}/reopen-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/reopen" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/reopen-input.json" \
  "${smoke_dir}/reopened.json")" = "200"
jq -e '.data.status == "pending" and .data.resolved_at == null and .data.lock_version == 7' \
  "${smoke_dir}/reopened.json" >/dev/null

jq -n '{reason: "误报", expected_lock_version: 7}' > "${smoke_dir}/invalidate-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/invalidate" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/invalidate-input.json" \
  "${smoke_dir}/forbidden-invalidate.json")" = "403"
test "$(request POST "/api/v1/badcases/${badcase_id}/invalidate" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/invalidate-input.json" \
  "${smoke_dir}/invalidated.json")" = "200"
jq -e '.data.invalidated_at != null and .data.invalid_reason == "误报" and .data.lock_version == 8' \
  "${smoke_dir}/invalidated.json" >/dev/null

jq -n '{note: "无效后不允许", expected_lock_version: 8}' > "${smoke_dir}/invalid-note-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/add-note" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/invalid-note-input.json" \
  "${smoke_dir}/invalid-note.json" "m5-invalid-note-${timestamp}")" = "409"
jq -e '.error.code == "BADCASE_INVALIDATED"' "${smoke_dir}/invalid-note.json" >/dev/null
test "$(request DELETE "/api/v1/attachments/${first_attachment_id}?expected_owner_lock_version=8" \
  "${member_a_cookie}" "${member_a_csrf}" "" "${smoke_dir}/invalid-delete.json")" = "409"

test "$(request GET "/api/v1/pages/badcases/${badcase_id}" "${member_b_cookie}" "" "" \
  "${smoke_dir}/invalid-detail.json")" = "200"
jq -e '.data.allowed_actions | length == 0' "${smoke_dir}/invalid-detail.json" >/dev/null
test "$(request GET "/api/v1/badcases?validity=invalid&source_type=business" \
  "${member_b_cookie}" "" "" "${smoke_dir}/invalid-list.json")" = "200"
jq -e --arg id "${badcase_id}" '.data.items | any(.id == $id)' \
  "${smoke_dir}/invalid-list.json" >/dev/null

jq -n '{reason: "复核后确认是真实问题", expected_lock_version: 8}' \
  > "${smoke_dir}/reactivate-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/reactivate" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/reactivate-input.json" \
  "${smoke_dir}/reactivated.json")" = "200"
jq -e '.data.invalidated_at == null and .data.lock_version == 9' \
  "${smoke_dir}/reactivated.json" >/dev/null

test "$(request DELETE "/api/v1/attachments/${first_attachment_id}?expected_owner_lock_version=9" \
  "${member_b_cookie}" "${member_b_csrf}" "" "${smoke_dir}/forbidden-delete.json")" = "403"
test "$(request DELETE "/api/v1/attachments/${first_attachment_id}?expected_owner_lock_version=9" \
  "${member_a_cookie}" "${member_a_csrf}" "" "${smoke_dir}/deleted.json")" = "200"

jq -n --arg occurred "${occurred_at}" \
  '{title: "预算条件识别失效（已补充）", description: "输入预算后仍推荐超预算商品",
    agent_response_text: "建议采购高配服务器", agent_version: "2026.07.28-hotfix",
    environment: "production", occurred_at: $occurred,
    business_reference: "ORDER-M5-001", session_id: "chat-m5-001",
    expected_lock_version: 10}' > "${smoke_dir}/edit-input.json"
test "$(request PATCH "/api/v1/badcases/${badcase_id}" "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/edit-input.json" "${smoke_dir}/edited.json")" = "200"
jq -e '.data.title == "预算条件识别失效（已补充）" and .data.lock_version == 11' \
  "${smoke_dir}/edited.json" >/dev/null

jq -n '{expected_lock_version: 11}' > "${smoke_dir}/unassign-input.json"
test "$(request POST "/api/v1/badcases/${badcase_id}/unassign" \
  "${member_b_cookie}" "${member_b_csrf}" "${smoke_dir}/unassign-input.json" \
  "${smoke_dir}/unassigned.json")" = "200"
jq -e '.data.assignee_id == null and .data.lock_version == 12 and
  ([.data.activities[] | select(.activity_type == "assignee_changed")] | length) == 2 and
  ([.data.activities[] | select(.activity_type == "status_changed")] | length) == 3 and
  ([.data.activities[] | select(.activity_type == "invalidated")] | length) == 1 and
  ([.data.activities[] | select(.activity_type == "reactivated")] | length) == 1' \
  "${smoke_dir}/unassigned.json" >/dev/null

test "$(request DELETE "/api/v1/badcases/${badcase_id}" "${member_a_cookie}" \
  "${member_a_csrf}" "" "${smoke_dir}/delete-badcase.json")" = "404"

jq -n '{expected_lock_version: 0}' > "${smoke_dir}/disable-scenario-input.json"
test "$(request POST "/api/v1/scenarios/${scenario_id}/disable" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/disable-scenario-input.json" \
  "${smoke_dir}/disabled-scenario.json")" = "200"
test "$(request POST /api/v1/badcases "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/create-reused-input.json" "${smoke_dir}/disabled-create.json" \
  "m5-disabled-create-${timestamp}")" = "409"
jq -e '.error.code == "RESOURCE_DISABLED"' "${smoke_dir}/disabled-create.json" >/dev/null
jq --arg target "${target_id}" \
  '.evaluation_target_id = $target | .scenario_id = null | .title = "待归类业务问题"' \
  "${smoke_dir}/create-input.json" > "${smoke_dir}/unclassified-create-input.json"
test "$(request POST /api/v1/badcases "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/unclassified-create-input.json" "${smoke_dir}/unclassified-create.json" \
  "m5-unclassified-create-${timestamp}")" = "201"
jq -e '.data.scenario_id == null and .data.scenario_assignment_status == "unclassified"' \
  "${smoke_dir}/unclassified-create.json" >/dev/null

storage_dir="${repo_dir}/uploads/badcases/${badcase_id}/attachments"
test -d "${storage_dir}"
test "$(find "${storage_dir}" -type f | wc -l | tr -d ' ')" = "1"

echo "M5 smoke passed: business registration including unclassified, evidence, assignment, tags, workflow, note idempotency, invalidation/reactivation, permissions and timeline"
