-- This migration repairs already-persisted legacy workspaces and is intentionally
-- not reversible. Removing the compatibility columns would make those records
-- unreadable by current application versions.
SELECT 1;
