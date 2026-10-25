-- Абсолютный срок жизни сессии (V3.3): expires_at теперь «дельлайн простоя»,
-- а created_at задаёт верхнюю границу, которую скольжение не отодвигает.
-- DEFAULT 0 означает «сессия до миграции»: такие записи пересоздаются, иначе
-- они остались бы бессмертными по абсолютной проверке.
ALTER TABLE sessions ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;

UPDATE sessions SET created_at = CAST(strftime('%s', 'now') AS INTEGER) WHERE created_at = 0;

-- Одноразовые токены сброса пароля (Cheat Sheet Forgot Password): хранится хэш,
-- короткий срок, признаки создания и использования. Каскад чистит токены вместе
-- с аккаунтом.
CREATE TABLE IF NOT EXISTS password_resets (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    used_at    INTEGER
);

CREATE INDEX IF NOT EXISTS idx_password_resets_user ON password_resets (user_id);
