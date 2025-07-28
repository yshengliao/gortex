-- Create tables for the advanced tracing example

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(50) PRIMARY KEY,
    user_id VARCHAR(50) NOT NULL,
    total DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Order items table
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(50) NOT NULL REFERENCES orders(id),
    item_id VARCHAR(50) NOT NULL,
    quantity INTEGER NOT NULL,
    price DECIMAL(10, 2) NOT NULL
);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    item_id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INTEGER NOT NULL DEFAULT 0
);

-- Inventory table
CREATE TABLE IF NOT EXISTS inventory (
    item_id VARCHAR(50) PRIMARY KEY REFERENCES items(item_id),
    stock INTEGER NOT NULL DEFAULT 0,
    last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO items (item_id, name, price, stock) VALUES
    ('item_001', 'Laptop', 999.99, 50),
    ('item_002', 'Mouse', 29.99, 200),
    ('item_003', 'Keyboard', 79.99, 150),
    ('item_004', 'Monitor', 299.99, 75),
    ('item_005', 'Headphones', 149.99, 100)
ON CONFLICT (item_id) DO NOTHING;

INSERT INTO inventory (item_id, stock) VALUES
    ('item_001', 50),
    ('item_002', 200),
    ('item_003', 150),
    ('item_004', 75),
    ('item_005', 100)
ON CONFLICT (item_id) DO NOTHING;

-- Create indexes
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);