## Тестирование API переводов

### 1. Парсинг перевода по URL
curl -X GET "http://localhost:8080/parse-transfer?url=https://bank.uz/perevodi" \n  -H "Content-Type: application/json" | jq '.'

### 2. Получение новых переводов с пагинацией
curl -X GET "http://localhost:8080/transfers/new?page=0&size=10&sort=app_name&direction=asc" \n  -H "Content-Type: application/json" | jq '.'

### 3. Получение старых переводов с пагинацией
curl -X GET "http://localhost:8080/transfers/old?page=0&size=10&sort=app_name&direction=asc" \n  -H "Content-Type: application/json" | jq '.'

### 4. Тестирование сортировки по комиссии
curl -X GET "http://localhost:8080/transfers/new?page=0&size=5&sort=commission&direction=desc" \n  -H "Content-Type: application/json" | jq '.'

### 5. Тестирование с большим размером страницы
curl -X GET "http://localhost:8080/transfers/new?page=0&size=20&sort=created_at&direction=desc" \n  -H "Content-Type: application/json" | jq '.'
