# Тестирование всех API Kliro Backend

## 🔄 Парсинг данных

### Парсинг валют
```bash
curl -X GET "http://localhost:8080/parse-currency?url=https://bank.uz/uz/currency" | jq '.'
```

### Парсинг автокредитов
```bash
curl -X GET "http://localhost:8080/parse-autocredit?url=https://bank.uz/uz/credits/avtokredit" | jq '.'
```

### Парсинг микрокредитов
```bash
curl -X GET "http://localhost:8080/parse-microcredit?url=https://bank.uz/uz/credits/mikrozaymy" | jq '.'
```

### Парсинг ипотеки
```bash
curl -X GET "http://localhost:8080/parse-mortgage?url=https://bank.uz/uz/credits/ipoteka" | jq '.'
```

### Парсинг депозитов
```bash
curl -X GET "http://localhost:8080/parse-deposit?url=https://bank.uz/uz/deposits" | jq '.'
```

### Парсинг карт
```bash
curl -X GET "http://localhost:8080/parse-card?url=https://bank.uz/uz/cards" | jq '.'
```

### Парсинг переводов
```bash
curl -X GET "http://localhost:8080/parse-transfer?url=https://bank.uz/perevodi" | jq '.'
```

## 💰 Валюты

### Получение новых курсов валют
```bash
curl -X GET "http://localhost:8080/currencies/new?page=0&size=10" | jq '.'
```

### Получение старых курсов валют
```bash
curl -X GET "http://localhost:8080/currencies/old?page=0&size=10" | jq '.'
```

### Получение курсов валют по дате
```bash
curl -X GET "http://localhost:8080/currencies/by-date?date=2025-08-05" | jq '.'
```

## 🚗 Автокредиты

### Получение новых автокредитов
```bash
curl -X GET "http://localhost:8080/autocredits/new?page=0&size=10" | jq '.'
```

### Получение старых автокредитов
```bash
curl -X GET "http://localhost:8080/autocredits/old?page=0&size=10" | jq '.'
```

### Сортировка автокредитов по ставке
```bash
curl -X GET "http://localhost:8080/autocredits/new?page=0&size=5&sort=rate&direction=desc" | jq '.'
```

### Сортировка автокредитов по банку
```bash
curl -X GET "http://localhost:8080/autocredits/new?page=0&size=5&sort=bank_name&direction=asc" | jq '.'
```

## 💳 Микрокредиты

### Получение новых микрокредитов
```bash
curl -X GET "http://localhost:8080/microcredits/new?page=0&size=10" | jq '.'
```

### Получение старых микрокредитов
```bash
curl -X GET "http://localhost:8080/microcredits/old?page=0&size=10" | jq '.'
```

### Сортировка микрокредитов
```bash
curl -X GET "http://localhost:8080/microcredits/new?page=0&size=5&sort=rate&direction=desc" | jq '.'
```

## 🏠 Ипотека

### Получение новых ипотечных кредитов
```bash
curl -X GET "http://localhost:8080/mortgages/new?page=0&limit=10" | jq '.'
```

### Получение старых ипотечных кредитов
```bash
curl -X GET "http://localhost:8080/mortgages/old?page=0&limit=10" | jq '.'
```

### Сортировка ипотеки
```bash
curl -X GET "http://localhost:8080/mortgages/new?page=0&limit=5&sortBy=rate&sortOrder=desc" | jq '.'
```

## 💰 Депозиты

### Получение новых депозитов
```bash
curl -X GET "http://localhost:8080/deposits/new?page=0&size=10" | jq '.'
```

### Получение старых депозитов
```bash
curl -X GET "http://localhost:8080/deposits/old?page=0&size=10" | jq '.'
```

### Сортировка депозитов по ставке
```bash
curl -X GET "http://localhost:8080/deposits/new?page=0&size=5&sort=rate&direction=desc" | jq '.'
```

### Сортировка депозитов по банку
```bash
curl -X GET "http://localhost:8080/deposits/new?page=0&size=5&sort=bank_name&direction=asc" | jq '.'
```

## 🏦 Карты

### Получение новых карт
```bash
curl -X GET "http://localhost:8080/cards/new?page=0&size=10" | jq '.'
```

### Получение старых карт
```bash
curl -X GET "http://localhost:8080/cards/old?page=0&size=10" | jq '.'
```

### Фильтрация карт по валюте
```bash
curl -X GET "http://localhost:8080/cards/new?page=0&size=10&currency=USD" | jq '.'
```

### Фильтрация карт по системе
```bash
curl -X GET "http://localhost:8080/cards/new?page=0&size=10&system=Visa" | jq '.'
```

### Сортировка карт
```bash
curl -X GET "http://localhost:8080/cards/new?page=0&size=5&sort=bank_name&direction=asc" | jq '.'
```

## 💸 Переводы

### Получение новых переводов
```bash
curl -X GET "http://localhost:8080/transfers/new?page=0&size=10" | jq '.'
```

### Получение старых переводов
```bash
curl -X GET "http://localhost:8080/transfers/old?page=0&size=10" | jq '.'
```

### Сортировка переводов по комиссии
```bash
curl -X GET "http://localhost:8080/transfers/new?page=0&size=5&sort=commission&direction=desc" | jq '.'
```

### Сортировка переводов по приложению
```bash
curl -X GET "http://localhost:8080/transfers/new?page=0&size=5&sort=app_name&direction=asc" | jq '.'
```

## 🔍 Тестирование пагинации

### Проверка количества страниц (депозиты)
```bash
curl -X GET "http://localhost:8080/deposits/new?page=0&size=10" | jq '.result.totalPages, .result.totalElements'
```

### Проверка последней страницы (депозиты)
```bash
curl -X GET "http://localhost:8080/deposits/new?page=18&size=10" | jq '.result.last, .result.numberOfElements'
```

### Проверка количества страниц (автокредиты)
```bash
curl -X GET "http://localhost:8080/autocredits/new?page=0&size=10" | jq '.result.totalPages, .result.totalElements'
```

### Проверка последней страницы (автокредиты)
```bash
curl -X GET "http://localhost:8080/autocredits/new?page=10&size=10" | jq '.result.last, .result.numberOfElements'
```

## 📊 Быстрые проверки

### Количество элементов в каждой таблице
```bash
echo "=== Количество элементов ==="
echo "Депозиты: $(curl -s 'http://localhost:8080/deposits/new?page=0&size=1' | jq '.result.totalElements')"
echo "Автокредиты: $(curl -s 'http://localhost:8080/autocredits/new?page=0&size=1' | jq '.result.totalElements')"
echo "Микрокредиты: $(curl -s 'http://localhost:8080/microcredits/new?page=0&size=1' | jq '.result.totalElements')"
echo "Ипотека: $(curl -s 'http://localhost:8080/mortgages/new?page=0&limit=1' | jq '.result.totalElements')"
echo "Карты: $(curl -s 'http://localhost:8080/cards/new?page=0&size=1' | jq '.result.totalElements')"
echo "Переводы: $(curl -s 'http://localhost:8080/transfers/new?page=0&size=1' | jq '.result.totalElements')"
echo "Валюты: $(curl -s 'http://localhost:8080/currencies/new?page=0&size=1' | jq '.result.totalElements')"
```

### Проверка первой записи в каждой таблице
```bash
echo "=== Первые записи ==="
echo "Депозиты: $(curl -s 'http://localhost:8080/deposits/new?page=0&size=1' | jq '.result.content[0].bank_name')"
echo "Автокредиты: $(curl -s 'http://localhost:8080/autocredits/new?page=0&size=1' | jq '.result.content[0].bank_name')"
echo "Микрокредиты: $(curl -s 'http://localhost:8080/microcredits/new?page=0&size=1' | jq '.result.content[0].bank_name')"
echo "Ипотека: $(curl -s 'http://localhost:8080/mortgages/new?page=0&limit=1' | jq '.result.content[0].bank_name')"
echo "Карты: $(curl -s 'http://localhost:8080/cards/new?page=0&size=1' | jq '.result.content[0].bank_name')"
echo "Переводы: $(curl -s 'http://localhost:8080/transfers/new?page=0&size=1' | jq '.result.content[0].app_name')"
echo "Валюты: $(curl -s 'http://localhost:8080/currencies/new?page=0&size=1' | jq '.result.content[0].bank_name')"
```

## 📝 Примечания

- **Пагинация начинается с 0** (zero-based indexing)
- **Размер страницы по умолчанию**: 10 элементов
- **Максимальный размер страницы**: 100 элементов
- **Сортировка**: `asc` (по возрастанию) или `desc` (по убыванию)
- **Все ответы обернуты в `result`** поле для консистентности

## 🚀 Запуск всех тестов

```bash
# Создаем скрипт для тестирования всех API
cat > test_all_apis.sh << 'EOF'
#!/bin/bash
echo "🧪 Тестирование всех API Kliro Backend"
echo "======================================"

# Тестируем парсинг
echo "1. Тестирование парсинга..."
curl -s "http://localhost:8080/parse-currency?url=https://bank.uz/uz/currency" | jq '.success' > /dev/null && echo "✅ Парсинг валют работает" || echo "❌ Парсинг валют не работает"

# Тестируем получение данных
echo "2. Тестирование получения данных..."
curl -s "http://localhost:8080/deposits/new?page=0&size=1" | jq '.success' > /dev/null && echo "✅ Депозиты работают" || echo "❌ Депозиты не работают"
curl -s "http://localhost:8080/autocredits/new?page=0&size=1" | jq '.success' > /dev/null && echo "✅ Автокредиты работают" || echo "❌ Автокредиты не работают"
curl -s "http://localhost:8080/microcredits/new?page=0&size=1" | jq '.success' > /dev/null && echo "✅ Микрокредиты работают" || echo "❌ Микрокредиты не работают"
curl -s "http://localhost:8080/mortgages/new?page=0&limit=1" | jq '.success' > /dev/null && echo "✅ Ипотека работает" || echo "❌ Ипотека не работает"
curl -s "http://localhost:8080/cards/new?page=0&size=1" | jq '.success' > /dev/null && echo "✅ Карты работают" || echo "❌ Карты не работают"
curl -s "http://localhost:8080/transfers/new?page=0&size=1" | jq '.success' > /dev/null && echo "✅ Переводы работают" || echo "❌ Переводы не работают"
curl -s "http://localhost:8080/currencies/new?page=0&size=1" | jq '.success' > /dev/null && echo "✅ Валюты работают" || echo "❌ Валюты не работают"

echo "======================================"
echo "🎉 Тестирование завершено!"
EOF

chmod +x test_all_apis.sh
./test_all_apis.sh
```
