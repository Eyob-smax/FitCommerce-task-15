package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Meta struct {
	Page    int `json:"page,omitempty"`
	PerPage int `json:"per_page,omitempty"`
	Total   int `json:"total,omitempty"`
}

type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func OKPaginated(c *gin.Context, data interface{}, meta Meta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func BadRequest(c *gin.Context, code, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": ErrorDetail{Code: code, Message: message}})
}

func Unauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": ErrorDetail{Code: "UNAUTHORIZED", Message: "authentication required"}})
}

func Forbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": ErrorDetail{Code: "FORBIDDEN", Message: "insufficient permissions"}})
}

func NotFound(c *gin.Context, entity string) {
	c.JSON(http.StatusNotFound, gin.H{"error": ErrorDetail{Code: "NOT_FOUND", Message: entity + " not found"}})
}

func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, gin.H{"error": ErrorDetail{Code: "CONFLICT", Message: message}})
}

func Unprocessable(c *gin.Context, message string, fields map[string]string) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{"error": ErrorDetail{Code: "VALIDATION_ERROR", Message: message, Fields: fields}})
}

func InternalError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"}})
}

func NotImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": ErrorDetail{Code: "NOT_IMPLEMENTED", Message: "this endpoint is not yet implemented"}})
}
