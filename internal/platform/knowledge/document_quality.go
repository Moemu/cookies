package knowledge

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const documentQualitySummaryVersion = "document-quality-summary/v1"

type DocumentQualitySignal struct {
	Code     string  `json:"code"`
	Severity string  `json:"severity"`
	Message  string  `json:"message"`
	Observed float64 `json:"observed"`
}

// DocumentQualitySummary contains routing signals, not a claim about factual
// or OCR accuracy. Page-level truth requires a labelled evaluation set and is
// deliberately deferred to the visual-fallback phase.
type DocumentQualitySummary struct {
	ContractVersion           string                  `json:"contract_version"`
	ScoringKind               string                  `json:"scoring_kind"`
	CharacterCount            int                     `json:"character_count"`
	LineCount                 int                     `json:"line_count"`
	CharactersPerPage         *float64                `json:"characters_per_page"`
	ReplacementCharacterRate  float64                 `json:"replacement_character_rate"`
	ControlCharacterRate      float64                 `json:"control_character_rate"`
	MeaningfulCharacterRate   float64                 `json:"meaningful_character_rate"`
	ShortLineRate             float64                 `json:"short_line_rate"`
	LocatorCoverage           float64                 `json:"locator_coverage"`
	MetadataImageSignals      int                     `json:"metadata_image_signals"`
	MetadataTableSignals      int                     `json:"metadata_table_signals"`
	MetadataImageSignalRatio  *float64                `json:"metadata_image_signal_ratio"`
	MetadataTableSignalRatio  *float64                `json:"metadata_table_signal_ratio"`
	EmptyPages                *int                    `json:"empty_pages"`
	EmptyPageRatio            *float64                `json:"empty_page_ratio"`
	ReadingOrderSignal        string                  `json:"reading_order_signal"`
	ShadowFallbackRecommended bool                    `json:"shadow_fallback_recommended"`
	Signals                   []DocumentQualitySignal `json:"signals"`
}

type documentQualityResult struct {
	Score          float64
	Tier           string
	FallbackReason string
	PreviewStatus  string
	TotalPages     *int
	Summary        DocumentQualitySummary
}

func evaluateParsedDocumentQuality(parsed ParsedDocument, chunks []Chunk) documentQualityResult {
	text := strings.ReplaceAll(parsed.Text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	characterCount := 0
	replacementCount := 0
	controlCount := 0
	meaningfulCount := 0
	shortLines := 0
	nonEmptyLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmptyLines++
			if utf8.RuneCountInString(trimmed) <= 3 {
				shortLines++
			}
		}
		for _, character := range line {
			if unicode.IsSpace(character) {
				continue
			}
			characterCount++
			if character == unicode.ReplacementChar {
				replacementCount++
			}
			if unicode.IsControl(character) {
				controlCount++
			}
			if unicode.IsLetter(character) || unicode.IsDigit(character) {
				meaningfulCount++
			}
		}
	}
	replacementRate := ratio(replacementCount, characterCount)
	controlRate := ratio(controlCount, characterCount)
	meaningfulRate := ratio(meaningfulCount, characterCount)
	shortLineRate := ratio(shortLines, nonEmptyLines)
	locatorCount := 0
	for _, chunk := range chunks {
		if chunk.StartLine > 0 && chunk.EndLine >= chunk.StartLine && len(chunk.Locator) > 0 {
			locatorCount++
		}
	}
	locatorCoverage := ratio(locatorCount, len(chunks))
	metadataSignals := qualityMetadataSignals(parsed.Metadata)
	totalPages := metadataSignals.TotalPages
	imageSignals := metadataSignals.ImageSignals
	tableSignals := metadataSignals.TableSignals
	emptyPages := metadataSignals.EmptyPages
	imageSignalRatio := metadataPageRatio(imageSignals, totalPages)
	tableSignalRatio := metadataPageRatio(tableSignals, totalPages)
	emptyPageRatio := metadataPageRatioValue(emptyPages, totalPages)
	var charactersPerPage *float64
	if totalPages != nil && *totalPages > 0 {
		value := float64(characterCount) / float64(*totalPages)
		charactersPerPage = &value
	}

	score := 1.0
	signals := make([]DocumentQualitySignal, 0, 6)
	addSignal := func(code, severity, message string, observed, penalty float64) {
		score -= penalty
		signals = append(signals, DocumentQualitySignal{Code: code, Severity: severity, Message: message, Observed: observed})
	}
	if characterCount < 80 {
		penalty := .65
		if parsed.ParserCode == "native" {
			penalty = .25
		}
		addSignal("very_low_text_volume", "high", "Very little text was extracted; the document may be scanned, image-led, or empty.", float64(characterCount), penalty)
	} else if characterCount < 300 {
		addSignal("low_text_volume", "medium", "Little text was extracted; check whether important content is missing.", float64(characterCount), .30)
	}
	if replacementRate > .01 {
		addSignal("replacement_character_rate", "high", "Many replacement characters suggest broken font mapping or text encoding.", replacementRate, .45)
	} else if replacementRate > .001 {
		addSignal("replacement_character_rate", "medium", "Some replacement characters were detected.", replacementRate, .18)
	}
	if controlRate > .005 {
		addSignal("control_character_rate", "medium", "Excess control characters may indicate an unstable reading order.", controlRate, .22)
	}
	if meaningfulRate < .45 {
		addSignal("meaningful_character_rate", "high", "Recognizable letters and digits are sparse; the output may contain noise or mojibake.", meaningfulRate, .35)
	} else if meaningfulRate < .65 {
		addSignal("meaningful_character_rate", "medium", "Text is mixed with a high proportion of layout symbols.", meaningfulRate, .14)
	}
	if charactersPerPage != nil {
		if *charactersPerPage < 40 {
			addSignal("page_character_density", "high", "Very little text was extracted per page; images may carry the main information.", *charactersPerPage, .35)
		} else if *charactersPerPage < 120 {
			addSignal("page_character_density", "medium", "The extracted text density per page is low.", *charactersPerPage, .15)
		}
	}
	if len(chunks) == 0 || locatorCoverage < .95 {
		addSignal("locator_coverage", "high", "Some extracted content lacks stable source locators.", locatorCoverage, .28)
	}
	if emptyPages != nil && *emptyPages > 0 {
		observed := float64(*emptyPages)
		if emptyPageRatio != nil {
			observed = *emptyPageRatio
		}
		addSignal("blank_pages", "medium", "One or more pages appear blank in the parser metadata.", observed, .10)
	}
	readingOrderSignal := "not_assessed"
	if nonEmptyLines >= 10 {
		readingOrderSignal = "acceptable"
		if shortLineRate > .7 {
			readingOrderSignal = "suspicious"
			addSignal("fragmented_reading_order", "medium", "A high short-line ratio suggests fragmented reading order.", shortLineRate, .16)
		}
	}
	score = math.Max(0, math.Min(1, score))
	tier := "low"
	if score >= .82 {
		tier = "high"
	} else if score >= .58 {
		tier = "medium"
	}
	sort.SliceStable(signals, func(left, right int) bool {
		return qualitySeverityRank(signals[left].Severity) > qualitySeverityRank(signals[right].Severity)
	})
	fallbackReason := ""
	if tier == "low" {
		if len(signals) > 0 {
			fallbackReason = signals[0].Message
		} else {
			fallbackReason = "Text quality signals are low; review the preview before considering visual parsing."
		}
	}
	previewStatus := "ready"
	if tier == "low" {
		previewStatus = "partial"
	}
	return documentQualityResult{
		Score: score, Tier: tier, FallbackReason: fallbackReason, PreviewStatus: previewStatus,
		TotalPages: totalPages,
		Summary: DocumentQualitySummary{
			ContractVersion: documentQualitySummaryVersion, ScoringKind: "routing_signal_not_accuracy",
			CharacterCount: characterCount, LineCount: nonEmptyLines, CharactersPerPage: charactersPerPage,
			ReplacementCharacterRate: replacementRate, ControlCharacterRate: controlRate,
			MeaningfulCharacterRate: meaningfulRate, ShortLineRate: shortLineRate,
			LocatorCoverage: locatorCoverage, MetadataImageSignals: imageSignals,
			MetadataTableSignals: tableSignals, MetadataImageSignalRatio: imageSignalRatio,
			MetadataTableSignalRatio: tableSignalRatio, EmptyPages: emptyPages, EmptyPageRatio: emptyPageRatio,
			ReadingOrderSignal:        readingOrderSignal,
			ShadowFallbackRecommended: tier == "low", Signals: signals,
		},
	}
}

func ratio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func qualitySeverityRank(value string) int {
	switch value {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

type documentMetadataQualitySignals struct {
	TotalPages   *int
	ImageSignals int
	TableSignals int
	EmptyPages   *int
}

func qualityMetadataSignals(raw json.RawMessage) documentMetadataQualitySignals {
	if len(raw) == 0 {
		return documentMetadataQualitySignals{}
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return documentMetadataQualitySignals{}
	}
	pageCount := 0
	imageSignals := 0
	tableSignals := 0
	emptyPageCount := 0
	emptyPagesObserved := false
	var walk func(any, string)
	walk = func(current any, key string) {
		normalizedKey := strings.ToLower(key)
		if metadataImageSignalKey(normalizedKey) {
			if count, ok := metadataOccurrenceCount(current); ok {
				imageSignals += count
			}
		}
		if metadataTableSignalKey(normalizedKey) {
			if count, ok := metadataOccurrenceCount(current); ok {
				tableSignals += count
			}
		}
		if metadataEmptyPageKey(normalizedKey) {
			if candidate, ok := nonNegativeMetadataInteger(current); ok {
				emptyPagesObserved = true
				if candidate > emptyPageCount {
					emptyPageCount = candidate
				}
			}
		}
		if !metadataEmptyPageKey(normalizedKey) && strings.Contains(normalizedKey, "page") &&
			(strings.Contains(normalizedKey, "count") || strings.Contains(normalizedKey, "pages")) {
			if candidate, ok := positiveMetadataInteger(current); ok && candidate > pageCount && candidate <= 100_000 {
				pageCount = candidate
			}
		}
		switch typed := current.(type) {
		case map[string]any:
			for childKey, child := range typed {
				walk(child, childKey)
			}
		case []any:
			for _, child := range typed {
				walk(child, "")
			}
		}
	}
	walk(value, "")
	result := documentMetadataQualitySignals{ImageSignals: imageSignals, TableSignals: tableSignals}
	if emptyPagesObserved {
		result.EmptyPages = &emptyPageCount
	}
	if pageCount == 0 {
		return result
	}
	result.TotalPages = &pageCount
	return result
}

func metadataImageSignalKey(key string) bool {
	if key == "" || strings.Contains(key, "width") || strings.Contains(key, "height") || strings.Contains(key, "dimension") {
		return false
	}
	return strings.Contains(key, "image") || strings.Contains(key, "embeddedresource")
}

func metadataTableSignalKey(key string) bool {
	return key != "" && strings.Contains(key, "table") && !strings.Contains(key, "width") && !strings.Contains(key, "height")
}

func metadataEmptyPageKey(key string) bool {
	return strings.Contains(key, "page") && (strings.Contains(key, "empty") || strings.Contains(key, "blank"))
}

func metadataOccurrenceCount(value any) (int, bool) {
	if count, ok := nonNegativeMetadataInteger(value); ok {
		return count, true
	}
	switch typed := value.(type) {
	case bool:
		if typed {
			return 1, true
		}
		return 0, true
	case []any:
		return len(typed), true
	case string:
		if strings.TrimSpace(typed) != "" {
			return 1, true
		}
	}
	return 0, false
}

func metadataPageRatio(count int, totalPages *int) *float64 {
	if totalPages == nil || *totalPages < 1 {
		return nil
	}
	value := math.Min(1, float64(count)/float64(*totalPages))
	return &value
}

func metadataPageRatioValue(count, totalPages *int) *float64 {
	if count == nil {
		return nil
	}
	return metadataPageRatio(*count, totalPages)
}

func positiveMetadataInteger(value any) (int, bool) {
	parsed, ok := nonNegativeMetadataInteger(value)
	return parsed, ok && parsed > 0
}

func nonNegativeMetadataInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), typed >= 0 && typed == math.Trunc(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil && parsed >= 0
	case json.Number:
		parsed, err := strconv.Atoi(string(typed))
		return parsed, err == nil && parsed >= 0
	default:
		return 0, false
	}
}
