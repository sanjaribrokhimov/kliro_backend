package services

import (
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
)

// StartHotelsReferenceCron schedules nightly refresh of reference lists into Redis via HoteliosService
func StartHotelsReferenceCron() {
	// Run once at startup, then nightly at 02:00
	go refreshReferences()

	c := cron.New()
	// 0 2 * * * — at 02:00 every day
	_, _ = c.AddFunc("0 2 * * *", refreshReferences)
	c.Start()
	log.Printf("[HOTELS CRON] Scheduler started. Reference lists will refresh nightly at 02:00")
}

func refreshReferences() {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("[HOTELS CRON] Refreshing Hotelios references...")

	svc := NewHoteliosService()
	actions := []string{
		"GetServicesInRoomList",
		"GetBedTypeList",
		"GetCurrencyList",
		"GetStarList",
		"GetFacilityList",
		"GetPriceRange",
		"GetHotelTypeList",
		"GetCountryList",
	}

	refreshed := 0
	for _, action := range actions {
		if _, _, err := svc.MakeHoteliosActionRequestRaw(action, nil); err != nil {
			logger.Printf("[HOTELS CRON] %s failed: %v", action, err)
			continue
		}
		refreshed++
		// Be nice to partner API
		time.Sleep(300 * time.Millisecond)
	}

	logger.Printf("[HOTELS CRON] Refresh complete: %d/%d lists updated", refreshed, len(actions))
}
