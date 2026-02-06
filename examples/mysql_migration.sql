-- Add new analytics columns
ALTER TABLE users ADD COLUMN last_login TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create index for faster lookups
CREATE INDEX idx_last_login ON users(last_login) ALGORITHM=INPLACE;

-- Modify existing column type
ALTER TABLE users MODIFY COLUMN status ENUM('active', 'inactive', 'suspended');
