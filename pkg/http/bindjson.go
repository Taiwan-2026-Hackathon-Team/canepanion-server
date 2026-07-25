package http

import (
	appErr "canepanion-server/pkg/errors"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

func BindJSON[T any](c *gin.Context) (*T, error) {
	var req T
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, appErr.NewBadRequest("Invalid JSON request body", err)
	}

	return &req, nil
}

func BindQuery[T any](c *gin.Context) (*T, error) {
	var req T
	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, appErr.NewBadRequest("Invalid query parameters", err)
	}

	return &req, nil
}

func BindFormJSON[T any](c *gin.Context, field string) (*T, error) {
	var req T

	jsonStr := c.PostForm(field)
	if jsonStr == "" {
		return nil, appErr.NewBadRequest("Mssing '"+field+"' field", nil)
	}

	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		return nil, appErr.NewBadRequest("Invalid JSON in '"+field+"' field", err)
	}

	return &req, nil
}
