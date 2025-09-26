# KLIRO Backend Setup Guide

## Ошибка подключения к базе данных

Если вы видите ошибку:
```
failed to connect to postgres: cannot parse `host= user= password=xxxxx dbname= port= sslmode=disable`: invalid port
```

Это означает, что переменные окружения для базы данных не настроены.

## Решение

### 1. Создайте файл `.env` в корне проекта:

```env
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=kliro_db

# JWT Secret
JWT_SECRET=your_jwt_secret_key_here

# Server Port
PORT=8080

# SMTP Configuration (for emails)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your_email@gmail.com
SMTP_PASS=your_email_password

# Google OAuth
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
GOOGLE_REDIRECT_URI=http://localhost:8080/auth/google/callback

# SMS Service (Eskiz)
ESKIZ_EMAIL=your_eskiz_email
ESKIZ_PASSWORD=your_eskiz_password

# NeoInsurance API
NEO_BASE_URL=https://api.neoinsurance.uz
NEO_LOGIN=your_neoinsurance_username
NEO_PASSWORD=your_neoinsurance_password

# Trust Insurance API
TRUST_BASE_URL=https://api.online-trust.uz
TRUST_LOGIN=your_trust_username
TRUST_PASSWORD=your_trust_password
```

### 2. Настройте PostgreSQL базу данных:

```bash
# Создайте базу данных
createdb kliro_db

# Или через psql:
psql -U postgres
CREATE DATABASE kliro_db;
```

### 3. Запустите миграции:

```bash
go run main.go
```

## Обязательные переменные

Минимально необходимые переменные для запуска:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=kliro_db
JWT_SECRET=your_jwt_secret_key_here
PORT=8080
```

## Дополнительные настройки

- **SMTP** - для отправки email уведомлений
- **Google OAuth** - для входа через Google
- **Eskiz** - для отправки SMS
- **NeoInsurance/Trust Insurance** - для интеграции со страховыми API

## Проверка

После настройки .env файла запустите:

```bash
go run main.go
```

Вы должны увидеть:
```
Connected to PostgreSQL
==========================================
НАЧИНАЕМ НАСТРОЙКУ РОУТЕРА!
==========================================
```
