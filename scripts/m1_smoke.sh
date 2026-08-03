#!/usr/bin/env bash
set -euo pipefail

api_base="${QUICKEVAL_API_BASE:-http://127.0.0.1:8080}"
admin_username="${QUICKEVAL_SMOKE_ADMIN:-admin}"
: "${QUICKEVAL_SMOKE_PASSWORD:?QUICKEVAL_SMOKE_PASSWORD is required}"

smoke_dir="/private/tmp/quickeval-m1-smoke"
mkdir -p "${smoke_dir}"
admin_cookie="${smoke_dir}/admin.cookie"
member_cookie="${smoke_dir}/member.cookie"
timestamp="$(date +%s)"
member_username="m1_member_${timestamp}"
member_password="MemberPass123!"

request() {
  local method="$1"
  local path="$2"
  local cookie="$3"
  local csrf="$4"
  local input="${5:-}"
  local output="$6"
  local status
  local args=(-sS -o "${output}" -w "%{http_code}" -X "${method}" -b "${cookie}")
  if [[ -n "${csrf}" ]]; then
    args+=(-H "X-CSRF-Token: ${csrf}")
  fi
  if [[ -n "${input}" ]]; then
    args+=(-H "Content-Type: application/json" --data-binary "@${input}")
  fi
  status="$(curl "${args[@]}" "${api_base}${path}")"
  printf "%s" "${status}"
}

jq -n --arg username "${admin_username}" --arg password "${QUICKEVAL_SMOKE_PASSWORD}" \
  '{username: $username, password: $password}' > "${smoke_dir}/admin-login.json"
admin_status="$(curl -sS -o "${smoke_dir}/admin-session.json" -w "%{http_code}" \
  -c "${admin_cookie}" -H "Content-Type: application/json" \
  --data-binary "@${smoke_dir}/admin-login.json" "${api_base}/api/v1/auth/login")"
test "${admin_status}" = "200"
admin_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/admin-session.json")"
admin_id="$(jq -er '.data.user.id' "${smoke_dir}/admin-session.json")"

jq -n '{name: "missing csrf"}' > "${smoke_dir}/missing-csrf.json"
test "$(request POST /api/v1/issue-tags "${admin_cookie}" "" \
  "${smoke_dir}/missing-csrf.json" "${smoke_dir}/missing-csrf-response.json")" = "403"
jq -e '.error.code == "CSRF_INVALID"' "${smoke_dir}/missing-csrf-response.json" >/dev/null

jq -n --arg username "${member_username}" --arg password "${member_password}" \
  '{username: $username, display_name: "M1 验收成员", email: null, role: "member", password: $password}' \
  > "${smoke_dir}/create-member.json"
test "$(request POST /api/v1/users "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/create-member.json" "${smoke_dir}/member.json")" = "201"
member_id="$(jq -er '.data.id' "${smoke_dir}/member.json")"

jq -n --arg name "M1 评测对象 ${timestamp}" \
  '{name: $name, description: "M1 smoke"}' > "${smoke_dir}/create-target.json"
test "$(request POST /api/v1/evaluation-targets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/create-target.json" "${smoke_dir}/target.json")" = "201"
target_id="$(jq -er '.data.id' "${smoke_dir}/target.json")"

jq -n --arg target_id "${target_id}" \
  '{evaluation_target_id: $target_id, name: "智能采购询价", description: "M1 smoke"}' \
  > "${smoke_dir}/create-scenario.json"
test "$(request POST /api/v1/scenarios "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/create-scenario.json" "${smoke_dir}/scenario.json")" = "201"
scenario_id="$(jq -er '.data.id' "${smoke_dir}/scenario.json")"

jq -n '{name: "询价准确性", description: "用例分类"}' > "${smoke_dir}/create-case-tag.json"
test "$(request POST "/api/v1/evaluation-targets/${target_id}/case-tags" "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/create-case-tag.json" "${smoke_dir}/case-tag.json")" = "201"
scenario_tag_id="$(jq -er '.data.id' "${smoke_dir}/case-tag.json")"
global_tag_name="意图识别 ${timestamp}"
jq -n --arg name "${global_tag_name}" \
  '{scope: "global", evaluation_target_id: null, name: $name, description: "跨对象通用能力"}' \
  > "${smoke_dir}/create-global-case-tag.json"
test "$(request POST /api/v1/case-tags "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/create-global-case-tag.json" "${smoke_dir}/global-case-tag.json")" = "201"
global_tag_id="$(jq -er '.data.id' "${smoke_dir}/global-case-tag.json")"
test "$(request GET "/api/v1/case-tags?scope=global" "${admin_cookie}" "" "" \
  "${smoke_dir}/global-case-tags.json")" = "200"
jq -e --arg id "${global_tag_id}" \
  '.data.items | any(.id == $id and .scope == "global" and .evaluation_target_id == null)' \
  "${smoke_dir}/global-case-tags.json" >/dev/null
test "$(request GET "/api/v1/evaluation-targets/${target_id}/available-case-tags" \
  "${admin_cookie}" "" "" "${smoke_dir}/available-case-tags.json")" = "200"
jq -e --arg global "${global_tag_id}" --arg target "${scenario_tag_id}" \
  '(.data.global | any(.id == $global)) and (.data.target | any(.id == $target))' \
  "${smoke_dir}/available-case-tags.json" >/dev/null
jq -n --arg name "${global_tag_name}" '{name: $name, description: "应与全局标签冲突"}' \
  > "${smoke_dir}/conflicting-case-tag.json"
test "$(request POST "/api/v1/evaluation-targets/${target_id}/case-tags" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/conflicting-case-tag.json" \
  "${smoke_dir}/conflicting-case-tag-response.json")" = "409"
jq -e '.error.code == "NAME_CONFLICT"' \
  "${smoke_dir}/conflicting-case-tag-response.json" >/dev/null
jq '{items: [{id: .data.id, sort_order: 10, expected_lock_version: .data.lock_version}]}' \
  "${smoke_dir}/case-tag.json" > "${smoke_dir}/reorder-case-tag.json"
test "$(request PUT "/api/v1/evaluation-targets/${target_id}/case-tags/reorder" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/reorder-case-tag.json" \
  "${smoke_dir}/reorder-case-tag-response.json")" = "204"

jq -n --arg name "事实错误 ${timestamp}" \
  '{name: $name, description: "回答包含错误事实"}' > "${smoke_dir}/create-issue-tag.json"
test "$(request POST /api/v1/issue-tags "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/create-issue-tag.json" "${smoke_dir}/issue-tag.json")" = "201"
issue_tag_id="$(jq -er '.data.id' "${smoke_dir}/issue-tag.json")"
jq -n '{expected_lock_version: 0}' > "${smoke_dir}/disable-issue-tag.json"
test "$(request POST "/api/v1/issue-tags/${issue_tag_id}/disable" "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/disable-issue-tag.json" "${smoke_dir}/disabled-issue-tag.json")" = "200"
test "$(request GET /api/v1/issue-tags "${admin_cookie}" "" "" \
  "${smoke_dir}/admin-issue-tags.json")" = "200"
jq '{items: (.data.items | to_entries | map({
  id: .value.id,
  sort_order: ((.key + 1) * 10),
  expected_lock_version: .value.lock_version
}))}' "${smoke_dir}/admin-issue-tags.json" > "${smoke_dir}/reorder-issue-tags.json"
test "$(request PUT /api/v1/issue-tags/reorder "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/reorder-issue-tags.json" "${smoke_dir}/reorder-issue-tags-response.json")" = "204"

test "$(request GET "/api/v1/scenarios?evaluation_target_id=${target_id}&page_size=100" \
  "${admin_cookie}" "" "" "${smoke_dir}/scenarios.json")" = "200"
jq -e --arg id "${scenario_id}" --arg target_id "${target_id}" \
  '.data.items | any(.id == $id and .evaluation_target_id == $target_id and (.evaluation_target_name | length > 0))' \
  "${smoke_dir}/scenarios.json" >/dev/null
test "$(request GET "/api/v1/users?page_size=100" "${admin_cookie}" "" "" \
  "${smoke_dir}/users.json")" = "200"
jq -e --arg id "${member_id}" '.data.items | any(.id == $id)' "${smoke_dir}/users.json" >/dev/null

test "$(request GET "/api/v1/audit-logs?page_size=100" "${admin_cookie}" "" "" \
  "${smoke_dir}/audit.json")" = "200"
jq -e '.data.items | any(.action == "issue_tag.created")' "${smoke_dir}/audit.json" >/dev/null

jq -n --arg username "${member_username}" --arg password "${member_password}" \
  '{username: $username, password: $password}' > "${smoke_dir}/member-login.json"
test "$(curl -sS -o "${smoke_dir}/member-session.json" -w "%{http_code}" -c "${member_cookie}" \
  -H "Content-Type: application/json" --data-binary "@${smoke_dir}/member-login.json" \
  "${api_base}/api/v1/auth/login")" = "200"
test "$(request GET /api/v1/evaluation-targets "${member_cookie}" "" "" \
  "${smoke_dir}/member-targets.json")" = "200"
test "$(request GET /api/v1/issue-tags "${member_cookie}" "" "" \
  "${smoke_dir}/member-issue-tags.json")" = "200"
jq -e --arg id "${issue_tag_id}" '.data.items | all(.id != $id)' \
  "${smoke_dir}/member-issue-tags.json" >/dev/null
test "$(request GET /api/v1/users "${member_cookie}" "" "" \
  "${smoke_dir}/member-users.json")" = "403"

jq -n --arg password "ResetPass123!" '{password: $password}' > "${smoke_dir}/reset-member.json"
test "$(request POST "/api/v1/users/${member_id}/reset-password" "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/reset-member.json" "${smoke_dir}/reset-response.json")" = "204"
test "$(request GET /api/v1/auth/session "${member_cookie}" "" "" \
  "${smoke_dir}/revoked-member-session.json")" = "401"

jq -n '{expected_lock_version: 0}' > "${smoke_dir}/self-disable.json"
test "$(request POST "/api/v1/users/${admin_id}/disable" "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/self-disable.json" "${smoke_dir}/self-disable-response.json")" = "409"

rate_username="rate_test_${timestamp}"
jq -n --arg username "${rate_username}" \
  '{username: $username, password: "wrong-password"}' > "${smoke_dir}/rate-login.json"
for _ in 1 2 3 4 5; do
  test "$(curl -sS -o "${smoke_dir}/rate-response.json" -w "%{http_code}" \
    -H "Content-Type: application/json" --data-binary "@${smoke_dir}/rate-login.json" \
    "${api_base}/api/v1/auth/login")" = "401"
done
test "$(curl -sS -o "${smoke_dir}/rate-limited.json" -w "%{http_code}" \
  -H "Content-Type: application/json" --data-binary "@${smoke_dir}/rate-login.json" \
  "${api_base}/api/v1/auth/login")" = "429"

test "$(request DELETE /api/v1/auth/session "${admin_cookie}" "${admin_csrf}" "" \
  "${smoke_dir}/logout.json")" = "204"
test "$(request GET /api/v1/auth/session "${admin_cookie}" "" "" \
  "${smoke_dir}/logged-out-session.json")" = "401"

echo "M1 smoke passed: auth, CSRF, users, catalog, tags, audit, RBAC, revocation, rate limit"
