package handler

import "github.com/gin-gonic/gin"

const publicReceiptCookieName = "openshare_receipt_code"
const publicReceiptCookieMaxAgeSeconds = 180 * 24 * 60 * 60

func readPublicReceiptCode(ctx *gin.Context) string {
	value, err := ctx.Cookie(publicReceiptCookieName)
	if err != nil {
		return ""
	}
	return value
}

func writePublicReceiptCode(ctx *gin.Context, receiptCode string) {
	ctx.SetCookie(publicReceiptCookieName, receiptCode, publicReceiptCookieMaxAgeSeconds, "/", "", false, false)
}
