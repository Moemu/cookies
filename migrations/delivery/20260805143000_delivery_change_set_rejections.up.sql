ALTER TABLE delivery_change_sets
  ADD COLUMN rejected_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER approved_at,
  ADD COLUMN rejected_at DATETIME(6) NULL AFTER rejected_by,
  ADD COLUMN rejection_reason VARCHAR(1000) NULL AFTER rejected_at;
