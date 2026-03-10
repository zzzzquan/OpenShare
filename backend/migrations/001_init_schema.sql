-- OpenShare 数据库初始化脚本
-- 此脚本仅供参考，实际迁移由 GORM AutoMigrate 完成

-- 启用必要扩展
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================
-- 管理员表
-- ============================================================
CREATE TABLE IF NOT EXISTS admins (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,           -- bcrypt 哈希
    role VARCHAR(20) NOT NULL DEFAULT 'admin', -- admin, super_admin
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- active, disabled
    last_login TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_admins_deleted_at ON admins(deleted_at);

-- 管理员权限表
CREATE TABLE IF NOT EXISTS admin_permissions (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    permission VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_permissions_unique ON admin_permissions(admin_id, permission);

-- ============================================================
-- 文件夹表
-- ============================================================
CREATE TABLE IF NOT EXISTS folders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    parent_id UUID REFERENCES folders(id) ON DELETE CASCADE,
    path VARCHAR(1000) NOT NULL,               -- 完整路径，如 /课程资料/数学
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_folders_parent_id ON folders(parent_id);
CREATE INDEX IF NOT EXISTS idx_folders_path ON folders(path);
CREATE INDEX IF NOT EXISTS idx_folders_deleted_at ON folders(deleted_at);
CREATE INDEX IF NOT EXISTS idx_folders_name_trgm ON folders USING gin (name gin_trgm_ops);

-- ============================================================
-- 文件表
-- ============================================================
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,                -- 原始文件名
    storage_path VARCHAR(1000) NOT NULL,       -- 磁盘存储路径
    size BIGINT NOT NULL,                       -- 文件大小（字节）
    mime_type VARCHAR(100),
    extension VARCHAR(20),
    folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    downloads BIGINT NOT NULL DEFAULT 0,
    title VARCHAR(255),                         -- 资料标题（可选）
    description TEXT,
    hash VARCHAR(64),                           -- 文件哈希（用于去重）
    upload_ip VARCHAR(45),                      -- 上传者IP
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_files_folder_id ON files(folder_id);
CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
CREATE INDEX IF NOT EXISTS idx_files_hash ON files(hash);
CREATE INDEX IF NOT EXISTS idx_files_deleted_at ON files(deleted_at);
CREATE INDEX IF NOT EXISTS idx_files_name_trgm ON files USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_files_title_trgm ON files USING gin (title gin_trgm_ops);

-- ============================================================
-- 标签表
-- ============================================================
CREATE TABLE IF NOT EXISTS tags (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    name_lower VARCHAR(50) NOT NULL UNIQUE,    -- 小写名称，用于唯一性校验
    color VARCHAR(20),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tags_created_by ON tags(created_by);
CREATE INDEX IF NOT EXISTS idx_tags_deleted_at ON tags(deleted_at);
CREATE INDEX IF NOT EXISTS idx_tags_name_trgm ON tags USING gin (name gin_trgm_ops);

-- 文件-标签关联表
CREATE TABLE IF NOT EXISTS file_tags (
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (file_id, tag_id)
);

-- 文件夹-标签关联表
CREATE TABLE IF NOT EXISTS folder_tags (
    folder_id UUID NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (folder_id, tag_id)
);

-- 标签申请表
CREATE TABLE IF NOT EXISTS tag_submissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    submitter_ip VARCHAR(45),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewer_id INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    review_reason TEXT,
    reviewed_at TIMESTAMPTZ,
    tag_id INTEGER REFERENCES tags(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tag_submissions_reviewer_id ON tag_submissions(reviewer_id);
CREATE INDEX IF NOT EXISTS idx_tag_submissions_deleted_at ON tag_submissions(deleted_at);

-- ============================================================
-- 投稿审核表
-- ============================================================
CREATE TABLE IF NOT EXISTS submissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    receipt_code VARCHAR(50) NOT NULL UNIQUE,   -- 回执码
    title VARCHAR(255) NOT NULL,
    description TEXT,
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    staging_path VARCHAR(1000) NOT NULL,        -- 暂存路径
    folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewer_id INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    review_reason TEXT,
    reviewed_at TIMESTAMPTZ,
    file_id UUID REFERENCES files(id) ON DELETE SET NULL, -- 审核通过后关联的文件
    upload_ip VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status);
CREATE INDEX IF NOT EXISTS idx_submissions_reviewer_id ON submissions(reviewer_id);
CREATE INDEX IF NOT EXISTS idx_submissions_deleted_at ON submissions(deleted_at);

-- 投稿-标签关联表
CREATE TABLE IF NOT EXISTS submission_tags (
    submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (submission_id, tag_id)
);

-- ============================================================
-- 举报表
-- ============================================================
CREATE TABLE IF NOT EXISTS reports (
    id SERIAL PRIMARY KEY,
    target_type VARCHAR(20) NOT NULL,          -- file, folder
    target_id UUID NOT NULL,
    reason VARCHAR(50) NOT NULL,               -- 举报原因类型
    description TEXT,
    reporter_ip VARCHAR(45),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewer_id INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    review_reason TEXT,
    reviewed_at TIMESTAMPTZ,
    action VARCHAR(20),                         -- offline, deleted, none
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_reports_target_type ON reports(target_type);
CREATE INDEX IF NOT EXISTS idx_reports_target_id ON reports(target_id);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_reviewer_id ON reports(reviewer_id);
CREATE INDEX IF NOT EXISTS idx_reports_deleted_at ON reports(deleted_at);

-- ============================================================
-- 公告表
-- ============================================================
CREATE TABLE IF NOT EXISTS announcements (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    author_id INTEGER NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_announcements_author_id ON announcements(author_id);
CREATE INDEX IF NOT EXISTS idx_announcements_deleted_at ON announcements(deleted_at);

-- ============================================================
-- 操作日志表（不使用软删除）
-- ============================================================
CREATE TABLE IF NOT EXISTS operation_logs (
    id SERIAL PRIMARY KEY,
    operator_id INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    operator_role VARCHAR(20) NOT NULL,
    action VARCHAR(50) NOT NULL,
    target_type VARCHAR(20),
    target_id VARCHAR(50),
    detail TEXT,                                -- JSON 格式详情
    ip VARCHAR(45) NOT NULL,
    user_agent VARCHAR(500),
    result VARCHAR(20) NOT NULL,               -- success, failure
    error_msg TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_operation_logs_operator_id ON operation_logs(operator_id);
CREATE INDEX IF NOT EXISTS idx_operation_logs_action ON operation_logs(action);
CREATE INDEX IF NOT EXISTS idx_operation_logs_created_at ON operation_logs(created_at);

-- ============================================================
-- 注释说明
-- ============================================================
COMMENT ON TABLE admins IS '管理员账号表';
COMMENT ON TABLE admin_permissions IS '管理员权限表，存储可配置的权限项';
COMMENT ON TABLE folders IS '文件夹表，支持树形结构';
COMMENT ON TABLE files IS '文件元数据表';
COMMENT ON TABLE tags IS '标签表，名称忽略大小写唯一';
COMMENT ON TABLE file_tags IS '文件-标签多对多关联表';
COMMENT ON TABLE folder_tags IS '文件夹-标签多对多关联表';
COMMENT ON TABLE tag_submissions IS '用户标签申请表';
COMMENT ON TABLE submissions IS '投稿审核记录表';
COMMENT ON TABLE submission_tags IS '投稿-标签关联表';
COMMENT ON TABLE reports IS '举报记录表';
COMMENT ON TABLE announcements IS '公告表';
COMMENT ON TABLE operation_logs IS '操作审计日志表';
