package utils

import "time"

// UzbekTime возвращает текущее время в часовом поясе Узбекистана
func UzbekTime() time.Time {
	// Узбекистан: UTC+5 (постоянно, без перехода на летнее время)
	uzbekLocation, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		// Если не удалось загрузить часовой пояс, используем UTC+5
		return time.Now().UTC().Add(5 * time.Hour)
	}
	return time.Now().In(uzbekLocation)
}
