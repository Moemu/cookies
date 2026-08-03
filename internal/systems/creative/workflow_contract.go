package creative

// Shared Creative contract versions are frozen together. Image-text and brand
// video implementations may add format-specific contracts, but must not change
// these values or reuse a version for a breaking shape change.
const (
	CreativeSharedWorkflowV1              = "creative-shared-workflow/v1"
	CreativeIntakeCreateV3ContractVersion = "creative-intake-create/v3"
	CreativeIntakeV3ContractVersion       = "creative-intake/v3"
	CreativePlanningContextV1             = "creative-planning-context/v1"
	CreativeDirectionBatchV1              = "creative-direction-candidate-batch/v1"
	CreativeDirectionVersionV1            = "creative-direction/v1"

	CreativeRouteImageText  = "image_text"
	CreativeRouteBrandVideo = "brand_video"
	ManualImageTextRouteID  = "route_manual_xiaohongshu_image_text_v1"
)

type CreativeDirectionBatchStatus string

const (
	DirectionBatchGenerating CreativeDirectionBatchStatus = "generating"
	DirectionBatchReady      CreativeDirectionBatchStatus = "ready"
	DirectionBatchFailed     CreativeDirectionBatchStatus = "failed"
)

type CreativeDirectionStatus string

const (
	DirectionStatusCandidate  CreativeDirectionStatus = "candidate"
	DirectionStatusConfirmed  CreativeDirectionStatus = "confirmed"
	DirectionStatusSuperseded CreativeDirectionStatus = "superseded"
)

// CanTransitionCreativeIntakeV3Status defines the only state changes shared by
// image-text and brand-video intake consumers. Equal states are accepted as
// idempotent replays; draft belongs to legacy intake contracts and is rejected.
func CanTransitionCreativeIntakeV3Status(from, to IntakeStatus) bool {
	if from == to {
		return from == IntakeNeedsClarification || from == IntakeReady || from == IntakeSuperseded
	}
	switch from {
	case IntakeNeedsClarification:
		return to == IntakeReady || to == IntakeSuperseded
	case IntakeReady:
		return to == IntakeSuperseded
	default:
		return false
	}
}

func CanTransitionDirectionBatchStatus(from, to CreativeDirectionBatchStatus) bool {
	if from == to {
		return from == DirectionBatchGenerating || from == DirectionBatchReady || from == DirectionBatchFailed
	}
	return from == DirectionBatchGenerating && (to == DirectionBatchReady || to == DirectionBatchFailed)
}

func CanTransitionDirectionStatus(from, to CreativeDirectionStatus) bool {
	if from == to {
		return from == DirectionStatusCandidate || from == DirectionStatusConfirmed || from == DirectionStatusSuperseded
	}
	switch from {
	case DirectionStatusCandidate:
		return to == DirectionStatusConfirmed || to == DirectionStatusSuperseded
	case DirectionStatusConfirmed:
		return to == DirectionStatusSuperseded
	default:
		return false
	}
}

// CanTransitionCreativeTaskStatus is the shared task shell. A delivered task
// may return to draft for a new immutable version; previously delivered
// CreativePackages remain unchanged.
func CanTransitionCreativeTaskStatus(from, to TaskStatus) bool {
	if from == to {
		return validCreativeTaskStatus(from)
	}
	switch from {
	case TaskDraft:
		return oneOfTaskStatus(to, TaskInProgress, TaskGenerating, TaskReady, TaskArchived)
	case TaskInProgress:
		return oneOfTaskStatus(to, TaskDraft, TaskGenerating, TaskReady, TaskArchived)
	case TaskGenerating:
		return oneOfTaskStatus(to, TaskInProgress, TaskGenerated, TaskArchived)
	case TaskGenerated:
		return oneOfTaskStatus(to, TaskInProgress, TaskRendering, TaskReady, TaskArchived)
	case TaskRendering:
		return oneOfTaskStatus(to, TaskGenerated, TaskReady, TaskArchived)
	case TaskReady:
		return oneOfTaskStatus(to, TaskDraft, TaskInProgress, TaskApproved, TaskArchived)
	case TaskApproved:
		return oneOfTaskStatus(to, TaskDraft, TaskDelivered, TaskArchived)
	case TaskDelivered:
		return oneOfTaskStatus(to, TaskDraft, TaskArchived)
	default:
		return false
	}
}

func CanTransitionCreativeVersionStatus(from, to CreativeVersionStatus) bool {
	if from == to {
		return validCreativeVersionStatus(from)
	}
	switch from {
	case CreativeVersionCreated:
		return to == CreativeVersionChecked || to == CreativeVersionSuperseded
	case CreativeVersionChecked:
		return to == CreativeVersionApproved || to == CreativeVersionSuperseded
	case CreativeVersionApproved:
		return to == CreativeVersionSuperseded
	default:
		return false
	}
}

func validCreativeTaskStatus(value TaskStatus) bool {
	return oneOfTaskStatus(
		value,
		TaskDraft,
		TaskInProgress,
		TaskReady,
		TaskGenerating,
		TaskGenerated,
		TaskRendering,
		TaskApproved,
		TaskDelivered,
		TaskArchived,
	)
}

func validCreativeVersionStatus(value CreativeVersionStatus) bool {
	return value == CreativeVersionCreated ||
		value == CreativeVersionChecked ||
		value == CreativeVersionApproved ||
		value == CreativeVersionSuperseded
}

func oneOfTaskStatus(value TaskStatus, allowed ...TaskStatus) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
