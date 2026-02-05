-- Migration: Add user analytics
-- This migration adds various columns and indexes for user analytics

-- Fast operation: Add nullable column without default
ALTER TABLE users ADD COLUMN last_login_at TIMESTAMP;

-- Risky operation: Add column with default on large table
ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'active';

-- Very risky: Change column type (requires full table rewrite)
ALTER TABLE orders ALTER COLUMN total TYPE NUMERIC(12,2);

-- Safe operation: Create index concurrently
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);

-- Risky operation: Create index without CONCURRENTLY
CREATE INDEX idx_orders_user_id ON orders(user_id);

-- Backward incompatible: Drop column
ALTER TABLE users DROP COLUMN deprecated_field;

-- Safe operation: Create new table
CREATE TABLE analytics_events (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
