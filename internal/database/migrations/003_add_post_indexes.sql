-- Add composite indexes for improved query performance on common patterns
-- Composite index for filtering by status and ordering by created_at
CREATE INDEX idx_posts_status_created_at ON posts (status, created_at DESC);
-- Composite index for author's posts with status filtering and pagination
CREATE INDEX idx_posts_author_status_created_at ON posts (author_id, status, created_at DESC);

---- create above / drop below ----

-- Remove composite indexes
DROP INDEX IF EXISTS idx_posts_author_status_created_at;
DROP INDEX IF EXISTS idx_posts_status_created_at;
