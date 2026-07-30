ALTER TABLE case_tags
    DROP FOREIGN KEY fk_case_tags_scenario,
    DROP CHECK chk_case_tags_scope,
    DROP INDEX uq_case_tags_scope_name,
    DROP INDEX idx_case_tags_scope_status_sort,
    DROP INDEX idx_case_tags_scenario_status_sort,
    DROP COLUMN scope_owner_id,
    DROP COLUMN scope,
    MODIFY COLUMN scenario_id BINARY(16) NOT NULL,
    ADD UNIQUE KEY uq_case_tags_scenario_name (scenario_id, name),
    ADD KEY idx_case_tags_scenario_status_sort (scenario_id, status, sort_order);

ALTER TABLE case_tags
    ADD CONSTRAINT fk_case_tags_scenario
    FOREIGN KEY (scenario_id) REFERENCES scenarios (id) ON DELETE RESTRICT;
