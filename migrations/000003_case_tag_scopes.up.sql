ALTER TABLE case_tags
    DROP FOREIGN KEY fk_case_tags_scenario,
    DROP INDEX uq_case_tags_scenario_name,
    DROP INDEX idx_case_tags_scenario_status_sort,
    ADD COLUMN scope VARCHAR(16) NOT NULL DEFAULT 'scenario' AFTER id,
    MODIFY COLUMN scenario_id BINARY(16) NULL,
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
    );

ALTER TABLE case_tags
    ADD CONSTRAINT fk_case_tags_scenario
    FOREIGN KEY (scenario_id) REFERENCES scenarios (id) ON DELETE RESTRICT;
