package utils

import (
	"github.com/gin-gonic/gin"
)

func SetCookie(c *gin.Context, token string, maxAge int) {
	c.SetCookie(
		"jwt",  //key
		token,  //value
		maxAge, //maxAge default: 3600*5
		"/",    //path
		"",     // domain
		false,  //secure (true for https)
		true,   //httpOnly
	)
}

func ClearCookie(c *gin.Context) {
	SetCookie(c, "", -1)
}
