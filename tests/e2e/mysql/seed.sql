-- MySQL seed data
-- Insert test data for E2E testing

USE testdb;

-- Insert users
INSERT INTO users (username) VALUES
    ('alice'),
    ('bob'),
    ('charlie'),
    ('david'),
    ('eve');

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
SELECT CONCAT('Users count: ', COUNT(*)) FROM users;
SELECT CONCAT('Products count: ', COUNT(*)) FROM products;
SELECT CONCAT('Orders count: ', COUNT(*)) FROM orders;
