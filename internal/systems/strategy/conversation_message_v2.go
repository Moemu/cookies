package strategy

import (
	"encoding/hex"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const MessageCreateContractV2 = "strategy-conversation-message-create/v2"

const (
	messageBlockLimit = 24
	messageTextLimit  = 64 << 10
)

// MessageContentBlock binds a conversation turn to immutable Knowledge or
// Asset resources. The flat wire shape keeps the composer contract easy to
// inspect while the service validates each discriminated block strictly.
type MessageContentBlock struct {
	Type                  string           `json:"type"`
	Text                  string           `json:"text,omitempty"`
	DocumentID            string           `json:"document_id,omitempty"`
	ExpectedContentSHA256 string           `json:"expected_content_sha256,omitempty"`
	ResearchArtifactID    string           `json:"research_artifact_id,omitempty"`
	ExpectedContentHash   string           `json:"expected_content_hash,omitempty"`
	AssetKind             string           `json:"asset_kind,omitempty"`
	AssetID               contract.AssetID `json:"asset_id,omitempty"`
	AssetVersion          int64            `json:"asset_version,omitempty"`
}

// MessageRequestedPolicy records user intent, not provider parameters. The
// effective policy is resolved server-side later from tenant policy, budget,
// task type, and provider capabilities.
type MessageRequestedPolicy struct {
	ReasoningMode string   `json:"reasoning_mode,omitempty"`
	WebSearch     string   `json:"web_search,omitempty"`
	MCPServerIDs  []string `json:"mcp_server_ids,omitempty"`
}

type SendMessageV2Request struct {
	ContractVersion string                  `json:"contract_version"`
	Content         []MessageContentBlock   `json:"content"`
	RequestedPolicy *MessageRequestedPolicy `json:"requested_policy,omitempty"`
}

type normalizedMessageV2 struct {
	ContentBlocks   []MessageContentBlock
	Projection      string
	RequestedPolicy *MessageRequestedPolicy
}

func normalizeMessageV2(request SendMessageV2Request) (normalizedMessageV2, error) {
	if request.ContractVersion != MessageCreateContractV2 || len(request.Content) == 0 || len(request.Content) > messageBlockLimit {
		return normalizedMessageV2{}, ErrInvalidRequest
	}

	blocks := make([]MessageContentBlock, 0, len(request.Content))
	projection := make([]string, 0, len(request.Content))
	seenRefs := make(map[string]struct{}, len(request.Content))
	totalTextBytes := 0
	hasResearchReference := false
	for _, raw := range request.Content {
		block := raw
		block.Type = strings.TrimSpace(block.Type)
		switch block.Type {
		case "text":
			if block.DocumentID != "" || block.ExpectedContentSHA256 != "" || block.ResearchArtifactID != "" || block.ExpectedContentHash != "" || block.AssetKind != "" || block.AssetID != "" || block.AssetVersion != 0 {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			block.Text = strings.TrimSpace(block.Text)
			if block.Text == "" {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			totalTextBytes += len(block.Text)
			if totalTextBytes > messageTextLimit {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			projection = append(projection, block.Text)
		case "document_ref":
			if block.Text != "" || block.ResearchArtifactID != "" || block.ExpectedContentHash != "" || block.AssetKind != "" || block.AssetID != "" || block.AssetVersion != 0 {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			block.DocumentID = strings.TrimSpace(block.DocumentID)
			block.ExpectedContentSHA256 = strings.TrimSpace(block.ExpectedContentSHA256)
			if !validMessageResourceID(block.DocumentID) || !validRawSHA256(block.ExpectedContentSHA256) {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			key := "document:" + block.DocumentID + ":" + block.ExpectedContentSHA256
			if _, duplicate := seenRefs[key]; duplicate {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			seenRefs[key] = struct{}{}
			projection = append(projection, "[文档 "+block.DocumentID+"]")
		case "asset_ref":
			if block.Text != "" || block.DocumentID != "" || block.ExpectedContentSHA256 != "" || block.ResearchArtifactID != "" || block.ExpectedContentHash != "" {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			block.AssetKind = strings.TrimSpace(block.AssetKind)
			block.AssetID = contract.AssetID(strings.TrimSpace(string(block.AssetID)))
			if block.AssetKind != "image" && block.AssetKind != "video" {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			ref := contract.AssetVersionRef{AssetID: block.AssetID, Version: block.AssetVersion}
			if !validMessageResourceID(string(block.AssetID)) || ref.Validate() != nil {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			key := "asset:" + string(block.AssetID) + ":" + block.AssetKind + ":" + formatInt64(block.AssetVersion)
			if _, duplicate := seenRefs[key]; duplicate {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			seenRefs[key] = struct{}{}
			label := "图片"
			if block.AssetKind == "video" {
				label = "视频"
			}
			projection = append(projection, "["+label+" "+string(block.AssetID)+"#"+formatInt64(block.AssetVersion)+"]")
		case "research_ref":
			if block.Text != "" || block.DocumentID != "" || block.ExpectedContentSHA256 != "" || block.AssetKind != "" || block.AssetID != "" || block.AssetVersion != 0 {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			block.ResearchArtifactID = strings.TrimSpace(block.ResearchArtifactID)
			block.ExpectedContentHash = strings.TrimSpace(block.ExpectedContentHash)
			if !validMessageResourceID(block.ResearchArtifactID) || !validRawSHA256(block.ExpectedContentHash) {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			key := "research:" + block.ResearchArtifactID + ":" + block.ExpectedContentHash
			if _, duplicate := seenRefs[key]; duplicate {
				return normalizedMessageV2{}, ErrInvalidRequest
			}
			seenRefs[key] = struct{}{}
			hasResearchReference = true
			projection = append(projection, "[联网证据 "+block.ResearchArtifactID+"]")
		default:
			return normalizedMessageV2{}, ErrInvalidRequest
		}
		blocks = append(blocks, block)
	}

	policy, err := normalizeRequestedPolicy(request.RequestedPolicy)
	if err != nil {
		return normalizedMessageV2{}, err
	}
	if hasResearchReference != (policy != nil && policy.WebSearch == "allowed") {
		// A search request is only truthful when the completed, immutable
		// research result travels with the message. Likewise, research evidence
		// cannot be smuggled in while the visible policy says search was off.
		return normalizedMessageV2{}, ErrInvalidRequest
	}
	plainText := strings.Join(projection, "\n")
	if plainText == "" || len(plainText) > messageTextLimit {
		return normalizedMessageV2{}, ErrInvalidRequest
	}
	return normalizedMessageV2{ContentBlocks: blocks, Projection: plainText, RequestedPolicy: policy}, nil
}

func normalizeRequestedPolicy(raw *MessageRequestedPolicy) (*MessageRequestedPolicy, error) {
	if raw == nil {
		return nil, nil
	}
	policy := *raw
	policy.ReasoningMode = strings.TrimSpace(policy.ReasoningMode)
	policy.WebSearch = strings.TrimSpace(policy.WebSearch)
	if policy.ReasoningMode != "" && policy.ReasoningMode != "standard" && policy.ReasoningMode != "deep" {
		return nil, ErrInvalidRequest
	}
	if policy.WebSearch != "" && policy.WebSearch != "disabled" && policy.WebSearch != "allowed" {
		return nil, ErrInvalidRequest
	}
	// Remote MCP is deliberately outside P0. Reject instead of silently
	// accepting a policy that the runtime cannot honor.
	if len(policy.MCPServerIDs) != 0 {
		return nil, ErrInvalidRequest
	}
	if policy.ReasoningMode == "" && policy.WebSearch == "" {
		return nil, nil
	}
	policy.MCPServerIDs = nil
	return &policy, nil
}

func validMessageResourceID(value string) bool {
	return value != "" && len(value) <= 96 && !strings.ContainsAny(value, " \t\r\n")
}

func validRawSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func formatInt64(value int64) string {
	// Asset versions are positive and small in practice. Keeping conversion
	// here avoids leaking presentation concerns into the immutable ref type.
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = digits[value%10]
		value /= 10
	}
	return string(buffer[position:])
}
