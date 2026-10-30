package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ==================== STRUCTS ====================

// SuccessResponse — untuk semua response sukses (GET, POST, PUT, PATCH, DELETE)
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta — khusus untuk pagination di GET list
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"perPage"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

// ErrorResponse — untuk semua error
type ErrorResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
}

// ==================== HELPERS ====================

// OK — 200, dengan data
func OK(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// OKWithMeta — 200, dengan data + pagination
func OKWithMeta(c *gin.Context, message string, data interface{}, meta Meta) {
	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    &meta,
	})
}

// Created — 201, untuk POST create
func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Fail — untuk error, dengan status code & optional field errors
func Fail(c *gin.Context, statusCode int, message string, errs map[string]string) {
	c.JSON(statusCode, ErrorResponse{
		Success: false,
		Message: message,
		Errors:  errs,
	})
}

// Shorthand untuk error umum
func BadRequest(c *gin.Context, message string, errs map[string]string) {
	Fail(c, http.StatusBadRequest, message, errs)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, message, nil)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, message, nil)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, message, nil)
}

func UnprocessableEntity(c *gin.Context, message string, errs map[string]string) {
	Fail(c, http.StatusUnprocessableEntity, message, errs)
}

func InternalServerError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, message, nil)
}

func TooManyRequests(c *gin.Context, message string) {
	Fail(c, http.StatusTooManyRequests, message, nil)
}
