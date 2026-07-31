package creative

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	CommerceProductCutTemplateID   = "commerce.product-cut"
	CommerceWindowRevealTemplateID = "commerce.window-reveal"
	CommerceOneClickTemplateID     = "commerce.one-click"
	CommerceMiniatureTemplateID    = "commerce.miniature"
	CommerceDeviceSummonTemplateID = "commerce.device-summon"
)

type VideoAudioPolicy string

const VideoAudioSilent VideoAudioPolicy = "silent"

type TimelinePurpose string

const (
	TimelineInformationGap       TimelinePurpose = "information_gap"
	TimelineSingleTransformation TimelinePurpose = "single_transformation"
	TimelineProductHold          TimelinePurpose = "product_hold"
)

type TemplateReference struct {
	ID      string `json:"template_id"`
	Version int64  `json:"template_version"`
}

type CommercePrerollPlanningInput struct {
	TaskID             string
	IntakeVersion      int64
	TemplateID         string
	TemplateVersion    int64
	BrandName          string
	ProductName        string
	ProductCategory    string
	SellingPoints      []string
	VisualKeywords     []string
	ProductAsset       contract.AssetVersionRef
	DurationSeconds    int
	AspectRatio        string
	Resolution         string
	AudioPolicy        VideoAudioPolicy
	MandatoryElements  []string
	ProhibitedElements []string
}

func (i CommercePrerollPlanningInput) Validate() error {
	if strings.TrimSpace(i.TaskID) == "" || i.IntakeVersion < 1 {
		return fmt.Errorf("creative task and intake version are required")
	}
	if !supportedCommerceTemplate(i.TemplateID) || i.TemplateVersion != 1 {
		return fmt.Errorf("unsupported commerce preroll template")
	}
	if strings.TrimSpace(i.BrandName) == "" || strings.TrimSpace(i.ProductName) == "" {
		return fmt.Errorf("brand and product are required")
	}
	if i.ProductAsset != (contract.AssetVersionRef{}) {
		if err := i.ProductAsset.Validate(); err != nil {
			return fmt.Errorf("product asset: %w", err)
		}
	}
	if i.DurationSeconds != 6 || i.AspectRatio != "9:16" || i.Resolution != "720p" || i.AudioPolicy != VideoAudioSilent {
		return fmt.Errorf("commerce preroll requires 6 seconds, 9:16, 720p, and silent output")
	}
	return nil
}

func supportedCommerceTemplate(templateID string) bool {
	switch templateID {
	case CommerceProductCutTemplateID,
		CommerceWindowRevealTemplateID,
		CommerceOneClickTemplateID,
		CommerceMiniatureTemplateID,
		CommerceDeviceSummonTemplateID:
		return true
	default:
		return false
	}
}

type CreativeFramePlan struct {
	ContractVersion string                   `json:"contract_version"`
	TaskID          string                   `json:"task_id"`
	Template        TemplateReference        `json:"template_ref"`
	ProductAsset    contract.AssetVersionRef `json:"product_asset_ref,omitempty"`
	WidthPixels     int                      `json:"width_pixels"`
	HeightPixels    int                      `json:"height_pixels"`
	StartFrameKind  string                   `json:"start_frame_kind"`
	TailFrameKind   string                   `json:"tail_frame_kind"`
}

type PromptTimelineSegment struct {
	StartSeconds float64         `json:"start_seconds"`
	EndSeconds   float64         `json:"end_seconds"`
	Purpose      TimelinePurpose `json:"purpose"`
	Instruction  string          `json:"instruction"`
}

type CreativeVideoPrompt struct {
	ContractVersion string                   `json:"contract_version"`
	TaskID          string                   `json:"task_id"`
	IntakeVersion   int64                    `json:"intake_version"`
	Template        TemplateReference        `json:"template_ref"`
	ProductAsset    contract.AssetVersionRef `json:"product_asset_ref,omitempty"`
	Version         int64                    `json:"prompt_version"`
	Fidelity        string                   `json:"fidelity"`
	Camera          string                   `json:"camera"`
	Environment     string                   `json:"environment"`
	Timeline        []PromptTimelineSegment  `json:"timeline"`
	Guardrails      []string                 `json:"guardrails"`
	CompiledPrompt  string                   `json:"compiled_prompt"`
	Hash            string                   `json:"prompt_hash"`
}

func (p *CreativeVideoPrompt) Seal() error {
	if p == nil ||
		p.ContractVersion != "creative-video-prompt/v1" ||
		strings.TrimSpace(p.TaskID) == "" ||
		p.IntakeVersion < 1 ||
		!supportedCommerceTemplate(p.Template.ID) ||
		p.Template.Version != 1 ||
		p.Version < 1 ||
		strings.TrimSpace(p.Fidelity) == "" ||
		strings.TrimSpace(p.Camera) == "" ||
		strings.TrimSpace(p.Environment) == "" ||
		strings.TrimSpace(p.CompiledPrompt) == "" ||
		len(p.Timeline) != 3 {
		return fmt.Errorf("creative video prompt is incomplete")
	}
	for index, segment := range p.Timeline {
		if segment.StartSeconds < 0 || segment.EndSeconds <= segment.StartSeconds ||
			strings.TrimSpace(segment.Instruction) == "" {
			return fmt.Errorf("creative video prompt timeline segment %d is invalid", index)
		}
		switch segment.Purpose {
		case TimelineInformationGap, TimelineSingleTransformation, TimelineProductHold:
		default:
			return fmt.Errorf("creative video prompt timeline segment %d has an invalid purpose", index)
		}
	}
	hash, err := contract.CanonicalJSONHash(struct {
		ContractVersion string                   `json:"contract_version"`
		TaskID          string                   `json:"task_id"`
		IntakeVersion   int64                    `json:"intake_version"`
		Template        TemplateReference        `json:"template_ref"`
		ProductAsset    contract.AssetVersionRef `json:"product_asset_ref"`
		Version         int64                    `json:"prompt_version"`
		Fidelity        string                   `json:"fidelity"`
		Camera          string                   `json:"camera"`
		Environment     string                   `json:"environment"`
		Timeline        []PromptTimelineSegment  `json:"timeline"`
		Guardrails      []string                 `json:"guardrails"`
		CompiledPrompt  string                   `json:"compiled_prompt"`
	}{
		ContractVersion: p.ContractVersion, TaskID: p.TaskID, IntakeVersion: p.IntakeVersion,
		Template: p.Template, ProductAsset: p.ProductAsset, Version: p.Version,
		Fidelity: p.Fidelity, Camera: p.Camera, Environment: p.Environment, Timeline: p.Timeline,
		Guardrails: p.Guardrails, CompiledPrompt: p.CompiledPrompt,
	})
	if err != nil {
		return fmt.Errorf("hash creative video prompt: %w", err)
	}
	p.Hash = "sha256:" + hash
	return nil
}

func (p CreativeVideoPrompt) ValidateHash() error {
	expected := p.Hash
	p.Hash = ""
	if err := p.Seal(); err != nil {
		return err
	}
	if !validSHA256Ref(expected) || p.Hash != expected {
		return fmt.Errorf("creative video prompt hash does not match its content")
	}
	return nil
}

type CreativeVideoGenerationSpec struct {
	ContractVersion    string                   `json:"contract_version"`
	TaskID             string                   `json:"task_id"`
	PromptHash         string                   `json:"prompt_hash"`
	ConditioningAssets []VideoConditioningAsset `json:"conditioning_assets"`
	DurationSeconds    int                      `json:"duration_seconds"`
	AspectRatio        string                   `json:"aspect_ratio"`
	Resolution         string                   `json:"resolution"`
	AudioPolicy        VideoAudioPolicy         `json:"audio_policy"`
	CandidateCount     int                      `json:"candidate_count"`
	GenerationReady    bool                     `json:"generation_ready"`
	ProductionReady    bool                     `json:"production_ready"`
	Hash               string                   `json:"generation_spec_hash"`
}

type CommercePrerollPlan struct {
	Template  TemplateReference           `json:"template"`
	FramePlan CreativeFramePlan           `json:"frame_plan"`
	Prompt    CreativeVideoPrompt         `json:"prompt"`
	Spec      CreativeVideoGenerationSpec `json:"generation_spec"`
}

type VideoConditioningRole string

const (
	VideoConditioningFirstFrame VideoConditioningRole = "first_frame"
	VideoConditioningLastFrame  VideoConditioningRole = "last_frame"
)

type VideoConditioningAsset struct {
	Role     VideoConditioningRole    `json:"role"`
	AssetRef contract.AssetVersionRef `json:"asset_ref"`
}

func (p CommercePrerollPlan) BindFrames(frames ConditionedFrames) (CreativeVideoGenerationSpec, error) {
	if err := frames.StartFrame.Validate(); err != nil {
		return CreativeVideoGenerationSpec{}, fmt.Errorf("start frame: %w", err)
	}
	if err := frames.TailFrame.Validate(); err != nil {
		return CreativeVideoGenerationSpec{}, fmt.Errorf("tail frame: %w", err)
	}
	if frames.StartFrame == frames.TailFrame {
		return CreativeVideoGenerationSpec{}, fmt.Errorf("start and tail frames must be distinct asset versions")
	}
	spec := p.Spec
	spec.ConditioningAssets = []VideoConditioningAsset{
		{Role: VideoConditioningFirstFrame, AssetRef: frames.StartFrame},
		{Role: VideoConditioningLastFrame, AssetRef: frames.TailFrame},
	}
	spec.GenerationReady = true
	if err := spec.Seal(); err != nil {
		return CreativeVideoGenerationSpec{}, err
	}
	return spec, nil
}

func (s *CreativeVideoGenerationSpec) Seal() error {
	if s.ContractVersion != "creative-video-generation-spec/v1" ||
		strings.TrimSpace(s.TaskID) == "" ||
		!validSHA256Ref(s.PromptHash) ||
		s.DurationSeconds != 6 ||
		s.AspectRatio != "9:16" ||
		s.Resolution != "720p" ||
		s.AudioPolicy != VideoAudioSilent ||
		s.CandidateCount != 1 ||
		!s.GenerationReady ||
		s.ProductionReady {
		return fmt.Errorf("commerce preroll video generation spec is incomplete")
	}
	if len(s.ConditioningAssets) != 2 ||
		s.ConditioningAssets[0].Role != VideoConditioningFirstFrame ||
		s.ConditioningAssets[1].Role != VideoConditioningLastFrame {
		return fmt.Errorf("generation spec requires ordered first and last frames")
	}
	for _, asset := range s.ConditioningAssets {
		if err := asset.AssetRef.Validate(); err != nil {
			return fmt.Errorf("%s asset: %w", asset.Role, err)
		}
	}
	if s.ConditioningAssets[0].AssetRef == s.ConditioningAssets[1].AssetRef {
		return fmt.Errorf("first and last frames must be distinct asset versions")
	}
	hash, err := contract.CanonicalJSONHash(struct {
		ContractVersion    string                   `json:"contract_version"`
		TaskID             string                   `json:"task_id"`
		PromptHash         string                   `json:"prompt_hash"`
		ConditioningAssets []VideoConditioningAsset `json:"conditioning_assets"`
		DurationSeconds    int                      `json:"duration_seconds"`
		AspectRatio        string                   `json:"aspect_ratio"`
		Resolution         string                   `json:"resolution"`
		AudioPolicy        VideoAudioPolicy         `json:"audio_policy"`
		CandidateCount     int                      `json:"candidate_count"`
		GenerationReady    bool                     `json:"generation_ready"`
		ProductionReady    bool                     `json:"production_ready"`
	}{
		ContractVersion: s.ContractVersion, TaskID: s.TaskID, PromptHash: s.PromptHash,
		ConditioningAssets: s.ConditioningAssets, DurationSeconds: s.DurationSeconds,
		AspectRatio: s.AspectRatio, Resolution: s.Resolution, AudioPolicy: s.AudioPolicy,
		CandidateCount: s.CandidateCount, GenerationReady: s.GenerationReady,
		ProductionReady: s.ProductionReady,
	})
	if err != nil {
		return fmt.Errorf("hash video generation spec: %w", err)
	}
	s.Hash = "sha256:" + hash
	return nil
}

func (s CreativeVideoGenerationSpec) ValidateHash() error {
	expected := s.Hash
	s.Hash = ""
	if err := s.Seal(); err != nil {
		return err
	}
	if !validSHA256Ref(expected) || s.Hash != expected {
		return fmt.Errorf("video generation spec hash does not match its content")
	}
	return nil
}

type VideoGenerationApproval struct {
	ContractVersion    string    `json:"contract_version"`
	TaskID             string    `json:"task_id"`
	GenerationSpecHash string    `json:"generation_spec_hash"`
	ConfirmedItems     []string  `json:"confirmed_items"`
	ConfirmedBy        string    `json:"confirmed_by"`
	ConfirmedAt        time.Time `json:"confirmed_at"`
}

func ApproveVideoGeneration(spec CreativeVideoGenerationSpec, principalID string, confirmedAt time.Time) (VideoGenerationApproval, error) {
	if strings.TrimSpace(principalID) == "" || confirmedAt.IsZero() {
		return VideoGenerationApproval{}, fmt.Errorf("principal and confirmation time are required")
	}
	if err := spec.Seal(); err != nil {
		return VideoGenerationApproval{}, err
	}
	return VideoGenerationApproval{
		ContractVersion: "creative-generation-approval/v1",
		TaskID:          spec.TaskID, GenerationSpecHash: spec.Hash,
		ConfirmedItems: []string{"product", "template", "motion", "result", "prompt", "paid_generation"},
		ConfirmedBy:    principalID, ConfirmedAt: confirmedAt.UTC(),
	}, nil
}

func (a VideoGenerationApproval) Authorizes(spec CreativeVideoGenerationSpec) error {
	if a.ContractVersion != "creative-generation-approval/v1" ||
		strings.TrimSpace(a.ConfirmedBy) == "" ||
		a.ConfirmedAt.IsZero() ||
		!validConfirmedItems(a.ConfirmedItems) {
		return fmt.Errorf("video generation approval is incomplete")
	}
	if err := spec.ValidateHash(); err != nil {
		return err
	}
	if a.TaskID != spec.TaskID || a.GenerationSpecHash != spec.Hash {
		return fmt.Errorf("video generation approval does not match the generation spec")
	}
	return nil
}

func validSHA256Ref(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return false
	}
	digest := strings.TrimPrefix(value, prefix)
	if digest != strings.ToLower(digest) {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

func validConfirmedItems(values []string) bool {
	required := map[string]struct{}{
		"product": {}, "template": {}, "motion": {}, "result": {}, "prompt": {}, "paid_generation": {},
	}
	if len(values) != len(required) {
		return false
	}
	for _, value := range values {
		if _, exists := required[value]; !exists {
			return false
		}
		delete(required, value)
	}
	return len(required) == 0
}

// CommercePrerollPlanner hides the template, timeline, prompt, and readiness
// rules behind one deterministic planning interface.
type CommercePrerollPlanner struct{}

func (CommercePrerollPlanner) Plan(input CommercePrerollPlanningInput) (CommercePrerollPlan, error) {
	if err := input.Validate(); err != nil {
		return CommercePrerollPlan{}, err
	}
	template := TemplateReference{ID: input.TemplateID, Version: input.TemplateVersion}
	directions := commerceTemplateDirectionsFor(input)
	timeline := directions.Timeline
	fidelity := fmt.Sprintf(
		"保持%s%s的真实商品外观、包装轮廓、比例、材质、颜色、标签文字与品牌标识不变。",
		input.BrandName,
		input.ProductName,
	)
	camera := directions.Camera
	guardrails := append([]string{}, input.MandatoryElements...)
	guardrails = append(guardrails, input.ProhibitedElements...)
	guardrails = append(guardrails,
		"不生成广告字幕、价格、促销信息或水印",
		"不增加第二件商品，手部不畸变，商品不穿模、不闪现、不漂移",
	)
	if len(input.SellingPoints) > 0 {
		guardrails = append(guardrails, "只可表现以下已确认卖点："+strings.Join(input.SellingPoints, "、"))
	}
	parts := []string{fidelity, camera}
	for _, segment := range timeline {
		parts = append(parts, segment.Instruction)
	}
	parts = append(parts, directions.Environment)
	parts = append(parts, strings.Join(guardrails, "；")+"。")
	compiled := strings.Join(parts, "\n")
	prompt := CreativeVideoPrompt{
		ContractVersion: "creative-video-prompt/v1", TaskID: input.TaskID, IntakeVersion: input.IntakeVersion,
		Template: template, ProductAsset: input.ProductAsset, Version: 1,
		Fidelity: fidelity, Camera: camera, Environment: directions.Environment, Timeline: timeline, Guardrails: guardrails,
		CompiledPrompt: compiled,
	}
	if err := prompt.Seal(); err != nil {
		return CommercePrerollPlan{}, err
	}
	return CommercePrerollPlan{
		Template: template,
		FramePlan: CreativeFramePlan{
			ContractVersion: "creative-frame-plan/v1", TaskID: input.TaskID, Template: template,
			ProductAsset: input.ProductAsset, WidthPixels: 720, HeightPixels: 1280,
			StartFrameKind: directions.StartFrameKind, TailFrameKind: directions.TailFrameKind,
		},
		Prompt: prompt,
		Spec: CreativeVideoGenerationSpec{
			ContractVersion: "creative-video-generation-spec/v1", TaskID: input.TaskID,
			PromptHash: prompt.Hash, DurationSeconds: input.DurationSeconds, AspectRatio: input.AspectRatio,
			Resolution: input.Resolution, AudioPolicy: input.AudioPolicy, CandidateCount: 1,
			GenerationReady: false, ProductionReady: false,
		},
	}, nil
}

type commerceTemplateDirections struct {
	Timeline       []PromptTimelineSegment
	Camera         string
	Environment    string
	StartFrameKind string
	TailFrameKind  string
}

func commerceTemplateDirectionsFor(input CommercePrerollPlanningInput) commerceTemplateDirections {
	product := strings.TrimSpace(input.ProductName)
	visualStyle := ""
	if len(input.VisualKeywords) > 0 {
		visualStyle = "，视觉关键词：" + strings.Join(input.VisualKeywords, "、")
	}
	commonCamera := "9:16 竖版写实商业摄影，固定构图，商品始终位于视觉中心" + visualStyle + "。"
	switch input.TemplateID {
	case CommerceProductCutTemplateID:
		return commerceTemplateDirections{
			Timeline: []PromptTimelineSegment{
				{StartSeconds: 0, EndSeconds: 1.5, Purpose: TimelineInformationGap, Instruction: "0.0–1.5 秒：完整商品稳定陈列，旁侧放置与商品质地或核心成分对应的可切割介质，建立感官期待。"},
				{StartSeconds: 1.5, EndSeconds: 4, Purpose: TimelineSingleTransformation, Instruction: "1.5–4.0 秒：一把干净刀具只切开旁侧介质，展示细腻截面或质地变化，不接触、不切割商品本体。"},
				{StartSeconds: 4, EndSeconds: 6, Purpose: TimelineProductHold, Instruction: fmt.Sprintf("4.0–6.0 秒：刀具退出，%s正面完整清晰，商品与质地截面共同稳定定格。", product)},
			},
			Camera:         commonCamera + " 微距质感、克制高光，切割动作清楚可读。",
			Environment:    "环境只允许切割介质产生少量真实碎屑，商品包装不得破损或变形。",
			StartFrameKind: "product_with_texture_medium",
			TailFrameKind:  "clear_product_with_texture_section",
		}
	case CommerceOneClickTemplateID:
		return commerceTemplateDirections{
			Timeline: []PromptTimelineSegment{
				{StartSeconds: 0, EndSeconds: 1.5, Purpose: TimelineInformationGap, Instruction: "0.0–1.5 秒：展示空置但符合品牌调性的台面或封闭展示位，商品尚未出现。"},
				{StartSeconds: 1.5, EndSeconds: 4, Purpose: TimelineSingleTransformation, Instruction: "1.5–4.0 秒：一只真实手指完成一次按压，展示位随即平稳打开，商品沿固定路径被取出或升起，只执行这一个触发动作。"},
				{StartSeconds: 4, EndSeconds: 6, Purpose: TimelineProductHold, Instruction: fmt.Sprintf("4.0–6.0 秒：%s回到画面中心并正面朝向镜头，手部离开，商品稳定定格。", product)},
			},
			Camera:         commonCamera + " 中近景，单次按压和商品出现路径连续可读。",
			Environment:    "触发装置必须符合真实使用场景，不生成不存在的屏幕界面或功能。",
			StartFrameKind: "empty_or_closed_display",
			TailFrameKind:  "clear_product_reveal",
		}
	case CommerceMiniatureTemplateID:
		sellingPoint := "已确认卖点"
		if len(input.SellingPoints) > 0 {
			sellingPoint = strings.Join(input.SellingPoints, "、")
		}
		return commerceTemplateDirections{
			Timeline: []PromptTimelineSegment{
				{StartSeconds: 0, EndSeconds: 1.5, Purpose: TimelineInformationGap, Instruction: fmt.Sprintf("0.0–1.5 秒：%s稳定陈列，微缩舞台尚未启动，建立功效如何被呈现的悬念。", product)},
				{StartSeconds: 1.5, EndSeconds: 4, Purpose: TimelineSingleTransformation, Instruction: fmt.Sprintf("1.5–4.0 秒：微缩角色或装置围绕商品完成一次连续演示，只用抽象、克制的视觉动作表达“%s”，不得新增功效结论。", sellingPoint)},
				{StartSeconds: 4, EndSeconds: 6, Purpose: TimelineProductHold, Instruction: fmt.Sprintf("4.0–6.0 秒：微缩动作停止并退居辅助位置，%s正面清晰稳定定格。", product)},
			},
			Camera:         commonCamera + " 微缩景深、真实材质，微缩元素不遮挡标签和品牌标识。",
			Environment:    "功效表现必须可解释为视觉隐喻，不能表现医疗结果、绝对效果或未经确认的数据。",
			StartFrameKind: "product_with_inactive_miniature_stage",
			TailFrameKind:  "clear_product_reveal",
		}
	case CommerceDeviceSummonTemplateID:
		return commerceTemplateDirections{
			Timeline: []PromptTimelineSegment{
				{StartSeconds: 0, EndSeconds: 1.5, Purpose: TimelineInformationGap, Instruction: "0.0–1.5 秒：展示与商品品类相符的真实场景装置，展示位闭合或空置，建立召回悬念。"},
				{StartSeconds: 1.5, EndSeconds: 4, Purpose: TimelineSingleTransformation, Instruction: "1.5–4.0 秒：装置完成一次机械滑出、翻转或升起动作，商品随装置平稳出现，不生成虚构应用界面。"},
				{StartSeconds: 4, EndSeconds: 6, Purpose: TimelineProductHold, Instruction: fmt.Sprintf("4.0–6.0 秒：装置停止，%s完整正面朝向镜头并稳定定格。", product)},
			},
			Camera:         commonCamera + " 中景，装置运动轴线明确，商品出现过程无遮挡。",
			Environment:    "美妆商品优先使用梳妆台、橱柜或展示台；只有真实 3C 商品才使用电子设备交互。",
			StartFrameKind: "closed_category_appropriate_device",
			TailFrameKind:  "clear_product_reveal",
		}
	default:
		return commerceTemplateDirections{
			Timeline: []PromptTimelineSegment{
				{StartSeconds: 0, EndSeconds: 1.5, Purpose: TimelineInformationGap, Instruction: "0.0–1.5 秒：雾面玻璃遮挡商品，只能看到位置稳定的商品轮廓，建立信息缺口。"},
				{StartSeconds: 1.5, EndSeconds: 4, Purpose: TimelineSingleTransformation, Instruction: "1.5–4.0 秒：一只戴手套的手左右擦拭玻璃，雾气连续消退，只执行这一个主动作。"},
				{StartSeconds: 4, EndSeconds: 6, Purpose: TimelineProductHold, Instruction: "4.0–6.0 秒：完整露出商品正面，包装文字与品牌标识清晰，画面稳定停留。"},
			},
			Camera:         "9:16 竖版写实商业摄影，中景，隔着玻璃橱窗，固定构图，品牌色侧光" + visualStyle + "。",
			Environment:    "环境只允许轻微光斑变化，不得争夺商品注意力。",
			StartFrameKind: "frosted_overlay",
			TailFrameKind:  "clear_product_reveal",
		}
	}
}
