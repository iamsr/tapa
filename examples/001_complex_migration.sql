-- Complex migration demonstrating various operations
-- Phase 1: Add new columns
ALTER TABLE users ADD COLUMN email VARCHAR(255);
ALTER TABLE users ADD COLUMN created_at TIMESTAMP DEFAULT NOW();

-- Phase 2: Create indexes for performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX CONCURRENTLY idx_users_created_at ON users(created_at);

-- Phase 3: Modify existing structure
ALTER TABLE orders ADD COLUMN total DECIMAL(10,2);
CREATE TABLE audit_log (
    id SERIAL PRIMARY KEY,
    user_id INTEGER,
    action VARCHAR(50),
    timestamp TIMESTAMP DEFAULT NOW()
);
