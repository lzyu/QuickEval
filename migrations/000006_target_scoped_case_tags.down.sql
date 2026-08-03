ALTER TABLE case_tags
    DROP FOREIGN KEY fk_case_tags_target,
    DROP CHECK chk_case_tags_scope,
    DROP INDEX idx_case_tags_scope_status_sort,
    DROP INDEX idx_case_tags_target_status_sort,
    ADD COLUMN scenario_id BINARY(16) NULL AFTER scope;

UPDATE case_tags
JOIN scenarios ON scenarios.evaluation_target_id = case_tags.evaluation_target_id
SET case_tags.scope = 'scenario',
    case_tags.scenario_id = scenarios.id
WHERE case_tags.scope = 'target'
  AND scenarios.id = (
      SELECT scenario_id FROM (
          SELECT MIN(id) AS scenario_id
          FROM scenarios
          GROUP BY evaluation_target_id
      ) AS first_scenarios
      WHERE first_scenarios.evaluation_target_id = case_tags.evaluation_target_id
  );

ALTER TABLE case_tags
    DROP COLUMN evaluation_target_id,
    ADD COLUMN scope_owner_id BINARY(16)
        GENERATED ALWAYS AS (
            IFNULL(scenario_id, 0x00000000000000000000000000000000)
        ) STORED AFTER scenario_id,
    ADD UNIQUE KEY uq_case_tags_scope_name (scope_owner_id, name),
    ADD KEY idx_case_tags_scope_status_sort (scope, status, sort_order),
    ADD KEY idx_case_tags_scenario_status_sort (scenario_id, status, sort_order),
    ADD CONSTRAINT chk_case_tags_scope CHECK (
        (scope = 'global' AND scenario_id IS NULL)
        OR
        (scope = 'scenario' AND scenario_id IS NOT NULL)
    ),
    ADD CONSTRAINT fk_case_tags_scenario
    FOREIGN KEY (scenario_id) REFERENCES scenarios (id) ON DELETE RESTRICT;
