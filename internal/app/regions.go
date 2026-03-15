package app

import (
	"sort"
	"strings"
)

type regionTier string

const (
	regionTierAll      regionTier = "all"
	regionTierExact    regionTier = "exact"
	regionTierCluster  regionTier = "cluster"
	regionTierDistrict regionTier = "district"
	regionTierOther    regionTier = "other"
)

type regionProfile struct {
	Canonical string
	District  string
	Aliases   []string
	Cluster   []string
}

type regionAlias struct {
	Normalized string
	Canonical  string
}

type regionIndex struct {
	byCanonical map[string]regionProfile
	byExact     map[string]string
	aliases     []regionAlias
}

type regionSelectionInfo struct {
	SelectedCanonical string
	Scope             string
	UsedTiers         []string
	Counts            map[string]int
	FallbackToAll     bool
}

var compiledRegionIndex = newRegionIndex(regionProfiles)

var regionProfiles = []regionProfile{
	{Canonical: "Москва", District: "Центральный федеральный округ", Aliases: []string{"москва", "г москва", "город москва"}, Cluster: []string{"Московская область"}},
	{Canonical: "Московская область", District: "Центральный федеральный округ", Aliases: []string{"московская область", "московская обл"}, Cluster: []string{"Москва"}},
	{Canonical: "Санкт-Петербург", District: "Северо-Западный федеральный округ", Aliases: []string{"санкт петербург", "спб", "г санкт петербург"}, Cluster: []string{"Ленинградская область"}},
	{Canonical: "Ленинградская область", District: "Северо-Западный федеральный округ", Aliases: []string{"ленинградская область", "ленинградская обл"}, Cluster: []string{"Санкт-Петербург"}},
	{Canonical: "Республика Крым", District: "Южный федеральный округ", Aliases: []string{"республика крым", "крым"}, Cluster: []string{"Севастополь"}},
	{Canonical: "Севастополь", District: "Южный федеральный округ", Aliases: []string{"севастополь", "г севастополь"}, Cluster: []string{"Республика Крым"}},
	{Canonical: "Архангельская область", District: "Северо-Западный федеральный округ", Aliases: []string{"архангельская область", "архангельская обл"}, Cluster: []string{"Ненецкий автономный округ"}},
	{Canonical: "Ненецкий автономный округ", District: "Северо-Западный федеральный округ", Aliases: []string{"ненецкий автономный округ", "нао"}, Cluster: []string{"Архангельская область"}},
	{Canonical: "Тюменская область", District: "Уральский федеральный округ", Aliases: []string{"тюменская область", "тюменская обл"}, Cluster: []string{"Ханты-Мансийский автономный округ - Югра", "Ямало-Ненецкий автономный округ"}},
	{Canonical: "Ханты-Мансийский автономный округ - Югра", District: "Уральский федеральный округ", Aliases: []string{"ханты мансийский автономный округ югра", "ханты мансийский автономный округ", "хмао", "хмао югра", "сургут"}, Cluster: []string{"Тюменская область"}},
	{Canonical: "Ямало-Ненецкий автономный округ", District: "Уральский федеральный округ", Aliases: []string{"ямало ненецкий автономный округ", "янао"}, Cluster: []string{"Тюменская область"}},
	{Canonical: "Алтайский край", District: "Сибирский федеральный округ", Aliases: []string{"алтайский край"}},
	{Canonical: "Амурская область", District: "Дальневосточный федеральный округ", Aliases: []string{"амурская область", "амурская"}},
	{Canonical: "Астраханская область", District: "Южный федеральный округ", Aliases: []string{"астраханская область"}},
	{Canonical: "Байконур", District: "Специальная территория", Aliases: []string{"байконур"}},
	{Canonical: "Белгородская область", District: "Центральный федеральный округ", Aliases: []string{"белгородская область"}},
	{Canonical: "Брянская область", District: "Центральный федеральный округ", Aliases: []string{"брянская область"}},
	{Canonical: "Владимирская область", District: "Центральный федеральный округ", Aliases: []string{"владимирская область"}},
	{Canonical: "Волгоградская область", District: "Южный федеральный округ", Aliases: []string{"волгоградская область"}},
	{Canonical: "Вологодская область", District: "Северо-Западный федеральный округ", Aliases: []string{"вологодская область"}},
	{Canonical: "Воронежская область", District: "Центральный федеральный округ", Aliases: []string{"воронежская область"}},
	{Canonical: "Донецкая Народная Республика", District: "Южный федеральный округ", Aliases: []string{"донецкая народная республика", "днр"}},
	{Canonical: "Забайкальский край", District: "Дальневосточный федеральный округ", Aliases: []string{"забайкальский край"}},
	{Canonical: "Запорожская область", District: "Южный федеральный округ", Aliases: []string{"запорожская область"}},
	{Canonical: "Ивановская область", District: "Центральный федеральный округ", Aliases: []string{"ивановская область", "иваново"}},
	{Canonical: "Иркутская область", District: "Сибирский федеральный округ", Aliases: []string{"иркутская область"}},
	{Canonical: "Кабардино-Балкарская Республика", District: "Северо-Кавказский федеральный округ", Aliases: []string{"кабардино балкарская республика"}},
	{Canonical: "Калининградская область", District: "Северо-Западный федеральный округ", Aliases: []string{"калининградская область"}},
	{Canonical: "Калужская область", District: "Центральный федеральный округ", Aliases: []string{"калужская область"}},
	{Canonical: "Карачаево-Черкесская Республика", District: "Северо-Кавказский федеральный округ", Aliases: []string{"карачаево черкесская республика"}},
	{Canonical: "Кемеровская область - Кузбасс", District: "Сибирский федеральный округ", Aliases: []string{"кемеровская область кузбасс", "кемеровская область"}},
	{Canonical: "Кировская область", District: "Приволжский федеральный округ", Aliases: []string{"кировская область"}},
	{Canonical: "Костромская область", District: "Центральный федеральный округ", Aliases: []string{"костромская область"}},
	{Canonical: "Краснодарский край", District: "Южный федеральный округ", Aliases: []string{"краснодарский край", "краснодарский"}},
	{Canonical: "Красноярский край", District: "Сибирский федеральный округ", Aliases: []string{"красноярский край"}},
	{Canonical: "Курганская область", District: "Уральский федеральный округ", Aliases: []string{"курганская область"}},
	{Canonical: "Курская область", District: "Центральный федеральный округ", Aliases: []string{"курская область"}},
	{Canonical: "Липецкая область", District: "Центральный федеральный округ", Aliases: []string{"липецкая область"}},
	{Canonical: "Луганская Народная Республика", District: "Южный федеральный округ", Aliases: []string{"луганская народная республика", "лнр"}},
	{Canonical: "Магаданская область", District: "Дальневосточный федеральный округ", Aliases: []string{"магаданская область"}},
	{Canonical: "Мурманская область", District: "Северо-Западный федеральный округ", Aliases: []string{"мурманская область"}},
	{Canonical: "Нижегородская область", District: "Приволжский федеральный округ", Aliases: []string{"нижегородская область"}},
	{Canonical: "Новгородская область", District: "Северо-Западный федеральный округ", Aliases: []string{"новгородская область"}},
	{Canonical: "Новосибирская область", District: "Сибирский федеральный округ", Aliases: []string{"новосибирская область"}},
	{Canonical: "Омская область", District: "Сибирский федеральный округ", Aliases: []string{"омская область", "омск"}},
	{Canonical: "Оренбургская область", District: "Приволжский федеральный округ", Aliases: []string{"оренбургская область"}},
	{Canonical: "Орловская область", District: "Центральный федеральный округ", Aliases: []string{"орловская область"}},
	{Canonical: "Пензенская область", District: "Приволжский федеральный округ", Aliases: []string{"пензенская область"}},
	{Canonical: "Пермский край", District: "Приволжский федеральный округ", Aliases: []string{"пермский край"}},
	{Canonical: "Приморский край", District: "Дальневосточный федеральный округ", Aliases: []string{"приморский край"}},
	{Canonical: "Псковская область", District: "Северо-Западный федеральный округ", Aliases: []string{"псковская область"}},
	{Canonical: "Республика Адыгея", District: "Южный федеральный округ", Aliases: []string{"республика адыгея", "адыгея"}},
	{Canonical: "Республика Алтай", District: "Сибирский федеральный округ", Aliases: []string{"республика алтай"}},
	{Canonical: "Республика Башкортостан", District: "Приволжский федеральный округ", Aliases: []string{"республика башкортостан", "башкортостан"}},
	{Canonical: "Республика Бурятия", District: "Дальневосточный федеральный округ", Aliases: []string{"республика бурятия", "бурятия"}},
	{Canonical: "Республика Дагестан", District: "Северо-Кавказский федеральный округ", Aliases: []string{"республика дагестан", "дагестан"}},
	{Canonical: "Республика Ингушетия", District: "Северо-Кавказский федеральный округ", Aliases: []string{"республика ингушетия", "ингушетия"}},
	{Canonical: "Республика Калмыкия", District: "Южный федеральный округ", Aliases: []string{"республика калмыкия", "калмыкия"}},
	{Canonical: "Республика Карелия", District: "Северо-Западный федеральный округ", Aliases: []string{"республика карелия", "карелия"}},
	{Canonical: "Республика Коми", District: "Северо-Западный федеральный округ", Aliases: []string{"республика коми", "коми"}},
	{Canonical: "Республика Марий Эл", District: "Приволжский федеральный округ", Aliases: []string{"республика марий эл", "марий эл"}},
	{Canonical: "Республика Мордовия", District: "Приволжский федеральный округ", Aliases: []string{"республика мордовия", "мордовия"}},
	{Canonical: "Республика Саха (Якутия)", District: "Дальневосточный федеральный округ", Aliases: []string{"республика саха якутия", "якутия", "саха якутия"}},
	{Canonical: "Республика Северная Осетия - Алания", District: "Северо-Кавказский федеральный округ", Aliases: []string{"республика северная осетия алания", "северная осетия алания"}},
	{Canonical: "Республика Татарстан", District: "Приволжский федеральный округ", Aliases: []string{"республика татарстан", "татарстан"}},
	{Canonical: "Республика Тыва", District: "Сибирский федеральный округ", Aliases: []string{"республика тыва", "тыва"}},
	{Canonical: "Республика Хакасия", District: "Сибирский федеральный округ", Aliases: []string{"республика хакасия", "хакасия"}},
	{Canonical: "Ростовская область", District: "Южный федеральный округ", Aliases: []string{"ростовская область"}},
	{Canonical: "Рязанская область", District: "Центральный федеральный округ", Aliases: []string{"рязанская область"}},
	{Canonical: "Самарская область", District: "Приволжский федеральный округ", Aliases: []string{"самарская область"}},
	{Canonical: "Саратовская область", District: "Приволжский федеральный округ", Aliases: []string{"саратовская область"}},
	{Canonical: "Свердловская область", District: "Уральский федеральный округ", Aliases: []string{"свердловская область"}},
	{Canonical: "Смоленская область", District: "Центральный федеральный округ", Aliases: []string{"смоленская область"}},
	{Canonical: "Ставропольский край", District: "Северо-Кавказский федеральный округ", Aliases: []string{"ставропольский край"}},
	{Canonical: "Тамбовская область", District: "Центральный федеральный округ", Aliases: []string{"тамбовская область"}},
	{Canonical: "Тверская область", District: "Центральный федеральный округ", Aliases: []string{"тверская область"}},
	{Canonical: "Томская область", District: "Сибирский федеральный округ", Aliases: []string{"томская область"}},
	{Canonical: "Тульская область", District: "Центральный федеральный округ", Aliases: []string{"тульская область"}},
	{Canonical: "Удмуртская Республика", District: "Приволжский федеральный округ", Aliases: []string{"удмуртская республика", "удмуртия"}},
	{Canonical: "Ульяновская область", District: "Приволжский федеральный округ", Aliases: []string{"ульяновская область"}},
	{Canonical: "Хабаровский край", District: "Дальневосточный федеральный округ", Aliases: []string{"хабаровский край"}},
	{Canonical: "Херсонская область", District: "Южный федеральный округ", Aliases: []string{"херсонская область"}},
	{Canonical: "Челябинская область", District: "Уральский федеральный округ", Aliases: []string{"челябинская область"}},
	{Canonical: "Чеченская Республика", District: "Северо-Кавказский федеральный округ", Aliases: []string{"чеченская республика", "чечня"}},
	{Canonical: "Чувашская Республика - Чувашия", District: "Приволжский федеральный округ", Aliases: []string{"чувашская республика чувашия", "чувашия"}},
	{Canonical: "Ярославская область", District: "Центральный федеральный округ", Aliases: []string{"ярославская область"}},
}

func newRegionIndex(profiles []regionProfile) regionIndex {
	index := regionIndex{
		byCanonical: make(map[string]regionProfile, len(profiles)),
		byExact:     make(map[string]string, len(profiles)*2),
		aliases:     make([]regionAlias, 0, len(profiles)*3),
	}
	for _, profile := range profiles {
		index.byCanonical[profile.Canonical] = profile
		allAliases := append([]string{profile.Canonical}, profile.Aliases...)
		for _, alias := range allAliases {
			norm := normalizeText(alias)
			if norm == "" {
				continue
			}
			index.byExact[norm] = profile.Canonical
			index.aliases = append(index.aliases, regionAlias{
				Normalized: norm,
				Canonical:  profile.Canonical,
			})
		}
	}
	sort.Slice(index.aliases, func(i, j int) bool {
		return len(index.aliases[i].Normalized) > len(index.aliases[j].Normalized)
	})
	return index
}

func canonicalizeRegion(raw string) string {
	norm := normalizeText(raw)
	if norm == "" {
		return ""
	}
	if canonical, ok := compiledRegionIndex.byExact[norm]; ok {
		return canonical
	}
	for _, alias := range compiledRegionIndex.aliases {
		if strings.Contains(norm, alias.Normalized) {
			return alias.Canonical
		}
	}
	return strings.TrimSpace(raw)
}

func canonicalRegionList(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		canonical := canonicalizeRegion(item)
		if canonical == "" {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	sort.Strings(out)
	return out
}

func displayRegionName(raw string) string {
	canonical := canonicalizeRegion(raw)
	if canonical != "" {
		return canonical
	}
	return strings.TrimSpace(raw)
}

func classifyRegionTier(requestedRegion, customerRegion, supplierRegion string) regionTier {
	selected := canonicalizeRegion(requestedRegion)
	if selected == "" {
		return regionTierAll
	}
	best := regionTierOther
	for _, candidate := range []string{customerRegion, supplierRegion} {
		tier := compareRegionTier(selected, canonicalizeRegion(candidate))
		if regionTierRank(tier) < regionTierRank(best) {
			best = tier
		}
	}
	return best
}

func compareRegionTier(selected, candidate string) regionTier {
	if selected == "" || candidate == "" {
		return regionTierOther
	}
	if selected == candidate {
		return regionTierExact
	}
	if isRegionClusterNeighbor(selected, candidate) {
		return regionTierCluster
	}
	selectedProfile, selectedOK := compiledRegionIndex.byCanonical[selected]
	candidateProfile, candidateOK := compiledRegionIndex.byCanonical[candidate]
	if selectedOK && candidateOK && selectedProfile.District != "" && selectedProfile.District == candidateProfile.District {
		return regionTierDistrict
	}
	return regionTierOther
}

func isRegionClusterNeighbor(left, right string) bool {
	leftProfile, leftOK := compiledRegionIndex.byCanonical[left]
	if leftOK {
		for _, item := range leftProfile.Cluster {
			if item == right {
				return true
			}
		}
	}
	rightProfile, rightOK := compiledRegionIndex.byCanonical[right]
	if rightOK {
		for _, item := range rightProfile.Cluster {
			if item == left {
				return true
			}
		}
	}
	return false
}

func selectContractsByRegionScope(contracts []contractRecord, requestedRegion string) ([]contractRecord, regionSelectionInfo) {
	selected := canonicalizeRegion(requestedRegion)
	if selected == "" {
		counts := map[string]int{
			tierLabel(regionTierAll): len(contracts),
		}
		return contracts, regionSelectionInfo{
			Scope:         "Поиск по всем регионам",
			UsedTiers:     []string{tierLabel(regionTierAll)},
			Counts:        counts,
			FallbackToAll: false,
		}
	}

	buckets := map[regionTier][]contractRecord{
		regionTierExact:    {},
		regionTierCluster:  {},
		regionTierDistrict: {},
		regionTierOther:    {},
	}
	counts := map[string]int{
		tierLabel(regionTierExact):    0,
		tierLabel(regionTierCluster):  0,
		tierLabel(regionTierDistrict): 0,
		tierLabel(regionTierOther):    0,
	}
	for _, contract := range contracts {
		tier := classifyRegionTier(selected, contract.CustomerRegion, contract.SupplierRegion)
		buckets[tier] = append(buckets[tier], contract)
		counts[tierLabel(tier)]++
	}

	included := []regionTier{regionTierExact}
	total := len(buckets[regionTierExact])
	if total < 12 {
		included = append(included, regionTierCluster)
		total += len(buckets[regionTierCluster])
	}
	if total < 12 {
		included = append(included, regionTierDistrict)
		total += len(buckets[regionTierDistrict])
	}
	if total < 12 {
		included = append(included, regionTierOther)
		total += len(buckets[regionTierOther])
	}

	out := make([]contractRecord, 0, total)
	used := make([]string, 0, len(included))
	for _, tier := range included {
		out = append(out, buckets[tier]...)
		used = append(used, tierLabel(tier))
	}

	return out, regionSelectionInfo{
		SelectedCanonical: selected,
		Scope:             describeRegionScope(selected, included),
		UsedTiers:         used,
		Counts:            counts,
		FallbackToAll:     containsRegionTier(included, regionTierOther),
	}
}

func regionWeightForTier(req CalculateRequest, tier regionTier) float64 {
	if req.Region == "" || tier == regionTierAll || tier == regionTierExact {
		return req.SameRegionWeight
	}
	switch tier {
	case regionTierCluster:
		return blendRegionWeights(req.SameRegionWeight, req.OtherRegionWeight, 0.75)
	case regionTierDistrict:
		return blendRegionWeights(req.SameRegionWeight, req.OtherRegionWeight, 0.40)
	default:
		return req.OtherRegionWeight
	}
}

func blendRegionWeights(same, other, closeness float64) float64 {
	return other + (same-other)*closeness
}

func describeRegionScope(selected string, tiers []regionTier) string {
	if len(tiers) == 0 {
		return "Подходящих закупок по региону не найдено"
	}
	if len(tiers) == 1 {
		switch tiers[0] {
		case regionTierExact:
			return "Использован только точный регион: " + selected
		case regionTierCluster:
			return "Использованы только близкие регионы для " + selected
		case regionTierDistrict:
			return "Использован тот же федеральный округ для " + selected
		case regionTierAll:
			return "Поиск по всем регионам"
		default:
			return "Использованы все регионы"
		}
	}
	labels := make([]string, 0, len(tiers))
	for _, tier := range tiers {
		labels = append(labels, strings.ToLower(tierLabel(tier)))
	}
	return "Использованы уровни: " + strings.Join(labels, ", ")
}

func tierLabel(tier regionTier) string {
	switch tier {
	case regionTierAll:
		return "Все регионы"
	case regionTierExact:
		return "Точный регион"
	case regionTierCluster:
		return "Близкие регионы"
	case regionTierDistrict:
		return "Тот же федеральный округ"
	default:
		return "Остальные регионы"
	}
}

func containsRegionTier(items []regionTier, target regionTier) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func regionTierRank(tier regionTier) int {
	switch tier {
	case regionTierAll:
		return 0
	case regionTierExact:
		return 1
	case regionTierCluster:
		return 2
	case regionTierDistrict:
		return 3
	default:
		return 4
	}
}
