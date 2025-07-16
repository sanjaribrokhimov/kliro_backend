package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type EskizAuthResponse struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

func GetEskizToken(email, password string) (string, error) {
	url := "https://notify.eskiz.uz/api/auth/login"
	body := map[string]string{"email": email, "password": password}
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var res EskizAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Data.Token, nil
}

func SendEskizSMS(token, phone, message string) error {
	url := "https://notify.eskiz.uz/api/message/sms/send"
	body := map[string]string{"mobile_phone": phone, "message": message, "from": "4546"}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Eskiz SMS error: %v", resp.Status)
	}
	return nil
}
