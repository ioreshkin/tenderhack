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

func CalculateBatchNMCK(ctx context.Context, pool *pgxpool.Pool, req BatchCalculateRequest) (BatchCalculateResponse, error) {
	req = normalizeBatchRequest(req)
	if len(req.Items) == 0 {
		return BatchCalculateResponse{}, fmt.Errorf("batch items are empty")
	}

	referenceDate, err := latestContractDate(ctx, pool)
	if err != nil {
		return BatchCalculateResponse{}, err
	}

	items := make([]BatchItemResult, 0, len(req.Items))
	warnings := make([]string, 0, len(req.Items))
	grandTotal := 0.0
	grandTotalNoVAT := 0.0
	totalDocumentContracts := 0

	for index, item := range req.Items {
		var result CalculateResponse
		if item.CTEID > 0 {
			singleReq := buildSingleRequestFromBatch(req, item)
			result, err = CalculateNMCK(ctx, pool, singleReq)
			if err != nil {
				return BatchCalculateResponse{}, fmt.Errorf("batch item %d (CTE %d): %w", index+1, item.CTEID, err)
			}
		} else {
			result, err = calculateManualBatchItem(referenceDate, req, item)
			if err != nil {
				return BatchCalculateResponse{}, fmt.Errorf("batch item %d (manual): %w", index+1, err)
			}
		}

		itemID := item.ItemID
		if itemID == "" {
			itemID = fmt.Sprintf("item-%d", index+1)
		}
		quantity := item.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		quantity = math.Max(1, math.Round(quantity))
		lineTotal := result.Summary.NMCKWeightedMean * quantity
		lineTotalNoVAT := result.Summary.NMCKWeightedMeanNoVAT * quantity
		grandTotal += lineTotal
		grandTotalNoVAT += lineTotalNoVAT
		totalDocumentContracts += result.Summary.DocumentContracts

		if len(result.Summary.Warnings) > 0 {
			warnings = append(warnings, fmt.Sprintf("%s: %s", result.Selected.Name, strings.Join(result.Summary.Warnings, "; ")))
		}

		items = append(items, BatchItemResult{
			ItemID:         itemID,
			Quantity:       quantity,
			LineTotal:      lineTotal,
			LineTotalNoVAT: lineTotalNoVAT,
			Result:         result,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Result.Selected.Name < items[j].Result.Selected.Name
	})

	return BatchCalculateResponse{
		Summary: BatchSummary{
			BatchName:              req.BatchName,
			ItemCount:              len(items),
			GrandTotal:             grandTotal,
			GrandTotalNoVAT:        grandTotalNoVAT,
			TotalDocumentContracts: totalDocumentContracts,
			Warnings:               warnings,
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
		Items: items,
	}, nil
}

func normalizeBatchRequest(req BatchCalculateRequest) BatchCalculateRequest {
	base := normalizeRequest(CalculateRequest{
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
	})
	req.BatchName = cleanCell(req.BatchName)
	if req.BatchName == "" {
		req.BatchName = "Лот НМЦК"
	}
	req.Region = base.Region
	req.MonthsBack = base.MonthsBack
	req.TimeDecay = base.TimeDecay
	req.TimeWeightMode = base.TimeWeightMode
	req.SameRegionWeight = base.SameRegionWeight
	req.OtherRegionWeight = base.OtherRegionWeight
	req.SettingsMode = base.SettingsMode
	req.TimeImportanceLabel = base.TimeImportanceLabel
	req.SameRegionImportanceLabel = base.SameRegionImportanceLabel
	req.OtherRegionImportanceLabel = base.OtherRegionImportanceLabel
	req.MaxResults = base.MaxResults

	out := make([]BatchItemRequest, 0, len(req.Items))
	for index, item := range req.Items {
		manualOnly := item.CTEID <= 0
		if !manualOnly && item.CTEID <= 0 {
			continue
		}
		item.ItemID = cleanCell(item.ItemID)
		if item.ItemID == "" {
			if manualOnly {
				item.ItemID = fmt.Sprintf("manual-%d", index+1)
			} else {
				item.ItemID = fmt.Sprintf("item-%d", index+1)
			}
		}
		item.Name = cleanCell(item.Name)
		item.Category = cleanCell(item.Category)
		item.Manufacturer = cleanCell(item.Manufacturer)
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
		item.Quantity = math.Max(1, math.Round(item.Quantity))
		item.Region = strings.TrimSpace(item.Region)
		if item.MonthsBack <= 0 {
			item.MonthsBack = req.MonthsBack
		}
		if item.SettingsMode != "advanced" && item.TimeDecay <= 0 {
			item.TimeDecay = req.TimeDecay
		}
		if strings.TrimSpace(item.TimeWeightMode) == "" {
			item.TimeWeightMode = req.TimeWeightMode
		}
		if item.SettingsMode != "advanced" && item.SameRegionWeight <= 0 {
			item.SameRegionWeight = req.SameRegionWeight
		}
		if item.SettingsMode != "advanced" && item.OtherRegionWeight <= 0 {
			item.OtherRegionWeight = req.OtherRegionWeight
		}
		if strings.TrimSpace(item.SettingsMode) == "" {
			item.SettingsMode = req.SettingsMode
		}
		if strings.TrimSpace(item.TimeImportanceLabel) == "" {
			item.TimeImportanceLabel = req.TimeImportanceLabel
		}
		if strings.TrimSpace(item.SameRegionImportanceLabel) == "" {
			item.SameRegionImportanceLabel = req.SameRegionImportanceLabel
		}
		if strings.TrimSpace(item.OtherRegionImportanceLabel) == "" {
			item.OtherRegionImportanceLabel = req.OtherRegionImportanceLabel
		}
		if item.MaxResults <= 0 {
			item.MaxResults = req.MaxResults
		}
		item.PriceOverrides = sanitizePriceOverrides(item.PriceOverrides)
		item.WeightOverrides = sanitizeWeightOverrides(item.WeightOverrides)
		item.ManualEntries = sanitizeManualEntries(item.ManualEntries)
		if manualOnly {
			if item.Name == "" {
				item.Name = fmt.Sprintf("Позиция пользователя %d", index+1)
			}
			if len(item.ManualEntries) == 0 {
				continue
			}
		}
		out = append(out, item)
	}
	req.Items = out
	return req
}

func buildSingleRequestFromBatch(batch BatchCalculateRequest, item BatchItemRequest) CalculateRequest {
	region := item.Region
	if region == "" {
		region = batch.Region
	}
	return normalizeRequest(CalculateRequest{
		CTEID:                      item.CTEID,
		Region:                     region,
		MonthsBack:                 item.MonthsBack,
		TimeDecay:                  item.TimeDecay,
		TimeWeightMode:             item.TimeWeightMode,
		SameRegionWeight:           item.SameRegionWeight,
		OtherRegionWeight:          item.OtherRegionWeight,
		SettingsMode:               item.SettingsMode,
		TimeImportanceLabel:        item.TimeImportanceLabel,
		SameRegionImportanceLabel:  item.SameRegionImportanceLabel,
		OtherRegionImportanceLabel: item.OtherRegionImportanceLabel,
		MaxResults:                 item.MaxResults,
		ExcludedIDs:                append([]int64(nil), item.ExcludedIDs...),
		PriceOverrides:             append([]PriceOverride(nil), item.PriceOverrides...),
		WeightOverrides:            append([]WeightOverride(nil), item.WeightOverrides...),
		ManualEntries:              append([]ManualEntry(nil), item.ManualEntries...),
	})
}

func calculateManualBatchItem(referenceDate time.Time, batch BatchCalculateRequest, item BatchItemRequest) (CalculateResponse, error) {
	req := buildSingleRequestFromBatch(batch, item)
	req.CTEID = 0
	req.ManualEntries = sanitizeManualEntries(item.ManualEntries)
	if len(req.ManualEntries) == 0 {
		return CalculateResponse{}, fmt.Errorf("manual item has no prices")
	}

	selectedVAT := VATInfo{Label: "не указана", Rate: 0, Included: true, Source: "ручной ввод"}
	if first := req.ManualEntries[0]; first.VATPercent != nil {
		selectedVAT = manualVATInfo(selectedVAT, first.VATPercent)
	}
	selected := cteRecord{
		ID:           0,
		Name:         cleanCell(item.Name),
		Category:     cleanCell(item.Category),
		Manufacturer: cleanCell(item.Manufacturer),
	}
	if selected.Name == "" {
		selected.Name = "Позиция пользователя"
	}

	valid := buildManualAnalogResults(req.ManualEntries, selected, referenceDate, req, selectedVAT)
	if len(valid) == 0 {
		return CalculateResponse{}, fmt.Errorf("manual item has no valid prices")
	}

	sort.Slice(valid, func(i, j int) bool {
		if valid[i].FinalWeight == valid[j].FinalWeight {
			return valid[i].CTEName < valid[j].CTEName
		}
		return valid[i].FinalWeight > valid[j].FinalWeight
	})

	rangeMin := weightedQuantile(valid, 0.10)
	rangeMax := weightedQuantile(valid, 0.90)
	rangeMinNoVAT := weightedQuantileNoVAT(valid, 0.10)
	rangeMaxNoVAT := weightedQuantileNoVAT(valid, 0.90)
	weightedMeanValue := weightedMean(valid)
	weightedMedian := weightedQuantile(valid, 0.50)
	weightedMeanNoVAT := weightedMeanNoVAT(valid)
	weightedMedianNoVAT := weightedQuantileNoVAT(valid, 0.50)

	windowHint := WindowHint{
		RequestedMonthsBack: 0,
		RequestedCount:      len(valid),
		AppliedMonthsBack:   0,
		AppliedCount:        len(valid),
		MinRequired:         1,
		NeedsExpansion:      false,
	}
	warnings := []string{
		"Позиция рассчитана только по данным, добавленным пользователем. Рыночные закупки для нее не использовались.",
	}

	return CalculateResponse{
		Selected: SelectedCTE{
			ID:           0,
			Name:         selected.Name,
			Category:     selected.Category,
			Manufacturer: selected.Manufacturer,
			Attributes:   map[string]string{},
			VAT:          selectedVAT,
		},
		Parameters: EffectiveParams{
			Region:                     req.Region,
			MonthsBack:                 0,
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
			RawContracts:            0,
			ValidContracts:          len(valid),
			DocumentContracts:       len(valid),
			ExcludedOutliers:        0,
			ManualExclusions:        0,
			ManualEntries:           len(valid),
			PriceRangeMin:           rangeMin,
			PriceRangeMax:           rangeMax,
			PriceRangeMinNoVAT:      rangeMinNoVAT,
			PriceRangeMaxNoVAT:      rangeMaxNoVAT,
			NMCKWeightedMean:        weightedMeanValue,
			NMCKWeightedMedian:      weightedMedian,
			NMCKWeightedMeanNoVAT:   weightedMeanNoVAT,
			NMCKWeightedMedianNoVAT: weightedMedianNoVAT,
			VAT:                     selectedVAT,
			FallbackToAllRegion:     false,
			RegionScope:             "использованы только данные пользователя",
			ReferenceDate:           referenceDate.Format(time.RFC3339),
			WindowHint:              windowHint,
			Warnings:                warnings,
		},
		TopRecommendations: topRecommendations(valid),
		Results:            valid,
		DocumentResults:    valid,
		Steps: []ExplainStep{
			{
				Title:   "1. Источник цены",
				Details: "Для позиции нет привязки к СТЕ. В расчет взяты только цены, которые пользователь добавил вручную.",
			},
			{
				Title:   "2. НДС и итог",
				Details: "Цена с НДС берется из ручного ввода. Цена без НДС рассчитывается из нее по указанной ставке НДС. Итоговая цена позиции считается как среднее по добавленным пользователем значениям с учетом их веса.",
			},
		},
	}, nil
}

func batchKey(req BatchCalculateRequest) string {
	name := normalizeText(req.BatchName)
	if name == "" {
		name = "lot"
	}
	return strings.ReplaceAll(name, " ", "_")
}
