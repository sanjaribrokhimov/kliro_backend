# Orders API - Документация

## Base URL
```
http://localhost:8080
```

## Авторизация
Все запросы требуют JWT токен:
```
Authorization: Bearer YOUR_JWT_TOKEN
```

## Эндпоинты

### 1. Создание заказа
**POST** `/user/orders/create`

```json
{
  "order_id": "ORDER-001",
  "category": "neoInsurance",
  "company_name": "NeoInsurance",
  "status": "pending"
}
```

### 2. Получение заказов
**GET** `/user/orders/my-orders`

Query параметры:
- `page` - номер страницы
- `limit` - количество записей
- `category` - фильтр по категории
- `status` - фильтр по статусу
- `company` - фильтр по компании

### 3. Получение заказа по ID
**GET** `/user/orders/{order_id}`

### 4. Обновление статуса
**PUT** `/user/orders/{order_id}/status`

```json
{
  "status": "completed"
}
```

### 5. Статистика заказов
**GET** `/user/orders/my-stats`

### 6. Удаление заказа
**DELETE** `/user/orders/{order_id}`

## Примеры cURL

### Создание заказа
```bash
curl -X POST http://localhost:8080/user/orders/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "order_id": "ORDER-001",
    "category": "insurance",
    "company_name": "NeoInsurance",
    "status": "pending"
  }'
```

### Получение заказов
```bash
curl -X GET http://localhost:8080/user/orders/my-orders \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Получение с фильтрами
```bash
curl -X GET "http://localhost:8080/user/orders/my-orders?category=neoInsurance&status=pending" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Обновление статуса
```bash
curl -X PUT http://localhost:8080/user/orders/ORDER-001/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"status": "completed"}'
```

### Статистика
```bash
curl -X GET http://localhost:8080/user/orders/my-stats \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Ответы

### Успешный ответ
```json
{
  "result": { ... },
  "success": true
}
```

### Ошибка
```json
{
  "error": "Описание ошибки"
}
```

## Статусы заказов
- `pending` - ожидает
- `processing` - в процессе
- `completed` - завершен
- `cancelled` - отменен
- `failed` - неудачный

## Категории
- `neoInsurance` - страхование
- `banking` - банковские услуги
- `travel` - путешествия
- `hotel` - отели
- `avia` - авиабилеты