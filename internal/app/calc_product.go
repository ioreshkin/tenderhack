package app

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultTimeWeightMode     = "linear_floor"
	defaultSettingsMode       = "simple"
	minContractsForStableCalc = 3
	maxWindowMonths           = 12
)

type resolvedSettings struct {
	SettingsMode               string
	TimeImportanceLabel        string
	SameRegionImportanceLabel  string
	OtherRegionImportanceLabel string
	TimeWeightMode             string
	TimeDecay                  float64
	SameRegionWeight           float64
	OtherRegionWeight          float64
}

func resolveSettings(mode, timeLabel, sameLabel, otherLabel, timeWeightMode string, timeDecay, sameWeight, otherWeight float64) resolvedSettings {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		if timeLabel != "" || sameLabel != "" || otherLabel != "" {
			mode = defaultSettingsMode
		} else if timeDecay != 0 || sameWeight != 0 || otherWeight != 0 {
			mode = "advanced"
		} else {
			mode = defaultSettingsMode
		}
	}

	resolved := resolvedSettings{
		SettingsMode:   mode,
		TimeWeightMode: normalizeTimeWeightMode(timeWeightMode),
	}

	if mode == "simple" {
		resolved.TimeImportanceLabel = normalizeImportanceLabel(timeLabel, "важно")
		resolved.SameRegionImportanceLabel = normalizeImportanceLabel(sameLabel, "очень важно")
		resolved.OtherRegionImportanceLabel = normalizeImportanceLabel(otherLabel, "средне")
		resolved.TimeDecay = simpleImportanceWeight("time", resolved.TimeImportanceLabel)
		resolved.SameRegionWeight = simpleImportanceWeight("same_region", resolved.SameRegionImportanceLabel)
		resolved.OtherRegionWeight = simpleImportanceWeight("other_region", resolved.OtherRegionImportanceLabel)
		return resolved
	}

	resolved.SettingsMode = "advanced"
	resolved.TimeImportanceLabel = "пользовательский"
	resolved.SameRegionImportanceLabel = "пользовательский"
	resolved.OtherRegionImportanceLabel = "пользовательский"
	resolved.TimeDecay = clamp(timeDecay, 0, 1)
	resolved.SameRegionWeight = clamp(sameWeight, 0, 1)
	resolved.OtherRegionWeight = clamp(otherWeight, 0, 1)
	return resolved
}

func normalizeImportanceLabel(raw, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "очень важно", "важно", "средне", "неважно":
		return value
	default:
		return fallback
	}
}

func simpleImportanceWeight(kind, label string) float64 {
	switch kind {
	case "time":
		switch label {
		case "очень важно":
			return 1.00
		case "важно":
			return 0.72
		case "средне":
			return 0.42
		default:
			return 0.12
		}
	case "same_region":
		switch label {
		case "очень важно":
			return 1.00
		case "важно":
			return 0.82
		case "средне":
			return 0.60
		default:
			return 0.35
		}
	case "other_region":
		switch label {
		case "очень важно":
			return 0.90
		case "важно":
			return 0.65
		case "средне":
			return 0.35
		default:
			return 0.10
		}
	default:
		return 0
	}
}

func normalizeTimeWeightMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", defaultTimeWeightMode:
		return defaultTimeWeightMode
	case "linear":
		return "linear"
	case "step":
		return "step"
	case "exponential":
		return "exponential"
	default:
		return defaultTimeWeightMode
	}
}

func calculateTimeWeight(req CalculateRequest, referenceDate, contractDate time.Time) float64 {
	ageMonths := monthsDiff(referenceDate, contractDate)
	horizon := math.Max(float64(maxInt(req.MonthsBack, 1)), 1)
	normalizedAge := clamp(ageMonths/horizon, 0, 1)
	strength := clamp(req.TimeDecay, 0, 1)

	switch req.TimeWeightMode {
	case "exponential":
		return math.Exp(-strength * ageMonths)
	case "step":
		switch {
		case normalizedAge <= 0.25:
			return 1
		case normalizedAge <= 0.50:
			return clamp(1-0.35*strength, 0.15, 1)
		case normalizedAge <= 0.75:
			return clamp(1-0.60*strength, 0.15, 1)
		default:
			return clamp(1-0.85*strength, 0.10, 1)
		}
	case "linear":
		return clamp(1-normalizedAge*strength, 0, 1)
	default:
		drop := 0.15 + 0.75*strength
		return clamp(1-normalizedAge*drop, 0.10, 1)
	}
}

func fetchContractsWithAdaptiveWindow(ctx context.Context, pool *pgxpool.Pool, cteIDs []int64, region string, referenceDate time.Time, initialMonthsBack int) ([]contractRecord, regionSelectionInfo, WindowHint, int, error) {
	initialMonthsBack = clampInt(initialMonthsBack, 1, maxWindowMonths)
	hint := WindowHint{
		RequestedMonthsBack: initialMonthsBack,
		MinRequired:         minContractsForStableCalc,
	}
	if len(cteIDs) == 0 {
		hint.AppliedMonthsBack = initialMonthsBack
		return nil, regionSelectionInfo{}, hint, initialMonthsBack, nil
	}

	allRows, err := queryContracts(ctx, pool, cteIDs, referenceDate.AddDate(0, -maxWindowMonths, 0))
	if err != nil {
		return nil, regionSelectionInfo{}, WindowHint{}, initialMonthsBack, err
	}

	requestedRows, requestedInfo := selectContractsByRegionScope(filterContractsByMonths(allRows, referenceDate, initialMonthsBack), region)
	hint.RequestedCount = len(requestedRows)
	hint.AppliedMonthsBack = initialMonthsBack
	hint.AppliedCount = len(requestedRows)
	hint.NeedsExpansion = len(requestedRows) < minContractsForStableCalc
	if !hint.NeedsExpansion {
		return requestedRows, requestedInfo, hint, initialMonthsBack, nil
	}

	selectedRows := requestedRows
	selectedInfo := requestedInfo
	appliedMonths := initialMonthsBack
	for months := initialMonthsBack + 1; months <= maxWindowMonths; months++ {
		candidateRows, candidateInfo := selectContractsByRegionScope(filterContractsByMonths(allRows, referenceDate, months), region)
		selectedRows = candidateRows
		selectedInfo = candidateInfo
		appliedMonths = months
		hint.AppliedMonthsBack = months
		hint.AppliedCount = len(candidateRows)
		if len(candidateRows) >= minContractsForStableCalc {
			hint.ExpandedAutomatically = true
			return selectedRows, selectedInfo, hint, appliedMonths, nil
		}
	}

	hint.ExpandedAutomatically = appliedMonths > initialMonthsBack
	return selectedRows, selectedInfo, hint, appliedMonths, nil
}

func buildWindowHint(ctx context.Context, pool *pgxpool.Pool, cteIDs []int64, region string, referenceDate time.Time, requestedMonthsBack, requestedCount int) (WindowHint, error) {
	hint := WindowHint{
		RequestedMonthsBack: requestedMonthsBack,
		RequestedCount:      requestedCount,
		AppliedMonthsBack:   requestedMonthsBack,
		AppliedCount:        requestedCount,
		MinRequired:         minContractsForStableCalc,
		NeedsExpansion:      requestedCount < minContractsForStableCalc,
	}
	if !hint.NeedsExpansion || len(cteIDs) == 0 {
		return hint, nil
	}

	options := make([]WindowExpansionOption, 0, 3)
	for _, months := range suggestedMonthsBacks(requestedMonthsBack) {
		windowStart := referenceDate.AddDate(0, -months, 0)
		rows, err := queryContracts(ctx, pool, cteIDs, windowStart)
		if err != nil {
			return WindowHint{}, err
		}
		selectedRows, _ := selectContractsByRegionScope(rows, region)
		options = append(options, WindowExpansionOption{
			Label:         fmt.Sprintf("Расширить до %d мес.", months),
			MonthsBack:    months,
			ContractCount: len(selectedRows),
		})
	}
	hint.Options = options
	return hint, nil
}

func filterContractsByMonths(rows []contractRecord, referenceDate time.Time, monthsBack int) []contractRecord {
	windowStart := referenceDate.AddDate(0, -clampInt(monthsBack, 1, maxWindowMonths), 0)
	filtered := make([]contractRecord, 0, len(rows))
	for _, row := range rows {
		if row.ContractDate.Before(windowStart) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func suggestedMonthsBacks(current int) []int {
	if current <= 0 {
		current = 1
	}
	candidates := []int{current + 1, current + 2, current + 3}
	seen := map[int]struct{}{}
	out := make([]int, 0, 3)
	for _, value := range candidates {
		value = maxInt(value, current+1)
		value = minInt(value, maxWindowMonths)
		if value <= current {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) == 3 {
			break
		}
	}
	sort.Ints(out)
	return out
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func timeWeightModeLabel(mode string) string {
	switch normalizeTimeWeightMode(mode) {
	case "exponential":
		return "Экспоненциальное снижение"
	case "step":
		return "Ступенчатое снижение"
	case "linear":
		return "Линейное снижение"
	default:
		return "Линейное снижение с нижним порогом"
	}
}

func timeWeightFormula(mode string, timeDecay float64, monthsBack int) string {
	switch normalizeTimeWeightMode(mode) {
	case "exponential":
		return fmt.Sprintf("Вес по времени = e ^ (-%.2f × возраст_в_месяцах)", timeDecay)
	case "step":
		return fmt.Sprintf("Вес по времени определяется ступенчато по доле возраста закупки в окне %d мес. с коэффициентом %.2f", monthsBack, timeDecay)
	case "linear":
		return fmt.Sprintf("Вес по времени = max(0; 1 - (возраст_в_месяцах / %d) × %.2f)", monthsBack, timeDecay)
	default:
		return fmt.Sprintf("Вес по времени = max(0.10; 1 - (возраст_в_месяцах / %d) × (0.15 + 0.75 × %.2f))", monthsBack, timeDecay)
	}
}

func detailedExplainSteps(result CalculateResponse) []ExplainStep {
	steps := append([]ExplainStep(nil), result.Steps...)
	steps = append(steps,
		ExplainStep{
			Title:   "7. Формулы итогового расчета",
			Details: "Итоговый вес каждой закупки считается как произведение похожести товара, веса по времени, веса по региону и ручной поправки. НМЦК считается как взвешенное среднее по всем отобранным закупкам.",
			Metrics: map[string]string{
				"Итоговый вес закупки":    "Итоговый вес = Похожесть × Вес по времени × Вес по региону × Ручная поправка",
				"Формула НМЦК":            "НМЦК = Сумма(Цена с НДС × Итоговый вес) / Сумма(Итоговый вес)",
				"Контрольная цена":        "Взвешенная медиана по отобранным закупкам",
				"Цена без НДС":            "Цена без НДС = Цена с НДС / (1 + ставка НДС)",
				"Режим учета давности":    timeWeightModeLabel(result.Parameters.TimeWeightMode),
				"Формула веса по времени": timeWeightFormula(result.Parameters.TimeWeightMode, result.Parameters.TimeDecay, maxInt(result.Parameters.MonthsBack, 1)),
			},
		},
		ExplainStep{
			Title:   "8. Порог выбросов и диапазон цен",
			Details: "Для очистки цен используется межквартильный размах. Закупки ниже нижнего квартиля минус 1.5 межквартильного размаха и выше верхнего квартиля плюс 1.5 межквартильного размаха считаются выбросами и не участвуют в итоговом расчете.",
			Metrics: map[string]string{
				"Нижняя граница":   "нижний квартиль - 1.5 × межквартильный размах",
				"Верхняя граница":  "верхний квартиль + 1.5 × межквартильный размах",
				"Рабочий диапазон": "взвешенный диапазон между 10-м и 90-м процентилями",
			},
		},
	)
	return steps
}
