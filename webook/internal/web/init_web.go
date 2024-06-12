package web

import "github.com/gin-gonic/gin"

func RegisterRoutes() *gin.Engine {
	server := gin.Default()
	err := server.Run()
	if err != nil {
		return nil
	}
	return server
}
