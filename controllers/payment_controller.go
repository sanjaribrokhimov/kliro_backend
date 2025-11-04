package controllers

import (
	"net/http"
	"os"
	"strconv"

	"kliro/models"
	"kliro/services"
	"kliro/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaymentController - контроллер для платежей
type PaymentController struct {
	db        *gorm.DB
	multicard *services.Multicard
}

// NewPaymentController создает новый контроллер
func NewPaymentController() *PaymentController {
	return &PaymentController{
		db:        utils.GetDB(),
		multicard: services.NewMulticard(),
	}
}

// CreatePayment - создает платеж
// POST /api/payment/create
func (pc *PaymentController) CreatePayment(c *gin.Context) {
	var req models.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Неверный запрос",
			"details": err.Error(),
		})
		return
	}

	// Проверяем, не существует ли уже платеж с таким invoice_id
	var existingPayment models.Payment
	if err := pc.db.Where("invoice_id = ?", req.InvoiceID).First(&existingPayment).Error; err == nil {
		// Платеж уже существует, возвращаем его
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"id":           existingPayment.ID,
				"invoice_id":   existingPayment.InvoiceID,
				"status":       existingPayment.Status,
				"checkout_url": existingPayment.CheckoutURL,
				"message":      "Платеж уже существует",
			},
		})
		return
	}

	// Валидация формата invoice_id (должен начинаться с префикса)
	if req.InvoiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invoice_id обязателен",
		})
		return
	}

	// Проверка формата invoice_id (рекомендуемый формат: avia{id}, hotel{id}, osago{id} и т.д.)
	validPrefixes := []string{"avia", "hotel", "osago", "travel", "insurance"}
	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if len(req.InvoiceID) > len(prefix) && req.InvoiceID[:len(prefix)] == prefix {
			hasValidPrefix = true
			break
		}
	}

	// Предупреждение если формат не рекомендуется (но не блокируем)
	if !hasValidPrefix {
		// Можно логировать предупреждение, но не блокировать запрос
	}

	// Получаем store_id из env
	storeID := os.Getenv("MULTICARD_STORE_ID")
	if storeID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Store ID не настроен",
		})
		return
	}

	// Подготовка OFD данных
	var ofdData []map[string]interface{}
	if len(req.OFD) > 0 {
		for _, item := range req.OFD {
			ofdItem := map[string]interface{}{
				"qty":          item.Qty,
				"price":        item.Price,
				"mxik":         item.MXIK,
				"package_code": item.PackageCode,
				"name":         item.Name,
			}

			if item.VAT != nil {
				ofdItem["vat"] = *item.VAT
			}
			if item.Total != nil {
				ofdItem["total"] = *item.Total
			}
			if item.TIN != nil {
				ofdItem["tin"] = *item.TIN
			}

			ofdData = append(ofdData, ofdItem)
		}
	}

	// Создаем платеж через Multicard
	result, err := pc.multicard.CreatePayment(req.PaymentMethod, req.Amount, req.InvoiceID, req.CardToken, ofdData)
	if err != nil {
		utils.LogError(err, "CreatePayment")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Не удалось создать платеж",
			"details": err.Error(),
		})
		return
	}

	// Извлекаем данные из ответа
	uuid, _ := result["uuid"].(string)
	checkoutURL, _ := result["checkout_url"].(string)
	status, _ := result["status"].(string)
	if status == "" {
		status = "pending"
	}

	// Логируем checkout_url для отладки
	if checkoutURL != "" {
		utils.LogError(nil, "Checkout URL: "+checkoutURL)
	}

	// Сохраняем платеж в БД
	payment := models.Payment{
		InvoiceID:     req.InvoiceID,
		PaymentMethod: req.PaymentMethod,
		Amount:        req.Amount,
		Status:        status,
		StoreID:       storeID,
		MulticardUUID: uuid,
		CheckoutURL:   checkoutURL,
	}

	if req.CardToken != nil {
		payment.CardToken = *req.CardToken
	}

	if err := pc.db.Create(&payment).Error; err != nil {
		// Если ошибка из-за дублирования invoice_id, возвращаем существующий платеж
		if err.Error() == "ERROR: duplicate key value violates unique constraint \"idx_payments_invoice_id\" (SQLSTATE 23505)" ||
			err.Error() == "ERROR: duplicate key value violates unique constraint \"idx_payments_invoice\" (SQLSTATE 23505)" {
			var existingPayment models.Payment
			if pc.db.Where("invoice_id = ?", req.InvoiceID).First(&existingPayment).Error == nil {
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"data": gin.H{
						"id":           existingPayment.ID,
						"invoice_id":   existingPayment.InvoiceID,
						"status":       existingPayment.Status,
						"checkout_url": existingPayment.CheckoutURL,
						"message":      "Платеж уже существует",
					},
				})
				return
			}
		}
		utils.LogError(err, "Save payment to DB")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Не удалось сохранить платеж",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           payment.ID,
			"invoice_id":   payment.InvoiceID,
			"status":       payment.Status,
			"checkout_url": payment.CheckoutURL,
			"uuid":         payment.MulticardUUID, // UUID для Multicard API
			"message":      "Платеж создан. Используйте checkout_url для оплаты.",
		},
	})
}

// GetPaymentStatus - получает статус платежа
// GET /api/payment/:id - где id это payment ID или invoice_id
func (pc *PaymentController) GetPaymentStatus(c *gin.Context) {
	id := c.Param("id")

	var payment models.Payment

	// Пытаемся найти по ID (число)
	paymentID, err := strconv.ParseUint(id, 10, 32)
	if err == nil {
		// Это числовой ID
		if err := pc.db.First(&payment, paymentID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Платеж не найден",
			})
			return
		}
	} else {
		// Это invoice_id (строка)
		if err := pc.db.Where("invoice_id = ?", id).First(&payment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Платеж не найден",
			})
			return
		}
	}

	// Если есть UUID, получаем актуальный статус из Multicard
	if payment.MulticardUUID != "" {
		statusData, err := pc.multicard.GetPaymentStatus(payment.MulticardUUID)
		if err == nil {
			if newStatus, ok := statusData["status"].(string); ok {
				payment.Status = newStatus
				pc.db.Save(&payment)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           payment.ID,
			"invoice_id":   payment.InvoiceID,
			"status":       payment.Status,
			"amount":       payment.Amount,
			"checkout_url": payment.CheckoutURL,
			"uuid":         payment.MulticardUUID, // UUID для Multicard API
		},
	})
}

// ConfirmPayment - подтверждает платеж с OTP
// POST /api/payment/:id/confirm
func (pc *PaymentController) ConfirmPayment(c *gin.Context) {
	id := c.Param("id")
	paymentID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Неверный ID платежа",
		})
		return
	}

	var req models.ConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Неверный запрос",
			"details": err.Error(),
		})
		return
	}

	// Находим платеж
	var payment models.Payment
	if err := pc.db.First(&payment, paymentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Платеж не найден",
		})
		return
	}

	if payment.MulticardUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Платеж не был создан в Multicard",
		})
		return
	}

	// Подтверждаем через Multicard
	result, err := pc.multicard.ConfirmPayment(payment.MulticardUUID, req.OTP)
	if err != nil {
		utils.LogError(err, "ConfirmPayment")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Не удалось подтвердить платеж",
			"details": err.Error(),
		})
		return
	}

	// Обновляем статус
	if status, ok := result["status"].(string); ok {
		payment.Status = status
		pc.db.Save(&payment)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":      payment.ID,
			"status":  payment.Status,
			"message": "Платеж подтвержден",
		},
	})
}

// CancelPayment - отменяет платеж
// POST /api/payment/:id/cancel - где id это payment ID, invoice_id или multicard_uuid
func (pc *PaymentController) CancelPayment(c *gin.Context) {
	id := c.Param("id")

	var payment models.Payment

	// Сначала пытаемся найти по multicard_uuid (UUID формат)
	if len(id) == 36 { // UUID обычно 36 символов
		if err := pc.db.Where("multicard_uuid = ?", id).First(&payment).Error; err == nil {
			// Найден по UUID, отменяем
			if err := pc.multicard.CancelPayment(payment.MulticardUUID); err != nil {
				utils.LogError(err, "CancelPayment")
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Не удалось отменить платеж",
					"details": err.Error(),
				})
				return
			}
			payment.Status = "cancelled"
			pc.db.Save(&payment)
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"id":      payment.ID,
					"status":  payment.Status,
					"message": "Платеж отменен",
				},
			})
			return
		}
	}

	// Пытаемся найти по ID (число)
	paymentID, err := strconv.ParseUint(id, 10, 32)
	if err == nil {
		// Это числовой ID
		if err := pc.db.First(&payment, paymentID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Платеж не найден",
			})
			return
		}
	} else {
		// Это invoice_id (строка)
		if err := pc.db.Where("invoice_id = ?", id).First(&payment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Платеж не найден",
			})
			return
		}
	}

	if payment.MulticardUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Платеж не был создан в Multicard",
		})
		return
	}

	// Отменяем через Multicard
	if err := pc.multicard.CancelPayment(payment.MulticardUUID); err != nil {
		utils.LogError(err, "CancelPayment")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Не удалось отменить платеж",
			"details": err.Error(),
		})
		return
	}

	// Обновляем статус
	payment.Status = "cancelled"
	pc.db.Save(&payment)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":      payment.ID,
			"status":  payment.Status,
			"message": "Платеж отменен",
		},
	})
}

// GetPayments - получает список платежей
// GET /api/payments?limit=20&offset=0&status=success&type=avia
func (pc *PaymentController) GetPayments(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")
	paymentType := c.Query("type") // avia, hotel, osago и т.д.

	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}

	query := pc.db.Model(&models.Payment{})

	// Фильтр по статусу
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Фильтр по типу платежа (по префиксу invoice_id)
	if paymentType != "" {
		query = query.Where("invoice_id LIKE ?", paymentType+"%")
	}

	var total int64
	query.Count(&total)

	var payments []models.Payment
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&payments)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"payments": payments,
			"total":    total,
			"limit":    limit,
			"offset":   offset,
		},
	})
}

// BindCard - создает сессию для привязки карты
// POST /api/card/bind
func (pc *PaymentController) BindCard(c *gin.Context) {
	var req models.BindCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Неверный запрос",
			"details": err.Error(),
		})
		return
	}

	result, err := pc.multicard.BindCard(req.Phone, req.ReturnURL)
	if err != nil {
		utils.LogError(err, "BindCard")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Не удалось создать сессию привязки",
			"details": err.Error(),
		})
		return
	}

	sessionID, _ := result["session_id"].(string)
	formURL, _ := result["form_url"].(string)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"session_id": sessionID,
			"form_url":   formURL,
			"message":    "Откройте form_url в браузере или WebView для привязки карты",
		},
	})
}

// GetCardBindingStatus - проверяет статус привязки карты
// GET /api/card/bind/:session_id
func (pc *PaymentController) GetCardBindingStatus(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Не указан session_id",
		})
		return
	}

	result, err := pc.multicard.GetCardBindingStatus(sessionID)
	if err != nil {
		utils.LogError(err, "GetCardBindingStatus")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Не удалось получить статус",
			"details": err.Error(),
		})
		return
	}

	status, _ := result["status"].(string)
	cardToken, _ := result["card_token"].(string)
	cardPAN, _ := result["card_pan"].(string)

	// Если карта успешно привязана и указан user_id, сохраняем в БД
	if status == "active" && cardToken != "" {
		userIDStr := c.Query("user_id")
		if userIDStr != "" {
			userID, _ := strconv.ParseUint(userIDStr, 10, 32)
			if userID > 0 {
				card := models.PaymentCard{
					UserID:        uint(userID),
					CardToken:     cardToken,
					CardPAN:       cardPAN,
					PaymentSystem: getPaymentSystem(cardPAN),
					Status:        "active",
				}

				if name, ok := result["holder_name"].(string); ok {
					card.CardName = name
				}

				pc.db.Create(&card)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetCards - получает список карт
// GET /api/cards
func (pc *PaymentController) GetCards(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Не указан user_id",
		})
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Неверный user_id",
		})
		return
	}

	var cards []models.PaymentCard
	pc.db.Where("user_id = ? AND status = ?", uint(userID), "active").Find(&cards)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cards,
	})
}

// DeleteCard - удаляет карту
// DELETE /api/card/:id
func (pc *PaymentController) DeleteCard(c *gin.Context) {
	id := c.Param("id")
	cardID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Неверный ID карты",
		})
		return
	}

	var card models.PaymentCard
	if err := pc.db.First(&card, cardID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Карта не найдена",
		})
		return
	}

	card.Status = "deleted"
	pc.db.Save(&card)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Карта удалена",
	})
}

// Callback - обрабатывает callback от Multicard
// POST /api/payment/callback
func (pc *PaymentController) Callback(c *gin.Context) {
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Неверный формат данных",
		})
		return
	}

	// Извлекаем данные
	uuid, _ := data["uuid"].(string)
	invoiceID, _ := data["store_invoice_id"].(string)
	if invoiceID == "" {
		invoiceID, _ = data["invoice_id"].(string)
	}

	if uuid == "" && invoiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Не указан uuid или invoice_id",
		})
		return
	}

	// Находим платеж
	var payment models.Payment
	query := pc.db
	if uuid != "" {
		query = query.Where("multicard_uuid = ?", uuid)
	} else {
		query = query.Where("invoice_id = ?", invoiceID)
	}

	if err := query.First(&payment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Платеж не найден",
		})
		return
	}

	// Обновляем статус
	if status, ok := data["status"].(string); ok {
		payment.Status = status
		pc.db.Save(&payment)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// getPaymentSystem определяет платежную систему по номеру карты
func getPaymentSystem(cardPAN string) string {
	if len(cardPAN) < 6 {
		return ""
	}

	firstDigit := cardPAN[0]
	firstTwo := cardPAN[:2]

	if firstTwo == "86" || firstTwo == "98" {
		if firstDigit == '8' {
			return "uzcard"
		}
		return "humo"
	}
	if firstDigit == '4' {
		return "visa"
	}
	if firstDigit == '5' {
		return "mastercard"
	}

	return "unknown"
}
