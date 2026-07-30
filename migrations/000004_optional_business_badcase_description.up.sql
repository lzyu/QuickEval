ALTER TABLE badcases
    DROP CHECK chk_badcases_source_result,
    ADD CONSTRAINT chk_badcases_source_result CHECK (
        (source_type = 'evaluation' AND case_result_id IS NOT NULL)
        OR
        (source_type = 'business' AND case_result_id IS NULL)
    );
