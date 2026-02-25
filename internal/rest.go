package internal

import (
	"kenmec/ha/jimmy/config"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 每次我想買一個東西的時候 這種購物欲都會持續很久
// 我想買東北的famas 幹實在太貴
// 我才剛買一把槍
// 但是又很想買
// 每次只要打完生存
// 這種感覺就會更強烈

func StartRestWebApi(arbiter *Arbiter) {
	r := gin.Default()
	r.Use(cors.Default())

	r.POST("/health", func(ctx *gin.Context) {
		arbiter.mu.RLock()
		defer arbiter.mu.RUnlock()

		if arbiter.Self.ECS && arbiter.Self.Fleet {
			ctx.JSON(http.StatusOK, gin.H{
				"status": "ok",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status": "not ok",
				"ecs":    arbiter.Self.ECS,
				"fleet":  arbiter.Self.Fleet,
			})
		}

	})

	r.Run(":" + config.Cfg.WEB_API_PORT)
}
