package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kliro/routes"

	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

// Функция для загрузки .env перед тестами
func TestMain(m *testing.M) {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}
	os.Exit(m.Run())
}

// 1️⃣ Тест регистрации по email
func TestRegisterEmail(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"email": "ibrokhimov3210@gmail.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code) // или другой ожидаемый код
	assert.Contains(t, w.Body.String(), "otp sent")
}

// 2️⃣ Тест регистрации по телефону
func TestRegisterPhone(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"phone": "+9983311108810"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "otp sent")
}

// 3️⃣ Тест подтверждения OTP (email)
func TestConfirmOTPEmail(t *testing.T) {
	// Для реального теста нужно получить реальный OTP из Redis или мокать Redis
	// Здесь пример с неверным OTP (негативный сценарий)
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"email": "ibrokhimov3210@gmail.com", "otp": "wrong"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/confirm-otp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "Неверный или истёкший код")
}

// 4️⃣ Тест подтверждения OTP (телефон)
func TestConfirmOTPPhone(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"phone": "+9983311108810", "otp": "wrong"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/confirm-otp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "Неверный или истёкший код")
}

// 5️⃣ Тест установки региона и пароля (email)
func TestSetRegionPasswordEmail(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]interface{}{"email": "ibrokhimov3210@gmail.com", "region_id": 1, "password": "testpass123"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/set-region-password-final", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	// Ожидаем ошибку, если OTP не был подтверждён
	assert.NotEqual(t, 200, w.Code)
}

// 6️⃣ Тест установки региона и пароля (телефон)
func TestSetRegionPasswordPhone(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]interface{}{"phone": "+9983311108810", "region_id": 1, "password": "testpass123"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/set-region-password-final", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	// Ожидаем ошибку, если OTP не был подтверждён
	assert.NotEqual(t, 200, w.Code)
}

// 7️⃣ Тест входа по email
func TestLoginEmail(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"email": "ibrokhimov3210@gmail.com", "password": "testpass123"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.True(t, w.Code == 200 || w.Code == 401)
}

// 8️⃣ Тест входа по телефону
func TestLoginPhone(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"phone": "+9983311108810", "password": "testpass123"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.True(t, w.Code == 200 || w.Code == 401)
}

// 9️⃣ Тест восстановления пароля по email
func TestForgotPasswordEmail(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"email": "ibrokhimov3210@gmail.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/forgot-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "otp sent")
}

// 🔟 Тест восстановления пароля по телефону
func TestForgotPasswordPhone(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"phone": "+9983311108810"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/forgot-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "otp sent")
}

// 1️⃣1️⃣ Тест сброса пароля по email
func TestResetPasswordEmail(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"email": "ibrokhimov3210@gmail.com", "otp": "wrong", "password": "newpass321"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/reset-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.NotEqual(t, 200, w.Code)
}

// 1️⃣2️⃣ Тест сброса пароля по телефону
func TestResetPasswordPhone(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"phone": "+9983311108810", "otp": "wrong", "password": "newpass321"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/reset-password", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.NotEqual(t, 200, w.Code)
}

// 1️⃣3️⃣ Тест входа с новым паролем (email)
func TestLoginWithNewPasswordEmail(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"email": "ibrokhimov3210@gmail.com", "password": "newpass321"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.True(t, w.Code == 200 || w.Code == 401)
}

// 1️⃣4️⃣ Тест входа с новым паролем (телефон)
func TestLoginWithNewPasswordPhone(t *testing.T) {
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	body := map[string]string{"phone": "+9983311108810", "password": "newpass321"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.True(t, w.Code == 200 || w.Code == 401)
}

// 1️⃣5️⃣ Тест Google OAuth (заглушка)
func TestGoogleOAuth(t *testing.T) {
	// Для реального теста нужен мок Google OAuth flow
	// Здесь просто проверяем, что роут существует
	r := routes.SetupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/auth/google", nil)
	r.ServeHTTP(w, req)
	assert.True(t, w.Code == 302 || w.Code == 200)
}
