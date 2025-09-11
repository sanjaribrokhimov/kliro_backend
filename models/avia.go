package models

// BukharaTokenResponse ответ на запрос токена
type BukharaTokenResponse struct {
	RequestID string `json:"request_id"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
	Data      struct {
		Token string `json:"token"`
	} `json:"data"`
}

// BukharaErrorResponse ответ с ошибкой
type BukharaErrorResponse struct {
	RequestID string                 `json:"request_id"`
	CreatedAt string                 `json:"created_at"`
	ErrorCode int                    `json:"error_code"`
	Message   string                 `json:"message"`
	Errors    map[string]interface{} `json:"errors,omitempty"`
}

// BukharaBaseResponse базовый ответ
type BukharaBaseResponse struct {
	RequestID string      `json:"request_id"`
	CreatedAt string      `json:"created_at"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
}

// FlightSearchRequest запрос на поиск рейсов
type FlightSearchRequest struct {
	Directions      []Direction `json:"directions" binding:"required"`
	ServiceClass    string      `json:"service_class" binding:"required"`
	Adults          int         `json:"adults" binding:"required,min=1"`
	Children        int         `json:"children" binding:"min=0"`
	Infants         int         `json:"infants" binding:"min=0"`
	InfantsWithSeat int         `json:"infants_with_seat" binding:"min=0"`
}

// Direction направление полета
type Direction struct {
	DepartureAirport string `json:"departure_airport" binding:"required"`
	ArrivalAirport   string `json:"arrival_airport" binding:"required"`
	Date             string `json:"date" binding:"required"`
}

// FlightOffer предложение авиабилета
type FlightOffer struct {
	ID                    string              `json:"id"`
	Type                  string              `json:"type"`
	IsCharter             bool                `json:"is_charter"`
	IsVtrip               bool                `json:"is_vtrip"`
	FlightType            string              `json:"flight_type"`
	Price                 Price               `json:"price"`
	RecommendedPrice      *Price              `json:"recommended_price"`
	TaxesAndFees          *Price              `json:"taxes_and_fees"`
	Transfers             bool                `json:"transfers"`
	Baggage               bool                `json:"baggage"`
	Refund                bool                `json:"refund"`
	IsFareFamily          bool                `json:"is_fare_family"`
	FareFamilyType        *string             `json:"fare_family_type"`
	FareFamilyServices    *[][]string         `json:"fare_family_services"`
	TicketingTimeLimit    string              `json:"ticketing_time_limit"`
	Passengers            []PassengerPrice    `json:"passengers"`
	AgeThresholds         *AgeThresholds      `json:"age_thresholds"`
	Documents             map[string][]string `json:"documents"`
	Directions            []FlightDirection   `json:"directions"`
	InformationForClients *[]ClientInfo       `json:"information_for_clients"`
}

// Price цена
type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// AgeThresholds возрастные пороги
type AgeThresholds struct {
	Adult  AgeRange `json:"adult"`
	Child  AgeRange `json:"child"`
	Infant AgeRange `json:"infant"`
}

// AgeRange возрастной диапазон
type AgeRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// PassengerPrice цена для пассажира
type PassengerPrice struct {
	Age   string  `json:"age"`
	Price float64 `json:"price"`
}

// FlightDirection направление полета
type FlightDirection struct {
	Departure     FlightPoint     `json:"departure"`
	Arrival       FlightPoint     `json:"arrival"`
	TravelTime    int             `json:"travel_time"`
	TransferTime  *int            `json:"transfer_time"`
	RouteDuration int             `json:"route_duration"`
	Segments      []FlightSegment `json:"segments"`
}

// FlightPoint точка полета
type FlightPoint struct {
	DateTime string  `json:"datetime"`
	Airport  Airport `json:"airport"`
}

// Airport аэропорт
type Airport struct {
	Code      string            `json:"code"`
	Title     string            `json:"title"`
	TitleIntl map[string]string `json:"title_intl"`
	Terminal  *string           `json:"terminal"`
	City      string            `json:"city"`
	CityIntl  map[string]string `json:"city_intl"`
	Country   Country           `json:"country"`
}

// Country страна
type Country struct {
	Code      string            `json:"code"`
	Title     string            `json:"title"`
	TitleIntl map[string]string `json:"title_intl"`
}

// FlightSegment сегмент полета
type FlightSegment struct {
	Departure             FlightPoint     `json:"departure"`
	Arrival               FlightPoint     `json:"arrival"`
	Airline               Airline         `json:"airline"`
	ServiceClass          string          `json:"service_class"`
	FlightNumber          string          `json:"flight_number"`
	Seats                 int             `json:"seats"`
	TravelTime            int             `json:"travel_time"`
	TransferTime          *int            `json:"transfer_time"`
	Aircraft              string          `json:"aircraft"`
	Refund                bool            `json:"refund"`
	Change                bool            `json:"change"`
	Handbags              BaggageInfo     `json:"handbags"`
	Baggage               BaggageInfo     `json:"baggage"`
	Comment               string          `json:"comment"`
	TechnicalStops        []TechnicalStop `json:"technical_stops"`
	InformationForClients []ClientInfo    `json:"information_for_clients"`
}

// Airline авиакомпания
type Airline struct {
	Code  string `json:"code"`
	Title string `json:"title"`
}

// BaggageInfo информация о багаже
type BaggageInfo struct {
	Piece  int `json:"piece"`
	Weight int `json:"weight"`
}

// TechnicalStop техническая остановка
type TechnicalStop struct {
	Airport           Airport `json:"airport"`
	ArrivalDatetime   string  `json:"arrival_datetime"`
	DepartureDatetime string  `json:"departure_datetime"`
	Duration          int     `json:"duration"`
}

// ClientInfo информация для клиентов
type ClientInfo struct {
	UZ string `json:"uz"`
	EN string `json:"en"`
	RU string `json:"ru"`
}

// BookingRequest запрос на бронирование
type BookingRequest struct {
	PayerName  string      `json:"payer_name" binding:"required"`
	PayerEmail string      `json:"payer_email" binding:"required,email"`
	PayerTel   string      `json:"payer_tel" binding:"required"`
	Passengers []Passenger `json:"passengers" binding:"required"`
}

// Passenger пассажир
type Passenger struct {
	FirstName   string `json:"first_name" binding:"required"`
	LastName    string `json:"last_name" binding:"required"`
	MiddleName  string `json:"middle_name"`
	Age         string `json:"age" binding:"required"`
	Birthdate   string `json:"birthdate" binding:"required"`
	Gender      string `json:"gender" binding:"required"`
	Citizenship string `json:"citizenship" binding:"required"`
	Tel         string `json:"tel" binding:"required"`
	DocType     string `json:"doc_type" binding:"required"`
	DocNumber   string `json:"doc_number" binding:"required"`
	DocExpire   string `json:"doc_expire" binding:"required"`
}

// BookingResponse ответ на бронирование
type BookingResponse struct {
	ID                    string             `json:"id"`
	Type                  string             `json:"type"`
	Status                string             `json:"status"`
	Created               string             `json:"created"`
	Expire                string             `json:"expire"`
	RefundAvailability    bool               `json:"refund_availability"`
	IsCharter             bool               `json:"is_charter"`
	FlightType            string             `json:"flight_type"`
	Price                 BookingPrice       `json:"price"`
	Payer                 PayerInfo          `json:"payer"`
	Passengers            []BookingPassenger `json:"passengers"`
	Directions            []FlightDirection  `json:"directions"`
	InformationForClients []ClientInfo       `json:"information_for_clients"`
}

// BookingPrice цена бронирования
type BookingPrice struct {
	HasChanged bool     `json:"has_changed"`
	PrevAmount *float64 `json:"prev_amount"`
	Amount     float64  `json:"amount"`
	Currency   string   `json:"currency"`
}

// PayerInfo информация о плательщике
type PayerInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Tel   string `json:"tel"`
}

// BookingPassenger пассажир в бронировании
type BookingPassenger struct {
	Key           string            `json:"key"`
	FirstName     string            `json:"first_name"`
	LastName      string            `json:"last_name"`
	MiddleName    *string           `json:"middle_name"`
	Email         string            `json:"email"`
	Tel           string            `json:"tel"`
	Gender        string            `json:"gender"`
	Birthdate     string            `json:"birthdate"`
	Citizenship   string            `json:"citizenship"`
	Age           string            `json:"age"`
	Document      PassengerDocument `json:"document"`
	Price         float64           `json:"price"`
	ExtendedPrice ExtendedPrice     `json:"extended_price"`
	Tickets       []Ticket          `json:"tickets"`
}

// PassengerDocument документ пассажира
type PassengerDocument struct {
	Type   string `json:"type"`
	Number string `json:"number"`
	Expire string `json:"expire"`
}

// ExtendedPrice расширенная цена
type ExtendedPrice struct {
	HasChanged bool     `json:"has_changed"`
	PrevAmount *float64 `json:"prev_amount"`
	Amount     float64  `json:"amount"`
}

// Ticket билет
type Ticket struct {
	PNR             string   `json:"pnr"`
	AirlineLocators []string `json:"airline_locators"`
	TicketNumber    *string  `json:"ticket_number"`
	Carrier         Airline  `json:"carrier"`
	Provider        string   `json:"provider"`
}

// PaymentResponse ответ на оплату
type PaymentResponse struct {
	Status        string        `json:"status"`
	Fiscalization Fiscalization `json:"fiscalization"`
}

// Fiscalization фискализация
type Fiscalization struct {
	ReceiptType          int     `json:"receipt_type"`
	IkpuProvider1        string  `json:"ikpu_provider_1"`
	PackageCodeProvider1 string  `json:"package_code_provider_1"`
	IDProvider1          string  `json:"id_provider_1"`
	Amount               float64 `json:"amount"`
	NdsProvider1         float64 `json:"nds_provider_1"`
	IkpuBookhara         string  `json:"ikpu_bookhara"`
	PackageCodeBookhara  string  `json:"package_code_bookhara"`
	ServiceFeeBookhara   float64 `json:"service_fee_bookhara"`
	NdsBookhara          float64 `json:"nds_bookhara"`
	Profit               float64 `json:"profit"`
	Discount             float64 `json:"discount"`
	TotalAmount          float64 `json:"total_amount"`
}

// FlightSearchResponse ответ на поиск рейсов
type FlightSearchResponse struct {
	RequestID string        `json:"request_id"`
	CreatedAt string        `json:"created_at"`
	Message   string        `json:"message"`
	Data      []FlightOffer `json:"data"`
}

// BookingStatus статус бронирования
type BookingStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// RefundRequest запрос на возврат
type RefundRequest struct {
	Comment string `json:"comment"`
}

// RefundResponse ответ на возврат
type RefundResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
