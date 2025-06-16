package web

import (
	"net/http"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

func (app *GinApp) setupRoutes() {
	app.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if app.ginConfig.EnablePprof {
		pprof.RouteRegister(&app.engine.RouterGroup, "/debug/pprof")
	}
}
