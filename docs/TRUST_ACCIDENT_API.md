# Trust Accident Insurance API

API для страхования от несчастных случаев (Trust Insurance)

## 🔐 Авторизация

API использует Basic Auth с credentials из `.env`:
- `TRUST_LOGIN`
- `TRUST_PASSWORD`

## 📋 Процесс оформления страховки

### Шаг 1: Получить тарифы
```bash
GET /trust-insurance/accident/tarifs
```

### Шаг 2: Создать заявку
```bash
POST /trust-insurance/accident/create
```

**Request:**
```json
{
  "start_date": "2024-11-10",
  "tariff_id": 1,
  "person": {
    "pinfl": "30101995750028",
    "pass_sery": "AB",
    "pass_num": "0160608",
    "date_birth": "1999-01-01",
    "last_name": "Rasulov",
    "first_name": "Bunyod",
    "patronym_name": "Ravshan o`g`li",
    "oblast": 10,
    "rayon": 1012,
    "phone": "998123456789",
    "address": "Tashkent, Yunusabad district"
  }
}
```

**Response:**
```json
{
  "result": {
    "result": 0,
    "result_message": "Successful request processing",
    "anketa_id": 46622,
    "insurance_premium": 3000000,
    "insurance_otv": 15000,
    "payment_urls": {
      "click": "https://my.click.uz/pay?service_id=23572&merchant_id=14417&amount=3000000&transaction_param=46622&return_url=https://kliro.uz/payment/return",
      "payme": "https://checkout.paycom.uz/646c8bff2cb83937a7551c95?amount=3000000&account[order_id]=46622"
    }
  }
}
```

### Шаг 3: Проверить статус оплаты
```bash
POST /trust-insurance/accident/check-payment
```

**Request:**
```json
{
  "anketa_id": 46622,
  "lan": "uz"
}
```

**Response (успешная оплата):**
```json
{
  "result": {
    "result": 0,
    "result_message": "Successful request processing",
    "policy_id": "8966133",
    "policy_sery": "NS",
    "policy_number": 4213,
    "status_policy": "2",
    "url": "http://api.online-trust.uz/users/policy/download/8966133",
    "url_napp": "https://ersp.e-osgo.uz/site/export-to-pdf?id=UUIDPOLISA",
    "status_payment": "2",
    "payment_type": "9"
  }
}
```

## 📊 Статусы полиса (status_policy)

| Код | Описание |
|-----|----------|
| 2   | ВЫДАН (успешно) |
| 3   | БРАКОВАН |
| 4   | УТЕРЯН |
| 8   | РАСТОРГНУТ |

## 💳 Статусы оплаты (status_payment)

| Код | Описание |
|-----|----------|
| 1   | НЕОПЛАЧЕННЫЙ |
| 2   | ОПЛАЧЕННЫЙ |

## 💰 Типы оплаты (payment_type)

| Код | Описание |
|-----|----------|
| 3   | ДЕНЬГИ У АГЕНТА |
| 8   | PayMe |
| 9   | Click |
| 10  | Paynet |

## 🔧 Параметры конфигурации (.env)

```env
# Trust Insurance
TRUST_BASE_URL=https://api.online-trust.uz
TRUST_LOGIN=your_login
TRUST_PASSWORD=your_password

# Payment Systems
CLICK_SERVICE_ID=23572
CLICK_MERCHANT_ID=14417
PAYME_MERCHANT_ID=646c8bff2cb83937a7551c95
PAYMENT_RETURN_URL=https://kliro.uz/payment/return
```

## 📱 Примеры использования

### Получить тарифы
```bash
curl http://localhost:8080/trust-insurance/accident/tarifs
```

### Создать заявку
```bash
curl -X POST http://localhost:8080/trust-insurance/accident/create \
  -H "Content-Type: application/json" \
  -d '{
    "start_date": "2024-11-10",
    "tariff_id": 1,
    "person": {
      "pinfl": "30101995750028",
      "pass_sery": "AB",
      "pass_num": "0160608",
      "date_birth": "1999-01-01",
      "last_name": "Rasulov",
      "first_name": "Bunyod",
      "patronym_name": "Ravshan o`g`li",
      "oblast": 10,
      "rayon": 1012,
      "phone": "998123456789",
      "address": "Tashkent, Yunusabad district"
    }
  }'
```

### Проверить статус
```bash
curl -X POST http://localhost:8080/trust-insurance/accident/check-payment \
  -H "Content-Type: application/json" \
  -d '{
    "anketa_id": 46622,
    "lan": "uz"
  }'
```

## ⚠️ Важные замечания

1. **anketa_id** - уникальный идентификатор заявки, сохраните его после создания
2. **payment_urls** - ссылки для оплаты через Click и Payme
3. **status_policy = 2** и **status_payment = 2** - полис готов к скачиванию
4. **url** и **url_napp** - ссылки для скачивания полиса (доступны после оплаты)
5. Все поля в `person` обязательны
6. Формат дат: `YYYY-MM-DD`
7. Формат телефона: `998XXXXXXXXX`
8. ПИНФЛ: 14 цифр

## 🎯 Логика работы

1. Пользователь получает список тарифов
2. Выбирает подходящий тариф
3. Заполняет данные и создает заявку
4. Получает ссылки для оплаты (Click/Payme)
5. Производит оплату через выбранную систему
6. Периодически проверяет статус заявки
7. Когда полис готов (status_policy=2, status_payment=2), скачивает его

