ALTER TABLE projects
  ADD COLUMN industry VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'ecommerce' AFTER status,
  ADD CONSTRAINT chk_projects_industry CHECK (industry IN ('short_drama', 'game', 'ecommerce', 'automotive_brand'));
