-- MySQL test migration
-- This migration tests various risky operations

-- Test 1: Add column (should be safe with ALGORITHM=INPLACE)
ALTER TABLE users ADD COLUMN email VARCHAR(255);

-- Test 2: Add NOT NULL column with default (should detect risk)
ALTER TABLE users ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'active';

-- Test 3: Create index (should be low-medium risk)
CREATE INDEX idx_users_email ON users(email);

-- Test 4: Add column with ALGORITHM and LOCK clauses
ALTER TABLE products 
  ADD COLUMN description TEXT,
  ALGORITHM=INPLACE, 
  LOCK=NONE;

-- Test 5: Modify column (should be high risk)
ALTER TABLE products MODIFY COLUMN price DECIMAL(12, 2) NOT NULL;

-- Test 6: Add foreign key (should detect dependency risk)
ALTER TABLE orders ADD COLUMN shipping_address_id INT;
