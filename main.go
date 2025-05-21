package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

type OpenRouterRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenRouterResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

type RequestBody struct {
	UserDescription string `json:"userDescription"`
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Sadece POST istekleri kabul edilir", http.StatusMethodNotAllowed)
		return
	}

	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	text := strings.TrimSpace(reqBody.UserDescription)
	if len(text) == 0 {
		http.Error(w, "Boş metin gönderilemez", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		http.Error(w, "API anahtarı eksik", http.StatusInternalServerError)
		return
	}

	prompt := fmt.Sprintf(`
Aşağıdaki açıklamayı analiz et ve sadece şu JSON yapısına birebir uygun şekilde dön (geçerli JSON üret):

{
  "personalInformation": {
    "fullName": "",
    "email": "",
    "phoneNumber": "",
    "location": "",
    "linkedin": "",
    "gitHub": "",
    "portfolio": ""
  },
  "summary": "",
  "skills": [
    { "title": "", "level": "" }
  ],
  "experience": [
    {
      "jobTitle": "",
      "company": "",
      "location": "",
      "duration": "",
      "responsibility": ""
    }
  ],
  "education": [
    {
      "degree": "",
      "university": "",
      "location": "",
      "graduationYear": ""
    }
  ],
  "certifications": [
    { "title": "", "issuingOrganization": "", "year": "" }
  ],
  "projects": [
    {
      "title": "",
      "description": "",
      "technologiesUsed": [],
      "githubLink": ""
    }
  ],
  "languages": [{ "name": "" }],
  "interests": [{ "name": "" }],
  "achievements": [{ "title": "", "year": "", "extraInformation": "" }]
}

Sadece geçerli JSON üret ve başka hiçbir şey yazma. Açıklama: %s
`, text)

	// OpenRouter API isteği
	reqPayload := OpenRouterRequest{
		Model: "gpt-3.5-turbo",
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	reqJSON, _ := json.Marshal(reqPayload)
	reqAPI, _ := http.NewRequest("POST", openRouterURL, strings.NewReader(string(reqJSON)))
	reqAPI.Header.Set("Content-Type", "application/json")
	reqAPI.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(reqAPI)
	if err != nil {
		http.Error(w, "API çağrısı başarısız oldu: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "API cevabı okunamadı", http.StatusInternalServerError)
		return
	}

	var openRouterResp OpenRouterResponse
	if err := json.Unmarshal(respBody, &openRouterResp); err != nil || len(openRouterResp.Choices) == 0 {
		http.Error(w, "API cevabı çözümlenemedi", http.StatusInternalServerError)
		return
	}

	responseContent := openRouterResp.Choices[0].Message.Content
	fmt.Println(">> Modelden gelen yanıt:\n", responseContent)

	// Doğrudan frontend'e uygun JSON'u döndür
	var parsedContent map[string]interface{}
	if err := json.Unmarshal([]byte(responseContent), &parsedContent); err != nil {
		http.Error(w, "ChatGPT çıktısı geçersiz JSON", http.StatusInternalServerError)
		return
	}

	projects, ok := parsedContent["projects"].([]interface{})
	if ok {
		for _, p := range projects {
			projMap, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			techUsed, exists := projMap["technologiesUsed"]
			if !exists {
				continue
			}
			switch v := techUsed.(type) {
			case string:
				parts := strings.Split(v, ",")
				arr := []string{}
				for _, part := range parts {
					trimmed := strings.TrimSpace(part)
					if trimmed != "" {
						arr = append(arr, trimmed)
					}
				}
				projMap["technologiesUsed"] = arr
			case []interface{}:
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parsedContent)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	http.Handle("/analyze", corsMiddleware(http.HandlerFunc(analyzeHandler)))

	port := "8080"
	fmt.Println("Sunucu başlatıldı: http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
