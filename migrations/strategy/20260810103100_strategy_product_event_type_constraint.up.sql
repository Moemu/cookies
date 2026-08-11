ALTER TABLE strategy_product_events
  ADD CONSTRAINT chk_strategy_product_event_type CHECK (
    event_type IN (
      'workspace.opened', 'stage.viewed', 'assistant.command_submitted',
      'assistant.first_ack', 'assistant.first_meaningful_update',
      'assistant.proposal_accepted', 'assistant.proposal_edited', 'assistant.proposal_ignored',
      'research.started', 'research.finding_verified', 'research.completed', 'research.partial',
      'research.failed', 'research.cancelled', 'research.proposal_applied', 'research.proposal_stale',
      'document.parse_started', 'document.ready', 'document.partial', 'document.failed',
      'document.vision_fallback', 'brief.confirmed', 'strategy.confirmed',
      'review.submitted', 'review.approved', 'review.returned', 'handoff.created',
      'activity.stalled', 'activity.retried'
    )
  );
