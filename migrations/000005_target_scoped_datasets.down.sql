ALTER TABLE badcases
    DROP FOREIGN KEY fk_badcases_target,
    DROP FOREIGN KEY fk_badcases_scenario,
    DROP CHECK chk_badcases_scenario_assignment,
    DROP INDEX idx_badcases_target_valid_status_time,
    DROP COLUMN scenario_assignment_status,
    DROP COLUMN evaluation_target_id,
    MODIFY COLUMN scenario_id BINARY(16) NOT NULL;

ALTER TABLE badcases
    ADD CONSTRAINT fk_badcases_scenario FOREIGN KEY (scenario_id) REFERENCES scenarios (id) ON DELETE RESTRICT;

ALTER TABLE datasets
    ADD COLUMN scenario_id BINARY(16) NULL AFTER id;

UPDATE datasets d
SET d.scenario_id = (
    SELECT MIN(vc.scenario_id)
    FROM dataset_versions dv
    JOIN version_cases vc ON vc.dataset_version_id = dv.id
    WHERE dv.dataset_id = d.id
      AND vc.scenario_id IS NOT NULL
);

UPDATE datasets d
SET d.scenario_id = (
    SELECT MIN(s.id)
    FROM scenarios s
    WHERE s.evaluation_target_id = d.evaluation_target_id
)
WHERE d.scenario_id IS NULL;

ALTER TABLE version_cases
    DROP FOREIGN KEY fk_version_cases_scenario,
    DROP CHECK chk_version_cases_scenario_assignment,
    DROP INDEX idx_version_cases_scenario_status,
    DROP COLUMN scenario_assignment_status,
    DROP COLUMN scenario_id;

ALTER TABLE datasets
    DROP FOREIGN KEY fk_datasets_target,
    DROP INDEX uq_datasets_target_name,
    DROP INDEX idx_datasets_target_status_updated,
    MODIFY COLUMN scenario_id BINARY(16) NOT NULL,
    ADD UNIQUE KEY uq_datasets_scenario_name (scenario_id, name),
    ADD KEY idx_datasets_scenario_status_updated (scenario_id, status, updated_at),
    ADD CONSTRAINT fk_datasets_scenario FOREIGN KEY (scenario_id) REFERENCES scenarios (id) ON DELETE RESTRICT,
    DROP COLUMN evaluation_target_id;
