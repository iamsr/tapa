-- MySQL seed data for E2E testing
-- Generates 100,000 rows per table for realistic time estimation

USE testdb;

-- Clean existing data
SET FOREIGN_KEY_CHECKS = 0;
TRUNCATE users;
TRUNCATE products;
TRUNCATE orders;
SET FOREIGN_KEY_CHECKS = 1;

-- Generate 100,000 users using recursive CTE (MySQL 8.0+)
INSERT INTO users (username, created_at)
WITH RECURSIVE nums AS (
    SELECT 1 AS n
    UNION ALL
    SELECT n + 1 FROM nums WHERE n < 100000
)
SELECT 
    CONCAT('user_', n),
    DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 365) DAY)
FROM nums;

-- Generate 100,000 products
INSERT INTO products (name, price, stock_quantity, created_at)
WITH RECURSIVE nums AS (
    SELECT 1 AS n
    UNION ALL
    SELECT n + 1 FROM nums WHERE n < 100000
)
SELECT 
    CONCAT('Product ', n),
    ROUND(RAND() * 1000, 2),
    FLOOR(RAND() * 1000),
    DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 365) DAY)
FROM nums;

-- Generate 100,000 orders
INSERT INTO orders (user_id, total_amount, status, created_at)
WITH RECURSIVE nums AS (
    SELECT 1 AS n
    UNION ALL
    SELECT n + 1 FROM nums WHERE n < 100000
)
SELECT 
    FLOOR(RAND() * 100000 + 1),
    ROUND(RAND() * 5000, 2),
    CASE FLOOR(RAND() * 3)
        WHEN 0 THEN 'pending'
        WHEN 1 THEN 'completed'
        ELSE 'shipped'
    END,
    DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 180) DAY)
FROM nums;

-- Verify counts
SELECT CONCAT('Users: ', COUNT(*)) AS count FROM users;
SELECT CONCAT('Products: ', COUNT(*)) AS count FROM products;
SELECT CONCAT('Orders: ', COUNT(*)) AS count FROM orders;

-- Force statistics update
ANALYZE TABLE users;
ANALYZE TABLE products;
ANALYZE TABLE orders;

-- Display table sizes
SELECT 
    table_name,
    ROUND(((data_length + index_length) / 1024 / 1024), 2) AS size_mb
FROM information_schema.tables
WHERE table_schema = 'testdb'
ORDER BY (data_length + index_length) DESC;
