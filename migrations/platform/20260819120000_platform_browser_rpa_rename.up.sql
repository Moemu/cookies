-- Rename the historical computer-use control-plane namespace to browser-rpa.
-- RENAME TABLE is a metadata-only operation: row bytes, canonical hashes and
-- embedded schema_version payloads are untouched, and foreign keys follow the
-- renamed tables automatically. Index and constraint names intentionally keep
-- their historical computer_use_* identifiers.
RENAME TABLE
  computer_use_environments TO browser_rpa_environments,
  computer_use_browser_profiles TO browser_rpa_browser_profiles,
  computer_use_site_policies TO browser_rpa_site_policies,
  computer_use_kill_switches TO browser_rpa_kill_switches,
  computer_use_runs TO browser_rpa_runs,
  computer_use_session_leases TO browser_rpa_session_leases,
  computer_use_run_steps TO browser_rpa_run_steps,
  computer_use_events TO browser_rpa_events,
  computer_use_evidence TO browser_rpa_evidence,
  computer_use_final_confirmations TO browser_rpa_final_confirmations,
  computer_use_confirmation_events TO browser_rpa_confirmation_events,
  computer_use_controlled_action_attempts TO browser_rpa_controlled_action_attempts;

-- CDP endpoint of the externally authenticated browser session this
-- environment drives (Playwright connectOverCDP target).
ALTER TABLE browser_rpa_environments
  ADD COLUMN cdp_endpoint VARCHAR(512) NULL;
