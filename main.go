package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"io"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	"github.com/nazzarr03/go-resume-ai/db"
	"github.com/nazzarr03/go-resume-ai/internal/auth"
	"github.com/nazzarr03/go-resume-ai/internal/user"
	"github.com/nazzarr03/go-resume-ai/pkg/middleware"
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	db.Connect()
	database := db.DB

	userRepository := user.NewUserRepository(database)
	userService := user.NewUserService(userRepository)
	userHandler := user.NewUserHandler(userService)

	authRepository := auth.NewAuthRepository(database)
	authService := auth.NewAuthService(authRepository, userRepository)
	authHandler := auth.NewAuthHandler(authService)

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Content-Type, Authorization",
		AllowCredentials: true,
	}))

	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "pong"})
	})

	app.Post("/login", authHandler.Login)
	app.Post("/register", authHandler.Register)

	api := app.Group("/users", middleware.AuthMiddleware)
	api.Get("/", userHandler.GetUsers)
	api.Get("/:id", userHandler.GetUserByID)
	api.Post("/", userHandler.CreateUser)
	api.Put("/:id", userHandler.UpdateUser)
	api.Delete("/:id", userHandler.DeleteUser)
	api.Get("/email/:email", userHandler.GetUserByEmail)
	api.Get("/name/:username", userHandler.GetUserByUsername)

	api.Post("/analyze", func(c *fiber.Ctx) error {
		var reqBody RequestBody
		if err := c.BodyParser(&reqBody); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Geçersiz JSON"})
		}

		text := strings.TrimSpace(reqBody.UserDescription)
		if text == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Boş açıklama gönderilemez"})
		}

		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "API anahtarı tanımsız"})
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

		payloadBytes, _ := json.Marshal(reqPayload)
		request, _ := http.NewRequest("POST", openRouterURL, strings.NewReader(string(payloadBytes)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{}
		resp, err := client.Do(request)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "API isteği başarısız", "detail": err.Error()})
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var openRouterResp OpenRouterResponse
		if err := json.Unmarshal(respBody, &openRouterResp); err != nil || len(openRouterResp.Choices) == 0 {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "AI cevabı çözümlenemedi"})
		}

		responseContent := openRouterResp.Choices[0].Message.Content
		fmt.Println(">> Model yanıtı:\n", responseContent)

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(responseContent), &parsed); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Geçersiz JSON üretildi"})
		}

		if projects, ok := parsed["projects"].([]interface{}); ok {
			for _, p := range projects {
				if projMap, ok := p.(map[string]interface{}); ok {
					if techUsed, exists := projMap["technologiesUsed"]; exists {
						switch v := techUsed.(type) {
						case string:
							arr := []string{}
							for _, part := range strings.Split(v, ",") {
								if trimmed := strings.TrimSpace(part); trimmed != "" {
									arr = append(arr, trimmed)
								}
							}
							projMap["technologiesUsed"] = arr
						case []interface{}:
							// zaten array
						}
					}
				}
			}
		}

		return c.JSON(parsed)
	})

	port := "8082"
	fmt.Println("Sunucu çalışıyor: http://localhost:" + port)
	log.Fatal(app.Listen(":" + port))
}
