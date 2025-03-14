# Website Analyzer Backend

Полнофункциональный backend для инструмента анализа веб-сайтов, написанный на Go с использованием фреймворка Fiber.

## Особенности

- Парсинг и анализ веб-сайтов
- Полный SEO, производительность, безопасность и анализ доступности
- Генерация рекомендаций по улучшению
- Интеграция с LLM (OpenAI) для улучшения контента
- Аутентификация и авторизация на основе JWT
- Интеграция с PostgreSQL (через GORM) и Redis
- WebSocket для обновлений в реальном времени
- Управление пользователями с ролевым контролем доступа

## Требования

- Go 1.18 или выше
- PostgreSQL 13 или выше
- Redis 6 или выше
- API ключ OpenAI (опционально, для генерации контента)

## Структура проекта

```
project/
├── cmd/
│   └── server/
│       └── main.go            # Точка входа
├── internal/
│   ├── api/                   # Обработчики API и маршрутизация
│   ├── config/                # Конфигурация приложения
│   ├── database/              # Интеграция с базами данных
│   ├── models/                # Структуры данных
│   ├── repository/            # Операции с базами данных
│   └── service/               # Бизнес-логика приложения
├── pkg/
│   └── utils/                 # Вспомогательные утилиты
├── .env                       # Переменные окружения
└── go.mod                     # Зависимости Go
```

## Установка и запуск

### Предварительные требования

1. Установите Go: https://golang.org/doc/install
2. Установите PostgreSQL: https://www.postgresql.org/download/
3. Установите Redis: https://redis.io/download

### Настройка

1. Клонируйте репозиторий:

   ```bash
   git clone https://github.com/yourusername/website-analyzer.git
   cd website-analyzer
   ```

2. Установите зависимости:

   ```bash
   go mod download
   ```

3. Создайте файл .env в корне проекта:

   ```
   PORT=8080
   ENVIRONMENT=development
   POSTGRES_URI=postgres://postgres:postgres@localhost:5432/website_analyzer?sslmode=disable
   REDIS_URI=redis://localhost:6379/0
   JWT_SECRET=your-super-secret-key
   JWT_EXPIRATION_HOURS=24
   OPENAI_API_KEY=your-openai-api-key
   ```

4. Создайте базу данных в PostgreSQL:
   ```sql
   CREATE DATABASE website_analyzer;
   ```

### Запуск

1. Соберите и запустите приложение:

   ```bash
   go run cmd/server/main.go
   ```

2. API будет доступно по адресу: `http://localhost:8080/api`

## API Эндпоинты

### Аутентификация

- `POST /api/auth/register` - Регистрация нового пользователя
- `POST /api/auth/login` - Вход в систему и получение JWT-токена
- `GET /api/auth/me` - Получение информации о текущем пользователе
- `POST /api/auth/refresh` - Обновление JWT-токена
- `POST /api/auth/logout` - Выход из системы

### Пользователи

- `GET /api/users` - Получение списка пользователей (Admin)
- `GET /api/users/:id` - Получение информации о пользователе
- `PUT /api/users/:id` - Обновление данных пользователя
- `DELETE /api/users/:id` - Удаление пользователя
- `PATCH /api/users/:id/role` - Изменение роли пользователя (Admin)

### Анализ сайтов

- `POST /api/analysis` - Создание нового анализа (отправка URL)
- `GET /api/analysis` - Получение списка анализов
- `GET /api/analysis/public` - Получение списка публичных анализов
- `GET /api/analysis/:id` - Получение детальной информации об анализе
- `DELETE /api/analysis/:id` - Удаление анализа
- `PATCH /api/analysis/:id/public` - Изменение публичного статуса анализа

### Метрики и результаты

- `GET /api/analysis/:id/metrics` - Получение всех метрик анализа
- `GET /api/analysis/:id/metrics/:category` - Получение метрик определенной категории
- `GET /api/analysis/:id/issues` - Получение списка проблем
- `GET /api/analysis/:id/recommendations` - Получение рекомендаций по улучшению

### Улучшение контента

- `GET /api/analysis/:id/content-improvements` - Получение улучшенного контента
- `POST /api/analysis/:id/content-improvements` - Запрос на генерацию нового улучшенного контента
- `GET /api/analysis/:id/code-snippets` - Получение сгенерированных фрагментов кода
- `POST /api/analysis/:id/code-snippets` - Запрос на генерацию новых фрагментов кода

### WebSocket

- `GET /ws/analysis/:id` - WebSocket для получения обновлений о статусе анализа в реальном времени

## Зависимости

Основные зависимости проекта:

- [Fiber](https://github.com/gofiber/fiber) - Быстрый HTTP веб-фреймворк
- [GORM](https://gorm.io/) - ORM для Go
- [Colly](https://github.com/gocolly/colly) - Парсинг веб-сайтов
- [go-redis](https://github.com/go-redis/redis) - Клиент Redis для Go
- [jwt-go](https://github.com/golang-jwt/jwt) - Работа с JWT токенами
- [godotenv](https://github.com/joho/godotenv) - Загрузка переменных окружения из .env файла

## Тестирование

Для запуска тестов:

```bash
go test ./...
```

## Вклад в проект

1. Форкните репозиторий
2. Создайте ветку для новой функциональности (`git checkout -b feature/amazing-feature`)
3. Зафиксируйте изменения (`git commit -m 'Add amazing feature'`)
4. Отправьте изменения в свой форк (`git push origin feature/amazing-feature`)
5. Создайте Pull Request

## Лицензия

Распространяется под лицензией MIT. См. `LICENSE` для получения дополнительной информации.
