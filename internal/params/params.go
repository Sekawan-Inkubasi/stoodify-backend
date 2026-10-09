package params

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"stoodify_backend/internal/response"
)

// IDParam mengambil :id dari path sebagai uint; balas 400 kalau bukan angka.
func IDParam(konteks *gin.Context, name string) (uint, bool) {
	raw := konteks.Param(name)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(konteks, "parameter "+name+" tidak valid", map[string]string{
			name: "harus berupa angka positif",
		})
		return 0, false
	}
	return uint(id), true
}

// Pagination membaca page/per_page dengan batas aman.
func Pagination(konteks *gin.Context, defaultPerPage, maxPerPage int) (page, perPage, offset int) {
	page, _ = strconv.Atoi(konteks.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ = strconv.Atoi(konteks.DefaultQuery("per_page", strconv.Itoa(defaultPerPage)))
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset = (page - 1) * perPage
	return
}
