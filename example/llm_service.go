package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/llm"
	"github.com/go-redis/redis/v8"
	"github.com/google/generative-ai-go/genai"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"
)

func main() {
	// Инициализация Redis для кэширования
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Проверка соединения с Redis
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Printf("Предупреждение: Ошибка подключения к Redis: %v. Кэширование будет отключено.", err)
		rdb = nil
	} else {
		log.Println("Соединение с Redis установлено успешно.")
	}

	defer func() {
		if rdb != nil {
			rdb.Close()
		}
	}()

	// Получаем API ключи из переменных окружения или конфигурации
	geminiAPIKey := "sad" // Замените на фактический ключ API

	// Валидируем API ключ Gemini перед использованием
	err := validateGeminiAPIKey(ctx, geminiAPIKey)
	if err != nil {
		log.Fatalf("Недействительный ключ API Gemini: %v", err)
	}

	// Создание LLM сервиса
	service := llm.NewService(llm.ServiceOptions{
		RedisClient:     rdb,
		RateLimit:       rate.Limit(5), // 5 запросов в секунду
		RateBurst:       1,
		CacheTTL:        24 * time.Hour,
		MaxRetries:      3,
		RetryDelay:      time.Second,
		DefaultProvider: "gemini",
	})

	// Регистрация провайдера Gemini
	geminiProvider, err := llm.NewGeminiProvider(
		geminiAPIKey,
		"gemini-2.0-flash",
		nil, // Используем стандартный логгер
	)
	if err != nil {
		log.Fatalf("Ошибка при создании Gemini провайдера: %v", err)
	}
	defer geminiProvider.Close()

	service.RegisterProvider(geminiProvider)

	// Создание запроса на улучшение контента
	request := &llm.ContentRequest{
		URL:      "https://www.viwoai.com/",
		Title:    "Наши услуги",
		CTAText:  "Узнать больше",
		Content:  "Мы предлагаем широкий спектр услуг для наших клиентов.",
		Language: "en",
	}

	// Очистка кэша перед запросом, чтобы избежать использования некорректных данных
	if rdb != nil {
		cacheKey := fmt.Sprintf("llm:content:%s:%s", request.URL, request.Language)
		rdb.Del(ctx, cacheKey)

		cacheKeyHTML := fmt.Sprintf("llm:html:%s:%s", request.URL, request.Language)
		rdb.Del(ctx, cacheKeyHTML)
	}

	// Выполнение запроса
	response, err := service.GenerateContent(ctx, request, "gemini")
	if err != nil {
		log.Fatalf("Ошибка при генерации контента: %v", err)
	}

	// Проверка полноты ответа
	if response.Title == "" || response.CTAText == "" || response.Content == "" {
		log.Println("Предупреждение: Получен неполный ответ от Gemini")
	}

	// Вывод результатов
	fmt.Println("Улучшенный заголовок:", response.Title)
	fmt.Println("Улучшенная CTA-кнопка:", response.CTAText)
	fmt.Println("Улучшенный контент:", response.Content)

	// Генерация HTML для улучшенного контента
	html, err := service.GenerateHTML(ctx, request, response, "gemini")
	if err != nil {
		log.Fatalf("Ошибка при генерации HTML: %v", err)
	}

	fmt.Println("\nСгенерированный HTML:")
	fmt.Println(html)
}

// validateGeminiAPIKey проверяет валидность ключа API Gemini
func validateGeminiAPIKey(ctx context.Context, apiKey string) error {
	if apiKey == "" || apiKey == "YOUR_GEMINI_API_KEY" || apiKey == "ACTUAL_GEMINI_API_KEY" {
		return fmt.Errorf("API ключ не указан или использовано значение-заполнитель")
	}

	// Быстрая проверка с простым запросом
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return fmt.Errorf("ошибка создания клиента: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-flash")
	_, err = model.GenerateContent(ctx, genai.Text("Hello"))
	if err != nil {
		return fmt.Errorf("ошибка проверки API ключа: %w", err)
	}

	return nil
}
