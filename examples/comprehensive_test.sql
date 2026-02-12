-- Comprehensive test migration demonstrating all advanced features

-- 1. High-impact operation: ADD COLUMN with DEFAULT (should trigger concurrency warnings)
ALTER TABLE users ADD COLUMN email VARCHAR(255) DEFAULT 'unknown@example.com';

-- 2. Concurrent index creation (low concurrency impact)
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);

-- 3. Regular index creation (medium concurrency impact)
CREATE INDEX idx_orders_status ON orders(status);

-- 4. ALTER COLUMN (high impact, requires rewrite)
ALTER TABLE products ALTER COLUMN price TYPE NUMERIC(12,2);

-- 5. DROP COLUMN (reversible with data loss)
ALTER TABLE sessions DROP COLUMN last_accessed;

-- 6. CREATE TABLE (safe operation)
CREATE TABLE notifications (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    message TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 7. ADD FOREIGN KEY (medium risk)
ALTER TABLE notifications ADD CONSTRAINT fk_user 
    FOREIGN KEY (user_id) REFERENCES users(id);
