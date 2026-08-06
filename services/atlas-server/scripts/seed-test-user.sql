-- Test user for atlas-server (password: 123456)
-- Run after init-mysql.sql:
--   mysql -u root -p atlas < scripts/seed-test-user.sql

USE atlas;

INSERT INTO users (
    user_id,
    email,
    password_hash,
    first_name,
    last_name,
    principal_type,
    principal_id,
    machine_code
) VALUES (
    'atlas-test-user',
    'test@atlas.local',
    '$2b$10$7txJUyD7pPLQxgVYYHV4BeF6Tnu8uU1AIbEaVWnropemyzc4HBpSa',
    'Atlas',
    'Test',
    'User',
    'atlas-test-user',
    'TEST-5678'
) ON DUPLICATE KEY UPDATE
    email = VALUES(email),
    password_hash = VALUES(password_hash),
    first_name = VALUES(first_name),
    last_name = VALUES(last_name),
    principal_type = VALUES(principal_type),
    principal_id = VALUES(principal_id),
    machine_code = VALUES(machine_code);
