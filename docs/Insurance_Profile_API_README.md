# Insurance Profile API

API для управления профилем страховки пользователя.

## Эндпоинты

### 1. Создание профиля страховки
**POST** `/api/insurance-profile`

**Заголовки:**
- `Authorization: Bearer <access_token>`
- `Content-Type: application/json`

**Тело запроса:**
```json
{
    "product": "OSAGO",
    "date": "2024-01-15", 
    "order_id": "ORD-12345",
    "amount": 150000.50,
    "is_paid": true,
    "document_url": "https://example.com/documents/insurance-policy.pdf"
}
```

**Альтернативные форматы даты:**
```json
{
    "product": "KASKO",
    "date": "2024/01/15", 
    "order_id": "ORD-67890",
    "amount": 250000.00,
    "is_paid": false
}
```

**Ответ:**
```json
{
    "result": {
        "id": 1,
        "user_id": 123,
        "product": "OSAGO",
        "date": "2024-01-15T00:00:00Z",
        "order_id": "ORD-12345", 
        "amount": 150000.50,
        "is_paid": true,
        "document_url": "https://example.com/documents/insurance-policy.pdf",
        "created_at": "2024-01-01T12:00:00Z",
        "updated_at": "2024-01-01T12:00:00Z"
    },
    "success": true
}
```

### 2. Получение всех профилей страховки
**GET** `/api/insurance-profile`

**Заголовки:**
- `Authorization: Bearer <access_token>`

**Параметры запроса:**
- `page` (опционально) - номер страницы (по умолчанию: 1)
- `limit` (опционально) - количество записей на странице (по умолчанию: 10, максимум: 100)
- `product` (опционально) - фильтр по продукту

**Примеры:**
- `/api/insurance-profile` - все профили
- `/api/insurance-profile?page=1&limit=5` - первая страница, 5 записей
- `/api/insurance-profile?product=OSAGO` - только профили OSAGO

**Ответ:**
```json
{
    "result": {
        "profiles": [
            {
                "id": 1,
                "user_id": 123,
                "product": "OSAGO",
                "date": "2024-01-15T00:00:00Z",
                "order_id": "ORD-12345",
                "amount": 150000.50,
                "is_paid": true,
                "document_url": "https://example.com/documents/insurance-policy.pdf",
                "created_at": "2024-01-01T12:00:00Z",
                "updated_at": "2024-01-01T12:00:00Z"
            }
        ],
        "total": 1,
        "page": 1,
        "limit": 10,
        "total_pages": 1
    },
    "success": true
}
```

### 3. Получение профиля страховки по ID
**GET** `/api/insurance-profile/{id}`

**Заголовки:**
- `Authorization: Bearer <access_token>`

**Ответ:**
```json
{
    "result": {
        "id": 1,
        "user_id": 123,
        "product": "OSAGO",
        "date": "2024-01-15T00:00:00Z",
        "order_id": "ORD-12345",
        "amount": 150000.50,
        "is_paid": true,
        "document_url": "https://example.com/documents/insurance-policy.pdf",
        "created_at": "2024-01-01T12:00:00Z",
        "updated_at": "2024-01-01T12:00:00Z"
    },
    "success": true
}
```

## Поля API

- **product** (обязательно) - продукт страховки. Разрешенные значения: **KASKO**, **OSAGO**, **TRAVEL**, **ACCIDENT**
- **date** (обязательно) - дата в одном из поддерживаемых форматов:
  - YYYY-MM-DD (например: "2024-01-15")
  - YYYY/MM/DD (например: "2024/01/15")
  - DD-MM-YYYY (например: "15-01-2024")
  - DD/MM/YYYY (например: "15/01/2024")
  - MM-DD-YYYY (например: "01-15-2024")
  - MM/DD/YYYY (например: "01/15/2024")
- **order_id** (обязательно) - ID заказа
- **amount** (обязательно) - сумма оплаты
- **is_paid** (опционально) - статус оплаты (true/false, по умолчанию: false)
- **document_url** (опционально) - ссылка на документ

## Коды ошибок

- `400` - Неверный запрос (отсутствуют обязательные поля, неверный формат даты)
- `401` - Не авторизован (отсутствует или неверный токен)
- `404` - Профиль не найден
- `500` - Внутренняя ошибка сервера

## Аутентификация

Все эндпоинты требуют JWT токен в заголовке `Authorization: Bearer <access_token>`.

Для получения токена используйте эндпоинт `/auth/login`.

## Postman коллекция

Используйте файл `Insurance_Profile_API.postman_collection.json` для тестирования API.

Коллекция включает:
1. Аутентификацию (логин)
2. Создание профиля страховки
3. Получение всех профилей
4. Получение профиля по ID
5. Тесты ошибок
6. Автоматическое сохранение токенов