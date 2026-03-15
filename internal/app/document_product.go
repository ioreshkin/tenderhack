package app

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"
)

type documentSourceCell struct {
	Price   string
	Caption string
}

type calculationTableRow struct {
	Number           int
	Name             string
	Manufacturer     string
	Terms            string
	Unit             string
	Quantity         string
	VATLabel         string
	Sources          []documentSourceCell
	AveragePrice     string
	StdDev           string
	Variation        string
	NMCKUnitWithVAT  string
	UnitPriceNoVAT   string
	LineTotalWithVAT string
	LineTotalNoVAT   string
}

type summaryField struct {
	Label string
	Value string
}

type calculationDocumentView struct {
	Title         string
	GeneratedDate string
	Lead          string
	SummaryLeft   []summaryField
	SummaryRight  []summaryField
	HowRows       []summaryField
	Rows          []calculationTableRow
	TotalWithVAT  string
	TotalNoVAT    string
	Footnote      string
}

func buildSingleDocumentHTML(result CalculateResponse) (string, error) {
	view := calculationDocumentView{
		Title:         "Обоснование начальной (максимальной) цены контракта",
		GeneratedDate: formatRussianDate(time.Now()),
		Lead:          "Основной лист расчета: одна таблица, краткая сводка и короткое объяснение формул без лишней простыни текста.",
		SummaryLeft: []summaryField{
			{Label: "Наименование предмета контракта", Value: fallbackText(result.Selected.Name, "не указано")},
			{Label: "Производитель", Value: fallbackText(result.Selected.Manufacturer, "не указан")},
			{Label: "Код СТЕ", Value: fmt.Sprintf("%d", result.Selected.ID)},
			{Label: "Существенные условия исполнения контракта", Value: defaultTerms(result.Selected)},
		},
		SummaryRight: []summaryField{
			{Label: "Регион закупки", Value: fallbackText(result.Parameters.Region, "все регионы")},
			{Label: "Период поиска цен", Value: calculationPeriodLabel(result.Summary.ReferenceDate, result.Summary.WindowHint, result.Parameters.MonthsBack)},
			{Label: "Ставка НДС", Value: vatSummaryLabel(result.Summary.VAT)},
			{Label: "Режим расчета", Value: compactSettingsSummary(result.Parameters)},
		},
		HowRows: []summaryField{
			{Label: "Что берем в расчет", Value: "Берем закупки по выбранной позиции за заданный период, затем оставляем только релевантные строки без явных выбросов. В таблице показываем до 3 самых сильных источников цены."},
			{Label: "Почему одни закупки важнее других", Value: "Для каждой закупки считаем итоговый вес: похожесть товара × свежесть цены × близость региона × ручная поправка. Чем закупка ближе к вашей ситуации, тем больше она влияет на результат."},
			{Label: "Как получается НМЦК", Value: "НМЦК за единицу считаем как взвешенное среднее по очищенной выборке: цена каждой закупки умножается на ее вес, затем сумма делится на сумму весов."},
			{Label: "Как считаем без НДС", Value: "Цены в исходных закупках уже идут с НДС. Ставку НДС берем из карточки товара или самой закупки, после чего отдельно показываем цену без НДС."},
			{Label: "Почему итогу можно доверять", Value: "Дополнительно показываем среднюю цену, стандартное отклонение и коэффициент вариации. Это помогает увидеть, насколько однородны выбранные источники."},
		},
		Rows:         []calculationTableRow{buildCalculationRow(1, result.Selected, result.Summary, result.DocumentResults, 1)},
		TotalWithVAT: formatMoneyValue(result.Summary.NMCKWeightedMean),
		TotalNoVAT:   formatMoneyValue(result.Summary.NMCKWeightedMeanNoVAT),
		Footnote:     compactDocumentFootnote(result.Parameters, result.Summary.WindowHint),
	}
	return executeCalculationTemplate(view)
}

func buildBatchDocumentHTML(result BatchCalculateResponse) (string, error) {
	rows := make([]calculationTableRow, 0, len(result.Items))
	for i, item := range result.Items {
		rows = append(rows, buildCalculationRow(i+1, item.Result.Selected, item.Result.Summary, item.Result.DocumentResults, item.Quantity))
	}
	regionLabel := summarizeBatchRegions(result)
	monthsLabel := summarizeBatchPeriods(result)
	view := calculationDocumentView{
		Title:         "Обоснование начальной (максимальной) цены контракта по лоту",
		GeneratedDate: formatRussianDate(time.Now()),
		Lead:          "Расчет по лоту собран в одном основном листе. Источники цен, итоговые цены и подписи находятся в одном месте и читаются без перехода по страницам.",
		SummaryLeft: []summaryField{
			{Label: "Наименование лота", Value: fallbackText(result.Summary.BatchName, "Лот НМЦК")},
			{Label: "Количество позиций", Value: fmt.Sprintf("%d", result.Summary.ItemCount)},
			{Label: "Источники в документе", Value: fmt.Sprintf("%d закупок", result.Summary.TotalDocumentContracts)},
			{Label: "Существенные условия исполнения контракта", Value: "В соответствии с техническим заданием и выбранными характеристиками"},
		},
		SummaryRight: []summaryField{
			{Label: "Регион закупки", Value: regionLabel},
			{Label: "Период поиска цен", Value: monthsLabel},
			{Label: "Итого по лоту с НДС", Value: formatMoneyValue(result.Summary.GrandTotal)},
			{Label: "Итого по лоту без НДС", Value: formatMoneyValue(result.Summary.GrandTotalNoVAT)},
		},
		HowRows: []summaryField{
			{Label: "Что берем в расчет", Value: "Каждую позицию лота считаем отдельно: подбираем релевантные закупки, очищаем выборку и переносим в документ до 3 самых сильных ценовых источников на позицию."},
			{Label: "Почему одни закупки важнее других", Value: "Для каждой закупки считаем итоговый вес: похожесть товара × свежесть цены × близость региона × ручная поправка. Этот вес определяет вклад строки в цену позиции."},
			{Label: "Как получается цена позиции", Value: "НМЦК по каждой позиции считаем как взвешенное среднее по очищенной выборке закупок. Потом умножаем цену позиции на количество и складываем итог по лоту."},
			{Label: "Как считаем без НДС", Value: "Цены из исходных закупок уже включают НДС. Ставку НДС берем из карточки товара или закупки, после чего отдельно считаем цену и итог без НДС."},
			{Label: "Почему итог по лоту корректен", Value: "В документе показаны средняя цена, стандартное отклонение и коэффициент вариации по отображаемым источникам, чтобы можно было быстро проверить однородность цен."},
		},
		Rows:         rows,
		TotalWithVAT: formatMoneyValue(result.Summary.GrandTotal),
		TotalNoVAT:   formatMoneyValue(result.Summary.GrandTotalNoVAT),
		Footnote:     batchDocumentFootnote(result),
	}
	return executeCalculationTemplate(view)
}

func executeCalculationTemplate(view calculationDocumentView) (string, error) {
	tpl := template.Must(template.New("calculation-doc").Funcs(documentTemplateFuncs()).Parse(calculationPDFTemplate))
	var htmlBuilder strings.Builder
	if err := tpl.Execute(&htmlBuilder, view); err != nil {
		return "", err
	}
	return htmlBuilder.String(), nil
}

func summarizeBatchRegions(result BatchCalculateResponse) string {
	if len(result.Items) == 0 {
		return fallbackText(result.Parameters.Region, "все регионы")
	}
	first := strings.TrimSpace(result.Items[0].Result.Parameters.Region)
	for _, item := range result.Items[1:] {
		if strings.TrimSpace(item.Result.Parameters.Region) != first {
			return "по настройкам лотов"
		}
	}
	return fallbackText(first, "все регионы")
}

func summarizeBatchPeriods(result BatchCalculateResponse) string {
	if len(result.Items) == 0 {
		return calculationPeriodLabel("", WindowHint{}, result.Parameters.MonthsBack)
	}
	hasMarketItems := false
	var earliest time.Time
	var latest time.Time
	for _, item := range result.Items {
		start, end, ok := calculationPeriodBounds(item.Result.Summary.ReferenceDate, item.Result.Summary.WindowHint, item.Result.Parameters.MonthsBack)
		if !ok {
			continue
		}
		if !hasMarketItems || start.Before(earliest) {
			earliest = start
		}
		if !hasMarketItems || end.After(latest) {
			latest = end
		}
		hasMarketItems = true
	}
	if !hasMarketItems {
		return "данные пользователя"
	}
	if sameDay(earliest, latest) {
		return formatShortDate(earliest)
	}
	return fmt.Sprintf("%s — %s", formatShortDate(earliest), formatShortDate(latest))
}

func buildCalculationRow(index int, selected SelectedCTE, summary SummaryBlock, sources []AnalogResult, quantity float64) calculationTableRow {
	if quantity <= 0 {
		quantity = 1
	}
	displayedSources := buildDisplayedSources(sources, 3)
	avg, stddev, variation := sourceStats(sources, 3)
	return calculationTableRow{
		Number:           index,
		Name:             fallbackText(selected.Name, "не указано"),
		Manufacturer:     fallbackText(selected.Manufacturer, "не указан"),
		Terms:            defaultTerms(selected),
		Unit:             "ед.",
		Quantity:         formatIntegerQuantity(quantity),
		VATLabel:         vatSummaryLabel(summary.VAT),
		Sources:          displayedSources,
		AveragePrice:     formatMoneyValue(avg),
		StdDev:           formatMoneyValue(stddev),
		Variation:        formatVariation(variation),
		NMCKUnitWithVAT:  formatMoneyValue(summary.NMCKWeightedMean),
		UnitPriceNoVAT:   formatMoneyValue(summary.NMCKWeightedMeanNoVAT),
		LineTotalWithVAT: formatMoneyValue(summary.NMCKWeightedMean * quantity),
		LineTotalNoVAT:   formatMoneyValue(summary.NMCKWeightedMeanNoVAT * quantity),
	}
}

func buildDisplayedSources(items []AnalogResult, limit int) []documentSourceCell {
	cells := make([]documentSourceCell, 0, limit)
	for i := 0; i < limit; i++ {
		if i >= len(items) {
			cells = append(cells, documentSourceCell{Price: "-", Caption: ""})
			continue
		}
		item := items[i]
		region := fallbackText(item.Region, item.SupplierRegion)
		caption := make([]string, 0, 3)
		if item.Manual {
			caption = append(caption, "добавлено пользователем")
		} else {
			caption = append(caption, item.ContractDate.Format("02.01.2006"))
		}
		if region != "" {
			caption = append(caption, region)
		}
		cells = append(cells, documentSourceCell{
			Price:   formatMoneyValue(item.UnitPrice),
			Caption: strings.Join(caption, "\n"),
		})
	}
	return cells
}

func sourceStats(items []AnalogResult, limit int) (float64, float64, float64) {
	if len(items) == 0 || limit <= 0 {
		return 0, 0, 0
	}
	count := minInt(len(items), limit)
	prices := make([]float64, 0, count)
	for _, item := range items[:count] {
		if item.UnitPrice > 0 {
			prices = append(prices, item.UnitPrice)
		}
	}
	if len(prices) == 0 {
		return 0, 0, 0
	}
	mean := 0.0
	for _, price := range prices {
		mean += price
	}
	mean /= float64(len(prices))
	if len(prices) == 1 || mean == 0 {
		return mean, 0, 0
	}
	varianceSum := 0.0
	for _, price := range prices {
		delta := price - mean
		varianceSum += delta * delta
	}
	stddev := math.Sqrt(varianceSum / float64(len(prices)-1))
	variation := (stddev / mean) * 100
	return mean, stddev, variation
}

func compactDocumentFootnote(params EffectiveParams, hint WindowHint) string {
	parts := []string{
		"Расчет выполнен методом сопоставимых рыночных цен по очищенной выборке релевантных закупок.",
		"В колонках Источник №1-№3 показаны закупки, которые лучше всего объясняют итоговую цену в документе.",
		fmt.Sprintf("Поиск цен начинается с 1 месяца и автоматически расширяется по 1 месяцу, пока не найдено хотя бы 3 закупки, но не более чем до 12 месяцев. Итоговая цена за единицу считается как взвешенное среднее. Режим учета давности: %s.", timeWeightModeLabel(params.TimeWeightMode)),
	}
	if hint.ExpandedAutomatically && hint.AppliedMonthsBack > hint.RequestedMonthsBack {
		parts = append(parts, fmt.Sprintf("В исходном окне %d мес. найдено %d закупок, поэтому окно автоматически расширено до %d мес., где найдено %d закупок.", hint.RequestedMonthsBack, hint.RequestedCount, hint.AppliedMonthsBack, hint.AppliedCount))
	} else if hint.NeedsExpansion {
		parts = append(parts, fmt.Sprintf("Даже после авторасширения окно осталось узким: в исходном окне %d мес. найдено %d закупок, в рабочем окне %d мес. найдено %d закупок.", hint.RequestedMonthsBack, hint.RequestedCount, hint.AppliedMonthsBack, hint.AppliedCount))
	}
	parts = append(parts, "Коэффициент вариации показан по отображаемым источникам. Цена без НДС рассчитывается из цены с НДС и ставки НДС.")
	return strings.Join(parts, " ")
}

func batchDocumentFootnote(result BatchCalculateResponse) string {
	if len(result.Items) == 0 {
		return compactDocumentFootnote(result.Parameters, WindowHint{})
	}
	onlyManual := true
	for _, item := range result.Items {
		if item.Result.Selected.ID > 0 {
			onlyManual = false
			break
		}
	}
	if onlyManual {
		return "Документ сформирован только по ценам, которые добавил пользователь. Рыночные закупки для этих позиций не использовались. Цена без НДС рассчитывается из введенной цены и указанной ставки НДС."
	}
	return compactDocumentFootnote(result.Parameters, WindowHint{})
}

func compactSettingsSummary(params EffectiveParams) string {
	if params.SettingsMode == "advanced" {
		return fmt.Sprintf("расширенный: свежесть %.2f, свой регион %.2f, другие регионы %.2f", params.TimeDecay, params.SameRegionWeight, params.OtherRegionWeight)
	}
	return fmt.Sprintf("простой: свежесть — %s, свой регион — %s, другие регионы — %s", fallbackText(params.TimeImportanceLabel, "важно"), fallbackText(params.SameRegionImportanceLabel, "очень важно"), fallbackText(params.OtherRegionImportanceLabel, "средне"))
}

func defaultTerms(selected SelectedCTE) string {
	parts := []string{"В соответствии с техническим заданием"}
	if selected.Category != "" {
		parts = append(parts, selected.Category)
	}
	if len(selected.Attributes) > 0 {
		parts = append(parts, "и выбранными характеристиками")
	}
	return strings.Join(parts, ", ")
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func formatIntegerQuantity(value float64) string {
	return fmt.Sprintf("%.0f", math.Round(value))
}

func formatVariation(value float64) string {
	if value <= 0 {
		return "0.00%"
	}
	return fmt.Sprintf("%.2f%%", value)
}

func formatRussianDate(t time.Time) string {
	months := []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	month := ""
	if m := int(t.Month()); m >= 1 && m <= 12 {
		month = months[m-1]
	}
	return fmt.Sprintf("%d %s %d", t.Day(), month, t.Year())
}

func calculationPeriodLabel(referenceDate string, hint WindowHint, fallbackMonths int) string {
	start, end, ok := calculationPeriodBounds(referenceDate, hint, fallbackMonths)
	if !ok {
		return "данные пользователя"
	}
	if sameDay(start, end) {
		return formatShortDate(end)
	}
	return fmt.Sprintf("%s — %s", formatShortDate(start), formatShortDate(end))
}

func calculationPeriodBounds(referenceDate string, hint WindowHint, fallbackMonths int) (time.Time, time.Time, bool) {
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(referenceDate))
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	months := hint.AppliedMonthsBack
	if months <= 0 {
		months = fallbackMonths
	}
	if months <= 0 {
		return time.Time{}, time.Time{}, false
	}
	start := end.AddDate(0, -months, 0)
	return start, end, true
}

func formatShortDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

const productDocumentExtraStyles = `
  <style>
    @page { size: A4 landscape; margin: 10mm; }
    body { font-size: 9px; }
    .doc-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
    .doc-date { min-width: 190px; text-align: right; font-size: 10px; color: var(--muted); }
    .lead { color: var(--muted); font-size: 10px; }
    .calc-table { font-size: 8.2px; }
    .calc-table th { text-align: center; vertical-align: middle; }
    .calc-table td { padding: 4px 5px; }
    .calc-table .source-cell { min-width: 86px; white-space: pre-line; text-align: center; }
    .calc-table .source-price { display: block; font-weight: 700; }
    .calc-table .source-caption { display: block; margin-top: 2px; color: var(--muted); font-size: 7.5px; line-height: 1.2; }
    .calc-table .th-main { display: block; }
    .calc-table .th-formula { display: block; margin-top: 3px; color: var(--muted); font-size: 7px; line-height: 1.2; font-weight: 500; text-transform: none; letter-spacing: 0; }
    .calc-table .formula-math { font-family: "Cambria Math", "Times New Roman", serif; font-size: 7.4px; }
    .calc-table .narrow { width: 34px; }
    .calc-table .qty { width: 44px; }
    .calc-table .vat { width: 60px; }
    .calc-table .money-col { width: 84px; }
    .calc-table .meta-col { width: 72px; }
    .summary-strip { display: grid; grid-template-columns: 1.3fr 1fr; gap: 10px; }
    .appendix { page-break-before: always; page-break-inside: avoid; display: grid; gap: 10px; }
    .signature-block { margin-top: 10px; width: 48%; display: grid; gap: 8px; page-break-inside: avoid; }
    .signature-title { font-weight: 700; font-size: 11px; }
    .signature-row { display: grid; grid-template-columns: 170px 1fr; gap: 10px; align-items: end; }
    .signature-line { min-height: 18px; border-bottom: 1px solid var(--ink); }
    .footnote { font-size: 8.2px; line-height: 1.35; color: var(--ink); }
    .how-table td:first-child { width: 180px; font-weight: 700; background: var(--soft); }
  </style>
`

const calculationPDFTemplate = `
<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <title>{{.Title}}</title>
` + documentBaseStyles + productDocumentExtraStyles + `
</head>
<body>
  <div class="doc">
    <div class="doc-head">
      <div class="title">
        <span class="label">Расчет</span>
        <h1>{{.Title}}</h1>
        <p class="lead">{{.Lead}}</p>
      </div>
      <div class="doc-date">
        <div>Дата формирования</div>
        <strong>{{.GeneratedDate}}</strong>
      </div>
    </div>

    <div class="summary-strip">
      <div class="meta-box">
        <table>
          <tbody>
            {{range .SummaryLeft}}
            <tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>
            {{end}}
          </tbody>
        </table>
      </div>
      <div class="meta-box">
        <table>
          <tbody>
            {{range .SummaryRight}}
            <tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>

    <table class="calc-table">
      <thead>
        <tr>
          <th class="narrow">№</th>
          <th>Наименование предмета контракта</th>
          <th>Производитель</th>
          <th>Существенные условия исполнения контракта</th>
          <th class="center">Ед. изм</th>
          <th class="center qty">Кол-во</th>
          <th class="center vat">Ставка НДС</th>
          <th>Источник №1</th>
          <th>Источник №2</th>
          <th>Источник №3</th>
          <th class="right money-col"><span class="th-main">Средняя арифметическая цена за единицу</span><span class="th-formula formula-math">&lang;u&rang; = (u<sub>1</sub> + u<sub>2</sub> + u<sub>3</sub>) / n</span></th>
          <th class="right meta-col"><span class="th-main">Среднее квадратичное отклонение</span><span class="th-formula formula-math">&sigma; = &radic;( &Sigma;(u<sub>i</sub> - &lang;u&rang;)<sup>2</sup> / (n - 1) )</span></th>
          <th class="right meta-col"><span class="th-main">Коэффициент вариации V (%)</span><span class="th-formula formula-math">V = &sigma; / &lang;u&rang; &times; 100</span></th>
          <th class="right money-col"><span class="th-main">Расчет Н(М)ЦК по формуле</span><span class="th-formula formula-math">НМЦК = &Sigma;(u<sub>i</sub> &times; w<sub>i</sub>) / &Sigma;w<sub>i</sub></span></th>
          <th class="right money-col"><span class="th-main">Цена за единицу без НДС</span><span class="th-formula formula-math">u<sub>без НДС</sub> = u / (1 + НДС)</span></th>
          <th class="right money-col">НМЦК позиции с НДС</th>
          <th class="right money-col">НМЦК позиции без НДС</th>
        </tr>
      </thead>
      <tbody>
        {{range .Rows}}
        <tr>
          <td class="center">{{.Number}}</td>
          <td>{{.Name}}</td>
          <td>{{.Manufacturer}}</td>
          <td>{{.Terms}}</td>
          <td class="center">{{.Unit}}</td>
          <td class="center">{{.Quantity}}</td>
          <td class="center">{{.VATLabel}}</td>
          {{range .Sources}}
          <td class="source-cell">
            <span class="source-price">{{.Price}}</span>
            {{if .Caption}}<span class="source-caption">{{.Caption}}</span>{{end}}
          </td>
          {{end}}
          <td class="right">{{.AveragePrice}}</td>
          <td class="right">{{.StdDev}}</td>
          <td class="right">{{.Variation}}</td>
          <td class="right">{{.NMCKUnitWithVAT}}</td>
          <td class="right">{{.UnitPriceNoVAT}}</td>
          <td class="right">{{.LineTotalWithVAT}}</td>
          <td class="right">{{.LineTotalNoVAT}}</td>
        </tr>
        {{end}}
      </tbody>
      <tfoot>
        <tr>
          <td colspan="15" class="right">Итого</td>
          <td class="right">{{.TotalWithVAT}}</td>
          <td class="right">{{.TotalNoVAT}}</td>
        </tr>
      </tfoot>
    </table>

    <div class="appendix">
      <div class="footnote">* {{.Footnote}}</div>

      <div class="section-title">Как считается итог</div>
      <table class="how-table">
        <tbody>
          {{range .HowRows}}
          <tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>
          {{end}}
        </tbody>
      </table>

      <div class="signature-block">
        <div class="signature-title">Работник контрактной службы</div>
        <div class="signature-row"><span>Дата составления</span><span>{{.GeneratedDate}}</span></div>
        <div class="signature-row"><span>Должность</span><span class="signature-line"></span></div>
        <div class="signature-row"><span>ФИО</span><span class="signature-line"></span></div>
        <div class="signature-row"><span>Подпись</span><span class="signature-line"></span></div>
        <div class="signature-row"><span>Контактный телефон</span><span class="signature-line"></span></div>
      </div>
    </div>
  </div>
</body>
</html>
`
