CREATE TABLE IF NOT EXISTS platform_project_runtimes (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code VARCHAR(96) NOT NULL,
  brand VARCHAR(255) NOT NULL,
  product VARCHAR(255) NOT NULL,
  goal TEXT NOT NULL,
  stage VARCHAR(128) NOT NULL,
  progress INT NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner VARCHAR(255) NOT NULL,
  budget DECIMAL(18,2) NOT NULL DEFAULT 0,
  currency CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'CNY',
  timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
  knowledge_count INT NOT NULL DEFAULT 0,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id),
  CONSTRAINT fk_platform_project_runtimes_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_platform_project_runtimes_progress CHECK (progress >= 0 AND progress <= 100),
  CONSTRAINT chk_platform_project_runtimes_budget CHECK (budget >= 0)
);

INSERT INTO platform_project_runtimes (organization_id, project_id, code, brand, product, goal, stage, progress, status, owner, budget, currency, timezone, knowledge_count, updated_at)
SELECT p.organization_id, p.id, p.id, COALESCE(b.name, '未指定品牌'), COALESCE(products.product_names, '尚未关联产品'), '尚未设定项目目标',
  CASE p.status WHEN 'archived' THEN '已归档' WHEN 'active' THEN '项目执行' ELSE '项目初始化' END,
  CASE p.status WHEN 'archived' THEN 100 WHEN 'active' THEN 10 ELSE 0 END,
  CASE p.status WHEN 'archived' THEN 'completed' WHEN 'draft' THEN 'blocked' ELSE 'active' END,
  '系统', 0, 'CNY', 'Asia/Shanghai', 0, p.updated_at
FROM projects p
LEFT JOIN brands b ON b.organization_id=p.organization_id AND b.id=p.primary_brand_id
LEFT JOIN (
  SELECT pp.organization_id, pp.project_id, GROUP_CONCAT(pr.name ORDER BY pr.name SEPARATOR '、') AS product_names
  FROM project_products pp JOIN products pr ON pr.organization_id=pp.organization_id AND pr.id=pp.product_id
  GROUP BY pp.organization_id, pp.project_id
) products ON products.organization_id=p.organization_id AND products.project_id=p.id
ON DUPLICATE KEY UPDATE project_id=VALUES(project_id);

CREATE TABLE IF NOT EXISTS platform_project_workbenches (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_code VARCHAR(96) NOT NULL, organization_name VARCHAR(255) NOT NULL, organization_owner VARCHAR(255) NOT NULL,
  client_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, client_code VARCHAR(96) NOT NULL, client_name VARCHAR(255) NOT NULL, client_industry VARCHAR(128) NOT NULL,
  brand_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, brand_code VARCHAR(96) NOT NULL, brand_name VARCHAR(255) NOT NULL, brand_category VARCHAR(128) NOT NULL,
  product_lines JSON NOT NULL, guideline_status VARCHAR(32) NOT NULL,
  stage VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, stage_label VARCHAR(128) NOT NULL, stage_percent INT NOT NULL, task_percent INT NOT NULL, risk_status VARCHAR(32) NOT NULL, blocker TEXT NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id),
  CONSTRAINT fk_platform_project_workbenches_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_platform_project_workbenches_product_lines_json CHECK (JSON_VALID(product_lines)),
  CONSTRAINT chk_platform_project_workbenches_percent CHECK (stage_percent BETWEEN 0 AND 100 AND task_percent BETWEEN 0 AND 100)
);

CREATE TABLE IF NOT EXISTS platform_project_workbench_ad_accounts (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  client_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, brand_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, platform VARCHAR(64) NOT NULL, account_name VARCHAR(255) NOT NULL, account_display_id VARCHAR(255) NOT NULL,
  currency CHAR(3) NOT NULL, timezone VARCHAR(64) NOT NULL, permission_status VARCHAR(32) NOT NULL, login_status VARCHAR(32) NOT NULL, tracking_status VARCHAR(32) NOT NULL, owner VARCHAR(255) NOT NULL, bound_asset_ids JSON NOT NULL, last_synced_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, id), CONSTRAINT fk_platform_project_workbench_ad_accounts_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id), CONSTRAINT chk_platform_project_workbench_ad_accounts_assets_json CHECK (JSON_VALID(bound_asset_ids))
);

CREATE TABLE IF NOT EXISTS platform_project_workbench_quality_checks (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, asset_version INT NOT NULL, status VARCHAR(32) NOT NULL, model VARCHAR(128) NOT NULL, rule_version VARCHAR(128) NOT NULL, prompt_version VARCHAR(128) NOT NULL, summary TEXT NOT NULL, issues JSON NOT NULL, created_at DATETIME(6) NOT NULL, completed_at DATETIME(6) NULL,
  PRIMARY KEY (organization_id, project_id, id), CONSTRAINT fk_platform_project_workbench_quality_checks_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id), CONSTRAINT chk_platform_project_workbench_quality_checks_issues_json CHECK (JSON_VALID(issues))
);

CREATE TABLE IF NOT EXISTS platform_project_workbench_material_confirmations (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  quality_check_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, asset_version INT NOT NULL, status VARCHAR(32) NOT NULL, scope VARCHAR(255) NOT NULL, confirmed_by VARCHAR(255) NOT NULL, note TEXT NOT NULL, created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, id), CONSTRAINT fk_platform_project_workbench_material_confirmations_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id)
);

CREATE TABLE IF NOT EXISTS platform_project_workbench_asset_pointers (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, working_version INT NOT NULL, quality_checked_version INT NULL, human_confirmed_version INT NULL, delivery_version INT NULL,
  versions JSON NOT NULL, authorization_platforms JSON NOT NULL, authorization_regions JSON NOT NULL, rights_holder VARCHAR(255) NOT NULL, expires_at DATETIME(6) NOT NULL, authorization_note TEXT NOT NULL, delivery_platform VARCHAR(64) NOT NULL, delivery_region VARCHAR(128) NOT NULL, owner VARCHAR(255) NOT NULL, updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id, id), CONSTRAINT fk_platform_project_workbench_asset_pointers_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id), CONSTRAINT chk_platform_project_workbench_asset_pointers_versions_json CHECK (JSON_VALID(versions)), CONSTRAINT chk_platform_project_workbench_asset_pointers_platforms_json CHECK (JSON_VALID(authorization_platforms)), CONSTRAINT chk_platform_project_workbench_asset_pointers_regions_json CHECK (JSON_VALID(authorization_regions))
);
