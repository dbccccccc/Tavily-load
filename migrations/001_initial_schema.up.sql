-- Create api_keys table
CREATE TABLE api_keys (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    key_value VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_blacklisted BOOLEAN NOT NULL DEFAULT FALSE,
    blacklisted_until TIMESTAMP NULL,
    blacklist_reason VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_key_value (key_value),
    INDEX idx_active (is_active),
    INDEX idx_blacklisted (is_blacklisted),
    INDEX idx_blacklisted_until (blacklisted_until)
);

-- Create key_usage_stats table
CREATE TABLE key_usage_stats (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    key_id BIGINT NOT NULL,
    requests_count BIGINT NOT NULL DEFAULT 0,
    errors_count BIGINT NOT NULL DEFAULT 0,
    last_used_at TIMESTAMP NULL,
    last_error_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    FOREIGN KEY (key_id) REFERENCES api_keys(id) ON DELETE CASCADE,
    INDEX idx_key_id (key_id),
    INDEX idx_last_used (last_used_at)
);

-- Create key_blacklist_history table
CREATE TABLE key_blacklist_history (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    key_id BIGINT NOT NULL,
    blacklisted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    blacklisted_until TIMESTAMP NULL,
    reason VARCHAR(500),
    is_permanent BOOLEAN NOT NULL DEFAULT FALSE,
    
    FOREIGN KEY (key_id) REFERENCES api_keys(id) ON DELETE CASCADE,
    INDEX idx_key_id (key_id),
    INDEX idx_blacklisted_at (blacklisted_at)
);