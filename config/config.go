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
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}
	return &Config{
		DBHost:         os.Getenv("DB_HOST"),
		DBPort:         os.Getenv("DB_PORT"),
		DBUser:         os.Getenv("DB_USER"),
		DBPassword:     os.Getenv("DB_PASSWORD"),
		DBName:         os.Getenv("DB_NAME"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		Port:           os.Getenv("PORT"),
		SMTPHost:       os.Getenv("SMTP_HOST"),
		SMTPPort:       os.Getenv("SMTP_PORT"),
		SMTPUser:       os.Getenv("SMTP_USER"),
		SMTPPass:       os.Getenv("SMTP_PASS"),
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirect: os.Getenv("GOOGLE_REDIRECT_URI"),
		EskizEmail:     os.Getenv("ESKIZ_EMAIL"),
		EskizPassword:  os.Getenv("ESKIZ_PASSWORD"),
		NeoBaseURL:     getenvOrDefault("NEO_BASE_URL", "https://api.neoinsurance.uz"),
		NeoLogin:       os.Getenv("NEO_LOGIN"),
		NeoPassword:    os.Getenv("NEO_PASSWORD"),
	}
}

// getenvOrDefault returns the environment variable value if set, otherwise returns def
func getenvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
