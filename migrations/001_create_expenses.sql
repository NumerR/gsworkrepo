-- amount хранится в копейках, чтобы не терять точность на дробных суммах
-- date хранится в формате YYYY-MM-DD
CREATE TABLE IF NOT EXISTS expenses (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    amount      INTEGER NOT NULL,
    description TEXT    NOT NULL,
    date        TEXT    NOT NULL
);
