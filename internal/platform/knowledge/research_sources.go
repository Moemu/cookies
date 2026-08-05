package knowledge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func canonicalResearchURL(raw string) (canonical, domain string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" || parsed.User != nil {
		return "", "", fmt.Errorf("research source URL is invalid")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	return parsed.String(), parsed.Hostname(), nil
}

func normalizedSourceClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "web", "toutiao", "douyin", "weather":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func normalizedMediaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "article", "video", "data":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func (s Service) insertResearchSource(
	ctx context.Context,
	tx *sql.Tx,
	run ResearchRun,
	artifactID string,
	source ExternalResearchSource,
) (ResearchSource, error) {
	canonical, domain, err := canonicalResearchURL(source.URL)
	if err != nil {
		return ResearchSource{}, err
	}
	urlSum := sha256.Sum256([]byte(canonical))
	urlHash := hex.EncodeToString(urlSum[:])

	var value ResearchSource
	var publishedAt sql.NullTime
	var metadata []byte
	err = tx.QueryRowContext(ctx, `SELECT id, organization_id, project_id, research_run_id,
		source_class, media_type, title, source_url, canonical_url, source_domain,
		published_at, retrieved_at, verification_status, content_hash,
		COALESCE(provider_metadata, JSON_OBJECT())
		FROM platform_research_sources
		WHERE organization_id = ? AND project_id = ? AND research_run_id = ?
			AND canonical_url_hash = ?`,
		run.OrganizationID, run.ProjectID, run.ID, urlHash,
	).Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.ResearchRunID,
		&value.SourceClass, &value.MediaType, &value.Title, &value.URL, &value.CanonicalURL,
		&value.Domain, &publishedAt, &value.RetrievedAt, &value.VerificationStatus,
		&value.ContentHash, &metadata,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ResearchSource{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		id, idErr := s.newID("researchsource")
		if idErr != nil {
			return ResearchSource{}, idErr
		}
		title := strings.TrimSpace(source.Title)
		if title == "" {
			title = domain
		}
		hashValue := struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		}{Title: title, URL: canonical}
		contentHash, hashErr := contract.CanonicalJSONHash(hashValue)
		if hashErr != nil {
			return ResearchSource{}, hashErr
		}
		now := s.now()
		value = ResearchSource{
			ID: id, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID,
			ResearchRunID: run.ID, SourceClass: normalizedSourceClass(source.SourceClass),
			MediaType: normalizedMediaType(source.MediaType), Title: title, URL: strings.TrimSpace(source.URL),
			CanonicalURL: canonical, Domain: domain, PublishedAt: source.PublishedAt,
			RetrievedAt: now, VerificationStatus: "model_cited", ContentHash: contentHash,
		}
		if _, insertErr := tx.ExecContext(ctx, `INSERT INTO platform_research_sources
			(id, organization_id, project_id, research_run_id, source_class, media_type,
			 title, source_url, canonical_url, canonical_url_hash, source_domain,
			 published_at, retrieved_at, verification_status, content_hash,
			 provider_metadata, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			value.ID, value.OrganizationID, value.ProjectID, value.ResearchRunID,
			value.SourceClass, value.MediaType, value.Title, value.URL, value.CanonicalURL,
			urlHash, value.Domain, value.PublishedAt, value.RetrievedAt,
			value.VerificationStatus, value.ContentHash, nullableJSON(source.ProviderLocator), now,
		); insertErr != nil {
			return ResearchSource{}, insertErr
		}
	} else {
		value.PublishedAt = nullTimePointer(publishedAt)
		if len(metadata) > 0 && string(metadata) != "{}" {
			value.ProviderLocator = append(json.RawMessage(nil), metadata...)
		}
	}

	value.StartIndex = source.StartIndex
	value.EndIndex = source.EndIndex
	value.SupportLevel = "model_cited"
	value.ProviderLocator = append(json.RawMessage(nil), source.ProviderLocator...)
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO platform_research_citations
		(organization_id, project_id, research_artifact_id, research_source_id,
		 output_start, output_end, support_level, provider_locator, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.OrganizationID, run.ProjectID, artifactID, value.ID,
		source.StartIndex, source.EndIndex, value.SupportLevel,
		nullableJSON(source.ProviderLocator), s.now(),
	); err != nil {
		return ResearchSource{}, err
	}
	return value, nil
}

func (s Service) listArtifactSources(
	ctx context.Context,
	organizationID, projectID, artifactID string,
) ([]ResearchSource, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT
		source.id, source.organization_id, source.project_id, source.research_run_id,
		source.source_class, source.media_type, source.title, source.source_url,
		source.canonical_url, source.source_domain, source.published_at,
		source.retrieved_at, source.verification_status, source.content_hash,
		citation.output_start, citation.output_end, citation.support_level,
		COALESCE(citation.provider_locator, JSON_OBJECT())
		FROM platform_research_citations citation
		JOIN platform_research_sources source
			ON source.organization_id = citation.organization_id
			AND source.project_id = citation.project_id
			AND source.id = citation.research_source_id
		WHERE citation.organization_id = ? AND citation.project_id = ?
			AND citation.research_artifact_id = ?
		ORDER BY citation.output_start ASC, source.created_at ASC`,
		organizationID, projectID, artifactID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]ResearchSource, 0)
	for rows.Next() {
		var value ResearchSource
		var publishedAt sql.NullTime
		var locator []byte
		if err := rows.Scan(
			&value.ID, &value.OrganizationID, &value.ProjectID, &value.ResearchRunID,
			&value.SourceClass, &value.MediaType, &value.Title, &value.URL,
			&value.CanonicalURL, &value.Domain, &publishedAt, &value.RetrievedAt,
			&value.VerificationStatus, &value.ContentHash, &value.StartIndex,
			&value.EndIndex, &value.SupportLevel, &locator,
		); err != nil {
			return nil, err
		}
		value.PublishedAt = nullTimePointer(publishedAt)
		if len(locator) > 0 && string(locator) != "{}" {
			value.ProviderLocator = append(json.RawMessage(nil), locator...)
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 || !json.Valid(value) {
		return nil
	}
	return []byte(value)
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
