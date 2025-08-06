package migrations

const createTransferTables = `
-- Создание таблицы для новых переводов
CREATE TABLE IF NOT EXISTS new_transfer (
    id SERIAL PRIMARY KEY,
    app_name VARCHAR(100) NOT NULL,
    commission VARCHAR(50) NOT NULL,
    limit_ru TEXT NULL,
    limit_uz TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);



-- Создание индексов для оптимизации
CREATE INDEX IF NOT EXISTS idx_new_transfer_app_name ON new_transfer(app_name);
CREATE INDEX IF NOT EXISTS idx_new_transfer_created_at ON new_transfer(created_at);

`
