ALTER TABLE creative_production_jobs
  DROP CHECK chk_creative_production_kind;

ALTER TABLE creative_production_jobs
  ADD CONSTRAINT chk_creative_production_kind
  CHECK (
    job_kind = 'cover_image'
    OR job_kind REGEXP '^image_plan_([1-9]|1[0-2])_job_[A-Za-z0-9_-]+$'
  );
