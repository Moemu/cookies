package delivery

import "strings"

// RunPreflight is the single authoritative rule engine used by both the Plan
// preview endpoint and the ChangeSet execution gate.
func RunPreflight(version DeliveryPlanVersion) []PreflightCheck {
	trackingPresent := strings.TrimSpace(version.Tracking.LandingPage) != "" &&
		strings.TrimSpace(version.Tracking.PixelID) != "" &&
		strings.TrimSpace(version.Tracking.ConversionEvent) != ""
	advertiserPresent := strings.TrimSpace(version.Advertiser.ID) != "" &&
		strings.TrimSpace(version.Advertiser.Name) != ""
	creativePresent := len(version.CreativeReferences) > 0
	creativeConfirmed := creativePresent
	for _, reference := range version.CreativeReferences {
		if !reference.Confirmed {
			creativeConfirmed = false
			break
		}
	}
	return []PreflightCheck{
		check(
			"advertiser_available",
			CheckSeverityError,
			advertiserPresent,
			"Mock 广告主已选择，可用于投前验证。",
			"缺少可用的 mock 广告主。",
			RepairTarget{Field: "advertiser_id", Section: "目标与账户", Label: "选择 mock 广告主"},
		),
		check(
			"budget_positive",
			CheckSeverityError,
			version.Budget.TotalMinor > 0,
			"预算大于 0，可以进入投前验证。",
			"总预算必须大于 0。",
			RepairTarget{Field: "budget_total", Section: "预算与排期", Label: "修复总预算"},
		),
		check(
			"schedule_valid",
			CheckSeverityError,
			!version.Schedule.StartAt.IsZero() && version.Schedule.EndAt.After(version.Schedule.StartAt),
			"排期起止时间有效。",
			"排期结束时间必须晚于开始时间。",
			RepairTarget{Field: "schedule_start", Section: "预算与排期", Label: "修复投放排期"},
		),
		check(
			"creative_present",
			CheckSeverityError,
			creativePresent,
			"已引用至少一个素材版本。",
			"至少需要一个素材版本引用。",
			RepairTarget{Field: "creative_asset_id", Section: "素材引用", Label: "添加素材引用"},
		),
		check(
			"creative_confirmed",
			CheckSeverityWarning,
			creativeConfirmed,
			"所有素材版本均已人工确认。",
			"存在未人工确认的素材版本；当前为警告，执行前仍需确认。",
			RepairTarget{Field: "creative_confirmed", Section: "素材引用", Label: "确认素材版本"},
		),
		check(
			"tracking_complete",
			CheckSeverityError,
			trackingPresent,
			"落地页、像素和转化事件追踪完整。",
			"追踪缺失：请补齐落地页、像素和转化事件。",
			RepairTarget{Field: "tracking_pixel_id", Section: "追踪", Label: "修复追踪配置"},
		),
	}
}

func check(code string, severity CheckSeverity, passed bool, successMessage, failureMessage string, repair RepairTarget) PreflightCheck {
	if passed {
		return PreflightCheck{Code: code, Severity: severity, Passed: true, Message: successMessage}
	}
	return PreflightCheck{Code: code, Severity: severity, Passed: false, Message: failureMessage, Repair: &repair}
}

func scenarioFor(draft PlanDraft) Scenario {
	if draft.Budget.TotalMinor == 0 {
		return ScenarioBudgetZero
	}
	if strings.TrimSpace(draft.Tracking.LandingPage) == "" ||
		strings.TrimSpace(draft.Tracking.PixelID) == "" ||
		strings.TrimSpace(draft.Tracking.ConversionEvent) == "" {
		return ScenarioTrackingMissing
	}
	for _, reference := range draft.CreativeReferences {
		if !reference.Confirmed {
			return ScenarioCreativeUnconfirmed
		}
	}
	if strings.TrimSpace(draft.Advertiser.ID) == "" || len(draft.CreativeReferences) == 0 {
		return ScenarioIncompleteDraft
	}
	return ScenarioGoldenPath
}
