-- QACache数据表结构设计 - 简化版以适配现有实现

-- 创建qacache表：直接存储问题和答案内容
CREATE TABLE IF NOT EXISTS qacache (
    id SERIAL PRIMARY KEY,
    content TEXT NOT NULL, -- 原始内容，包含问题和答案信息，格式与markdown文件相同
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 添加索引以提高查询性能（可选，由于是全部查询，无需索引也行）
-- CREATE INDEX IF NOT EXISTS idx_qacache_content ON qacache USING gin (content gin_trgm_ops);

-- 注意事项：
-- 1. 此表设计直接存储content，完全适配现有的文件读取实现
-- 2. content字段格式应与markdown文件相同，包含<!-- aliases: [...] -->元数据
-- 3. 应用程序将负责解析content中的问题和答案信息
-- 4. 如需使用全文搜索功能，建议安装pg_trgm扩展
--    CREATE EXTENSION IF NOT EXISTS pg_trgm;
