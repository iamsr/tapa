-- PostgreSQL seed data
-- Insert test data for E2E testing

-- Insert users
INSERT INTO users (username) VALUES
    ('alice'),
    ('bob'),
    ('charlie'),
    ('david'),
    ('eve')
ON CONFLICT (username) DO NOTHING;

-- Insert products
INSERT INTO products (name, price, stock_quantity) VALUES
    ('Laptop', 999.99, 50),
    ('Mouse', 29.99, 200),
    ('Keyboard', 79.99, 150),
    ('Monitor', 299.99, 75),
    ('Headphones', 149.99, 100);

-- Insert orders
INSERT INTO orders (user_id, total_amount, status) VALUES
    (1, 999.99, 'completed'),
    (2, 109.98, 'pending'),
    (3, 379.98, 'completed'),
    (1, 449.98, 'shipped'),
    (4, 29.99, 'pending');

-- Verify data
SELECT 'Users count: ' || COUNT(*) FROM users;
SELECT 'Products count: ' || COUNT(*) FROM products;
SELECT 'Orders count: ' || COUNT(*) FROM orders;
