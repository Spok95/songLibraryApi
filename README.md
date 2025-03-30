# 🎶 Song Library API

REST API для управления онлайн-библиотекой песен с возможностью обогащения данных через внешний API и поддержки пагинации.

---

## 🚀 Возможности
- Добавление песен (с данными из внешнего mock API)
- Получение списка с фильтрацией и пагинацией
- Получение текста песни с разбивкой на куплеты
- Редактирование и удаление песен
- Swagger UI по адресу `/swagger/index.html`

---

## ⚙️ Переменные окружения (.env)

```
DATABASE_URL=postgres://postgres:password@localhost:5432/songlibrary?sslmode=disable
MOCK_API_URL=http://localhost:8081
```

---

## 🐳 Запуск через Docker

```bash
docker-compose up --build
```

API будет доступно по: `http://localhost:8080`

---

## 🛠 Локальный запуск

1. Убедитесь, что PostgreSQL запущен и доступен
2. Пропишите в `.env` данные подключения
3. При локальной проверке запустите в разных терминалах:

```bash
go run ./cmd/api
go run ./cmd/mockapi
```

---

## 📘 Swagger UI

Доступно по:  
[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

---

## 📂 Структура проекта

```
cmd/
  api/         — основной сервер
  mockapi/     — внешний мок-сервис
internal/
  models/      — модели и структуры
.env           — конфигурация
Dockerfile     — основной сервис
Dockerfile.mockapi — внешний API
```

---

## 📌 Примеры запросов

### Получить список песен
```http
GET /songs?group=Muse&limit=5&offset=0
```

### Получить куплеты
```http
GET /songs/{id}/verses?page=1&perPage=2
```

---

## ✍️ Автор

Konstantin  
GitHub: [Spok95](https://github.com/Spok95)  
Email: kostya2211@yandex.ru