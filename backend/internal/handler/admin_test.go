package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openshare/backend/internal/config"
	"github.com/openshare/backend/internal/handler"
	"github.com/openshare/backend/internal/model"
	"github.com/openshare/backend/internal/router"
	"github.com/openshare/backend/internal/service"
	"github.com/openshare/backend/pkg/jwt"
	"github.com/openshare/backend/pkg/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRouter(t *testing.T) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)

	// 初始化一个空日志配置
	log, _ := logger.New("info", "console")

	// 使用 SQLite 内存数据库作为测试底座
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("连通数据库失败: %v", err)
	}

	// 运行全部模型构建
	err = db.AutoMigrate(model.AllModels()...)
	if err != nil {
		t.Fatalf("自动迁移失败: %v", err)
	}

	// 模拟必要配置
	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "test"},
		JWT:    config.JWTConfig{Secret: "test_secret_key", ExpireHour: 24},
	}

	jwtManager := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.ExpireHour)

	// 构建服务层
	services := service.New(&service.Options{
		DB:     db,
		Config: cfg,
		Logger: log,
	})

	// 初始化默认超级管理员并捕捉系统生成的强随机密码
	created, password, err := services.Admin.InitSuperAdmin()
	if err != nil {
		t.Fatalf("初始化超级管理员失败: %v", err)
	}
	if !created {
		t.Fatalf("未能成功创建超级管理员（被跳过）")
	}
	t.Logf("[模拟] 极强随机超管密码已生成: %s", password)

	// 构建接口层
	handlers := handler.New(&handler.Options{
		Services:   services,
		Config:     cfg,
		Logger:     log,
		JWTManager: jwtManager,
	})

	// 构建路由层
	r := router.Setup(&router.Options{
		Config:     cfg,
		Logger:     log,
		Handlers:   handlers,
		JWTManager: jwtManager,
	})

	return r, password
}

func TestAdminLoginAndAuth(t *testing.T) {
	r, password := setupTestRouter(t)

	// ============================================
	// 测试步骤一：发起账号密码 POST 请求尝试登录
	// ============================================
	loginReqBody := map[string]string{
		"username": "admin",
		"password": password,
	}
	jsonBody, _ := json.Marshal(loginReqBody)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// 使用 httptest 发送虚拟请求，不会占用系统真实端口
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 校验通过状态
	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, 实际得到 %v: %v", w.Code, w.Body.String())
	}

	// 解析出返回结果里的 JWT Token
	var loginResp struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Login Response 反序列化失败: %v", err)
	}

	token := loginResp.Data.Token
	if token == "" {
		t.Fatalf("没有接到服务器下发的 Token!")
	}
	t.Logf("[测试 1 万岁] 成功获取到登录发票 Token: %v...", token[:15])

	// ============================================
	// 测试步骤二：携带刚获取的 Token，试图访问受保护的自身信息接口
	// ============================================
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
	// 将 Token 插入请求头部，试图穿过 Auth() 中间件防线
	req2.Header.Set("Authorization", "Bearer "+token)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, 实际得到 %v: %v", w2.Code, w2.Body.String())
	}

	// 验证请求被解析出的 Context 结果
	var meResp struct {
		Code int `json:"code"`
		Data struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("Me Response 反序列化失败: %v", err)
	}

	if meResp.Data.Username != "admin" {
		t.Fatalf("期望拿到 admin 信息, 实际拿到了 %v", meResp.Data.Username)
	}

	t.Logf("[测试 2 万岁] 取回身份成功! 用户名: %v, 角色权限: %v", meResp.Data.Username, meResp.Data.Role)
}
