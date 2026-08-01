ALTER TABLE datasets
    ADD COLUMN evaluation_target_id BINARY(16) NULL AFTER id;

UPDATE datasets d
JOIN scenarios s ON s.id = d.scenario_id
SET d.evaluation_target_id = s.evaluation_target_id;

ALTER TABLE version_cases
    ADD COLUMN scenario_id BINARY(16) NULL AFTER case_key,
    ADD COLUMN scenario_assignment_status VARCHAR(20) NOT NULL DEFAULT 'unclassified' AFTER scenario_id;

UPDATE version_cases vc
JOIN dataset_versions dv ON dv.id = vc.dataset_version_id
JOIN datasets d ON d.id = dv.dataset_id
SET vc.scenario_id = d.scenario_id,
    vc.scenario_assignment_status = 'confirmed';

ALTER TABLE badcases
    ADD COLUMN evaluation_target_id BINARY(16) NULL AFTER source_type,
    ADD COLUMN scenario_assignment_status VARCHAR(20) NOT NULL DEFAULT 'unclassified' AFTER scenario_id;

UPDATE badcases b
JOIN scenarios s ON s.id = b.scenario_id
SET b.evaluation_target_id = s.evaluation_target_id,
    b.scenario_assignment_status = 'confirmed';

ALTER TABLE datasets
    DROP FOREIGN KEY fk_datasets_scenario,
    DROP INDEX uq_datasets_scenario_name,
    DROP INDEX idx_datasets_scenario_status_updated,
    MODIFY COLUMN evaluation_target_id BINARY(16) NOT NULL,
    ADD UNIQUE KEY uq_datasets_target_name (evaluation_target_id, name),
    ADD KEY idx_datasets_target_status_updated (evaluation_target_id, status, updated_at),
    ADD CONSTRAINT fk_datasets_target FOREIGN KEY (evaluation_target_id) REFERENCES evaluation_targets (id) ON DELETE RESTRICT,
    DROP COLUMN scenario_id;

ALTER TABLE version_cases
    ADD KEY idx_version_cases_scenario_status (scenario_id, scenario_assignment_status),
    ADD CONSTRAINT chk_version_cases_scenario_assignment CHECK (
        (scenario_assignment_status = 'unclassified' AND scenario_id IS NULL)
        OR
        (scenario_assignment_status IN ('suggested', 'confirmed') AND scenario_id IS NOT NULL)
    ),
    ADD CONSTRAINT fk_version_cases_scenario FOREIGN KEY (scenario_id) REFERENCES scenarios (id) ON DELETE RESTRICT;

ALTER TABLE badcases
    DROP FOREIGN KEY fk_badcases_scenario;

ALTER TABLE badcases
    MODIFY COLUMN evaluation_target_id BINARY(16) NOT NULL,
    MODIFY COLUMN scenario_id BINARY(16) NULL,
    ADD KEY idx_badcases_target_valid_status_time (evaluation_target_id, invalidated_at, status, occurred_at),
    ADD CONSTRAINT chk_badcases_scenario_assignment CHECK (
        (scenario_assignment_status = 'unclassified' AND scenario_id IS NULL)
        OR
        (scenario_assignment_status IN ('suggested', 'confirmed') AND scenario_id IS NOT NULL)
    ),
    ADD CONSTRAINT fk_badcases_target FOREIGN KEY (evaluation_target_id) REFERENCES evaluation_targets (id) ON DELETE RESTRICT;

ALTER TABLE badcases
    ADD CONSTRAINT fk_badcases_scenario FOREIGN KEY (scenario_id) REFERENCES scenarios (id) ON DELETE RESTRICT;
