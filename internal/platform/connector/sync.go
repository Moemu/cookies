package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
)

type EvidenceCipher interface {
	Encrypt([]byte) ([]byte, string, error)
}

type ReaderFactory interface {
	Open(context.Context, SyncRequest) (oceanengine.Reader, func(), error)
}

type SyncWriter interface {
	StartSync(context.Context, SyncRun) (bool, error)
	CompleteSync(context.Context, string, string, string, time.Time) error
	AppendRaw(context.Context, RawSnapshot) (bool, error)
	AppendObject(context.Context, ObjectSnapshot) (bool, error)
	AppendConfiguration(context.Context, ConfigurationSnapshot) (bool, error)
	AppendChange(context.Context, ConfigurationChangeEvent) (bool, error)
	AppendMetric(context.Context, MetricWindow) (bool, error)
	AppendMaterialMetric(context.Context, MaterialMetricWindow) (bool, error)
	AppendConversionRevision(context.Context, ConversionRevision) (bool, error)
	AppendBinding(context.Context, MaterialBinding) (bool, error)
	AppendStatus(context.Context, PlatformStatusEvent) (bool, error)
	AppendDiagnosis(context.Context, PlatformDiagnosisSnapshot) (bool, error)
	LatestConfiguration(context.Context, string, string, string, string, time.Time) (ConfigurationSnapshot, bool, error)
	LatestMetric(context.Context, string, string, string, string, time.Time, time.Time, string, string, time.Time) (MetricWindow, int, bool, error)
}

type SyncRequest struct {
	OrganizationID string
	ProjectID      string
	AccountRef     string
	IdempotencyKey string
	WindowStart    time.Time
	WindowEnd      time.Time
	TimeZone       string
	Currency       string
}

type SyncResult struct {
	RunID       string `json:"run_id"`
	Replayed    bool   `json:"replayed"`
	ObjectCount int    `json:"object_count"`
	MetricCount int    `json:"metric_count"`
}

type Synchronizer struct {
	Writer    SyncWriter
	Readers   ReaderFactory
	Cipher    EvidenceCipher
	Now       func() time.Time
	PageLimit int
	MaxPages  int
}

func (s Synchronizer) Sync(ctx context.Context, request SyncRequest) (result SyncResult, resultErr error) {
	if s.Writer == nil || s.Readers == nil || s.Cipher == nil || request.OrganizationID == "" || request.ProjectID == "" || request.AccountRef == "" || request.IdempotencyKey == "" || !request.WindowEnd.After(request.WindowStart) {
		return result, ErrInvalidFact
	}
	if request.TimeZone == "" {
		request.TimeZone = "Asia/Shanghai"
	}
	if request.Currency == "" {
		request.Currency = "CNY"
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	runID := "sync_" + canonicalHash([]string{request.OrganizationID, request.ProjectID, request.AccountRef, request.IdempotencyKey})
	created, err := s.Writer.StartSync(ctx, SyncRun{ID: runID, OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, AccountRef: opaqueRef(request.AccountRef), StartedAt: now, Attempt: 1})
	if err != nil {
		return result, err
	}
	result.RunID = runID
	if !created {
		result.Replayed = true
		return result, nil
	}
	status, cursor := "failed", "start"
	defer func() {
		completeAt := time.Now().UTC()
		if s.Now != nil {
			completeAt = s.Now().UTC()
		}
		completeErr := s.Writer.CompleteSync(ctx, runID, cursor, status, completeAt)
		if resultErr == nil && completeErr != nil {
			resultErr = completeErr
		}
	}()
	reader, closeReader, err := s.Readers.Open(ctx, request)
	if err != nil {
		return result, err
	}
	if closeReader != nil {
		defer closeReader()
	}
	account, err := reader.AccountInfo(ctx)
	if err != nil {
		return result, err
	}
	rawID, accountObservedAt, err := s.storeRaw(ctx, request, runID, "account_info", map[string]any{"account": request.AccountRef}, account)
	if err != nil {
		return result, err
	}
	accountState := redactCanonical(account)
	accountHeader := s.header(request, runID, rawID, canonicalHash(accountState), accountObservedAt, accountObservedAt)
	if _, err = s.Writer.AppendObject(ctx, ObjectSnapshot{FactHeader: accountHeader, ID: "obj_" + canonicalHash([]string{rawID, "account"}), ObjectKind: "account", ObjectRef: opaqueRef(request.AccountRef), State: accountState}); err != nil {
		return result, err
	}
	result.ObjectCount++
	limit := s.PageLimit
	if limit < 1 {
		limit = 50
	}
	maxPages := s.MaxPages
	if maxPages < 1 {
		maxPages = 1000
	}
	promotions := []map[string]any{}
	seenParents := map[string]struct{}{}
	for page := 1; page <= maxPages; page++ {
		cursor = fmt.Sprintf("promotion_page:%d", page)
		payload, readErr := reader.ListPage(ctx, oceanengine.ListRequest{Start: request.WindowStart.Format("2006-01-02"), End: request.WindowEnd.Format("2006-01-02"), Page: page, Limit: limit})
		if readErr != nil {
			return result, readErr
		}
		pageRawID, pageObservedAt, storeErr := s.storeRaw(ctx, request, runID, "promotion_list", map[string]any{"page": page, "limit": limit, "start": request.WindowStart, "end": request.WindowEnd}, payload)
		if storeErr != nil {
			return result, storeErr
		}
		items, totalPages := promotionItems(payload)
		for _, item := range items {
			item["_evidence_ref"] = pageRawID
			item["_collected_at"] = pageObservedAt
			promotions = append(promotions, item)
		}
		if len(items) == 0 || totalPages == 0 || page >= totalPages {
			break
		}
	}
	for _, promotion := range promotions {
		promotionRef := firstString(promotion, "promotion_id", "promotionId", "id")
		if promotionRef == "" {
			continue
		}
		evidenceRef, _ := promotion["_evidence_ref"].(string)
		observedAt, _ := promotion["_collected_at"].(time.Time)
		delete(promotion, "_evidence_ref")
		delete(promotion, "_collected_at")
		state := redactCanonical(promotion)
		header := s.header(request, runID, evidenceRef, canonicalHash(state), observedAt, platformValidTime(promotion, observedAt))
		objectID := "obj_" + canonicalHash([]string{evidenceRef, promotionRef, header.PayloadHash})
		createdObject, appendObjectErr := s.Writer.AppendObject(ctx, ObjectSnapshot{FactHeader: header, ID: objectID, ObjectKind: "promotion", ObjectRef: opaqueRef(promotionRef), ParentRef: opaqueRef(firstString(promotion, "project_id", "projectId")), State: state})
		if appendObjectErr != nil {
			return result, appendObjectErr
		}
		if createdObject {
			result.ObjectCount++
		}
		for _, parent := range []struct{ kind, ref string }{{"project", firstString(promotion, "project_id", "projectId")}, {"campaign", firstString(promotion, "campaign_id", "campaignId")}} {
			if parent.ref == "" {
				continue
			}
			key := parent.kind + ":" + parent.ref
			if _, ok := seenParents[key]; ok {
				continue
			}
			seenParents[key] = struct{}{}
			parentState := map[string]any{"observed_from": "promotion_list"}
			parentHeader := s.header(request, runID, evidenceRef, canonicalHash(parentState), observedAt, observedAt)
			createdParent, parentErr := s.Writer.AppendObject(ctx, ObjectSnapshot{FactHeader: parentHeader, ID: "obj_" + canonicalHash([]string{evidenceRef, parent.kind, parent.ref}), ObjectKind: parent.kind, ObjectRef: opaqueRef(parent.ref), State: parentState})
			if parentErr != nil {
				return result, parentErr
			}
			if createdParent {
				result.ObjectCount++
			}
		}
		configuration, readErr := reader.PromotionConfiguration(ctx, promotionRef)
		if readErr != nil {
			return result, readErr
		}
		configurationRawID, configurationObservedAt, storeErr := s.storeRaw(ctx, request, runID, "promotion_configuration", map[string]any{"promotion_ref": opaqueRef(promotionRef)}, configuration)
		if storeErr != nil {
			return result, storeErr
		}
		values := redactCanonical(configuration)
		configHeader := s.header(request, runID, configurationRawID, canonicalHash(values), configurationObservedAt, platformValidTime(configuration, configurationObservedAt))
		config := ConfigurationSnapshot{FactHeader: configHeader, ID: "cfg_" + canonicalHash([]string{configurationRawID, promotionRef, configHeader.PayloadHash}), ObjectRef: opaqueRef(promotionRef), Values: values}
		previous, found, latestErr := s.Writer.LatestConfiguration(ctx, request.OrganizationID, request.ProjectID, config.SourceRef, config.ObjectRef, config.AvailableAt)
		if latestErr != nil {
			return result, latestErr
		}
		if _, err = s.Writer.AppendConfiguration(ctx, config); err != nil {
			return result, err
		}
		if found {
			for _, change := range diffConfigurations(previous, config) {
				if _, err = s.Writer.AppendChange(ctx, change); err != nil {
					return result, err
				}
			}
		}
		materials, readErr := reader.PromotionMaterials(ctx, promotionRef, true)
		if readErr != nil {
			return result, readErr
		}
		materialRawID, materialObservedAt, storeErr := s.storeRaw(ctx, request, runID, "promotion_materials", map[string]any{"promotion_ref": opaqueRef(promotionRef)}, materials)
		if storeErr != nil {
			return result, storeErr
		}
		for _, materialRef := range collectStringIDs(materials, "material_ids", "material_id") {
			materialState := map[string]any{"observed_from": "promotion_materials"}
			materialHeader := s.header(request, runID, materialRawID, canonicalHash(materialState), materialObservedAt, materialObservedAt)
			createdMaterial, materialErr := s.Writer.AppendObject(ctx, ObjectSnapshot{FactHeader: materialHeader, ID: "obj_" + canonicalHash([]string{materialRawID, "material", materialRef}), ObjectKind: "material", ObjectRef: opaqueRef(materialRef), ParentRef: opaqueRef(promotionRef), State: materialState})
			if materialErr != nil {
				return result, materialErr
			}
			if createdMaterial {
				result.ObjectCount++
			}
			bindingHeader := s.header(request, runID, materialRawID, canonicalHash([]string{materialRef, promotionRef, request.WindowStart.Format(time.RFC3339), request.WindowEnd.Format(time.RFC3339)}), materialObservedAt, request.WindowStart)
			bindingValidTo := request.WindowEnd
			bindingHeader.ValidTo = &bindingValidTo
			binding := MaterialBinding{FactHeader: bindingHeader, ID: "binding_" + canonicalHash([]string{materialRawID, materialRef, promotionRef}), MaterialRef: opaqueRef(materialRef), PromotionRef: opaqueRef(promotionRef)}
			if _, err = s.Writer.AppendBinding(ctx, binding); err != nil {
				return result, err
			}
		}
		attributes, readErr := reader.Attributes(ctx, []string{promotionRef}, runID)
		if readErr != nil {
			return result, readErr
		}
		attributeRawID, attributeObservedAt, storeErr := s.storeRaw(ctx, request, runID, "promotion_attributes", map[string]any{"promotion_ref": opaqueRef(promotionRef)}, attributes)
		if storeErr != nil {
			return result, storeErr
		}
		diagnosis := redactCanonical(attributes)
		diagnosisHeader := s.header(request, runID, attributeRawID, canonicalHash(diagnosis), attributeObservedAt, attributeObservedAt)
		if _, err = s.Writer.AppendDiagnosis(ctx, PlatformDiagnosisSnapshot{FactHeader: diagnosisHeader, ID: "diagnosis_" + canonicalHash([]string{attributeRawID, promotionRef}), ObjectRef: opaqueRef(promotionRef), EligibleAsPrelaunchFeature: false, Diagnosis: diagnosis}); err != nil {
			return result, err
		}
		statusValue := firstString(promotion, "status", "promotion_status", "delivery_status")
		if statusValue != "" {
			statusHeader := s.header(request, runID, evidenceRef, canonicalHash(statusValue), observedAt, observedAt)
			if _, err = s.Writer.AppendStatus(ctx, PlatformStatusEvent{FactHeader: statusHeader, ID: "status_" + canonicalHash([]string{evidenceRef, promotionRef, statusValue}), ObjectRef: opaqueRef(promotionRef), Status: statusValue}); err != nil {
				return result, err
			}
		}
	}
	metricRequest := oceanengine.StatQueryRequest{DatasetKey: "promotion", Dimensions: []string{"promotion_id", "stat_time_day"}, Metrics: []string{"stat_cost", "show_cnt", "click_cnt", "convert_cnt"}, StartTime: request.WindowStart.Format(time.RFC3339), EndTime: request.WindowEnd.Format(time.RFC3339), Limit: 500}
	for page := 0; page < maxPages; page++ {
		metricRequest.Offset = page * metricRequest.Limit
		cursor = fmt.Sprintf("metric_offset:%d", metricRequest.Offset)
		metricsPayload, readErr := reader.StatQueryPage(ctx, metricRequest)
		if readErr != nil {
			return result, readErr
		}
		metricRawID, metricObservedAt, storeErr := s.storeRaw(ctx, request, runID, "metric_window", map[string]any{"dataset": "promotion", "start": request.WindowStart, "end": request.WindowEnd, "offset": metricRequest.Offset, "limit": metricRequest.Limit}, metricsPayload)
		if storeErr != nil {
			return result, storeErr
		}
		rows := metricRows(metricsPayload)
		for _, row := range rows {
			metric, ok := normalizeMetricRow(row, request, runID, metricRawID, metricObservedAt)
			if !ok {
				continue
			}
			previous, revisionNumber, found, latestErr := s.Writer.LatestMetric(ctx, request.OrganizationID, request.ProjectID, metric.SourceRef, metric.ObjectRef, metric.WindowStart, metric.WindowEnd, metric.AttributionWindow, metric.MetricDefinitionVersion, metric.AvailableAt)
			if latestErr != nil {
				return result, latestErr
			}
			if found && previous.PayloadHash != metric.PayloadHash {
				metric.RevisionOf = previous.ID
				metric.QualityIssues = append(metric.QualityIssues, AssessMetricRevision(previous, metric)...)
				metric.QualityStatus = qualityDisposition(metric.QualityIssues)
			}
			createdMetric, appendErr := s.Writer.AppendMetric(ctx, metric)
			if appendErr != nil {
				return result, appendErr
			}
			if createdMetric {
				result.MetricCount++
			}
			if found && previous.Metrics["conversions"] != metric.Metrics["conversions"] {
				revision := ConversionRevision{MetricWindow: metric, OriginalWindowID: previous.ID, RevisionNumber: revisionNumber + 1}
				if _, appendErr = s.Writer.AppendConversionRevision(ctx, revision); appendErr != nil {
					return result, appendErr
				}
			}
		}
		if len(rows) < metricRequest.Limit {
			break
		}
	}
	materialRequest := oceanengine.StatQueryRequest{DatasetKey: "material", Dimensions: []string{"promotion_id", "material_id", "stat_time_day"}, Metrics: []string{"stat_cost", "show_cnt", "click_cnt", "convert_cnt"}, StartTime: request.WindowStart.Format(time.RFC3339), EndTime: request.WindowEnd.Format(time.RFC3339), Limit: 500}
	for page := 0; page < maxPages; page++ {
		materialRequest.Offset = page * materialRequest.Limit
		cursor = fmt.Sprintf("material_metric_offset:%d", materialRequest.Offset)
		payload, readErr := reader.StatQueryPage(ctx, materialRequest)
		if readErr != nil {
			return result, readErr
		}
		rawID, materialMetricObservedAt, storeErr := s.storeRaw(ctx, request, runID, "material_metric_window", map[string]any{"dataset": "material", "start": request.WindowStart, "end": request.WindowEnd, "offset": materialRequest.Offset, "limit": materialRequest.Limit}, payload)
		if storeErr != nil {
			return result, storeErr
		}
		rows := metricRows(payload)
		for _, row := range rows {
			metric, ok := normalizeMetricRow(row, request, runID, rawID, materialMetricObservedAt)
			if !ok {
				continue
			}
			dimensions, _ := row["Dimensions"].(map[string]any)
			materialRef := firstString(dimensions, "material_id")
			promotionRef := firstString(dimensions, "promotion_id")
			if materialRef == "" || promotionRef == "" {
				continue
			}
			metric.ID = "material_metric_" + canonicalHash([]string{rawID, materialRef, promotionRef, metric.WindowStart.String(), metric.PayloadHash})
			metric.ObjectRef = opaqueRef(materialRef)
			createdMetric, appendErr := s.Writer.AppendMaterialMetric(ctx, MaterialMetricWindow{MetricWindow: metric, MaterialRef: opaqueRef(materialRef), PromotionRef: opaqueRef(promotionRef)})
			if appendErr != nil {
				return result, appendErr
			}
			if createdMetric {
				result.MetricCount++
			}
		}
		if len(rows) < materialRequest.Limit {
			break
		}
	}
	status, cursor = "completed", "complete"
	return result, nil
}

func (s Synchronizer) header(request SyncRequest, runID, evidenceRef, payloadHash string, collected, valid time.Time) FactHeader {
	return FactHeader{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, SourceSystem: SourceSystem, SourceRef: opaqueRef(request.AccountRef), IngestRunID: runID, SchemaVersion: DatasetVersion, PayloadHash: payloadHash, CollectedAt: collected, AvailableAt: collected, DataThrough: request.WindowEnd, ValidFrom: valid, QualityStatus: QualityAccept, EvidenceRef: evidenceRef}
}
func (s Synchronizer) storeRaw(ctx context.Context, request SyncRequest, runID, endpoint string, requestShape, response map[string]any) (string, time.Time, error) {
	collected := time.Now().UTC()
	if s.Now != nil {
		collected = s.Now().UTC()
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return "", time.Time{}, err
	}
	ciphertext, keyVersion, err := s.Cipher.Encrypt(responseBytes)
	for index := range responseBytes {
		responseBytes[index] = 0
	}
	if err != nil {
		return "", time.Time{}, err
	}
	requestHash := canonicalHash(requestShape)
	payloadHash := canonicalHash(response)
	id := "raw_" + canonicalHash([]string{runID, endpoint, requestHash, payloadHash})
	header := s.header(request, runID, id, payloadHash, collected, collected)
	_, err = s.Writer.AppendRaw(ctx, RawSnapshot{Header: header, ID: id, Endpoint: endpoint, RequestHash: requestHash, EncryptedEvidence: ciphertext, KeyVersion: keyVersion})
	return id, collected, err
}
func promotionItems(payload map[string]any) ([]map[string]any, int) {
	data, _ := payload["data"].(map[string]any)
	raw, _ := data["ads"].([]any)
	result := []map[string]any{}
	for _, item := range raw {
		if value, ok := item.(map[string]any); ok {
			result = append(result, value)
		}
	}
	pagination, _ := data["pagination"].(map[string]any)
	pages := int(numberValue(pagination["total_page"]))
	return result, pages
}
func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
		if number, ok := values[key].(json.Number); ok {
			return number.String()
		}
		if number, ok := values[key].(float64); ok {
			return fmt.Sprintf("%.0f", number)
		}
	}
	return ""
}
func platformValidTime(values map[string]any, fallback time.Time) time.Time {
	for _, key := range []string{"modify_time", "update_time", "create_time"} {
		if raw, ok := values[key].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
				return parsed
			}
		}
	}
	return fallback
}
func redactCanonical(value map[string]any) map[string]any {
	result := map[string]any{}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "cookie") || strings.Contains(lower, "token") || strings.Contains(lower, "csrf") || strings.Contains(lower, "device") || strings.Contains(lower, "url") || lower == "id" || strings.HasSuffix(lower, "_id") || strings.HasSuffix(lower, "_ids") {
			continue
		}
		switch item := value[key].(type) {
		case map[string]any:
			result[key] = redactCanonical(item)
		case []any:
			clean := make([]any, 0, len(item))
			for _, nested := range item {
				if mapped, ok := nested.(map[string]any); ok {
					clean = append(clean, redactCanonical(mapped))
					continue
				}
				if text, ok := nested.(string); ok && (strings.HasPrefix(strings.ToLower(text), "http://") || strings.HasPrefix(strings.ToLower(text), "https://")) {
					continue
				}
				clean = append(clean, nested)
			}
			result[key] = clean
		case string:
			if strings.HasPrefix(strings.ToLower(item), "http://") || strings.HasPrefix(strings.ToLower(item), "https://") {
				continue
			}
			result[key] = item
		default:
			result[key] = item
		}
	}
	return result
}
func collectStringIDs(values map[string]any, keys ...string) []string {
	found := map[string]struct{}{}
	var walk func(any)
	walk = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			for key, nested := range item {
				for _, wanted := range keys {
					if key == wanted {
						switch ids := nested.(type) {
						case []any:
							for _, id := range ids {
								if text, ok := id.(string); ok {
									found[text] = struct{}{}
								}
							}
						case string:
							found[ids] = struct{}{}
						}
					}
				}
				walk(nested)
			}
		case []any:
			for _, nested := range item {
				walk(nested)
			}
		}
	}
	walk(values)
	result := make([]string, 0, len(found))
	for value := range found {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func metricRows(payload map[string]any) []map[string]any {
	data, _ := payload["data"].(map[string]any)
	stats, _ := data["StatsData"].(map[string]any)
	return oceanengine.FlattenRows(stats["Rows"])
}
func normalizeMetricRow(row map[string]any, request SyncRequest, runID, evidenceRef string, collected time.Time) (MetricWindow, bool) {
	dimensions, _ := row["Dimensions"].(map[string]any)
	metrics, _ := row["Metrics"].(map[string]any)
	objectRef := firstString(dimensions, "promotion_id")
	if objectRef == "" {
		objectRef = firstString(row, "promotion_id")
	}
	if objectRef == "" {
		return MetricWindow{}, false
	}
	start, end := request.WindowStart, request.WindowEnd
	if date := firstString(dimensions, "stat_time_day", "date"); date != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", date, time.UTC); err == nil {
			start = parsed
			end = parsed.AddDate(0, 0, 1)
		}
	}
	counts := map[string]int64{"spend": int64(numberValue(metrics["stat_cost"])), "impressions": int64(numberValue(metrics["show_cnt"])), "clicks": int64(numberValue(metrics["click_cnt"])), "conversions": int64(numberValue(metrics["convert_cnt"]))}
	payloadHash := canonicalHash(map[string]any{"object": objectRef, "start": start, "end": end, "metrics": counts})
	header := FactHeader{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, SourceSystem: SourceSystem, SourceRef: opaqueRef(request.AccountRef), IngestRunID: runID, SchemaVersion: DatasetVersion, PayloadHash: payloadHash, CollectedAt: collected, AvailableAt: collected, DataThrough: request.WindowEnd, ValidFrom: start, QualityStatus: QualityQuarantine, EvidenceRef: evidenceRef}
	value := MetricWindow{FactHeader: header, ID: "metric_" + canonicalHash([]string{evidenceRef, objectRef, start.String(), end.String(), payloadHash}), ObjectRef: opaqueRef(objectRef), WindowStart: start, WindowEnd: end, Granularity: "day", TimeZone: request.TimeZone, AttributionWindow: "platform_default_unconfirmed", MetricDefinitionVersion: "oceanengine-atomic-v1", Currency: request.Currency, AmountUnit: "fen", Metrics: counts}
	value.QualityIssues = AssessMetric(value, false, true, true)
	derived := map[string]float64{}
	for _, key := range []string{"ctr", "cvr"} {
		if _, ok := metrics[key]; ok {
			derived[key] = numberValue(metrics[key])
		}
	}
	value.QualityIssues = append(value.QualityIssues, AssessDerivedRates(value, derived)...)
	value.QualityStatus = qualityDisposition(value.QualityIssues)
	return value, true
}
func numberValue(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case json.Number:
		value, _ := number.Float64()
		return value
	case string:
		var parsed json.Number = json.Number(number)
		value, _ := parsed.Float64()
		return value
	}
	return 0
}
