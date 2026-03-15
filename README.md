# TenderHack NMCK

Сервис для расчета НМЦК по справочнику СТЕ и историческим закупкам.

Стек:
- `Go` backend
- `PostgreSQL`
- встроенный web UI
- генерация PDF-обоснования

## Что умеет сейчас

- Импорт `xlsx` в БД (`СТЕ` + `Контракты`)
- Быстрый поиск СТЕ:
  - по названию
  - по характеристикам
  - с исправлением раскладки и части опечаток
- Работа через лоты:
  - несколько лотов одновременно
  - один PDF по активному лоту или сводный PDF по заполненным лотам
- Добавление пользовательских позиций вручную:
  - цена
  - НДС
  - пометка источника `добавлено пользователем`
- Расчет НМЦК:
  - очистка выбросов
  - веса по времени и региону
  - автоокно поиска цен: старт с `1` месяца, расширение по `+1` до минимума `3` закупки, но не больше `12` месяцев
- PDF-обоснование:
  - для одной позиции
  - для лота
  - с НДС/без НДС и пояснениями расчета

## Быстрый старт через Docker

Требования:
- Docker Desktop

Запуск:

```powershell
docker compose up -d postgres app
```

Открыть:
- `http://127.0.0.1:8080`

Важно:
- При первом запуске контейнер `app` делает `init-db` и автоимпорт при пустой БД.
- Первый старт может быть долгим (импорт + построение search index).

Проверка:

```powershell
docker compose ps
docker compose logs app --tail 100
```

Ручной импорт (если нужно отдельно):

```powershell
docker compose run --rm import
```

Остановка:

```powershell
docker compose down
```

## Локальный запуск без Docker

1. Поднять PostgreSQL:

```powershell
docker compose up -d postgres
```

2. Инициализировать схему:

```powershell
go run ./cmd/tenderhack init-db
```

3. Импортировать данные:

```powershell
go run ./cmd/tenderhack import
```

4. Запустить сервер:

```powershell
go run ./cmd/tenderhack serve
```

Открыть:
- `http://127.0.0.1:8080`

## Команды CLI

```powershell
go run ./cmd/tenderhack serve
go run ./cmd/tenderhack import
go run ./cmd/tenderhack init-db
```

Если команда не указана, по умолчанию запускается `serve`.

## Основные API

- `GET /api/health`
- `GET /api/bootstrap`
- `GET /api/suggest?q=...`
- `GET /api/search?q=...`
- `POST /api/calculate`
- `POST /api/calculate/batch`
- `POST /api/document`
- `POST /api/document/batch`

## Переменные окружения

- `DATABASE_URL`  
  по умолчанию: `postgres://postgres:postgres@localhost:5432/tenderhack?sslmode=disable`
- `HTTP_ADDR`  
  по умолчанию: `:8080`
- `DOCS_DIR`  
  по умолчанию: `./generated`
- `CTE_FILE`  
  путь к `xlsx` со СТЕ (если имя файла нестандартное)
- `CONTRACTS_FILE`  
  путь к `xlsx` с контрактами (если имя файла нестандартное)
- `PDF_BROWSER`  
  путь к браузеру для PDF (`chromium/chrome/edge`)
- `AUTO_IMPORT`  
  для docker entrypoint (`1` = автоимпорт при пустой БД)

## Структура проекта

- `cmd/tenderhack` — точка входа CLI
- `internal/app` — backend:
  - HTTP API
  - импорт
  - поиск
  - расчет НМЦК
  - генерация PDF
  - SQL-схема
- `internal/xlsx` — потоковое чтение `xlsx`
- `internal/app/web` — встроенный frontend
- `generated` — сгенерированные PDF
- `docker-compose.yml` / `Dockerfile` — контейнерный запуск

## Тесты и проверка сборки

```powershell
node --check internal/app/web/app.js
go test ./internal/app
go build ./cmd/tenderhack
```

## Текущее поведение UI

- Правый блок лота скрыт, пока в активном лоте нет позиций.
- Переключение на другой лот очищает текущий поиск.
- Кнопка `Новый лот` создает пустой лот и сбрасывает поиск.
- Левая колонка скроллится отдельно от основной страницы.

## Ограничения

- PDF генерируется через headless browser, поэтому в окружении должен быть доступен `chromium/chrome/edge`.
- На больших наборах данных первый запуск может занимать заметное время.
