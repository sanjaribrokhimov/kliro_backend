# ОСАГО API Документация

## Обзор
Расширенный функционал для работы с ОСАГО страховкой, поддерживающий двух провайдеров: Neo Insurance и Trust Insurance.

## API Endpoints

### 1. POST /unified/osago/nacalo
**Описание**: Инициализация сессии с данными автомобиля
**Входные данные**:
```json
{
  "govNumber": "01J409QC",
  "techPassportNumber": "3899020",
  "techPassportSeria": "AAG",
  "ownerPNumber": "7784524",
  "ownerPSeries": "AD"
}
```
**Ответ**: `sessionId` (UUID) для дальнейших операций

### 2. POST /unified/osago/initcon
**Описание**: Инициализация контракта с данными водителей и заявителя
**Входные данные**:
```json
{
  "uuid": "session-uuid",
  "isDrivers": 0,
  "drivers": [
    {
      "driverPinfl": "12345678901234",
      "driverPSeries": "AB",
      "driverPNumber": "1234567"
    }
  ],
  "applPinfl": "30101995750028",
  "applPSeries": "AB",
  "applPNumber": "0160608",
  "applPhone": "998951124422",
  "contractBegin": "2024-01-01"
}
```
**Валидация**:
- Если `isDrivers == 0`: массив `drivers` обязателен (максимум 5 водителей)
- Если `isDrivers == 1`: водители не ограничены

### 3. POST /unified/osago/calc
**Описание**: Расчет стоимости страховки
**Входные данные**:
```json
{
  "strahovkaMonth": 6,
  "isDrivers": 1,
  "carType": 1,
  "uuid": "session-uuid",
  "provider": "neo",
  "contractBegin": "2024-01-01"
}
```
**Ответ**:
```json
{
  "sessionId": "session-uuid",
  "summaStrahovki": 56000
}
```

### 4. POST /unified/osago/submit
**Описание**: Создание страхового полиса
**Входные данные**:
```json
{
  "uuid": "session-uuid"
}
```

**Ответ для Neo**:
```json
{
  "sessionId": "session-uuid",
  "orderId": 905374,
  "contractId": 1158393,
  "payUrl": "https://...",
  "paymeUrl": "https://...",
  "amount": "56000"
}
```

**Ответ для Trust**:
```json
{
  "sessionId": "session-uuid",
  "providerUuid": "b6bc98ea-...",
  "insurancePremium": "39200",
  "anketaId": 778390
}
```

### 5. GET /unified/osago/session/:id
**Описание**: Получение полных данных сессии
**Ответ**: Полная структура `VehicleData`

## Структуры данных

### VehicleData
Основная структура для хранения всех данных о страховке:
- Данные автомобиля (госномер, техпаспорт, модель и т.д.)
- Данные владельца
- Массив водителей (`[]DriverStored`)
- Данные заявителя (`ApplicantStored`)
- Параметры расчета (период, тип автомобиля, провайдер)
- Результаты расчета и создания полиса

### DriverStored
Данные водителя:
- ПИНФЛ, паспортные данные
- ФИО на латинице
- Дата рождения, область, район
- Данные водительского удостоверения

### ApplicantStored
Данные заявителя:
- ПИНФЛ, паспортные данные
- Телефон
- ФИО на латинице
- Дата рождения, область, район

## Маппинги

### periodMapping
Сопоставление периодов страхования между нашими ID и ID провайдеров:
```go
{OurID: 1, NeoID: 1, TrustID: 2}
{OurID: 6, NeoID: 2, TrustID: 3}
```

### driversMapping
Сопоставление ограничений по водителям:
```go
{OurID: 0, NeoID: 0, TrustID: 0}  // неограниченно
{OurID: 5, NeoID: 4, TrustID: 1}  // ограничено
```

### carTypeMapping
Сопоставление типов автомобилей:
```go
{OurID: 1, NeoID: 10, TrustID: 5}
{OurID: 2, NeoID: 2, TrustID: 6}
// и т.д.
```

## Хранение данных
- In-memory хранилище с потокобезопасным доступом (`sync.RWMutex`)
- Ключ: UUID сессии
- Значение: указатель на `VehicleData`
- Автоматическое получение дополнительных данных через внешние API

## Обработка ошибок
- Валидация входных данных
- Проверка существования сессий
- Обработка ошибок внешних API
- Детальные сообщения об ошибках

## Поток работы
1. **nacalo** → создание сессии с данными автомобиля
2. **initcon** → добавление данных водителей и заявителя
3. **calc** → расчет стоимости
4. **submit** → создание страхового полиса

Все данные сохраняются в едином хранилище и доступны через `sessionId`.
