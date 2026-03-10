package model

import (
	"log"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDatabaseMigration 测试是否能够正常建表和简单的读写操作
// 使用 SQLite 内存数据库进行快速验证，不需要依赖外部 PostgreSQL
func TestDatabaseMigration(t *testing.T) {
	// 1. 连接到 SQLite 内存数据库
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("无法连接到内存数据库: %v", err)
	}

	// 2. 运行自动迁移
	log.Println("正在运行数据库迁移...")
	models := AllModels()
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}
	log.Println("数据库迁移成功完成！所有表已经建立。")

	// 3. 测试插入数据：创建一个带权限的Admin
	admin := Admin{
		Username: "test_admin",
		Password: "hashed_password",
		Role:     RoleAdmin,
		Status:   AdminStatusActive,
		Permissions: []AdminPermission{
			{Permission: PermissionEditFile},
			{Permission: PermissionReviewSubmission},
		},
	}

	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("创建 Admin 失败: %v", err)
	}

	// 4. 测试 UUID 钩子：创建一个 File
	file := File{
		Name:        "测试文件.pdf",
		StoragePath: "/storage/test.pdf",
		Size:        1024,
		MimeType:    "application/pdf",
	}

	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("创建 File 失败: %v", err)
	}

	// 验证 UUID 是否由钩子生成
	if file.ID == uuid.Nil {
		t.Fatalf("BeforeCreate 钩子失败，File ID 仍然是空 UUID")
	}

	log.Printf("测试成功！创建了 Admin (ID: %d) 和 File (UUID: %s)\n", admin.ID, file.ID.String())
}
