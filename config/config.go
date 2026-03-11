package config

import (
	"log"
	"os"
	"strconv"
	"strings"

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
	EuroasiaAllBaseURL    string
	EuroasiaAllAPIKey     string
	EuroasiaSeasonalID12  string // UUID сезонности 12 мес (365 дней)
	EuroasiaSeasonalID6   string // UUID сезонности 6 мес (180 дней)
	EuroasiaSeasonalID20  string // UUID сезонности 20 дней
	// Apex OSAGO settings
	ApexBaseURL           string
	ApexLogin             string
	ApexPassword          string
	ApexUserID            int    // из .env APEX_USER_ID
	ApexForeignVehicleID  string // из .env APEX_FOREIGN_VEHICLE_ID (id справочника «не иностранное ТС»)
	ApexContractTerm12    string // contractTermConclusionId для 12 мес
	ApexContractTerm6     string // для 6 мес / сезон
	ApexSeasonalID12      string // seasonalInsuranceId 12 мес
	ApexSeasonalID6       string // seasonalInsuranceId 6 мес
	ApexSeasonalID20      string // seasonalInsuranceId 20 дней
	// Trust OSAGO calc
	TrustDefaultDiscountID int // из .env TRUST_DEFAULT_DISCOUNT_ID (1 = без льгот)
	// Маппинг территории Find (external_id 1,2,3) → Trust use_territory (1–14). Find: 1=Ташкент и обл., 2=Другие регионы, 3=Для иностранцев. Trust: 1=город Ташкент, 2=Ташкентская обл., 3–14=остальные.
	TrustTerritoryFind1 int // TRUST_TERRITORY_FIND_1 (default 1)
	TrustTerritoryFind2 int // TRUST_TERRITORY_FIND_2 (default 10 — Самаркандская, для «Другие регионы»)
	TrustTerritoryFind3 int // TRUST_TERRITORY_FIND_3 (default 10)
	// Inson OSAGO settings
	InsonBaseURL  string
	InsonLogin    string
	InsonPassword string
	// Translation API settings (бесплатный API, без токенов)
	TranslationAPIURL string // URL для LibreTranslate (опционально, по умолчанию используется публичный)
	// App Links / Universal Links (для Android App Links и iOS Associated Domains)
	AndroidPackageName       string   // package_name для assetlinks.json (например com.kliro.app)
	AndroidSHA256Fingerprints []string // SHA256 отпечатки сертификатов (через запятую в .env)
	AppleTeamID             string   // Team ID для apple-app-site-association (например 9JA89Q95N)
	AppleBundleID           string   // Bundle ID для iOS (например com.kliro.app)
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
		EuroasiaAllBaseURL:   getenvOrDefault("EUROASIA_ALL_BASE_URL", "https://erp.eai.uz"),
		EuroasiaAllAPIKey:    os.Getenv("EUROASIA_ALL_API_KEY"),
		EuroasiaSeasonalID12: getenvOrDefault("EUROASIA_SEASONAL_INSURANCE_ID_12", "8465a831-850f-4445-a995-ef71195094ab"),
		EuroasiaSeasonalID6:  getenvOrDefault("EUROASIA_SEASONAL_INSURANCE_ID_6", "9848096e-cc12-4dbd-893b-41f2cdfc9a0e"),
		EuroasiaSeasonalID20: getenvOrDefault("EUROASIA_SEASONAL_INSURANCE_ID_20", "0d546748-0ba6-43bc-9ce2-1b977ad9e494"),
		ApexBaseURL:         getenvOrDefault("APEX_BASE_URL", "https://rest.aic.uz/api/ins/apex_box"),
		ApexLogin:           os.Getenv("APEX_LOGIN"),
		ApexPassword:        os.Getenv("APEX_PASSWORD"),
		ApexUserID:            getenvIntOrDefault("APEX_USER_ID", 30541),
		ApexForeignVehicleID:  getenvOrDefault("APEX_FOREIGN_VEHICLE_ID", "2"),
		ApexContractTerm12:    getenvOrDefault("APEX_CONTRACT_TERM_12", "1"),
		ApexContractTerm6:    getenvOrDefault("APEX_CONTRACT_TERM_6", "2"),
		ApexSeasonalID12:     getenvOrDefault("APEX_SEASONAL_INSURANCE_ID_12", "7"),
		ApexSeasonalID6:      getenvOrDefault("APEX_SEASONAL_INSURANCE_ID_6", "1"),
		ApexSeasonalID20:     getenvOrDefault("APEX_SEASONAL_INSURANCE_ID_20", "8"),
		TrustDefaultDiscountID: getenvIntOrDefault("TRUST_DEFAULT_DISCOUNT_ID", 1),
		TrustTerritoryFind1:    getenvIntOrDefault("TRUST_TERRITORY_FIND_1", 1),
		TrustTerritoryFind2:    getenvIntOrDefault("TRUST_TERRITORY_FIND_2", 10),
		TrustTerritoryFind3:    getenvIntOrDefault("TRUST_TERRITORY_FIND_3", 10),
		InsonBaseURL:          getenvOrDefault("INSON_BASE_URL", "https://testapi-ersp.insonline.uz"),
		InsonLogin:          os.Getenv("INSON_LOGIN"),
		InsonPassword:       os.Getenv("INSON_PASSWORD"),
		TranslationAPIURL:   getenvOrDefault("TRANSLATION_API_URL", "https://libretranslate.com/translate"),
		AndroidPackageName:  getenvOrDefault("ANDROID_PACKAGE_NAME", "com.kliro.app"),
		AndroidSHA256Fingerprints: getenvSliceOrDefault("ANDROID_SHA256_CERT_FINGERPRINTS", []string{"F7:34:EE:03:5C:83:AA:B7:EF:44:43:67:95:28:9B:D0:16:99:0F:E5:52:B8:0F:98:E5:12:76:F2:33:E2"}),
		AppleTeamID:         os.Getenv("APPLE_TEAM_ID"),
		AppleBundleID:       getenvOrDefault("APPLE_BUNDLE_ID", "com.kliro.app"),
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

// getenvSliceOrDefault returns the environment variable split by comma (trimmed), or def if empty
func getenvSliceOrDefault(key string, def []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
