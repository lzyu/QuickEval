#!/usr/bin/env bash
set -euo pipefail

api_base="${QUICKEVAL_API_BASE:-http://127.0.0.1:8080}"
admin_username="${QUICKEVAL_SMOKE_ADMIN:-admin}"
: "${QUICKEVAL_SMOKE_PASSWORD:?QUICKEVAL_SMOKE_PASSWORD is required}"

smoke_dir="/private/tmp/quickeval-m3-smoke"
mkdir -p "${smoke_dir}"
timestamp="$(date +%s)"
admin_cookie="${smoke_dir}/admin.cookie"
member_a_cookie="${smoke_dir}/member-a.cookie"
member_b_cookie="${smoke_dir}/member-b.cookie"
member_password="MemberPass123!"

request() {
  local method="$1"
  local path="$2"
  local cookie="$3"
  local csrf="$4"
  local input="${5:-}"
  local output="$6"
  local idempotency_key="${7:-}"
  local args=(-sS -o "${output}" -w "%{http_code}" -X "${method}" -b "${cookie}")
  if [[ -n "${csrf}" ]]; then
    args+=(-H "X-CSRF-Token: ${csrf}")
  fi
  if [[ -n "${idempotency_key}" ]]; then
    args+=(-H "Idempotency-Key: ${idempotency_key}")
  fi
  if [[ -n "${input}" ]]; then
    args+=(-H "Content-Type: application/json" --data-binary "@${input}")
  fi
  curl "${args[@]}" "${api_base}${path}"
}

login() {
  local username="$1"
  local password="$2"
  local cookie="$3"
  local output="$4"
  jq -n --arg username "${username}" --arg password "${password}" \
    '{username: $username, password: $password}' > "${output}-input.json"
  test "$(curl -sS -o "${output}.json" -w "%{http_code}" -c "${cookie}" \
    -H "Content-Type: application/json" --data-binary "@${output}-input.json" \
    "${api_base}/api/v1/auth/login")" = "200"
}

activate_member() {
  local username="$1" password="$2" cookie="$3" prefix="$4"
  login "${username}" "123456" "${cookie}" "${prefix}-initial"
  local csrf
  csrf="$(jq -er '.data.csrf_token' "${prefix}-initial.json")"
  jq -n --arg password "${password}" \
    '{current_password: "123456", new_password: $password}' > "${prefix}-change-password.json"
  test "$(request POST /api/v1/auth/change-password "${cookie}" "${csrf}" \
    "${prefix}-change-password.json" "${prefix}-change-password-response.json")" = "204"
  login "${username}" "${password}" "${cookie}" "${prefix}"
}

login "${admin_username}" "${QUICKEVAL_SMOKE_PASSWORD}" "${admin_cookie}" "${smoke_dir}/admin-session"
admin_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/admin-session.json")"

for suffix in a b; do
  username="m3_member_${suffix}_${timestamp}"
  jq -n --arg username "${username}" \
    --arg name "M3 验收成员 ${suffix}" \
    '{username: $username, display_name: $name, email: null, role: "member"}' \
    > "${smoke_dir}/member-${suffix}-input.json"
  test "$(request POST /api/v1/users "${admin_cookie}" "${admin_csrf}" \
    "${smoke_dir}/member-${suffix}-input.json" "${smoke_dir}/member-${suffix}.json")" = "201"
done
member_a_username="$(jq -er '.data.username' "${smoke_dir}/member-a.json")"
member_b_username="$(jq -er '.data.username' "${smoke_dir}/member-b.json")"
member_a_id="$(jq -er '.data.id' "${smoke_dir}/member-a.json")"

jq -n --arg name "M3 评测对象 ${timestamp}" \
  '{name: $name, description: "M3 smoke"}' > "${smoke_dir}/target-input.json"
test "$(request POST /api/v1/evaluation-targets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/target-input.json" "${smoke_dir}/target.json")" = "201"
target_id="$(jq -er '.data.id' "${smoke_dir}/target.json")"

jq -n --arg id "${target_id}" \
  '{evaluation_target_id: $id, name: "采购推荐", description: "M3 smoke"}' \
  > "${smoke_dir}/scenario-input.json"
test "$(request POST /api/v1/scenarios "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/scenario-input.json" "${smoke_dir}/scenario.json")" = "201"
scenario_id="$(jq -er '.data.id' "${smoke_dir}/scenario.json")"

jq -n --arg id "${target_id}" --arg name "M3 基础评测集 ${timestamp}" \
  '{evaluation_target_id: $id, name: $name, description: "M3 smoke"}' > "${smoke_dir}/dataset-input.json"
test "$(request POST /api/v1/datasets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/dataset-input.json" "${smoke_dir}/dataset.json")" = "201"
draft_id="$(jq -er '.data.draft.id' "${smoke_dir}/dataset.json")"

for position in 1 2; do
  jq -n --arg scenario "${scenario_id}" --arg name "用例 ${position}" --arg prompt "请回答采购问题 ${position}" \
    '{scenario_id: $scenario, name: $name, user_prompt: $prompt, precondition: null, expected_result: null,
      judging_guide: "回答准确", is_enabled: true, tag_ids: []}' \
    > "${smoke_dir}/case-${position}-input.json"
  test "$(request POST "/api/v1/dataset-versions/${draft_id}/cases" \
    "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/case-${position}-input.json" \
    "${smoke_dir}/case-${position}.json")" = "201"
done

test "$(request GET "/api/v1/dataset-versions/${draft_id}" "${admin_cookie}" "" "" \
  "${smoke_dir}/draft.json")" = "200"
draft_lock="$(jq -er '.data.lock_version' "${smoke_dir}/draft.json")"
jq -n --argjson lock "${draft_lock}" \
  '{release_note: "M3 smoke", expected_lock_version: $lock}' > "${smoke_dir}/publish-input.json"
test "$(request POST "/api/v1/dataset-versions/${draft_id}/publish" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/publish-input.json" "${smoke_dir}/version.json")" = "200"

activate_member "${member_a_username}" "${member_password}" "${member_a_cookie}" "${smoke_dir}/member-a-session"
member_a_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/member-a-session.json")"
activate_member "${member_b_username}" "${member_password}" "${member_b_cookie}" "${smoke_dir}/member-b-session"
member_b_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/member-b-session.json")"

jq -n --arg version "${draft_id}" \
  '{dataset_version_id: $version, agent_version: "2026.07.28",
    environment: "staging", purpose_note: "M3 smoke", config_note: null}' \
  > "${smoke_dir}/run-input.json"
idempotency_key="m3-${timestamp}-primary"
test "$(request POST /api/v1/evaluation-runs "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/run-input.json" "${smoke_dir}/run.json" "${idempotency_key}")" = "201"
run_id="$(jq -er '.data.id' "${smoke_dir}/run.json")"
jq -e '.data.status == "in_progress" and .data.progress.total_count == 2 and
  .data.progress.pending_count == 2' "${smoke_dir}/run.json" >/dev/null

test "$(request POST /api/v1/evaluation-runs "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/run-input.json" "${smoke_dir}/run-replay.json" "${idempotency_key}")" = "200"
jq -e --arg id "${run_id}" '.data.id == $id' "${smoke_dir}/run-replay.json" >/dev/null

test "$(request POST /api/v1/evaluation-runs "${member_a_cookie}" "${member_a_csrf}" \
  "${smoke_dir}/run-input.json" "${smoke_dir}/run-second.json" "m3-${timestamp}-second")" = "201"
second_run_id="$(jq -er '.data.id' "${smoke_dir}/run-second.json")"
test "${second_run_id}" != "${run_id}"

jq -n '{expected_lock_version: 0}' > "${smoke_dir}/complete-pending-input.json"
test "$(request POST "/api/v1/evaluation-runs/${run_id}/complete" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/complete-pending-input.json" \
  "${smoke_dir}/complete-pending.json")" = "409"
jq -e '.error.code == "PENDING_RESULTS_EXIST" and .error.details.pending_count == 2' \
  "${smoke_dir}/complete-pending.json" >/dev/null

test "$(request GET "/api/v1/pages/evaluation-runs/${run_id}/workbench?page_size=100" \
  "${member_a_cookie}" "" "" "${smoke_dir}/workbench.json")" = "200"
first_result_id="$(jq -er '.data.results.items[0].id' "${smoke_dir}/workbench.json")"
second_result_id="$(jq -er '.data.results.items[1].id' "${smoke_dir}/workbench.json")"

jq -n '{status: "evaluated", answer_text: null, score: null, comment: null,
  skip_reason: null, expected_lock_version: 0}' > "${smoke_dir}/missing-score-input.json"
test "$(request PATCH "/api/v1/case-results/${first_result_id}" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/missing-score-input.json" \
  "${smoke_dir}/missing-score.json")" = "422"

jq -n '{status: "evaluated", answer_text: null, score: 2, comment: null,
  skip_reason: null, expected_lock_version: 0}' > "${smoke_dir}/low-score-input.json"
test "$(request PATCH "/api/v1/case-results/${first_result_id}" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/low-score-input.json" \
  "${smoke_dir}/low-score.json")" = "422"

jq -n '{status: "evaluated", answer_text: "Agent 实际回答", score: 4,
  comment: "回答基本正确", skip_reason: null, expected_lock_version: 0}' \
  > "${smoke_dir}/evaluate-input.json"
test "$(request PATCH "/api/v1/case-results/${first_result_id}" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/evaluate-input.json" \
  "${smoke_dir}/evaluated.json")" = "200"
jq -e '.data.progress.evaluated_count == 1 and .data.progress.scored_count == 1 and
  .data.progress.average_score == 4' "${smoke_dir}/evaluated.json" >/dev/null

jq -n '{status: "skipped", answer_text: null, score: 5, comment: null,
  skip_reason: "当前账号无权限", expected_lock_version: 0}' > "${smoke_dir}/skip-input.json"
test "$(request PATCH "/api/v1/case-results/${second_result_id}" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/skip-input.json" \
  "${smoke_dir}/skipped.json")" = "200"
jq -e '.data.result.status == "skipped" and .data.result.score == null and
  .data.progress.pending_count == 0 and .data.progress.skipped_count == 1' \
  "${smoke_dir}/skipped.json" >/dev/null
run_lock="$(jq -er '.data.run_lock_version' "${smoke_dir}/skipped.json")"

test "$(request GET "/api/v1/evaluation-runs/${run_id}" "${member_b_cookie}" "" "" \
  "${smoke_dir}/forbidden.json")" = "403"

jq -n --argjson lock "${run_lock}" '{expected_lock_version: $lock}' \
  > "${smoke_dir}/complete-input.json"
test "$(request POST "/api/v1/evaluation-runs/${run_id}/complete" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/complete-input.json" \
  "${smoke_dir}/completed.json")" = "200"
jq -e '.data.status == "completed" and .data.first_completed_at != null and
  .data.completed_at != null and .data.progress.completion_rate == 1' \
  "${smoke_dir}/completed.json" >/dev/null
first_completed_at="$(jq -er '.data.first_completed_at' "${smoke_dir}/completed.json")"

jq '.expected_lock_version = 1 | .answer_text = "完成后不可修改"' \
  "${smoke_dir}/evaluate-input.json" > "${smoke_dir}/immutable-result-input.json"
test "$(request PATCH "/api/v1/case-results/${first_result_id}" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/immutable-result-input.json" \
  "${smoke_dir}/immutable-result.json")" = "409"
jq -e '.error.code == "RUN_NOT_EDITABLE"' "${smoke_dir}/immutable-result.json" >/dev/null

completed_lock="$(jq -er '.data.lock_version' "${smoke_dir}/completed.json")"
jq -n --argjson lock "${completed_lock}" \
  '{reason: "", expected_lock_version: $lock}' > "${smoke_dir}/reopen-missing-input.json"
test "$(request POST "/api/v1/evaluation-runs/${run_id}/reopen" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/reopen-missing-input.json" \
  "${smoke_dir}/reopen-missing.json")" = "422"

jq -n --argjson lock "${completed_lock}" \
  '{reason: "补充验证", expected_lock_version: $lock}' > "${smoke_dir}/reopen-input.json"
test "$(request POST "/api/v1/evaluation-runs/${run_id}/reopen" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/reopen-input.json" \
  "${smoke_dir}/reopened.json")" = "200"
jq -e --arg first "${first_completed_at}" \
  '.data.status == "in_progress" and .data.completed_at == null and .data.first_completed_at == $first' \
  "${smoke_dir}/reopened.json" >/dev/null

reopened_lock="$(jq -er '.data.lock_version' "${smoke_dir}/reopened.json")"
jq -n --argjson lock "${reopened_lock}" \
  '{expected_lock_version: $lock}' > "${smoke_dir}/complete-again-input.json"
test "$(request POST "/api/v1/evaluation-runs/${run_id}/complete" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/complete-again-input.json" \
  "${smoke_dir}/completed-again.json")" = "200"

completed_again_lock="$(jq -er '.data.lock_version' "${smoke_dir}/completed-again.json")"
test "$(request DELETE "/api/v1/evaluation-runs/${run_id}?expected_lock_version=${completed_again_lock}" \
  "${member_a_cookie}" "${member_a_csrf}" "" "${smoke_dir}/delete-completed.json")" = "409"
jq -e '.error.code == "RUN_DELETE_FORBIDDEN"' "${smoke_dir}/delete-completed.json" >/dev/null

jq -n --argjson lock "${completed_again_lock}" \
  '{reason: "验收作废", expected_lock_version: $lock}' > "${smoke_dir}/void-input.json"
test "$(request POST "/api/v1/evaluation-runs/${run_id}/void" \
  "${member_a_cookie}" "${member_a_csrf}" "${smoke_dir}/void-input.json" \
  "${smoke_dir}/voided.json")" = "200"
jq -e '.data.status == "voided" and .data.void_reason == "验收作废"' \
  "${smoke_dir}/voided.json" >/dev/null

second_lock="$(jq -er '.data.lock_version' "${smoke_dir}/run-second.json")"
test "$(request DELETE "/api/v1/evaluation-runs/${second_run_id}?expected_lock_version=${second_lock}" \
  "${member_a_cookie}" "${member_a_csrf}" "" "${smoke_dir}/delete-second.json")" = "204"

test "$(request GET "/api/v1/evaluation-runs?evaluator_id=${member_a_id}&page_size=100" \
  "${admin_cookie}" "" "" "${smoke_dir}/admin-runs.json")" = "200"
jq -e --arg id "${run_id}" '.data.items | any(.id == $id and .status == "voided")' \
  "${smoke_dir}/admin-runs.json" >/dev/null

echo "M3 smoke passed: independent runs, idempotency, evidence, scoring, skip, ownership, complete, reopen, void and delete"
