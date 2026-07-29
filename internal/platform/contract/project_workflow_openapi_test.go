package contract

import (
	"os"
	"strings"
	"testing"
)

func TestProjectWorkflowOpenAPIContractIncludesTask3Routes(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi/platform-v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	document := string(data)
	for _, required := range []string{
		"/platform/v1/projects/{project_id}:",
		"operationId: getProjectDetail",
		"/platform/v1/projects/{project_id}/tasks:",
		"operationId: createProjectTask",
		"/platform/v1/projects/{project_id}/tasks/{task_id}:",
		"operationId: updateProjectTask",
		"/platform/v1/projects/{project_id}/operations:",
		"operationId: createProjectOperation",
		"/platform/v1/projects/{project_id}/operations/{operation_id}:",
		"operationId: upsertProjectOperation",
		"/platform/v1/projects/{project_id}/change-sets:",
		"operationId: createProjectChangeSet",
		"/platform/v1/projects/{project_id}/change-sets/{change_set_id}/preflight:",
		"operationId: preflightProjectChangeSet",
		"/platform/v1/projects/{project_id}/change-sets/{change_set_id}/approve:",
		"operationId: approveProjectChangeSet",
		"/platform/v1/projects/{project_id}/change-sets/{change_set_id}/execute:",
		"operationId: executeProjectChangeSet",
		"/platform/v1/projects/{project_id}/change-sets/{change_set_id}/rollback:",
		"operationId: rollbackProjectChangeSet",
		"/platform/v1/projects/{project_id}/audit-events:",
		"operationId: listProjectAuditEvents",
		"ProjectDetail:",
		"BusinessTask:",
		"OperationalRecord:",
		"ChangeSet:",
		"AuditEvent:",
	} {
		if !strings.Contains(document, required) {
			t.Fatalf("OpenAPI contract is missing %q", required)
		}
	}
}
