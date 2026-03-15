package app

import "time"

type SearchItem struct {
	ID                    int64      `json:"id"`
	Name                  string     `json:"name"`
	Category              string     `json:"category"`
	Manufacturer          string     `json:"manufacturer"`
	ContractCount         int        `json:"contract_count"`
	AvgUnitPrice          *float64   `json:"avg_unit_price,omitempty"`
	LastContractAt        *time.Time `json:"last_contract_at,omitempty"`
	Score                 float64    `json:"score"`
	Confidence            float64    `json:"confidence"`
	ConfidenceLabel       string     `json:"confidence_label,omitempty"`
	ConfidenceExplanation string     `json:"confidence_explanation,omitempty"`
	MatchReason           string     `json:"match_reason"`
	QueryTokenCount       int        `json:"query_token_count,omitempty"`
	MatchedNameTokens     int        `json:"matched_name_tokens,omitempty"`
	AllQueryTokensMatched bool       `json:"all_query_tokens_matched,omitempty"`
	ExactOrderMatch       bool       `json:"exact_order_match,omitempty"`
	ManufacturerHits      int        `json:"manufacturer_hits,omitempty"`
	CategoryHits          int        `json:"category_hits,omitempty"`
	AttributeHits         int        `json:"attribute_hits,omitempty"`
	KeyboardCorrected     bool       `json:"keyboard_corrected,omitempty"`
	SpellingCorrected     bool       `json:"spelling_corrected,omitempty"`
	ReorderedTokens       bool       `json:"reordered_tokens,omitempty"`
}

type SearchResponse struct {
	Query        string       `json:"query"`
	Normalized   string       `json:"normalized"`
	Alternatives []string     `json:"alternatives,omitempty"`
	Items        []SearchItem `json:"items"`
}

type RecentDocument struct {
	DocType      string    `json:"doc_type"`
	CTEID        int64     `json:"cte_id"`
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	Manufacturer string    `json:"manufacturer"`
	Region       string    `json:"region"`
	MonthsBack   int       `json:"months_back"`
	Version      int       `json:"version"`
	Summary      string    `json:"summary"`
	BatchKey     string    `json:"batch_key"`
	ItemCount    int       `json:"item_count"`
	FilePath     string    `json:"file_path"`
	FileURL      string    `json:"file_url"`
	CreatedAt    time.Time `json:"created_at"`
}

type CalculateRequest struct {
	CTEID                      int64            `json:"cte_id"`
	Region                     string           `json:"region"`
	MonthsBack                 int              `json:"months_back"`
	TimeDecay                  float64          `json:"time_decay"`
	TimeWeightMode             string           `json:"time_weight_mode,omitempty"`
	SameRegionWeight           float64          `json:"same_region_weight"`
	OtherRegionWeight          float64          `json:"other_region_weight"`
	SettingsMode               string           `json:"settings_mode,omitempty"`
	TimeImportanceLabel        string           `json:"time_importance_label,omitempty"`
	SameRegionImportanceLabel  string           `json:"same_region_importance_label,omitempty"`
	OtherRegionImportanceLabel string           `json:"other_region_importance_label,omitempty"`
	MaxResults                 int              `json:"max_results"`
	ExcludedIDs                []int64          `json:"excluded_ids"`
	PriceOverrides             []PriceOverride  `json:"price_overrides,omitempty"`
	WeightOverrides            []WeightOverride `json:"weight_overrides,omitempty"`
	ManualEntries              []ManualEntry    `json:"manual_entries,omitempty"`
}

type PriceOverride struct {
	ContractRowID int64   `json:"contract_row_id"`
	UnitPrice     float64 `json:"unit_price"`
}

type WeightOverride struct {
	ContractRowID    int64   `json:"contract_row_id"`
	WeightMultiplier float64 `json:"weight_multiplier"`
}

type ManualEntry struct {
	Label            string   `json:"label"`
	Region           string   `json:"region"`
	SupplierRegion   string   `json:"supplier_region"`
	UnitPrice        float64  `json:"unit_price"`
	VATPercent       *float64 `json:"vat_percent,omitempty"`
	WeightMultiplier float64  `json:"weight_multiplier"`
	Similarity       float64  `json:"similarity"`
}

type CalculateResponse struct {
	Selected           SelectedCTE      `json:"selected"`
	Parameters         EffectiveParams  `json:"parameters"`
	Summary            SummaryBlock     `json:"summary"`
	TopRecommendations []Recommendation `json:"top_recommendations"`
	Results            []AnalogResult   `json:"results"`
	DocumentResults    []AnalogResult   `json:"document_results"`
	Steps              []ExplainStep    `json:"steps"`
}

type VATInfo struct {
	Label    string  `json:"label"`
	Rate     float64 `json:"rate"`
	Included bool    `json:"included"`
	Source   string  `json:"source,omitempty"`
}

type SelectedCTE struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	Category     string            `json:"category"`
	Manufacturer string            `json:"manufacturer"`
	Attributes   map[string]string `json:"attributes"`
	VAT          VATInfo           `json:"vat"`
}

type EffectiveParams struct {
	Region                     string  `json:"region"`
	MonthsBack                 int     `json:"months_back"`
	TimeDecay                  float64 `json:"time_decay"`
	TimeWeightMode             string  `json:"time_weight_mode,omitempty"`
	SameRegionWeight           float64 `json:"same_region_weight"`
	OtherRegionWeight          float64 `json:"other_region_weight"`
	SettingsMode               string  `json:"settings_mode,omitempty"`
	TimeImportanceLabel        string  `json:"time_importance_label,omitempty"`
	SameRegionImportanceLabel  string  `json:"same_region_importance_label,omitempty"`
	OtherRegionImportanceLabel string  `json:"other_region_importance_label,omitempty"`
	MaxResults                 int     `json:"max_results"`
}

type SummaryBlock struct {
	RawContracts            int            `json:"raw_contracts"`
	ValidContracts          int            `json:"valid_contracts"`
	DocumentContracts       int            `json:"document_contracts"`
	ExcludedOutliers        int            `json:"excluded_outliers"`
	ManualExclusions        int            `json:"manual_exclusions"`
	ManualEntries           int            `json:"manual_entries"`
	PriceRangeMin           float64        `json:"price_range_min"`
	PriceRangeMax           float64        `json:"price_range_max"`
	PriceRangeMinNoVAT      float64        `json:"price_range_min_no_vat"`
	PriceRangeMaxNoVAT      float64        `json:"price_range_max_no_vat"`
	NMCKWeightedMean        float64        `json:"nmck_weighted_mean"`
	NMCKWeightedMedian      float64        `json:"nmck_weighted_median"`
	NMCKWeightedMeanNoVAT   float64        `json:"nmck_weighted_mean_no_vat"`
	NMCKWeightedMedianNoVAT float64        `json:"nmck_weighted_median_no_vat"`
	VAT                     VATInfo        `json:"vat"`
	FallbackToAllRegion     bool           `json:"fallback_to_all_region"`
	RegionScope             string         `json:"region_scope,omitempty"`
	UsedRegionTiers         []string       `json:"used_region_tiers,omitempty"`
	RegionCounts            map[string]int `json:"region_counts,omitempty"`
	ReferenceDate           string         `json:"reference_date"`
	WindowHint              WindowHint     `json:"window_hint"`
	Warnings                []string       `json:"warnings"`
}

type WindowExpansionOption struct {
	Label         string `json:"label"`
	MonthsBack    int    `json:"months_back"`
	ContractCount int    `json:"contract_count"`
}

type WindowHint struct {
	RequestedMonthsBack int                     `json:"requested_months_back"`
	RequestedCount      int                     `json:"requested_count"`
	AppliedMonthsBack   int                     `json:"applied_months_back,omitempty"`
	AppliedCount        int                     `json:"applied_count,omitempty"`
	MinRequired         int                     `json:"min_required"`
	NeedsExpansion      bool                    `json:"needs_expansion"`
	ExpandedAutomatically bool                  `json:"expanded_automatically,omitempty"`
	Options             []WindowExpansionOption `json:"options,omitempty"`
}

type Recommendation struct {
	CTEID              int64   `json:"cte_id"`
	Name               string  `json:"name"`
	Manufacturer       string  `json:"manufacturer"`
	Region             string  `json:"region"`
	LatestPrice        float64 `json:"latest_price"`
	LatestPriceNoVAT   float64 `json:"latest_price_no_vat"`
	WeightedPrice      float64 `json:"weighted_price"`
	WeightedPriceNoVAT float64 `json:"weighted_price_no_vat"`
	ContractCount      int     `json:"contract_count"`
	Similarity         float64 `json:"similarity"`
	VAT                VATInfo `json:"vat"`
}

type AnalogResult struct {
	ID                int64     `json:"id"`
	ContractID        int64     `json:"contract_id"`
	CTEID             int64     `json:"cte_id"`
	CTEName           string    `json:"cte_name"`
	Manufacturer      string    `json:"manufacturer"`
	Region            string    `json:"region"`
	SupplierRegion    string    `json:"supplier_region"`
	RegionTier        string    `json:"region_tier,omitempty"`
	UnitPrice         float64   `json:"unit_price"`
	UnitPriceNoVAT    float64   `json:"unit_price_no_vat"`
	OriginalUnitPrice float64   `json:"original_unit_price"`
	Quantity          float64   `json:"quantity"`
	Method            string    `json:"method"`
	ContractDate      time.Time `json:"contract_date"`
	Similarity        float64   `json:"similarity"`
	TimeWeight        float64   `json:"time_weight"`
	RegionWeight      float64   `json:"region_weight"`
	FinalWeight       float64   `json:"final_weight"`
	WeightMultiplier  float64   `json:"weight_multiplier"`
	Manual            bool      `json:"manual"`
	SourceLabel       string    `json:"source_label"`
	PriceOverridden   bool      `json:"price_overridden"`
	Outlier           bool      `json:"outlier"`
	VAT               VATInfo   `json:"vat"`
}

type ExplainStep struct {
	Title   string            `json:"title"`
	Details string            `json:"details"`
	Metrics map[string]string `json:"metrics,omitempty"`
}

type BatchCalculateRequest struct {
	BatchName                  string             `json:"batch_name"`
	Region                     string             `json:"region"`
	MonthsBack                 int                `json:"months_back"`
	TimeDecay                  float64            `json:"time_decay"`
	TimeWeightMode             string             `json:"time_weight_mode,omitempty"`
	SameRegionWeight           float64            `json:"same_region_weight"`
	OtherRegionWeight          float64            `json:"other_region_weight"`
	SettingsMode               string             `json:"settings_mode,omitempty"`
	TimeImportanceLabel        string             `json:"time_importance_label,omitempty"`
	SameRegionImportanceLabel  string             `json:"same_region_importance_label,omitempty"`
	OtherRegionImportanceLabel string             `json:"other_region_importance_label,omitempty"`
	MaxResults                 int                `json:"max_results"`
	Items                      []BatchItemRequest `json:"items"`
}

type BatchItemRequest struct {
	ItemID                     string           `json:"item_id"`
	CTEID                      int64            `json:"cte_id"`
	Name                       string           `json:"name,omitempty"`
	Category                   string           `json:"category,omitempty"`
	Manufacturer               string           `json:"manufacturer,omitempty"`
	Quantity                   float64          `json:"quantity"`
	Region                     string           `json:"region,omitempty"`
	MonthsBack                 int              `json:"months_back,omitempty"`
	TimeDecay                  float64          `json:"time_decay,omitempty"`
	TimeWeightMode             string           `json:"time_weight_mode,omitempty"`
	SameRegionWeight           float64          `json:"same_region_weight,omitempty"`
	OtherRegionWeight          float64          `json:"other_region_weight,omitempty"`
	SettingsMode               string           `json:"settings_mode,omitempty"`
	TimeImportanceLabel        string           `json:"time_importance_label,omitempty"`
	SameRegionImportanceLabel  string           `json:"same_region_importance_label,omitempty"`
	OtherRegionImportanceLabel string           `json:"other_region_importance_label,omitempty"`
	MaxResults                 int              `json:"max_results,omitempty"`
	ExcludedIDs                []int64          `json:"excluded_ids,omitempty"`
	PriceOverrides             []PriceOverride  `json:"price_overrides,omitempty"`
	WeightOverrides            []WeightOverride `json:"weight_overrides,omitempty"`
	ManualEntries              []ManualEntry    `json:"manual_entries,omitempty"`
}

type BatchCalculateResponse struct {
	Summary    BatchSummary      `json:"summary"`
	Parameters EffectiveParams   `json:"parameters"`
	Items      []BatchItemResult `json:"items"`
}

type BatchItemResult struct {
	ItemID         string            `json:"item_id"`
	Quantity       float64           `json:"quantity"`
	LineTotal      float64           `json:"line_total"`
	LineTotalNoVAT float64           `json:"line_total_no_vat"`
	Result         CalculateResponse `json:"result"`
}

type BatchSummary struct {
	BatchName              string   `json:"batch_name"`
	ItemCount              int      `json:"item_count"`
	GrandTotal             float64  `json:"grand_total"`
	GrandTotalNoVAT        float64  `json:"grand_total_no_vat"`
	TotalDocumentContracts int      `json:"total_document_contracts"`
	Warnings               []string `json:"warnings,omitempty"`
}

type bootstrapResponse struct {
	Regions         []string         `json:"regions"`
	Imported        bool             `json:"imported"`
	CTECount        int              `json:"cte_count"`
	ContractCount   int              `json:"contract_count"`
	RecentDocuments []RecentDocument `json:"recent_documents"`
}
