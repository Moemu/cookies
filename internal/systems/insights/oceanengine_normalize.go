package insights

import (
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const OceanEnginePlatform Platform = "ocean_engine"
const OceanEngineWebAPI IngestMode = "web_api"

// OceanEngineMetricFact converts one connector metric window to the existing
// canonical daily fact shape without fabricating facts for empty reports.
func OceanEngineMetricFact(window oceanengine.MetricWindow, organizationID contract.OrganizationID, projectID contract.ProjectID, sourceID, batchID, factID string) (MetricFact, error) {
	if window.Kind.Valid() == false || window.ExternalID == "" {
		return MetricFact{}, fmt.Errorf("invalid ocean engine metric identity")
	}
	if window.End.Before(window.Start) {
		return MetricFact{}, fmt.Errorf("invalid ocean engine metric window")
	}
	return MetricFact{
		ID: factID, OrganizationID: organizationID, ProjectID: projectID,
		DataSourceID: sourceID, ImportBatchID: batchID,
		Platform: OceanEnginePlatform, PlatformObjectKind: string(window.Kind),
		PlatformObjectID: window.ExternalID, StatDate: window.Start,
		Caliber: MetricCaliber{TimeZone: window.TimeZone, Currency: window.Currency, AttributionWindow: window.AttributionWindow, MetricSchemaVersion: window.MetricSchema},
		Counts:  MetricCounts{Impressions: window.Counts.Impressions, Clicks: window.Counts.Clicks, Conversions: window.Counts.Conversions, SpendCents: window.Counts.SpendCents},
		Raw:     window.Raw, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, nil
}
