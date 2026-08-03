ALTER TABLE users
    DROP CHECK chk_users_role;

UPDATE users
SET role = 'super_admin'
WHERE id = (
    SELECT selected_admin.id
    FROM (
        SELECT id
        FROM users
        WHERE role = 'admin'
        ORDER BY (status = 'active') DESC, created_at ASC, id ASC
        LIMIT 1
    ) AS selected_admin
);

UPDATE users
SET role = 'operator'
WHERE role = 'admin';

ALTER TABLE users
    ADD CONSTRAINT chk_users_role CHECK (role IN ('member', 'operator', 'super_admin'));
