ALTER TABLE insight_miyun_handoffs
    ADD COLUMN crawl_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER product_profile_id,
    ADD INDEX idx_miyun_handoffs_crawl_job (organization_id, project_id, crawl_job_id),
    ADD CONSTRAINT fk_miyun_handoffs_crawl_job
        FOREIGN KEY (organization_id, project_id, crawl_job_id)
        REFERENCES insight_miyun_crawl_jobs(organization_id, project_id, id);
