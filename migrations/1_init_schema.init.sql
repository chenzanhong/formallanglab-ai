-- 创建自定义 AI 模型表
CREATE TABLE IF NOT EXISTS custom_ai_models (
    id BIGSERIAL PRIMARY KEY,              -- 修改点 1: 使用 BIGSERIAL 替代 AUTO_INCREMENT
    user_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,            -- 修改点 2: 指定 VARCHAR 长度
    provider VARCHAR(100) NOT NULL,        -- 修改点 2: 指定长度
    api_base_url TEXT NOT NULL,            -- 修改点 2: 长网址建议用 TEXT
    api_key TEXT NOT NULL,                 -- 修改点 2: 密钥建议用 TEXT
    model_name VARCHAR(255) NOT NULL,      -- 修改点 2: 指定长度
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 创建用户 AI 配置表
CREATE TABLE IF NOT EXISTS user_ai_profiles (
    id BIGSERIAL PRIMARY KEY,              -- 修改点 1: 使用 BIGSERIAL
    user_id BIGINT NOT NULL,
    current_model_id BIGINT DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_ai_profiles_user_id_current_model_id
ON user_ai_profiles(user_id, current_model_id);