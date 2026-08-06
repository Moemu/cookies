ALTER TABLE strategy_messages
    ADD COLUMN content_blocks JSON NULL AFTER content,
    ADD COLUMN requested_policy JSON NULL AFTER content_blocks;
