package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	DocumentVisionEvaluationDatasetContractVersion = "document-vision-evaluation-dataset/v1"
	DocumentVisionEvaluationReportContractVersion  = "document-vision-evaluation-report/v1"
	maxDocumentVisionEvaluationCases               = 500
	maxDocumentVisionEvaluationTextBytes           = 2 * 1024 * 1024
	minDocumentVisionCasesPerCategory              = 3
)

var documentVisionEvaluationCategories = map[string]struct{}{
	"text_pdf": {}, "scanned_pdf": {}, "two_column": {}, "table_dense": {},
	"chinese_ppt": {}, "broken_font_map": {}, "header_footer_noise": {}, "image_led": {},
}

var documentVisionReviewOrders = map[string]struct{}{
	"baseline_first": {}, "hybrid_first": {},
}

type DocumentVisionEvaluationDataset struct {
	ContractVersion        string                         `json:"contract_version"`
	DatasetID              string                         `json:"dataset_id"`
	LabelPolicyVersion     string                         `json:"label_policy_version"`
	RedactionPolicyVersion string                         `json:"redaction_policy_version"`
	CostPolicyVersion      string                         `json:"cost_policy_version"`
	CollectedAt            string                         `json:"collected_at"`
	Deidentified           bool                           `json:"deidentified"`
	Cases                  []DocumentVisionEvaluationCase `json:"cases"`
}

type DocumentVisionEvaluationCase struct {
	ID                    string                            `json:"id"`
	Category              string                            `json:"category"`
	SourceSHA256          string                            `json:"source_sha256"`
	SourceMIMEType        string                            `json:"source_mime_type"`
	PageNumbers           []int                             `json:"page_numbers"`
	GoldMarkdown          string                            `json:"gold_markdown"`
	TextBaselineMarkdown  string                            `json:"text_baseline_markdown"`
	HybridMarkdown        string                            `json:"hybrid_markdown"`
	BaselineParserCode    string                            `json:"baseline_parser_code"`
	BaselineParserVersion string                            `json:"baseline_parser_version"`
	HybridParserCode      string                            `json:"hybrid_parser_code"`
	HybridParserVersion   string                            `json:"hybrid_parser_version"`
	HybridModelAlias      string                            `json:"hybrid_model_alias"`
	HybridRouteRevisionID string                            `json:"hybrid_route_revision_id"`
	HybridPromptVersion   string                            `json:"hybrid_prompt_version"`
	ConverterCode         string                            `json:"converter_code,omitempty"`
	ConverterVersion      string                            `json:"converter_version,omitempty"`
	OutputsBlinded        bool                              `json:"outputs_blinded"`
	Reviews               []DocumentVisionReviewMeasurement `json:"reviews"`
	Adjudicated           bool                              `json:"adjudicated"`
	AdjudicatorID         string                            `json:"adjudicator_id"`
	BaselineLatencyMS     int64                             `json:"baseline_latency_ms"`
	HybridLatencyMS       int64                             `json:"hybrid_latency_ms"`
	HybridBillablePages   int                               `json:"hybrid_billable_pages"`
	HybridCostMilliCNY    int64                             `json:"hybrid_cost_millicny"`
}

type DocumentVisionReviewMeasurement struct {
	BlindLabelID                  string `json:"blind_label_id"`
	ReviewerID                    string `json:"reviewer_id"`
	ReviewOrder                   string `json:"review_order"`
	BaselineCorrections           int    `json:"baseline_corrections"`
	HybridCorrections             int    `json:"hybrid_corrections"`
	BaselineReviewAndCorrectionMS int64  `json:"baseline_review_and_correction_ms"`
	HybridReviewAndCorrectionMS   int64  `json:"hybrid_review_and_correction_ms"`
}

type DocumentVisionEvaluationReport struct {
	ContractVersion                string                    `json:"contract_version"`
	DatasetID                      string                    `json:"dataset_id"`
	DatasetSHA256                  string                    `json:"dataset_sha256"`
	LabelPolicyVersion             string                    `json:"label_policy_version"`
	CostPolicyVersion              string                    `json:"cost_policy_version"`
	CaseCount                      int                       `json:"case_count"`
	ReviewCount                    int                       `json:"review_count"`
	CategoryCount                  int                       `json:"category_count"`
	CasesByCategory                map[string]int            `json:"cases_by_category"`
	ReviewOrdersByCategory         map[string]map[string]int `json:"review_orders_by_category"`
	MeanBaselineQuality            float64                   `json:"mean_baseline_quality"`
	MeanHybridQuality              float64                   `json:"mean_hybrid_quality"`
	MeanQualityGain                float64                   `json:"mean_quality_gain"`
	WorstCaseRegression            float64                   `json:"worst_case_regression"`
	CorrectionCountReduction       float64                   `json:"correction_count_reduction"`
	TotalBaselineCorrectionTimeMS  int64                     `json:"total_baseline_correction_time_ms"`
	TotalHybridCorrectionTimeMS    int64                     `json:"total_hybrid_correction_time_ms"`
	TotalCorrectionTimeSavedMS     int64                     `json:"total_correction_time_saved_ms"`
	MeanBaselineCorrectionTimeMS   int64                     `json:"mean_baseline_correction_time_ms"`
	MeanHybridCorrectionTimeMS     int64                     `json:"mean_hybrid_correction_time_ms"`
	MedianBaselineCorrectionTimeMS int64                     `json:"median_baseline_correction_time_ms"`
	MedianHybridCorrectionTimeMS   int64                     `json:"median_hybrid_correction_time_ms"`
	CorrectionTimeReduction        float64                   `json:"correction_time_reduction"`
	MeanBaselineLatencyMS          int64                     `json:"mean_baseline_latency_ms"`
	MeanHybridLatencyMS            int64                     `json:"mean_hybrid_latency_ms"`
	TotalHybridBillablePages       int                       `json:"total_hybrid_billable_pages"`
	TotalHybridCostMilliCNY        int64                     `json:"total_hybrid_cost_millicny"`
	AutoEnableAllowed              bool                      `json:"auto_enable_allowed"`
	Blockers                       []string                  `json:"blockers"`
}

// EvaluateDocumentVision compares blinded, adjudicated outputs from the same
// source pages. It is intentionally conservative: incomplete evidence is an
// error, while valid but statistically thin or weak evidence becomes a rollout
// blocker. The report measures correction time; it does not by itself establish
// a causal product-wide time-saving claim.
func EvaluateDocumentVision(dataset DocumentVisionEvaluationDataset) (DocumentVisionEvaluationReport, error) {
	report := DocumentVisionEvaluationReport{
		ContractVersion:        DocumentVisionEvaluationReportContractVersion,
		DatasetID:              dataset.DatasetID,
		LabelPolicyVersion:     dataset.LabelPolicyVersion,
		CostPolicyVersion:      dataset.CostPolicyVersion,
		CaseCount:              len(dataset.Cases),
		CasesByCategory:        map[string]int{},
		ReviewOrdersByCategory: map[string]map[string]int{},
		Blockers:               []string{},
	}
	if err := validateDocumentVisionEvaluationDataset(dataset); err != nil {
		return report, err
	}
	canonical, err := json.Marshal(dataset)
	if err != nil {
		return report, fmt.Errorf("encode document vision evaluation dataset: %w", err)
	}
	digest := sha256.Sum256(canonical)
	report.DatasetSHA256 = hex.EncodeToString(digest[:])

	baselineCorrections, hybridCorrections := 0, 0
	var baselineQuality, hybridQuality float64
	var baselineLatency, hybridLatency int64
	baselineCorrectionTimes := make([]int64, 0, len(dataset.Cases))
	hybridCorrectionTimes := make([]int64, 0, len(dataset.Cases))
	worstRegression := 0.0
	for _, item := range dataset.Cases {
		if _, ok := report.ReviewOrdersByCategory[item.Category]; !ok {
			report.ReviewOrdersByCategory[item.Category] = map[string]int{}
		}
		report.CasesByCategory[item.Category]++
		baseline := normalizedRuneSimilarity(item.GoldMarkdown, item.TextBaselineMarkdown)
		hybrid := normalizedRuneSimilarity(item.GoldMarkdown, item.HybridMarkdown)
		baselineQuality += baseline
		hybridQuality += hybrid
		if regression := baseline - hybrid; regression > worstRegression {
			worstRegression = regression
		}
		for _, review := range item.Reviews {
			report.ReviewCount++
			report.ReviewOrdersByCategory[item.Category][review.ReviewOrder]++
			baselineCorrections += review.BaselineCorrections
			hybridCorrections += review.HybridCorrections
			report.TotalBaselineCorrectionTimeMS += review.BaselineReviewAndCorrectionMS
			report.TotalHybridCorrectionTimeMS += review.HybridReviewAndCorrectionMS
			baselineCorrectionTimes = append(baselineCorrectionTimes, review.BaselineReviewAndCorrectionMS)
			hybridCorrectionTimes = append(hybridCorrectionTimes, review.HybridReviewAndCorrectionMS)
		}
		baselineLatency += item.BaselineLatencyMS
		hybridLatency += item.HybridLatencyMS
		report.TotalHybridBillablePages += item.HybridBillablePages
		report.TotalHybridCostMilliCNY += item.HybridCostMilliCNY
	}
	report.CategoryCount = len(report.CasesByCategory)
	report.MeanBaselineQuality = roundMetric(baselineQuality / float64(len(dataset.Cases)))
	report.MeanHybridQuality = roundMetric(hybridQuality / float64(len(dataset.Cases)))
	report.MeanQualityGain = roundMetric(report.MeanHybridQuality - report.MeanBaselineQuality)
	report.WorstCaseRegression = roundMetric(worstRegression)
	if baselineCorrections > 0 {
		report.CorrectionCountReduction = roundMetric(float64(baselineCorrections-hybridCorrections) / float64(baselineCorrections))
	}
	report.TotalCorrectionTimeSavedMS = report.TotalBaselineCorrectionTimeMS - report.TotalHybridCorrectionTimeMS
	report.MeanBaselineCorrectionTimeMS = report.TotalBaselineCorrectionTimeMS / int64(report.ReviewCount)
	report.MeanHybridCorrectionTimeMS = report.TotalHybridCorrectionTimeMS / int64(report.ReviewCount)
	report.MedianBaselineCorrectionTimeMS = medianInt64(baselineCorrectionTimes)
	report.MedianHybridCorrectionTimeMS = medianInt64(hybridCorrectionTimes)
	report.CorrectionTimeReduction = roundMetric(float64(report.TotalCorrectionTimeSavedMS) / float64(report.TotalBaselineCorrectionTimeMS))
	report.MeanBaselineLatencyMS = baselineLatency / int64(len(dataset.Cases))
	report.MeanHybridLatencyMS = hybridLatency / int64(len(dataset.Cases))

	for category := range documentVisionEvaluationCategories {
		if report.CasesByCategory[category] < minDocumentVisionCasesPerCategory {
			report.Blockers = append(report.Blockers, "LABELLED_CATEGORY_COVERAGE_INSUFFICIENT")
			break
		}
	}
	for category := range documentVisionEvaluationCategories {
		orders := report.ReviewOrdersByCategory[category]
		if orders["baseline_first"] == 0 || orders["hybrid_first"] == 0 {
			report.Blockers = append(report.Blockers, "BLINDED_REVIEW_ORDER_NOT_COUNTERBALANCED")
			break
		}
	}
	if report.MeanQualityGain < .12 {
		report.Blockers = append(report.Blockers, "MEAN_QUALITY_GAIN_BELOW_THRESHOLD")
	}
	if report.WorstCaseRegression > .08 {
		report.Blockers = append(report.Blockers, "WORST_CASE_REGRESSION_ABOVE_THRESHOLD")
	}
	if report.CorrectionCountReduction < .30 {
		report.Blockers = append(report.Blockers, "HUMAN_CORRECTION_COUNT_REDUCTION_BELOW_THRESHOLD")
	}
	if report.CorrectionTimeReduction < .30 {
		report.Blockers = append(report.Blockers, "HUMAN_CORRECTION_TIME_REDUCTION_BELOW_THRESHOLD")
	}
	sort.Strings(report.Blockers)
	report.AutoEnableAllowed = len(report.Blockers) == 0
	return report, nil
}

func validateDocumentVisionEvaluationDataset(dataset DocumentVisionEvaluationDataset) error {
	if dataset.ContractVersion != DocumentVisionEvaluationDatasetContractVersion {
		return fmt.Errorf("document vision evaluation contract_version must be %q", DocumentVisionEvaluationDatasetContractVersion)
	}
	if !validEvaluationID(dataset.DatasetID) || !validBoundedValue(dataset.LabelPolicyVersion, 96) || !validBoundedValue(dataset.RedactionPolicyVersion, 96) || !validBoundedValue(dataset.CostPolicyVersion, 96) {
		return fmt.Errorf("document vision evaluation dataset identity and policy versions are required")
	}
	if _, err := time.Parse(time.RFC3339, dataset.CollectedAt); err != nil {
		return fmt.Errorf("document vision evaluation collected_at must be RFC3339")
	}
	if !dataset.Deidentified {
		return fmt.Errorf("document vision evaluation dataset must be deidentified")
	}
	if len(dataset.Cases) == 0 {
		return fmt.Errorf("document vision evaluation requires labelled cases")
	}
	if len(dataset.Cases) > maxDocumentVisionEvaluationCases {
		return fmt.Errorf("document vision evaluation exceeds the case limit")
	}
	seen := map[string]struct{}{}
	seenBlindLabels := map[string]struct{}{}
	seenSourcePages := map[string]struct{}{}
	sourceMIMETypes := map[string]string{}
	for _, item := range dataset.Cases {
		if err := validateDocumentVisionEvaluationCase(item, seen, seenBlindLabels, seenSourcePages, sourceMIMETypes); err != nil {
			return err
		}
		seen[item.ID] = struct{}{}
		for _, review := range item.Reviews {
			seenBlindLabels[review.BlindLabelID] = struct{}{}
		}
	}
	return nil
}

func validateDocumentVisionEvaluationCase(item DocumentVisionEvaluationCase, seen, seenBlindLabels, seenSourcePages map[string]struct{}, sourceMIMETypes map[string]string) error {
	if !validEvaluationID(item.ID) || strings.TrimSpace(item.GoldMarkdown) == "" || strings.TrimSpace(item.TextBaselineMarkdown) == "" || strings.TrimSpace(item.HybridMarkdown) == "" {
		return fmt.Errorf("evaluation case identity and all labelled outputs are required")
	}
	if _, duplicate := seen[item.ID]; duplicate {
		return fmt.Errorf("evaluation case %q is duplicated", item.ID)
	}
	if _, supported := documentVisionEvaluationCategories[item.Category]; !supported {
		return fmt.Errorf("evaluation category %q is unsupported", item.Category)
	}
	if len(item.GoldMarkdown) > maxDocumentVisionEvaluationTextBytes || len(item.TextBaselineMarkdown) > maxDocumentVisionEvaluationTextBytes || len(item.HybridMarkdown) > maxDocumentVisionEvaluationTextBytes {
		return fmt.Errorf("evaluation case %q contains an oversized labelled output", item.ID)
	}
	if !validSHA256(item.SourceSHA256) || !validEvaluationMIMEType(item.SourceMIMEType) {
		return fmt.Errorf("evaluation case %q requires valid source lineage", item.ID)
	}
	if err := validateEvaluationPages(item.ID, item.PageNumbers); err != nil {
		return err
	}
	if knownMIMEType, exists := sourceMIMETypes[item.SourceSHA256]; exists && knownMIMEType != item.SourceMIMEType {
		return fmt.Errorf("evaluation case %q changes the MIME type for an existing source SHA", item.ID)
	}
	for _, page := range item.PageNumbers {
		key := fmt.Sprintf("%s:%d", item.SourceSHA256, page)
		if _, duplicate := seenSourcePages[key]; duplicate {
			return fmt.Errorf("evaluation case %q reuses an already counted source page", item.ID)
		}
		seenSourcePages[key] = struct{}{}
	}
	sourceMIMETypes[item.SourceSHA256] = item.SourceMIMEType
	lineage := []string{item.BaselineParserCode, item.BaselineParserVersion, item.HybridParserCode, item.HybridParserVersion, item.HybridModelAlias, item.HybridRouteRevisionID, item.HybridPromptVersion}
	for _, value := range lineage {
		if !validBoundedValue(value, 160) {
			return fmt.Errorf("evaluation case %q requires complete parser, model, route and prompt lineage", item.ID)
		}
	}
	converterPresent := item.ConverterCode != "" || item.ConverterVersion != ""
	if converterPresent && (!validBoundedValue(item.ConverterCode, 160) || !validBoundedValue(item.ConverterVersion, 160)) {
		return fmt.Errorf("evaluation case %q converter code and version must be recorded together", item.ID)
	}
	if item.SourceMIMEType != "application/pdf" && !converterPresent {
		return fmt.Errorf("evaluation case %q requires presentation converter lineage", item.ID)
	}
	if !item.OutputsBlinded {
		return fmt.Errorf("evaluation case %q requires blinded outputs", item.ID)
	}
	if err := validateEvaluationReviews(item, seenBlindLabels); err != nil {
		return err
	}
	if item.BaselineLatencyMS <= 0 || item.HybridLatencyMS <= 0 || item.HybridBillablePages <= 0 || item.HybridBillablePages > 24 || item.HybridCostMilliCNY < 0 {
		return fmt.Errorf("evaluation case %q requires positive latency and billable pages and non-negative cost", item.ID)
	}
	return nil
}

func validateEvaluationPages(caseID string, pages []int) error {
	if len(pages) == 0 || len(pages) > 24 {
		return fmt.Errorf("evaluation case %q requires between 1 and 24 source pages", caseID)
	}
	previous := 0
	for _, page := range pages {
		if page <= previous {
			return fmt.Errorf("evaluation case %q page_numbers must be positive, unique and sorted", caseID)
		}
		previous = page
	}
	return nil
}

func validateEvaluationReviews(item DocumentVisionEvaluationCase, seenBlindLabels map[string]struct{}) error {
	if len(item.Reviews) < 2 || len(item.Reviews) > 8 || !item.Adjudicated || !validEvaluationID(item.AdjudicatorID) {
		return fmt.Errorf("evaluation case %q requires two or more blinded reviewers and recorded adjudication", item.ID)
	}
	seenReviewers := map[string]struct{}{}
	seenCaseBlindLabels := map[string]struct{}{}
	for _, review := range item.Reviews {
		reviewerID := strings.TrimSpace(review.ReviewerID)
		blindLabelID := strings.TrimSpace(review.BlindLabelID)
		if !validEvaluationID(reviewerID) || !validEvaluationID(blindLabelID) {
			return fmt.Errorf("evaluation case %q contains an empty reviewer or blind-label id", item.ID)
		}
		if _, duplicate := seenReviewers[reviewerID]; duplicate {
			return fmt.Errorf("evaluation case %q contains a duplicated reviewer id", item.ID)
		}
		if _, duplicate := seenBlindLabels[blindLabelID]; duplicate {
			return fmt.Errorf("evaluation case %q contains a reused blind-label id", item.ID)
		}
		if _, duplicate := seenCaseBlindLabels[blindLabelID]; duplicate {
			return fmt.Errorf("evaluation case %q contains a duplicated blind-label id", item.ID)
		}
		if _, supported := documentVisionReviewOrders[review.ReviewOrder]; !supported {
			return fmt.Errorf("evaluation case %q contains an invalid review order", item.ID)
		}
		if review.BaselineCorrections < 0 || review.HybridCorrections < 0 || review.BaselineReviewAndCorrectionMS <= 0 || review.HybridReviewAndCorrectionMS <= 0 {
			return fmt.Errorf("evaluation case %q requires positive review times and non-negative correction counts", item.ID)
		}
		seenReviewers[reviewerID] = struct{}{}
		seenCaseBlindLabels[blindLabelID] = struct{}{}
	}
	if _, selfAdjudicated := seenReviewers[item.AdjudicatorID]; selfAdjudicated {
		return fmt.Errorf("evaluation case %q requires an adjudicator independent from its reviewers", item.ID)
	}
	return nil
}

func validBoundedValue(value string, maximum int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(trimmed) <= maximum
}

func validEvaluationID(value string) bool {
	if len(value) == 0 || len(value) > 96 || !isASCIIAlphaNumeric(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !isASCIIAlphaNumeric(character) && character != '.' && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(character byte) bool {
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validEvaluationMIMEType(value string) bool {
	switch value {
	case "application/pdf", "application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return true
	default:
		return false
	}
}

func medianInt64(values []int64) int64 {
	ordered := append([]int64(nil), values...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}

func normalizedRuneSimilarity(expected, actual string) float64 {
	left := []rune(normalizeVisionEvaluationText(expected))
	right := []rune(normalizeVisionEvaluationText(actual))
	denominator := max(len(left), len(right))
	if denominator == 0 {
		return 1
	}
	if string(left) == string(right) {
		return 1
	}
	// Exact edit distance is useful for short labelled snippets but becomes
	// quadratic for real documents. Long cases use a bounded-memory bigram Dice
	// score so the offline evaluator cannot freeze on a large PDF page.
	if denominator > 2_000 {
		return runeBigramSimilarity(left, right)
	}
	return math.Max(0, 1-float64(runeEditDistance(left, right))/float64(denominator))
}

func runeBigramSimilarity(left, right []rune) float64 {
	if len(left) < 2 || len(right) < 2 {
		return 0
	}
	counts := make(map[[2]rune]int, len(left)-1)
	for index := 0; index < len(left)-1; index++ {
		counts[[2]rune{left[index], left[index+1]}]++
	}
	intersection := 0
	for index := 0; index < len(right)-1; index++ {
		key := [2]rune{right[index], right[index+1]}
		if counts[key] > 0 {
			counts[key]--
			intersection++
		}
	}
	return 2 * float64(intersection) / float64(len(left)+len(right)-2)
}

func normalizeVisionEvaluationText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func runeEditDistance(left, right []rune) int {
	if len(left) > len(right) {
		left, right = right, left
	}
	previous := make([]int, len(left)+1)
	for index := range previous {
		previous[index] = index
	}
	for rightIndex, rightRune := range right {
		current := make([]int, len(left)+1)
		current[0] = rightIndex + 1
		for leftIndex, leftRune := range left {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current[leftIndex+1] = min(current[leftIndex]+1, previous[leftIndex+1]+1, previous[leftIndex]+cost)
		}
		previous = current
	}
	return previous[len(left)]
}

func roundMetric(value float64) float64 {
	return math.Round(value*10_000) / 10_000
}
