// oceanengine-api-probe performs one controlled project-create contract probe.
// It is disabled unless the normal write switch and account allowlist permit it.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/systems/insights"
)

const executeConfirmation = "CREATE_PROJECT_ONCE"

type harDocument struct {
	Log struct {
		Entries []struct {
			Request struct {
				Method  string `json:"method"`
				URL     string `json:"url"`
				Cookies []struct {
					Name string `json:"name"`
				} `json:"cookies"`
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
				PostData struct {
					Text string `json:"text"`
				} `json:"postData"`
			} `json:"request"`
		} `json:"entries"`
	} `json:"log"`
}

type accountBinding struct {
	organizationID string
	projectID      string
	accountID      string
	externalID     string
	session        connector.OceanEngineAccountSession
}

type probeSummary struct {
	Mode             string   `json:"mode"`
	NameSHA256       string   `json:"name_sha256,omitempty"`
	PayloadSHA256    string   `json:"payload_sha256,omitempty"`
	OptionalFields   string   `json:"optional_fields"`
	ErrorKind        string   `json:"error_kind,omitempty"`
	CSRFHTTPStatus   int      `json:"csrf_http_status,omitempty"`
	CSRFHeader       bool     `json:"csrf_header_present,omitempty"`
	CSRFPartCount    int      `json:"csrf_part_count,omitempty"`
	CSRFStatusZero   bool     `json:"csrf_status_zero,omitempty"`
	CSRFTokenPresent bool     `json:"csrf_token_present,omitempty"`
	CSRFMaxAgeValid  bool     `json:"csrf_max_age_valid,omitempty"`
	CSRFDowngrade    bool     `json:"csrf_downgrade,omitempty"`
	ConnectorCookies int      `json:"connector_cookie_name_count,omitempty"`
	CapturedCookies  int      `json:"captured_cookie_name_count,omitempty"`
	MissingCookies   []string `json:"missing_cookie_names,omitempty"`
	HTTPStatus       int      `json:"http_status,omitempty"`
	BusinessCode     int      `json:"business_code,omitempty"`
	ExactMatchCount  int      `json:"exact_match_count,omitempty"`
	ResponseIDFound  bool     `json:"response_id_found,omitempty"`
	QueryIDFound     bool     `json:"query_id_found,omitempty"`
	ResponseIDsMatch bool     `json:"response_ids_match,omitempty"`
	Result           string   `json:"result"`
}

func main() {
	var harPath, accountDigest, confirmation, reconcileDigest string
	var execute, csrfOnly, cookieSchema bool
	flag.StringVar(&harPath, "har", "", "path to the local HAR capture")
	flag.StringVar(&accountDigest, "account-sha256", "", "SHA-256 of the selected external account ID")
	flag.BoolVar(&execute, "execute", false, "send one project-create POST")
	flag.BoolVar(&csrfOnly, "csrf-only", false, "perform only the protected-path HEAD token request")
	flag.BoolVar(&cookieSchema, "cookie-schema", false, "compare Connector and captured Cookie names without values")
	flag.StringVar(&reconcileDigest, "reconcile-name-sha256", "", "query only and match an exact object-name digest")
	flag.StringVar(&confirmation, "confirm", "", "must equal "+executeConfirmation+" when execute is set")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := run(ctx, harPath, accountDigest, execute, csrfOnly, cookieSchema, reconcileDigest, confirmation); err != nil {
		fmt.Fprintln(os.Stderr, "probe failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, harPath, accountDigest string, execute, csrfOnly, cookieSchema bool, reconcileDigest, confirmation string) error {
	modeCount := 0
	for _, selected := range []bool{execute, csrfOnly, cookieSchema, reconcileDigest != ""} {
		if selected {
			modeCount++
		}
	}
	if len(accountDigest) != 64 || modeCount > 1 {
		return fmt.Errorf("one valid probe mode and the account digest are required")
	}
	var payload map[string]any
	var name string
	var err error
	summary := probeSummary{OptionalFields: "omitted", Result: "ready"}
	if !csrfOnly && !cookieSchema && reconcileDigest == "" {
		if strings.TrimSpace(harPath) == "" {
			return fmt.Errorf("HAR path is required")
		}
		payload, err = projectPayload(harPath, time.Now().UTC())
		if err != nil {
			return err
		}
		name, _ = payload["name"].(string)
		encoded, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			return fmt.Errorf("encode probe payload: %w", encodeErr)
		}
		summary.NameSHA256 = digestString(name)
		summary.PayloadSHA256 = digestBytes(encoded)
	}
	if !execute && !csrfOnly && !cookieSchema && reconcileDigest == "" {
		summary.Mode = "dry_run"
		return writeSummary(summary)
	}
	if execute && confirmation != executeConfirmation {
		return fmt.Errorf("exact execution confirmation is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if !cfg.OceanEngine.Enabled || execute && !cfg.OceanEngine.WebAPIWriteEnabled {
		return fmt.Errorf("required Ocean Engine switches are not enabled")
	}
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		return err
	}
	defer db.Close()
	repository := connector.MySQLRepository{DB: db}
	binding, err := findAccount(ctx, repository, strings.ToLower(accountDigest))
	if err != nil {
		return err
	}
	if execute && !slices.Contains(cfg.OceanEngine.WebAPIWriteAccounts, binding.externalID) {
		return fmt.Errorf("selected account is not in the Web API write allowlist")
	}
	cipher, err := insights.NewAESGCMSecretCipher(cfg.OceanEngine.MasterKey, cfg.OceanEngine.MasterKeyVersion)
	if err != nil {
		return fmt.Errorf("configure Connector session decryption: %w", err)
	}
	plaintext, err := cipher.Decrypt(binding.session.SessionCiphertext, binding.session.SessionKeyVersion)
	if err != nil {
		return fmt.Errorf("decrypt Connector session")
	}
	defer clear(plaintext)
	if cookieSchema {
		captured, captureErr := capturedCookieNames(harPath)
		if captureErr != nil {
			return captureErr
		}
		connectorNames := cookieNames(string(plaintext))
		summary.Mode = "cookie_schema"
		summary.CapturedCookies = len(captured)
		summary.ConnectorCookies = len(connectorNames)
		for _, name := range captured {
			if !slices.Contains(connectorNames, name) {
				summary.MissingCookies = append(summary.MissingCookies, name)
			}
		}
		summary.Result = "compared"
		return writeSummary(summary)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	writer, err := oceanengine.NewWriteClient(cfg.OceanEngine.BaseURL, binding.externalID, binding.session.Version, oceanengine.Session{Cookies: string(plaintext)}, httpClient, nil)
	if err != nil {
		return err
	}
	defer writer.Close()
	reader, err := oceanengine.NewClient(cfg.OceanEngine.BaseURL, binding.externalID, oceanengine.Session{Cookies: string(plaintext)}, httpClient)
	if err != nil {
		return err
	}
	reader.Delay = 0
	reader.MaxAttempts = 1
	if csrfOnly {
		summary.Mode = "csrf_only"
		diagnostic, prepareErr := writer.DiagnoseCSRF(ctx, oceanengine.ProjectCreatePath)
		summary.CSRFHTTPStatus = diagnostic.HTTPStatus
		summary.CSRFHeader = diagnostic.HeaderPresent
		summary.CSRFPartCount = diagnostic.PartCount
		summary.CSRFStatusZero = diagnostic.StatusZero
		summary.CSRFTokenPresent = diagnostic.TokenPresent
		summary.CSRFMaxAgeValid = diagnostic.MaxAgeValid
		summary.CSRFDowngrade = diagnostic.Downgrade
		if prepareErr != nil {
			summary.ErrorKind = errorKind(prepareErr)
			summary.Result = "failed"
		} else {
			summary.Result = "ready"
		}
		return writeSummary(summary)
	}
	if reconcileDigest != "" {
		summary.Mode = "reconcile_only"
		query, queryErr := reader.ListPage(ctx, oceanengine.ListRequest{Start: "2026-09-01", End: "2026-09-02", Page: 1, Limit: 100})
		if queryErr != nil {
			summary.ErrorKind = errorKind(queryErr)
			summary.Result = "query_failed"
			return writeSummary(summary)
		}
		matches := exactNameDigestObjects(query, strings.ToLower(reconcileDigest))
		summary.ExactMatchCount = len(matches)
		summary.QueryIDFound = len(matches) == 1 && findNumericID(matches[0]) != ""
		switch len(matches) {
		case 0:
			summary.Result = "not_found"
		case 1:
			summary.Result = "confirmed"
		default:
			summary.Result = "manual_reconciliation"
		}
		return writeSummary(summary)
	}

	response, writeErr := writer.SubmitJSON(ctx, oceanengine.ProjectCreatePath, payload)
	summary.Mode = "execute"
	summary.HTTPStatus = response.StatusCode
	responseObject := decodeObject(response.Body)
	summary.BusinessCode = businessCode(responseObject)
	summary.ErrorKind = errorKind(writeErr)
	responseID := findNumericID(responseObject)
	summary.ResponseIDFound = responseID != ""

	query, queryErr := reader.ListPage(ctx, oceanengine.ListRequest{
		Start: "2026-09-01", End: "2026-09-02", Page: 1, Limit: 100,
	})
	matches := exactNameObjects(query, name)
	summary.ExactMatchCount = len(matches)
	queryID := ""
	if len(matches) == 1 {
		queryID = findNumericID(matches[0])
	}
	summary.QueryIDFound = queryID != ""
	summary.ResponseIDsMatch = responseID != "" && queryID != "" && responseID == queryID
	switch {
	case len(matches) > 1:
		summary.Result = "manual_reconciliation"
	case len(matches) == 1 && queryID != "" && (responseID == "" || responseID == queryID):
		summary.Result = "confirmed"
	case writeErr != nil || queryErr != nil || len(matches) == 0:
		summary.Result = "result_unknown"
	default:
		summary.Result = "contract_mismatch"
	}
	if err := writeSummary(summary); err != nil {
		return err
	}
	if queryErr != nil {
		return fmt.Errorf("reconciliation query failed after the one write")
	}
	if writeErr != nil && summary.Result != "confirmed" {
		return fmt.Errorf("write did not produce a confirmed result")
	}
	return nil
}

func projectPayload(path string, now time.Time) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read HAR: %w", err)
	}
	var document harDocument
	if err = json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode HAR: %w", err)
	}
	for _, entry := range document.Log.Entries {
		parsed, parseErr := url.Parse(entry.Request.URL)
		if parseErr != nil || entry.Request.Method != http.MethodPost || parsed.Path != oceanengine.ProjectCreatePath {
			continue
		}
		var payload map[string]any
		if err = json.Unmarshal([]byte(entry.Request.PostData.Text), &payload); err != nil {
			return nil, fmt.Errorf("decode captured project request: %w", err)
		}
		stamp := now.Format("20060102T150405Z")
		payload["name"] = "cookies-api-probe-" + stamp + "-project"
		payload["start_time"] = replaceDate(payload["start_time"], "2026-09-01")
		payload["end_time"] = replaceDate(payload["end_time"], "2026-09-02")
		return payload, nil
	}
	return nil, fmt.Errorf("captured project-create request was not found")
}

func capturedCookieNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read HAR: %w", err)
	}
	var document harDocument
	if err = json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode HAR: %w", err)
	}
	for _, entry := range document.Log.Entries {
		parsed, parseErr := url.Parse(entry.Request.URL)
		if parseErr != nil || entry.Request.Method != http.MethodPost || parsed.Path != oceanengine.ProjectCreatePath {
			continue
		}
		var names []string
		for _, cookie := range entry.Request.Cookies {
			if cookie.Name != "" {
				names = append(names, strings.ToLower(cookie.Name))
			}
		}
		if len(names) == 0 {
			for _, header := range entry.Request.Headers {
				if !strings.EqualFold(header.Name, "cookie") {
					continue
				}
				names = cookieNames(header.Value)
				break
			}
		}
		slices.Sort(names)
		return slices.Compact(names), nil
	}
	return nil, fmt.Errorf("captured project-create request was not found")
}

func cookieNames(header string) []string {
	var names []string
	for _, part := range strings.Split(header, ";") {
		name, _, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && name != "" {
			names = append(names, strings.ToLower(strings.TrimSpace(name)))
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func findAccount(ctx context.Context, repository connector.MySQLRepository, digest string) (accountBinding, error) {
	sessions, err := repository.ListReadyAccountSessions(ctx, 1000)
	if err != nil {
		return accountBinding{}, fmt.Errorf("list ready Connector sessions: %w", err)
	}
	var match *accountBinding
	for _, item := range sessions {
		externalID, resolveErr := repository.ResolveAnyExternalAccountID(ctx, item.OrganizationID, item.ProjectID, item.AccountID)
		if resolveErr != nil || digestString(externalID) != digest {
			continue
		}
		fullSession, getErr := repository.GetAccountSession(ctx, item.OrganizationID, item.AccountID)
		if getErr != nil {
			return accountBinding{}, fmt.Errorf("load Connector session: %w", getErr)
		}
		candidate := accountBinding{organizationID: item.OrganizationID, projectID: item.ProjectID, accountID: item.AccountID, externalID: externalID, session: fullSession}
		if match != nil && match.accountID != candidate.accountID {
			return accountBinding{}, fmt.Errorf("account digest matched multiple Connector accounts")
		}
		match = &candidate
	}
	if match == nil {
		return accountBinding{}, fmt.Errorf("selected Edge account has no ready Connector session")
	}
	return *match, nil
}

func replaceDate(value any, date string) string {
	original, _ := value.(string)
	if len(original) >= 10 && original[4] == '-' && original[7] == '-' {
		return date + original[10:]
	}
	return date
}

func decodeObject(data json.RawMessage) map[string]any {
	var value map[string]any
	_ = json.Unmarshal(data, &value)
	return value
}

func businessCode(value map[string]any) int {
	switch code := value["code"].(type) {
	case float64:
		return int(code)
	case json.Number:
		parsed, _ := code.Int64()
		return int(parsed)
	}
	return 0
}

func exactNameObjects(value any, name string) []map[string]any {
	var result []map[string]any
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if typedName, ok := typed["name"].(string); ok && typedName == name {
				result = append(result, typed)
				return
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return result
}

func exactNameDigestObjects(value any, nameDigest string) []map[string]any {
	var result []map[string]any
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if name, ok := typed["name"].(string); ok && digestString(name) == nameDigest {
				result = append(result, typed)
				return
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return result
}

func errorKind(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, oceanengine.ErrCSRFTokenInvalid):
		return "csrf_token_invalid"
	case errors.Is(err, oceanengine.ErrAuthRequired):
		return "auth_required"
	case errors.Is(err, oceanengine.ErrResultUnknown):
		return "transport_unknown"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		var status oceanengine.HTTPStatusError
		if errors.As(err, &status) {
			return fmt.Sprintf("http_%d", status.StatusCode)
		}
		return "request_failed"
	}
}

func findNumericID(value any) string {
	preferred := []string{"project_id", "campaign_id", "id"}
	if object, ok := value.(map[string]any); ok {
		for _, key := range preferred {
			if found := numericString(object[key]); found != "" {
				return found
			}
		}
		for _, child := range object {
			if found := findNumericID(child); found != "" {
				return found
			}
		}
	}
	if list, ok := value.([]any); ok {
		for _, child := range list {
			if found := findNumericID(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func numericString(value any) string {
	switch typed := value.(type) {
	case string:
		if typed != "" && strings.IndexFunc(typed, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
			return typed
		}
	case float64:
		if typed > 0 && typed == float64(int64(typed)) {
			return fmt.Sprintf("%.0f", typed)
		}
	case json.Number:
		return string(typed)
	}
	return ""
}

func digestString(value string) string { return digestBytes([]byte(value)) }

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func writeSummary(summary probeSummary) error {
	encoded, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
