package connector

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const OfflineXLSXImportSchemaVersion = "connector-oceanengine-offline-xlsx/v2"

const (
	offlineRoleAccountDaily      = "account_daily"
	offlineRoleProjectDaily      = "project_daily"
	offlineRolePromotionDaily    = "promotion_daily"
	offlineRoleMaterialAggregate = "material_aggregate"
)

type OfflineXLSXSource struct {
	Name    string
	Content []byte
}

type OfflineXLSXImportRequest struct {
	OrganizationID  string
	ProjectID       string
	AccountID       string
	ExternalAccount string
	IdempotencyKey  string
	TimeZone        string
	Currency        string
	Sources         []OfflineXLSXSource
}

type OfflineXLSXImportResult struct {
	RunID                        string    `json:"run_id"`
	Replayed                     bool      `json:"replayed"`
	DateStart                    time.Time `json:"date_start"`
	DateEnd                      time.Time `json:"date_end"`
	ProjectCount                 int       `json:"project_count"`
	PromotionCount               int       `json:"promotion_count"`
	MetricWindowCount            int       `json:"metric_window_count"`
	SourceMetricWindowCount      int       `json:"source_metric_window_count"`
	ImputedZeroMetricWindowCount int       `json:"imputed_zero_metric_window_count"`
	MaterialRowCount             int       `json:"material_row_count"`
	Reconciliation               string    `json:"reconciliation"`
	MaterialMetricUsage          string    `json:"material_metric_usage"`
}

type OfflineXLSXImportWriter interface {
	StartSync(context.Context, SyncRun) (bool, error)
	UpdateSyncCursor(context.Context, string, string) error
	CompleteSync(context.Context, string, string, string, time.Time) error
	AppendRaw(context.Context, RawSnapshot) (bool, error)
	AppendObject(context.Context, ObjectSnapshot) (bool, error)
	AppendMetric(context.Context, MetricWindow) (bool, error)
}

type OfflineXLSXImporter struct {
	Writer OfflineXLSXImportWriter
	Cipher EvidenceCipher
	Now    func() time.Time
}

type offlineAtomicMetrics struct {
	Spend       int64
	Impressions int64
	Clicks      int64
	Conversions int64
}

type offlineDailyMetric struct {
	Date        time.Time
	ObjectID    string
	Metrics     offlineAtomicMetrics
	SourceRow   int
	ImputedZero bool
}

type offlineParsedSource struct {
	Role    string
	Name    string
	Content []byte
	Rows    [][]string
}

type offlineImportDataset struct {
	Sources                    map[string]offlineParsedSource
	AccountDaily               map[time.Time]offlineAtomicMetrics
	ProjectDaily               []offlineDailyMetric
	PromotionDaily             []offlineDailyMetric
	ProjectFirstSeen           map[string]time.Time
	PromotionFirst             map[string]time.Time
	MaterialRows               int
	SourcePromotionMetricCount int
	ImputedZeroMetricCount     int
	DateStart                  time.Time
	DateEnd                    time.Time
}

func DetectOfflineXLSXExternalAccount(sources []OfflineXLSXSource) (string, error) {
	if len(sources) != 4 {
		return "", ErrInvalidFact
	}
	externalAccount := ""
	for _, source := range sources {
		base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(source.Name)), filepath.Ext(source.Name))
		parts := strings.Split(base, "_")
		if len(parts) < 3 || (parts[0] != "基础数据" && parts[0] != "素材数据") || !validOfflinePlatformID(parts[1]) {
			return "", fmt.Errorf("%w: offline source name does not contain a valid account binding", ErrInvalidFact)
		}
		if externalAccount != "" && externalAccount != parts[1] {
			return "", fmt.Errorf("%w: offline sources contain different account bindings", ErrInvalidFact)
		}
		externalAccount = parts[1]
	}
	return externalAccount, nil
}

func BuildOfflineXLSXSnapshot(request OfflineXLSXImportRequest, collected time.Time) (CanonicalSnapshot, OfflineXLSXImportResult, error) {
	request.OrganizationID = strings.TrimSpace(request.OrganizationID)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.AccountID = strings.TrimSpace(request.AccountID)
	request.ExternalAccount = strings.TrimSpace(request.ExternalAccount)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.TimeZone = strings.TrimSpace(request.TimeZone)
	request.Currency = strings.TrimSpace(request.Currency)
	if request.TimeZone == "" {
		request.TimeZone = "Asia/Shanghai"
	}
	if request.Currency == "" {
		request.Currency = "CNY"
	}
	if request.OrganizationID == "" || !strings.HasPrefix(request.AccountID, "oeacct_") || !validOfflinePlatformID(request.ExternalAccount) || request.IdempotencyKey == "" || len(request.Sources) != 4 || collected.IsZero() {
		return CanonicalSnapshot{}, OfflineXLSXImportResult{}, ErrInvalidFact
	}
	dataset, err := parseOfflineImportDataset(request)
	if err != nil {
		return CanonicalSnapshot{}, OfflineXLSXImportResult{}, err
	}
	runID := "sync_" + canonicalHash([]string{OfflineXLSXImportSchemaVersion, request.OrganizationID, request.ProjectID, request.AccountID, request.IdempotencyKey})
	result := OfflineXLSXImportResult{
		RunID: runID, DateStart: dataset.DateStart, DateEnd: dataset.DateEnd,
		ProjectCount: len(dataset.ProjectFirstSeen), PromotionCount: len(dataset.PromotionFirst), MetricWindowCount: len(dataset.PromotionDaily), SourceMetricWindowCount: dataset.SourcePromotionMetricCount, ImputedZeroMetricWindowCount: dataset.ImputedZeroMetricCount, MaterialRowCount: dataset.MaterialRows,
		Reconciliation: "passed", MaterialMetricUsage: "excluded_missing_daily_promotion_binding_and_duplicate_attribution",
	}
	snapshot := CanonicalSnapshot{DatasetVersion: DatasetVersion, PredictionCutoff: collected.UTC()}
	for _, item := range sortedOfflineFirstSeen(dataset.ProjectFirstSeen) {
		snapshot.Objects = append(snapshot.Objects, offlineObject(request, runID, "offline_project_evidence", "project", item.ID, "", item.FirstSeen, dataset.DateEnd, collected.UTC(), QualityAccept))
	}
	for _, item := range sortedOfflineFirstSeen(dataset.PromotionFirst) {
		snapshot.Objects = append(snapshot.Objects, offlineObject(request, runID, "offline_promotion_evidence", "promotion", item.ID, "", item.FirstSeen, dataset.DateEnd, collected.UTC(), QualityWarning))
	}
	for _, value := range dataset.PromotionDaily {
		snapshot.Metrics = append(snapshot.Metrics, offlineMetric(request, runID, "offline_promotion_evidence", value, collected.UTC()))
	}
	return snapshot, result, nil
}

func (i OfflineXLSXImporter) Import(ctx context.Context, request OfflineXLSXImportRequest) (result OfflineXLSXImportResult, resultErr error) {
	request.OrganizationID = strings.TrimSpace(request.OrganizationID)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.AccountID = strings.TrimSpace(request.AccountID)
	request.ExternalAccount = strings.TrimSpace(request.ExternalAccount)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.TimeZone = strings.TrimSpace(request.TimeZone)
	request.Currency = strings.TrimSpace(request.Currency)
	if request.TimeZone == "" {
		request.TimeZone = "Asia/Shanghai"
	}
	if request.Currency == "" {
		request.Currency = "CNY"
	}
	if i.Writer == nil || i.Cipher == nil || request.OrganizationID == "" || !strings.HasPrefix(request.AccountID, "oeacct_") || !validOfflinePlatformID(request.ExternalAccount) || request.IdempotencyKey == "" || len(request.Sources) != 4 {
		return result, ErrInvalidFact
	}
	dataset, err := parseOfflineImportDataset(request)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	if i.Now != nil {
		now = i.Now().UTC()
	}
	runID := "sync_" + canonicalHash([]string{OfflineXLSXImportSchemaVersion, request.OrganizationID, request.ProjectID, request.AccountID, request.IdempotencyKey})
	result = OfflineXLSXImportResult{
		RunID: runID, DateStart: dataset.DateStart, DateEnd: dataset.DateEnd,
		ProjectCount: len(dataset.ProjectFirstSeen), PromotionCount: len(dataset.PromotionFirst), MetricWindowCount: len(dataset.PromotionDaily), SourceMetricWindowCount: dataset.SourcePromotionMetricCount, ImputedZeroMetricWindowCount: dataset.ImputedZeroMetricCount, MaterialRowCount: dataset.MaterialRows,
		Reconciliation: "passed", MaterialMetricUsage: "excluded_missing_daily_promotion_binding_and_duplicate_attribution",
	}
	created, err := i.Writer.StartSync(ctx, SyncRun{ID: runID, OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, AccountRef: opaqueRef(request.AccountID), StartedAt: now, Attempt: 1})
	if err != nil {
		return result, err
	}
	if !created {
		result.Replayed = true
		return result, nil
	}
	status, cursor := "failed", "offline_xlsx_start"
	defer func() {
		completeAt := time.Now().UTC()
		if i.Now != nil {
			completeAt = i.Now().UTC()
		}
		if completeErr := i.Writer.CompleteSync(ctx, runID, cursor, status, completeAt); resultErr == nil && completeErr != nil {
			resultErr = completeErr
		}
	}()
	evidence := make(map[string]string, len(dataset.Sources))
	roles := []string{offlineRoleAccountDaily, offlineRoleProjectDaily, offlineRolePromotionDaily, offlineRoleMaterialAggregate}
	for _, role := range roles {
		cursor = "offline_xlsx_raw_" + role
		if err = i.Writer.UpdateSyncCursor(ctx, runID, cursor); err != nil {
			return result, err
		}
		source := dataset.Sources[role]
		evidence[role], err = i.appendOfflineRaw(ctx, request, runID, role, source.Content, dataset.DateStart, dataset.DateEnd, now)
		if err != nil {
			return result, err
		}
	}
	cursor = "offline_xlsx_objects"
	if err = i.Writer.UpdateSyncCursor(ctx, runID, cursor); err != nil {
		return result, err
	}
	for _, item := range sortedOfflineFirstSeen(dataset.ProjectFirstSeen) {
		if err = i.appendOfflineObject(ctx, request, runID, evidence[offlineRoleProjectDaily], "project", item.ID, "", item.FirstSeen, dataset.DateEnd, now, QualityAccept); err != nil {
			return result, err
		}
	}
	for _, item := range sortedOfflineFirstSeen(dataset.PromotionFirst) {
		if err = i.appendOfflineObject(ctx, request, runID, evidence[offlineRolePromotionDaily], "promotion", item.ID, "", item.FirstSeen, dataset.DateEnd, now, QualityWarning); err != nil {
			return result, err
		}
	}
	cursor = "offline_xlsx_metrics"
	if err = i.Writer.UpdateSyncCursor(ctx, runID, cursor); err != nil {
		return result, err
	}
	for _, value := range dataset.PromotionDaily {
		if err = i.appendOfflineMetric(ctx, request, runID, evidence[offlineRolePromotionDaily], value, now); err != nil {
			return result, fmt.Errorf("append offline promotion metric at source row %d: %w", value.SourceRow, err)
		}
	}
	cursor, status = "complete", "completed"
	return result, nil
}

func parseOfflineImportDataset(request OfflineXLSXImportRequest) (offlineImportDataset, error) {
	detectedAccount, err := DetectOfflineXLSXExternalAccount(request.Sources)
	if err != nil || detectedAccount != request.ExternalAccount {
		return offlineImportDataset{}, fmt.Errorf("%w: offline source account does not match the registered Connector account", ErrInvalidFact)
	}
	parsed := make(map[string]offlineParsedSource, len(request.Sources))
	for _, source := range request.Sources {
		rows, err := readOfflineXLSX(source.Content)
		if err != nil {
			return offlineImportDataset{}, err
		}
		role, err := classifyOfflineXLSX(rows[0])
		if err != nil || parsed[role].Role != "" {
			return offlineImportDataset{}, fmt.Errorf("%w: offline workbook roles must be unique and complete", ErrInvalidFact)
		}
		parsed[role] = offlineParsedSource{Role: role, Name: filepath.Base(source.Name), Content: source.Content, Rows: rows}
	}
	for _, role := range []string{offlineRoleAccountDaily, offlineRoleProjectDaily, offlineRolePromotionDaily, offlineRoleMaterialAggregate} {
		if parsed[role].Role == "" {
			return offlineImportDataset{}, fmt.Errorf("%w: required offline workbook role is missing", ErrInvalidFact)
		}
	}
	location, err := time.LoadLocation(request.TimeZone)
	if err != nil {
		if request.TimeZone != "Asia/Shanghai" {
			return offlineImportDataset{}, fmt.Errorf("%w: unsupported offline export time zone", ErrInvalidFact)
		}
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	account, err := parseOfflineDailyRows(parsed[offlineRoleAccountDaily].Rows, "", location)
	if err != nil {
		return offlineImportDataset{}, err
	}
	projects, err := parseOfflineDailyRows(parsed[offlineRoleProjectDaily].Rows, "项目ID", location)
	if err != nil {
		return offlineImportDataset{}, err
	}
	promotions, err := parseOfflineDailyRows(parsed[offlineRolePromotionDaily].Rows, "单元ID", location)
	if err != nil {
		return offlineImportDataset{}, err
	}
	accountByDate, err := offlineAccountDailyMap(account)
	if err != nil {
		return offlineImportDataset{}, err
	}
	if err = reconcileOfflineDaily(accountByDate, projects); err != nil {
		return offlineImportDataset{}, fmt.Errorf("project daily reconciliation: %w", err)
	}
	if err = reconcileOfflineDaily(accountByDate, promotions); err != nil {
		return offlineImportDataset{}, fmt.Errorf("promotion daily reconciliation: %w", err)
	}
	promotionFirst := offlineFirstSeen(promotions)
	sourcePromotionMetricCount := len(promotions)
	promotions, imputedZeroMetricCount := completeOfflinePromotionDaily(accountByDate, promotions, promotionFirst)
	materialRows, err := validateOfflineMaterialRows(parsed[offlineRoleMaterialAggregate].Rows)
	if err != nil {
		return offlineImportDataset{}, err
	}
	dateStart, dateEnd := offlineDateRange(accountByDate)
	return offlineImportDataset{
		Sources: parsed, AccountDaily: accountByDate, ProjectDaily: projects, PromotionDaily: promotions,
		ProjectFirstSeen: offlineFirstSeen(projects), PromotionFirst: promotionFirst, MaterialRows: materialRows,
		SourcePromotionMetricCount: sourcePromotionMetricCount, ImputedZeroMetricCount: imputedZeroMetricCount,
		DateStart: dateStart, DateEnd: dateEnd,
	}, nil
}

func completeOfflinePromotionDaily(account map[time.Time]offlineAtomicMetrics, values []offlineDailyMetric, firstSeen map[string]time.Time) ([]offlineDailyMetric, int) {
	result := append([]offlineDailyMetric(nil), values...)
	existing := make(map[string]struct{}, len(values))
	for _, value := range values {
		existing[value.Date.Format("2006-01-02")+"|"+value.ObjectID] = struct{}{}
	}
	for objectID, first := range firstSeen {
		for date := range account {
			if date.Before(first) {
				continue
			}
			key := date.Format("2006-01-02") + "|" + objectID
			if _, ok := existing[key]; ok {
				continue
			}
			result = append(result, offlineDailyMetric{Date: date, ObjectID: objectID, ImputedZero: true})
			existing[key] = struct{}{}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Date.Equal(result[j].Date) {
			return result[i].ObjectID < result[j].ObjectID
		}
		return result[i].Date.Before(result[j].Date)
	})
	return result, len(result) - len(values)
}

func classifyOfflineXLSX(header []string) (string, error) {
	columns, err := offlineHeaderMap(header)
	if err != nil {
		return "", err
	}
	for _, required := range []string{"消耗", "展示数", "点击数", "转化数"} {
		if _, ok := columns[required]; !ok {
			return "", fmt.Errorf("%w: offline workbook misses an atomic metric", ErrInvalidFact)
		}
	}
	_, hasDate := columns["时间-天"]
	_, hasProject := columns["项目ID"]
	_, hasPromotion := columns["单元ID"]
	_, hasMaterial := columns["素材ID"]
	switch {
	case hasDate && hasPromotion && !hasProject && !hasMaterial:
		return offlineRolePromotionDaily, nil
	case hasDate && hasProject && !hasPromotion && !hasMaterial:
		return offlineRoleProjectDaily, nil
	case hasDate && !hasProject && !hasPromotion && !hasMaterial:
		return offlineRoleAccountDaily, nil
	case !hasDate && hasMaterial && !hasProject && !hasPromotion:
		return offlineRoleMaterialAggregate, nil
	default:
		return "", fmt.Errorf("%w: offline workbook grain is unsupported", ErrInvalidFact)
	}
}

func offlineHeaderMap(header []string) (map[string]int, error) {
	result := make(map[string]int, len(header))
	for index, value := range header {
		value = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
		if value == "" {
			continue
		}
		if _, exists := result[value]; exists {
			return nil, fmt.Errorf("%w: duplicate offline workbook header", ErrInvalidFact)
		}
		result[value] = index
	}
	return result, nil
}

func parseOfflineDailyRows(rows [][]string, objectColumn string, location *time.Location) ([]offlineDailyMetric, error) {
	header, err := offlineHeaderMap(rows[0])
	if err != nil {
		return nil, err
	}
	result := make([]offlineDailyMetric, 0, len(rows)-1)
	seen := make(map[string]struct{}, len(rows)-1)
	for index, row := range rows[1:] {
		date, parseErr := time.ParseInLocation("2006-01-02", offlineCell(row, header["时间-天"]), location)
		if parseErr != nil {
			return nil, fmt.Errorf("%w: invalid date at source row %d", ErrInvalidFact, index+2)
		}
		objectID := "account"
		if objectColumn != "" {
			objectID = offlineCell(row, header[objectColumn])
			if !validOfflinePlatformID(objectID) {
				return nil, fmt.Errorf("%w: invalid object identifier at source row %d", ErrInvalidFact, index+2)
			}
		}
		key := date.Format("2006-01-02") + "|" + objectID
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%w: duplicate daily object row at source row %d", ErrInvalidFact, index+2)
		}
		seen[key] = struct{}{}
		spend, parseErr := parseOfflineMinor(offlineCell(row, header["消耗"]))
		if parseErr != nil {
			return nil, fmt.Errorf("%w: invalid spend at source row %d", ErrInvalidFact, index+2)
		}
		impressions, parseErr := parseOfflineCount(offlineCell(row, header["展示数"]))
		if parseErr != nil {
			return nil, fmt.Errorf("%w: invalid impressions at source row %d", ErrInvalidFact, index+2)
		}
		clicks, parseErr := parseOfflineCount(offlineCell(row, header["点击数"]))
		if parseErr != nil {
			return nil, fmt.Errorf("%w: invalid clicks at source row %d", ErrInvalidFact, index+2)
		}
		conversions, parseErr := parseOfflineCount(offlineCell(row, header["转化数"]))
		if parseErr != nil {
			return nil, fmt.Errorf("%w: invalid conversions at source row %d", ErrInvalidFact, index+2)
		}
		result = append(result, offlineDailyMetric{Date: date.UTC(), ObjectID: objectID, Metrics: offlineAtomicMetrics{Spend: spend, Impressions: impressions, Clicks: clicks, Conversions: conversions}, SourceRow: index + 2})
	}
	return result, nil
}

func offlineAccountDailyMap(values []offlineDailyMetric) (map[time.Time]offlineAtomicMetrics, error) {
	result := make(map[time.Time]offlineAtomicMetrics, len(values))
	for _, value := range values {
		if _, exists := result[value.Date]; exists {
			return nil, fmt.Errorf("%w: duplicate account daily row", ErrInvalidFact)
		}
		result[value.Date] = value.Metrics
	}
	return result, nil
}

func reconcileOfflineDaily(account map[time.Time]offlineAtomicMetrics, values []offlineDailyMetric) error {
	totals := make(map[time.Time]offlineAtomicMetrics, len(account))
	for _, value := range values {
		current := totals[value.Date]
		current.Spend += value.Metrics.Spend
		current.Impressions += value.Metrics.Impressions
		current.Clicks += value.Metrics.Clicks
		current.Conversions += value.Metrics.Conversions
		totals[value.Date] = current
	}
	if len(totals) != len(account) {
		return fmt.Errorf("%w: daily coverage differs from account export", ErrInvalidFact)
	}
	for date, expected := range account {
		if totals[date] != expected {
			return fmt.Errorf("%w: atomic totals differ on an export date", ErrInvalidFact)
		}
	}
	return nil
}

func validateOfflineMaterialRows(rows [][]string) (int, error) {
	header, err := offlineHeaderMap(rows[0])
	if err != nil {
		return 0, err
	}
	seen := make(map[string]struct{}, len(rows)-1)
	for index, row := range rows[1:] {
		id := offlineCell(row, header["素材ID"])
		if !validOfflinePlatformID(id) {
			return 0, fmt.Errorf("%w: invalid material identifier at source row %d", ErrInvalidFact, index+2)
		}
		if _, exists := seen[id]; exists {
			return 0, fmt.Errorf("%w: duplicate material row at source row %d", ErrInvalidFact, index+2)
		}
		seen[id] = struct{}{}
	}
	return len(seen), nil
}

func offlineCell(row []string, column int) string {
	if column < 0 || column >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[column])
}

func parseOfflineMinor(value string) (int64, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	if value == "" || strings.HasPrefix(value, "-") {
		return 0, ErrInvalidFact
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalidFact
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, ErrInvalidFact
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents := int64(0)
	if fraction != "" {
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	if whole > (1<<63-1-cents)/100 {
		return 0, ErrInvalidFact
	}
	return whole*100 + cents, nil
}

func parseOfflineCount(value string) (int64, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	if value == "" || strings.Contains(value, ".") {
		return 0, ErrInvalidFact
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, ErrInvalidFact
	}
	return parsed, nil
}

func validOfflinePlatformID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func offlineFirstSeen(values []offlineDailyMetric) map[string]time.Time {
	result := map[string]time.Time{}
	for _, value := range values {
		current, exists := result[value.ObjectID]
		if !exists || value.Date.Before(current) {
			result[value.ObjectID] = value.Date
		}
	}
	return result
}

type offlineFirstSeenValue struct {
	ID        string
	FirstSeen time.Time
}

func sortedOfflineFirstSeen(values map[string]time.Time) []offlineFirstSeenValue {
	result := make([]offlineFirstSeenValue, 0, len(values))
	for id, firstSeen := range values {
		result = append(result, offlineFirstSeenValue{ID: id, FirstSeen: firstSeen})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func offlineDateRange(values map[time.Time]offlineAtomicMetrics) (time.Time, time.Time) {
	var start, end time.Time
	for date := range values {
		if start.IsZero() || date.Before(start) {
			start = date
		}
		windowEnd := date.Add(24 * time.Hour)
		if end.IsZero() || windowEnd.After(end) {
			end = windowEnd
		}
	}
	return start, end
}

func (i OfflineXLSXImporter) appendOfflineRaw(ctx context.Context, request OfflineXLSXImportRequest, runID, role string, content []byte, start, end, collected time.Time) (string, error) {
	payloadHash := canonicalHash(content)
	requestHash := canonicalHash(map[string]string{"role": role, "schema": OfflineXLSXImportSchemaVersion})
	id := "raw_" + canonicalHash([]string{runID, role, requestHash, payloadHash})
	ciphertext, keyVersion, err := i.Cipher.Encrypt(content)
	if err != nil {
		return "", err
	}
	header := FactHeader{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, SourceSystem: SourceSystem, SourceRef: opaqueRef(request.AccountID), IngestRunID: runID, SchemaVersion: OfflineXLSXImportSchemaVersion, PayloadHash: payloadHash, CollectedAt: collected, AvailableAt: collected, DataThrough: end, ValidFrom: start, QualityStatus: QualityAccept, EvidenceRef: id}
	_, err = i.Writer.AppendRaw(ctx, RawSnapshot{Header: header, ID: id, Endpoint: "offline_xlsx_" + role, RequestHash: requestHash, EncryptedEvidence: ciphertext, KeyVersion: keyVersion})
	return id, err
}

func (i OfflineXLSXImporter) appendOfflineObject(ctx context.Context, request OfflineXLSXImportRequest, runID, evidenceRef, kind, rawID, parent string, firstSeen, dataThrough, collected time.Time, quality QualityDisposition) error {
	value := offlineObject(request, runID, evidenceRef, kind, rawID, parent, firstSeen, dataThrough, collected, quality)
	_, err := i.Writer.AppendObject(ctx, value)
	return err
}

func offlineObject(request OfflineXLSXImportRequest, runID, evidenceRef, kind, rawID, parent string, firstSeen, dataThrough, collected time.Time, quality QualityDisposition) ObjectSnapshot {
	objectRef := opaqueRef(rawID)
	parentRef := opaqueRef(parent)
	payloadHash := canonicalHash(map[string]any{"kind": kind, "object_ref": objectRef, "parent_ref": parentRef, "first_seen": firstSeen})
	header := FactHeader{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, SourceSystem: SourceSystem, SourceRef: opaqueRef(request.AccountID), IngestRunID: runID, SchemaVersion: OfflineXLSXImportSchemaVersion, PayloadHash: payloadHash, CollectedAt: collected, AvailableAt: collected, DataThrough: dataThrough, ValidFrom: firstSeen, QualityStatus: quality, EvidenceRef: evidenceRef}
	return ObjectSnapshot{FactHeader: header, ID: "object_" + canonicalHash([]string{runID, kind, objectRef, payloadHash}), ObjectKind: kind, ObjectRef: objectRef, ParentRef: parentRef, State: map[string]any{}}
}

func (i OfflineXLSXImporter) appendOfflineMetric(ctx context.Context, request OfflineXLSXImportRequest, runID, evidenceRef string, value offlineDailyMetric, collected time.Time) error {
	metric := offlineMetric(request, runID, evidenceRef, value, collected)
	_, err := i.Writer.AppendMetric(ctx, metric)
	return err
}

func offlineMetric(request OfflineXLSXImportRequest, runID, evidenceRef string, value offlineDailyMetric, collected time.Time) MetricWindow {
	objectRef := opaqueRef(value.ObjectID)
	windowEnd := value.Date.Add(24 * time.Hour)
	metrics := map[string]int64{"spend": value.Metrics.Spend, "impressions": value.Metrics.Impressions, "clicks": value.Metrics.Clicks, "conversions": value.Metrics.Conversions}
	payloadHash := canonicalHash(map[string]any{"object_ref": objectRef, "start": value.Date, "end": windowEnd, "metrics": metrics})
	header := FactHeader{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, SourceSystem: SourceSystem, SourceRef: opaqueRef(request.AccountID), IngestRunID: runID, SchemaVersion: OfflineXLSXImportSchemaVersion, PayloadHash: payloadHash, CollectedAt: collected, AvailableAt: collected, DataThrough: windowEnd, ValidFrom: value.Date, QualityStatus: QualityQuarantine, EvidenceRef: evidenceRef}
	metric := MetricWindow{FactHeader: header, ID: "metric_" + canonicalHash([]string{runID, objectRef, value.Date.String(), payloadHash}), ObjectRef: objectRef, WindowStart: value.Date, WindowEnd: windowEnd, Granularity: "day", TimeZone: request.TimeZone, AttributionWindow: "platform_default_unconfirmed", MetricDefinitionVersion: "oceanengine-offline-export-atomic-v1", Currency: request.Currency, AmountUnit: "fen", Metrics: metrics}
	metric.QualityIssues = AssessMetric(metric, false, true, true)
	if value.ImputedZero {
		metric.QualityIssues = append(metric.QualityIssues, QualityIssue{Disposition: QualityWarning, Code: "reconciled_absent_unit_day_zero"})
	}
	metric.QualityStatus = qualityDisposition(metric.QualityIssues)
	return metric
}

func IsOfflineXLSXImportError(err error) bool {
	return errors.Is(err, ErrInvalidFact) || errors.Is(err, ErrImmutableConflict)
}
