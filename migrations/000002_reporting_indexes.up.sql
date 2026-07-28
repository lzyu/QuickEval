CREATE INDEX idx_badcases_valid_status_time
    ON badcases (invalidated_at, status, occurred_at);

CREATE INDEX idx_evaluation_runs_evaluator_status_updated
    ON evaluation_runs (evaluator_id, status, updated_at);
