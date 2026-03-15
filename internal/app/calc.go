package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var genericAttrKeys = map[string]struct{}{
	"вид продукции": {},
	"вид товаров":   {},
	"вид товара":    {},
	"описание":      {},
}

type cteRecord struct {
	ID               int64
	Name             string
	Category         string
	Manufacturer     string
	NameNorm         string
	CategoryNorm     string
	ManufacturerNorm string
	AttrsJSON        []byte
	ContractCount    int
}

type contractRecord struct {
	ID             int64
	ContractID     int64
	CTEID          int64
	CTEName        string
	Manufacturer   string
	VAT            string
	CustomerRegion string
	SupplierRegion string
	UnitPrice      float64
	Quantity       float64
	Method         string
	ContractDate   time.Time
}

func CalculateNMCK(ctx context.Context, pool *pgxpool.Pool, req CalculateRequest) (CalculateResponse, error) {
	req = normalizeRequest(req)
	selected, err := fetchCTE(ctx, pool, req.CTEID)
	if err != nil {
		return CalculateResponse{}, err
	}

	referenceDate, err := latestContractDate(ctx, pool)
	if err != nil {
		return CalculateResponse{}, err
	}

	candidates, err := fetchCandidateCTEs(ctx, pool, selected)
	if err != nil {
		return CalculateResponse{}, err
	}

	selectedAttrs := decodeAttrs(selected.AttrsJSON)
	selectedVAT, err := resolveSelectedVAT(ctx, pool, selected, selectedAttrs)
	if err != nil {
		return CalculateResponse{}, err
	}

	similarityByCTE := make(map[int64]float64, len(candidates))
	filteredIDs := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		candidateAttrs := decodeAttrs(candidate.AttrsJSON)
		similarity := cteSimilarity(selected, selectedAttrs, candidate, candidateAttrs)
		if candidate.ID == selected.ID {
			similarity = 1
		}
		if similarity >= 0.34 || candidate.ID == selected.ID {
			similarityByCTE[candidate.ID] = similarity
			filteredIDs = append(filteredIDs, candidate.ID)
		}
	}

	contracts, regionInfo, windowHint, appliedMonthsBack, err := fetchContractsWithAdaptiveWindow(ctx, pool, filteredIDs, req.Region, referenceDate, req.MonthsBack)
	if err != nil {
		return CalculateResponse{}, err
	}
	req.MonthsBack = appliedMonthsBack
	windowStart := referenceDate.AddDate(0, -req.MonthsBack, 0)

	excludedSet := make(map[int64]struct{}, len(req.ExcludedIDs))
	for _, id := range req.ExcludedIDs {
		excludedSet[id] = struct{}{}
	}

	priceOverrides := make(map[int64]float64, len(req.PriceOverrides))
	for _, item := range req.PriceOverrides {
		if item.ContractRowID <= 0 || item.UnitPrice <= 0 {
			continue
		}
		priceOverrides[item.ContractRowID] = item.UnitPrice
	}

	weightOverrides := make(map[int64]float64, len(req.WeightOverrides))
	for _, item := range req.WeightOverrides {
		if item.ContractRowID <= 0 || item.WeightMultiplier <= 0 {
			continue
		}
		weightOverrides[item.ContractRowID] = item.WeightMultiplier
	}

	rawCount := len(contracts)
	autoResults := make([]AnalogResult, 0, len(contracts))
	trustedManual := buildManualAnalogResults(req.ManualEntries, selected, referenceDate, req, selectedVAT)
	prices := make([]float64, 0, len(contracts))
	overriddenContracts := 0

	for _, contract := range contracts {
		if _, excluded := excludedSet[contract.ID]; excluded {
			continue
		}

		similarity := similarityByCTE[contract.CTEID]
		timeWeight := calculateTimeWeight(req, referenceDate, contract.ContractDate)
		regionTier := classifyRegionTier(req.Region, contract.CustomerRegion, contract.SupplierRegion)
		regionWeight := regionWeightForTier(req, regionTier)
		weightMultiplier := 1.0
		if value, ok := weightOverrides[contract.ID]; ok {
			weightMultiplier = value
		}

		unitPrice := contract.UnitPrice
		priceOverridden := false
		if value, ok := priceOverrides[contract.ID]; ok {
			unitPrice = value
			priceOverridden = true
			overriddenContracts++
		}

		contractVAT := selectedVAT
		if parsedVAT, ok := parseVATInfo(contract.VAT); ok {
			contractVAT = parsedVAT
			contractVAT.Source = "закупка"
		}

		result := AnalogResult{
			ID:                contract.ID,
			ContractID:        contract.ContractID,
			CTEID:             contract.CTEID,
			CTEName:           contract.CTEName,
			Manufacturer:      contract.Manufacturer,
			Region:            displayRegionName(contract.CustomerRegion),
			SupplierRegion:    displayRegionName(contract.SupplierRegion),
			RegionTier:        tierLabel(regionTier),
			UnitPrice:         unitPrice,
			UnitPriceNoVAT:    amountWithoutVAT(unitPrice, contractVAT),
			OriginalUnitPrice: contract.UnitPrice,
			Quantity:          contract.Quantity,
			Method:            contract.Method,
			ContractDate:      contract.ContractDate,
			Similarity:        similarity,
			TimeWeight:        timeWeight,
			RegionWeight:      regionWeight,
			WeightMultiplier:  weightMultiplier,
			FinalWeight:       similarity * timeWeight * regionWeight * weightMultiplier,
			Manual:            priceOverridden,
			SourceLabel:       contract.Method,
			PriceOverridden:   priceOverridden,
			VAT:               contractVAT,
		}
		if priceOverridden {
			trustedManual = append(trustedManual, result)
			continue
		}
		autoResults = append(autoResults, result)
		prices = append(prices, unitPrice)
	}

	lower, upper := iqrBounds(prices)
	valid := make([]AnalogResult, 0, len(autoResults)+len(trustedManual))
	outliers := 0
	for _, item := range autoResults {
		if item.UnitPrice < lower || item.UnitPrice > upper {
			outliers++
			continue
		}
		valid = append(valid, item)
	}
	valid = append(valid, trustedManual...)

	sort.Slice(valid, func(i, j int) bool {
		if valid[i].FinalWeight == valid[j].FinalWeight {
			return valid[i].ContractDate.After(valid[j].ContractDate)
		}
		return valid[i].FinalWeight > valid[j].FinalWeight
	})
	if len(valid) > req.MaxResults {
		valid = valid[:req.MaxResults]
	}

	rangeMin := weightedQuantile(valid, 0.10)
	rangeMax := weightedQuantile(valid, 0.90)
	weightedMeanValue := weightedMean(valid)
	weightedMedian := weightedQuantile(valid, 0.50)
	rangeMinNoVAT := weightedQuantileNoVAT(valid, 0.10)
	rangeMaxNoVAT := weightedQuantileNoVAT(valid, 0.90)
	weightedMeanNoVAT := weightedMeanNoVAT(valid)
	weightedMedianNoVAT := weightedQuantileNoVAT(valid, 0.50)
	top := topRecommendations(valid)
	documentResults := selectDocumentAnalogs(valid)
	manualAdjustments := len(trustedManual)

	steps := []ExplainStep{
		{
			Title:   "1. Подбор похожих позиций СТЕ",
			Details: "Система берет позиции из той же категории, сравнивает их по названию, производителю и характеристикам и оставляет только действительно близкие варианты.",
			Metrics: map[string]string{
				"Выбранный код СТЕ":       fmt.Sprintf("%d", selected.ID),
				"Найдено похожих позиций": fmt.Sprintf("%d", len(candidates)),
				"Оставили подходящих":     fmt.Sprintf("%d", len(filteredIDs)),
			},
		},
		{
			Title:   "2. Отбор закупок по периоду и региону",
			Details: "Берутся закупки только за выбранный период. Если по нужному региону данных мало, система добавляет близкие регионы, тот же федеральный округ и только затем остальные регионы. Если закупок в окне меньше трех, пользователю предлагаются более широкие окна поиска.",
			Metrics: map[string]string{
				"Найдено закупок до очистки": fmt.Sprintf("%d", rawCount),
				"Начало периода поиска":      windowStart.Format("2006-01-02"),
				"Дата расчета":               referenceDate.Format("2006-01-02"),
				"region_scope":               regionInfo.Scope,
				"region_tiers":               strings.Join(regionInfo.UsedTiers, ", "),
				"requested_count":            fmt.Sprintf("%d", windowHint.RequestedCount),
				"min_required":               fmt.Sprintf("%d", windowHint.MinRequired),
			},
		},
		{
			Title:   "3. Очистка выбросов",
			Details: "Слишком низкие и слишком высокие цены автоматически отбрасываются, чтобы единичные аномалии не искажали итоговый результат. Ручные цены пользователя не выбрасываются автоматически.",
			Metrics: map[string]string{
				"Нижняя граница нормальной цены":  fmtFloat(lower),
				"Верхняя граница нормальной цены": fmtFloat(upper),
				"Убрано выбросов":                 fmt.Sprintf("%d", outliers),
			},
		},
		{
			Title:   "4. Расчет важности каждой закупки",
			Details: "Каждая закупка получает свой вклад в расчет. Чем закупка свежее, ближе по региону и больше похожа на ваш товар, тем сильнее она влияет на итоговую цену.",
			Metrics: map[string]string{
				"Скорость устаревания цен":   fmt.Sprintf("%.3f", req.TimeDecay),
				"Режим учета давности":       timeWeightModeLabel(req.TimeWeightMode),
				"Важность своего региона":    fmt.Sprintf("%.2f", req.SameRegionWeight),
				"Важность других регионов":   fmt.Sprintf("%.2f", req.OtherRegionWeight),
				"Режим настройки":            req.SettingsMode,
				"Важность свежих цен":        req.TimeImportanceLabel,
				"Важность региона заказчика": req.SameRegionImportanceLabel,
				"Учет других регионов":       req.OtherRegionImportanceLabel,
				"region_scope":               regionInfo.Scope,
				"Ручных правок":              fmt.Sprintf("%d", manualAdjustments+overriddenContracts),
			},
		},
		{
			Title:   "5. НДС и итоговые цены",
			Details: "Цены из файла используются как цены с НДС. Для наглядности система дополнительно рассчитывает цену без НДС по применяемой ставке товара.",
			Metrics: map[string]string{
				"Ставка НДС":       vatSummaryLabel(selectedVAT),
				"НМЦК с НДС":       fmtFloat(weightedMeanValue),
				"НМЦК без НДС":     fmtFloat(weightedMeanNoVAT),
				"Диапазон с НДС":   fmt.Sprintf("%s - %s", fmtFloat(rangeMin), fmtFloat(rangeMax)),
				"Диапазон без НДС": fmt.Sprintf("%s - %s", fmtFloat(rangeMinNoVAT), fmtFloat(rangeMaxNoVAT)),
			},
		},
		{
			Title:   "6. Отбор закупок в документ",
			Details: "В PDF попадают только самые полезные закупки: минимум 3 и максимум 10. Система добирает их по важности, покрытию вклада в расчет и разнообразию по контрактам.",
			Metrics: map[string]string{
				"Закупок в документе":       fmt.Sprintf("%d", len(documentResults)),
				"Целевое покрытие вклада":   ">= 82%",
				"Лимит закупок в документе": "3..10",
			},
		},
	}
	steps = detailedExplainSteps(CalculateResponse{
		Selected: SelectedCTE{
			ID:           selected.ID,
			Name:         selected.Name,
			Category:     selected.Category,
			Manufacturer: selected.Manufacturer,
			Attributes:   selectedAttrs,
			VAT:          selectedVAT,
		},
		Parameters: EffectiveParams{
			Region:                     req.Region,
			MonthsBack:                 req.MonthsBack,
			TimeDecay:                  req.TimeDecay,
			TimeWeightMode:             req.TimeWeightMode,
			SameRegionWeight:           req.SameRegionWeight,
			OtherRegionWeight:          req.OtherRegionWeight,
			SettingsMode:               req.SettingsMode,
			TimeImportanceLabel:        req.TimeImportanceLabel,
			SameRegionImportanceLabel:  req.SameRegionImportanceLabel,
			OtherRegionImportanceLabel: req.OtherRegionImportanceLabel,
			MaxResults:                 req.MaxResults,
		},
		Summary: SummaryBlock{
			WindowHint: windowHint,
			VAT:        selectedVAT,
		},
		Steps: steps,
	})

	warnings := []string{}
	if len(valid) < 5 {
		warnings = append(warnings, "Найдено мало подходящих закупок. Итоговую цену лучше проверить вручную.")
	}
	if regionInfo.FallbackToAll {
		warnings = append(warnings, "По выбранному региону данных не хватило, поэтому в расчет добавлены закупки из других регионов.")
	}
	if manualAdjustments > 0 || overriddenContracts > 0 {
		warnings = append(warnings, "В расчете есть ручные изменения цен или влияния отдельных закупок.")
	}
	if len(documentResults) < 3 {
		warnings = append(warnings, "Для документа удалось отобрать меньше трех закупок. Желательно уточнить товар или добавить свои цены вручную.")
	}
	if windowHint.ExpandedAutomatically && windowHint.AppliedMonthsBack > windowHint.RequestedMonthsBack {
		warnings = append(warnings, fmt.Sprintf("Поиск цен автоматически расширен с %d до %d мес., потому что в исходном окне было только %d закупок.", windowHint.RequestedMonthsBack, windowHint.AppliedMonthsBack, windowHint.RequestedCount))
	} else if windowHint.NeedsExpansion {
		warnings = append(warnings, fmt.Sprintf("Даже после авторасширения с %d до %d мес. найдено только %d закупок.", windowHint.RequestedMonthsBack, windowHint.AppliedMonthsBack, windowHint.AppliedCount))
	}
	if selectedVAT.Label == "не указана" {
		warnings = append(warnings, "Ставку НДС не удалось определить автоматически. Цены без НДС показаны как равные ценам с НДС.")
	}

	return CalculateResponse{
		Selected: SelectedCTE{
			ID:           selected.ID,
			Name:         selected.Name,
			Category:     selected.Category,
			Manufacturer: selected.Manufacturer,
			Attributes:   selectedAttrs,
			VAT:          selectedVAT,
		},
		Parameters: EffectiveParams{
			Region:                     req.Region,
			MonthsBack:                 req.MonthsBack,
			TimeDecay:                  req.TimeDecay,
			TimeWeightMode:             req.TimeWeightMode,
			SameRegionWeight:           req.SameRegionWeight,
			OtherRegionWeight:          req.OtherRegionWeight,
			SettingsMode:               req.SettingsMode,
			TimeImportanceLabel:        req.TimeImportanceLabel,
			SameRegionImportanceLabel:  req.SameRegionImportanceLabel,
			OtherRegionImportanceLabel: req.OtherRegionImportanceLabel,
			MaxResults:                 req.MaxResults,
		},
		Summary: SummaryBlock{
			RawContracts:            rawCount,
			ValidContracts:          len(valid),
			DocumentContracts:       len(documentResults),
			ExcludedOutliers:        outliers,
			ManualExclusions:        len(req.ExcludedIDs),
			ManualEntries:           manualAdjustments,
			PriceRangeMin:           rangeMin,
			PriceRangeMax:           rangeMax,
			PriceRangeMinNoVAT:      rangeMinNoVAT,
			PriceRangeMaxNoVAT:      rangeMaxNoVAT,
			NMCKWeightedMean:        weightedMeanValue,
			NMCKWeightedMedian:      weightedMedian,
			NMCKWeightedMeanNoVAT:   weightedMeanNoVAT,
			NMCKWeightedMedianNoVAT: weightedMedianNoVAT,
			VAT:                     selectedVAT,
			FallbackToAllRegion:     regionInfo.FallbackToAll,
			RegionScope:             regionInfo.Scope,
			UsedRegionTiers:         regionInfo.UsedTiers,
			RegionCounts:            regionInfo.Counts,
			ReferenceDate:           referenceDate.Format(time.RFC3339),
			WindowHint:              windowHint,
			Warnings:                warnings,
		},
		TopRecommendations: top,
		Results:            valid,
		DocumentResults:    documentResults,
		Steps:              steps,
	}, nil
}

func fetchCTE(ctx context.Context, pool *pgxpool.Pool, id int64) (cteRecord, error) {
	const sql = `
	SELECT id, name, category, manufacturer, name_norm, category_norm, manufacturer_norm, attrs_json, contract_count
	FROM cte_items
	WHERE id = $1
	`
	var row cteRecord
	err := pool.QueryRow(ctx, sql, id).Scan(&row.ID, &row.Name, &row.Category, &row.Manufacturer, &row.NameNorm, &row.CategoryNorm, &row.ManufacturerNorm, &row.AttrsJSON, &row.ContractCount)
	if err != nil {
		return cteRecord{}, err
	}
	return row, nil
}

func latestContractDate(ctx context.Context, pool *pgxpool.Pool) (time.Time, error) {
	var value time.Time
	err := pool.QueryRow(ctx, "SELECT COALESCE(MAX(contract_date), now()) FROM contract_items").Scan(&value)
	return value, err
}

func fetchCandidateCTEs(ctx context.Context, pool *pgxpool.Pool, selected cteRecord) ([]cteRecord, error) {
	const sql = `
	SELECT id, name, category, manufacturer, name_norm, category_norm, manufacturer_norm, attrs_json, contract_count
	FROM cte_items
	WHERE category = $1 AND contract_count > 0
	ORDER BY
		CASE WHEN id = $2 THEN 0 ELSE 1 END,
		similarity(name_norm, $3) DESC,
		CASE WHEN manufacturer_norm = $4 AND $4 <> '' THEN 0 ELSE 1 END,
		contract_count DESC
	LIMIT 300
	`
	rows, err := pool.Query(ctx, sql, selected.Category, selected.ID, selected.NameNorm, selected.ManufacturerNorm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []cteRecord
	for rows.Next() {
		var item cteRecord
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.Manufacturer, &item.NameNorm, &item.CategoryNorm, &item.ManufacturerNorm, &item.AttrsJSON, &item.ContractCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func fetchContracts(ctx context.Context, pool *pgxpool.Pool, cteIDs []int64, region string, windowStart time.Time) ([]contractRecord, regionSelectionInfo, error) {
	rows, err := queryContracts(ctx, pool, cteIDs, windowStart)
	if err != nil {
		return nil, regionSelectionInfo{}, err
	}
	selectedRows, info := selectContractsByRegionScope(rows, region)
	return selectedRows, info, nil
}

func queryContracts(ctx context.Context, pool *pgxpool.Pool, cteIDs []int64, windowStart time.Time) ([]contractRecord, error) {
	const sql = `
	SELECT
		ci.id,
		ci.contract_id,
		ci.cte_id,
		ci.cte_name,
		c.manufacturer,
		ci.vat,
		ci.customer_region,
		ci.supplier_region,
		ci.unit_price::float8,
		COALESCE(ci.quantity::float8, 0),
		ci.method,
		COALESCE(ci.contract_date, now())
	FROM contract_items ci
	JOIN cte_items c ON c.id = ci.cte_id
	WHERE ci.cte_id = ANY($1)
		AND ci.unit_price > 0
		AND ci.contract_date >= $2
	ORDER BY ci.contract_date DESC
	LIMIT 6000
	`
	rows, err := pool.Query(ctx, sql, cteIDs, windowStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []contractRecord
	for rows.Next() {
		var item contractRecord
		if err := rows.Scan(&item.ID, &item.ContractID, &item.CTEID, &item.CTEName, &item.Manufacturer, &item.VAT, &item.CustomerRegion, &item.SupplierRegion, &item.UnitPrice, &item.Quantity, &item.Method, &item.ContractDate); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func cteSimilarity(selected cteRecord, selectedAttrs map[string]string, candidate cteRecord, candidateAttrs map[string]string) float64 {
	nameOverlap := tokenJaccard(selected.NameNorm, candidate.NameNorm)
	attrOverlap := attrsSimilarity(selectedAttrs, candidateAttrs)
	manufacturerScore := 0.0
	if selected.ManufacturerNorm != "" && selected.ManufacturerNorm == candidate.ManufacturerNorm {
		manufacturerScore = 1
	}
	popularityScore := math.Min(float64(candidate.ContractCount)/15.0, 1)
	score := 0.45*nameOverlap + 0.35*attrOverlap + 0.10*manufacturerScore + 0.10*popularityScore
	return clamp(score, 0, 1)
}

func tokenJaccard(a, b string) float64 {
	setA := splitSet(a)
	setB := splitSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	union := make(map[string]struct{}, len(setA)+len(setB))
	for key := range setA {
		union[key] = struct{}{}
		if _, ok := setB[key]; ok {
			inter++
		}
	}
	for key := range setB {
		union[key] = struct{}{}
	}
	return float64(inter) / float64(len(union))
}

func splitSet(s string) map[string]struct{} {
	parts := strings.Fields(s)
	set := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		set[part] = struct{}{}
	}
	return set
}

func attrsSimilarity(a, b map[string]string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	total := 0.0
	count := 0.0
	for key, valueA := range a {
		if _, generic := genericAttrKeys[key]; generic {
			continue
		}
		valueB, ok := b[key]
		if !ok {
			continue
		}
		count++
		score := 0.0
		normA := normalizeText(valueA)
		normB := normalizeText(valueB)
		switch {
		case normA == normB:
			score = 1
		case strings.Contains(normA, normB) || strings.Contains(normB, normA):
			score = 0.8
		default:
			floatA, okA := parseFloat(normA)
			floatB, okB := parseFloat(normB)
			if okA && okB {
				diff := math.Abs(floatA-floatB) / math.Max(floatA, floatB)
				if diff <= 0.05 {
					score = 0.95
				} else if diff <= 0.12 {
					score = 0.7
				}
			}
		}
		total += score
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func decodeAttrs(raw []byte) map[string]string {
	out := make(map[string]string)
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func normalizeRequest(req CalculateRequest) CalculateRequest {
	if req.MonthsBack <= 0 {
		req.MonthsBack = 1
	}
	if req.MaxResults <= 0 || req.MaxResults > 500 {
		req.MaxResults = 150
	}
	settings := resolveSettings(req.SettingsMode, req.TimeImportanceLabel, req.SameRegionImportanceLabel, req.OtherRegionImportanceLabel, req.TimeWeightMode, req.TimeDecay, req.SameRegionWeight, req.OtherRegionWeight)
	req.SettingsMode = settings.SettingsMode
	req.TimeImportanceLabel = settings.TimeImportanceLabel
	req.SameRegionImportanceLabel = settings.SameRegionImportanceLabel
	req.OtherRegionImportanceLabel = settings.OtherRegionImportanceLabel
	req.TimeWeightMode = settings.TimeWeightMode
	req.TimeDecay = settings.TimeDecay
	req.SameRegionWeight = settings.SameRegionWeight
	req.OtherRegionWeight = settings.OtherRegionWeight
	req.Region = canonicalizeRegion(strings.TrimSpace(req.Region))
	req.PriceOverrides = sanitizePriceOverrides(req.PriceOverrides)
	req.WeightOverrides = sanitizeWeightOverrides(req.WeightOverrides)
	req.ManualEntries = sanitizeManualEntries(req.ManualEntries)
	return req
}

func sanitizePriceOverrides(items []PriceOverride) []PriceOverride {
	out := make([]PriceOverride, 0, len(items))
	for _, item := range items {
		if item.ContractRowID <= 0 || item.UnitPrice <= 0 {
			continue
		}
		out = append(out, item)
	}
	return out
}

func sanitizeWeightOverrides(items []WeightOverride) []WeightOverride {
	out := make([]WeightOverride, 0, len(items))
	for _, item := range items {
		if item.ContractRowID <= 0 || item.WeightMultiplier <= 0 {
			continue
		}
		out = append(out, item)
	}
	return out
}

func sanitizeManualEntries(items []ManualEntry) []ManualEntry {
	out := make([]ManualEntry, 0, len(items))
	for _, item := range items {
		item.Label = cleanCell(item.Label)
		item.Region = canonicalizeRegion(cleanCell(item.Region))
		item.SupplierRegion = canonicalizeRegion(cleanCell(item.SupplierRegion))
		if item.UnitPrice <= 0 {
			continue
		}
		if item.WeightMultiplier <= 0 {
			item.WeightMultiplier = 1
		}
		if item.Similarity <= 0 {
			item.Similarity = 1
		}
		if item.VATPercent != nil {
			value := clamp(*item.VATPercent, 0, 100)
			item.VATPercent = &value
		}
		item.Similarity = clamp(item.Similarity, 0.05, 1)
		out = append(out, item)
	}
	return out
}

func buildManualAnalogResults(items []ManualEntry, selected cteRecord, referenceDate time.Time, req CalculateRequest, selectedVAT VATInfo) []AnalogResult {
	results := make([]AnalogResult, 0, len(items))
	for index, item := range items {
		label := item.Label
		if label == "" {
			label = fmt.Sprintf("Ручная позиция %d", index+1)
		}
		region := item.Region
		if region == "" {
			if req.Region != "" {
				region = req.Region
			} else {
				region = "любой регион"
			}
		}
		supplierRegion := item.SupplierRegion
		if supplierRegion == "" {
			supplierRegion = region
		}
		regionTier := classifyRegionTier(req.Region, region, supplierRegion)
		regionWeight := regionWeightForTier(req, regionTier)
		manualVAT := manualVATInfo(selectedVAT, item.VATPercent)
		result := AnalogResult{
			ID:                -1 - int64(index),
			ContractID:        0,
			CTEID:             selected.ID,
			CTEName:           label,
			Manufacturer:      selected.Manufacturer,
			Region:            displayRegionName(region),
			SupplierRegion:    displayRegionName(supplierRegion),
			RegionTier:        tierLabel(regionTier),
			UnitPrice:         item.UnitPrice,
			UnitPriceNoVAT:    amountWithoutVAT(item.UnitPrice, manualVAT),
			OriginalUnitPrice: item.UnitPrice,
			Quantity:          1,
			Method:            "добавлено пользователем",
			ContractDate:      referenceDate,
			Similarity:        item.Similarity,
			TimeWeight:        1,
			RegionWeight:      regionWeight,
			WeightMultiplier:  item.WeightMultiplier,
			FinalWeight:       item.Similarity * regionWeight * item.WeightMultiplier,
			Manual:            true,
			PriceOverridden:   true,
			SourceLabel:       "добавлено пользователем",
			VAT:               manualVAT,
		}
		results = append(results, result)
	}
	return results
}

func monthsDiff(ref, other time.Time) float64 {
	days := ref.Sub(other).Hours() / 24
	return math.Max(days/30.0, 0)
}

func iqrBounds(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	q1 := quantile(sorted, 0.25)
	q3 := quantile(sorted, 0.75)
	iqr := q3 - q1
	if iqr == 0 {
		return sorted[0], sorted[len(sorted)-1]
	}
	return q1 - 1.5*iqr, q3 + 1.5*iqr
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := q * float64(len(sorted)-1)
	low := int(math.Floor(pos))
	high := int(math.Ceil(pos))
	if low == high {
		return sorted[low]
	}
	weight := pos - float64(low)
	return sorted[low]*(1-weight) + sorted[high]*weight
}

func weightedMean(items []AnalogResult) float64 {
	totalWeight := 0.0
	totalValue := 0.0
	for _, item := range items {
		totalWeight += item.FinalWeight
		totalValue += item.UnitPrice * item.FinalWeight
	}
	if totalWeight == 0 {
		return 0
	}
	return totalValue / totalWeight
}

func weightedMeanNoVAT(items []AnalogResult) float64 {
	totalWeight := 0.0
	totalValue := 0.0
	for _, item := range items {
		totalWeight += item.FinalWeight
		totalValue += item.UnitPriceNoVAT * item.FinalWeight
	}
	if totalWeight == 0 {
		return 0
	}
	return totalValue / totalWeight
}

func weightedQuantile(items []AnalogResult, q float64) float64 {
	if len(items) == 0 {
		return 0
	}
	sorted := append([]AnalogResult(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UnitPrice < sorted[j].UnitPrice
	})
	totalWeight := 0.0
	for _, item := range sorted {
		totalWeight += item.FinalWeight
	}
	if totalWeight == 0 {
		return sorted[len(sorted)/2].UnitPrice
	}
	target := totalWeight * q
	acc := 0.0
	for _, item := range sorted {
		acc += item.FinalWeight
		if acc >= target {
			return item.UnitPrice
		}
	}
	return sorted[len(sorted)-1].UnitPrice
}

func weightedQuantileNoVAT(items []AnalogResult, q float64) float64 {
	if len(items) == 0 {
		return 0
	}
	sorted := append([]AnalogResult(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UnitPriceNoVAT < sorted[j].UnitPriceNoVAT
	})
	totalWeight := 0.0
	for _, item := range sorted {
		totalWeight += item.FinalWeight
	}
	if totalWeight == 0 {
		return sorted[len(sorted)/2].UnitPriceNoVAT
	}
	target := totalWeight * q
	acc := 0.0
	for _, item := range sorted {
		acc += item.FinalWeight
		if acc >= target {
			return item.UnitPriceNoVAT
		}
	}
	return sorted[len(sorted)-1].UnitPriceNoVAT
}

func topRecommendations(items []AnalogResult) []Recommendation {
	type agg struct {
		Name               string
		Manufacturer       string
		Region             string
		Similarity         float64
		Weight             float64
		LatestPrice        float64
		LatestPriceNoVAT   float64
		WeightedTotal      float64
		WeightedTotalNoVAT float64
		Count              int
		LatestDate         time.Time
		VAT                VATInfo
	}
	byCTE := make(map[int64]*agg)
	for _, item := range items {
		current, ok := byCTE[item.CTEID]
		if !ok {
			current = &agg{
				Name:             item.CTEName,
				Manufacturer:     item.Manufacturer,
				Region:           item.Region,
				Similarity:       item.Similarity,
				LatestPrice:      item.UnitPrice,
				LatestPriceNoVAT: item.UnitPriceNoVAT,
				LatestDate:       item.ContractDate,
				VAT:              item.VAT,
			}
			byCTE[item.CTEID] = current
		}
		if item.ContractDate.After(current.LatestDate) {
			current.LatestDate = item.ContractDate
			current.LatestPrice = item.UnitPrice
			current.LatestPriceNoVAT = item.UnitPriceNoVAT
			current.Region = item.Region
			current.VAT = item.VAT
		}
		current.Weight += item.FinalWeight
		current.WeightedTotal += item.FinalWeight * item.UnitPrice
		current.WeightedTotalNoVAT += item.FinalWeight * item.UnitPriceNoVAT
		current.Count++
		current.Similarity = math.Max(current.Similarity, item.Similarity)
	}

	recs := make([]Recommendation, 0, len(byCTE))
	for cteID, item := range byCTE {
		weightedPrice := 0.0
		weightedPriceNoVAT := 0.0
		if item.Weight > 0 {
			weightedPrice = item.WeightedTotal / item.Weight
			weightedPriceNoVAT = item.WeightedTotalNoVAT / item.Weight
		}
		recs = append(recs, Recommendation{
			CTEID:              cteID,
			Name:               item.Name,
			Manufacturer:       item.Manufacturer,
			Region:             item.Region,
			LatestPrice:        item.LatestPrice,
			LatestPriceNoVAT:   item.LatestPriceNoVAT,
			WeightedPrice:      weightedPrice,
			WeightedPriceNoVAT: weightedPriceNoVAT,
			ContractCount:      item.Count,
			Similarity:         item.Similarity,
			VAT:                item.VAT,
		})
	}
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Similarity == recs[j].Similarity {
			return recs[i].ContractCount > recs[j].ContractCount
		}
		return recs[i].Similarity > recs[j].Similarity
	})
	if len(recs) > 3 {
		recs = recs[:3]
	}
	return recs
}

func selectDocumentAnalogs(items []AnalogResult) []AnalogResult {
	if len(items) == 0 {
		return nil
	}
	if len(items) <= 3 {
		return append([]AnalogResult(nil), items...)
	}

	sorted := append([]AnalogResult(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].FinalWeight == sorted[j].FinalWeight {
			if sorted[i].Similarity == sorted[j].Similarity {
				return sorted[i].ContractDate.After(sorted[j].ContractDate)
			}
			return sorted[i].Similarity > sorted[j].Similarity
		}
		return sorted[i].FinalWeight > sorted[j].FinalWeight
	})

	totalWeight := 0.0
	for _, item := range sorted {
		totalWeight += math.Max(item.FinalWeight, 0)
	}
	targetCoverage := totalWeight * 0.82
	selected := make([]AnalogResult, 0, minInt(len(sorted), 10))
	selectedKeys := map[string]struct{}{}
	usedContracts := map[int64]struct{}{}
	perCTE := map[int64]int{}
	coveredWeight := 0.0

	canTake := func(item AnalogResult, strict bool) bool {
		key := documentAnalogKey(item)
		if _, exists := selectedKeys[key]; exists {
			return false
		}
		if item.ContractID > 0 {
			if _, exists := usedContracts[item.ContractID]; exists {
				return false
			}
		}
		if strict && item.CTEID > 0 && perCTE[item.CTEID] >= 2 {
			return false
		}
		return true
	}

	appendItem := func(item AnalogResult) {
		selected = append(selected, item)
		selectedKeys[documentAnalogKey(item)] = struct{}{}
		if item.ContractID > 0 {
			usedContracts[item.ContractID] = struct{}{}
		}
		if item.CTEID > 0 {
			perCTE[item.CTEID]++
		}
		coveredWeight += math.Max(item.FinalWeight, 0)
	}

	for _, item := range sorted {
		if len(selected) >= 10 {
			break
		}
		if !canTake(item, true) {
			continue
		}
		appendItem(item)
		if len(selected) >= 3 && (coveredWeight >= targetCoverage || len(selected) >= 8) {
			break
		}
	}

	if len(selected) < 3 {
		for _, item := range sorted {
			if len(selected) >= 3 {
				break
			}
			if !canTake(item, false) {
				continue
			}
			appendItem(item)
		}
	}

	if coveredWeight < targetCoverage && len(selected) < 10 {
		for _, item := range sorted {
			if len(selected) >= 10 || coveredWeight >= targetCoverage {
				break
			}
			if !canTake(item, false) {
				continue
			}
			appendItem(item)
		}
	}

	if len(selected) > 10 {
		selected = selected[:10]
	}
	return selected
}

func documentAnalogKey(item AnalogResult) string {
	if item.ContractID > 0 {
		return fmt.Sprintf("contract:%d", item.ContractID)
	}
	return fmt.Sprintf("row:%d", item.ID)
}
