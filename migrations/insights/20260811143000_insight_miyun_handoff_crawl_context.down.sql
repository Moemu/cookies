ALTER TABLE insight_miyun_handoffs
    DROP FOREIGN KEY fk_miyun_handoffs_crawl_job,
    DROP INDEX idx_miyun_handoffs_crawl_job,
    DROP COLUMN crawl_job_id;
