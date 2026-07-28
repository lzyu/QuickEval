#!/usr/bin/env bash
set -euo pipefail

api_base="${QUICKEVAL_API_BASE:-http://127.0.0.1:8080}"
admin_username="${QUICKEVAL_SMOKE_ADMIN:-admin}"
: "${QUICKEVAL_SMOKE_PASSWORD:?QUICKEVAL_SMOKE_PASSWORD is required}"

smoke_dir="/private/tmp/quickeval-m6-smoke"
mkdir -p "${smoke_dir}"
timestamp="$(date +%s)"
admin_cookie="${smoke_dir}/admin.cookie"
member_cookie="${smoke_dir}/member.cookie"
member_password="MemberPass123!"

request() {
  local method="$1" path="$2" cookie="$3" csrf="$4" input="${5:-}" output="$6"
  local key="${7:-}"
  local args=(-sS -o "${output}" -w "%{http_code}" -X "${method}" -b "${cookie}")
  [[ -n "${csrf}" ]] && args+=(-H "X-CSRF-Token: ${csrf}")
  [[ -n "${key}" ]] && args+=(-H "Idempotency-Key: ${key}")
  [[ -n "${input}" ]] && args+=(-H "Content-Type: application/json" --data-binary "@${input}")
  curl "${args[@]}" "${api_base}${path}"
}

login() {
  local username="$1" password="$2" cookie="$3" prefix="$4"
  jq -n --arg username "${username}" --arg password "${password}" \
    '{username:$username,password:$password}' > "${prefix}-input.json"
  test "$(curl -sS -o "${prefix}.json" -w "%{http_code}" -c "${cookie}" \
    -H "Content-Type: application/json" --data-binary "@${prefix}-input.json" \
    "${api_base}/api/v1/auth/login")" = "200"
}

login "${admin_username}" "${QUICKEVAL_SMOKE_PASSWORD}" "${admin_cookie}" "${smoke_dir}/admin-session"
admin_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/admin-session.json")"

jq -n --arg username "m6_member_${timestamp}" --arg password "${member_password}" \
  '{username:$username,display_name:"M6 验收成员",email:null,role:"member",password:$password}' \
  > "${smoke_dir}/member-input.json"
test "$(request POST /api/v1/users "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/member-input.json" "${smoke_dir}/member.json")" = "201"
member_id="$(jq -er '.data.id' "${smoke_dir}/member.json")"
member_username="$(jq -er '.data.username' "${smoke_dir}/member.json")"

jq -n --arg name "M6 智能采购 ${timestamp}" \
  '{name:$name,description:"M6 dashboard smoke"}' > "${smoke_dir}/target-input.json"
test "$(request POST /api/v1/evaluation-targets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/target-input.json" "${smoke_dir}/target.json")" = "201"
target_id="$(jq -er '.data.id' "${smoke_dir}/target.json")"

jq -n --arg target "${target_id}" --arg name "M6 采购分析 ${timestamp}" \
  '{evaluation_target_id:$target,name:$name,description:"M6 search marker"}' \
  > "${smoke_dir}/scenario-input.json"
test "$(request POST /api/v1/scenarios "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/scenario-input.json" "${smoke_dir}/scenario.json")" = "201"
scenario_id="$(jq -er '.data.id' "${smoke_dir}/scenario.json")"

jq -n --arg name "M6 事实问题 ${timestamp}" \
  '{name:$name,description:"M6 distribution"}' > "${smoke_dir}/tag-input.json"
test "$(request POST /api/v1/issue-tags "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/tag-input.json" "${smoke_dir}/tag.json")" = "201"
tag_id="$(jq -er '.data.id' "${smoke_dir}/tag.json")"

jq -n --arg scenario "${scenario_id}" --arg name "M6 版本对比 ${timestamp}" \
  '{scenario_id:$scenario,name:$name,description:"M6 search and comparison"}' \
  > "${smoke_dir}/dataset-input.json"
test "$(request POST /api/v1/datasets "${admin_cookie}" "${admin_csrf}" \
  "${smoke_dir}/dataset-input.json" "${smoke_dir}/dataset.json")" = "201"
dataset_id="$(jq -er '.data.dataset.id' "${smoke_dir}/dataset.json")"
v1_id="$(jq -er '.data.draft.id' "${smoke_dir}/dataset.json")"

for position in 1 2; do
  jq -n --arg name "M6 用例 ${position} ${timestamp}" --arg prompt "M6 采购问题 ${position} ${timestamp}" \
    '{name:$name,user_prompt:$prompt,precondition:null,expected_result:null,
      judging_guide:"按事实评分",is_enabled:true,tag_ids:[]}' \
    > "${smoke_dir}/case-${position}-input.json"
  test "$(request POST "/api/v1/dataset-versions/${v1_id}/cases" \
    "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/case-${position}-input.json" \
    "${smoke_dir}/case-${position}.json")" = "201"
done
test "$(request GET "/api/v1/dataset-versions/${v1_id}" "${admin_cookie}" "" "" \
  "${smoke_dir}/v1-draft.json")" = "200"
v1_lock="$(jq -er '.data.lock_version' "${smoke_dir}/v1-draft.json")"
jq -n --argjson lock "${v1_lock}" \
  '{release_note:"M6 V1",expected_lock_version:$lock}' > "${smoke_dir}/publish-v1-input.json"
test "$(request POST "/api/v1/dataset-versions/${v1_id}/publish" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/publish-v1-input.json" \
  "${smoke_dir}/published-v1.json")" = "200"

test "$(request GET "/api/v1/datasets/${dataset_id}" "${admin_cookie}" "" "" \
  "${smoke_dir}/dataset-detail.json")" = "200"
dataset_lock="$(jq -er '.data.dataset.lock_version' "${smoke_dir}/dataset-detail.json")"
jq -n --arg base "${v1_id}" --argjson lock "${dataset_lock}" \
  '{base_version_id:$base,expected_dataset_lock_version:$lock}' > "${smoke_dir}/draft-v2-input.json"
test "$(request POST "/api/v1/datasets/${dataset_id}/drafts" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/draft-v2-input.json" \
  "${smoke_dir}/draft-v2.json")" = "201"
v2_id="$(jq -er '.data.id' "${smoke_dir}/draft-v2.json")"
v2_lock="$(jq -er '.data.lock_version' "${smoke_dir}/draft-v2.json")"
jq -n --argjson lock "${v2_lock}" \
  '{release_note:"M6 V2",expected_lock_version:$lock}' > "${smoke_dir}/publish-v2-input.json"
test "$(request POST "/api/v1/dataset-versions/${v2_id}/publish" \
  "${admin_cookie}" "${admin_csrf}" "${smoke_dir}/publish-v2-input.json" \
  "${smoke_dir}/published-v2.json")" = "200"

login "${member_username}" "${member_password}" "${member_cookie}" "${smoke_dir}/member-session"
member_csrf="$(jq -er '.data.csrf_token' "${smoke_dir}/member-session.json")"

create_run() {
  local version="$1" suffix="$2"
  jq -n --arg version "${version}" --arg agent "m6-agent-${timestamp}" \
    '{dataset_version_id:$version,agent_version:$agent,environment:"production",
      purpose_note:"M6 smoke",config_note:null}' > "${smoke_dir}/run-${suffix}-input.json"
  test "$(request POST /api/v1/evaluation-runs "${member_cookie}" "${member_csrf}" \
    "${smoke_dir}/run-${suffix}-input.json" "${smoke_dir}/run-${suffix}.json" \
    "m6-run-${suffix}-${timestamp}")" = "201"
}

create_run "${v1_id}" v1
run_v1="$(jq -er '.data.id' "${smoke_dir}/run-v1.json")"
test "$(request GET "/api/v1/pages/evaluation-runs/${run_v1}/workbench?page_size=100" \
  "${member_cookie}" "" "" "${smoke_dir}/workbench-v1.json")" = "200"
v1_result_1="$(jq -er '.data.results.items[0].id' "${smoke_dir}/workbench-v1.json")"
v1_result_2="$(jq -er '.data.results.items[1].id' "${smoke_dir}/workbench-v1.json")"
jq -n '{status:"evaluated",answer_text:"M6 V1 回答",score:4,comment:"M6 评语",
  skip_reason:null,expected_lock_version:0}' > "${smoke_dir}/evaluate-v1.json"
test "$(request PATCH "/api/v1/case-results/${v1_result_1}" "${member_cookie}" "${member_csrf}" \
  "${smoke_dir}/evaluate-v1.json" "${smoke_dir}/evaluated-v1.json")" = "200"
jq -n '{status:"skipped",answer_text:null,score:null,comment:null,
  skip_reason:"当前账号无权限",expected_lock_version:0}' > "${smoke_dir}/skip-v1.json"
test "$(request PATCH "/api/v1/case-results/${v1_result_2}" "${member_cookie}" "${member_csrf}" \
  "${smoke_dir}/skip-v1.json" "${smoke_dir}/skipped-v1.json")" = "200"
jq -n --arg tag "${tag_id}" \
  '{expected_result_lock_version:1,badcase:{title:"M6 评测 Badcase",
    description:"M6 评测问题",issue_tag_ids:[$tag]}}' > "${smoke_dir}/mark-v1.json"
test "$(request POST "/api/v1/case-results/${v1_result_1}/mark-badcase" \
  "${member_cookie}" "${member_csrf}" "${smoke_dir}/mark-v1.json" \
  "${smoke_dir}/marked-v1.json" "m6-mark-${timestamp}")" = "201"
run_v1_lock="$(jq -er '.data.run_lock_version' "${smoke_dir}/marked-v1.json")"
jq -n --argjson lock "${run_v1_lock}" '{expected_lock_version:$lock}' \
  > "${smoke_dir}/complete-v1.json"
test "$(request POST "/api/v1/evaluation-runs/${run_v1}/complete" \
  "${member_cookie}" "${member_csrf}" "${smoke_dir}/complete-v1.json" \
  "${smoke_dir}/completed-v1.json")" = "200"

create_run "${v2_id}" v2
run_v2="$(jq -er '.data.id' "${smoke_dir}/run-v2.json")"
test "$(request GET "/api/v1/pages/evaluation-runs/${run_v2}/workbench?page_size=100" \
  "${member_cookie}" "" "" "${smoke_dir}/workbench-v2.json")" = "200"
run_v2_lock=0
position=0
for result_id in $(jq -er '.data.results.items[].id' "${smoke_dir}/workbench-v2.json"); do
  score=2
  [[ "${position}" = "1" ]] && score=5
  jq -n --arg answer "M6 V2 回答 ${position} ${timestamp}" --argjson score "${score}" \
    '{status:"evaluated",answer_text:$answer,score:$score,comment:"M6 V2",
      skip_reason:null,expected_lock_version:0}' > "${smoke_dir}/evaluate-v2-${position}.json"
  test "$(request PATCH "/api/v1/case-results/${result_id}" "${member_cookie}" "${member_csrf}" \
    "${smoke_dir}/evaluate-v2-${position}.json" "${smoke_dir}/evaluated-v2-${position}.json")" = "200"
  run_v2_lock="$(jq -er '.data.run_lock_version' "${smoke_dir}/evaluated-v2-${position}.json")"
  position=$((position + 1))
done
jq -n --argjson lock "${run_v2_lock}" '{expected_lock_version:$lock}' \
  > "${smoke_dir}/complete-v2.json"
test "$(request POST "/api/v1/evaluation-runs/${run_v2}/complete" \
  "${member_cookie}" "${member_csrf}" "${smoke_dir}/complete-v2.json" \
  "${smoke_dir}/completed-v2.json")" = "200"

create_run "${v2_id}" open
open_run_id="$(jq -er '.data.id' "${smoke_dir}/run-open.json")"

occurred_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
create_business() {
  local suffix="$1" title="$2"
  jq -n --arg scenario "${scenario_id}" --arg tag "${tag_id}" \
    --arg occurred "${occurred_at}" --arg title "${title}" \
    '{scenario_id:$scenario,title:$title,description:"M6 业务问题",
      agent_response_text:"M6 业务回答",agent_version:"m6-business",
      environment:"production",occurred_at:$occurred,business_reference:"M6-ORDER",
      session_id:"M6-SESSION",issue_tag_ids:[$tag]}' > "${smoke_dir}/business-${suffix}-input.json"
  test "$(request POST /api/v1/badcases "${member_cookie}" "${member_csrf}" \
    "${smoke_dir}/business-${suffix}-input.json" "${smoke_dir}/business-${suffix}.json" \
    "m6-business-${suffix}-${timestamp}")" = "201"
}
create_business valid "M6 业务 Badcase ${timestamp}"
business_id="$(jq -er '.data.id' "${smoke_dir}/business-valid.json")"
jq -n --arg assignee "${member_id}" \
  '{assignee_id:$assignee,expected_lock_version:0}' > "${smoke_dir}/assign.json"
test "$(request POST "/api/v1/badcases/${business_id}/assign" "${member_cookie}" "${member_csrf}" \
  "${smoke_dir}/assign.json" "${smoke_dir}/assigned.json")" = "200"

create_business invalid "M6 无效 Badcase ${timestamp}"
invalid_id="$(jq -er '.data.id' "${smoke_dir}/business-invalid.json")"
jq -n '{reason:"M6 验证无效排除",expected_lock_version:0}' > "${smoke_dir}/invalidate.json"
test "$(request POST "/api/v1/badcases/${invalid_id}/invalidate" \
  "${member_cookie}" "${member_csrf}" "${smoke_dir}/invalidate.json" \
  "${smoke_dir}/invalidated.json")" = "200"

test "$(request GET /api/v1/pages/home "${member_cookie}" "" "" "${smoke_dir}/home.json")" = "200"
jq -e --arg open "${open_run_id}" --arg badcase "${business_id}" \
  '.data.metrics | map({key,value}) | from_entries |
    .in_progress == 1 and .completed == 2 and .assigned_badcases == 1' \
  "${smoke_dir}/home.json" >/dev/null
jq -e --arg open "${open_run_id}" --arg badcase "${business_id}" \
  '(.data.continue_evaluations | any(.id == $open)) and
   (.data.assigned_badcases | any(.id == $badcase))' "${smoke_dir}/home.json" >/dev/null

test "$(request GET "/api/v1/pages/dashboard?scenario_id=${scenario_id}" \
  "${member_cookie}" "" "" "${smoke_dir}/dashboard.json")" = "200"
jq -e --arg tag "${tag_id}" \
  '.data.metrics.completed_run_count == 2 and
   .data.metrics.evaluated_case_count == 3 and
   .data.metrics.scored_case_count == 3 and
   (.data.metrics.average_score > 3.66 and .data.metrics.average_score < 3.67) and
   .data.metrics.skipped_case_count == 1 and
   .data.metrics.evaluation_badcase_count == 1 and
   (.data.metrics.evaluation_badcase_rate > 0.33 and .data.metrics.evaluation_badcase_rate < 0.34) and
   .data.metrics.valid_badcase_count == 2 and
   (.data.issue_tag_distribution | any(.key == $tag and .count == 2))' \
  "${smoke_dir}/dashboard.json" >/dev/null

test "$(request GET "/api/v1/pages/dashboard?dataset_id=${dataset_id}" \
  "${member_cookie}" "" "" "${smoke_dir}/comparison.json")" = "200"
jq -e '.data.version_comparison | length == 2 and
  any(.version_no == 1 and .average_score == 4 and .evaluation_badcase_rate == 1) and
  any(.version_no == 2 and .average_score == 3.5 and .evaluation_badcase_rate == 0)' \
  "${smoke_dir}/comparison.json" >/dev/null

test "$(request GET "/api/v1/evaluation-results?scenario_id=${scenario_id}&score=4&page_size=100" \
  "${member_cookie}" "" "" "${smoke_dir}/score-details.json")" = "200"
jq -e --arg run "${run_v1}" --arg result "${v1_result_1}" \
  '.data.total == 1 and .data.items[0].evaluation_run_id == $run and
   .data.items[0].id == $result and .data.items[0].score == 4 and
   (.data.items[0].result_detail_url | contains($result))' \
  "${smoke_dir}/score-details.json" >/dev/null

test "$(request GET "/api/v1/evaluation-results?scenario_id=${scenario_id}&result_status=skipped&page_size=100" \
  "${member_cookie}" "" "" "${smoke_dir}/skip-details.json")" = "200"
jq -e '.data.total == 1 and .data.items[0].result_status == "skipped" and
  .data.items[0].skip_reason == "当前账号无权限"' "${smoke_dir}/skip-details.json" >/dev/null

test "$(request GET "/api/v1/badcases?dataset_id=${dataset_id}&dataset_version_id=${v1_id}&evaluator_id=${member_id}&issue_tag_id=${tag_id}&page_size=100" \
  "${member_cookie}" "" "" "${smoke_dir}/drill-badcases.json")" = "200"
jq -e '.data.total == 1 and .data.items[0].source_type == "evaluation"' \
  "${smoke_dir}/drill-badcases.json" >/dev/null

test "$(request GET "/api/v1/pages/dashboard?scenario_id=${scenario_id}&from=2099-01-01T00:00:00Z" \
  "${member_cookie}" "" "" "${smoke_dir}/empty-dashboard.json")" = "200"
jq -e '.data.metrics.completed_run_count == 0 and
  .data.metrics.evaluated_case_count == 0 and
  .data.metrics.average_score == null and
  .data.metrics.evaluation_badcase_rate == null and
  .data.metrics.valid_badcase_count == 0' "${smoke_dir}/empty-dashboard.json" >/dev/null

test "$(request GET "/api/v1/search?q=${timestamp}&types=target,scenario,dataset,case,evaluation_result,badcase&page_size=100" \
  "${member_cookie}" "" "" "${smoke_dir}/search.json")" = "200"
jq -e '[.data.items[].type] |
  (index("target") != null) and (index("scenario") != null) and
  (index("dataset") != null) and (index("case") != null) and
  (index("evaluation_result") != null) and (index("badcase") != null)' \
  "${smoke_dir}/search.json" >/dev/null
jq -e '.data.items |
  any(.type == "case" and (.url | startswith("/version-cases/"))) and
  any(.type == "evaluation_result" and (.url | contains("result_id=")))' \
  "${smoke_dir}/search.json" >/dev/null

for export in evaluation-results badcases badcase-distribution; do
  test "$(curl -sS -o "${smoke_dir}/${export}.csv" -w "%{http_code}" -b "${member_cookie}" \
    "${api_base}/api/v1/exports/${export}.csv?scenario_id=${scenario_id}")" = "200"
  test "$(xxd -p -l 3 "${smoke_dir}/${export}.csv")" = "efbbbf"
done
test "$(wc -l < "${smoke_dir}/evaluation-results.csv" | tr -d ' ')" = "5"
test "$(wc -l < "${smoke_dir}/badcases.csv" | tr -d ' ')" = "3"
head -n 1 "${smoke_dir}/evaluation-results.csv" | grep -q "评测平均分"
test "$(curl -sS -o "${smoke_dir}/anonymous-export.json" -w "%{http_code}" \
  "${api_base}/api/v1/exports/badcases.csv")" = "401"
test "$(request GET "/api/v1/pages/dashboard?environment=invalid" \
  "${member_cookie}" "" "" "${smoke_dir}/invalid-filter.json")" = "422"

echo "M6 smoke passed: personal home, completed-only metrics, exact drill-down, version comparison, answer search and private streamed BOM CSV exports"
