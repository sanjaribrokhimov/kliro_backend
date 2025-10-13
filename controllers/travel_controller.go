package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type ApexCountry struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	NameRuEng string  `json:"name_ru_eng"`
	Kod       *string `json:"kod"`
}

var apexCountries []ApexCountry

var countryCodeMap = map[string]string{
	"AF": "AFG", "AL": "ALB", "DZ": "DZA", "AS": "ASM", "AD": "AND",
	"AO": "AGO", "AG": "ATG", "AR": "ARG", "AM": "ARM", "AW": "ABW",
	"AU": "AUS", "AT": "AUT", "AZ": "AZE", "BS": "BHS", "BH": "BHR",
	"BD": "BGD", "BB": "BRB", "BY": "BLR", "BE": "BEL", "BZ": "BLZ",
	"BJ": "BEN", "BM": "BMU", "BT": "BTN", "BO": "BOL", "BA": "BIH",
	"BW": "BWA", "BR": "BRA", "BN": "BRN", "BG": "BGR", "BF": "BFA",
	"BI": "BDI", "KH": "KHM", "CM": "CMR", "CA": "CAN", "CV": "CPV",
	"KY": "CYM", "CF": "CAF", "TD": "TCD", "CL": "CHL", "CN": "CHN",
	"CO": "COL", "KM": "COM", "CG": "COG", "CD": "COD", "CR": "CRI",
	"CI": "CIV", "HR": "HRV", "CU": "CUB", "CY": "CYP", "CZ": "CZE",
	"DK": "DNK", "DJ": "DJI", "DM": "DMA", "DO": "DOM", "EC": "ECU",
	"EG": "EGY", "SV": "SLV", "GQ": "GNQ", "ER": "ERI", "EE": "EST",
	"ET": "ETH", "FJ": "FJI", "FI": "FIN", "FR": "FRA", "GA": "GAB",
	"GM": "GMB", "GE": "GEO", "DE": "DEU", "GH": "GHA", "GR": "GRC",
	"GD": "GRD", "GT": "GTM", "GN": "GIN", "GW": "GNB", "GY": "GUY",
	"HT": "HTI", "HN": "HND", "HK": "HKG", "HU": "HUN", "IS": "ISL",
	"IN": "IND", "ID": "IDN", "IR": "IRN", "IQ": "IRQ", "IE": "IRL",
	"IL": "ISR", "IT": "ITA", "JM": "JAM", "JP": "JPN", "JO": "JOR",
	"KZ": "KAZ", "KE": "KEN", "KI": "KIR", "KP": "PRK", "KR": "KOR",
	"KW": "KWT", "KG": "KGZ", "LA": "LAO", "LV": "LVA", "LB": "LBN",
	"LS": "LSO", "LR": "LBR", "LY": "LBY", "LI": "LIE", "LT": "LTU",
	"LU": "LUX", "MO": "MAC", "MK": "MKD", "MG": "MDG", "MW": "MWI",
	"MY": "MYS", "MV": "MDV", "ML": "MLI", "MT": "MLT", "MH": "MHL",
	"MR": "MRT", "MU": "MUS", "MX": "MEX", "FM": "FSM", "MD": "MDA",
	"MC": "MCO", "MN": "MNG", "ME": "MNE", "MA": "MAR", "MZ": "MOZ",
	"MM": "MMR", "NA": "NAM", "NR": "NRU", "NP": "NPL", "NL": "NLD",
	"NZ": "NZL", "NI": "NIC", "NE": "NER", "NG": "NGA", "NO": "NOR",
	"OM": "OMN", "PK": "PAK", "PW": "PLW", "PS": "PSE", "PA": "PAN",
	"PG": "PNG", "PY": "PRY", "PE": "PER", "PH": "PHL", "PL": "POL",
	"PT": "PRT", "QA": "QAT", "RO": "ROU", "RU": "RUS", "RW": "RWA",
	"KN": "KNA", "LC": "LCA", "VC": "VCT", "WS": "WSM", "SM": "SMR",
	"ST": "STP", "SA": "SAU", "SN": "SEN", "RS": "SRB", "SC": "SYC",
	"SL": "SLE", "SG": "SGP", "SK": "SVK", "SI": "SVN", "SB": "SLB",
	"SO": "SOM", "ZA": "ZAF", "SS": "SSD", "ES": "ESP", "LK": "LKA",
	"SD": "SDN", "SR": "SUR", "SZ": "SWZ", "SE": "SWE", "CH": "CHE",
	"SY": "SYR", "TW": "TWN", "TJ": "TJK", "TZ": "TZA", "TH": "THA",
	"TL": "TLS", "TG": "TGO", "TO": "TON", "TT": "TTO", "TN": "TUN",
	"TR": "TUR", "TM": "TKM", "TV": "TUV", "UG": "UGA", "UA": "UKR",
	"AE": "ARE", "GB": "GBR", "US": "USA", "UY": "URY", "UZ": "UZB",
	"VU": "VUT", "VE": "VEN", "VN": "VNM", "YE": "YEM", "ZM": "ZMB",
	"ZW": "ZWE",
}

var purposeMapping = map[string]map[int]int{
	"neo": {
		0: 1, // Путешествие (Travel)
		1: 5, // Работа (Work)
		2: 3, // Спорт (Sport)
		3: 4, // Учеба (Education)
	},
	"gross": {
		0: 1, // Путешествие (Travel)
		1: 3, // Работа (Work)
		2: 4, // Спорт (Sport)
		3: 2, // Учеба (Education)
	},
	"trust": {
		0: 0, // Путешествие (Туризм)
		1: 1, // Работа (Работа физическая)
		2: 4, // Спорт (Спорт 1)
		6: 6, // Спорт экстремальный (Спорт 3)
	},
	"apex": {
		0: 0, // Путешествие
		1: 1, // Работа
		2: 2, // Спорт
		3: 3, // Учеба
		4: 4, // Командировка
		5: 5, // Водитель
	},
}

func getProviderPurposeID(provider string, ourPurposeID int) (int, bool) {
	if providerMapping, exists := purposeMapping[provider]; exists {
		if mappedID, exists := providerMapping[ourPurposeID]; exists {
			return mappedID, true
		}
	}
	return 0, false
}

func hasProviderPurpose(provider string, ourPurposeID int) bool {
	_, exists := getProviderPurposeID(provider, ourPurposeID)
	return exists
}

type TravelController struct {
	RDB *redis.Client
}

func NewTravelController(rdb *redis.Client) *TravelController {
	loadApexCountries()
	return &TravelController{RDB: rdb}
}

func loadApexCountries() {
	if len(apexCountries) > 0 {
		return
	}

	data, err := os.ReadFile("staticDate/trustCountry.json")
	if err != nil {
		fmt.Println("Warning: Could not load staticDate/trustCountry.json:", err)
		return
	}

	if err := json.Unmarshal(data, &apexCountries); err != nil {
		fmt.Println("Warning: Could not parse staticDate/trustCountry.json:", err)
		return
	}

	fmt.Printf("Loaded %d countries from trustCountry.json\n", len(apexCountries))
}

func getCountryIDByCode(code string) int {
	fmt.Printf("Searching for country code: %s\n", code)
	for _, country := range apexCountries {
		if country.Kod != nil && *country.Kod == code {
			fmt.Printf("Found country: %s -> ID: %d\n", code, country.ID)
			return country.ID
		}
	}
	fmt.Printf("Country code %s not found\n", code)
	return 0
}

type TravelPurposeRequest struct {
	PurposeID    int      `json:"purpose_id" binding:"required,max=6"`
	Destinations []string `json:"destinations" binding:"required,min=1,max=5"`
}

func (tc *TravelController) SetTravelPurpose(c *gin.Context) {
	fmt.Println("\n========================================")
	fmt.Println("API 1: SET TRAVEL PURPOSE")
	fmt.Println("========================================")

	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	fmt.Println("REQUEST BODY:")
	fmt.Println(string(bodyBytes))

	var req TravelPurposeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("VALIDATION ERROR: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	fmt.Printf("PARSED: PurposeID=%d, Destinations=%v\n", req.PurposeID, req.Destinations)

	if len(req.Destinations) > 5 {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "maximum 5 countries allowed"})
		return
	}

	sessionID := uuid.New().String()
	ctx := context.Background()
	redisKey := "travel:session:" + sessionID

	sessionData := map[string]interface{}{
		"purpose_id":   req.PurposeID,
		"destinations": req.Destinations,
	}

	sessionDataJSON, err := json.Marshal(sessionData)
	if err != nil {
		fmt.Printf("ERROR: Failed to marshal session: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create session"})
		return
	}

	fmt.Printf("\nSAVING TO REDIS: key=%s\n", redisKey)
	fmt.Printf("SESSION DATA: %s\n", string(sessionDataJSON))

	err = tc.RDB.Set(ctx, redisKey, sessionDataJSON, 30*time.Minute).Err()
	if err != nil {
		fmt.Printf("ERROR: Failed to save to Redis: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to store session"})
		return
	}

	response := gin.H{
		"result": gin.H{
			"session_id":   sessionID,
			"purpose_id":   req.PurposeID,
			"destinations": req.Destinations,
		},
		"success": true,
	}

	fmt.Println("\nRESPONSE:")
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(responseJSON))
	fmt.Println("========================================\n")

	c.JSON(200, response)
}

type TravelDetailsRequest struct {
	SessionID           string   `json:"session_id" binding:"required"`
	StartDate           string   `json:"start_date" binding:"required"`
	EndDate             string   `json:"end_date" binding:"required"`
	TravelersBirthdates []string `json:"travelers_birthdates" binding:"required"`
	AnnualPolicy        bool     `json:"annual_policy"`
	CovidProtection     bool     `json:"covid_protection"`
	FamilyTravel        bool     `json:"family_travel"`
}

func (tc *TravelController) SetTravelDetails(c *gin.Context) {
	fmt.Println("\n========================================")
	fmt.Println("API 2: SET TRAVEL DETAILS")
	fmt.Println("========================================")

	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	fmt.Println("REQUEST BODY:")
	fmt.Println(string(bodyBytes))

	var req TravelDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("VALIDATION ERROR: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	fmt.Printf("PARSED: SessionID=%s, StartDate=%s, EndDate=%s\n", req.SessionID, req.StartDate, req.EndDate)
	fmt.Printf("        TravelersBirthdates=%v\n", req.TravelersBirthdates)
	fmt.Printf("        AnnualPolicy=%v, CovidProtection=%v, FamilyTravel=%v\n",
		req.AnnualPolicy, req.CovidProtection, req.FamilyTravel)

	ctx := context.Background()
	redisKey := "travel:session:" + req.SessionID

	fmt.Printf("LOADING FROM REDIS: key=%s\n", redisKey)

	existingData, err := tc.RDB.Get(ctx, redisKey).Result()
	if err != nil {
		fmt.Printf("ERROR: Session not found: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "session not found or expired"})
		return
	}

	fmt.Printf("SESSION DATA FROM REDIS: %s\n", existingData)

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(existingData), &sessionData); err != nil {
		fmt.Printf("ERROR: Failed to parse session: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse session data"})
		return
	}

	fmt.Println("PARSED SESSION DATA:")
	for key, value := range sessionData {
		fmt.Printf("  %s: %v (type: %T)\n", key, value, value)
	}

	sessionData["start_date"] = req.StartDate
	sessionData["end_date"] = req.EndDate
	sessionData["travelers_birthdates"] = req.TravelersBirthdates
	sessionData["annual_policy"] = req.AnnualPolicy
	sessionData["covid_protection"] = req.CovidProtection
	sessionData["family_travel"] = req.FamilyTravel

	updatedDataJSON, err := json.Marshal(sessionData)
	if err != nil {
		fmt.Printf("ERROR: Failed to marshal updated session: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to update session"})
		return
	}

	fmt.Printf("\nUPDATED SESSION DATA: %s\n", string(updatedDataJSON))

	err = tc.RDB.Set(ctx, redisKey, updatedDataJSON, 30*time.Minute).Err()
	if err != nil {
		fmt.Printf("ERROR: Failed to save session: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to save session"})
		return
	}

	response := gin.H{
		"result": gin.H{
			"session_id":           req.SessionID,
			"purpose_id":           sessionData["purpose_id"],
			"destinations":         sessionData["destinations"],
			"start_date":           req.StartDate,
			"end_date":             req.EndDate,
			"travelers_birthdates": req.TravelersBirthdates,
			"annual_policy":        req.AnnualPolicy,
			"covid_protection":     req.CovidProtection,
			"family_travel":        req.FamilyTravel,
		},
		"success": true,
	}

	fmt.Println("\nRESPONSE:")
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(responseJSON))
	fmt.Println("========================================\n")

	c.JSON(200, response)
}

type TravelCalculateRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	Accident     bool   `json:"accident"`
	Luggage      bool   `json:"luggage"`
	CancelTravel bool   `json:"cancel_travel"`
	PersonRespon bool   `json:"person_respon"`
	DelayTravel  bool   `json:"delay_travel"`
}

type NeoInsuranceCalculatorRequest struct {
	BeginDate  string   `json:"begin_date"`
	EndDate    string   `json:"end_date,omitempty"`
	Days       int      `json:"days,omitempty"`
	Countries  []string `json:"countries"`
	PurposeID  int      `json:"purpose_id"`
	KopMartali bool     `json:"kop_martali"`
	IsFamily   bool     `json:"is_family"`
	HasCovid   bool     `json:"has_covid"`
	Travelers  []string `json:"travelers"`
	Risklar    struct {
		Accident     int `json:"accident"`
		Luggage      int `json:"luggage"`
		CancelTravel int `json:"cancel_travel"`
		PersonRespon int `json:"person_respon"`
		DelayTravel  int `json:"delay_travel"`
	} `json:"risklar"`
}

type GrossCalculatorRequest struct {
	Countries []string `json:"countries"`
	BeginDate string   `json:"begin_date"`
	EndDate   string   `json:"end_date"`
	HasCovid  bool     `json:"has_covid"`
	Birthdays []string `json:"birthdays"`
}

type TrustCalculatorRequest struct {
	Day        int      `json:"day"`
	ActivityID int      `json:"activity_id"`
	Countries  []int    `json:"countries"`
	GroupID    int      `json:"group_id"`
	TypeID     int      `json:"type_id"`
	MultiID    int      `json:"multi_id"`
	DateReg    string   `json:"date_reg"`
	DateBirths []string `json:"date_births"`
}

type ApexCountryInfo struct {
	ISO string `json:"iso"`
}

type ApexTravelInfo struct {
	StartDate string            `json:"start_date"`
	EndDate   string            `json:"end_date"`
	Country   []ApexCountryInfo `json:"country"`
	ProgramID int               `json:"program_id"`
	PurposeID int               `json:"purpose_id"`
	GroupID   int               `json:"group_id"`
}

type ApexPersonInfo struct {
	Birthday string `json:"birthday"`
}

type ApexCalculatorRequest struct {
	TravelInfo ApexTravelInfo   `json:"travel_info"`
	PersonInfo []ApexPersonInfo `json:"person_info"`
}

func (tc *TravelController) CalculateTravel(c *gin.Context) {
	fmt.Println("\n========================================")
	fmt.Println("API 3: CALCULATE TRAVEL")
	fmt.Println("========================================")

	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	fmt.Println("REQUEST BODY:")
	fmt.Println(string(bodyBytes))

	var req TravelCalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("VALIDATION ERROR: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	fmt.Printf("PARSED: SessionID=%s\n", req.SessionID)
	fmt.Printf("        Risks: accident=%v, luggage=%v, cancel=%v, person_respon=%v, delay=%v\n",
		req.Accident, req.Luggage, req.CancelTravel, req.PersonRespon, req.DelayTravel)

	ctx := context.Background()
	redisKey := "travel:session:" + req.SessionID

	fmt.Printf("\nLOADING FROM REDIS: key=%s\n", redisKey)

	sessionDataStr, err := tc.RDB.Get(ctx, redisKey).Result()
	if err != nil {
		fmt.Printf("ERROR: Session not found: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "session not found or expired"})
		return
	}

	fmt.Printf("SESSION DATA FROM REDIS: %s\n", sessionDataStr)

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(sessionDataStr), &sessionData); err != nil {
		fmt.Printf("ERROR: Failed to parse session: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse session data"})
		return
	}

	fmt.Println("\nPARSED SESSION DATA:")
	for key, value := range sessionData {
		fmt.Printf("  %s: %v (type: %T)\n", key, value, value)
	}

	destinationsInterface := sessionData["destinations"].([]interface{})
	countries := make([]string, len(destinationsInterface))
	for i, v := range destinationsInterface {
		countries[i] = v.(string)
	}

	travelersInterface := sessionData["travelers_birthdates"].([]interface{})
	travelers := make([]string, len(travelersInterface))
	for i, v := range travelersInterface {
		travelers[i] = v.(string)
	}

	purposeID := int(sessionData["purpose_id"].(float64))
	annualPolicy := sessionData["annual_policy"].(bool)
	hasCovid := sessionData["covid_protection"].(bool)
	startDate := sessionData["start_date"].(string)
	endDate := sessionData["end_date"].(string)

	sessionData["accident"] = req.Accident
	sessionData["luggage"] = req.Luggage
	sessionData["cancel_travel"] = req.CancelTravel
	sessionData["person_respon"] = req.PersonRespon
	sessionData["delay_travel"] = req.DelayTravel

	updatedSessionJSON, _ := json.Marshal(sessionData)
	tc.RDB.Set(ctx, redisKey, updatedSessionJSON, 30*time.Minute)

	fmt.Printf("Updated session with risks: accident=%v, luggage=%v, cancel=%v, person_respon=%v, delay=%v\n",
		req.Accident, req.Luggage, req.CancelTravel, req.PersonRespon, req.DelayTravel)

	type ProviderResult struct {
		Provider string
		Data     map[string]interface{}
		Error    error
	}

	resultChan := make(chan ProviderResult, 4)
	activeProviders := 0

	if hasProviderPurpose("neo", purposeID) {
		activeProviders++
		go func() {
			neoPurposeID, _ := getProviderPurposeID("neo", purposeID)
			neoReq := NeoInsuranceCalculatorRequest{
				PurposeID:  neoPurposeID,
				Countries:  countries,
				KopMartali: annualPolicy,
				IsFamily:   false,
				HasCovid:   hasCovid,
				Travelers:  travelers,
			}

			if purposeID == 1 || purposeID == 2 {
				neoReq.BeginDate = startDate
				neoReq.EndDate = endDate
			} else if purposeID == 3 || purposeID == 4 || annualPolicy {
				neoReq.BeginDate = startDate
				neoReq.Days = 30
			}

			neoReq.Risklar.Accident = boolToInt(req.Accident)
			neoReq.Risklar.Luggage = boolToInt(req.Luggage)
			neoReq.Risklar.CancelTravel = boolToInt(req.CancelTravel)
			neoReq.Risklar.PersonRespon = boolToInt(req.PersonRespon)
			neoReq.Risklar.DelayTravel = boolToInt(req.DelayTravel)

			baseURL := os.Getenv("NEO_BASE_URL")
			if baseURL == "" {
				baseURL = "https://api.neoinsurance.uz"
			}

			jsonData, err := json.Marshal(neoReq)
			if err != nil {
				resultChan <- ProviderResult{Provider: "neo", Error: err}
				return
			}

			neoLogin := os.Getenv("NEO_LOGIN")
			neoPassword := os.Getenv("NEO_PASSWORD")
			creds := neoLogin + ":" + neoPassword
			authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

			httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/travel-neo/calculator-total", baseURL), bytes.NewBuffer(jsonData))
			if err != nil {
				resultChan <- ProviderResult{Provider: "neo", Error: err}
				return
			}

			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", authHeader)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				resultChan <- ProviderResult{Provider: "neo", Error: err}
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				resultChan <- ProviderResult{Provider: "neo", Error: err}
				return
			}

			var neoResponse map[string]interface{}
			if err := json.Unmarshal(body, &neoResponse); err != nil {
				resultChan <- ProviderResult{Provider: "neo", Error: err}
				return
			}

			resultChan <- ProviderResult{Provider: "neo", Data: neoResponse}
		}()
	}

	if hasProviderPurpose("gross", purposeID) {
		activeProviders++
		go func() {
			grossReq := GrossCalculatorRequest{
				Countries: countries,
				BeginDate: convertDateFormat(startDate),
				EndDate:   convertDateFormat(endDate),
				HasCovid:  hasCovid,
			}

			grossBirthdays := make([]string, len(travelers))
			for i, bd := range travelers {
				grossBirthdays[i] = convertDateFormat(bd)
			}
			grossReq.Birthdays = grossBirthdays

			baseURL := os.Getenv("GROSS_BASE_URL")
			if baseURL == "" {
				baseURL = "https://gross.uz/ru"
			}

			jsonData, err := json.Marshal(grossReq)
			if err != nil {
				resultChan <- ProviderResult{Provider: "gross", Error: err}
				return
			}

			grossLogin := os.Getenv("GROSS_LOGIN")
			grossPassword := os.Getenv("GROSS_PASSWORD")
			creds := grossLogin + ":" + grossPassword
			authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

			httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/travelapi/calc-amount", baseURL), bytes.NewBuffer(jsonData))
			if err != nil {
				resultChan <- ProviderResult{Provider: "gross", Error: err}
				return
			}

			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", authHeader)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				resultChan <- ProviderResult{Provider: "gross", Error: err}
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				resultChan <- ProviderResult{Provider: "gross", Error: err}
				return
			}

			var grossResponse map[string]interface{}
			if err := json.Unmarshal(body, &grossResponse); err != nil {
				resultChan <- ProviderResult{Provider: "gross", Error: err}
				return
			}

			resultChan <- ProviderResult{Provider: "gross", Data: grossResponse}
		}()
	}

	if hasProviderPurpose("trust", purposeID) {
		activeProviders++
		go func() {
			trustCountryIDs := make([]int, 0)
			for _, countryCode := range countries {
				alpha3Code := countryCode
				if mapped, ok := countryCodeMap[countryCode]; ok {
					alpha3Code = mapped
				}
				if id := getCountryIDByCode(alpha3Code); id > 0 {
					trustCountryIDs = append(trustCountryIDs, id)
				}
			}

			days := calculateDays(startDate, endDate)

			location, _ := time.LoadLocation("Asia/Tashkent")
			currentDate := time.Now().In(location).Format("02.01.2006")

			trustPurposeID, _ := getProviderPurposeID("trust", purposeID)
			trustReq := TrustCalculatorRequest{
				Day:        days,
				ActivityID: trustPurposeID,
				Countries:  trustCountryIDs,
				GroupID:    0,
				TypeID:     boolToInt(annualPolicy),
				MultiID:    0,
				DateReg:    currentDate,
			}

			trustBirthdays := make([]string, len(travelers))
			for i, bd := range travelers {
				trustBirthdays[i] = convertDateFormat(bd)
			}
			trustReq.DateBirths = trustBirthdays

			baseURL := os.Getenv("TRUST_BASE_URL")
			if baseURL == "" {
				baseURL = "https://api.online-trust.uz"
			}

			jsonData, err := json.Marshal(trustReq)
			if err != nil {
				resultChan <- ProviderResult{Provider: "trust", Error: err}
				return
			}

			trustLogin := os.Getenv("TRUST_LOGIN")
			trustPassword := os.Getenv("TRUST_PASSWORD")

			fmt.Println("=== TRUST REQUEST ===")
			fmt.Println("URL:", fmt.Sprintf("%s/api/travel/price/total-with-country", baseURL))
			fmt.Println("Login:", trustLogin)
			fmt.Println("Password:", trustPassword)
			fmt.Println("Body:", string(jsonData))

			creds := trustLogin + ":" + trustPassword
			authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
			fmt.Println("Auth Header:", authHeader)

			httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/travel/price/total-with-country", baseURL), bytes.NewBuffer(jsonData))
			if err != nil {
				resultChan <- ProviderResult{Provider: "trust", Error: err}
				return
			}

			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", authHeader)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				resultChan <- ProviderResult{Provider: "trust", Error: err}
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				resultChan <- ProviderResult{Provider: "trust", Error: err}
				return
			}

			fmt.Println("=== TRUST RESPONSE ===")
			fmt.Println("Status:", resp.StatusCode)
			fmt.Println("Body:", string(body))
			fmt.Println("======================")

			if len(body) == 0 {
				resultChan <- ProviderResult{Provider: "trust", Error: fmt.Errorf("empty response from Trust API")}
				return
			}

			var trustResponseArray []interface{}
			if err := json.Unmarshal(body, &trustResponseArray); err != nil {
				resultChan <- ProviderResult{Provider: "trust", Error: fmt.Errorf("parse error: %v, body: %s", err, string(body))}
				return
			}

			trustResponse := map[string]interface{}{
				"programs": trustResponseArray,
			}

			resultChan <- ProviderResult{Provider: "trust", Data: trustResponse}
		}()
	}

	if hasProviderPurpose("apex", purposeID) {
		activeProviders++
		go func() {
			apexCountryInfo := make([]ApexCountryInfo, len(countries))
			for i, country := range countries {
				apexCountryInfo[i] = ApexCountryInfo{ISO: country}
			}

			apexPersonInfo := make([]ApexPersonInfo, len(travelers))
			for i, bd := range travelers {
				apexPersonInfo[i] = ApexPersonInfo{Birthday: convertDateFormat(bd)}
			}

			baseURL := os.Getenv("APEX_TRAVEL_BASE_URL")
			if baseURL == "" {
				baseURL = "https://rest.aic.uz/api/ins/apex_travel"
			}

			apexLogin := os.Getenv("APEX_LOGIN")
			apexPassword := os.Getenv("APEX_PASSWORD")

			fmt.Println("=== APEX REQUEST ===")
			fmt.Println("URL:", fmt.Sprintf("%s/calculator_travel", baseURL))
			fmt.Println("Login:", apexLogin)
			fmt.Println("Password:", apexPassword)

			creds := apexLogin + ":" + apexPassword
			authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

			apexResults := make([]map[string]interface{}, 0)

			apexPurposeID, _ := getProviderPurposeID("apex", purposeID)

			for programID := 1; programID <= 5; programID++ {
				apexReq := ApexCalculatorRequest{
					TravelInfo: ApexTravelInfo{
						StartDate: convertDateFormat(startDate),
						EndDate:   convertDateFormat(endDate),
						Country:   apexCountryInfo,
						ProgramID: programID,
						PurposeID: apexPurposeID,
						GroupID:   0,
					},
					PersonInfo: apexPersonInfo,
				}

				jsonData, err := json.Marshal(apexReq)
				if err != nil {
					fmt.Printf("Program %d: Failed to marshal request: %v\n", programID, err)
					continue
				}

				fmt.Printf("Program %d Body: %s\n", programID, string(jsonData))

				httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/calculator_travel", baseURL), bytes.NewBuffer(jsonData))
				if err != nil {
					fmt.Printf("Program %d: Failed to create request: %v\n", programID, err)
					continue
				}

				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("Authorization", authHeader)

				client := &http.Client{Timeout: 30 * time.Second}
				resp, err := client.Do(httpReq)
				if err != nil {
					fmt.Printf("Program %d: Failed to send request: %v\n", programID, err)
					continue
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					fmt.Printf("Program %d: Failed to read response: %v\n", programID, err)
					continue
				}

				fmt.Printf("=== APEX PROGRAM %d RESPONSE ===\n", programID)
				fmt.Printf("Status: %d\n", resp.StatusCode)
				fmt.Printf("Body: %s\n", string(body))

				var apexResponse map[string]interface{}
				if err := json.Unmarshal(body, &apexResponse); err != nil {
					fmt.Printf("Program %d: Failed to parse response: %v\n", programID, err)
					continue
				}

				if result, ok := apexResponse["result"].(float64); ok {
					if result == 0 {
						apexResponse["program_id"] = programID
						apexResults = append(apexResults, apexResponse)
						fmt.Printf("Program %d: Success (result=0)\n", programID)
					} else {
						fmt.Printf("Program %d: No tariff (result=%.0f)\n", programID, result)
					}
				}
			}

			fmt.Println("=== APEX FINAL RESULTS ===")
			fmt.Printf("Found %d valid programs\n", len(apexResults))

			apexFinalResponse := map[string]interface{}{
				"programs": apexResults,
			}

			resultChan <- ProviderResult{Provider: "apex", Data: apexFinalResponse}
		}()
	}

	fmt.Printf("Active providers for purpose %d: %d\n", purposeID, activeProviders)

	results := make(map[string]interface{})
	for i := 0; i < activeProviders; i++ {
		result := <-resultChan
		if result.Error != nil {
			results[result.Provider] = gin.H{"error": result.Error.Error()}
		} else {
			results[result.Provider] = result.Data
		}
	}

	response := gin.H{
		"result":  results,
		"success": true,
	}

	fmt.Println("FINAL RESPONSE:")
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(responseJSON))
	fmt.Println("========================================\n")

	c.JSON(200, response)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func convertDateFormat(date string) string {
	return strings.ReplaceAll(date, "-", ".")
}

func calculateDays(startDate, endDate string) int {
	layout := "02-01-2006"
	start, err := time.Parse(layout, startDate)
	if err != nil {
		return 0
	}
	end, err := time.Parse(layout, endDate)
	if err != nil {
		return 0
	}
	duration := end.Sub(start)
	return int(duration.Hours()/24) + 1
}

type TravelSaveRequest struct {
	SessionID     string            `json:"session_id" binding:"required"`
	Provider      string            `json:"provider" binding:"required"`
	SummaAll      int               `json:"summa_all" binding:"required"`
	ProgramID     string            `json:"program_id" binding:"required"`
	Sugurtalovchi SugurtalovchiInfo `json:"sugurtalovchi" binding:"required"`
	Travelers     []TravelerInfo    `json:"travelers" binding:"required"`
}

type SugurtalovchiInfo struct {
	Type           int    `json:"type"`
	PassportSeries string `json:"passportSeries" binding:"required"`
	PassportNumber string `json:"passportNumber" binding:"required"`
	Birthday       string `json:"birthday" binding:"required"`
	Phone          string `json:"phone" binding:"required"`
	PINFL          string `json:"pinfl" binding:"required"`
	LastName       string `json:"last_name" binding:"required"`
	FirstName      string `json:"first_name" binding:"required"`
	MiddleName     string `json:"middle_name" binding:"required"`
}

type TravelerInfo struct {
	PassportSeries string `json:"passportSeries" binding:"required"`
	PassportNumber string `json:"passportNumber" binding:"required"`
	Birthday       string `json:"birthday" binding:"required"`
	PINFL          string `json:"pinfl" binding:"required"`
	LastName       string `json:"last_name" binding:"required"`
	FirstName      string `json:"first_name" binding:"required"`
}

type NeoSaveRequest struct {
	BeginDate     string            `json:"begin_date"`
	EndDate       string            `json:"end_date"`
	Days          int               `json:"days"`
	SummaAll      int               `json:"summa_all"`
	Sugurtalovchi SugurtalovchiInfo `json:"sugurtalovchi"`
	Countries     []string          `json:"countries"`
	ProgramID     string            `json:"program_id"`
	PurposeID     int               `json:"purpose_id"`
	KopMartali    bool              `json:"kop_martali"`
	IsFamily      bool              `json:"is_family"`
	HasCovid      bool              `json:"has_covid"`
	Travelers     []TravelerInfo    `json:"travelers"`
	Risklar       map[string]int    `json:"risklar"`
}

func (tc *TravelController) SaveTravel(c *gin.Context) {
	fmt.Println("\n========================================")
	fmt.Println("API 4: SAVE TRAVEL")
	fmt.Println("========================================")

	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	fmt.Println("REQUEST BODY:")
	fmt.Println(string(bodyBytes))

	var req TravelSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("VALIDATION ERROR: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	fmt.Printf("PARSED REQUEST:\n")
	fmt.Printf("  SessionID: %s\n", req.SessionID)
	fmt.Printf("  Provider: %s\n", req.Provider)
	fmt.Printf("  SummaAll: %d\n", req.SummaAll)
	fmt.Printf("  ProgramID: %s\n", req.ProgramID)
	fmt.Printf("  Sugurtalovchi: Type=%d, Passport=%s%s, Birthday=%s\n",
		req.Sugurtalovchi.Type, req.Sugurtalovchi.PassportSeries, req.Sugurtalovchi.PassportNumber, req.Sugurtalovchi.Birthday)
	fmt.Printf("  Travelers count: %d\n", len(req.Travelers))

	ctx := context.Background()
	redisKey := "travel:session:" + req.SessionID

	fmt.Printf("\nLOADING FROM REDIS: key=%s\n", redisKey)

	existingData, err := tc.RDB.Get(ctx, redisKey).Result()
	if err != nil {
		fmt.Printf("ERROR: Redis error: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": fmt.Sprintf("session not found or expired: %v", err)})
		return
	}

	fmt.Printf("SESSION DATA FROM REDIS: %s\n", existingData)

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(existingData), &sessionData); err != nil {
		fmt.Printf("ERROR: JSON unmarshal error: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": fmt.Sprintf("failed to parse session data: %v", err)})
		return
	}

	fmt.Println("\nPARSED SESSION DATA:")
	for key, value := range sessionData {
		fmt.Printf("  %s: %v (type: %T)\n", key, value, value)
	}

	startDate := sessionData["start_date"].(string)
	endDate := sessionData["end_date"].(string)
	destinations := sessionData["destinations"].([]interface{})
	purposeID := int(sessionData["purpose_id"].(float64))
	annualPolicy := sessionData["annual_policy"].(bool)
	covidProtection := sessionData["covid_protection"].(bool)

	accident := false
	luggage := false
	cancelTravel := false
	personRespon := false
	delayTravel := false

	if val, ok := sessionData["accident"].(bool); ok {
		accident = val
	}
	if val, ok := sessionData["luggage"].(bool); ok {
		luggage = val
	}
	if val, ok := sessionData["cancel_travel"].(bool); ok {
		cancelTravel = val
	}
	if val, ok := sessionData["person_respon"].(bool); ok {
		personRespon = val
	}
	if val, ok := sessionData["delay_travel"].(bool); ok {
		delayTravel = val
	}

	fmt.Printf("\nEXTRACTED VALUES:\n")
	fmt.Printf("  startDate: %s\n", startDate)
	fmt.Printf("  endDate: %s\n", endDate)
	fmt.Printf("  purposeID: %d\n", purposeID)
	fmt.Printf("  destinations: %v\n", destinations)
	fmt.Printf("  annualPolicy: %v\n", annualPolicy)
	fmt.Printf("  covidProtection: %v\n", covidProtection)
	fmt.Printf("  Risks from session: accident=%v, luggage=%v, cancel=%v, person_respon=%v, delay=%v\n",
		accident, luggage, cancelTravel, personRespon, delayTravel)

	countries := make([]string, len(destinations))
	for i, dest := range destinations {
		countries[i] = dest.(string)
	}

	days := calculateDays(startDate, endDate)
	fmt.Printf("  days: %d\n", days)

	fmt.Printf("\nPURPOSE MAPPING: ourPurposeID=%d\n", purposeID)

	if req.Provider == "neo" {
		neoPurposeID, exists := getProviderPurposeID("neo", purposeID)
		fmt.Printf("Neo purpose mapping: %d -> %d (exists: %v)\n", purposeID, neoPurposeID, exists)

		if !exists {
			fmt.Printf("ERROR: Provider 'neo' does not support purpose %d\n", purposeID)
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "provider does not support this purpose"})
			return
		}

		neoReq := NeoSaveRequest{
			BeginDate:     startDate,
			EndDate:       endDate,
			Days:          days,
			SummaAll:      req.SummaAll,
			Sugurtalovchi: req.Sugurtalovchi,
			Countries:     countries,
			ProgramID:     req.ProgramID,
			PurposeID:     neoPurposeID,
			KopMartali:    annualPolicy,
			IsFamily:      false,
			HasCovid:      covidProtection,
			Travelers:     req.Travelers,
			Risklar: map[string]int{
				"accident":      boolToInt(accident),
				"luggage":       boolToInt(luggage),
				"cancel_travel": boolToInt(cancelTravel),
				"person_respon": boolToInt(personRespon),
				"delay_travel":  boolToInt(delayTravel),
			},
		}

		fmt.Println("\n>>> SENDING TO NEO API <<<")
		neoReqJSON, _ := json.MarshalIndent(neoReq, "", "  ")
		fmt.Println("REQUEST TO NEO:")
		fmt.Println(string(neoReqJSON))

		baseURL := os.Getenv("NEO_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.neoinsurance.uz"
		}

		neoLogin := os.Getenv("NEO_LOGIN")
		neoPassword := os.Getenv("NEO_PASSWORD")

		creds := neoLogin + ":" + neoPassword
		authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

		reqBody, _ := json.Marshal(neoReq)
		neoURL := fmt.Sprintf("%s/api/travel-neo/save-polis", baseURL)

		fmt.Printf("NEO URL: %s\n", neoURL)

		httpReq, err := http.NewRequest("POST", neoURL, bytes.NewBuffer(reqBody))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create request"})
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", authHeader)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to send request to Neo"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var neoResponse map[string]interface{}
		if err := json.Unmarshal(body, &neoResponse); err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse Neo response"})
			return
		}

		fmt.Println("\n<<< RESPONSE FROM NEO API >>>")
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
		fmt.Println("RESPONSE BODY:")
		neoResponseJSON, _ := json.MarshalIndent(neoResponse, "", "  ")
		fmt.Println(string(neoResponseJSON))

		if neoResponseData, ok := neoResponse["response"].(map[string]interface{}); ok {
			if orderID, ok := neoResponseData["order_id"]; ok {
				sessionData["order_id"] = orderID
				sessionData["provider"] = req.Provider

				updatedSessionJSON, _ := json.Marshal(sessionData)
				tc.RDB.Set(ctx, redisKey, updatedSessionJSON, 30*time.Minute)

				fmt.Printf("\nSaved to session: provider=%s, order_id=%v\n", req.Provider, orderID)
			}
		}

		c.JSON(200, gin.H{
			"result": gin.H{
				"session_id": req.SessionID,
				"provider":   "neo",
				"response":   neoResponse,
			},
			"success": true,
		})
		return
	}

	if req.Provider == "trust" {
		trustPurposeID, exists := getProviderPurposeID("trust", purposeID)
		if !exists {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "provider does not support this purpose"})
			return
		}

		trustCountryIDs := make([]int, 0)
		for _, countryCode := range countries {
			alpha3Code := countryCode
			if mapped, ok := countryCodeMap[countryCode]; ok {
				alpha3Code = mapped
			}
			if id := getCountryIDByCode(alpha3Code); id > 0 {
				trustCountryIDs = append(trustCountryIDs, id)
			}
		}

		location, _ := time.LoadLocation("Asia/Tashkent")
		currentDate := time.Now().In(location).Format("02.01.2006")

		trustApplicant := map[string]interface{}{
			"fizyur":     0,
			"pass_sery":  req.Sugurtalovchi.PassportSeries,
			"pass_num":   req.Sugurtalovchi.PassportNumber,
			"date_birth": convertDateFormat(req.Sugurtalovchi.Birthday),
			"last_name":  req.Sugurtalovchi.LastName,
			"first_name": req.Sugurtalovchi.FirstName,
			"phone":      req.Sugurtalovchi.Phone,
		}

		trustInsured := make([]map[string]interface{}, len(req.Travelers))
		for i, traveler := range req.Travelers {
			trustInsured[i] = map[string]interface{}{
				"pinfl":      traveler.PINFL,
				"pass_sery":  traveler.PassportSeries,
				"pass_num":   traveler.PassportNumber,
				"date_birth": convertDateFormat(traveler.Birthday),
				"last_name":  traveler.LastName,
				"first_name": traveler.FirstName,
			}
		}

		trustReq := map[string]interface{}{
			"activity_id": trustPurposeID,
			"applicant":   trustApplicant,
			"countries":   trustCountryIDs,
			"date_reg":    currentDate,
			"days":        days,
			"end_date":    convertDateFormat(endDate),
			"start_date":  convertDateFormat(startDate),
			"group_id":    0,
			"program_id":  req.ProgramID,
			"type_id":     0,
			"multi_id":    0,
			"insured":     trustInsured,
		}

		fmt.Println("=== TRUST SAVE REQUEST ===")
		trustReqJSON, _ := json.MarshalIndent(trustReq, "", "  ")
		fmt.Println("Request Body:", string(trustReqJSON))

		baseURL := os.Getenv("TRUST_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.online-trust.uz"
		}

		trustLogin := os.Getenv("TRUST_LOGIN")
		trustPassword := os.Getenv("TRUST_PASSWORD")

		creds := trustLogin + ":" + trustPassword
		authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

		reqBody, _ := json.Marshal(trustReq)
		trustURL := fmt.Sprintf("%s/api/travel/save/create", baseURL)

		fmt.Printf("Trust URL: %s\n", trustURL)
		fmt.Printf("Trust Login: %s\n", trustLogin)
		fmt.Printf("Trust Password: %s\n", trustPassword)

		httpReq, err := http.NewRequest("POST", trustURL, bytes.NewBuffer(reqBody))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create request"})
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", authHeader)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to send request to Trust"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		fmt.Printf("Trust API Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Trust API Response Body: %s\n", string(body))

		var trustResponse map[string]interface{}
		if err := json.Unmarshal(body, &trustResponse); err != nil {
			fmt.Printf("Trust parse error: %v\n", err)
			c.JSON(500, gin.H{"result": nil, "success": false, "error": fmt.Sprintf("failed to parse Trust response: %v, body: %s", err, string(body))})
			return
		}

		if result, ok := trustResponse["result"].(float64); ok && result == 0 {
			if anketaID, ok := trustResponse["anketa_id"].(string); ok {
				sessionData["order_id"] = anketaID
				sessionData["provider"] = req.Provider

				updatedSessionJSON, _ := json.Marshal(sessionData)
				tc.RDB.Set(ctx, redisKey, updatedSessionJSON, 30*time.Minute)

				premiumUZS := "0"
				premiumTiyin := "0"
				if premium, ok := trustResponse["premium_uzs"].(string); ok {
					premiumUZS = premium
					if amount, err := strconv.ParseFloat(premium, 64); err == nil {
						premiumTiyin = fmt.Sprintf("%.0f", amount*100)
					}
				}

				clickURL := fmt.Sprintf("https://my.click.uz/services/pay?service_id=23572&merchant_id=14417&amount=%s&transaction_param=%s", premiumUZS, anketaID)

				paymeString := fmt.Sprintf("m=646c8bff2cb83937a7551c95;ac.order_id=%s;a=%s", anketaID, premiumTiyin)
				paymeEncoded := base64.StdEncoding.EncodeToString([]byte(paymeString))
				paymeURL := fmt.Sprintf("https://checkout.paycom.uz/%s", paymeEncoded)

				trustResponse["click_url"] = clickURL
				trustResponse["payme_url"] = paymeURL
			}
		}

		c.JSON(200, gin.H{
			"result": gin.H{
				"session_id": req.SessionID,
				"provider":   "trust",
				"response":   trustResponse,
			},
			"success": true,
		})
		return
	}

	if req.Provider == "apex" {
		apexPurposeID, exists := getProviderPurposeID("apex", purposeID)
		if !exists {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "provider does not support this purpose"})
			return
		}

		destinationsRaw := sessionData["destinations"].([]interface{})
		apexCountries := make([]map[string]interface{}, len(destinationsRaw))
		for i, dest := range destinationsRaw {
			apexCountries[i] = map[string]interface{}{
				"iso": dest.(string),
			}
		}

		trID := fmt.Sprintf("%d%d", time.Now().Unix(), time.Now().UnixNano()%1000000)

		apexPersonInfo := make([]map[string]interface{}, len(req.Travelers))
		for i, traveler := range req.Travelers {
			apexPersonInfo[i] = map[string]interface{}{
				"resident_p": 0,
				"country_p":  "UZ",
				"region_p":   0,
				"district_p": 0,
				"address_p":  "-",
				"gender_p":   0,
				"surname_p":  traveler.LastName,
				"name_p":     traveler.FirstName,
				"middle_p":   "-",
				"birthday_p": convertDateFormat(traveler.Birthday),
				"phone_p":    req.Sugurtalovchi.Phone,
				"email_p":    "-",
				"passport_p": map[string]interface{}{
					"pinfl_p":  nil,
					"series_p": traveler.PassportSeries,
					"number_p": traveler.PassportNumber,
				},
			}
		}

		apexReq := map[string]interface{}{
			"transaction_info": map[string]interface{}{
				"tr_id":   trID,
				"user_id": 30541,
			},
			"travel_info": map[string]interface{}{
				"start_date": convertDateFormat(startDate),
				"end_date":   convertDateFormat(endDate),
				"country":    apexCountries,
				"program_id": req.ProgramID,
				"purpose_id": apexPurposeID,
				"group_id":   0,
			},
			"insurance_info": map[string]interface{}{
				"resident_s": 0,
				"country_s":  "UZ",
				"region_s":   0,
				"district_s": 0,
				"address_s":  "-",
				"gender_s":   0,
				"surname_s":  req.Sugurtalovchi.LastName,
				"name_s":     req.Sugurtalovchi.FirstName,
				"middle_s":   "-",
				"birthday_s": convertDateFormat(req.Sugurtalovchi.Birthday),
				"phone_s":    req.Sugurtalovchi.Phone,
				"email_s":    "-",
				"passport_s": map[string]interface{}{
					"pinfl_s":  nil,
					"series_s": req.Sugurtalovchi.PassportSeries,
					"number_s": req.Sugurtalovchi.PassportNumber,
				},
			},
			"person_info": apexPersonInfo,
		}

		baseURL := os.Getenv("APEX_TRAVEL_BASE_URL")
		if baseURL == "" {
			baseURL = "https://rest.aic.uz/api/ins/apex_travel"
		}

		apexLogin := os.Getenv("APEX_LOGIN")
		apexPassword := os.Getenv("APEX_PASSWORD")

		creds := apexLogin + ":" + apexPassword
		authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

		reqBody, _ := json.Marshal(apexReq)
		apexURL := fmt.Sprintf("%s/contract_wop", baseURL)

		fmt.Println("\n=== APEX SAVE REQUEST ===")
		fmt.Printf("URL: %s\n", apexURL)
		apexReqJSON, _ := json.MarshalIndent(apexReq, "", "  ")
		fmt.Println(string(apexReqJSON))

		httpReq, err := http.NewRequest("POST", apexURL, bytes.NewBuffer(reqBody))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create request"})
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", authHeader)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			fmt.Printf("ERROR: Failed to send request: %v\n", err)
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to send request to Apex"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		fmt.Println("\n=== APEX SAVE RESPONSE ===")
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
		fmt.Printf("Response Body: %s\n", string(body))

		var apexResponse map[string]interface{}
		if err := json.Unmarshal(body, &apexResponse); err != nil {
			fmt.Printf("Parse error: %v\n", err)
			c.JSON(500, gin.H{"result": nil, "success": false, "error": fmt.Sprintf("failed to parse Apex response: %v", err)})
			return
		}

		if result, ok := apexResponse["result"].(float64); ok && result == 0 {
			if contractID, ok := apexResponse["contract_id"]; ok {
				sessionData["order_id"] = contractID
				sessionData["provider"] = req.Provider

				updatedSessionJSON, _ := json.Marshal(sessionData)
				tc.RDB.Set(ctx, redisKey, updatedSessionJSON, 30*time.Minute)
			}
		}

		c.JSON(200, gin.H{
			"result": gin.H{
				"session_id": req.SessionID,
				"provider":   "apex",
				"response":   apexResponse,
			},
			"success": true,
		})
		return
	}

	c.JSON(400, gin.H{"result": nil, "success": false, "error": "unsupported provider"})
}

type TravelCheckRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

func (tc *TravelController) CheckTravel(c *gin.Context) {
	fmt.Println("\n========================================")
	fmt.Println("API 5: CHECK TRAVEL")
	fmt.Println("========================================")

	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	fmt.Println("REQUEST BODY:")
	fmt.Println(string(bodyBytes))

	var req TravelCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("VALIDATION ERROR: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	fmt.Printf("PARSED: SessionID=%s\n", req.SessionID)

	ctx := context.Background()
	redisKey := "travel:session:" + req.SessionID

	fmt.Printf("\nLOADING FROM REDIS: key=%s\n", redisKey)

	existingData, err := tc.RDB.Get(ctx, redisKey).Result()
	if err != nil {
		fmt.Printf("ERROR: Session not found: %v\n", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "session not found or expired"})
		return
	}

	fmt.Printf("SESSION DATA FROM REDIS: %s\n", existingData)

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(existingData), &sessionData); err != nil {
		fmt.Printf("ERROR: Failed to parse session: %v\n", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse session data"})
		return
	}

	fmt.Println("\nPARSED SESSION DATA:")
	for key, value := range sessionData {
		fmt.Printf("  %s: %v (type: %T)\n", key, value, value)
	}

	provider, providerOk := sessionData["provider"].(string)
	if !providerOk {
		fmt.Println("ERROR: Provider not found in session")
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "provider not found in session"})
		return
	}

	var orderID interface{}
	var orderIDInt int
	var orderIDString string

	if orderIDFloat, ok := sessionData["order_id"].(float64); ok {
		orderID = orderIDFloat
		orderIDInt = int(orderIDFloat)
		orderIDString = fmt.Sprintf("%.0f", orderIDFloat)
	} else if orderIDStr, ok := sessionData["order_id"].(string); ok {
		orderID = orderIDStr
		orderIDString = orderIDStr
		if val, err := strconv.Atoi(orderIDStr); err == nil {
			orderIDInt = val
		}
	} else {
		fmt.Println("ERROR: Order ID not found in session")
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "order_id not found in session"})
		return
	}

	fmt.Printf("\nEXTRACTED FROM SESSION:\n")
	fmt.Printf("  provider: %s\n", provider)
	fmt.Printf("  order_id: %v\n", orderID)

	if provider == "neo" {
		checkReq := map[string]interface{}{
			"order_id": orderIDInt,
		}

		fmt.Println("\n>>> SENDING CHECK REQUEST TO NEO API <<<")
		checkReqJSON, _ := json.MarshalIndent(checkReq, "", "  ")
		fmt.Println("REQUEST TO NEO:")
		fmt.Println(string(checkReqJSON))

		baseURL := os.Getenv("NEO_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.neoinsurance.uz"
		}

		neoLogin := os.Getenv("NEO_LOGIN")
		neoPassword := os.Getenv("NEO_PASSWORD")

		creds := neoLogin + ":" + neoPassword
		authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

		reqBody, _ := json.Marshal(checkReq)
		neoURL := fmt.Sprintf("%s/api/travel-neo/checkPolis", baseURL)

		fmt.Printf("NEO URL: %s\n", neoURL)

		httpReq, err := http.NewRequest("POST", neoURL, bytes.NewBuffer(reqBody))
		if err != nil {
			fmt.Printf("ERROR: Failed to create request: %v\n", err)
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create request"})
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", authHeader)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			fmt.Printf("ERROR: Failed to send request to Neo: %v\n", err)
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to send request to Neo"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var neoResponse map[string]interface{}
		if err := json.Unmarshal(body, &neoResponse); err != nil {
			fmt.Printf("ERROR: Failed to parse Neo response: %v\n", err)
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse Neo response"})
			return
		}

		fmt.Println("\n<<< RESPONSE FROM NEO API >>>")
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
		fmt.Println("RESPONSE BODY:")
		neoResponseJSON, _ := json.MarshalIndent(neoResponse, "", "  ")
		fmt.Println(string(neoResponseJSON))

		c.JSON(200, gin.H{
			"result": gin.H{
				"session_id": req.SessionID,
				"provider":   provider,
				"response":   neoResponse,
			},
			"success": true,
		})
		return
	}

	if provider == "trust" {
		checkReq := map[string]interface{}{
			"anketa_id": orderIDString,
			"lan":       "uz",
		}

		baseURL := os.Getenv("TRUST_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.online-trust.uz"
		}

		trustLogin := os.Getenv("TRUST_LOGIN")
		trustPassword := os.Getenv("TRUST_PASSWORD")

		creds := trustLogin + ":" + trustPassword
		authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

		reqBody, _ := json.Marshal(checkReq)
		trustURL := fmt.Sprintf("%s/api/payments/check", baseURL)

		httpReq, err := http.NewRequest("POST", trustURL, bytes.NewBuffer(reqBody))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create request"})
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", authHeader)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to send request to Trust"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var trustResponse map[string]interface{}
		if err := json.Unmarshal(body, &trustResponse); err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse Trust response"})
			return
		}

		c.JSON(200, gin.H{
			"result": gin.H{
				"session_id": req.SessionID,
				"provider":   provider,
				"response":   trustResponse,
			},
			"success": true,
		})
		return
	}

	c.JSON(400, gin.H{"result": nil, "success": false, "error": "unsupported provider"})
}

func (tc *TravelController) GetCountries(c *gin.Context) {
	baseURL := os.Getenv("NEO_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.neoinsurance.uz"
	}

	neoLogin := os.Getenv("NEO_LOGIN")
	neoPassword := os.Getenv("NEO_PASSWORD")

	creds := neoLogin + ":" + neoPassword
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

	neoURL := fmt.Sprintf("%s/api/travel-neo/get-data", baseURL)

	httpReq, err := http.NewRequest("GET", neoURL, nil)
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create request"})
		return
	}

	httpReq.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to send request"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var neoResponse map[string]interface{}
	if err := json.Unmarshal(body, &neoResponse); err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse response"})
		return
	}

	c.JSON(200, gin.H{
		"result":  neoResponse,
		"success": true,
	})
}

