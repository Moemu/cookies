package connector

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const LaunchBatchCalibrationSchemaVersion = "connector-launch-batch-calibration/v1"

type LaunchBatchCalibrationSnapshot struct {
	ID                     string                                  `json:"id"`
	OrganizationID         string                                  `json:"organization_id"`
	AccountID              string                                  `json:"account_id"`
	SchemaVersion          string                                  `json:"schema_version"`
	ModelVersion           string                                  `json:"model_version"`
	Status                 string                                  `json:"status"`
	TrainingBatches        int                                     `json:"training_batches"`
	TrainingDates          int                                     `json:"training_dates"`
	EvaluationBatches      int                                     `json:"evaluation_batches"`
	EvaluationDates        int                                     `json:"evaluation_dates"`
	BreakoutThresholdMinor float64                                 `json:"breakout_threshold_minor"`
	BreakoutProbability    float64                                 `json:"breakout_probability"`
	Typical                []LaunchBatchScenarioMetricDistribution `json:"typical"`
	Breakout               []LaunchBatchScenarioMetricDistribution `json:"breakout"`
	BrierScore             float64                                 `json:"brier_score"`
	CalibrationError       float64                                 `json:"calibration_error"`
	FinalBrierScore        float64                                 `json:"final_brier_score"`
	FinalCalibrationError  float64                                 `json:"final_calibration_error"`
	SourceHashes           []string                                `json:"source_hashes"`
	PayloadHash            string                                  `json:"payload_hash"`
	CreatedAt              time.Time                               `json:"created_at"`
}

func NewLaunchBatchCalibrationSnapshot(organizationID, accountID string, result LaunchBatchBacktestResult, sources []OfflineXLSXSource, now time.Time) (LaunchBatchCalibrationSnapshot, error) {
	organizationID, accountID = strings.TrimSpace(organizationID), strings.TrimSpace(accountID)
	if organizationID == "" || !strings.HasPrefix(accountID, "oeacct_") || result.Status != "ready_for_probabilistic_shadow" || len(result.Scenario.Typical) == 0 || len(result.Scenario.Breakout) == 0 {
		return LaunchBatchCalibrationSnapshot{}, ErrInvalidFact
	}
	hashes := make([]string, 0, len(sources))
	for _, source := range sources {
		digest := sha256.Sum256(source.Content)
		hashes = append(hashes, hex.EncodeToString(digest[:]))
	}
	value := LaunchBatchCalibrationSnapshot{
		OrganizationID: organizationID, AccountID: accountID, SchemaVersion: LaunchBatchCalibrationSchemaVersion,
		ModelVersion: result.ModelVersion, Status: result.Status, TrainingBatches: result.Split.TrainingBatches,
		TrainingDates: result.Split.TrainingDates, EvaluationBatches: result.Split.EvaluationBatches,
		EvaluationDates: result.Split.EvaluationDates, BreakoutThresholdMinor: result.Scenario.ThresholdSpendMinor,
		BreakoutProbability: result.Scenario.BreakoutProbability, Typical: append([]LaunchBatchScenarioMetricDistribution(nil), result.Scenario.Typical...),
		Breakout: append([]LaunchBatchScenarioMetricDistribution(nil), result.Scenario.Breakout...), BrierScore: result.ScenarioEvaluation.BrierScore,
		CalibrationError: result.ScenarioEvaluation.AbsoluteCalibrationError, FinalBrierScore: result.FinalScenarioEvaluation.BrierScore,
		FinalCalibrationError: result.FinalScenarioEvaluation.AbsoluteCalibrationError, SourceHashes: hashes, CreatedAt: now.UTC(),
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return LaunchBatchCalibrationSnapshot{}, err
	}
	digest := sha256.Sum256(payload)
	value.PayloadHash = hex.EncodeToString(digest[:])
	value.ID = "oecal_" + value.PayloadHash[:32]
	return value, nil
}

func (r MySQLRepository) AppendLaunchBatchCalibration(ctx context.Context, value LaunchBatchCalibrationSnapshot) (bool, error) {
	if value.ID == "" || value.OrganizationID == "" || value.AccountID == "" || value.SchemaVersion != LaunchBatchCalibrationSchemaVersion || value.Status != "ready_for_probabilistic_shadow" || value.PayloadHash == "" || value.CreatedAt.IsZero() {
		return false, ErrInvalidFact
	}
	db, err := r.db()
	if err != nil {
		return false, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	result, err := db.ExecContext(ctx, `INSERT IGNORE INTO connector_launch_batch_calibrations (id,organization_id,account_id,schema_version,model_version,status,payload_hash,prior_json,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.AccountID, value.SchemaVersion, value.ModelVersion, value.Status, value.PayloadHash, payload, value.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("append launch batch calibration: %w", err)
	}
	return inserted(result)
}

func (r MySQLRepository) LatestLaunchBatchCalibration(ctx context.Context, organizationID, accountID string) (LaunchBatchCalibrationSnapshot, error) {
	db, err := r.db()
	if err != nil {
		return LaunchBatchCalibrationSnapshot{}, err
	}
	var payload []byte
	err = db.QueryRowContext(ctx, `SELECT prior_json FROM connector_launch_batch_calibrations WHERE organization_id=? AND account_id=? AND status='ready_for_probabilistic_shadow' ORDER BY created_at DESC,id DESC LIMIT 1`, organizationID, accountID).Scan(&payload)
	if err != nil {
		return LaunchBatchCalibrationSnapshot{}, err
	}
	var value LaunchBatchCalibrationSnapshot
	if err = json.Unmarshal(payload, &value); err != nil {
		return LaunchBatchCalibrationSnapshot{}, fmt.Errorf("decode launch batch calibration: %w", err)
	}
	if value.OrganizationID != organizationID || value.AccountID != accountID {
		return LaunchBatchCalibrationSnapshot{}, sql.ErrNoRows
	}
	return value, nil
}
