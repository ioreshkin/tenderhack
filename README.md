# TenderHack MVP

Go backend with PostgreSQL for:

- streaming import of `TenderHack_СТЕ_*.xlsx` and `TenderHack_Контракты_*.xlsx`
- fast CTE search with trigram + full text ranking
- weighted NMCK calculation with region/time weighting and outlier cleanup
- browser UI for search, recalculation, manual exclusions, and document generation

## Commands

1. Start PostgreSQL:

```powershell
docker compose up -d postgres
```

2. Import data:

```powershell
go run ./cmd/tenderhack import
```

3. Start the UI:

```powershell
go run ./cmd/tenderhack serve
```

Open `http://127.0.0.1:8080`.

## Environment

Defaults:

- `DATABASE_URL=postgres://postgres:postgres@localhost:5432/tenderhack?sslmode=disable`
- `HTTP_ADDR=:8080`
- `DOCS_DIR=./generated`

If the Excel files are renamed, set:

- `CTE_FILE`
- `CONTRACTS_FILE`
