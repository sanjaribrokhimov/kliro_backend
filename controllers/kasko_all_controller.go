package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"kliro/config"
	"kliro/utils"
)

type KaskoAllController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewKaskoAllController(cfg *config.Config) *KaskoAllController {
	return &KaskoAllController{
		cfg: cfg,
		cl:  &http.Client{Timeout: 30 * time.Second},
	}
}

// -------- session ----------

type KaskoStartResponse struct {
	SessionID    string   `json:"session_id"`
	ProductTypes []string `json:"product_types"`
	Providers    []string `json:"providers"`
}

func (kc *KaskoAllController) Start(c *gin.Context) {
	sessionID := uuid.New().String()

	// keep minimal session payload; client will update it via calculate
	kc.saveSession(sessionID, map[string]interface{}{"created_at": time.Now().Format(time.RFC3339)})

	c.JSON(http.StatusOK, KaskoStartResponse{
		SessionID:    sessionID,
		ProductTypes: []string{"full_kasko", "mini_kasko", "euro_kasko"},
		Providers:    []string{"neo", "gross", "trust", "euroasia"},
	})
}

func (kc *KaskoAllController) sessionKey(sessionID string) string {
	return "kasko_all:session:" + sessionID
}

func (kc *KaskoAllController) saveSession(sessionID string, payload interface{}) {
	rdb := utils.GetRedis()
	if rdb == nil {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = rdb.Set(context.Background(), kc.sessionKey(sessionID), b, 30*time.Minute).Err()
}

func (kc *KaskoAllController) getSession(sessionID string) (map[string]interface{}, error) {
	rdb := utils.GetRedis()
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}
	val, err := rdb.Get(context.Background(), kc.sessionKey(sessionID)).Result()
	if err != nil {
		return nil, fmt.Errorf("session not found: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %v", err)
	}
	return out, nil
}

// -------- models ----------

type KaskoProductType string

const (
	ProductFull KaskoProductType = "full_kasko"
	ProductMini KaskoProductType = "mini_kasko"
	ProductEuro KaskoProductType = "euro_kasko"
)

// KaskoCalculateRequest — единый запрос расчёта.
// Важно: мы делаем «максимальное сравнение»: считаем всё, что можем, а если данных не хватает —
// возвращаем missing_fields по каждому провайдеру.
type KaskoCalculateRequest struct {
	SessionID    string          `json:"session_id" binding:"required"`
	ProductType  KaskoProductType `json:"product_type" binding:"required"`
	Days         int             `json:"days,omitempty"` // важно для trust; можно поддержать для всех
	Providers    []string        `json:"providers,omitempty"` // если пусто — считаем всех подходящих
	Language     string          `json:"language,omitempty"`  // ru/uz/en (если провайдер поддерживает)

	Vehicle VehicleInput `json:"vehicle,omitempty"`
	// Full KASKO inputs
	CarPriceUZS   *int64   `json:"car_price_uzs,omitempty"`   // можно передать сразу (Neo)
	PercentOfCar  *float64 `json:"percent_of_car,omitempty"`  // Trust (0.1..1.0)
	AutoNS        *int64   `json:"auto_ns,omitempty"`          // Trust
	AutoAGO       *int64   `json:"auto_ago,omitempty"`         // Trust
	// Euro KASKO inputs
	Risks []string `json:"risks,omitempty"` // EuroAsia
	// Gross inputs
	GrossTariffID *int64 `json:"gross_tariff_id,omitempty"`
	// Neo inputs
	NeoTariffID *int64  `json:"neo_tariff_id,omitempty"` // optional, если захотим выбрать тариф заранее
	NeoAddons   []int64 `json:"neo_addons,omitempty"`    // konstruktor ids (optional)
}

type VehicleInput struct {
	Year *int `json:"year,omitempty"`
	// Provider-specific references (если есть)
	NeoCarPositionID   *int64 `json:"neo_car_position_id,omitempty"`
	GrossAutoCompID    *int64 `json:"gross_autocomp_id,omitempty"`
	TrustModelID       *int64 `json:"trust_model_id,omitempty"`
	// Optional: keep client selections (not used by providers directly)
	BrandName string `json:"brand_name,omitempty"`
	ModelName string `json:"model_name,omitempty"`
	TrimName  string `json:"trim_name,omitempty"`
}

type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type Offer struct {
	Provider    string                 `json:"provider"`
	ProductType KaskoProductType        `json:"product_type"`
	PlanName    string                 `json:"plan_name,omitempty"` // Basic/Comfort/Premium/SILVER...
	PlanID      string                 `json:"plan_id,omitempty"`   // provider-specific id if needed
	Premium     *Money                 `json:"premium,omitempty"`
	SumInsured  *Money                 `json:"sum_insured,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
	Raw         interface{}            `json:"raw,omitempty"`
}

type ProviderResult struct {
	Provider      string   `json:"provider"`
	Offers        []Offer  `json:"offers,omitempty"`
	MissingFields []string `json:"missing_fields,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

type KaskoCalculateResponse struct {
	SessionID string           `json:"session_id"`
	ProductType KaskoProductType `json:"product_type"`
	Results  []ProviderResult  `json:"results"`
	Errors   []string          `json:"errors,omitempty"`
}

// -------- handlers ----------

func (kc *KaskoAllController) Calculate(c *gin.Context) {
	var req KaskoCalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// verify session exists; also update it with last request snapshot
	if _, err := kc.getSession(req.SessionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session_id", "details": err.Error()})
		return
	}
	kc.saveSession(req.SessionID, map[string]interface{}{"last_calculate_request": req, "updated_at": time.Now().Format(time.RFC3339)})

	providerSet := make(map[string]bool)
	for _, p := range req.Providers {
		providerSet[strings.ToLower(strings.TrimSpace(p))] = true
	}
	allowProvider := func(p string) bool {
		if len(providerSet) == 0 {
			return true
		}
		return providerSet[p]
	}

	lang := strings.ToLower(strings.TrimSpace(req.Language))
	if lang == "" {
		lang = "ru"
	}

	results := make([]ProviderResult, 0)

	switch req.ProductType {
	case ProductMini:
		if allowProvider("neo") {
			results = append(results, kc.calcNeoMini(lang))
		}
	case ProductEuro:
		if allowProvider("euroasia") {
			results = append(results, kc.calcEuroAsia(lang, req.Risks))
		}
	case ProductFull:
		if allowProvider("neo") {
			results = append(results, kc.calcNeoFull(lang, req))
		}
		if allowProvider("gross") {
			results = append(results, kc.calcGross(lang, req))
		}
		if allowProvider("trust") {
			results = append(results, kc.calcTrust(lang, req))
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_type", "details": string(req.ProductType)})
		return
	}

	c.JSON(http.StatusOK, KaskoCalculateResponse{
		SessionID: req.SessionID,
		ProductType: req.ProductType,
		Results:  results,
	})
}

// -------- lookups (provider adapters, v1 minimal) ----------

func (kc *KaskoAllController) LookupsProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"product_types": []string{"full_kasko", "mini_kasko", "euro_kasko"},
		"providers":     []string{"neo", "gross", "trust", "euroasia"},
	})
}

func (kc *KaskoAllController) LookupsNeoCars(c *gin.Context) {
	kc.proxyNeo(c, http.MethodGet, "/api/sayt/kasko/cars", nil)
}

func (kc *KaskoAllController) LookupsNeoTariffs(c *gin.Context) {
	kc.proxyNeo(c, http.MethodGet, "/api/kasko-neo/get-tarif", nil)
}

func (kc *KaskoAllController) LookupsNeoMiniTariffs(c *gin.Context) {
	kc.proxyNeo(c, http.MethodGet, "/api/mini-casco/getTariffs", nil)
}

func (kc *KaskoAllController) LookupsTrustMarks(c *gin.Context) {
	kc.proxyTrust(c, http.MethodGet, "/api/v1/kasko/kasko-vehicle-marka", nil)
}

func (kc *KaskoAllController) LookupsTrustModels(c *gin.Context) {
	var body map[string]interface{}
	if id := c.Query("id"); id != "" {
		body = map[string]interface{}{"id": id}
	}
	if body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing query param id"})
		return
	}
	kc.proxyTrust(c, http.MethodPost, "/api/v1/kasko/kasko-vehicle-model", body)
}

func (kc *KaskoAllController) LookupsGrossBrands(c *gin.Context) {
	kc.proxyGross(c, http.MethodGet, "/ru/kasko-gross/autobrand", nil)
}

func (kc *KaskoAllController) LookupsGrossModels(c *gin.Context) {
	var body map[string]interface{}
	if id := c.Query("autobrand_id"); id != "" {
		body = map[string]interface{}{"autobrand_id": id}
	}
	if body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing query param autobrand_id"})
		return
	}
	kc.proxyGross(c, http.MethodPost, "/ru/kasko-gross/automodel", body)
}

func (kc *KaskoAllController) LookupsGrossComps(c *gin.Context) {
	var body map[string]interface{}
	if id := c.Query("automodel_id"); id != "" {
		body = map[string]interface{}{"automodel_id": id}
	}
	if body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing query param automodel_id"})
		return
	}
	kc.proxyGross(c, http.MethodPost, "/ru/kasko-gross/autocomp", body)
}

func (kc *KaskoAllController) LookupsGrossYears(c *gin.Context) {
	kc.proxyGross(c, http.MethodGet, "/ru/kasko-gross/years", nil)
}

func (kc *KaskoAllController) LookupsGrossTariffs(c *gin.Context) {
	kc.proxyGross(c, http.MethodGet, "/ru/kasko-gross/tariff", nil)
}

func (kc *KaskoAllController) LookupsEuroAsiaRisks(c *gin.Context) {
	kc.proxyEuroAsia(c, http.MethodGet, "/api/v1/insurance/lookups/kasko-risks", nil, c.GetHeader("Accept-Language"))
}

// -------- provider impl ----------

func (kc *KaskoAllController) basicAuthHeader(login, password string) string {
	creds := login + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (kc *KaskoAllController) doJSON(method, url string, headers map[string]string, body interface{}) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		r = bytes.NewBuffer(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("accept") == "" && req.Header.Get("Accept") == "" {
		req.Header.Set("accept", "application/json")
	}
	resp, err := kc.cl.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return b, resp.StatusCode, nil
}

func (kc *KaskoAllController) proxyNeo(c *gin.Context, method, path string, body interface{}) {
	url := kc.cfg.NeoBaseURL + path
	headers := map[string]string{
		"Authorization": kc.basicAuthHeader(kc.cfg.NeoLogin, kc.cfg.NeoPassword),
	}
	if al := c.GetHeader("Accept-Language"); al != "" {
		headers["Accept-Language"] = al
	}
	b, status, err := kc.doJSON(method, url, headers, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Status(status)
	_, _ = c.Writer.Write(b)
}

func (kc *KaskoAllController) proxyTrust(c *gin.Context, method, path string, body interface{}) {
	url := kc.cfg.TrustBaseURL + path
	headers := map[string]string{
		"Authorization": kc.basicAuthHeader(kc.cfg.TrustLogin, kc.cfg.TrustPassword),
	}
	if al := c.GetHeader("Accept-Language"); al != "" {
		headers["Accept-Language"] = al
	}
	b, status, err := kc.doJSON(method, url, headers, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Status(status)
	_, _ = c.Writer.Write(b)
}

func (kc *KaskoAllController) proxyGross(c *gin.Context, method, path string, body interface{}) {
	url := kc.cfg.GrossBaseURL + path
	headers := map[string]string{
		"Authorization": kc.basicAuthHeader(kc.cfg.GrossLogin, kc.cfg.GrossPassword),
	}
	if al := c.GetHeader("Accept-Language"); al != "" {
		headers["Accept-Language"] = al
	}
	b, status, err := kc.doJSON(method, url, headers, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Status(status)
	_, _ = c.Writer.Write(b)
}

func (kc *KaskoAllController) proxyEuroAsia(c *gin.Context, method, path string, body interface{}, acceptLanguage string) {
	url := kc.cfg.EuroasiaAllBaseURL + path
	headers := map[string]string{
		"Authorization": kc.cfg.EuroasiaAllAPIKey,
		"accept":        "application/json",
	}
	al := strings.TrimSpace(acceptLanguage)
	if al == "" {
		al = "ru"
	}
	headers["Accept-Language"] = al
	b, status, err := kc.doJSON(method, url, headers, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Status(status)
	_, _ = c.Writer.Write(b)
}

// -------- calculations ----------

func (kc *KaskoAllController) calcNeoMini(lang string) ProviderResult {
	res := ProviderResult{Provider: "neo"}
	url := kc.cfg.NeoBaseURL + "/api/mini-casco/getTariffs"
	headers := map[string]string{
		"Authorization":   kc.basicAuthHeader(kc.cfg.NeoLogin, kc.cfg.NeoPassword),
		"Accept-Language": lang,
	}
	b, status, err := kc.doJSON(http.MethodGet, url, headers, nil)
	if err != nil {
		res.Errors = []string{err.Error()}
		return res
	}
	if status != http.StatusOK {
		res.Errors = []string{fmt.Sprintf("neo status %d: %s", status, string(b))}
		return res
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)
	tariffs, _ := raw["tariffs"].([]interface{})
	for _, t := range tariffs {
		m, _ := t.(map[string]interface{})
		if m == nil {
			continue
		}
		name := asString(m["name"])
		price := extractInt64(m["price"])
		liability := extractInt64(m["liability"])
		offer := Offer{
			Provider:    "neo",
			ProductType: ProductMini,
			PlanName:    name,
			PlanID:      asString(m["id"]),
			Premium:     &Money{Amount: price, Currency: "UZS"},
			SumInsured:  &Money{Amount: liability, Currency: "UZS"},
			Raw:         m,
		}
		res.Offers = append(res.Offers, offer)
	}
	return res
}

func (kc *KaskoAllController) calcEuroAsia(lang string, risks []string) ProviderResult {
	res := ProviderResult{Provider: "euroasia"}
	if len(risks) == 0 {
		res.MissingFields = []string{"risks"}
		return res
	}
	url := kc.cfg.EuroasiaAllBaseURL + "/api/v1/insurance/euro-kasko/calculate"
	headers := map[string]string{
		"Authorization":   kc.cfg.EuroasiaAllAPIKey,
		"Accept-Language": lang,
		"Content-Type":    "application/json",
		"accept":          "application/json",
	}
	body := map[string]interface{}{"risks": risks}
	b, status, err := kc.doJSON(http.MethodPost, url, headers, body)
	if err != nil {
		res.Errors = []string{err.Error()}
		return res
	}
	if status != http.StatusOK {
		res.Errors = []string{fmt.Sprintf("euroasia status %d: %s", status, string(b))}
		return res
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)
	data, _ := raw["data"].(map[string]interface{})
	prem := extractNestedInt64(data, "premium", "amount")
	sum := extractNestedInt64(data, "sum", "amount")
	offer := Offer{
		Provider:    "euroasia",
		ProductType: ProductEuro,
		PlanName:    "Euro Kasko",
		Premium:     &Money{Amount: prem, Currency: "UZS"},
		SumInsured:  &Money{Amount: sum, Currency: "UZS"},
		Raw:         raw,
	}
	res.Offers = []Offer{offer}
	return res
}

func (kc *KaskoAllController) calcNeoFull(lang string, req KaskoCalculateRequest) ProviderResult {
	res := ProviderResult{Provider: "neo"}

	// Determine car price:
	var price int64
	if req.CarPriceUZS != nil && *req.CarPriceUZS > 0 {
		price = *req.CarPriceUZS
	} else if req.Vehicle.NeoCarPositionID != nil && req.Vehicle.Year != nil {
		// call car_price_cal
		url := kc.cfg.NeoBaseURL + "/api/kasko-neo/car_price_cal"
		headers := map[string]string{
			"Authorization":   kc.basicAuthHeader(kc.cfg.NeoLogin, kc.cfg.NeoPassword),
			"Accept-Language": lang,
			"Content-Type":    "application/json",
			"accept":          "application/json",
		}
		body := map[string]interface{}{
			"car_position_id": *req.Vehicle.NeoCarPositionID,
			"year":            *req.Vehicle.Year,
		}
		b, status, err := kc.doJSON(http.MethodPost, url, headers, body)
		if err != nil {
			res.Errors = []string{err.Error()}
			return res
		}
		if status != http.StatusOK {
			res.Errors = []string{fmt.Sprintf("neo car_price_cal status %d: %s", status, string(b))}
			return res
		}
		var raw map[string]interface{}
		_ = json.Unmarshal(b, &raw)
		price = extractInt64(raw["price"])
		if price <= 0 {
			res.Errors = []string{"neo car_price_cal returned empty price"}
			return res
		}
	} else {
		res.MissingFields = []string{"car_price_uzs (or vehicle.neo_car_position_id + vehicle.year)"}
		return res
	}

	// call hisoblash
	url := kc.cfg.NeoBaseURL + "/api/kasko-neo/hisoblash"
	headers := map[string]string{
		"Authorization":   kc.basicAuthHeader(kc.cfg.NeoLogin, kc.cfg.NeoPassword),
		"Accept-Language": lang,
		"Content-Type":    "application/json",
		"accept":          "application/json",
	}
	b, status, err := kc.doJSON(http.MethodPost, url, headers, map[string]interface{}{"price": price})
	if err != nil {
		res.Errors = []string{err.Error()}
		return res
	}
	if status != http.StatusOK {
		res.Errors = []string{fmt.Sprintf("neo hisoblash status %d: %s", status, string(b))}
		return res
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)

	offers := make([]Offer, 0, 3)
	// Neo returns tarif_1/2/3 without names in calc response; map to Basic/Comfort/Premium by convention from get-tarif.
	for i, plan := range []string{"Basic", "Comfort", "Premium"} {
		key := fmt.Sprintf("tarif_%d", i+1)
		amt := extractNestedInt64(raw, key)
		if amt <= 0 {
			continue
		}
		offers = append(offers, Offer{
			Provider:    "neo",
			ProductType: ProductFull,
			PlanName:    plan,
			Premium:     &Money{Amount: amt, Currency: "UZS"},
			SumInsured:  &Money{Amount: price, Currency: "UZS"},
			Meta: map[string]interface{}{
				"car_price_uzs": price,
			},
			Raw: raw,
		})
	}
	res.Offers = offers
	if len(res.Offers) == 0 {
		res.Errors = []string{"neo returned empty tariffs"}
	}
	return res
}

func (kc *KaskoAllController) calcGross(lang string, req KaskoCalculateRequest) ProviderResult {
	res := ProviderResult{Provider: "gross"}
	if req.Vehicle.GrossAutoCompID == nil {
		res.MissingFields = append(res.MissingFields, "vehicle.gross_autocomp_id")
	}
	if req.Vehicle.Year == nil {
		res.MissingFields = append(res.MissingFields, "vehicle.year")
	}
	if req.GrossTariffID == nil {
		res.MissingFields = append(res.MissingFields, "gross_tariff_id")
	}
	if len(res.MissingFields) > 0 {
		return res
	}

	url := kc.cfg.GrossBaseURL + "/ru/kasko-gross/calc-kasko"
	headers := map[string]string{
		"Authorization":   kc.basicAuthHeader(kc.cfg.GrossLogin, kc.cfg.GrossPassword),
		"Accept-Language": lang,
		"Content-Type":    "application/json",
		"accept":          "application/json",
	}
	body := map[string]interface{}{
		"autocomp_id": *req.Vehicle.GrossAutoCompID,
		"tariff_id":   *req.GrossTariffID,
		"year":        *req.Vehicle.Year,
	}
	b, status, err := kc.doJSON(http.MethodPost, url, headers, body)
	if err != nil {
		res.Errors = []string{err.Error()}
		return res
	}
	if status != http.StatusOK {
		res.Errors = []string{fmt.Sprintf("gross status %d: %s", status, string(b))}
		return res
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)
	resp, _ := raw["response"].(map[string]interface{})
	amt := extractInt64(resp["amount_uzs"])
	autoPrice := extractInt64(resp["autoprice"])
	offer := Offer{
		Provider:    "gross",
		ProductType: ProductFull,
		PlanName:    "Gross Kasko",
		PlanID:      fmt.Sprintf("%d", *req.GrossTariffID),
		Premium:     &Money{Amount: amt, Currency: "UZS"},
		SumInsured:  &Money{Amount: autoPrice, Currency: "UZS"},
		Raw:         raw,
	}
	res.Offers = []Offer{offer}
	return res
}

func (kc *KaskoAllController) calcTrust(lang string, req KaskoCalculateRequest) ProviderResult {
	res := ProviderResult{Provider: "trust"}
	if req.Vehicle.TrustModelID == nil {
		res.MissingFields = append(res.MissingFields, "vehicle.trust_model_id")
	}
	if req.Vehicle.Year == nil {
		res.MissingFields = append(res.MissingFields, "vehicle.year")
	}
	if req.PercentOfCar == nil {
		res.MissingFields = append(res.MissingFields, "percent_of_car")
	}
	if req.Days <= 0 {
		res.MissingFields = append(res.MissingFields, "days")
	}
	if req.AutoNS == nil {
		res.MissingFields = append(res.MissingFields, "auto_ns")
	}
	if req.AutoAGO == nil {
		res.MissingFields = append(res.MissingFields, "auto_ago")
	}
	if len(res.MissingFields) > 0 {
		return res
	}

	url := kc.cfg.TrustBaseURL + "/api/v1/kasko/cal-prem"
	headers := map[string]string{
		"Authorization":   kc.basicAuthHeader(kc.cfg.TrustLogin, kc.cfg.TrustPassword),
		"Accept-Language": lang,
		"Content-Type":    "application/json",
		"accept":          "application/json",
	}
	body := map[string]interface{}{
		"issue_year":     *req.Vehicle.Year,
		"model_id":       *req.Vehicle.TrustModelID,
		"percent_of_car": *req.PercentOfCar,
		"days":           req.Days,
		"auto_ns":        *req.AutoNS,
		"auto_ago":       *req.AutoAGO,
	}
	b, status, err := kc.doJSON(http.MethodPost, url, headers, body)
	if err != nil {
		res.Errors = []string{err.Error()}
		return res
	}
	if status != http.StatusOK {
		res.Errors = []string{fmt.Sprintf("trust status %d: %s", status, string(b))}
		return res
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)

	// Trust returns otv_* (limits) and prem_* (premiums). We'll use prem_ns+prem_ago as addons and otv_all as sum insured.
	premNS := extractInt64(raw["prem_ns"])
	premAGO := extractInt64(raw["prem_ago"])
	premAuto := extractInt64(raw["prem_auto"])
	otvAll := extractInt64(raw["otv_all"])
	total := premAuto + premNS + premAGO

	meta := map[string]interface{}{
		"prem_auto": premAuto,
		"prem_ns":   premNS,
		"prem_ago":  premAGO,
		"otv_auto":  extractInt64(raw["otv_auto"]),
		"otv_ns":    extractInt64(raw["otv_ns"]),
		"otv_ago":   extractInt64(raw["otv_ago"]),
	}

	offer := Offer{
		Provider:    "trust",
		ProductType: ProductFull,
		PlanName:    "Trust Kasko",
		Premium:     &Money{Amount: total, Currency: "UZS"},
		SumInsured:  &Money{Amount: otvAll, Currency: "UZS"},
		Meta:        meta,
		Raw:         raw,
	}
	res.Offers = []Offer{offer}
	return res
}

// -------- small helpers ----------

func extractInt64(v interface{}) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case json.Number:
		i, _ := t.Int64()
		return i
	case string:
		// trust/gross sometimes numbers in strings
		n := json.Number(strings.TrimSpace(t))
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func extractNestedInt64(m interface{}, path ...string) int64 {
	cur := m
	for _, p := range path {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return 0
		}
		cur = mm[p]
	}
	return extractInt64(cur)
}

