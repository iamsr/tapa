-- PostgreSQL test migration
-- This migration tests various risky operations

-- Test 1: Add column (should be safe with proper defaults)
ALTER TABLE users ADD COLUMN email VARCHAR(255);

-- Test 2: Add NOT NULL column with default (should be medium risk)
ALTER TABLE users ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'active';

-- Test 3: Create index (should recommend CONCURRENTLY)
CREATE INDEX idx_users_email ON users(email);

-- Test 4: Add foreign key (should detect dependency risk)
ALTER TABLE orders ADD COLUMN shipping_address_id INTEGER;

-- Test 5: Change column type (should be high risk - requires table rewrite)
ALTER TABLE products ADD COLUMN description TEXT;
