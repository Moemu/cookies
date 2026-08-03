export const CREATIVE_SHARED_WORKFLOW_VERSION = 'creative-shared-workflow/v1' as const

export const CREATIVE_CONTRACT_VERSIONS = {
  intake_create: 'creative-intake-create/v3',
  intake: 'creative-intake/v3',
  planning_context: 'creative-planning-context/v1',
  direction_candidate_batch: 'creative-direction-candidate-batch/v1',
  direction: 'creative-direction/v1',
} as const

export const CREATIVE_ROUTE_PROFILES = {
  image_text: {
    deliverable_type: 'image_text',
    purpose: 'brand',
  },
  brand_video: {
    deliverable_type: 'video',
    purpose: 'brand',
    performance_mode: 'brand_video',
  },
} as const

export const CREATIVE_STATE_MACHINE = {
  intake: {
    initial: ['needs_clarification', 'ready'],
    terminal: ['superseded'],
    transitions: {
      needs_clarification: ['ready', 'superseded'],
      ready: ['superseded'],
      superseded: [],
    },
  },
  direction_batch: {
    initial: ['generating'],
    terminal: ['ready', 'failed'],
    transitions: {
      generating: ['ready', 'failed'],
      ready: [],
      failed: [],
    },
  },
  direction: {
    initial: ['candidate'],
    terminal: ['superseded'],
    transitions: {
      candidate: ['confirmed', 'superseded'],
      confirmed: ['superseded'],
      superseded: [],
    },
  },
  task: {
    initial: ['draft'],
    terminal: ['archived'],
    transitions: {
      draft: ['in_progress', 'generating', 'ready_for_review', 'archived'],
      in_progress: ['draft', 'generating', 'ready_for_review', 'archived'],
      generating: ['in_progress', 'generated', 'archived'],
      generated: ['in_progress', 'rendering', 'ready_for_review', 'archived'],
      rendering: ['generated', 'ready_for_review', 'archived'],
      ready_for_review: ['draft', 'in_progress', 'approved', 'archived'],
      approved: ['draft', 'delivered', 'archived'],
      delivered: ['draft', 'archived'],
      archived: [],
    },
  },
  creative_version: {
    initial: ['created'],
    terminal: ['superseded'],
    transitions: {
      created: ['checked', 'superseded'],
      checked: ['approved', 'superseded'],
      approved: ['superseded'],
      superseded: [],
    },
  },
} as const

export type CreativeIntakeStatus = keyof typeof CREATIVE_STATE_MACHINE.intake.transitions
export type CreativeDirectionBatchStatus = keyof typeof CREATIVE_STATE_MACHINE.direction_batch.transitions
export type CreativeDirectionStatus = keyof typeof CREATIVE_STATE_MACHINE.direction.transitions
export type CreativeTaskStatus = keyof typeof CREATIVE_STATE_MACHINE.task.transitions
export type CreativeVersionStatus = keyof typeof CREATIVE_STATE_MACHINE.creative_version.transitions

type CreativeStateMachineName = keyof typeof CREATIVE_STATE_MACHINE
type CreativeWorkflowState<Machine extends CreativeStateMachineName> =
  keyof typeof CREATIVE_STATE_MACHINE[Machine]['transitions'] & string

export function canTransitionCreativeState<Machine extends CreativeStateMachineName>(
  machine: Machine,
  from: CreativeWorkflowState<Machine>,
  to: CreativeWorkflowState<Machine>,
) {
  const transitions = CREATIVE_STATE_MACHINE[machine].transitions as Record<string, readonly string[]>
  return from === to
    ? Object.prototype.hasOwnProperty.call(transitions, from)
    : transitions[from]?.includes(to) === true
}
