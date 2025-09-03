# Тестирование API Авиабилетов

## Проверка состояния сервиса

```bash
curl -X GET "http://localhost:8080/avia/health"
```

## Получение справочников

### Список аэропортов
```bash
curl -X GET "http://localhost:8080/avia/airports"
```

### Классы обслуживания
```bash
curl -X GET "http://localhost:8080/avia/service-classes"
```

### Типы пассажиров
```bash
curl -X GET "http://localhost:8080/avia/passenger-types"
```

## Поиск авиабилетов

### Поиск в одну сторону (OW)
```bash
curl -X POST "http://localhost:8080/avia/search" \
  -H "Content-Type: application/json" \
  -d '{
    "directions": [
      {
        "departure_airport": "TAS",
        "arrival_airport": "IST",
        "date": "2024-01-15"
      }
    ],
    "service_class": "E",
    "adults": 1,
    "children": 0,
    "infants": 0,
    "infants_with_seat": 0
  }'
```

### Поиск туда-обратно (RT)
```bash
curl -X POST "http://localhost:8080/avia/search" \
  -H "Content-Type: application/json" \
  -d '{
    "directions": [
      {
        "departure_airport": "TAS",
        "arrival_airport": "IST",
        "date": "2024-01-15"
      },
      {
        "departure_airport": "IST",
        "arrival_airport": "TAS",
        "date": "2024-01-22"
      }
    ],
    "service_class": "E",
    "adults": 2,
    "children": 1,
    "infants": 0,
    "infants_with_seat": 0
  }'
```

### Поиск с младенцем
```bash
curl -X POST "http://localhost:8080/avia/search" \
  -H "Content-Type: application/json" \
  -d '{
    "directions": [
      {
        "departure_airport": "TAS",
        "arrival_airport": "MOW",
        "date": "2024-01-15"
      }
    ],
    "service_class": "E",
    "adults": 2,
    "children": 0,
    "infants": 1,
    "infants_with_seat": 0
  }'
```

## Работа с офферами

### Получение информации об оффере
```bash
# Замените offer_id на реальный ID из поиска
curl -X GET "http://localhost:8080/avia/offers/OFFER_ID_HERE"
```

### Получение правил тарифа
```bash
curl -X GET "http://localhost:8080/avia/offers/OFFER_ID_HERE/rules"
```

## Бронирование

### Создание бронирования
```bash
curl -X POST "http://localhost:8080/avia/offers/OFFER_ID_HERE/booking" \
  -H "Content-Type: application/json" \
  -d '{
    "payer_name": "Иван Иванов",
    "payer_email": "ivan@example.com",
    "payer_tel": "+998901234567",
    "passengers": [
      {
        "first_name": "Иван",
        "last_name": "Иванов",
        "middle_name": "Иванович",
        "age": "adt",
        "birthdate": "1990-01-01",
        "gender": "M",
        "citizenship": "UZ",
        "tel": "+998901234567",
        "doc_type": "A",
        "doc_number": "AA1234567",
        "doc_expire": "2030-01-01"
      }
    ]
  }'
```

### Создание бронирования для семьи
```bash
curl -X POST "http://localhost:8080/avia/offers/OFFER_ID_HERE/booking" \
  -H "Content-Type: application/json" \
  -d '{
    "payer_name": "Иван Иванов",
    "payer_email": "ivan@example.com",
    "payer_tel": "+998901234567",
    "passengers": [
      {
        "first_name": "Иван",
        "last_name": "Иванов",
        "middle_name": "Иванович",
        "age": "adt",
        "birthdate": "1990-01-01",
        "gender": "M",
        "citizenship": "UZ",
        "tel": "+998901234567",
        "doc_type": "A",
        "doc_number": "AA1234567",
        "doc_expire": "2030-01-01"
      },
      {
        "first_name": "Мария",
        "last_name": "Иванова",
        "middle_name": "Ивановна",
        "age": "adt",
        "birthdate": "1992-05-15",
        "gender": "F",
        "citizenship": "UZ",
        "tel": "+998901234568",
        "doc_type": "A",
        "doc_number": "AA1234568",
        "doc_expire": "2030-01-01"
      },
      {
        "first_name": "Алексей",
        "last_name": "Иванов",
        "middle_name": "Иванович",
        "age": "chd",
        "birthdate": "2015-08-20",
        "gender": "M",
        "citizenship": "UZ",
        "tel": "+998901234569",
        "doc_type": "A",
        "doc_number": "AA1234569",
        "doc_expire": "2030-01-01"
      }
    ]
  }'
```

## Управление бронированиями

### Получение информации о бронировании
```bash
# Замените booking_id на реальный ID из создания бронирования
curl -X GET "http://localhost:8080/avia/booking/BOOKING_ID_HERE"
```

### Оплата бронирования
```bash
curl -X POST "http://localhost:8080/avia/booking/BOOKING_ID_HERE/payment"
```

### Отмена бронирования
```bash
curl -X POST "http://localhost:8080/avia/booking/BOOKING_ID_HERE/cancel"
```

## Тестирование ошибок

### Неверные параметры поиска
```bash
curl -X POST "http://localhost:8080/avia/search" \
  -H "Content-Type: application/json" \
  -d '{
    "directions": [],
    "service_class": "E",
    "adults": 0,
    "children": 0,
    "infants": 0,
    "infants_with_seat": 0
  }'
```

### Превышение лимита пассажиров
```bash
curl -X POST "http://localhost:8080/avia/search" \
  -H "Content-Type: application/json" \
  -d '{
    "directions": [
      {
        "departure_airport": "TAS",
        "arrival_airport": "IST",
        "date": "2024-01-15"
      }
    ],
    "service_class": "E",
    "adults": 5,
    "children": 5,
    "infants": 0,
    "infants_with_seat": 0
  }'
```

### Неверный формат даты
```bash
curl -X POST "http://localhost:8080/avia/search" \
  -H "Content-Type: application/json" \
  -d '{
    "directions": [
      {
        "departure_airport": "TAS",
        "arrival_airport": "IST",
        "date": "invalid-date"
      }
    ],
    "service_class": "E",
    "adults": 1,
    "children": 0,
    "infants": 0,
    "infants_with_seat": 0
  }'
```

## Проверка логов

При тестировании следите за логами сервера для диагностики проблем:

```bash
# В терминале где запущен сервер
tail -f logs/parser_errors.log
```

## Примечания по тестированию

1. **Тестовая среда**: Все запросы выполняются к тестовой среде Bukhara API
2. **Реальные данные**: Используйте реальные паспортные данные для тестирования
3. **Лимиты**: Учитывайте ограничения API (максимум 9 пассажиров, время жизни офферов)
4. **Токены**: Токен авторизации обновляется автоматически каждые 28 дней
5. **Ошибки**: При ошибках проверяйте коды ответов и сообщения об ошибках

## Успешное тестирование

При успешном тестировании вы должны увидеть:

- ✅ Статус 200 для всех корректных запросов
- ✅ Структурированные JSON ответы
- ✅ Автоматическое обновление токенов в логах
- ✅ Корректную обработку ошибок валидации
- ✅ Успешное создание и управление бронированиями 