-- AI Platform Database Schema
-- 混合大模型服务平台数据库初始化脚本

-- 创建数据库
CREATE DATABASE IF NOT EXISTS ai_platform DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE ai_platform;

-- ============================================
-- 功能配置表
-- ============================================
CREATE TABLE IF NOT EXISTS features (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(64) NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    version INT DEFAULT 1,
    routing JSON,
    cost JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_category (category),
    INDEX idx_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- Provider配置表
-- ============================================
CREATE TABLE IF NOT EXISTS provider_configs (
    id VARCHAR(64) PRIMARY KEY,
    feature_id VARCHAR(64) NOT NULL,
    type VARCHAR(32) NOT NULL,           -- self_hosted, third_party
    vendor VARCHAR(64),
    enabled BOOLEAN DEFAULT TRUE,
    priority INT DEFAULT 1,
    weight INT DEFAULT 100,
    image VARCHAR(256),
    min_instances INT DEFAULT 0,
    max_instances INT DEFAULT 5,
    capability_match JSON,
    model VARCHAR(128),
    api_key_ref VARCHAR(128),
    rate_limit JSON,
    endpoint VARCHAR(512),
    extra JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE,
    INDEX idx_feature_type (feature_id, type),
    INDEX idx_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- API密钥表
-- ============================================
CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(64) PRIMARY KEY,
    vendor VARCHAR(32) NOT NULL,
    service VARCHAR(64) NOT NULL,
    encrypted_dek TEXT NOT NULL,
    encrypted_key TEXT NOT NULL,
    key_hash VARCHAR(64) NOT NULL,
    key_alias VARCHAR(128),
    tier VARCHAR(16) DEFAULT 'primary',
    quota_daily_requests INT DEFAULT 0,
    quota_daily_tokens BIGINT DEFAULT 0,
    quota_monthly_requests INT DEFAULT 0,
    enabled BOOLEAN DEFAULT TRUE,
    auto_rotate BOOLEAN DEFAULT FALSE,
    rotate_days INT DEFAULT 90,
    last_rotated_at TIMESTAMP NULL,
    last_used_at TIMESTAMP NULL,
    expires_at TIMESTAMP NULL,
    created_by VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_vendor_service (vendor, service),
    INDEX idx_enabled (enabled),
    INDEX idx_tier (tier),
    INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- API密钥使用记录表 (永久保存)
-- ============================================
CREATE TABLE IF NOT EXISTS api_key_usage_log (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    key_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    feature VARCHAR(64) NOT NULL,
    request_size INT,
    response_size INT,
    status VARCHAR(16) NOT NULL,
    error_code VARCHAR(32),
    latency_ms INT,
    cost_amount DECIMAL(10,4),
    cost_currency VARCHAR(8) DEFAULT 'CNY',
    requested_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP NULL,
    INDEX idx_key_time (key_id, requested_at),
    INDEX idx_feature_time (feature, requested_at),
    INDEX idx_requested_at (requested_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
PARTITION BY RANGE (TO_DAYS(requested_at)) (
    PARTITION p202401 VALUES LESS THAN (TO_DAYS('2024-02-01')),
    PARTITION p202402 VALUES LESS THAN (TO_DAYS('2024-03-01')),
    PARTITION p202403 VALUES LESS THAN (TO_DAYS('2024-04-01')),
    PARTITION p_future VALUES LESS THAN MAXVALUE
);

-- ============================================
-- 已注册服务表
-- ============================================
CREATE TABLE IF NOT EXISTS registered_services (
    id VARCHAR(64) PRIMARY KEY,
    service_type VARCHAR(64) NOT NULL,
    version VARCHAR(32),
    hostname VARCHAR(128),
    ip_address VARCHAR(64),
    port INT,
    capabilities JSON,
    resources JSON,
    performance JSON,
    status VARCHAR(32) NOT NULL,           -- healthy, degraded, unhealthy, draining, terminated
    last_heartbeat TIMESTAMP NOT NULL,
    heartbeat_missed INT DEFAULT 0,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    -- 运行时状态
    current_load DECIMAL(5,2) DEFAULT 0,
    queue_size INT DEFAULT 0,
    processed_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    cpu_utilization DECIMAL(5,2) DEFAULT 0,
    gpu_utilization DECIMAL(5,2) DEFAULT 0,
    memory_usage BIGINT DEFAULT 0,
    metadata JSON,
    INDEX idx_type_status (service_type, status),
    INDEX idx_last_heartbeat (last_heartbeat),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- 队列统计快照表 (永久保存)
-- ============================================
CREATE TABLE IF NOT EXISTS queue_stats_snapshot (
    snapshot_id BIGINT PRIMARY KEY AUTO_INCREMENT,
    snapshot_time TIMESTAMP NOT NULL,
    feature VARCHAR(64) NOT NULL,
    queue_depth INT NOT NULL,
    queue_wait_ms_p50 INT,
    queue_wait_ms_p95 INT,
    queue_wait_ms_p99 INT,
    queue_wait_ms_max INT,
    waiting_requests INT DEFAULT 0,
    processing_requests INT DEFAULT 0,
    completed_requests INT DEFAULT 0,
    failed_requests INT DEFAULT 0,
    exec_time_ms_p50 INT,
    exec_time_ms_p95 INT,
    exec_time_ms_p99 INT,
    routed_to_self_hosted INT DEFAULT 0,
    routed_to_third_party INT DEFAULT 0,
    INDEX idx_feature_time (feature, snapshot_time),
    INDEX idx_snapshot_time (snapshot_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- 请求日志表
-- ============================================
CREATE TABLE IF NOT EXISTS request_log (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    request_id VARCHAR(64) UNIQUE NOT NULL,
    feature VARCHAR(64) NOT NULL,
    provider_type VARCHAR(32) NOT NULL,
    provider_id VARCHAR(64),
    api_key_id VARCHAR(64),
    prompt_hash VARCHAR(64),
    prompt_length INT,
    parameters JSON,
    received_at TIMESTAMP NOT NULL,
    dispatched_at TIMESTAMP NULL,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    wait_time_ms INT,
    queue_time_ms INT,
    exec_time_ms INT,
    total_latency_ms INT,
    status VARCHAR(32) NOT NULL,
    error_code VARCHAR(32),
    error_message TEXT,
    tokens_input INT DEFAULT 0,
    tokens_output INT DEFAULT 0,
    image_count INT DEFAULT 0,
    cost_compute DECIMAL(10,4) DEFAULT 0,
    cost_api DECIMAL(10,4) DEFAULT 0,
    cost_total DECIMAL(10,4) DEFAULT 0,
    tenant_id VARCHAR(64),
    user_id VARCHAR(64),
    trace_id VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_feature_time (feature, received_at),
    INDEX idx_provider (provider_type, provider_id),
    INDEX idx_status (status, received_at),
    INDEX idx_tenant (tenant_id, received_at),
    INDEX idx_trace_id (trace_id),
    INDEX idx_cost (cost_total)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- 配置变更历史表 (永久保存)
-- ============================================
CREATE TABLE IF NOT EXISTS config_change_log (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    config_type VARCHAR(64) NOT NULL,
    config_id VARCHAR(64) NOT NULL,
    action VARCHAR(32) NOT NULL,          -- create, update, delete, enable, disable
    old_value JSON,
    new_value JSON,
    changed_fields JSON,
    changed_by VARCHAR(64) NOT NULL,
    change_reason TEXT,
    approved_by VARCHAR(64),
    approved_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_config (config_type, config_id),
    INDEX idx_changed_by (changed_by, created_at),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- 成本统计表 (永久保存)
-- ============================================
CREATE TABLE IF NOT EXISTS cost_statistics (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    statistic_date DATE NOT NULL,
    feature VARCHAR(64) NOT NULL,
    provider_type VARCHAR(32) NOT NULL,
    provider_id VARCHAR(64),
    tenant_id VARCHAR(64),
    request_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    failed_count INT DEFAULT 0,
    total_tokens_input BIGINT DEFAULT 0,
    total_tokens_output BIGINT DEFAULT 0,
    total_images BIGINT DEFAULT 0,
    total_gpu_seconds BIGINT DEFAULT 0,
    cost_compute DECIMAL(12,4) DEFAULT 0,
    cost_api DECIMAL(12,4) DEFAULT 0,
    cost_storage DECIMAL(12,4) DEFAULT 0,
    cost_network DECIMAL(12,4) DEFAULT 0,
    cost_total DECIMAL(12,4) DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_date_feature (statistic_date, feature, provider_type, provider_id, tenant_id),
    INDEX idx_date (statistic_date),
    INDEX idx_feature (feature),
    INDEX idx_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- 初始化数据
-- ============================================

-- 插入默认功能配置
INSERT INTO features (id, name, category, description, enabled) VALUES
('text_to_image', '文生图', 'image_generation', '根据文本描述生成图像', TRUE),
('image_editing', '图像编辑', 'image_editing', '对图像进行编辑和修改', TRUE),
('image_stylization', '图像风格化', 'image_processing', '对图像进行风格化处理', TRUE),
('text_generation', '文本生成', 'text_generation', '根据提示生成文本', TRUE)
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;

-- 插入默认Provider配置示例
INSERT INTO provider_configs (id, feature_id, type, vendor, enabled, priority, weight, model, tier) VALUES
('self_hosted_sdxl', 'text_to_image', 'self_hosted', NULL, TRUE, 1, 70, 'sdxl', 'primary'),
('openai_dalle', 'text_to_image', 'third_party', 'openai', TRUE, 2, 30, 'dall-e-3', 'backup')
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;
