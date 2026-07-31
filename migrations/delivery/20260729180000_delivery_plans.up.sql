-- Extend the #21 DeliveryPlan projection with mock lifecycle provenance and
-- immutable versions. The existing root remains authoritative for ChangeSet,
-- Execution and Insights joins.
ALTER TABLE delivery_plans
  ADD COLUMN platform VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'ocean_engine_mock' AFTER version,
  ADD COLUMN source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'mock' AFTER platform,
  ADD COLUMN scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'golden_path' AFTER source,
  ADD COLUMN current_version INT NOT NULL DEFAULT 1 AFTER scenario,
  DROP CHECK chk_delivery_plans_budget,
  ADD CONSTRAINT chk_delivery_plans_budget CHECK (budget_cents >= 0),
  ADD CONSTRAINT chk_delivery_plans_current_version CHECK (current_version >= 1),
  ADD CONSTRAINT chk_delivery_plans_source CHECK (source = 'mock');

CREATE TABLE delivery_plan_versions (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version_number INT NOT NULL,
  config_json JSON NOT NULL,
  source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, plan_id, version_number),
  KEY idx_delivery_plan_versions_project (organization_id, project_id, plan_id, version_number),
  CONSTRAINT fk_delivery_plan_versions_plan
    FOREIGN KEY (organization_id, plan_id) REFERENCES delivery_plans(organization_id, id),
  CONSTRAINT chk_delivery_plan_versions_number CHECK (version_number >= 1),
  CONSTRAINT chk_delivery_plan_versions_source CHECK (source = 'mock')
);

-- #21 plans created before this migration become V1 mock snapshots. This keeps
-- their package lineage and execution behavior while making version reads total.
INSERT INTO delivery_plan_versions (
  organization_id, project_id, plan_id, version_number, config_json,
  source, scenario, created_by_kind, created_by_id, created_at
)
SELECT
  organization_id,
  project_id,
  id,
  version,
  JSON_OBJECT(
    'plan_id', id,
    'organization_id', organization_id,
    'project_id', project_id,
    'version_number', version,
    'name', name,
    'objective', objective,
    'advertiser', JSON_OBJECT(
      'id', 'mock-advertiser-001',
      'name', 'Cookies Mock Advertiser',
      'platform', 'ocean_engine',
      'source', 'mock',
      'scenario', 'golden_path'
    ),
    'budget', JSON_OBJECT('total_minor', budget_cents, 'currency', 'CNY'),
    'schedule', JSON_OBJECT(
      'start_at', DATE_FORMAT(start_at, '%Y-%m-%dT%H:%i:%s.000Z'),
      'end_at', DATE_FORMAT(end_at, '%Y-%m-%dT%H:%i:%s.000Z'),
      'timezone', 'Asia/Shanghai'
    ),
    'tracking', JSON_OBJECT(
      'landing_page', 'https://demo.cookies.local',
      'pixel_id', 'PX-LOCAL',
      'conversion_event', 'conversion'
    ),
    'creative_references', JSON_ARRAY(JSON_OBJECT(
      'asset_id', creative_package_id,
      'version', 1,
      'confirmed', TRUE
    )),
    'source_strategy_version', '',
    'source', 'mock',
    'scenario', 'golden_path',
    'created_by', JSON_OBJECT('kind', 'user', 'id', created_by),
    'created_at', DATE_FORMAT(created_at, '%Y-%m-%dT%H:%i:%s.000Z')
  ),
  'mock',
  'golden_path',
  'user',
  created_by,
  created_at
FROM delivery_plans;
