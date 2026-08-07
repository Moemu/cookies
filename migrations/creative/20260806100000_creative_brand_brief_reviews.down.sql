ALTER TABLE creative_direction_batches
    DROP COLUMN brand_brief_content_hash,
    DROP COLUMN brand_brief_revision;

DROP TABLE IF EXISTS creative_brand_brief_reviews;
