package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
)

const DEEPSEEK_API_URL = "https://api.deepseek.com/v1/chat/completions"

type DeepSeekRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type ParserController struct{}

func NewParserController() *ParserController {
	return &ParserController{}
}

func (pc *ParserController) ParsePage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}
	resp, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch url: %v", err)})
		return
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse HTML: %v", err)})
		return
	}
	text := doc.Find("body").Text()
	// Удаляем все ссылки (http/https/ftp/mailto и т.д.)
	text = regexp.MustCompile(`https?://\S+|ftp://\S+|mailto:\S+`).ReplaceAllString(text, "")
	// Удаляем html-теги (на всякий случай, если остались)
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, "")
	// Удаляем лишние пробелы и пустые строки
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	text = strings.Join(cleaned, " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if len(text) > 4000 {
		text = text[:4000]
	}
	fmt.Println("[PARSE] Очищенный текст для DeepSeek (первые 400 символов):")
	if len(text) > 400 {
		fmt.Println(text[:400])
	} else {
		fmt.Println(text)
	}
	prompt := fmt.Sprintf(`В данном тексте описаны условия кредита.
Пожалуйста, найди минимальную и максимальную процентные ставки по кредиту, указанные на странице.
Верни ответ строго в формате JSON:

{
"min_rate": "<минимальная ставка в процентах, например 12%%>",
"max_rate": "<максимальная ставка в процентах, например 26%%>"
}

Если ставка не указана — укажи null.
Вот текст:
%s`, text)
	reqBody := DeepSeekRequest{
		Model:       "deepseek-chat",
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   256,
		Temperature: 0.0,
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", DEEPSEEK_API_URL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "DeepSeek API key not set"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{}
	dsResp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to call DeepSeek API: %v", err)})
		return
	}
	defer dsResp.Body.Close()
	body, _ := ioutil.ReadAll(dsResp.Body)
	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse DeepSeek response"})
		return
	}
	if len(deepSeekResponse.Choices) > 0 {
		raw := deepSeekResponse.Choices[0].Message.Content
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse final JSON", "raw": raw})
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": result, "success": true})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "No response from DeepSeek"})
	}
}
