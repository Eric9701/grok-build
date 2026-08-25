-- Atlas Server schema (MySQL 8+)
-- Matches internal/store/mysql.go Migrate()

CREATE DATABASE IF NOT EXISTS atlas
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE atlas;

-- Optional: create app user (adjust host/password for production)
CREATE USER IF NOT EXISTS 'atlas'@'%' IDENTIFIED BY 'atlas';
GRANT ALL PRIVILEGES ON atlas.* TO 'atlas'@'%';
FLUSH PRIVILEGES;

-- ---------------------------------------------------------------------------
-- users: account login + CLI machine code
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    user_id         VARCHAR(64)   NOT NULL COMMENT 'Primary id, maps to principalId',
    email           VARCHAR(255)  NOT NULL COMMENT 'Login email, unique',
    password_hash   VARCHAR(255)  NOT NULL COMMENT 'bcrypt hash',
    first_name      VARCHAR(128)  NOT NULL DEFAULT '',
    last_name       VARCHAR(128)  NOT NULL DEFAULT '',
    principal_type  VARCHAR(32)   NOT NULL DEFAULT 'User',
    principal_id    VARCHAR(64)   NOT NULL COMMENT 'Usually same as user_id',
    machine_code    VARCHAR(32)   NULL     COMMENT 'Per-user code for device approval, e.g. ABCD-EFGH',
    created_at      TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (user_id),
    UNIQUE KEY uk_users_email (email),
    UNIQUE KEY uk_users_machine_code (machine_code),
    KEY idx_users_machine_code (machine_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- sessions: web login cookies (atlas_session)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sessions (
    token       VARCHAR(64)  NOT NULL COMMENT 'Random hex session token',
    user_id     VARCHAR(64)  NOT NULL,
    expires_at  TIMESTAMP    NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (token),
    KEY idx_sessions_user (user_id),
    KEY idx_sessions_expires (expires_at),
    CONSTRAINT fk_sessions_user
        FOREIGN KEY (user_id) REFERENCES users (user_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- refresh_tokens: CLI OAuth refresh tokens (must survive atlas-server restart)
-- Device codes stay in-process; only refresh is durable.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS refresh_tokens (
    token       VARCHAR(128) NOT NULL COMMENT 'Opaque refresh token issued at /oauth2/token',
    user_id     VARCHAR(64)  NOT NULL,
    email       VARCHAR(255) NOT NULL DEFAULT '',
    client_id   VARCHAR(255) NOT NULL DEFAULT '',
    scope       VARCHAR(512) NOT NULL DEFAULT '',
    expires_at  TIMESTAMP    NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (token),
    KEY idx_refresh_tokens_user (user_id),
    KEY idx_refresh_tokens_expires (expires_at),
    CONSTRAINT fk_refresh_tokens_user
        FOREIGN KEY (user_id) REFERENCES users (user_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- telemetry_traces: OTLP batches from CLI, attributed per user
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS telemetry_traces (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id      VARCHAR(64)     NULL COMMENT 'From JWT sub or x-userid',
    email        VARCHAR(255)    NULL,
    team_id      VARCHAR(64)     NULL COMMENT 'From x-teamid header',
    content_type VARCHAR(128)    NOT NULL DEFAULT '',
    body         LONGBLOB        NOT NULL COMMENT 'Raw OTLP protobuf/json body',
    body_size    INT UNSIGNED    NOT NULL DEFAULT 0,
    client_ip    VARCHAR(64)     NULL,
    created_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    KEY idx_traces_user_created (user_id, created_at),
    KEY idx_traces_email_created (email, created_at),
    KEY idx_traces_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- task_reports: per-subagent-task reports from CLI (what task, which agent,
-- what artifacts), attributed per user
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS task_reports (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id           VARCHAR(64)     NULL COMMENT 'Report User from JSON body (userId)',
    email             VARCHAR(255)    NULL,
    team_id           VARCHAR(64)     NULL COMMENT 'From x-teamid header',
    subagent_id       VARCHAR(128)    NULL,
    parent_session_id VARCHAR(128)    NULL,
    child_session_id  VARCHAR(128)    NULL,
    subagent_type     VARCHAR(128)    NOT NULL DEFAULT '' COMMENT 'Agent that handled the task',
    model             VARCHAR(128)    NULL COMMENT 'Catalog ID (picker / config.toml key)',
    model_routing     VARCHAR(128)    NULL COMMENT 'Routing Name (model.model) in plaintext',
    description       TEXT            NULL COMMENT 'Short task description',
    prompt            MEDIUMTEXT      NULL COMMENT 'Task prompt (truncated to ~4 KiB)',
    status            VARCHAR(32)     NOT NULL DEFAULT '' COMMENT 'completed|cancelled|error',
    success           TINYINT(1)      NOT NULL DEFAULT 0,
    duration_ms       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    tool_calls        INT UNSIGNED    NOT NULL DEFAULT 0,
    turns             INT UNSIGNED    NOT NULL DEFAULT 0,
    tokens_used       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    artifacts         JSON            NULL COMMENT 'Array of {path, kind} produced files',
    artifact_count    INT UNSIGNED    NOT NULL DEFAULT 0,
    cwd               VARCHAR(1024)   NULL,
    worktree_path     VARCHAR(1024)   NULL,
    error             TEXT            NULL,
    started_at        VARCHAR(40)     NULL COMMENT 'RFC3339',
    completed_at      VARCHAR(40)     NULL COMMENT 'RFC3339',
    client_ip         VARCHAR(64)     NULL,
    client_version    VARCHAR(64)     NULL COMMENT 'Compiled CLI version from report body',
    created_at        TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    KEY idx_reports_user_created (user_id, created_at),
    KEY idx_reports_email_created (email, created_at),
    KEY idx_reports_agent (subagent_type),
    KEY idx_reports_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- session_signals: cumulative session metrics from CLI
-- POST /v1/sessions/{sessionId}/signals
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_signals (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id           VARCHAR(64)     NULL COMMENT 'From JWT sub or x-userid',
    email             VARCHAR(255)    NULL,
    team_id           VARCHAR(64)     NULL COMMENT 'From x-teamid header',
    session_id        VARCHAR(128)    NOT NULL,
    client_type       VARCHAR(64)     NULL,
    total_turns       BIGINT          NOT NULL DEFAULT 0,
    tool_call_count   BIGINT          NOT NULL DEFAULT 0,
    error_count       BIGINT          NOT NULL DEFAULT 0,
    primary_model_id  VARCHAR(128)    NULL,
    payload           JSON            NOT NULL COMMENT 'Full SessionSignalsUpdate JSON',
    client_ip         VARCHAR(64)     NULL,
    created_at        TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    KEY idx_signals_session_created (session_id, created_at),
    KEY idx_signals_user_created (user_id, created_at),
    KEY idx_signals_email_created (email, created_at),
    KEY idx_signals_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- managed_models: cloud catalog entries (api_key stored as ENC(...))
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS managed_models (
    id              VARCHAR(128)  NOT NULL,
    model           VARCHAR(128)  NOT NULL,
    name            VARCHAR(255)  NOT NULL DEFAULT '',
    description     TEXT          NULL,
    base_url        VARCHAR(1024) NOT NULL DEFAULT '',
    api_backend     VARCHAR(64)   NOT NULL DEFAULT 'messages',
    api_key_enc     TEXT          NOT NULL,
    context_window  BIGINT        NOT NULL DEFAULT 200000,
    owned_by        VARCHAR(64)   NOT NULL DEFAULT 'atlas',
    enabled         TINYINT(1)    NOT NULL DEFAULT 1,
    extra_json      JSON          NULL,
    created_at      TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_models (
    user_id     VARCHAR(64)  NOT NULL,
    model_id    VARCHAR(128) NOT NULL,
    is_default  TINYINT(1)   NOT NULL DEFAULT 0,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, model_id),
    KEY idx_user_models_model (model_id),
    CONSTRAINT fk_user_models_user FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT fk_user_models_model FOREIGN KEY (model_id) REFERENCES managed_models(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- user_groups: named sets for Group Assignment of managed models
-- name UNIQUE under utf8mb4_unicode_ci ⇒ case-insensitive
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_groups (
    group_id    VARCHAR(64)  NOT NULL,
    name        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id),
    UNIQUE KEY uk_user_groups_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS group_members (
    group_id    VARCHAR(64)  NOT NULL,
    user_id     VARCHAR(64)  NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id),
    KEY idx_group_members_user (user_id),
    CONSTRAINT fk_group_members_group FOREIGN KEY (group_id) REFERENCES user_groups(group_id) ON DELETE CASCADE,
    CONSTRAINT fk_group_members_user FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS group_models (
    group_id    VARCHAR(64)  NOT NULL,
    model_id    VARCHAR(128) NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, model_id),
    KEY idx_group_models_model (model_id),
    CONSTRAINT fk_group_models_group FOREIGN KEY (group_id) REFERENCES user_groups(group_id) ON DELETE CASCADE,
    CONSTRAINT fk_group_models_model FOREIGN KEY (model_id) REFERENCES managed_models(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- Test user (password: 123456)
-- Login: test@atlas.local / 123456
-- Machine code for device approval: TEST-5678
-- Or run: mysql -u root -p atlas < scripts/seed-test-user.sql
-- ---------------------------------------------------------------------------
INSERT INTO users (
    user_id, email, password_hash, first_name, last_name,
    principal_type, principal_id, machine_code
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
    password_hash = VALUES(password_hash),
    machine_code = VALUES(machine_code);
