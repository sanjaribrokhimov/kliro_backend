package config

import (
	"log"
	"os"

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
	}
}

// getenvOrDefault returns the environment variable value if set, otherwise returns def
func getenvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
