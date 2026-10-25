-- Пользователи. Пароль хранится только как bcrypt-хэш, открытого пароля в базе нет.
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Сессии: cookie хранит токен, а здесь лежит его sha256-хэш — если кто-то
-- получит дамп базы, он не сможет подставить чужой токен.
-- expires_at в unix-секундах: срок легко сравнить с time.Now().Unix(),
-- а без такой проверки сессия из cookie была бы бессрочной.
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id);

-- Каждая трата принадлежит одному пользователю: без user_id список был бы общим.
ALTER TABLE expenses ADD COLUMN user_id INTEGER REFERENCES users (id);

CREATE INDEX IF NOT EXISTS idx_expenses_user ON expenses (user_id);
