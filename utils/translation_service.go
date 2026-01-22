package utils

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/go-redis/redis/v8"
)

// TranslationService - сервис для переводов с использованием внешнего API
type TranslationService struct {
	apiURL string // URL API (LibreTranslate или MyMemory)
	client *http.Client
	redis  *redis.Client
}

// NewTranslationService создает новый сервис переводов (бесплатный, без токенов)
func NewTranslationService(apiURL string) *TranslationService {
	// Если URL не указан, используем публичный LibreTranslate
	if apiURL == "" {
		apiURL = "https://libretranslate.com/translate"
	}
	
	return &TranslationService{
		apiURL: apiURL,
		client: &http.Client{Timeout: 10 * time.Second},
		redis:  GetRedis(),
	}
}

// Translate переводит текст с узбекского на указанный язык
// Использует кэш Redis для сохранения переводов
func (ts *TranslationService) Translate(text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	// Генерируем ключ кэша
	cacheKey := ts.getCacheKey(text, targetLang)

	// Проверяем кэш
	if ts.redis != nil {
		ctx := context.Background()
		cached, err := ts.redis.Get(ctx, cacheKey).Result()
		if err == nil && cached != "" {
			log.Printf("[TRANSLATION CACHE] HIT: %s -> %s", text[:min(50, len(text))], targetLang)
			return cached, nil
		}
	}

	// Переводим через бесплатный API
	translated, err := ts.translateFree(text, targetLang)

	if err != nil {
		log.Printf("[TRANSLATION ERROR] Failed to translate: %v", err)
		return text, err // Возвращаем оригинал при ошибке
	}

	// Сохраняем в кэш на 30 дней
	if ts.redis != nil && translated != "" {
		ctx := context.Background()
		ts.redis.Set(ctx, cacheKey, translated, 30*24*time.Hour)
		log.Printf("[TRANSLATION CACHE] MISS: %s -> %s (stored)", text[:min(50, len(text))], targetLang)
	}

	return translated, nil
}

// getCacheKey генерирует ключ для кэша
func (ts *TranslationService) getCacheKey(text, lang string) string {
	hash := md5.Sum([]byte(text + ":" + lang))
	return fmt.Sprintf("translation:uz:%s:%x", lang, hash)
}

// translateFree переводит через бесплатный API (LibreTranslate или MyMemory)
func (ts *TranslationService) translateFree(text, targetLang string) (string, error) {
	// Маппинг языков
	langMap := map[string]string{
		"ru": "ru",
		"en": "en",
	}

	targetLangCode, ok := langMap[targetLang]
	if !ok {
		return text, fmt.Errorf("unsupported target language: %s", targetLang)
	}

	// Пробуем сначала LibreTranslate
	translated, err := ts.translateLibreTranslate(text, targetLangCode)
	if err == nil && translated != "" {
		return translated, nil
	}

	// Если LibreTranslate не сработал, пробуем MyMemory
	translated, err = ts.translateMyMemory(text, targetLangCode)
	if err == nil && translated != "" {
		return translated, nil
	}

	return text, fmt.Errorf("all translation APIs failed")
}

// translateLibreTranslate переводит через LibreTranslate (бесплатный, без токенов)
func (ts *TranslationService) translateLibreTranslate(text, targetLang string) (string, error) {
	// Используем публичный LibreTranslate API
	apiURL := "https://libretranslate.com/translate"
	
	requestData := map[string]interface{}{
		"q":      text,
		"source": "uz",
		"target": targetLang,
		"format": "text",
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return text, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return text, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		return text, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return text, fmt.Errorf("libretranslate API error: %s", string(body))
	}

	var result struct {
		TranslatedText string `json:"translatedText"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return text, err
	}

	if result.TranslatedText == "" {
		return text, fmt.Errorf("no translation returned")
	}

	return result.TranslatedText, nil
}

// translateMyMemory переводит через MyMemory API (бесплатный, без токенов, с лимитами)
func (ts *TranslationService) translateMyMemory(text, targetLang string) (string, error) {
	// MyMemory API - бесплатный, без токенов
	apiURL := "https://api.mymemory.translated.net/get"
	
	// Маппинг языков для MyMemory
	langMap := map[string]string{
		"ru": "ru",
		"en": "en",
	}

	myMemoryLang, ok := langMap[targetLang]
	if !ok {
		return text, fmt.Errorf("unsupported target language: %s", targetLang)
	}

	reqURL := fmt.Sprintf("%s?q=%s&langpair=uz|%s", apiURL, url.QueryEscape(text), myMemoryLang)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return text, err
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		return text, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return text, fmt.Errorf("mymemory API error: %s", string(body))
	}

	var result struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		QuotaFinished bool `json:"quotaFinished"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return text, err
	}

	if result.QuotaFinished {
		return text, fmt.Errorf("mymemory quota finished")
	}

	if result.ResponseData.TranslatedText == "" {
		return text, fmt.Errorf("no translation returned")
	}

	return result.ResponseData.TranslatedText, nil
}

// min возвращает минимум из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
