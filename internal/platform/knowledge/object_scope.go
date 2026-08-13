package knowledge

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func knowledgeDocumentObjectPrefix(organizationID contract.OrganizationID, projectID contract.ProjectID, documentID string) string {
	return fmt.Sprintf("assets/%s/%s/knowledge/%s/", organizationID, projectID, documentID)
}

func knowledgeDocumentLocationInScope(
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	documentID string,
	location assets.ObjectLocation,
	bucket string,
) bool {
	return strings.TrimSpace(bucket) != "" && location.Bucket == bucket &&
		strings.HasPrefix(location.Key, knowledgeDocumentObjectPrefix(organizationID, projectID, documentID))
}

func knowledgeDocumentBlobInScope(document Document, bucket string) bool {
	return knowledgeDocumentLocationInScope(
		document.OrganizationID, document.ProjectID, document.ID, document.Blob, bucket,
	)
}
