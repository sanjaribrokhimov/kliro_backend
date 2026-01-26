package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"kliro/config"
	"kliro/utils"
)

// asString converts common JSON-unmarshaled types to string (string/float64/json.Number/int/etc).
// Useful when external APIs sometimes return numeric IDs that we need as strings (e.g., PINFL).
func asString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		// JSON numbers become float64 when unmarshaled into interface{}
		// PINFL is an integer-like identifier, so we keep 0 decimals.
		return strings.TrimSpace(strconv.FormatFloat(t, 'f', 0, 64))
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return strings.TrimSpace(t.String())
	default:
		return ""
	}
}

type OsagoAllController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewOsagoAllController(cfg *config.Config) *OsagoAllController {
	return &OsagoAllController{
		cfg: cfg,
		cl:  &http.Client{Timeout: 30 * time.Second},
	}
}

// FindRequest - структура запроса для поиска
type FindRequest struct {
	// Поля для поиска машины
	LicensePlate       *string `json:"license_plate,omitempty"`
	TechPassportNumber *string `json:"tech_passport_number,omitempty"`
	TechPassportSeries *string `json:"tech_passport_series,omitempty"`

	// Поля для поиска человека
	Birthdate      *string `json:"birthdate,omitempty"`
	PassportNumber *string `json:"passport_number,omitempty"`
	PassportSeries *string `json:"passport_series,omitempty"`
	Pinfl          *string `json:"pinfl,omitempty"`
}

// FindResponse - структура ответа
type FindResponse struct {
	SessionID string      `json:"session_id,omitempty"`
	Owner     *bool       `json:"owner,omitempty"`
	Vehicle   interface{} `json:"vehicle,omitempty"`
	Person    interface{} `json:"person,omitempty"`
	Errors    []string    `json:"errors,omitempty"`
}

// OsagoAllCalcRequest - структура запроса для расчета
type OsagoAllCalcRequest struct {
	SessionID       string `json:"session_id" binding:"required"`
	PeriodID        *int   `json:"period_id" binding:"required"`         // только 12 или 6 месяцев
	NumberDriversID *int   `json:"number_drivers_id" binding:"required"` // только 0 (unlimited) или 5 (limited)
}

func (oc *OsagoAllController) makeExternalRequest(method, path string, bodyData interface{}) ([]byte, int, error) {
	url := oc.cfg.EuroasiaAllBaseURL + path

	var body io.Reader
	if bodyData != nil {
		jsonData, err := json.Marshal(bodyData)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("Accept-Language", "ru")
	req.Header.Set("Authorization", oc.cfg.EuroasiaAllAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := oc.cl.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

// Find - универсальный метод для поиска машины и/или человека
func (oc *OsagoAllController) Find(c *gin.Context) {
	var req FindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	response := FindResponse{
		Errors: []string{},
	}

	// Проверяем, есть ли данные для поиска машины
	hasVehicleData := req.LicensePlate != nil || req.TechPassportNumber != nil || req.TechPassportSeries != nil

	// Проверяем, есть ли данные для поиска человека
	hasPersonData := req.Birthdate != nil || req.PassportNumber != nil || req.PassportSeries != nil || req.Pinfl != nil

	if !hasVehicleData && !hasPersonData {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one search parameter is required"})
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Поиск машины (параллельно)
	if hasVehicleData {
		wg.Add(1)
		go func() {
			defer wg.Done()

			vehicleReq := map[string]string{}
			if req.LicensePlate != nil {
				vehicleReq["license_plate"] = *req.LicensePlate
			}
			if req.TechPassportNumber != nil {
				vehicleReq["tech_passport_number"] = *req.TechPassportNumber
			}
			if req.TechPassportSeries != nil {
				vehicleReq["tech_passport_series"] = *req.TechPassportSeries
			}

			vehicleBody, statusCode, err := oc.makeExternalRequest(
				http.MethodPost,
				"/api/v1/insurance/vehicles/find",
				vehicleReq,
			)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				response.Errors = append(response.Errors, "vehicle search error: "+err.Error())
			} else {
				var vehicleData interface{}
				if err := json.Unmarshal(vehicleBody, &vehicleData); err != nil {
					response.Errors = append(response.Errors, "vehicle response parse error: "+err.Error())
				} else {
					if statusCode == http.StatusOK {
						response.Vehicle = vehicleData
					} else {
						response.Errors = append(response.Errors, "vehicle search failed with status "+strconv.Itoa(statusCode))
					}
				}
			}
		}()
	}

	// Поиск человека (параллельно)
	if hasPersonData {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var personPath string
			personReq := map[string]string{}

			// Если есть ПИНФЛ, используем find-by-pinfl
			if req.Pinfl != nil {
				personPath = "/api/v1/insurance/persons/find-by-pinfl"
				personReq["pinfl"] = *req.Pinfl
				if req.PassportNumber != nil {
					personReq["passport_number"] = *req.PassportNumber
				}
				if req.PassportSeries != nil {
					personReq["passport_series"] = *req.PassportSeries
				}
			} else {
				// Иначе используем find-by-birthdate
				personPath = "/api/v1/insurance/persons/find-by-birthdate"
				if req.Birthdate != nil {
					personReq["birthdate"] = *req.Birthdate
				}
				if req.PassportNumber != nil {
					personReq["passport_number"] = *req.PassportNumber
				}
				if req.PassportSeries != nil {
					personReq["passport_series"] = *req.PassportSeries
				}
			}

			personBody, statusCode, err := oc.makeExternalRequest(
				http.MethodPost,
				personPath,
				personReq,
			)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				response.Errors = append(response.Errors, "person search error: "+err.Error())
			} else {
				var personData interface{}
				if err := json.Unmarshal(personBody, &personData); err != nil {
					response.Errors = append(response.Errors, "person response parse error: "+err.Error())
				} else {
					if statusCode == http.StatusOK {
						response.Person = personData
					} else {
						response.Errors = append(response.Errors, "person search failed with status "+strconv.Itoa(statusCode))
					}
				}
			}
		}()
	}

	// Ждем завершения всех запросов
	wg.Wait()

	// Проверяем, является ли человек владельцем машины
	if response.Vehicle != nil && response.Person != nil {
		owner := oc.checkOwner(response.Vehicle, response.Person)
		response.Owner = &owner
	}

	// Генерируем session_id и сохраняем данные в Redis
	sessionID := uuid.New().String()
	response.SessionID = sessionID

	// Сохраняем все данные в Redis
	rdb := utils.GetRedis()
	if rdb != nil {
		ctx := context.Background()
		redisKey := "osago_all:session:" + sessionID

		sessionData := map[string]interface{}{
			"vehicle": response.Vehicle,
			"person":  response.Person,
			"owner":   response.Owner,
		}

		sessionDataJSON, err := json.Marshal(sessionData)
		if err == nil {
			rdb.Set(ctx, redisKey, sessionDataJSON, 30*time.Minute)
		}
	}

	// Возвращаем ответ
	if len(response.Errors) > 0 && response.Vehicle == nil && response.Person == nil {
		c.JSON(http.StatusBadGateway, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Calc - метод для расчета на основе сохраненных данных из find
func (oc *OsagoAllController) Calc(c *gin.Context) {
	var req OsagoAllCalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Получаем данные из Redis
	rdb := utils.GetRedis()
	if rdb == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redis not available"})
		return
	}

	ctx := context.Background()
	redisKey := "osago_all:session:" + req.SessionID

	sessionDataStr, err := rdb.Get(ctx, redisKey).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found or expired"})
		return
	}

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(sessionDataStr), &sessionData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse session data"})
		return
	}

	// Валидация period_id (3, 6, 12 или 20)
	if req.PeriodID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period_id is required"})
		return
	}
	if *req.PeriodID != 3 && *req.PeriodID != 6 && *req.PeriodID != 12 && *req.PeriodID != 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period_id must be 3, 6, 12 or 20"})
		return
	}

	// Валидация number_drivers_id (только 0 или 5)
	if req.NumberDriversID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "number_drivers_id is required"})
		return
	}
	if *req.NumberDriversID != 0 && *req.NumberDriversID != 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "number_drivers_id must be 0 or 5"})
		return
	}

	// Извлекаем данные vehicle из session
	vehicleData, ok := sessionData["vehicle"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vehicle data not found in session"})
		return
	}

	// Извлекаем нужные поля из vehicle
	var gosNumber, techSery, techNumber string
	var vehicleTypeID int
	var vehicleGroupID string
	var useTerritoryRegionID string
	var useTerritoryRegionExternalID int
	var ownerPinfl, ownerPassportSeries, ownerPassportNumber string

	if data, ok := vehicleData["data"].(map[string]interface{}); ok {
		// license_plate -> gos_number
		if lp, ok := data["license_plate"].(string); ok {
			gosNumber = lp
		}

		// tech_passport
		if tp, ok := data["tech_passport"].(map[string]interface{}); ok {
			if series, ok := tp["series"].(string); ok {
				techSery = series
			}
			if number, ok := tp["number"].(string); ok {
				techNumber = number
			}
		}

		// vehicle_type.external_id и vehicle_group для маппинга в Gross vehicleTypeId
		if vt, ok := data["vehicle_type"].(map[string]interface{}); ok {
			if externalID, ok := vt["external_id"].(float64); ok {
				vehicleTypeID = int(externalID)
			}
			if vg, ok := vt["vehicle_group"].(string); ok {
				vehicleGroupID = vg
			}
		}

		// use_territory_region для Euroasia и Apex
		if utr, ok := data["use_territory_region"].(map[string]interface{}); ok {
			if id, ok := utr["id"].(string); ok {
				useTerritoryRegionID = id
			}
			if externalID, ok := utr["external_id"].(float64); ok {
				useTerritoryRegionExternalID = int(externalID)
			}
		}

		// owner.person данные для Apex
		if owner, ok := data["owner"].(map[string]interface{}); ok {
			if ownerType, ok := owner["type"].(string); ok && ownerType == "person" {
				if person, ok := owner["person"].(map[string]interface{}); ok {
					// pinfl из external_id
					ownerPinfl = asString(person["external_id"])
					// passport данные
					if passport, ok := person["passport"].(map[string]interface{}); ok {
						if series, ok := passport["series"].(string); ok {
							ownerPassportSeries = series
						}
						if number, ok := passport["number"].(string); ok {
							ownerPassportNumber = number
						}
					}
				}
			}
		}
	}

	// Извлекаем данные person для drivers (Apex)
	// Если person нет, используем owner.person как fallback
	var personPinfl, personPassportSeries, personPassportNumber string
	personData, personOk := sessionData["person"].(map[string]interface{})
	if personOk {
		if pData, ok := personData["data"].(map[string]interface{}); ok {
			// pinfl из external_id
			personPinfl = asString(pData["external_id"])
			// passport данные
			if passport, ok := pData["passport"].(map[string]interface{}); ok {
				if series, ok := passport["series"].(string); ok {
					personPassportSeries = series
				}
				if number, ok := passport["number"].(string); ok {
					personPassportNumber = number
				}
			}
		}
	}

	// Если person данных нет, используем owner.person как fallback
	if personPinfl == "" || personPassportSeries == "" || personPassportNumber == "" {
		if vehicleData, ok := sessionData["vehicle"].(map[string]interface{}); ok {
			if data, ok := vehicleData["data"].(map[string]interface{}); ok {
				if owner, ok := data["owner"].(map[string]interface{}); ok {
					if ownerType, ok := owner["type"].(string); ok && ownerType == "person" {
						if person, ok := owner["person"].(map[string]interface{}); ok {
							if personPinfl == "" {
								personPinfl = asString(person["external_id"])
							}
							// passport данные
							if passport, ok := person["passport"].(map[string]interface{}); ok {
								if personPassportSeries == "" {
									if series, ok := passport["series"].(string); ok {
										personPassportSeries = series
									}
								}
								if personPassportNumber == "" {
									if number, ok := passport["number"].(string); ok {
										personPassportNumber = number
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if gosNumber == "" || techSery == "" || techNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "required vehicle data missing in session"})
		return
	}

	if vehicleTypeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vehicle_type_id not found in vehicle data"})
		return
	}

	// Маппинг vehicle_type.external_id из find API в vehicleTypeId для Gross
	grossVehicleTypeID := oc.mapVehicleTypeToGross(vehicleTypeID, vehicleGroupID)
	if grossVehicleTypeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported vehicle type for Gross API"})
		return
	}

	// Маппинг period_id: 12 -> 1, 6 -> 2 (для Neo и Gross)
	// 20 дней поддерживается только для Euroasia
	periodID := 1 // 12 месяцев
	if *req.PeriodID == 6 {
		periodID = 2 // 6 месяцев
	}
	// Для 20 дней periodID остается 1 для Neo/Gross (они не поддерживают 20 дней)

	// Маппинг number_drivers_id: 0 -> 1 (unlimited), 5 -> 4 (limited) (для обоих API одинаково)
	numberDriversID := 1 // unlimited
	if *req.NumberDriversID == 5 {
		numberDriversID = 4 // limited to 5
	}

	// Инициализируем переменные для ответов
	var neoResponseData interface{}
	var grossResponseData interface{}

	// Neo и Gross отправляются только если period_id != 20 и != 3 (они не поддерживают 20 дней и 2 месяца)
	if *req.PeriodID != 20 && *req.PeriodID != 3 {
		// Формируем запрос к Neo Insurance (period_id и number_drivers_id как строки)
		neoRequest := map[string]string{
			"gos_number":        gosNumber,
			"tech_sery":         techSery,
			"tech_number":       techNumber,
			"period_id":         strconv.Itoa(periodID),
			"number_drivers_id": strconv.Itoa(numberDriversID),
		}

		// Отправляем запрос на Neo Insurance
		neoURL := oc.cfg.NeoBaseURL + "/api/osago-neo/get-calc-osago"

		jsonData, err := json.Marshal(neoRequest)
		if err == nil {
			// Логируем запрос к Neo
			log.Printf("[NEO CALC] URL: %s", neoURL)
			log.Printf("[NEO CALC] Request: %s", string(jsonData))

			httpReq, err := http.NewRequest(http.MethodPost, neoURL, bytes.NewBuffer(jsonData))
			if err == nil {
				// Устанавливаем заголовки для Neo Insurance
				httpReq.Header.Set("Content-Type", "application/json")

				creds := oc.cfg.NeoLogin + ":" + oc.cfg.NeoPassword
				auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
				httpReq.Header.Set("Authorization", auth)

				resp, err := oc.cl.Do(httpReq)
				if err == nil {
					defer resp.Body.Close()
					neoResponseBody, err := io.ReadAll(resp.Body)
					if err == nil {
						// Логируем ответ от Neo
						log.Printf("[NEO CALC] Status: %d", resp.StatusCode)
						log.Printf("[NEO CALC] Response: %s", string(neoResponseBody))

						if err := json.Unmarshal(neoResponseBody, &neoResponseData); err != nil {
							neoResponseData = string(neoResponseBody)
						}
					} else {
						log.Printf("[NEO CALC] Error reading response: %v", err)
					}
				} else {
					log.Printf("[NEO CALC] Error sending request: %v", err)
				}
			} else {
				log.Printf("[NEO CALC] Error creating request: %v", err)
			}
		} else {
			log.Printf("[NEO CALC] Error marshaling request: %v", err)
		}

		// Формируем запрос к Gross Insurance (все поля как числа)
		grossRequest := map[string]interface{}{
			"gos_number":        gosNumber,
			"tech_sery":         techSery,
			"tech_number":       techNumber,
			"period_id":         periodID,
			"number_drivers_id": numberDriversID,
			"vehicleTypeId":     grossVehicleTypeID,
		}

		// Отправляем запрос на Gross Insurance
		grossURL := oc.cfg.GrossBaseURL + "/osago-gross/get-calc-osago"

		grossJsonData, err := json.Marshal(grossRequest)
		if err == nil {
			grossHttpReq, err := http.NewRequest(http.MethodPost, grossURL, bytes.NewBuffer(grossJsonData))
			if err == nil {
				grossHttpReq.Header.Set("Content-Type", "application/json")
				grossHttpReq.Header.Set("Accept", "application/json")
				grossHttpReq.SetBasicAuth(oc.cfg.GrossLogin, oc.cfg.GrossPassword)

				grossResp, err := oc.cl.Do(grossHttpReq)
				if err == nil {
					defer grossResp.Body.Close()
					grossResponseBody, err := io.ReadAll(grossResp.Body)
					if err == nil {
						if err := json.Unmarshal(grossResponseBody, &grossResponseData); err != nil {
							grossResponseData = string(grossResponseBody)
						}
					}
				}
			}
		}
	}

	// Формируем и отправляем запрос на Euroasia Insurance
	// Euroasia отправляется только если period_id != 3 (не поддерживает 2 месяца)
	var euroasiaResponseData interface{} = map[string]interface{}{"error": "euroasia calculation was not attempted"}
	if *req.PeriodID != 3 {
		// Логируем данные для отладки
		log.Printf("[EUROASIA DEBUG] useTerritoryRegionID: %s, vehicleGroupID: %s", useTerritoryRegionID, vehicleGroupID)
		log.Printf("[EUROASIA DEBUG] period_id: %d, number_drivers_id: %d", *req.PeriodID, *req.NumberDriversID)
		if _, ok := sessionData["person"].(map[string]interface{}); ok {
			log.Printf("[EUROASIA DEBUG] person data found in session")
		} else {
			log.Printf("[EUROASIA DEBUG] person data NOT found in session")
		}

		euroasiaRequest := oc.buildEuroasiaRequest(sessionData, req, useTerritoryRegionID, vehicleGroupID)
		if euroasiaRequest == nil {
			errorMsg := "euroasia request could not be built from session data"
			// Детализируем причину
			if useTerritoryRegionID == "" {
				errorMsg += ": useTerritoryRegionID is empty"
			}
			if vehicleGroupID == "" {
				errorMsg += ": vehicleGroupID is empty"
			}
			if _, ok := sessionData["person"].(map[string]interface{}); !ok {
				errorMsg += ": person data not found in session"
			}
			euroasiaResponseData = map[string]interface{}{"error": errorMsg}
		} else {
			euroasiaURL := oc.cfg.EuroasiaAllBaseURL + "/api/v1/insurance/osago/calculate"

			euroasiaJsonData, err := json.Marshal(euroasiaRequest)
			if err != nil {
				euroasiaResponseData = map[string]interface{}{"error": "failed to marshal euroasia request", "details": err.Error()}
			} else {
				log.Printf("[EUROASIA CALC] URL: %s", euroasiaURL)
				log.Printf("[EUROASIA CALC] Request: %s", string(euroasiaJsonData))

				euroasiaHttpReq, err := http.NewRequest(http.MethodPost, euroasiaURL, bytes.NewBuffer(euroasiaJsonData))
				if err != nil {
					euroasiaResponseData = map[string]interface{}{"error": "failed to create euroasia request", "details": err.Error()}
				} else {
					euroasiaHttpReq.Header.Set("Content-Type", "application/json")
					euroasiaHttpReq.Header.Set("Accept", "application/json")
					euroasiaHttpReq.Header.Set("Accept-Language", "ru")
					euroasiaHttpReq.Header.Set("Authorization", oc.cfg.EuroasiaAllAPIKey)

					euroasiaResp, err := oc.cl.Do(euroasiaHttpReq)
					if err != nil {
						euroasiaResponseData = map[string]interface{}{"error": "failed to send request to euroasia", "details": err.Error()}
					} else {
						defer euroasiaResp.Body.Close()
						euroasiaResponseBody, readErr := io.ReadAll(euroasiaResp.Body)
						if readErr != nil {
							euroasiaResponseData = map[string]interface{}{"error": "failed to read euroasia response", "details": readErr.Error()}
						} else {
							log.Printf("[EUROASIA CALC] Status: %d", euroasiaResp.StatusCode)
							log.Printf("[EUROASIA CALC] Response: %s", string(euroasiaResponseBody))

							// Try to decode JSON even on non-200 to preserve error info for client.
							var decoded interface{}
							if err := json.Unmarshal(euroasiaResponseBody, &decoded); err != nil {
								decoded = string(euroasiaResponseBody)
							}
							if euroasiaResp.StatusCode >= 200 && euroasiaResp.StatusCode < 300 {
								euroasiaResponseData = decoded
							} else {
								euroasiaResponseData = map[string]interface{}{
									"error":  "euroasia returned non-2xx status",
									"status": euroasiaResp.StatusCode,
									"body":   decoded,
								}
							}
						}
					}
				}
			}
		}
	}

	// Формируем и отправляем запрос на Apex Insurance
	// Apex отправляется только если period_id != 3 (не поддерживает 2 месяца)
	var apexResponseData interface{} = map[string]interface{}{"error": "apex calculation was not attempted"}
	if *req.PeriodID != 3 {
		// Логируем данные для отладки
		log.Printf("[APEX DEBUG] ownerPinfl: %s, ownerPassportSeries: %s, ownerPassportNumber: %s", ownerPinfl, ownerPassportSeries, ownerPassportNumber)
		log.Printf("[APEX DEBUG] personPinfl: %s, personPassportSeries: %s, personPassportNumber: %s", personPinfl, personPassportSeries, personPassportNumber)
		log.Printf("[APEX DEBUG] vehicleTypeID: %d, vehicleGroupID: %s", vehicleTypeID, vehicleGroupID)

		apexRequest := oc.buildApexRequest(sessionData, req,
			ownerPinfl, ownerPassportSeries, ownerPassportNumber,
			personPinfl, personPassportSeries, personPassportNumber,
			useTerritoryRegionExternalID, vehicleTypeID, vehicleGroupID,
			gosNumber, techSery, techNumber)
		if apexRequest == nil {
			errorMsg := "apex request could not be built from session data"
			// Детализируем причину
			if ownerPinfl == "" || ownerPassportSeries == "" || ownerPassportNumber == "" {
				errorMsg += ": owner passport data is missing"
			}
			if personPinfl == "" || personPassportSeries == "" || personPassportNumber == "" {
				errorMsg += ": person passport data is missing"
			}
			apexVehicleTypeID := oc.mapVehicleTypeToApex(vehicleTypeID, vehicleGroupID)
			if apexVehicleTypeID == 0 {
				errorMsg += ": unsupported vehicle type (vehicleTypeID=" + strconv.Itoa(vehicleTypeID) + ", vehicleGroupID=" + vehicleGroupID + ")"
			}
			apexResponseData = map[string]interface{}{"error": errorMsg}
		} else {
			apexURL := oc.cfg.ApexBaseURL + "/osago_calculation"

			apexJsonData, err := json.Marshal(apexRequest)
			if err != nil {
				apexResponseData = map[string]interface{}{"error": "failed to marshal apex request", "details": err.Error()}
			} else {
				log.Printf("[APEX CALC] URL: %s", apexURL)
				log.Printf("[APEX CALC] Request: %s", string(apexJsonData))

				apexHttpReq, err := http.NewRequest(http.MethodPost, apexURL, bytes.NewBuffer(apexJsonData))
				if err != nil {
					apexResponseData = map[string]interface{}{"error": "failed to create apex request", "details": err.Error()}
				} else {
					apexHttpReq.Header.Set("Content-Type", "application/json")
					apexHttpReq.Header.Set("Accept", "application/json")

					creds := oc.cfg.ApexLogin + ":" + oc.cfg.ApexPassword
					auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
					apexHttpReq.Header.Set("Authorization", auth)

					apexResp, err := oc.cl.Do(apexHttpReq)
					if err != nil {
						apexResponseData = map[string]interface{}{"error": "failed to send request to apex", "details": err.Error()}
					} else {
						defer apexResp.Body.Close()
						apexResponseBody, readErr := io.ReadAll(apexResp.Body)
						if readErr != nil {
							apexResponseData = map[string]interface{}{"error": "failed to read apex response", "details": readErr.Error()}
						} else {
							log.Printf("[APEX CALC] Status: %d", apexResp.StatusCode)
							log.Printf("[APEX CALC] Response: %s", string(apexResponseBody))

							// Try to decode JSON even on non-200 to preserve error info for client.
							var decoded interface{}
							if err := json.Unmarshal(apexResponseBody, &decoded); err != nil {
								decoded = string(apexResponseBody)
							}
							if apexResp.StatusCode >= 200 && apexResp.StatusCode < 300 {
								apexResponseData = decoded
							} else {
								apexResponseData = map[string]interface{}{
									"error":  "apex returned non-2xx status",
									"status": apexResp.StatusCode,
									"body":   decoded,
								}
							}
						}
					}
				}
			}
		}
	}

	// Формируем и отправляем запрос на Trust Insurance
	var trustResponseData interface{} = map[string]interface{}{"error": "trust calculation was not attempted"}
	trustRequest := oc.buildTrustRequest(
		ownerPinfl, personPinfl,
		useTerritoryRegionExternalID, vehicleTypeID, vehicleGroupID,
		gosNumber,
		*req.PeriodID, *req.NumberDriversID)
	if trustRequest == nil {
		trustResponseData = map[string]interface{}{"error": "trust request could not be built from session data"}
	} else {
		trustURL := oc.cfg.TrustBaseURL + "/api/osgo/v2/calc-prem"

		trustJsonData, err := json.Marshal(trustRequest)
		if err == nil {
			log.Printf("[TRUST CALC] URL: %s", trustURL)
			log.Printf("[TRUST CALC] Request: %s", string(trustJsonData))

			trustHttpReq, err := http.NewRequest(http.MethodPost, trustURL, bytes.NewBuffer(trustJsonData))
			if err == nil {
				trustHttpReq.Header.Set("Content-Type", "application/json")
				trustHttpReq.Header.Set("Accept", "application/json")

				creds := oc.cfg.TrustLogin + ":" + oc.cfg.TrustPassword
				auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
				trustHttpReq.Header.Set("Authorization", auth)

				trustResp, err := oc.cl.Do(trustHttpReq)
				if err != nil {
					trustResponseData = map[string]interface{}{"error": "failed to send request to trust", "details": err.Error()}
				} else {
					defer trustResp.Body.Close()
					trustResponseBody, readErr := io.ReadAll(trustResp.Body)
					if readErr != nil {
						trustResponseData = map[string]interface{}{"error": "failed to read trust response", "details": readErr.Error()}
					} else {
						log.Printf("[TRUST CALC] Status: %d", trustResp.StatusCode)
						log.Printf("[TRUST CALC] Response: %s", string(trustResponseBody))

						// Try to decode JSON even on non-200 to preserve error info for client.
						var decoded interface{}
						if err := json.Unmarshal(trustResponseBody, &decoded); err != nil {
							decoded = string(trustResponseBody)
						}
						if trustResp.StatusCode >= 200 && trustResp.StatusCode < 300 {
							trustResponseData = decoded
						} else {
							trustResponseData = map[string]interface{}{
								"error":  "trust returned non-2xx status",
								"status": trustResp.StatusCode,
								"body":   decoded,
							}
						}
					}
				}
			} else {
				trustResponseData = map[string]interface{}{"error": "failed to create trust request", "details": err.Error()}
			}
		} else {
			trustResponseData = map[string]interface{}{"error": "failed to marshal trust request", "details": err.Error()}
		}
	}

	// Извлекаем суммы из ответов провайдеров
	result := oc.extractProviderAmounts(neoResponseData, grossResponseData, euroasiaResponseData, apexResponseData, trustResponseData)

	// Сохраняем результат расчета в Redis для использования в create_policy
	calcResultKey := "osago_all:calc:" + req.SessionID
	calcResultData := map[string]interface{}{
		"result":            result,
		"neo":               neoResponseData,
		"gross":             grossResponseData,
		"euroasia":          euroasiaResponseData,
		"apex":              apexResponseData,
		"trust":             trustResponseData,
		"period_id":         req.PeriodID,
		"number_drivers_id": req.NumberDriversID,
	}
	calcResultJSON, err := json.Marshal(calcResultData)
	if err == nil {
		rdb.Set(ctx, calcResultKey, calcResultJSON, 30*time.Minute)
	}

	// Возвращаем ответ в JSON формате с полями result, neo, gross, euroasia, apex и trust
	// Используем структуру для гарантированного порядка полей
	type CalcResponse struct {
		SessionID string                 `json:"session_id"`
		Result    map[string]interface{} `json:"result"`
		Neo       interface{}            `json:"neo"`
		Gross     interface{}            `json:"gross"`
		Euroasia  interface{}            `json:"euroasia"`
		Apex      interface{}            `json:"apex"`
		Trust     interface{}            `json:"trust"`
		Success   bool                   `json:"success"`
	}
	c.JSON(http.StatusOK, CalcResponse{
		SessionID: req.SessionID,
		Result:    result,
		Neo:       neoResponseData,
		Gross:     grossResponseData,
		Euroasia:  euroasiaResponseData,
		Apex:      apexResponseData,
		Trust:     trustResponseData,
		Success:   true,
	})
}

// mapVehicleTypeToApex маппит vehicle_type.external_id из find API в vehicleTypeId для Apex API
func (oc *OsagoAllController) mapVehicleTypeToApex(externalID int, vehicleGroupID string) int {
	// Прямой маппинг по external_id
	switch externalID {
	case 2: // Легковые автомобили
		return 1
	case 6: // Грузовые автомобили
		return 6
	case 1: // Автобусы
		return 9
	}

	// Маппинг по vehicle_group для спецтехники
	// Группа "66b3d262-2d6b-49a9-949e-6c87765fcbef" -> 15
	if vehicleGroupID == "66b3d262-2d6b-49a9-949e-6c87765fcbef" {
		return 15
	}

	// Дополнительная проверка для типов из группы спецтехники
	specialEquipmentTypes := map[int]bool{
		8:  true, // Спецтехника
		13: true, // Трамваи
		15: true, // Железнодорожный транспорт
		9:  true, // Другие наземные ТС
	}
	if specialEquipmentTypes[externalID] {
		return 15
	}

	// Если не найден маппинг, возвращаем 0 (ошибка)
	return 0
}

// mapUseTerritoryToApex маппит use_territory_region.external_id в useTerritoryId для Apex
func (oc *OsagoAllController) mapUseTerritoryToApex(externalID int) string {
	// Маппинг: 1 -> "1", 2 -> "2", 3 -> "3"
	if externalID >= 1 && externalID <= 3 {
		return strconv.Itoa(externalID)
	}
	return "1" // дефолт
}

// mapPeriodToApex маппит period_id в seasonalInsuranceId для Apex (возвращает число)
// Маппинг period_id в seasonalInsuranceId для Apex:
// 6 месяцев -> 1 (6 месяцев)
// 12 месяцев -> 7 (1 год)
// 20 дней -> 8 (20 дней)
func (oc *OsagoAllController) mapPeriodToApex(periodID int) int {
	if periodID == 6 {
		return 1 // 6 месяцев
	} else if periodID == 12 {
		return 7 // 1 год
	} else if periodID == 20 {
		return 8 // 20 дней
	}
	return 7 // дефолт (1 год)
}

// buildApexRequest формирует запрос для Apex API на основе данных из session
func (oc *OsagoAllController) buildApexRequest(sessionData map[string]interface{}, req OsagoAllCalcRequest,
	ownerPinfl, ownerPassportSeries, ownerPassportNumber, personPinfl, personPassportSeries, personPassportNumber string,
	useTerritoryRegionExternalID, vehicleTypeID int, vehicleGroupID string,
	gosNumber, techSery, techNumber string) map[string]interface{} {

	// Проверяем наличие обязательных данных
	if ownerPinfl == "" || ownerPassportSeries == "" || ownerPassportNumber == "" {
		log.Printf("[APEX BUILD] Error: owner passport data missing (pinfl=%s, seria=%s, number=%s)", ownerPinfl, ownerPassportSeries, ownerPassportNumber)
		return nil
	}
	if personPinfl == "" || personPassportSeries == "" || personPassportNumber == "" {
		log.Printf("[APEX BUILD] Error: person passport data missing (pinfl=%s, seria=%s, number=%s)", personPinfl, personPassportSeries, personPassportNumber)
		return nil
	}

	// Маппинг для Apex
	useTerritoryID := oc.mapUseTerritoryToApex(useTerritoryRegionExternalID)
	seasonalInsuranceID := oc.mapPeriodToApex(*req.PeriodID)
	apexVehicleTypeID := oc.mapVehicleTypeToApex(vehicleTypeID, vehicleGroupID)

	// Проверяем, что vehicleTypeId успешно замаппился
	if apexVehicleTypeID == 0 {
		log.Printf("[APEX BUILD] Error: unsupported vehicle type (vehicleTypeID=%d, vehicleGroupID=%s)", vehicleTypeID, vehicleGroupID)
		return nil // Неподдерживаемый тип ТС
	}

	// Маппинг number_drivers_id в driverNumberRestriction для Apex
	// 0 (unlimited) -> false, 5 (limited) -> true
	driverNumberRestriction := *req.NumberDriversID == 5

	// contractTermConclusionId всегда статический = "2"
	contractTermConclusionID := "2"

	return map[string]interface{}{
		"owner": map[string]interface{}{
			"person": map[string]interface{}{
				"passportData": map[string]string{
					"pinfl":  ownerPinfl,
					"seria":  ownerPassportSeries,
					"number": ownerPassportNumber,
				},
			},
		},
		"details": map[string]interface{}{
			"driverNumberRestriction": driverNumberRestriction,
		},
		"cost": map[string]interface{}{
			"contractTermConclusionId": contractTermConclusionID,
			"useTerritoryId":           useTerritoryID,
			"seasonalInsuranceId":      seasonalInsuranceID,
			"foreignVehicleId":         "2",
		},
		"vehicle": map[string]interface{}{
			"vehicleTypeId": apexVehicleTypeID,
			"techPassport": map[string]string{
				"number": techNumber,
				"seria":  techSery,
			},
			"govNumber": gosNumber,
		},
		"drivers": []map[string]interface{}{
			{
				"passportData": map[string]string{
					"pinfl":  personPinfl,
					"seria":  personPassportSeries,
					"number": personPassportNumber,
				},
			},
		},
	}
}

// checkOwner проверяет, является ли человек владельцем машины по паспортным данным
func (oc *OsagoAllController) checkOwner(vehicleData, personData interface{}) bool {
	// Преобразуем в map для доступа к данным
	vehicleMap, ok := vehicleData.(map[string]interface{})
	if !ok {
		return false
	}

	personMap, ok := personData.(map[string]interface{})
	if !ok {
		return false
	}

	// Извлекаем паспортные данные из person
	var personPassportNumber, personPassportSeries string

	if data, ok := personMap["data"].(map[string]interface{}); ok {
		if passport, ok := data["passport"].(map[string]interface{}); ok {
			if number, ok := passport["number"].(string); ok {
				personPassportNumber = number
			}
			if series, ok := passport["series"].(string); ok {
				personPassportSeries = series
			}
		}
	}

	// Извлекаем паспортные данные из vehicle.owner.person
	var vehicleOwnerPassportNumber, vehicleOwnerPassportSeries string

	if data, ok := vehicleMap["data"].(map[string]interface{}); ok {
		if owner, ok := data["owner"].(map[string]interface{}); ok {
			if ownerType, ok := owner["type"].(string); ok && ownerType == "person" {
				if person, ok := owner["person"].(map[string]interface{}); ok {
					if passport, ok := person["passport"].(map[string]interface{}); ok {
						if number, ok := passport["number"].(string); ok {
							vehicleOwnerPassportNumber = number
						}
						if series, ok := passport["series"].(string); ok {
							vehicleOwnerPassportSeries = series
						}
					}
				}
			}
		}
	}

	// Сравниваем паспортные данные
	if personPassportNumber != "" && personPassportSeries != "" &&
		vehicleOwnerPassportNumber != "" && vehicleOwnerPassportSeries != "" {
		return personPassportNumber == vehicleOwnerPassportNumber &&
			personPassportSeries == vehicleOwnerPassportSeries
	}

	return false
}

// mapVehicleTypeToGross маппит vehicle_type.external_id из find API в vehicleTypeId для Gross API
// Маппинг на основе данных:
// Find API external_id -> Gross vehicleTypeId:
//
//	2 (Легковые автомобили) -> 1 (Легковые автомобили)
//	6 (Грузовые автомобили) -> 2 (Грузовые автомобили)
//	1 (Автобусы) -> 3 (Автобус)
//	4 (Мотоциклы и мотороллеры) -> 4 (Мотоцикл)
//	Группа "66b3d262-2d6b-49a9-949e-6c87765fcbef" (Трамваи, мотоциклы, тракторы, спецтехника) -> 6 (Трактор и дорожно-строительный автомобиль)
func (oc *OsagoAllController) mapVehicleTypeToGross(externalID int, vehicleGroupID string) int {
	// Прямой маппинг по external_id
	switch externalID {
	case 2: // Легковые автомобили
		return 1
	case 6: // Грузовые автомобили
		return 2
	case 1: // Автобусы
		return 3
	case 4: // Мотоциклы и мотороллеры
		return 4
	}

	// Маппинг по vehicle_group для спецтехники и тракторов
	// Группа "66b3d262-2d6b-49a9-949e-6c87765fcbef" включает:
	// - Трамваи, мотоциклы и мотороллеры, тракторы, самоходные дорожно-строительные и иные машины
	if vehicleGroupID == "66b3d262-2d6b-49a9-949e-6c87765fcbef" {
		// Если это уже мотоцикл (external_id 4), он уже обработан выше
		// Для остальных из этой группы (тракторы, спецтехника) -> 6
		if externalID != 4 {
			return 6
		}
	}

	// Дополнительная проверка для типов из группы спецтехники
	specialEquipmentTypes := map[int]bool{
		8:  true, // Спецтехника
		13: true, // Трамваи
		15: true, // Железнодорожный транспорт
		9:  true, // Другие наземные ТС
	}
	if specialEquipmentTypes[externalID] {
		return 6
	}

	// Если не найден маппинг, возвращаем 0 (ошибка)
	return 0
}

// buildEuroasiaRequest формирует запрос для Euroasia API на основе данных из session
func (oc *OsagoAllController) buildEuroasiaRequest(sessionData map[string]interface{}, req OsagoAllCalcRequest, useTerritoryRegionID, vehicleGroupID string) map[string]interface{} {
	// Маппинг period_id в seasonal_insurance_id для Euroasia
	// 12 месяцев -> external_id 7 (1 год)
	// 6 месяцев -> external_id 1 (6 месяцев)
	// 20 дней -> external_id 8 (20 дней)
	var seasonalInsuranceID string
	if *req.PeriodID == 12 {
		seasonalInsuranceID = "8465a831-850f-4445-a995-ef71195094ab" // 1 год
	} else if *req.PeriodID == 6 {
		seasonalInsuranceID = "9848096e-cc12-4dbd-893b-41f2cdfc9a0e" // 6 месяцев
	} else if *req.PeriodID == 20 {
		seasonalInsuranceID = "0d546748-0ba6-43bc-9ce2-1b977ad9e494" // 20 дней
	} else {
		return nil // Неподдерживаемый период
	}

	// Маппинг number_drivers_id в driver_restriction
	// 0 -> false (unlimited), 5 -> true (limited)
	driverRestriction := *req.NumberDriversID == 5

	// Извлекаем данные person из session для drivers
	// Если person нет, используем owner.person как fallback
	var drivers []map[string]string
	var passportBirthdate, passportNumber, passportSeries string

	// Сначала пробуем получить из person
	personData, personOk := sessionData["person"].(map[string]interface{})
	if personOk {
		if data, ok := personData["data"].(map[string]interface{}); ok {
			// Извлекаем passport данные
			if passport, ok := data["passport"].(map[string]interface{}); ok {
				if number, ok := passport["number"].(string); ok {
					passportNumber = number
				}
				if series, ok := passport["series"].(string); ok {
					passportSeries = series
				}
			}

			// Извлекаем birthdate и преобразуем в формат YYYY-MM-DD
			if birthdate, ok := data["birthdate"].(string); ok {
				// Парсим дату и преобразуем в формат YYYY-MM-DD
				if t, err := time.Parse(time.RFC3339, birthdate); err == nil {
					passportBirthdate = t.Format("2006-01-02")
				} else if t, err := time.Parse("2006-01-02T15:04:05Z07:00", birthdate); err == nil {
					passportBirthdate = t.Format("2006-01-02")
				} else {
					// Пробуем другие форматы
					if t, err := time.Parse("2006-01-02", birthdate); err == nil {
						passportBirthdate = t.Format("2006-01-02")
					}
				}
			}
		}
	}

	// Если person данных нет, пробуем использовать owner.person из vehicle
	if passportNumber == "" || passportSeries == "" || passportBirthdate == "" {
		if vehicleData, ok := sessionData["vehicle"].(map[string]interface{}); ok {
			if data, ok := vehicleData["data"].(map[string]interface{}); ok {
				if owner, ok := data["owner"].(map[string]interface{}); ok {
					if ownerType, ok := owner["type"].(string); ok && ownerType == "person" {
						if person, ok := owner["person"].(map[string]interface{}); ok {
							// Извлекаем passport данные
							if passport, ok := person["passport"].(map[string]interface{}); ok {
								if passportNumber == "" {
									if number, ok := passport["number"].(string); ok {
										passportNumber = number
									}
								}
								if passportSeries == "" {
									if series, ok := passport["series"].(string); ok {
										passportSeries = series
									}
								}
							}
							// Извлекаем birthdate
							if passportBirthdate == "" {
								if birthdate, ok := person["birthdate"].(string); ok {
									// Парсим дату и преобразуем в формат YYYY-MM-DD
									if t, err := time.Parse(time.RFC3339, birthdate); err == nil {
										passportBirthdate = t.Format("2006-01-02")
									} else if t, err := time.Parse("2006-01-02T15:04:05Z07:00", birthdate); err == nil {
										passportBirthdate = t.Format("2006-01-02")
									} else if t, err := time.Parse("2006-01-02", birthdate); err == nil {
										passportBirthdate = t.Format("2006-01-02")
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if passportBirthdate != "" && passportNumber != "" && passportSeries != "" {
		drivers = []map[string]string{
			{
				"passport_birthdate": passportBirthdate,
				"passport_number":    passportNumber,
				"passport_series":    passportSeries,
			},
		}
	}

	if len(drivers) == 0 {
		log.Printf("[EUROASIA BUILD] Error: drivers array is empty")
		return nil
	}
	if useTerritoryRegionID == "" {
		log.Printf("[EUROASIA BUILD] Error: useTerritoryRegionID is empty")
		return nil
	}
	if vehicleGroupID == "" {
		log.Printf("[EUROASIA BUILD] Error: vehicleGroupID is empty")
		return nil
	}

	return map[string]interface{}{
		"driver_restriction":      driverRestriction,
		"drivers":                 drivers,
		"seasonal_insurance_id":   seasonalInsuranceID,
		"use_territory_region_id": useTerritoryRegionID,
		"vehicle_group_id":        vehicleGroupID,
	}
}

// mapVehicleTypeToTrust маппит vehicle_type.external_id из find API в vehicleTypeId для Trust API
func (oc *OsagoAllController) mapVehicleTypeToTrust(externalID int, vehicleGroupID string) int {
	// Прямой маппинг по external_id
	switch externalID {
	case 2: // Легковые автомобили
		return 1
	case 6: // Грузовые автомобили
		return 6
	case 1: // Автобусы
		return 9
	}

	// Маппинг по vehicle_group для спецтехники
	// Группа "66b3d262-2d6b-49a9-949e-6c87765fcbef" -> 15
	if vehicleGroupID == "66b3d262-2d6b-49a9-949e-6c87765fcbef" {
		return 15
	}

	// Дополнительная проверка для типов из группы спецтехники
	specialEquipmentTypes := map[int]bool{
		8:  true, // Спецтехника
		13: true, // Трамваи
		15: true, // Железнодорожный транспорт
		9:  true, // Другие наземные ТС
	}
	if specialEquipmentTypes[externalID] {
		return 15
	}

	// Если не найден маппинг, возвращаем 0 (ошибка)
	return 0
}

// mapUseTerritoryToTrust маппит use_territory_region.external_id в useTerritoryId для Trust
// Маппинг: 1 -> 1, 2 -> 2, 3 -> 3, и т.д. (прямой маппинг для территорий 1-14)
func (oc *OsagoAllController) mapUseTerritoryToTrust(externalID int) int {
	// Trust использует прямую нумерацию от 1 до 14
	// Пока используем прямой маппинг, если нужно будет корректировать - добавим маппинг
	if externalID >= 1 && externalID <= 14 {
		return externalID
	}
	return 1 // дефолт (ГОРОД ТАШКЕНТ)
}

// mapPeriodToTrust маппит period_id в period для Trust
// Маппинг Trust: 1 -> 6 месяцев, 2 -> 12 месяцев, 3 -> 2 месяца, 4 -> 15 и 20 дней
// period_id: 6 -> period 1 (6 месяцев), 12 -> period 2 (1 год), 20 -> period 4 (15/20 дней), 3 -> period 3 (2 месяца)
func (oc *OsagoAllController) mapPeriodToTrust(periodID int) int {
	if periodID == 6 {
		return 1 // 6 месяцев
	} else if periodID == 12 {
		return 2 // 12 месяцев (1 год)
	} else if periodID == 20 {
		return 4 // 15 или 20 дней
	} else if periodID == 3 {
		return 3 // 2 месяца
	}
	return 2 // дефолт (12 месяцев)
}

// buildTrustRequest формирует запрос для Trust API на основе данных из session
func (oc *OsagoAllController) buildTrustRequest(
	ownerPinfl, personPinfl string,
	useTerritoryRegionExternalID, vehicleTypeID int, vehicleGroupID string,
	gosNumber string,
	periodID, numberDriversID int) map[string]interface{} {

	// Проверяем наличие обязательных данных
	if ownerPinfl == "" && personPinfl == "" {
		return nil
	}
	// Fallbacks: if one PINFL is missing, reuse the other one.
	// This avoids returning nil when upstream returns numeric external_id or only one person is present.
	if ownerPinfl == "" {
		ownerPinfl = personPinfl
	}
	if personPinfl == "" {
		personPinfl = ownerPinfl
	}

	if gosNumber == "" {
		return nil
	}

	// Маппинг для Trust
	trustVehicleTypeID := oc.mapVehicleTypeToTrust(vehicleTypeID, vehicleGroupID)
	if trustVehicleTypeID == 0 {
		return nil // Неподдерживаемый тип ТС
	}

	trustUseTerritoryID := oc.mapUseTerritoryToTrust(useTerritoryRegionExternalID)
	trustPeriodID := oc.mapPeriodToTrust(periodID)

	// Маппинг number_drivers_id для Trust: 0 -> 0 (unlimited), 5 -> 1 (limited)
	// Trust API: 0 = неограниченное количество, 1 = ограниченное количество
	driverLimit := 0 // unlimited по умолчанию
	if numberDriversID == 5 {
		driverLimit = 1 // limited
	}

	return map[string]interface{}{
		"vehicle": map[string]interface{}{
			"vehicle":        trustVehicleTypeID,
			"renumber":       gosNumber,
			"foreignVehicle": false,
		},
		"period":        trustPeriodID,
		"use_territory": trustUseTerritoryID,
		"driver_limit":  driverLimit,
		"discount":      1,
		"owner": map[string]interface{}{
			"owner_pinfl": ownerPinfl,
		},
		"drivers": []map[string]interface{}{
			{
				"pinfl":       personPinfl,
				"coefficient": 1,
			},
		},
	}
}

// extractProviderAmounts извлекает суммы из ответов провайдеров
func (oc *OsagoAllController) extractProviderAmounts(neo, gross, euroasia, apex, trust interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Neo: response.amount_uzs
	if neoMap, ok := neo.(map[string]interface{}); ok {
		if response, ok := neoMap["response"].(map[string]interface{}); ok {
			if amount, ok := response["amount_uzs"].(float64); ok {
				result["neo"] = int(amount)
			} else if amount, ok := response["amount_uzs"].(int); ok {
				result["neo"] = amount
			} else {
				result["neo"] = "does not exist"
			}
		} else {
			result["neo"] = "does not exist"
		}
	} else {
		result["neo"] = "does not exist"
	}

	// Gross: response.amount_uzs
	if grossMap, ok := gross.(map[string]interface{}); ok {
		if response, ok := grossMap["response"].(map[string]interface{}); ok {
			if amount, ok := response["amount_uzs"].(float64); ok {
				result["gross"] = int(amount)
			} else if amount, ok := response["amount_uzs"].(int); ok {
				result["gross"] = amount
			} else {
				result["gross"] = "does not exist"
			}
		} else {
			result["gross"] = "does not exist"
		}
	} else {
		result["gross"] = "does not exist"
	}

	// Euroasia: data.premium.amount
	if euroasiaMap, ok := euroasia.(map[string]interface{}); ok {
		if data, ok := euroasiaMap["data"].(map[string]interface{}); ok {
			if premium, ok := data["premium"].(map[string]interface{}); ok {
				if amount, ok := premium["amount"].(float64); ok {
					result["euroasia"] = int(amount)
				} else if amount, ok := premium["amount"].(int); ok {
					result["euroasia"] = amount
				} else {
					result["euroasia"] = "does not exist"
				}
			} else {
				result["euroasia"] = "does not exist"
			}
		} else {
			result["euroasia"] = "does not exist"
		}
	} else {
		result["euroasia"] = "does not exist"
	}

	// Apex: insurance_premium (строка вида "192 000,00 UZS" -> 192000)
	if apexMap, ok := apex.(map[string]interface{}); ok {
		if premiumStr, ok := apexMap["insurance_premium"].(string); ok {
			// Парсим строку "192 000,00 UZS" -> 192000
			// Формат: пробелы разделяют тысячи, запятая - десятичные
			// Нужно взять только целую часть (до запятой), убрать пробелы
			cleaned := strings.ToUpper(strings.TrimSpace(premiumStr))
			// Убираем "UZS" если есть
			cleaned = strings.TrimSuffix(cleaned, "UZS")
			cleaned = strings.TrimSpace(cleaned)

			// Находим часть до запятой (целая часть)
			parts := strings.Split(cleaned, ",")
			integerPart := parts[0]

			// Убираем все пробелы из целой части
			integerPart = strings.ReplaceAll(integerPart, " ", "")

			// Преобразуем в число
			if amount, err := strconv.Atoi(integerPart); err == nil {
				result["apex"] = amount
			} else {
				result["apex"] = "does not exist"
			}
		} else {
			result["apex"] = "does not exist"
		}
	} else {
		result["apex"] = "does not exist"
	}

	// Trust: insurance_premium
	if trustMap, ok := trust.(map[string]interface{}); ok {
		// Try several common shapes:
		// - {"insurance_premium": 12345}
		// - {"data": {"insurance_premium": 12345}}
		// - {"tariffs": [{"insurance_premium": 12345}, ...]}
		var prem interface{} = nil
		if v, ok := trustMap["insurance_premium"]; ok {
			prem = v
		} else if data, ok := trustMap["data"].(map[string]interface{}); ok {
			prem = data["insurance_premium"]
		} else if tariffs, ok := trustMap["tariffs"].([]interface{}); ok && len(tariffs) > 0 {
			if t0, ok := tariffs[0].(map[string]interface{}); ok {
				prem = t0["insurance_premium"]
			}
		}

		switch v := prem.(type) {
		case float64:
			result["trust"] = int(v)
		case int:
			result["trust"] = v
		case string:
			// accept "192 000" or "192000"
			s := strings.ReplaceAll(strings.TrimSpace(v), " ", "")
			if n, err := strconv.Atoi(s); err == nil {
				result["trust"] = n
			} else {
				result["trust"] = "does not exist"
			}
		default:
			result["trust"] = "does not exist"
		}
	} else {
		result["trust"] = "does not exist"
	}

	return result
}

// DriverInfo - структура для данных водителя
type DriverInfo struct {
	PassportSeria    string `json:"passport__seria"`
	PassportNumber   string `json:"passport__number"`
	DriverBirthday   string `json:"driver_birthday"`
	LicenseNumber    string `json:"licenseNumber,omitempty"`
	LicenseSeria     string `json:"licenseSeria,omitempty"`
	LicenseIssueDate string `json:"licenseIssueDate,omitempty"`
	Relative         int    `json:"relative"`
	Name             string `json:"name,omitempty"`
}

// OsagoAllCreatePolicyRequest - структура запроса для создания полиса
type OsagoAllCreatePolicyRequest struct {
	SessionID   string       `json:"session_id" binding:"required"`
	Provider    string       `json:"provider" binding:"required"` // "neo", "gross", "apex", "trust", "euroasia"
	Drivers     []DriverInfo `json:"drivers,omitempty"`           // только если number_drivers_id == 5
	PhoneNumber string       `json:"phone_number" binding:"required"`
}

// CreatePolicy - метод для создания полиса на основе данных из session и calc
func (oc *OsagoAllController) CreatePolicy(c *gin.Context) {
	var req OsagoAllCreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Проверяем provider
	validProviders := map[string]bool{
		"neo":      true,
		"gross":    true,
		"apex":     true,
		"trust":    true,
		"euroasia": true,
	}
	if !validProviders[req.Provider] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider. Supported: neo, gross, apex, trust, euroasia"})
		return
	}

	// Пока реализуем только для neo
	if req.Provider != "neo" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "provider not implemented yet", "provider": req.Provider})
		return
	}

	// Получаем данные из Redis
	rdb := utils.GetRedis()
	if rdb == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redis not available"})
		return
	}

	ctx := context.Background()
	sessionKey := "osago_all:session:" + req.SessionID
	calcResultKey := "osago_all:calc:" + req.SessionID

	// Получаем данные сессии
	sessionDataStr, err := rdb.Get(ctx, sessionKey).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found or expired"})
		return
	}

	// Получаем результат расчета
	calcResultStr, err := rdb.Get(ctx, calcResultKey).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "calc result not found or expired. Please call calc first"})
		return
	}

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(sessionDataStr), &sessionData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse session data"})
		return
	}

	var calcResultData map[string]interface{}
	if err := json.Unmarshal([]byte(calcResultStr), &calcResultData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse calc result data"})
		return
	}

	// Извлекаем данные vehicle из session
	vehicleData, ok := sessionData["vehicle"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vehicle data not found in session"})
		return
	}

	// Извлекаем нужные поля из vehicle
	var gosNumber, techSery, techNumber string

	if data, ok := vehicleData["data"].(map[string]interface{}); ok {
		// license_plate -> gos_number
		if lp, ok := data["license_plate"].(string); ok {
			gosNumber = lp
		}

		// tech_passport
		if tp, ok := data["tech_passport"].(map[string]interface{}); ok {
			if series, ok := tp["series"].(string); ok {
				techSery = series
			}
			if number, ok := tp["number"].(string); ok {
				techNumber = number
			}
		}
	}

	if gosNumber == "" || techSery == "" || techNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "required vehicle data missing in session"})
		return
	}

	// Извлекаем данные owner из vehicle для owner и applicant полей
	var ownerPassSeria, ownerPassNumber, ownerBirthday string
	if data, ok := vehicleData["data"].(map[string]interface{}); ok {
		if owner, ok := data["owner"].(map[string]interface{}); ok {
			if ownerType, ok := owner["type"].(string); ok && ownerType == "person" {
				if person, ok := owner["person"].(map[string]interface{}); ok {
					// passport данные
					if passport, ok := person["passport"].(map[string]interface{}); ok {
						if series, ok := passport["series"].(string); ok {
							ownerPassSeria = series
						}
						if number, ok := passport["number"].(string); ok {
							ownerPassNumber = number
						}
					}
					// birthdate - преобразуем в формат DD.MM.YYYY
					if birthdate, ok := person["birthdate"].(string); ok {
						// Парсим дату и преобразуем в формат DD.MM.YYYY
						if t, err := time.Parse(time.RFC3339, birthdate); err == nil {
							ownerBirthday = t.Format("02.01.2006")
						} else if t, err := time.Parse("2006-01-02T15:04:05Z07:00", birthdate); err == nil {
							ownerBirthday = t.Format("02.01.2006")
						} else if t, err := time.Parse("2006-01-02", birthdate); err == nil {
							ownerBirthday = t.Format("02.01.2006")
						}
					}
				}
			}
		}
	}

	if ownerPassSeria == "" || ownerPassNumber == "" || ownerBirthday == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "owner passport data not found in session"})
		return
	}

	// Извлекаем amount_uzs из calc result
	var amountUZS int
	neoData, ok := calcResultData["neo"].(map[string]interface{})
	if ok {
		if response, ok := neoData["response"].(map[string]interface{}); ok {
			if amount, ok := response["amount_uzs"].(float64); ok {
				amountUZS = int(amount)
			} else if amount, ok := response["amount_uzs"].(int); ok {
				amountUZS = amount
			}
		}
	}

	if amountUZS == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_uzs not found in calc result"})
		return
	}

	// Извлекаем period_id и number_drivers_id из calc result
	var periodID int = 1            // default для Neo: 1 = 12 месяцев
	var numberDriversID int = 1     // default для Neo: 1 = unlimited
	var calcNumberDriversID int = 0 // из calc result
	if pid, ok := calcResultData["period_id"].(float64); ok {
		// Маппинг для Neo: 12 -> 1, 6 -> 2
		if int(pid) == 12 {
			periodID = 1
		} else if int(pid) == 6 {
			periodID = 2
		}
	}
	if ndid, ok := calcResultData["number_drivers_id"].(float64); ok {
		calcNumberDriversID = int(ndid)
		// Маппинг для Neo API: 0 (unlimited) -> 1, 5 (limited) -> 4
		if calcNumberDriversID == 0 {
			numberDriversID = 1 // unlimited
		} else if calcNumberDriversID == 5 {
			numberDriversID = 4 // limited
		}
	}

	// Обрабатываем drivers: если number_drivers_id == 5 (из calc), принимаем из запроса
	var drivers []map[string]interface{}
	if calcNumberDriversID == 5 {
		// Если limited (5), принимаем drivers из запроса
		if len(req.Drivers) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "drivers are required when number_drivers_id is 5 (limited)"})
			return
		}
		// Преобразуем drivers из запроса
		for _, driver := range req.Drivers {
			driverMap := map[string]interface{}{
				"passport__seria":  strings.ToLower(driver.PassportSeria),
				"passport__number": driver.PassportNumber,
				"driver_birthday":  driver.DriverBirthday,
				"relative":         driver.Relative,
			}
			// Добавляем опциональные поля лицензии
			if driver.LicenseNumber != "" {
				driverMap["licenseNumber"] = driver.LicenseNumber
			}
			if driver.LicenseSeria != "" {
				driverMap["licenseSeria"] = driver.LicenseSeria
			}
			if driver.LicenseIssueDate != "" {
				driverMap["licenseIssueDate"] = driver.LicenseIssueDate
			}
			if driver.Name != "" {
				driverMap["name"] = driver.Name
			}
			drivers = append(drivers, driverMap)
		}
	}

	// Формируем запрос для Neo save-policy/v2 согласно документации
	savePolicyRequest := map[string]interface{}{
		"gos_number":             gosNumber,
		"tech_sery":              techSery,
		"tech_number":            techNumber,
		"period_id":              periodID,
		"number_drivers_id":      numberDriversID, // Для Neo это int, не string
		"owner__pass_seria":      ownerPassSeria,
		"owner__pass_number":     ownerPassNumber,
		"owner_birthday":         ownerBirthday,
		"applicant__pass_seria":  ownerPassSeria, // Всегда те же данные что owner
		"applicant__pass_number": ownerPassNumber,
		"applicant__birthday":    ownerBirthday,
		"applicant_is_driver":    false, // Всегда false
		"phone_number":           req.PhoneNumber,
		"amount_uzs":             amountUZS,
	}

	// Добавляем drivers только если number_drivers_id == 5 (из calc)
	if calcNumberDriversID == 5 && len(drivers) > 0 {
		savePolicyRequest["drivers"] = drivers
	}

	// Отправляем запрос на save-policy/v2
	savePolicyURL := oc.cfg.NeoBaseURL + "/api/osago-neo/save-policy/v2"
	savePolicyJsonData, err := json.Marshal(savePolicyRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal save-policy request", "details": err.Error()})
		return
	}

	log.Printf("[NEO SAVE POLICY] URL: %s", savePolicyURL)
	log.Printf("[NEO SAVE POLICY] Request: %s", string(savePolicyJsonData))

	savePolicyHttpReq, err := http.NewRequest(http.MethodPost, savePolicyURL, bytes.NewBuffer(savePolicyJsonData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create save-policy request", "details": err.Error()})
		return
	}

	savePolicyHttpReq.Header.Set("Content-Type", "application/json")
	creds := oc.cfg.NeoLogin + ":" + oc.cfg.NeoPassword
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
	savePolicyHttpReq.Header.Set("Authorization", auth)

	savePolicyResp, err := oc.cl.Do(savePolicyHttpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send save-policy request", "details": err.Error()})
		return
	}
	defer savePolicyResp.Body.Close()

	savePolicyResponseBody, err := io.ReadAll(savePolicyResp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read save-policy response", "details": err.Error()})
		return
	}

	log.Printf("[NEO SAVE POLICY] Status: %d", savePolicyResp.StatusCode)
	log.Printf("[NEO SAVE POLICY] Response: %s", string(savePolicyResponseBody))

	var savePolicyResponseData interface{}
	if err := json.Unmarshal(savePolicyResponseBody, &savePolicyResponseData); err != nil {
		// Если не удалось распарсить JSON, возвращаем как строку
		savePolicyResponseData = string(savePolicyResponseBody)
	}

	// Возвращаем сырой ответ от save-policy/v2
	c.JSON(savePolicyResp.StatusCode, savePolicyResponseData)
}
