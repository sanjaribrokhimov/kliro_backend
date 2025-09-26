# Trust Insurance API Proxy - Trust Insurance API Proxy

Bu hujjat Trust Insurance API proxy endpointlarini tavsiflaydi. Barcha so'rovlar va javoblar Trust Insurance API orqali uzatiladi.

## API Endpointlar

### Autentifikatsiya (Kirish)

#### 1. Login - `/trustInsurance/auth/login`
**Metod:** POST  
**Tavsif:** Trust Insurance API ga kirish uchun token olish

**So'rov misoli:**
```json
{
  "username": "your_username",
  "password": "your_password"
}
```

**Javob misoli:**
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

### OSAGO (Avtoliabilnost sug'urtasi)

#### 1. Yaratish - `/trustInsurance/osago/create`
**Metod:** POST  
**Tavsif:** OSAGO polisini yaratish

**So'rov misoli:**
```json
{
  "vehicle_type": "car",
  "engine_power": 150,
  "year": 2020,
  "region": "tashkent",
  "driver_age": 25,
  "experience": 3
}
```

**Javob misoli:**
```json
{
  "success": true,
  "premium": 450000,
  "currency": "UZS",
  "calculation_date": "2024-01-15T10:30:00Z"
}
```

#### 2. Premiya hisoblash - `/trustInsurance/osago/calc-prem`
**Metod:** POST  
**Tavsif:** OSAGO premiasini hisoblash

**So'rov misoli:**
```json
{
  "client_info": {
    "name": "Akmal Karimov",
    "passport": "AA1234567",
    "phone": "+998901234567"
  },
  "vehicle_info": {
    "number": "01A123AA",
    "model": "Chevrolet Cobalt",
    "year": 2020
  },
  "policy_period": {
    "start_date": "2024-01-15",
    "end_date": "2025-01-15"
  }
}
```

**Javob misoli:**
```json
{
  "success": true,
  "contract_id": "TR-2024-001234",
  "policy_number": "OSAGO-2024-001234",
  "status": "active",
  "premium_amount": 450000,
  "payment_url": "https://payment.trust.uz/pay/TR-2024-001234"
}
```

### Reference (Ma'lumotlar)

#### 3. Qarindoshlar - `/trustInsurance/reference/relatives`
**Metod:** GET  
**Tavsif:** Qarindoshlik darajalari ro'yxati

**Javob misoli:**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "Ota",
      "name_uz": "Ота"
    },
    {
      "id": 2,
      "name": "Ona", 
      "name_uz": "Она"
    },
    {
      "id": 3,
      "name": "Aka",
      "name_uz": "Ака"
    }
  ]
}
```

### Provider (Ta'minotchi)

#### 4. Transport vositasini tekshirish - `/trustInsurance/provider/vehicle`
**Metod:** GET  
**Tavsif:** Transport vositasining ma'lumotlarini olish

**So'rov parametrlari:**
- `number` - Transport vositasining raqami (masalan: 01A123AA)

**Javob misoli:**
```json
{
  "success": true,
  "data": {
    "number": "01A123AA",
    "model": "Chevrolet Cobalt",
    "year": 2020,
    "color": "Oq",
    "owner": "Akmal Karimov",
    "status": "active"
  }
}
```

#### 5. Passport va PINFL tekshirish - `/trustInsurance/provider/passport-pinfl`
**Metod:** GET  
**Tavsif:** Passport va PINFL ma'lumotlarini tekshirish

**So'rov parametrlari:**
- `passport` - Passport raqami
- `pinfl` - PINFL raqami

**Javob misoli:**
```json
{
  "success": true,
  "data": {
    "passport": "AA1234567",
    "pinfl": "12345678901234",
    "name": "Akmal Karimov",
    "birth_date": "1995-05-15",
    "status": "valid"
  }
}
```

#### 6. Tug'ilgan sana tekshirish - `/trustInsurance/provider/passport-birth-date`
**Metod:** GET  
**Tavsif:** Passport orqali tug'ilgan sanani tekshirish

**So'rov parametrlari:**
- `passport` - Passport raqami

**Javob misoli:**
```json
{
  "success": true,
  "data": {
    "passport": "AA1234567",
    "birth_date": "1995-05-15",
    "age": 29
  }
}
```

#### 7. Haydovchi ma'lumotlari - `/trustInsurance/provider/driver-summary`
**Metod:** GET  
**Tavsif:** Haydovchining umumiy ma'lumotlari

**So'rov parametrlari:**
- `passport` - Passport raqami

**Javob misoli:**
```json
{
  "success": true,
  "data": {
    "passport": "AA1234567",
    "name": "Akmal Karimov",
    "license_number": "DL123456789",
    "license_expiry": "2029-12-31",
    "violations": 0,
    "experience_years": 5
  }
}
```

## Xatoliklar

Barcha endpointlar quyidagi xatolik formatini qaytaradi:

```json
{
  "error": "Xatolik xabari",
  "code": "ERROR_CODE",
  "details": "Batafsil xatolik ma'lumoti"
}
```

## Autentifikatsiya

Barcha so'rovlar Bearer token orqali autentifikatsiya qilinadi:

```
Authorization: Bearer YOUR_TRUST_TOKEN
```

## .env o'zgaruvchilari

Quyidagi o'zgaruvchilarni `.env` fayliga qo'shing:

```env
# Trust Insurance API
TRUST_BASE_URL=https://api.online-trust.uz
TRUST_LOGIN=KLIRO_TECH_API
TRUST_PASSWORD=$23KLIRO09TECH25#
```

**Eslatma:** 
- Barcha API so'rovlari avtomatik ravishda `TRUST_LOGIN` va `TRUST_PASSWORD` orqali token olinadi
- Agar token yo'q bo'lsa, avtomatik ravishda yangi token olinadi
- `/trustInsurance/auth/login` endpoint ixtiyoriy - foydalanuvchi o'z credentials bilan kirishi mumkin

## Misollar

### OSAGO polisini yaratish
```bash
curl -X POST "http://localhost:8080/trustInsurance/osago/create" \
  -H "Content-Type: application/json" \
  -d '{
    "vehicle_type": "car",
    "engine_power": 150,
    "year": 2020,
    "region": "tashkent"
  }'
```

### OSAGO premiasini hisoblash
```bash
curl -X POST "http://localhost:8080/trustInsurance/osago/calc-prem" \
  -H "Content-Type: application/json" \
  -d '{
    "vehicle_type": "car",
    "engine_power": 150,
    "year": 2020,
    "region": "tashkent",
    "driver_age": 25,
    "experience": 3
  }'
```

### Qarindoshlar ro'yxatini olish
```bash
curl -X GET "http://localhost:8080/trustInsurance/reference/relatives"
```

### Transport vositasini tekshirish
```bash
curl -X GET "http://localhost:8080/trustInsurance/provider/vehicle?number=01A123AA"
```
