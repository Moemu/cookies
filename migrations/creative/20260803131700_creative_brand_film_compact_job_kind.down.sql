ALTER TABLE creative_production_jobs
  DROP CHECK chk_creative_production_kind,
  ADD CONSTRAINT chk_creative_production_kind
  CHECK (
    job_kind = 'cover_image'
    OR job_kind = 'video_generate'
    OR job_kind REGEXP '^viral_candidate_viralcandidate_[A-Za-z0-9_-]+$'
    OR job_kind REGEXP '^image_plan_([1-9]|1[0-2])_job_[A-Za-z0-9_-]+$'
  );
