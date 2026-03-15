package app

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var vatRatePattern = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*%`)

func resolveSelectedVAT(ctx context.Context, pool *pgxpool.Pool, selected cteRecord, attrs map[string]string) (VATInfo, error) {
	if vat, ok := vatInfoFromAttrs(attrs); ok {
		vat.Source = "карточка товара"
		return vat, nil
	}
	vat, ok, err := mostCommonVATForCTE(ctx, pool, selected.ID)
	if err != nil {
		return VATInfo{}, err
	}
	if ok {
		vat.Source = "история закупок"
		return vat, nil
	}
	return VATInfo{
		Label:    "не указана",
		Rate:     0,
		Included: true,
		Source:   "не найдена",
	}, nil
}

func vatInfoFromAttrs(attrs map[string]string) (VATInfo, bool) {
	for key, value := range attrs {
		keyNorm := normalizeText(key)
		if !strings.Contains(keyNorm, "ндс") && !strings.Contains(keyNorm, "vat") {
			continue
		}
		vat, ok := parseVATInfo(value)
		if ok {
			return vat, true
		}
	}
	return VATInfo{}, false
}

func mostCommonVATForCTE(ctx context.Context, pool *pgxpool.Pool, cteID int64) (VATInfo, bool, error) {
	var raw string
	err := pool.QueryRow(ctx, `
		SELECT vat
		FROM contract_items
		WHERE cte_id = $1 AND vat <> ''
		GROUP BY vat
		ORDER BY COUNT(*) DESC, vat ASC
		LIMIT 1
	`, cteID).Scan(&raw)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return VATInfo{}, false, nil
		}
		return VATInfo{}, false, err
	}
	vat, ok := parseVATInfo(raw)
	if !ok {
		return VATInfo{}, false, nil
	}
	return vat, true, nil
}

func parseVATInfo(raw string) (VATInfo, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return VATInfo{}, false
	}

	lower := strings.ToLower(strings.ReplaceAll(value, "ё", "е"))
	switch {
	case strings.Contains(lower, "без ндс"):
		return VATInfo{Label: "Без НДС", Rate: 0, Included: true}, true
	case strings.Contains(lower, "ндс не облагается"):
		return VATInfo{Label: "Без НДС", Rate: 0, Included: true}, true
	}

	match := vatRatePattern.FindStringSubmatch(lower)
	if len(match) == 2 {
		rateRaw := strings.ReplaceAll(match[1], ",", ".")
		rateValue, err := strconv.ParseFloat(rateRaw, 64)
		if err != nil {
			return VATInfo{}, false
		}
		label := strings.TrimSpace(strings.ToUpper(match[1]) + "%")
		return VATInfo{
			Label:    label,
			Rate:     rateValue / 100.0,
			Included: true,
		}, true
	}

	numeric := strings.ReplaceAll(lower, ",", ".")
	if rateValue, err := strconv.ParseFloat(numeric, 64); err == nil {
		if rateValue > 1 {
			rateValue = rateValue / 100.0
		}
		return VATInfo{
			Label:    fmt.Sprintf("%.0f%%", rateValue*100),
			Rate:     rateValue,
			Included: true,
		}, true
	}

	return VATInfo{}, false
}

func manualVATInfo(fallback VATInfo, vatPercent *float64) VATInfo {
	if vatPercent == nil {
		return fallback
	}
	value := clamp(*vatPercent, 0, 100)
	if value == 0 {
		return VATInfo{
			Label:    "Без НДС",
			Rate:     0,
			Included: true,
			Source:   "ручной ввод",
		}
	}
	return VATInfo{
		Label:    fmt.Sprintf("%.0f%%", value),
		Rate:     value / 100.0,
		Included: true,
		Source:   "ручной ввод",
	}
}

func amountWithoutVAT(amount float64, vat VATInfo) float64 {
	if amount <= 0 {
		return 0
	}
	if vat.Rate <= 0 {
		return amount
	}
	return amount / (1 + vat.Rate)
}

func amountWithVAT(amount float64, vat VATInfo) float64 {
	if amount <= 0 {
		return 0
	}
	if vat.Rate <= 0 {
		return amount
	}
	if vat.Included {
		return amount
	}
	return amount * (1 + vat.Rate)
}

func vatSummaryLabel(vat VATInfo) string {
	if strings.TrimSpace(vat.Label) == "" {
		return "не указана"
	}
	return vat.Label
}
