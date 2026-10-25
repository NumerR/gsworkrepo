-- Справочник категорий. name UNIQUE, чтобы опции select не плодились дублями.
CREATE TABLE IF NOT EXISTS categories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO categories (name) VALUES
    ('Еда'),
    ('Транспорт'),
    ('Жильё'),
    ('Развлечения'),
    ('Здоровье'),
    ('Прочее');

-- Внешний ключ на categories: трата либо в категории, либо NULL («без категории»).
ALTER TABLE expenses ADD COLUMN category_id INTEGER REFERENCES categories (id);

-- Траты, добавленные до появления категорий, относим к «Прочее».
UPDATE expenses
SET category_id = (SELECT id FROM categories WHERE name = 'Прочее')
WHERE category_id IS NULL;
