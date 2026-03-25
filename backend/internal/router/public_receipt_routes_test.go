package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicReceiptCodeEnsuresSessionCode(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	engine := New(db, cfg, newRouterSessionManager(db))

	request := httptest.NewRequest(http.MethodGet, "/api/public/receipt-code", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptCode string `json:"receipt_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if response.ReceiptCode == "" {
		t.Fatal("expected receipt code to be generated")
	}
	if len(recorder.Result().Cookies()) == 0 || recorder.Result().Cookies()[0].Value == "" {
		t.Fatal("expected receipt code cookie to be set")
	}
	if recorder.Result().Cookies()[0].MaxAge != 180*24*60*60 {
		t.Fatalf("expected persistent receipt code cookie MaxAge=%d, got %d", 180*24*60*60, recorder.Result().Cookies()[0].MaxAge)
	}
}

func TestPublicReceiptCodeReusesCookie(t *testing.T) {
	cfg := newRouterTestConfig(t)
	db := newRouterTestDB(t)
	engine := New(db, cfg, newRouterSessionManager(db))

	request := httptest.NewRequest(http.MethodGet, "/api/public/receipt-code", nil)
	request.AddCookie(&http.Cookie{Name: "openshare_receipt_code", Value: "SESSION88"})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptCode string `json:"receipt_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if response.ReceiptCode != "SESSION88" {
		t.Fatalf("expected existing receipt code SESSION88, got %q", response.ReceiptCode)
	}
	if len(recorder.Result().Cookies()) == 0 || recorder.Result().Cookies()[0].MaxAge != 180*24*60*60 {
		t.Fatalf("expected reused receipt code cookie MaxAge=%d", 180*24*60*60)
	}
}
