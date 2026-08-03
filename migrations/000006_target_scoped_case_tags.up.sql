ALTER TABLE case_tags
    DROP FOREIGN KEY fk_case_tags_scenario,
    DROP CHECK chk_case_tags_scope,
    DROP INDEX uq_case_tags_scope_name,
    DROP INDEX idx_case_tags_scope_status_sort,
    DROP INDEX idx_case_tags_scenario_status_sort,
    DROP COLUMN scope_owner_id,
    ADD COLUMN evaluation_target_id BINARY(16) NULL AFTER scope;

UPDATE case_tags
JOIN scenarios ON scenarios.id = case_tags.scenario_id
SET case_tags.scope = 'target',
    case_tags.evaluation_target_id = scenarios.evaluation_target_id
WHERE case_tags.scope = 'scenario';

ALTER TABLE case_tags
    DROP COLUMN scenario_id,
    ADD KEY idx_case_tags_scope_status_sort (scope, status, sort_order),
    ADD KEY idx_case_tags_target_status_sort (evaluation_target_id, status, sort_order),
    ADD CONSTRAINT chk_case_tags_scope CHECK (
        (scope = 'global' AND evaluation_target_id IS NULL)
        OR
        (scope = 'target' AND evaluation_target_id IS NOT NULL)
    ),
    ADD CONSTRAINT fk_case_tags_target
    FOREIGN KEY (evaluation_target_id) REFERENCES evaluation_targets (id) ON DELETE RESTRICT;
