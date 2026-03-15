package app

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var searchStopWords = map[string]struct{}{
	"\u0434\u043b\u044f": {},
	"\u043f\u043e\u0434": {},
	"\u043d\u0430\u0434": {},
	"\u043f\u0440\u0438": {},
	"\u0431\u0435\u0437": {},
	"\u0438\u043b\u0438": {},
	"and":                {},
}

type searchMode struct {
	ResultLimit    int
	CandidateLimit int
	MinQueryRunes  int
	Suggest        bool
}

type searchSignals struct {
	QueryTokenCount       int
	MatchedNameTokens     int
	ExactNameHits         int
	PrefixNameHits        int
	ContainsNameHits      int
	AttributeHits         int
	ManufacturerHits      int
	CategoryHits          int
	AllQueryTokensMatched bool
	ExactOrderMatch       bool
}

type indexedSearchItem struct {
	ID                 int64
	Name               string
	Category           string
	Manufacturer       string
	NameNorm           string
	CategoryNorm       string
	ManufacturerNorm   string
	AttrsText          string
	SearchText         string
	NameTokens         []string
	AttrTokens         []string
	CategoryTokens     []string
	ManufacturerTokens []string
	ContractCount      int
	AvgUnitPrice       *float64
}

type candidateState struct {
	baseScore float64
	tokenHits int
}

type queryVariant struct {
	Text              string
	KeyboardCorrected bool
	SpellingCorrected bool
	ReorderedTokens   bool
}

func (v queryVariant) altered() bool {
	return v.KeyboardCorrected || v.SpellingCorrected || v.ReorderedTokens
}

func (v queryVariant) penalty() float64 {
	penalty := 0.0
	if v.KeyboardCorrected {
		penalty += 0.30
	}
	if v.SpellingCorrected {
		penalty += 0.42
	}
	if v.ReorderedTokens {
		penalty += 0.06
	}
	return penalty
}

type responseCache struct {
	mu         sync.RWMutex
	items      map[string]SearchResponse
	maxEntries int
}

func newResponseCache(maxEntries int) responseCache {
	return responseCache{
		items:      make(map[string]SearchResponse, maxEntries),
		maxEntries: maxEntries,
	}
}

func (c *responseCache) get(key string) (SearchResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	resp, ok := c.items[key]
	return resp, ok
}

func (c *responseCache) set(key string, value SearchResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= c.maxEntries {
		c.items = make(map[string]SearchResponse, c.maxEntries)
	}
	c.items[key] = value
}

type memorySearchIndex struct {
	items             []indexedSearchItem
	byID              map[int64]int
	byIDPrefix        map[string][]int
	byExactName       map[string][]int
	byNamePrefix      map[string][]int
	byNameToken       map[string][]int
	byNameTokenPrefix map[string][]int
	byAttrToken       map[string][]int
	byAttrTokenPrefix map[string][]int
	byToken           map[string][]int
	byTokenPrefix     map[string][]int
	tokenFreq         map[string]int
	deleteIndex       map[string][]string
	correctionCache   sync.Map
	searchCache       responseCache
	suggestCache      responseCache
}

var (
	searchIndexMu sync.RWMutex
	searchIndex   *memorySearchIndex
)

func LoadSearchIndex(ctx context.Context, pool *pgxpool.Pool) error {
	started := time.Now()
	idx, err := buildMemorySearchIndex(ctx, pool)
	if err != nil {
		return err
	}
	searchIndexMu.Lock()
	searchIndex = idx
	searchIndexMu.Unlock()
	log.Printf("search index loaded: %d items in %s", len(idx.items), time.Since(started).Round(time.Millisecond))
	return nil
}

func ensureSearchIndex(ctx context.Context, pool *pgxpool.Pool) (*memorySearchIndex, error) {
	searchIndexMu.RLock()
	idx := searchIndex
	searchIndexMu.RUnlock()
	if idx != nil {
		return idx, nil
	}
	if err := LoadSearchIndex(ctx, pool); err != nil {
		return nil, err
	}
	searchIndexMu.RLock()
	defer searchIndexMu.RUnlock()
	return searchIndex, nil
}

func SearchCTE(ctx context.Context, pool *pgxpool.Pool, rawQuery string) (SearchResponse, error) {
	idx, err := ensureSearchIndex(ctx, pool)
	if err != nil {
		return SearchResponse{}, err
	}
	return idx.search(rawQuery, searchMode{
		ResultLimit:    20,
		CandidateLimit: 420,
		MinQueryRunes:  2,
	}), nil
}

func SuggestCTE(ctx context.Context, pool *pgxpool.Pool, rawQuery string) (SearchResponse, error) {
	idx, err := ensureSearchIndex(ctx, pool)
	if err != nil {
		return SearchResponse{}, err
	}
	return idx.search(rawQuery, searchMode{
		ResultLimit:    8,
		CandidateLimit: 180,
		MinQueryRunes:  1,
		Suggest:        true,
	}), nil
}

func buildMemorySearchIndex(ctx context.Context, pool *pgxpool.Pool) (*memorySearchIndex, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, name, category, manufacturer, name_norm, category_norm, manufacturer_norm, attrs_text, search_text, contract_count, avg_unit_price::float8
		FROM cte_items
		ORDER BY contract_count DESC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idx := &memorySearchIndex{
		items:             make([]indexedSearchItem, 0, 260000),
		byID:              make(map[int64]int, 260000),
		byIDPrefix:        make(map[string][]int, 4096),
		byExactName:       make(map[string][]int, 260000),
		byNamePrefix:      make(map[string][]int, 8192),
		byNameToken:       make(map[string][]int, 131072),
		byNameTokenPrefix: make(map[string][]int, 65536),
		byAttrToken:       make(map[string][]int, 131072),
		byAttrTokenPrefix: make(map[string][]int, 65536),
		byToken:           make(map[string][]int, 131072),
		byTokenPrefix:     make(map[string][]int, 65536),
		tokenFreq:         make(map[string]int, 131072),
		deleteIndex:       make(map[string][]string, 262144),
		searchCache:       newResponseCache(4096),
		suggestCache:      newResponseCache(4096),
	}

	for rows.Next() {
		var item indexedSearchItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.Manufacturer, &item.NameNorm, &item.CategoryNorm, &item.ManufacturerNorm, &item.AttrsText, &item.SearchText, &item.ContractCount, &item.AvgUnitPrice); err != nil {
			return nil, err
		}
		item.NameTokens = uniqueTokens(item.NameNorm)
		item.AttrTokens = uniqueTokens(item.AttrsText)
		item.CategoryTokens = uniqueTokens(item.CategoryNorm)
		item.ManufacturerTokens = uniqueTokens(item.ManufacturerNorm)
		docID := len(idx.items)
		idx.items = append(idx.items, item)
		idx.byID[item.ID] = docID
		idx.byExactName[item.NameNorm] = append(idx.byExactName[item.NameNorm], docID)
		for _, prefix := range stringPrefixes(item.NameNorm, 6) {
			idx.byNamePrefix[prefix] = append(idx.byNamePrefix[prefix], docID)
		}
		for _, prefix := range stringPrefixes(strconv.FormatInt(item.ID, 10), 6) {
			idx.byIDPrefix[prefix] = append(idx.byIDPrefix[prefix], docID)
		}
		seenNamePrefixes := make(map[string]struct{}, 16)
		for _, token := range item.NameTokens {
			idx.byNameToken[token] = append(idx.byNameToken[token], docID)
			for _, prefix := range stringPrefixes(token, 4) {
				if _, ok := seenNamePrefixes[prefix]; ok {
					continue
				}
				seenNamePrefixes[prefix] = struct{}{}
				idx.byNameTokenPrefix[prefix] = append(idx.byNameTokenPrefix[prefix], docID)
			}
		}
		seenAttrPrefixes := make(map[string]struct{}, 24)
		for _, token := range item.AttrTokens {
			idx.byAttrToken[token] = append(idx.byAttrToken[token], docID)
			for _, prefix := range stringPrefixes(token, 4) {
				if _, ok := seenAttrPrefixes[prefix]; ok {
					continue
				}
				seenAttrPrefixes[prefix] = struct{}{}
				idx.byAttrTokenPrefix[prefix] = append(idx.byAttrTokenPrefix[prefix], docID)
			}
		}
		seenTokenPrefixes := make(map[string]struct{}, 32)
		for _, token := range uniqueTokens(item.SearchText) {
			idx.byToken[token] = append(idx.byToken[token], docID)
			idx.tokenFreq[token]++
			for _, prefix := range stringPrefixes(token, 4) {
				if _, ok := seenTokenPrefixes[prefix]; ok {
					continue
				}
				seenTokenPrefixes[prefix] = struct{}{}
				idx.byTokenPrefix[prefix] = append(idx.byTokenPrefix[prefix], docID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for token := range idx.tokenFreq {
		if len([]rune(token)) < 3 || isDigitsOnly(token) {
			continue
		}
		for _, del := range deleteVariants(token, 1) {
			idx.deleteIndex[del] = appendUniqueString(idx.deleteIndex[del], token)
		}
	}
	return idx, nil
}

func (idx *memorySearchIndex) search(rawQuery string, mode searchMode) SearchResponse {
	query := normalizeText(rawQuery)
	resp := SearchResponse{Query: rawQuery, Normalized: query}
	if query == "" {
		return resp
	}
	if len([]rune(query)) < mode.MinQueryRunes && !isDigitsOnly(query) {
		return resp
	}
	if cached, ok := idx.cacheForMode(mode).get(query); ok {
		cached.Query = rawQuery
		cached.Normalized = query
		return cached
	}

	variants := idx.queryVariants(query)
	alternatives := make([]string, 0, len(variants))
	bestByID := make(map[int64]SearchItem, mode.CandidateLimit)

	for _, variant := range variants {
		items := idx.searchVariant(variant, mode)
		if variant.altered() && len(items) > 0 && variant.Text != query {
			alternatives = append(alternatives, variant.Text)
		}
		for _, item := range items {
			prev, ok := bestByID[item.ID]
			if !ok || item.Score > prev.Score {
				bestByID[item.ID] = item
			}
		}
	}

	items := make([]SearchItem, 0, len(bestByID))
	for _, item := range bestByID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			if items[i].ContractCount == items[j].ContractCount {
				return items[i].ID < items[j].ID
			}
			return items[i].ContractCount > items[j].ContractCount
		}
		return items[i].Score > items[j].Score
	})
	if len(items) > mode.ResultLimit {
		items = items[:mode.ResultLimit]
	}
	fillConfidence(items)
	resp.Items = items
	resp.Alternatives = uniqueStrings(alternatives)
	idx.cacheForMode(mode).set(query, resp)
	return resp
}

func (idx *memorySearchIndex) cacheForMode(mode searchMode) *responseCache {
	if mode.Suggest {
		return &idx.suggestCache
	}
	return &idx.searchCache
}

func (idx *memorySearchIndex) searchVariant(variant queryVariant, mode searchMode) []SearchItem {
	query := variant.Text
	tokens := searchTokens(query)
	candidates := make(map[int]*candidateState, mode.CandidateLimit)

	if isDigitsOnly(query) {
		if docID, ok := idx.byID[mustParseInt64(query)]; ok {
			addCandidate(candidates, docID, 18, 1)
		}
		for _, docID := range idx.byIDPrefix[prefixKey(query, 6)] {
			if strings.HasPrefix(strconv.FormatInt(idx.items[docID].ID, 10), query) {
				addCandidate(candidates, docID, 8, 1)
			}
		}
	}

	for _, docID := range idx.byExactName[query] {
		addCandidate(candidates, docID, 16, 1)
	}
	for _, docID := range idx.byNamePrefix[prefixKey(query, 6)] {
		item := idx.items[docID]
		switch {
		case item.NameNorm == query:
			addCandidate(candidates, docID, 18, 1)
		case strings.HasPrefix(item.NameNorm, query):
			addCandidate(candidates, docID, 10, 1)
		case hasWholePhrase(item.NameNorm, query):
			addCandidate(candidates, docID, 6, 1)
		}
	}

	for _, token := range tokens {
		nameDocs, nameExact, nameSource := idx.lookupDocsForToken(token, mode, true)
		attrDocs, attrExact, attrSource := idx.lookupAttributeDocsForToken(token, mode)
		boost := 1.8
		hits := 1
		if nameExact {
			boost = 4.6
		} else if nameSource {
			boost = 2.8
		}
		for _, docID := range nameDocs {
			addCandidate(candidates, docID, boost, hits)
		}
		if len(attrDocs) > 0 {
			boost = 1.4
			if attrExact {
				boost = 2.5
			} else if attrSource {
				boost = 1.9
			}
			for _, docID := range attrDocs {
				addCandidate(candidates, docID, boost, 1)
			}
		}
		if nameSource || attrSource {
			continue
		}
		docs, exact, _ := idx.lookupDocsForToken(token, mode, false)
		boost = 0.75
		if exact {
			boost = 1.15
		}
		for _, docID := range docs {
			addCandidate(candidates, docID, boost, 0)
		}
	}

	results := make([]SearchItem, 0, len(candidates))
	for _, item := range idx.limitCandidates(candidates, mode.CandidateLimit) {
		score, reason, signals := rerankIndexedCandidate(query, tokens, item.baseScore, idx.items[item.docID], mode, variant)
		result := SearchItem{
			ID:                    idx.items[item.docID].ID,
			Name:                  idx.items[item.docID].Name,
			Category:              idx.items[item.docID].Category,
			Manufacturer:          idx.items[item.docID].Manufacturer,
			ContractCount:         idx.items[item.docID].ContractCount,
			AvgUnitPrice:          idx.items[item.docID].AvgUnitPrice,
			Score:                 score,
			MatchReason:           reason,
			QueryTokenCount:       signals.QueryTokenCount,
			MatchedNameTokens:     signals.MatchedNameTokens,
			AllQueryTokensMatched: signals.AllQueryTokensMatched,
			ExactOrderMatch:       signals.ExactOrderMatch,
			AttributeHits:         signals.AttributeHits,
			ManufacturerHits:      signals.ManufacturerHits,
			CategoryHits:          signals.CategoryHits,
			KeyboardCorrected:     variant.KeyboardCorrected,
			SpellingCorrected:     variant.SpellingCorrected,
			ReorderedTokens:       variant.ReorderedTokens,
		}
		if result.Score > 0.45 {
			results = append(results, result)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			if results[i].ContractCount == results[j].ContractCount {
				return results[i].ID < results[j].ID
			}
			return results[i].ContractCount > results[j].ContractCount
		}
		return results[i].Score > results[j].Score
	})
	return results
}

type rankedCandidate struct {
	docID     int
	baseScore float64
	tokenHits int
}

func (idx *memorySearchIndex) limitCandidates(states map[int]*candidateState, limit int) []rankedCandidate {
	out := make([]rankedCandidate, 0, len(states))
	for docID, state := range states {
		out = append(out, rankedCandidate{docID: docID, baseScore: state.baseScore, tokenHits: state.tokenHits})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].baseScore == out[j].baseScore {
			return idx.items[out[i].docID].ContractCount > idx.items[out[j].docID].ContractCount
		}
		return out[i].baseScore > out[j].baseScore
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func rerankIndexedCandidate(query string, tokens []string, baseScore float64, item indexedSearchItem, mode searchMode, variant queryVariant) (float64, string, searchSignals) {
	exactNameHits := 0
	prefixNameHits := 0
	containsNameHits := 0
	exactAttrHits := 0
	prefixAttrHits := 0
	containsAttrHits := 0
	manufacturerHits := 0
	categoryHits := 0
	matchedTokens := 0

	for _, token := range tokens {
		switch tokenFieldMatch(token, item.NameTokens, item.NameNorm) {
		case matchExact:
			exactNameHits++
			matchedTokens++
			continue
		case matchPrefix:
			prefixNameHits++
			matchedTokens++
			continue
		case matchContains:
			containsNameHits++
			matchedTokens++
			continue
		}
		switch tokenFieldMatch(token, item.AttrTokens, item.AttrsText) {
		case matchExact:
			exactAttrHits++
			matchedTokens++
			continue
		case matchPrefix:
			prefixAttrHits++
			matchedTokens++
			continue
		case matchContains:
			containsAttrHits++
			matchedTokens++
			continue
		}
		if tokenFieldMatch(token, item.ManufacturerTokens, item.ManufacturerNorm) != matchNone {
			manufacturerHits++
			matchedTokens++
			continue
		}
		if tokenFieldMatch(token, item.CategoryTokens, item.CategoryNorm) != matchNone {
			categoryHits++
			matchedTokens++
		}
	}

	tokenCount := maxInt(len(tokens), 1)
	matchedNameTokens := exactNameHits + prefixNameHits + containsNameHits
	attributeHits := exactAttrHits + prefixAttrHits + containsAttrHits
	allTokensMatched := matchedTokens >= tokenCount
	exactOrderMatch := hasWholePhrase(item.NameNorm, query) || item.NameNorm == query
	orderedHits := orderedTokenHits(tokens, item.NameTokens)
	prefixOrderMatch := strings.HasPrefix(item.NameNorm, query)

	score := baseScore * 0.48
	reason := "похоже по написанию"

	switch {
	case isDigitsOnly(query) && strconv.FormatInt(item.ID, 10) == query:
		score += 16
		reason = "точное совпадение по коду СТЕ"
	case item.NameNorm == query:
		score += 10
		reason = "точное совпадение по названию"
	case exactOrderMatch:
		score += 3.2
		reason = "вся фраза найдена в названии"
	case prefixOrderMatch:
		score += 2.2
		reason = "совпадает начало названия"
	case strings.Contains(item.NameNorm, query):
		score += 1.2
		reason = "часть запроса найдена в названии"
	}

	coverage := (float64(exactNameHits) + float64(prefixNameHits)*0.80 + float64(containsNameHits)*0.45 +
		float64(exactAttrHits)*0.92 + float64(prefixAttrHits)*0.72 + float64(containsAttrHits)*0.40 +
		float64(manufacturerHits)*0.70 + float64(categoryHits)*0.45) / float64(tokenCount)
	score += float64(exactNameHits) * 2.3
	score += float64(prefixNameHits) * 1.25
	score += float64(containsNameHits) * 0.38
	score += float64(exactAttrHits) * 1.55
	score += float64(prefixAttrHits) * 0.95
	score += float64(containsAttrHits) * 0.30
	score += coverage * 5.8
	if allTokensMatched {
		score += 4.8
	}
	if orderedHits == tokenCount {
		score += 0.9
	}
	if exactOrderMatch {
		score += 0.8
	}
	score += minFloat(float64(manufacturerHits)*0.32, 0.64)
	score += minFloat(float64(categoryHits)*0.10, 0.24)
	score += minFloat(math.Log1p(float64(item.ContractCount))*0.15, 0.55)

	if len(tokens) > 1 && !allTokensMatched {
		score -= 2.1
	}
	if matchedNameTokens == 0 && attributeHits == 0 {
		score -= 1.5
	}
	if matchedNameTokens == 0 && attributeHits == 0 && manufacturerHits == 0 && categoryHits > 0 {
		score -= 1.8
	}
	if mode.Suggest && len(tokens) > 1 && !allTokensMatched {
		score -= 1.0
	}
	score -= variant.penalty()

	switch {
	case allTokensMatched && attributeHits > 0:
		reason = fmt.Sprintf("часть слов совпала в названии, часть в характеристиках: %d из %d", matchedTokens, tokenCount)
	case allTokensMatched:
		reason = fmt.Sprintf("в названии найдено %d из %d слов", matchedNameTokens, tokenCount)
	case matchedNameTokens > 0 && attributeHits > 0:
		reason = fmt.Sprintf("совпали название и характеристики: %d из %d слов", matchedTokens, tokenCount)
	case exactNameHits > 0:
		reason = fmt.Sprintf("в названии совпало %d из %d слов", exactNameHits, tokenCount)
	case attributeHits > 0:
		reason = "есть совпадение по характеристикам товара"
	case prefixNameHits > 0:
		reason = "совпадает начало одного из слов в названии"
	case containsNameHits > 0:
		reason = "часть слова найдена в названии"
	case manufacturerHits > 0:
		reason = "совпадение по производителю"
	case categoryHits > 0:
		reason = "подобрано по категории"
	}

	return clamp(score, 0, 30), reason, searchSignals{
		QueryTokenCount:       tokenCount,
		MatchedNameTokens:     matchedNameTokens,
		ExactNameHits:         exactNameHits,
		PrefixNameHits:        prefixNameHits,
		ContainsNameHits:      containsNameHits,
		AttributeHits:         attributeHits,
		ManufacturerHits:      manufacturerHits,
		CategoryHits:          categoryHits,
		AllQueryTokensMatched: allTokensMatched,
		ExactOrderMatch:       exactOrderMatch,
	}
}

func fillConfidence(items []SearchItem) {
	if len(items) == 0 {
		return
	}
	top := items[0].Score
	second := 0.0
	if len(items) > 1 {
		second = items[1].Score
	}
	gap := top - second
	for i := range items {
		tokenCount := maxInt(items[i].QueryTokenCount, 1)
		coverage := float64(items[i].MatchedNameTokens+items[i].AttributeHits+items[i].ManufacturerHits+items[i].CategoryHits) / float64(tokenCount)
		base := 0.16 + coverage*0.54
		if items[i].AllQueryTokensMatched {
			base += 0.16
		}
		if items[i].ExactOrderMatch {
			base += 0.05
		}
		base += minFloat(float64(items[i].ManufacturerHits)*0.02, 0.04)
		base += minFloat(float64(items[i].CategoryHits)*0.01, 0.03)
		base += minFloat(float64(items[i].AttributeHits)*0.03, 0.06)
		if items[i].KeyboardCorrected {
			base -= 0.03
		}
		if items[i].SpellingCorrected {
			base -= 0.05
		}
		if items[i].QueryTokenCount > 1 && !items[i].AllQueryTokensMatched {
			base -= 0.12
		}
		if i == 0 {
			base += minFloat(0.08, gap*0.025)
		}
		items[i].Confidence = clamp(base, 0.05, 0.995)
		items[i].ConfidenceLabel = confidenceLabel(items[i].Confidence)
		items[i].ConfidenceExplanation = buildConfidenceExplanation(items[i], gap, i == 0)
	}
}

func confidenceLabel(value float64) string {
	switch {
	case value >= 0.93:
		return "Очень точное совпадение"
	case value >= 0.82:
		return "Хорошее совпадение"
	case value >= 0.67:
		return "Похоже, но лучше проверить"
	default:
		return "Слабое совпадение"
	}
}

func buildConfidenceExplanation(item SearchItem, topGap float64, isTop bool) string {
	parts := make([]string, 0, 4)
	parts = append(parts, "Это оценка совпадения, а не строгая вероятность.")
	tokenCount := maxInt(item.QueryTokenCount, 1)
	if item.AllQueryTokensMatched {
		parts = append(parts, fmt.Sprintf("Совпали все %d %s запроса.", tokenCount, russianNoun(tokenCount, "слово", "слова", "слов")))
	} else {
		totalHits := item.MatchedNameTokens + item.AttributeHits + item.ManufacturerHits + item.CategoryHits
		parts = append(parts, fmt.Sprintf("Найдено %d из %d %s запроса.", totalHits, tokenCount, russianNoun(tokenCount, "слова", "слов", "слов")))
	}
	if item.ExactOrderMatch {
		parts = append(parts, "Совпадает и вся фраза целиком.")
	} else if tokenCount > 1 {
		parts = append(parts, "Порядок слов не обязателен: важнее, чтобы совпали сами слова.")
	}
	if item.AttributeHits > 0 {
		parts = append(parts, fmt.Sprintf("В характеристиках найдено еще %d %s.", item.AttributeHits, russianNoun(item.AttributeHits, "совпадение", "совпадения", "совпадений")))
	}
	if item.KeyboardCorrected {
		parts = append(parts, "Запрос дополнительно исправлен по раскладке клавиатуры.")
	}
	if item.SpellingCorrected {
		parts = append(parts, "Запрос дополнительно исправлен по опечатке.")
	}
	if item.ManufacturerHits > 0 {
		parts = append(parts, "Есть дополнительное совпадение по производителю.")
	} else if item.CategoryHits > 0 {
		parts = append(parts, "Категория помогает ранжированию, но не решает его одна.")
	}
	if isTop {
		switch {
		case topGap >= 2.0:
			parts = append(parts, "Этот вариант заметно сильнее следующего.")
		case topGap >= 0.8:
			parts = append(parts, "Этот вариант немного сильнее следующего.")
		}
	}
	return strings.Join(parts, " ")
}

func (idx *memorySearchIndex) queryVariants(query string) []queryVariant {
	best := make(map[string]queryVariant, 8)
	add := func(variant queryVariant) {
		variant.Text = normalizeText(variant.Text)
		if variant.Text == "" {
			return
		}
		current, ok := best[variant.Text]
		if !ok || variant.penalty() < current.penalty() {
			best[variant.Text] = variant
		}
	}

	add(queryVariant{Text: query})
	if reordered, ok := reorderedQueryVariant(query); ok {
		add(queryVariant{Text: reordered, ReorderedTokens: true})
	}
	if switched, ok := idx.switchKeyboardQuery(query); ok {
		add(queryVariant{Text: switched, KeyboardCorrected: true})
		if reordered, ok := reorderedQueryVariant(switched); ok {
			add(queryVariant{Text: reordered, KeyboardCorrected: true, ReorderedTokens: true})
		}
		if corrected, ok := idx.correctQuery(switched); ok {
			add(queryVariant{Text: corrected, KeyboardCorrected: true, SpellingCorrected: true})
			if reordered, ok := reorderedQueryVariant(corrected); ok {
				add(queryVariant{Text: reordered, KeyboardCorrected: true, SpellingCorrected: true, ReorderedTokens: true})
			}
		}
	}
	if corrected, ok := idx.correctQuery(query); ok {
		add(queryVariant{Text: corrected, SpellingCorrected: true})
		if reordered, ok := reorderedQueryVariant(corrected); ok {
			add(queryVariant{Text: reordered, SpellingCorrected: true, ReorderedTokens: true})
		}
	}

	variants := make([]queryVariant, 0, len(best))
	for _, variant := range best {
		variants = append(variants, variant)
	}
	sort.Slice(variants, func(i, j int) bool {
		if variants[i].penalty() == variants[j].penalty() {
			return len([]rune(variants[i].Text)) > len([]rune(variants[j].Text))
		}
		return variants[i].penalty() < variants[j].penalty()
	})
	return variants
}

func (idx *memorySearchIndex) correctQuery(query string) (string, bool) {
	tokens := rawQueryTokens(query)
	if len(tokens) == 0 {
		return "", false
	}
	changed := false
	out := make([]string, len(tokens))
	for i, token := range tokens {
		corrected, ok := idx.correctToken(token)
		if ok && corrected != token {
			out[i] = corrected
			changed = true
			continue
		}
		out[i] = token
	}
	if !changed {
		return "", false
	}
	return strings.Join(out, " "), true
}

func (idx *memorySearchIndex) correctToken(token string) (string, bool) {
	if token == "" || isDigitsOnly(token) || len([]rune(token)) < 3 {
		return token, false
	}
	if _, ok := idx.byToken[token]; ok {
		return token, false
	}
	prefixDocs := idx.byNameTokenPrefix[prefixKey(token, 4)]
	if len([]rune(token)) <= 3 && len(prefixDocs) > 0 {
		return token, false
	}
	if len([]rune(token)) == 4 && len(prefixDocs) > 20 {
		return token, false
	}
	if cached, ok := idx.correctionCache.Load(token); ok {
		value := cached.(string)
		if value == "" || value == token {
			return token, false
		}
		return value, true
	}
	candidateSet := make(map[string]struct{}, 16)
	for _, candidate := range idx.deleteIndex[token] {
		candidateSet[candidate] = struct{}{}
	}
	maxDistance := 1
	if len([]rune(token)) >= 6 {
		maxDistance = 2
	}
	for _, del := range deleteVariants(token, maxDistance) {
		for _, candidate := range idx.deleteIndex[del] {
			candidateSet[candidate] = struct{}{}
		}
	}
	best := ""
	bestDistance := maxDistance + 1
	bestFreq := -1
	for candidate := range candidateSet {
		distance := damerauLevenshteinWithin(token, candidate, maxDistance)
		if distance < 0 {
			continue
		}
		freq := idx.tokenFreq[candidate] + idx.tokenPresenceScore(candidate) + correctionSimilarityBonus(token, candidate)
		if distance < bestDistance || (distance == bestDistance && freq > bestFreq) {
			best = candidate
			bestDistance = distance
			bestFreq = freq
		}
	}
	idx.correctionCache.Store(token, best)
	if best == "" {
		return token, false
	}
	return best, true
}

func (idx *memorySearchIndex) lookupDocsForToken(token string, mode searchMode, preferName bool) ([]int, bool, bool) {
	if preferName {
		if docs, ok := idx.byNameToken[token]; ok {
			return limitIntSlice(docs, perTokenLimit(mode, true, true)), true, true
		}
		if prefix := prefixKey(token, 4); prefix != "" {
			if docs := idx.byNameTokenPrefix[prefix]; len(docs) > 0 {
				return limitIntSlice(docs, perTokenLimit(mode, true, false)), false, true
			}
		}
	}
	if docs, ok := idx.byToken[token]; ok {
		return limitIntSlice(docs, perTokenLimit(mode, false, true)), true, false
	}
	prefix := prefixKey(token, 4)
	if prefix == "" {
		return nil, false, false
	}
	docs := idx.byTokenPrefix[prefix]
	return limitIntSlice(docs, perTokenLimit(mode, false, false)), false, false
}

func (idx *memorySearchIndex) lookupAttributeDocsForToken(token string, mode searchMode) ([]int, bool, bool) {
	if docs, ok := idx.byAttrToken[token]; ok {
		return limitIntSlice(docs, perTokenLimit(mode, true, true)), true, true
	}
	prefix := prefixKey(token, 4)
	if prefix == "" {
		return nil, false, false
	}
	docs := idx.byAttrTokenPrefix[prefix]
	if len(docs) == 0 {
		return nil, false, false
	}
	return limitIntSlice(docs, perTokenLimit(mode, true, false)), false, true
}

func addCandidate(states map[int]*candidateState, docID int, boost float64, hits int) {
	state, ok := states[docID]
	if !ok {
		states[docID] = &candidateState{baseScore: boost, tokenHits: hits}
		return
	}
	state.baseScore += boost
	state.tokenHits += hits
}

func perTokenLimit(mode searchMode, nameSource bool, exact bool) int {
	if mode.Suggest {
		if nameSource && exact {
			return 100
		}
		if nameSource {
			return 70
		}
		if exact {
			return 140
		}
		return 90
	}
	if nameSource && exact {
		return 220
	}
	if nameSource {
		return 140
	}
	if exact {
		return 260
	}
	return 150
}

func searchTokens(query string) []string {
	parts := strings.Fields(normalizeText(query))
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if len([]rune(part)) < 2 && !isDigitsOnly(part) {
			continue
		}
		if _, stop := searchStopWords[part]; stop {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	if len(out) == 0 && query != "" {
		out = append(out, query)
	}
	return out
}

func uniqueTokens(text string) []string {
	return searchTokens(text)
}

func rawQueryTokens(query string) []string {
	parts := strings.Fields(normalizeText(query))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func reorderedQueryVariant(query string) (string, bool) {
	tokens := rawQueryTokens(query)
	if len(tokens) != 2 {
		return "", false
	}
	reordered := tokens[1] + " " + tokens[0]
	if reordered == query {
		return "", false
	}
	return reordered, true
}

func (idx *memorySearchIndex) switchKeyboardQuery(query string) (string, bool) {
	tokens := rawQueryTokens(query)
	if len(tokens) == 0 {
		return "", false
	}
	changed := false
	out := make([]string, len(tokens))
	for i, token := range tokens {
		switched, ok := idx.switchKeyboardToken(token)
		if ok && switched != token {
			out[i] = switched
			changed = true
			continue
		}
		out[i] = token
	}
	if !changed {
		return "", false
	}
	return strings.Join(out, " "), true
}

func (idx *memorySearchIndex) switchKeyboardToken(token string) (string, bool) {
	if token == "" || isDigitsOnly(token) {
		return token, false
	}
	if _, ok := idx.byToken[token]; ok {
		return token, false
	}
	if docs := idx.byNameTokenPrefix[prefixKey(token, 4)]; len(docs) > 0 {
		return token, false
	}
	best := token
	bestScore := 0
	for _, candidate := range uniqueStrings([]string{switchLatinToCyr(token), switchCyrToLatin(token)}) {
		if candidate == "" || candidate == token {
			continue
		}
		score := idx.tokenPresenceScore(candidate)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	if bestScore == 0 {
		return token, false
	}
	return best, true
}

func (idx *memorySearchIndex) tokenPresenceScore(token string) int {
	if token == "" {
		return 0
	}
	if docs, ok := idx.byNameToken[token]; ok {
		return 1000 + len(docs)
	}
	if docs, ok := idx.byToken[token]; ok {
		return 700 + len(docs)
	}
	if docs := idx.byNameTokenPrefix[prefixKey(token, 4)]; len(docs) > 0 {
		return 250 + len(docs)
	}
	if docs := idx.byTokenPrefix[prefixKey(token, 4)]; len(docs) > 0 {
		return 120 + len(docs)
	}
	return 0
}

func switchLatinToCyr(s string) string {
	latin := []rune("`qwertyuiop[]asdfghjkl;'zxcvbnm,.")
	cyr := []rune("\u0451\u0439\u0446\u0443\u043a\u0435\u043d\u0433\u0448\u0449\u0437\u0445\u044a\u0444\u044b\u0432\u0430\u043f\u0440\u043e\u043b\u0434\u0436\u044d\u044f\u0447\u0441\u043c\u0438\u0442\u044c\u0431\u044e")
	forward := make(map[rune]rune, len(latin))
	for i, r := range latin {
		forward[r] = cyr[i]
	}

	var b strings.Builder
	changed := false
	for _, r := range s {
		if mapped, ok := forward[r]; ok {
			b.WriteRune(mapped)
			changed = true
			continue
		}
		b.WriteRune(r)
	}
	if !changed {
		return ""
	}
	return normalizeText(b.String())
}

func switchCyrToLatin(s string) string {
	latin := []rune("`qwertyuiop[]asdfghjkl;'zxcvbnm,.")
	cyr := []rune("\u0451\u0439\u0446\u0443\u043a\u0435\u043d\u0433\u0448\u0449\u0437\u0445\u044a\u0444\u044b\u0432\u0430\u043f\u0440\u043e\u043b\u0434\u0436\u044d\u044f\u0447\u0441\u043c\u0438\u0442\u044c\u0431\u044e")
	backward := make(map[rune]rune, len(cyr))
	for i, r := range cyr {
		backward[r] = latin[i]
	}
	var b strings.Builder
	changed := false
	for _, r := range s {
		if mapped, ok := backward[r]; ok {
			b.WriteRune(mapped)
			changed = true
			continue
		}
		b.WriteRune(r)
	}
	if !changed {
		return ""
	}
	return normalizeText(b.String())
}

func deleteVariants(token string, maxDistance int) []string {
	seen := map[string]struct{}{}
	var out []string
	var visit func(string, int)
	visit = func(current string, dist int) {
		if dist == 0 || len([]rune(current)) <= 1 {
			return
		}
		runes := []rune(current)
		for i := range runes {
			next := string(append(append([]rune{}, runes[:i]...), runes[i+1:]...))
			if next == "" {
				continue
			}
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			out = append(out, next)
			visit(next, dist-1)
		}
	}
	visit(token, maxDistance)
	return out
}

func damerauLevenshteinWithin(a, b string, maxDistance int) int {
	ra := []rune(a)
	rb := []rune(b)
	if diff := len(ra) - len(rb); diff > maxDistance || diff < -maxDistance {
		return -1
	}
	prevPrev := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := 0; j <= len(rb); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= len(rb); j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			curr[j] = minInt3(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				curr[j] = minInt(curr[j], prevPrev[j-2]+1)
			}
			if curr[j] < rowMin {
				rowMin = curr[j]
			}
		}
		if rowMin > maxDistance {
			return -1
		}
		copy(prevPrev, prev)
		copy(prev, curr)
	}
	if prev[len(rb)] > maxDistance {
		return -1
	}
	return prev[len(rb)]
}

func stringPrefixes(value string, maxLen int) []string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return nil
	}
	limit := minInt(len(runes), maxLen)
	out := make([]string, 0, limit)
	for i := 1; i <= limit; i++ {
		out = append(out, string(runes[:i]))
	}
	return out
}

func prefixKey(value string, maxLen int) string {
	prefixes := stringPrefixes(value, maxLen)
	if len(prefixes) == 0 {
		return ""
	}
	return prefixes[len(prefixes)-1]
}

const (
	matchNone = iota
	matchContains
	matchPrefix
	matchExact
)

func tokenFieldMatch(token string, fieldTokens []string, fieldNorm string) int {
	if token == "" || len(fieldTokens) == 0 {
		return matchNone
	}
	for _, fieldToken := range fieldTokens {
		if fieldToken == token {
			return matchExact
		}
	}
	for _, fieldToken := range fieldTokens {
		if strings.HasPrefix(fieldToken, token) {
			return matchPrefix
		}
	}
	if strings.Contains(fieldNorm, token) {
		return matchContains
	}
	return matchNone
}

func orderedTokenHits(queryTokens []string, fieldTokens []string) int {
	if len(queryTokens) == 0 || len(fieldTokens) == 0 {
		return 0
	}
	position := -1
	hits := 0
	for _, token := range queryTokens {
		for i := position + 1; i < len(fieldTokens); i++ {
			if fieldTokens[i] == token || strings.HasPrefix(fieldTokens[i], token) {
				hits++
				position = i
				break
			}
		}
	}
	return hits
}

func correctionSimilarityBonus(source string, candidate string) int {
	sourceRunes := []rune(source)
	candidateRunes := []rune(candidate)
	if len(sourceRunes) == 0 || len(candidateRunes) == 0 {
		return 0
	}
	score := 0
	if sourceRunes[0] == candidateRunes[0] {
		score += 400
	}
	if sourceRunes[len(sourceRunes)-1] == candidateRunes[len(candidateRunes)-1] {
		score += 120
	}
	score += commonPrefixLen(sourceRunes, candidateRunes) * 80
	score += commonSuffixLen(sourceRunes, candidateRunes) * 90
	if len(sourceRunes) == len(candidateRunes) {
		score += 60
	}
	return score
}

func commonPrefixLen(left []rune, right []rune) int {
	limit := minInt(len(left), len(right))
	count := 0
	for i := 0; i < limit; i++ {
		if left[i] != right[i] {
			break
		}
		count++
	}
	return count
}

func commonSuffixLen(left []rune, right []rune) int {
	limit := minInt(len(left), len(right))
	count := 0
	for i := 1; i <= limit; i++ {
		if left[len(left)-i] != right[len(right)-i] {
			break
		}
		count++
	}
	return count
}

func hasWholePhrase(text, phrase string) bool {
	if text == "" || phrase == "" {
		return false
	}
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func clamp(v, minValue, maxValue float64) float64 {
	switch {
	case v < minValue:
		return minValue
	case v > maxValue:
		return maxValue
	default:
		return v
	}
}

func russianNoun(count int, one string, few string, many string) string {
	mod100 := count % 100
	if mod100 >= 11 && mod100 <= 14 {
		return many
	}
	switch count % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func appendUniqueString(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func limitIntSlice(items []int, limit int) []int {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func mustParseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minInt3(a, b, c int) int {
	return minInt(minInt(a, b), c)
}
