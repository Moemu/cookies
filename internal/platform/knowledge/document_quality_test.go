package knowledge

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEvaluateParsedDocumentQualityKeepsHealthyTextUsable(t *testing.T) {
	parsed := ParsedDocument{
		Text:       strings.Repeat("Market evidence supports a measurable audience decision. ", 24),
		ParserCode: "tika", Metadata: json.RawMessage(`[{"xmpTPg:NPages":"2"}]`),
	}
	document := Document{ID: "doc_1", ContentSHA256: strings.Repeat("a", 64), UpdatedAt: time.Unix(1, 0)}
	quality := evaluateParsedDocumentQuality(parsed, chunksForParsedDocument(document, parsed))
	if quality.Tier == "low" || quality.Summary.ShadowFallbackRecommended {
		t.Fatalf("healthy parse should remain usable: %#v", quality)
	}
	if quality.TotalPages == nil || *quality.TotalPages != 2 || quality.Summary.ScoringKind != "routing_signal_not_accuracy" {
		t.Fatalf("metadata or scoring kind missing: %#v", quality)
	}
}

func TestEvaluateParsedDocumentQualityRoutesBrokenTextToShadowFallback(t *testing.T) {
	parsed := ParsedDocument{
		Text: strings.Repeat("\ufffd\ufffd\ufffd !!! ", 40), ParserCode: "tika",
		Metadata: json.RawMessage(`[{"xmpTPg:NPages":12,"embeddedResourceCount":4}]`),
	}
	document := Document{ID: "doc_2", ContentSHA256: strings.Repeat("b", 64), UpdatedAt: time.Unix(1, 0)}
	quality := evaluateParsedDocumentQuality(parsed, chunksForParsedDocument(document, parsed))
	if quality.Tier != "low" || !quality.Summary.ShadowFallbackRecommended || quality.PreviewStatus != "partial" {
		t.Fatalf("broken parse should become a manual visual candidate: %#v", quality)
	}
	if quality.FallbackReason == "" {
		t.Fatal("low-quality parse must explain the fallback signal")
	}
}

func TestEvaluateParsedDocumentQualityDoesNotRejectShortNativeBrief(t *testing.T) {
	parsed := ParsedDocument{Text: "Launch goal: improve qualified leads by 20%.", ParserCode: "native"}
	document := Document{ID: "doc_3", ContentSHA256: strings.Repeat("c", 64), UpdatedAt: time.Unix(1, 0)}
	quality := evaluateParsedDocumentQuality(parsed, chunksForParsedDocument(document, parsed))
	if quality.Tier == "low" {
		t.Fatalf("short native brief should stay usable: %#v", quality)
	}
}

func TestEvaluateParsedDocumentQualityExposesPageCompositionSignals(t *testing.T) {
	parsed := ParsedDocument{
		Text:       strings.Repeat("A traceable document section with stable reading order.\n", 30),
		ParserCode: "tika",
		Metadata:   json.RawMessage(`[{"xmpTPg:NPages":"4","embeddedResourceCount":3,"tableCount":"2","blankPageCount":1,"imageWidth":1920}]`),
	}
	document := Document{ID: "doc_4", ContentSHA256: strings.Repeat("d", 64), UpdatedAt: time.Unix(1, 0)}
	quality := evaluateParsedDocumentQuality(parsed, chunksForParsedDocument(document, parsed))
	if quality.TotalPages == nil || *quality.TotalPages != 4 {
		t.Fatalf("total pages = %#v", quality.TotalPages)
	}
	if quality.Summary.MetadataImageSignals != 3 || quality.Summary.MetadataTableSignals != 2 {
		t.Fatalf("composition signals = %#v", quality.Summary)
	}
	if quality.Summary.MetadataImageSignalRatio == nil || *quality.Summary.MetadataImageSignalRatio != .75 ||
		quality.Summary.MetadataTableSignalRatio == nil || *quality.Summary.MetadataTableSignalRatio != .5 {
		t.Fatalf("composition ratios = %#v", quality.Summary)
	}
	if quality.Summary.EmptyPages == nil || *quality.Summary.EmptyPages != 1 ||
		quality.Summary.EmptyPageRatio == nil || *quality.Summary.EmptyPageRatio != .25 {
		t.Fatalf("blank-page signals = %#v", quality.Summary)
	}
	if quality.Summary.MetadataImageSignals == 1920 {
		t.Fatal("image dimensions must not be counted as image occurrences")
	}
}

func TestQualityMetadataSignalsDoesNotTreatBlankPagesAsTotalPages(t *testing.T) {
	signals := qualityMetadataSignals(json.RawMessage(`{"empty_pages":2,"imageCount":0,"tableCount":0}`))
	if signals.TotalPages != nil {
		t.Fatalf("blank-page count became total pages: %#v", signals)
	}
	if signals.EmptyPages == nil || *signals.EmptyPages != 2 || signals.ImageSignals != 0 || signals.TableSignals != 0 {
		t.Fatalf("metadata signals = %#v", signals)
	}
}
