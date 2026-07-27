package remix

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrQualityReportNotFound = errors.New("quality report not found")

type QualityReportStore interface {
	CreateQualityReport(context.Context, QualityReport) (QualityReport, error)
	GetQualityReport(context.Context, contract.OrganizationID, contract.ProjectID, string) (QualityReport, error)
	GetQualityReportForRenderJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (QualityReport, error)
}

type QualityEvaluator interface {
	EvaluateQuality(context.Context, QualityEvaluationInput) (QualityEvaluationResult, error)
}

type QualityEvaluationInput struct {
	RenderJob   RenderJob
	OutputAsset *contract.ProjectAssetRef
	Policy      string
}

type QualityEvaluationResult struct {
	Verdict           QualityVerdict
	Score             float64
	Dimensions        []QualityDimension
	Issues            []QualityIssue
	Evidence          []QualityEvidence
	RepairSuggestions []string
}

type FakeQualityEvaluator struct{}

func (FakeQualityEvaluator) EvaluateQuality(_ context.Context, input QualityEvaluationInput) (QualityEvaluationResult, error) {
	plan := input.RenderJob.InputSnapshot.Plan
	if hasQualityRisk(plan, "critical") {
		return QualityEvaluationResult{
			Verdict: QualityVerdictCritical,
			Score:   0.32,
			Dimensions: []QualityDimension{
				{Name: "subject_defects", Score: 0.2, Verdict: string(QualityVerdictCritical), Summary: "主体在关键画面中出现严重崩坏"},
				{Name: "compliance", Score: 0.72, Verdict: string(QualityVerdictPass), Summary: "未发现明确合规风险"},
			},
			Issues: []QualityIssue{{
				Code:             "SUBJECT_COLLAPSE",
				Severity:         QualityVerdictCritical,
				Dimension:        "subject_defects",
				StartSeconds:     1.2,
				EndSeconds:       2.8,
				Description:      "主体五官和商品边缘严重变形，critical 质检阻断导出",
				RepairSuggestion: "替换 opening 镜头或重新生成主体稳定版本",
			}},
			Evidence: []QualityEvidence{{Kind: "vlm_frame", TimestampSec: 1.8, Summary: "fake VLM 检出主体崩坏帧"}},
			RepairSuggestions: []string{
				"替换 opening 镜头",
				"降低运动幅度后重新渲染",
			},
		}, nil
	}
	if hasQualityRisk(plan, "major") {
		return QualityEvaluationResult{
			Verdict: QualityVerdictMajor,
			Score:   0.64,
			Dimensions: []QualityDimension{
				{Name: "aesthetics", Score: 0.58, Verdict: string(QualityVerdictMajor), Summary: "画面可读性下降，需要人工复核"},
				{Name: "compliance", Score: 0.86, Verdict: string(QualityVerdictPass), Summary: "未发现明确合规风险"},
			},
			Issues: []QualityIssue{{
				Code:             "LOW_READABILITY",
				Severity:         QualityVerdictMajor,
				Dimension:        "aesthetics",
				StartSeconds:     5,
				EndSeconds:       6.5,
				Description:      "字幕和商品主体重叠，major 质检要求人工复核",
				RepairSuggestion: "调整字幕安全区并降低背景亮度",
			}},
			Evidence: []QualityEvidence{{Kind: "vlm_frame", TimestampSec: 5.8, Summary: "fake VLM 检出字幕遮挡主体"}},
			RepairSuggestions: []string{
				"调整字幕安全区",
				"降低背景亮度",
			},
		}, nil
	}
	return QualityEvaluationResult{
		Verdict: QualityVerdictPass,
		Score:   0.91,
		Dimensions: []QualityDimension{
			{Name: "subject_defects", Score: 0.94, Verdict: string(QualityVerdictPass), Summary: "主体边缘稳定"},
			{Name: "corruption", Score: 0.9, Verdict: string(QualityVerdictPass), Summary: "未发现画面崩坏"},
			{Name: "aesthetics", Score: 0.88, Verdict: string(QualityVerdictPass), Summary: "构图和字幕安全区可用"},
			{Name: "compliance", Score: 0.92, Verdict: string(QualityVerdictPass), Summary: "未发现明确合规风险"},
		},
		Issues:            []QualityIssue{},
		Evidence:          []QualityEvidence{{Kind: "vlm_summary", TimestampSec: 0, Summary: "fake VLM 质检通过"}},
		RepairSuggestions: []string{},
	}, nil
}

func hasQualityRisk(plan Plan, token string) bool {
	token = "quality:" + token
	for _, warning := range plan.Warnings {
		if warning == token {
			return true
		}
	}
	for _, segment := range plan.Segments {
		for _, shot := range segment.Shots {
			for _, risk := range shot.Risks {
				if risk == token {
					return true
				}
			}
		}
	}
	return false
}

type MemoryQualityReportStore struct {
	mu          sync.RWMutex
	reports     map[string]QualityReport
	byRenderJob map[string]string
}

func NewMemoryQualityReportStore() *MemoryQualityReportStore {
	return &MemoryQualityReportStore{
		reports:     map[string]QualityReport{},
		byRenderJob: map[string]string{},
	}
}

func (s *MemoryQualityReportStore) CreateQualityReport(_ context.Context, report QualityReport) (QualityReport, error) {
	if err := report.Validate(); err != nil {
		return QualityReport{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := qualityReportRenderScope(report.OrganizationID, report.ProjectID, report.RenderJobID)
	if existingID := s.byRenderJob[key]; existingID != "" {
		return QualityReport{}, fmt.Errorf("quality report already exists for render job")
	}
	s.reports[report.ID] = cloneQualityReport(report)
	s.byRenderJob[key] = report.ID
	return cloneQualityReport(report), nil
}

func (s *MemoryQualityReportStore) GetQualityReport(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (QualityReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	report, ok := s.reports[id]
	if !ok || report.OrganizationID != org || report.ProjectID != project {
		return QualityReport{}, ErrQualityReportNotFound
	}
	return cloneQualityReport(report), nil
}

func (s *MemoryQualityReportStore) GetQualityReportForRenderJob(_ context.Context, org contract.OrganizationID, project contract.ProjectID, renderJobID string) (QualityReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id := s.byRenderJob[qualityReportRenderScope(org, project, renderJobID)]
	if id == "" {
		return QualityReport{}, ErrQualityReportNotFound
	}
	report := s.reports[id]
	return cloneQualityReport(report), nil
}

type MySQLQualityReportStore struct{ DB *sql.DB }

func (s MySQLQualityReportStore) CreateQualityReport(ctx context.Context, report QualityReport) (QualityReport, error) {
	if s.DB == nil {
		return QualityReport{}, fmt.Errorf("quality report database is required")
	}
	if err := report.Validate(); err != nil {
		return QualityReport{}, err
	}
	dimensions, err := json.Marshal(report.Dimensions)
	if err != nil {
		return QualityReport{}, err
	}
	issues, err := json.Marshal(report.Issues)
	if err != nil {
		return QualityReport{}, err
	}
	evidence, err := json.Marshal(report.Evidence)
	if err != nil {
		return QualityReport{}, err
	}
	suggestions, err := json.Marshal(report.RepairSuggestions)
	if err != nil {
		return QualityReport{}, err
	}
	var outputAssetID any
	var outputVersion any
	if report.OutputAsset != nil {
		outputAssetID = report.OutputAsset.AssetVersion.AssetID
		outputVersion = report.OutputAsset.AssetVersion.Version
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO remix_quality_reports (
		id, organization_id, project_id, principal_kind, principal_id, render_job_id,
		output_asset_id, output_asset_version, verdict, score, dimensions, issues, evidence,
		repair_suggestions, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		report.ID, report.OrganizationID, report.ProjectID, report.CreatedBy.Kind, report.CreatedBy.ID,
		report.RenderJobID, outputAssetID, outputVersion, report.Verdict, report.Score, dimensions,
		issues, evidence, suggestions, report.CreatedAt, report.UpdatedAt)
	if err == nil {
		return cloneQualityReport(report), nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return QualityReport{}, fmt.Errorf("quality report already exists for render job")
	}
	return QualityReport{}, err
}

func (s MySQLQualityReportStore) GetQualityReport(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (QualityReport, error) {
	if s.DB == nil {
		return QualityReport{}, fmt.Errorf("quality report database is required")
	}
	return scanQualityReport(s.DB.QueryRowContext(ctx, qualityReportSelect+` WHERE organization_id=? AND project_id=? AND id=?`, org, project, id))
}

func (s MySQLQualityReportStore) GetQualityReportForRenderJob(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, renderJobID string) (QualityReport, error) {
	if s.DB == nil {
		return QualityReport{}, fmt.Errorf("quality report database is required")
	}
	return scanQualityReport(s.DB.QueryRowContext(ctx, qualityReportSelect+` WHERE organization_id=? AND project_id=? AND render_job_id=?`, org, project, renderJobID))
}

const qualityReportSelect = `SELECT id, organization_id, project_id, principal_kind, principal_id, render_job_id,
	output_asset_id, output_asset_version, verdict, score, dimensions, issues, evidence, repair_suggestions, created_at, updated_at FROM remix_quality_reports`

func scanQualityReport(row renderJobScanner) (QualityReport, error) {
	var report QualityReport
	var dimensions, issues, evidence, suggestions []byte
	var outputAssetID sql.NullString
	var outputVersion sql.NullInt64
	err := row.Scan(&report.ID, &report.OrganizationID, &report.ProjectID, &report.CreatedBy.Kind,
		&report.CreatedBy.ID, &report.RenderJobID, &outputAssetID, &outputVersion, &report.Verdict,
		&report.Score, &dimensions, &issues, &evidence, &suggestions, &report.CreatedAt, &report.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return QualityReport{}, ErrQualityReportNotFound
	}
	if err != nil {
		return QualityReport{}, err
	}
	if outputAssetID.Valid && outputVersion.Valid {
		report.OutputAsset = &contract.ProjectAssetRef{
			ProjectID:    report.ProjectID,
			AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID(outputAssetID.String), Version: outputVersion.Int64},
		}
	}
	if err := json.Unmarshal(dimensions, &report.Dimensions); err != nil {
		return QualityReport{}, err
	}
	if err := json.Unmarshal(issues, &report.Issues); err != nil {
		return QualityReport{}, err
	}
	if err := json.Unmarshal(evidence, &report.Evidence); err != nil {
		return QualityReport{}, err
	}
	if err := json.Unmarshal(suggestions, &report.RepairSuggestions); err != nil {
		return QualityReport{}, err
	}
	return cloneQualityReport(report), nil
}

func qualityReportRenderScope(org contract.OrganizationID, project contract.ProjectID, renderJobID string) string {
	return string(org) + "/" + string(project) + "/" + renderJobID
}
