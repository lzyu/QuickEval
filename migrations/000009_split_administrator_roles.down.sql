ALTER TABLE users
    DROP CHECK chk_users_role;

UPDATE users
SET role = 'admin'
WHERE role IN ('operator', 'super_admin');

ALTER TABLE users
    ADD CONSTRAINT chk_users_role CHECK (role IN ('member', 'admin'));
