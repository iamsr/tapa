-- PostgreSQL seed data for E2E testing
-- Generates 100,000 rows per table for realistic time estimation

-- Clean existing data
TRUNCATE users, products, orders CASCADE;

-- Generate 100,000 users
INSERT INTO users (username, created_at)
SELECT 
    'user_' || i,
    CURRENT_TIMESTAMP - (random() * interval '365 days')
FROM generate_series(1, 100000) i;

-- Generate 100,000 products
INSERT INTO products (name, price, stock_quantity, created_at)
SELECT 
    'Product ' || i,
    (random() * 1000)::numeric(10,2),
    (random() * 1000)::integer,
    CURRENT_TIMESTAMP - (random() * interval '365 days')
FROM generate_series(1, 100000) i;

-- Generate 100,000 orders (with valid foreign keys)
INSERT INTO orders (user_id, total_amount, status, created_at)
SELECT 
    ((random() * 99999)::integer + 1),  -- Generates 1-100000
    (random() * 5000)::numeric(10,2),
    CASE (random() * 3)::integer
        WHEN 0 THEN 'pending'
        WHEN 1 THEN 'completed'
        ELSE 'shipped'
    END,
    CURRENT_TIMESTAMP - (random() * interval '180 days')
FROM generate_series(1, 100000) i;

-- Verify counts
DO $$
BEGIN
    RAISE NOTICE 'Users: %', (SELECT COUNT(*) FROM users);
    RAISE NOTICE 'Products: %', (SELECT COUNT(*) FROM products);
    RAISE NOTICE 'Orders: %', (SELECT COUNT(*) FROM orders);
END $$;

-- Force statistics update
ANALYZE users;
ANALYZE products;
ANALYZE orders;

-- Display table sizes
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
