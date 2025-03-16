package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/chynybekuuludastan/website_optimizer/internal/service/parser"
)

func main() {
	// Настраиваем опции парсинга
	opts := parser.DefaultParseOptions()
	opts.UseHeadlessBrowser = true
	opts.ExecuteJavaScript = true
	opts.CaptureScreenshots = true
	opts.DetectTechnologies = true

	// Парсим сайт
	websiteData, err := parser.ParseWebsite("https://optimalux.com", opts)
	if err != nil {
		log.Fatalf("Error parsing website: %v", err)
	}

	// Сохраняем результаты в JSON
	outputPath := "results/example_com_report.json"
	if err := SaveToJSON(websiteData, outputPath); err != nil {
		log.Fatalf("Error saving results: %v", err)
	}

	fmt.Printf("Website analysis completed and saved to %s\n", outputPath)
}

// SaveToJSON сохраняет результаты парсинга в JSON файл
func SaveToJSON(data *parser.WebsiteData, filePath string) error {
	// Если путь не указан, генерируем имя файла на основе домена и текущей даты
	if filePath == "" {
		parsedURL, err := url.Parse(data.URL)
		if err != nil {
			return err
		}
		timestamp := time.Now().Format("20060102-150405")
		filePath = filepath.Join("reports", parsedURL.Hostname()+"-"+timestamp+".json")
	}

	// Создаем директорию, если она не существует
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Создаем файл
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Настраиваем encoder для красивого форматирования JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false) // Чтобы HTML не экранировался в JSON

	// Копируем данные, чтобы модифицировать их перед сохранением
	dataCopy := *data

	// Обрабатываем некоторые поля перед сохранением, если нужно
	// Например, можно конвертировать скриншоты в base64 или сохранять их отдельно

	// Если скриншоты есть и они занимают много места, можно сохранить их в отдельных файлах
	if len(dataCopy.Screenshots) > 0 {
		// Получаем имя хоста для формирования имён файлов
		parsedURL, err := url.Parse(data.URL)
		if err != nil {
			return err
		}

		screenshotsDir := filepath.Join(dir, "screenshots")
		if err := os.MkdirAll(screenshotsDir, 0755); err == nil {
			for name, imgData := range dataCopy.Screenshots {
				imgFilePath := filepath.Join(screenshotsDir, parsedURL.Hostname()+"-"+name+".png")

				// Сохраняем скриншот в отдельный файл
				if err := os.WriteFile(imgFilePath, imgData, 0644); err == nil {
					// Заменяем бинарные данные на путь к файлу в JSON
					delete(dataCopy.Screenshots, name)

					// Добавляем информацию о пути к файлу в метаданные
					if dataCopy.MetaTags == nil {
						dataCopy.MetaTags = make(map[string]string)
					}
					dataCopy.MetaTags["screenshot_"+name] = imgFilePath
				}
			}
		}
	}

	// Кодируем данные в JSON
	return encoder.Encode(dataCopy)
}
