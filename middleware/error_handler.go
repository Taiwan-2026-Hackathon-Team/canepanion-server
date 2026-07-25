package middleware

import (
	"log"
	"net/http"
	"os"

	appErr "canepanion-server/pkg/errors"

	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			log.Printf("Error: %v", err)

			debug := os.Getenv("GIN_MODE") != "release"

			if appError, ok := err.(*appErr.AppError); ok {
				response := gin.H{
					"success": false,
					"message": appError.Message,
				}
				if debug && appError.Err != nil {
					response["error"] = appError.Err.Error()
				}
				c.JSON(appError.Code, response)
				return
			}

			response := gin.H{
				"success": false,
				"message": "Internal server error",
			}
			if debug {
				response["error"] = err.Error()
			}
			c.JSON(http.StatusInternalServerError, response)
		}

	}
}
