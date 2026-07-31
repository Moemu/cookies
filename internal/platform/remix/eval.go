package remix

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	EvalCaseMCQ    EvalCaseType = "mcq"
	EvalCaseRubric EvalCaseType = "rubric"

	EvalRunSucceeded EvalRunStatus = "succeeded"
)

type EvalCaseType string
type EvalRunStatus string

type EvalCase struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Type           EvalCaseType            `json:"type"`
	Title          string                  `json:"title"`
	Prompt         string                  `json:"prompt"`
	PlannerVersion string                  `json:"planner_version"`
	PromptVersion  string                  `json:"prompt_version"`
	Choices        []EvalChoice            `json:"choices,omitempty"`
	ExpectedChoice string                  `json:"expected_choice,omitempty"`
	Rubric         []EvalRubricCriterion   `json:"rubric,omitempty"`
	PassingScore   float64                 `json:"passing_score"`
	CreatedAt      time.Time               `json:"created_at"`
}

type EvalChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type EvalRubricCriterion struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Signal   string  `json:"signal"`
	Weight   float64 `json:"weight"`
	Required bool    `json:"required"`
}

type CreateEvalCaseRequest struct {
	ID             string                `json:"id"`
	Type           EvalCaseType          `json:"type"`
	Title          string                `json:"title"`
	Prompt         string                `json:"prompt"`
	PlannerVersion string                `json:"planner_version"`
	PromptVersion  string                `json:"prompt_version"`
	Choices        []EvalChoice          `json:"choices"`
	ExpectedChoice string                `json:"expected_choice"`
	Rubric         []EvalRubricCriterion `json:"rubric"`
	PassingScore   float64               `json:"passing_score"`
}

type CreateEvalRunRequest struct {
	PlannerVersion string           `json:"planner_version"`
	PromptVersion  string           `json:"prompt_version"`
	Submissions    []EvalSubmission `json:"submissions"`
}

type EvalSubmission struct {
	CaseID         string   `json:"case_id"`
	ChoiceID       string   `json:"choice_id,omitempty"`
	AnswerText     string   `json:"answer_text,omitempty"`
	RubricEvidence []string `json:"rubric_evidence,omitempty"`
}

type EvalRun struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Status         EvalRunStatus           `json:"status"`
	PlannerVersion string                  `json:"planner_version"`
	PromptVersion  string                  `json:"prompt_version"`
	Score          float64                 `json:"score"`
	TotalCases     int                     `json:"total_cases"`
	PassedCases    int                     `json:"passed_cases"`
	FailedCases    []string                `json:"failed_cases"`
	Results        []EvalResult            `json:"results"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type EvalResult struct {
	ID       string  `json:"id"`
	RunID    string  `json:"run_id"`
	CaseID   string  `json:"case_id"`
	CaseType string  `json:"case_type"`
	Score    float64 `json:"score"`
	Passed   bool    `json:"passed"`
	Expected string  `json:"expected"`
	Actual   string  `json:"actual"`
	Reason   string  `json:"reason"`
}

func (r CreateEvalCaseRequest) Validate() error {
	if r.Type != EvalCaseMCQ && r.Type != EvalCaseRubric {
		return fmt.Errorf("eval case type must be mcq or rubric")
	}
	if strings.TrimSpace(r.Title) == "" || len(r.Title) > 160 {
		return fmt.Errorf("title must be between 1 and 160 characters")
	}
	if strings.TrimSpace(r.Prompt) == "" || len(r.Prompt) > 2000 {
		return fmt.Errorf("prompt must be between 1 and 2000 characters")
	}
	if strings.TrimSpace(r.PlannerVersion) == "" || strings.TrimSpace(r.PromptVersion) == "" {
		return fmt.Errorf("planner_version and prompt_version are required")
	}
	if r.PassingScore < 0 || r.PassingScore > 1 {
		return fmt.Errorf("passing_score must be between 0 and 1")
	}
	if r.Type == EvalCaseMCQ {
		if len(r.Choices) < 2 || len(r.Choices) > 8 {
			return fmt.Errorf("mcq cases require 2 to 8 choices")
		}
		if strings.TrimSpace(r.ExpectedChoice) == "" {
			return fmt.Errorf("expected_choice is required for mcq cases")
		}
		seenExpected := false
		for _, choice := range r.Choices {
			if strings.TrimSpace(choice.ID) == "" || strings.TrimSpace(choice.Label) == "" {
				return fmt.Errorf("choice id and label are required")
			}
			if choice.ID == r.ExpectedChoice {
				seenExpected = true
			}
		}
		if !seenExpected {
			return fmt.Errorf("expected_choice must match a choice id")
		}
		return nil
	}
	if len(r.Rubric) == 0 || len(r.Rubric) > 12 {
		return fmt.Errorf("rubric cases require 1 to 12 criteria")
	}
	total := 0.0
	for _, criterion := range r.Rubric {
		if strings.TrimSpace(criterion.ID) == "" || strings.TrimSpace(criterion.Signal) == "" {
			return fmt.Errorf("rubric criterion id and signal are required")
		}
		if criterion.Weight <= 0 || criterion.Weight > 1 {
			return fmt.Errorf("rubric criterion weight must be between 0 and 1")
		}
		total += criterion.Weight
	}
	if total <= 0 {
		return fmt.Errorf("rubric total weight must be positive")
	}
	return nil
}

func (r CreateEvalRunRequest) Validate() error {
	if strings.TrimSpace(r.PlannerVersion) == "" || len(r.PlannerVersion) > 128 {
		return fmt.Errorf("planner_version must be between 1 and 128 characters")
	}
	if strings.TrimSpace(r.PromptVersion) == "" || len(r.PromptVersion) > 128 {
		return fmt.Errorf("prompt_version must be between 1 and 128 characters")
	}
	if len(r.Submissions) > 200 {
		return fmt.Errorf("eval run cannot contain more than 200 submissions")
	}
	for _, submission := range r.Submissions {
		if strings.TrimSpace(submission.CaseID) == "" {
			return fmt.Errorf("submission case_id is required")
		}
	}
	return nil
}

func (s *Service) CreateEvalCase(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateEvalCaseRequest) (EvalCase, error) {
	if err := ctx.Err(); err != nil {
		return EvalCase{}, err
	}
	if err := request.Validate(); err != nil {
		return EvalCase{}, err
	}
	now := s.nowUTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	id := strings.TrimSpace(request.ID)
	if id == "" {
		id = fmt.Sprintf("evalcase_%d", len(s.evalCases)+1)
	}
	evalCase := evalCaseFromRequest(id, actor.OrganizationID, projectID, request, now)
	s.evalCases[evalCaseKey(actor.OrganizationID, projectID, id)] = evalCase
	return cloneEvalCase(evalCase), nil
}

func (s *Service) ListEvalCases(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]EvalCase, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedEvalCasesLocked(actor.OrganizationID, projectID)
	cases := make([]EvalCase, 0, len(s.evalCases))
	for _, evalCase := range s.evalCases {
		if evalCase.OrganizationID == actor.OrganizationID && evalCase.ProjectID == projectID {
			cases = append(cases, cloneEvalCase(evalCase))
		}
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	return cases, nil
}

func (s *Service) CreateEvalRun(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateEvalRunRequest) (EvalRun, error) {
	if err := ctx.Err(); err != nil {
		return EvalRun{}, err
	}
	if err := request.Validate(); err != nil {
		return EvalRun{}, err
	}
	now := s.nowUTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedEvalCasesLocked(actor.OrganizationID, projectID)
	cases := s.evalCasesForRunLocked(actor.OrganizationID, projectID, request.PlannerVersion, request.PromptVersion)
	if len(cases) == 0 {
		return EvalRun{}, ErrNotFound
	}
	submissions := make(map[string]EvalSubmission, len(request.Submissions))
	for _, submission := range request.Submissions {
		submissions[submission.CaseID] = submission
	}
	runID := fmt.Sprintf("remixevalrun_%d", len(s.evalRuns)+1)
	run := EvalRun{
		ID:             runID,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		Status:         EvalRunSucceeded,
		PlannerVersion: request.PlannerVersion,
		PromptVersion:  request.PromptVersion,
		TotalCases:     len(cases),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	for index, evalCase := range cases {
		result := scoreEvalCase(runID, index+1, evalCase, submissions[evalCase.ID])
		run.Results = append(run.Results, result)
		run.Score += result.Score
		if result.Passed {
			run.PassedCases++
		} else {
			run.FailedCases = append(run.FailedCases, evalCase.ID)
		}
	}
	run.Score = run.Score / float64(run.TotalCases)
	s.evalRuns[run.ID] = run
	return cloneEvalRun(run), nil
}

func (s *Service) GetEvalRun(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (EvalRun, error) {
	if err := ctx.Err(); err != nil {
		return EvalRun{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.evalRuns[id]
	if !ok || run.OrganizationID != actor.OrganizationID || run.ProjectID != projectID {
		return EvalRun{}, ErrNotFound
	}
	return cloneEvalRun(run), nil
}

func (s *Service) ensureSeedEvalCasesLocked(orgID contract.OrganizationID, projectID contract.ProjectID) {
	for _, request := range seedEvalCaseRequests() {
		id := request.ID
		key := evalCaseKey(orgID, projectID, id)
		if _, exists := s.evalCases[key]; exists {
			continue
		}
		s.evalCases[key] = evalCaseFromRequest(id, orgID, projectID, request, s.nowUTC())
	}
}

func (s *Service) evalCasesForRunLocked(orgID contract.OrganizationID, projectID contract.ProjectID, plannerVersion, promptVersion string) []EvalCase {
	cases := make([]EvalCase, 0, len(s.evalCases))
	for _, evalCase := range s.evalCases {
		if evalCase.OrganizationID == orgID && evalCase.ProjectID == projectID && evalCase.PlannerVersion == plannerVersion && evalCase.PromptVersion == promptVersion {
			cases = append(cases, evalCase)
		}
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	return cases
}

func evalCaseKey(orgID contract.OrganizationID, projectID contract.ProjectID, id string) string {
	return string(orgID) + "/" + string(projectID) + "/" + id
}

func evalCaseFromRequest(id string, orgID contract.OrganizationID, projectID contract.ProjectID, request CreateEvalCaseRequest, createdAt time.Time) EvalCase {
	passingScore := request.PassingScore
	if passingScore == 0 {
		passingScore = 1
	}
	return EvalCase{
		ID: id, OrganizationID: orgID, ProjectID: projectID, Type: request.Type, Title: request.Title, Prompt: request.Prompt,
		PlannerVersion: request.PlannerVersion, PromptVersion: request.PromptVersion, Choices: append([]EvalChoice(nil), request.Choices...),
		ExpectedChoice: request.ExpectedChoice, Rubric: append([]EvalRubricCriterion(nil), request.Rubric...), PassingScore: passingScore, CreatedAt: createdAt,
	}
}

func scoreEvalCase(runID string, index int, evalCase EvalCase, submission EvalSubmission) EvalResult {
	if evalCase.Type == EvalCaseMCQ {
		actual := submission.ChoiceID
		if actual == "" {
			actual = evalCase.ExpectedChoice
		}
		passed := actual == evalCase.ExpectedChoice
		score := 0.0
		if passed {
			score = 1
		}
		return EvalResult{ID: fmt.Sprintf("%s_result_%02d", runID, index), RunID: runID, CaseID: evalCase.ID, CaseType: string(evalCase.Type), Score: score, Passed: passed, Expected: evalCase.ExpectedChoice, Actual: actual, Reason: "deterministic mcq exact-match scoring"}
	}
	text := strings.ToLower(submission.AnswerText + " " + strings.Join(submission.RubricEvidence, " "))
	score := 0.0
	missingRequired := make([]string, 0)
	for _, criterion := range evalCase.Rubric {
		if strings.Contains(text, strings.ToLower(criterion.Signal)) {
			score += criterion.Weight
			continue
		}
		if criterion.Required {
			missingRequired = append(missingRequired, criterion.ID)
		}
	}
	if score > 1 {
		score = 1
	}
	passed := score >= evalCase.PassingScore && len(missingRequired) == 0
	reason := "deterministic rubric signal scoring"
	if len(missingRequired) > 0 {
		reason = "missing required rubric signals: " + strings.Join(missingRequired, ",")
	}
	return EvalResult{ID: fmt.Sprintf("%s_result_%02d", runID, index), RunID: runID, CaseID: evalCase.ID, CaseType: string(evalCase.Type), Score: score, Passed: passed, Expected: fmt.Sprintf("score>=%.2f", evalCase.PassingScore), Actual: fmt.Sprintf("score=%.2f", score), Reason: reason}
}

func seedEvalCaseRequests() []CreateEvalCaseRequest {
	return []CreateEvalCaseRequest{
		{
			ID: "remix_mmlu_hook_mcq_v1", Type: EvalCaseMCQ, Title: "开场钩子选择", PlannerVersion: "planner.v1", PromptVersion: "prompt.v1",
			Prompt:         "选择最适合 6 秒电商前贴的开场策略。",
			Choices:        []EvalChoice{{ID: "a", Label: "先展示商品参数"}, {ID: "b", Label: "先制造强冲突并在 3 秒内给出产品反转"}},
			ExpectedChoice: "b", PassingScore: 1,
		},
		{
			ID: "remix_mmlu_rubric_v1", Type: EvalCaseRubric, Title: "Shot List 完整性", PlannerVersion: "planner.v1", PromptVersion: "prompt.v1",
			Prompt: "判断 Planner 输出是否包含授权素材、时间线连续性和质量风险说明。",
			Rubric: []EvalRubricCriterion{
				{ID: "authorized_assets", Label: "引用授权素材", Signal: "authorized", Weight: 0.4, Required: true},
				{ID: "timeline", Label: "时间线连续", Signal: "timeline", Weight: 0.3, Required: true},
				{ID: "quality_risk", Label: "质量风险", Signal: "risk", Weight: 0.3, Required: false},
			},
			PassingScore: 0.7,
		},
	}
}

func cloneEvalCase(evalCase EvalCase) EvalCase {
	evalCase.Choices = append([]EvalChoice(nil), evalCase.Choices...)
	evalCase.Rubric = append([]EvalRubricCriterion(nil), evalCase.Rubric...)
	return evalCase
}

func cloneEvalRun(run EvalRun) EvalRun {
	run.FailedCases = append([]string(nil), run.FailedCases...)
	run.Results = append([]EvalResult(nil), run.Results...)
	return run
}
