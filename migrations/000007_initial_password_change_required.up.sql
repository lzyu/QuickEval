ALTER TABLE user_identities
    ADD COLUMN password_change_required BOOLEAN NOT NULL DEFAULT FALSE AFTER password_hash;
