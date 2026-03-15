package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tenderhack/internal/xlsx"
)

var multiSpace = regexp.MustCompile(`\s+`)

func RunInitDB(ctx context.Context, cfg Config) error {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, schemaSQL)
	return err
}

func RunImport(ctx context.Context, cfg Config) error {
	if err := cfg.ValidateImportFiles(); err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Printf("initializing schema")
	if _, err := pool.Exec(ctx, resetImportSQL); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return err
	}

	log.Printf("importing CTE rows from %s", filepath.Base(cfg.CTEFile))
	if err := importCTE(ctx, pool, cfg.CTEFile); err != nil {
		return err
	}

	log.Printf("importing contract rows from %s", filepath.Base(cfg.ContractsFile))
	if err := importContracts(ctx, pool, cfg.ContractsFile); err != nil {
		return err
	}

	log.Printf("refreshing aggregates")
	if err := refreshAggregates(ctx, pool); err != nil {
		return err
	}

	log.Printf("import completed")
	return nil
}

func importCTE(ctx context.Context, pool *pgxpool.Pool, filePath string) error {
	buffer := make([][]any, 0, 2000)
	flush := func() error {
		if len(buffer) == 0 {
			return nil
		}
		_, err := pool.CopyFrom(ctx,
			pgx.Identifier{"cte_items"},
			[]string{"id", "name", "category", "manufacturer", "attrs_json", "attrs_text", "name_norm", "category_norm", "manufacturer_norm", "search_text"},
			pgx.CopyFromRows(buffer),
		)
		buffer = buffer[:0]
		return err
	}

	rowCount := 0
	err := xlsx.StreamRows(filePath, func(rowIndex int, values xlsx.Row) error {
		if rowIndex == 1 {
			return nil
		}

		id, ok := parseInt64(values[1])
		if !ok {
			return nil
		}

		name := values[2]
		category := values[3]
		manufacturer := values[4]
		attrsMap, attrsText := parseAttrs(values[5])
		attrsJSON, err := json.Marshal(attrsMap)
		if err != nil {
			return err
		}

		buffer = append(buffer, []any{
			id,
			cleanCell(name),
			cleanCell(category),
			cleanCell(manufacturer),
			json.RawMessage(attrsJSON),
			attrsText,
			normalizeText(name),
			normalizeText(category),
			normalizeText(manufacturer),
			buildSearchText(name, category, manufacturer, attrsText),
		})
		rowCount++
		if rowCount%10000 == 0 {
			log.Printf("cte rows imported: %d", rowCount)
		}
		if len(buffer) >= 2000 {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
}

func importContracts(ctx context.Context, pool *pgxpool.Pool, filePath string) error {
	buffer := make([][]any, 0, 3000)
	flush := func() error {
		if len(buffer) == 0 {
			return nil
		}
		_, err := pool.CopyFrom(ctx,
			pgx.Identifier{"contract_items"},
			[]string{
				"procurement_name", "quantity", "unit", "contract_id", "method",
				"initial_cost", "final_cost", "discount_pct", "vat", "contract_date",
				"customer_inn", "customer_region", "supplier_inn", "supplier_region",
				"cte_id", "cte_name", "unit_price",
			},
			pgx.CopyFromRows(buffer),
		)
		buffer = buffer[:0]
		return err
	}

	rowCount := 0
	err := xlsx.StreamRows(filePath, func(rowIndex int, values xlsx.Row) error {
		if rowIndex == 1 {
			return nil
		}

		cteID, ok := parseInt64(values[15])
		if !ok {
			return nil
		}
		contractID, _ := parseInt64(values[4])
		quantity, quantityOK := parseFloat(values[2])
		initialCost, initialOK := parseFloat(values[6])
		finalCost, finalOK := parseFloat(values[7])
		discountPct, discountOK := parseFloat(values[8])
		unitPrice, ok := parseFloat(values[17])
		if !ok || unitPrice <= 0 {
			return nil
		}
		contractDate, dateOK := parseTime(values[10])

		buffer = append(buffer, []any{
			cleanCell(values[1]),
			nullableFloat(quantity, quantityOK),
			cleanCell(values[3]),
			contractID,
			cleanCell(values[5]),
			nullableFloat(initialCost, initialOK),
			nullableFloat(finalCost, finalOK),
			nullableFloat(discountPct, discountOK),
			cleanCell(values[9]),
			nullableTime(contractDate, dateOK),
			cleanCell(values[11]),
			cleanCell(values[12]),
			cleanCell(values[13]),
			cleanCell(values[14]),
			cteID,
			cleanCell(values[16]),
			unitPrice,
		})
		rowCount++
		if rowCount%20000 == 0 {
			log.Printf("contract rows imported: %d", rowCount)
		}
		if len(buffer) >= 3000 {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
}

func refreshAggregates(ctx context.Context, pool *pgxpool.Pool) error {
	queries := []string{
		`
		UPDATE cte_items c
		SET
			contract_count = agg.contract_count,
			avg_unit_price = agg.avg_unit_price,
			last_contract_at = agg.last_contract_at
		FROM (
			SELECT cte_id, COUNT(*) AS contract_count, AVG(unit_price) AS avg_unit_price, MAX(contract_date) AS last_contract_at
			FROM contract_items
			GROUP BY cte_id
		) agg
		WHERE c.id = agg.cte_id;
		`,
		`
		INSERT INTO dataset_meta(key, value)
		VALUES
			('latest_contract_date', COALESCE((SELECT MAX(contract_date)::text FROM contract_items), '')),
			('cte_count', (SELECT COUNT(*)::text FROM cte_items)),
			('contract_count', (SELECT COUNT(*)::text FROM contract_items))
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
		`,
	}
	for _, query := range queries {
		if _, err := pool.Exec(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

func normalizeText(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "\u0451", "\u0435"))
	var b strings.Builder
	lastSpace := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(multiSpace.ReplaceAllString(b.String(), " "))
}

func parseAttrs(raw string) (map[string]string, string) {
	out := make(map[string]string)
	parts := strings.Split(raw, ";")
	for _, part := range parts {
		part = cleanCell(part)
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, ":")
		if !found {
			continue
		}
		key = normalizeText(key)
		value = cleanCell(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}

	flat := make([]string, 0, len(out))
	for key, value := range out {
		flat = append(flat, key+" "+normalizeText(value))
	}
	return out, strings.Join(flat, " ")
}

func buildSearchText(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizeText(part)
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	return strings.Join(normalized, " ")
}

func cleanCell(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " "))
	return multiSpace.ReplaceAllString(s, " ")
}

func parseInt64(s string) (int64, bool) {
	s = cleanCell(s)
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
		if err != nil {
			return 0, false
		}
		return int64(math.Round(f)), true
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return v, true
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return 0, false
	}
	return int64(math.Round(f)), true
}

func parseFloat(s string) (float64, bool) {
	s = cleanCell(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func parseTime(s string) (time.Time, bool) {
	s = cleanCell(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func nullableFloat(v float64, ok bool) any {
	if !ok {
		return nil
	}
	return v
}

func nullableTime(v time.Time, ok bool) any {
	if !ok {
		return nil
	}
	return v
}

func ensureDir(path string) error {
	if path == "" {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func fmtFloat(v float64) string {
	return fmt.Sprintf("%.2f", v)
}
