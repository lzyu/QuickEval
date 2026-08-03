ALTER TABLE audit_logs
    ADD COLUMN actor_username VARCHAR(64) NULL AFTER actor_user_id,
    ADD COLUMN subject_username VARCHAR(64) NULL AFTER entity_id;

UPDATE audit_logs AS audit_log
LEFT JOIN user_identities AS actor_identity
    ON actor_identity.user_id = audit_log.actor_user_id
    AND actor_identity.provider = 'local'
LEFT JOIN user_identities AS subject_identity
    ON subject_identity.user_id = audit_log.entity_id
    AND subject_identity.provider = 'local'
    AND audit_log.entity_type = 'user'
SET audit_log.actor_username = actor_identity.provider_subject,
    audit_log.subject_username = subject_identity.provider_subject;
