package utils

import (
	"strings"
	"sync"
)

// BankNormalizer - утилита для нормализации названий банков
type BankNormalizer struct {
	bankMappings map[string]string
	mutex        sync.RWMutex
}

var globalNormalizer *BankNormalizer
var once sync.Once

// GetBankNormalizer - возвращает глобальный экземпляр нормализатора
func GetBankNormalizer() *BankNormalizer {
	once.Do(func() {
		globalNormalizer = &BankNormalizer{
			bankMappings: make(map[string]string),
		}
		globalNormalizer.initializeMappings()
	})
	return globalNormalizer
}

// initializeMappings - инициализирует маппинг названий банков
func (bn *BankNormalizer) initializeMappings() {
	bn.mutex.Lock()
	defer bn.mutex.Unlock()

	mappings := map[string]string{
		"ipak yo'li banki": "Ipak Yo'li Banki",
		"asakabank": "Asaka Bank",
		"o‘zbekiston milliy banki": "O‘zbekiston Milliy Banki",
		"ipoteka bank": "Ipoteka Bank",
		"saderat bank": "Saderat Bank",
		"trastbank": "Trast Bank",
		"xalq banki": "Xalq Banki",
		"o‘zsanoatqurilishbank": "O‘zsanoatqurilish Bank",
		"mkbank": "MK Bank",
		"infinbank": "Infin Bank",
		"brb": "BRB",
		"orient finans bank": "Orient Finans Bank",
		"davr bank": "Davr Bank",
		"agrobank": "Agro Bank",
		"ziraat bank": "Ziraat Bank",
		"asia alliance bank": "Asia Alliance Bank",
		"tenge bank": "Tenge Bank",
		"turon bank": "Turon Bank",
		"universal bank": "Universal Bank",
		"hamkorbank": "Hamkor Bank",
		"anorbank": "Anor Bank",
		"aloqabank": "Aloqa Bank",
		"poytaxt bank": "Poytaxt Bank",
		"garant bank": "Garant Bank",
		"kapitalbank": "Kapital Bank",
		"tbc bank": "TBC Bank",
		"kdb bank uzbekiston": "KDB Bank Uzbekiston",
		"octobank": "Octo Bank",
		"hayot bank": "Hayot Bank",
		"uzum bank": "Uzum Bank",
		"avo bank": "AVO Bank",
		"mybank": "My Bank",
		"apexbank": "APEX Bank",
		"smartbank": "Smart Bank",
		"yangi bank": "Yangi Bank",
	}

	for key, value := range mappings {
		bn.bankMappings[key] = value
	}
}

// NormalizeBankName - нормализует название банка к единому формату
func (bn *BankNormalizer) NormalizeBankName(bankName string) string {
	bn.mutex.RLock()
	defer bn.mutex.RUnlock()

	if bankName == "" {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(bankName))
	if mappedName, exists := bn.bankMappings[normalized]; exists {
		return mappedName
	}

	// Если не найдено в маппинге, применяем общие правила
	return bn.capitalizeBankName(bankName)
}

// capitalizeBankName - приводит название банка к правильному регистру и отделяет 'Bank'/'Banki' как отдельное слово
func (bn *BankNormalizer) capitalizeBankName(bankName string) string {
	name := strings.TrimSpace(bankName)
	name = strings.ReplaceAll(name, "-", " ")

	// Если bank или banki слитно с другим словом, разделяем
	name = strings.ReplaceAll(name, "banki", " banki")
	name = strings.ReplaceAll(name, "Banki", " Banki")
	name = strings.ReplaceAll(name, "BANKI", " Banki")
	name = strings.ReplaceAll(name, "bank", " bank")
	name = strings.ReplaceAll(name, "Bank", " Bank")
	name = strings.ReplaceAll(name, "BANK", " Bank")

	// Удаляем двойные пробелы
	name = strings.Join(strings.Fields(name), " ")

	words := strings.Fields(name)
	for i, word := range words {
		if strings.ToLower(word) == "bank" || strings.ToLower(word) == "banki" {
			words[i] = "Bank"
			if strings.ToLower(word) == "banki" {
				words[i] = "Banki"
			}
		} else {
			words[i] = strings.Title(strings.ToLower(word))
		}
	}
	return strings.Join(words, " ")
}

// GetStandardBankNames - возвращает список стандартных названий банков
func (bn *BankNormalizer) GetStandardBankNames() []string {
	bn.mutex.RLock()
	defer bn.mutex.RUnlock()

	uniqueNames := make(map[string]bool)
	for _, name := range bn.bankMappings {
		uniqueNames[name] = true
	}

	var result []string
	for name := range uniqueNames {
		result = append(result, name)
	}
	return result
}

// IsBankName - проверяет, является ли название банком (а не мобильным приложением)
func (bn *BankNormalizer) IsBankName(bankName string) bool {
	normalized := bn.NormalizeBankName(bankName)

	// Список мобильных приложений и платежных систем (не банки)
	mobileApps := map[string]bool{
		"Davr Mobile 2.0": true,
		"Paynet":          true,
		"xazna":           true,
		"Mavrid":          true,
		"SQB Mobile":      true,
		"Sello SuperApp":  true,
		"alif mobi":       true,
		"Chakanapay":      true,
		"Oq":              true,
		"Humans":          true,
		"Multicard":       true,
		"Tenge24":         true,
		"Zoomrad":         true,
		"Beepul":          true,
		"OSON":            true,
		"Unired":          true,
		"Plum":            true,
		"Hambi":           true,
		"Digital Pay":     true,
		"Payway":          true,
		"iWon":            true,
		"My Uztelecom":    true,
		"Payme":           true,
		"Click Up":        true,
		"Paylov":          true,
		"A-Pay":           true,
		"Limon Pay":       true,
	}

	return !mobileApps[normalized]
}
