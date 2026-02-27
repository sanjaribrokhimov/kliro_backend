package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	JWTSecret      string
	Port           string
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPass       string
	GoogleClientID string
	GoogleSecret   string
	GoogleRedirect string
	EskizEmail     string
	EskizPassword  string
	// NeoInsurance external API settings
	NeoBaseURL  string
	NeoLogin    string
	NeoPassword string
	// Trust Insurance external API settings
	TrustBaseURL  string
	TrustLogin    string
	TrustPassword string
	// Payment system settings
	ClickServiceID   string
	ClickMerchantID  string
	PaymeMerchantID  string
	PaymentReturnURL string
	GrossBaseURL     string
	GrossLogin       string
	GrossPassword    string
	// Euroasia OSAGO settings
	EuroasiaBaseURL string
	EuroasiaAPIKey  string
	// Euroasia All OSAGO settings (erp.eai.uz)
	EuroasiaAllBaseURL string
	EuroasiaAllAPIKey  string
	// Apex OSAGO settings
	ApexBaseURL  string
	ApexLogin    string
	ApexPassword string
	ApexUserID   int // статический user_id для create (из .env APEX_USER_ID, по умолчанию 30541)
	// Inson OSAGO settings
	InsonBaseURL  string
	InsonLogin    string
	InsonPassword string
	// Translation API settings (бесплатный API, без токенов)
	TranslationAPIURL string // URL для LibreTranslate (опционально, по умолчанию используется публичный)
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}
	return &Config{
		DBHost:           os.Getenv("DB_HOST"),
		DBPort:           os.Getenv("DB_PORT"),
		DBUser:           os.Getenv("DB_USER"),
		DBPassword:       os.Getenv("DB_PASSWORD"),
		DBName:           os.Getenv("DB_NAME"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		Port:             os.Getenv("PORT"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         os.Getenv("SMTP_PORT"),
		SMTPUser:         os.Getenv("SMTP_USER"),
		SMTPPass:         os.Getenv("SMTP_PASS"),
		GoogleClientID:   os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:     os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirect:   os.Getenv("GOOGLE_REDIRECT_URI"),
		EskizEmail:       os.Getenv("ESKIZ_EMAIL"),
		EskizPassword:    os.Getenv("ESKIZ_PASSWORD"),
		NeoBaseURL:       getenvOrDefault("NEO_BASE_URL", "https://api.neoinsurance.uz"),
		NeoLogin:         os.Getenv("NEO_LOGIN"),
		NeoPassword:      os.Getenv("NEO_PASSWORD"),
		TrustBaseURL:     getenvOrDefault("TRUST_BASE_URL", "https://api.online-trust.uz"),
		TrustLogin:       os.Getenv("TRUST_LOGIN"),
		TrustPassword:    os.Getenv("TRUST_PASSWORD"),
		ClickServiceID:   os.Getenv("CLICK_SERVICE_ID"),
		ClickMerchantID:  os.Getenv("CLICK_MERCHANT_ID"),
		PaymeMerchantID:  os.Getenv("PAYME_MERCHANT_ID"),
		PaymentReturnURL: getenvOrDefault("PAYMENT_RETURN_URL", "https://your-domain.com/payment/return"),
		GrossBaseURL:     getenvOrDefault("GROSS_BASE_URL", "https://gross.uz"),
		GrossLogin:       os.Getenv("GROSS_LOGIN"),
		GrossPassword:    os.Getenv("GROSS_PASSWORD"),
		EuroasiaBaseURL:    getenvOrDefault("EUROASIA_BASE_URL", "https://api.example.com"),
		EuroasiaAPIKey:      os.Getenv("EUROASIA_API_KEY"),
		EuroasiaAllBaseURL: getenvOrDefault("EUROASIA_ALL_BASE_URL", "https://erp.eai.uz"),
		EuroasiaAllAPIKey:   os.Getenv("EUROASIA_ALL_API_KEY"),
		ApexBaseURL:         getenvOrDefault("APEX_BASE_URL", "https://rest.aic.uz/api/ins/apex_box"),
		ApexLogin:           os.Getenv("APEX_LOGIN"),
		ApexPassword:        os.Getenv("APEX_PASSWORD"),
		ApexUserID:          getenvIntOrDefault("APEX_USER_ID", 30541),
		InsonBaseURL:        getenvOrDefault("INSON_BASE_URL", "https://testapi-ersp.insonline.uz"),
		InsonLogin:          os.Getenv("INSON_LOGIN"),
		InsonPassword:       os.Getenv("INSON_PASSWORD"),
		TranslationAPIURL:   getenvOrDefault("TRANSLATION_API_URL", "https://libretranslate.com/translate"),
	}
}

// getenvOrDefault returns the environment variable value if set, otherwise returns def
func getenvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getenvIntOrDefault returns the environment variable as int if set and valid, otherwise returns def
func getenvIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
