package api

import (
	mastergrpc "github.com/crawlab-team/spider-lab/internal/master/grpc"
	"github.com/crawlab-team/spider-lab/internal/master/scheduler"
	"github.com/crawlab-team/spider-lab/internal/master/spider"
	"github.com/crawlab-team/spider-lab/internal/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(sched *scheduler.Scheduler, deployer *spider.Deployer, grpcServer *mastergrpc.MasterServer, store *storage.Client) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		authCtrl := NewAuthController()
		api.POST("/login", authCtrl.Login)
		api.POST("/register", authCtrl.Register)

		// Public routes: serve spider output/image files
		spiderCtrl := NewSpiderController(deployer, sched, store)
		api.GET("/spiders/:id/tasks/:taskId/output/*filepath", spiderCtrl.ServeOutputFile)
		api.GET("/spiders/:id/images/*filepath", spiderCtrl.ServeImageFile)

		auth := api.Group("", AuthMiddleware())
		{
			nodeCtrl := NewNodeController(grpcServer)
			auth.GET("/nodes", nodeCtrl.GetList)
			auth.GET("/nodes/:id", nodeCtrl.GetById)
			auth.PUT("/nodes/:id", nodeCtrl.Update)
			auth.POST("/nodes/:id/enable", nodeCtrl.Enable)
			auth.POST("/nodes/:id/disable", nodeCtrl.Disable)
			auth.DELETE("/nodes/:id", AdminMiddleware(), nodeCtrl.Delete)

			auth.GET("/spiders/templates", spiderCtrl.GetTemplates)
			auth.GET("/spiders/weibo/search-user", spiderCtrl.SearchWeiboUser)
			auth.GET("/spiders/weibo/lookup-user", spiderCtrl.LookupWeiboUser)
			auth.GET("/spiders/xueqiu/search-user", spiderCtrl.SearchXueqiuUser)
			auth.GET("/spiders/xueqiu/lookup-user", spiderCtrl.LookupXueqiuUser)
			auth.GET("/spiders/xhs/lookup-user", spiderCtrl.LookupXhsUser)
			auth.GET("/spiders/wechat/search-user", spiderCtrl.SearchWechatUser)
			auth.GET("/spiders", spiderCtrl.GetList)
			auth.GET("/spiders/:id", spiderCtrl.GetById)
			auth.POST("/spiders", spiderCtrl.Create)
			auth.PUT("/spiders/:id", spiderCtrl.Update)
			auth.DELETE("/spiders/:id", spiderCtrl.Delete)
			auth.PUT("/spiders/:id/run", spiderCtrl.Run)
			auth.GET("/spiders/:id/files", spiderCtrl.GetFiles)
			auth.GET("/spiders/:id/file", spiderCtrl.GetFileContent)
			auth.PUT("/spiders/:id/file", spiderCtrl.SaveFileContent)
			auth.POST("/spiders/:id/upload", spiderCtrl.Upload)
			auth.PUT("/spiders/:id/config", spiderCtrl.UpdateConfig)

			taskCtrl := NewTaskController(sched, grpcServer)
			auth.GET("/tasks", taskCtrl.GetList)
			auth.GET("/tasks/:id", taskCtrl.GetById)
			auth.POST("/tasks/:id/cancel", taskCtrl.Cancel)
			auth.POST("/tasks/:id/restart", taskCtrl.Restart)
			auth.DELETE("/tasks/:id", taskCtrl.Delete)
			auth.GET("/tasks/:id/logs", taskCtrl.GetLogs)
			auth.GET("/tasks/:id/results", taskCtrl.GetResults)

			schedCtrl := NewScheduleController(sched)
			auth.GET("/schedules", schedCtrl.GetList)
			auth.GET("/schedules/:id", schedCtrl.GetById)
			auth.POST("/schedules", schedCtrl.Create)
			auth.PUT("/schedules/:id", schedCtrl.Update)
			auth.DELETE("/schedules/:id", schedCtrl.Delete)

			resultCtrl := NewResultController()
			auth.GET("/results", resultCtrl.GetList)
			auth.GET("/results/stats", resultCtrl.GetStats)

			postCtrl := NewPostController()
			auth.GET("/posts", postCtrl.GetList)
			auth.GET("/posts/stats", postCtrl.GetStats)

			settingCtrl := NewSettingController()
			auth.GET("/settings", settingCtrl.GetList)
			auth.PUT("/settings/:key", settingCtrl.Update)

			statsCtrl := NewStatsController()
			auth.GET("/stats/overview", statsCtrl.GetOverview)
			auth.GET("/stats/tasks", statsCtrl.GetTaskStats)

			logCtrl := NewLogController()
			auth.GET("/logs/system", logCtrl.GetSystemLogs)

			userCtrl := NewUserController()
			auth.GET("/users/me", userCtrl.GetMe)
			auth.PUT("/users/me", userCtrl.UpdateMe)

			cookieCtrl := NewCookiePoolController()
			auth.GET("/xueqiu/cookies", cookieCtrl.GetList)
			auth.DELETE("/xueqiu/cookies/:id", cookieCtrl.Delete)
			auth.PUT("/xueqiu/cookies/:id/status", cookieCtrl.ToggleStatus)
			auth.POST("/xueqiu/cookies/:id/check", cookieCtrl.CheckHealth)
			auth.POST("/xueqiu/cookies/qr-start", cookieCtrl.QRStart)
			auth.GET("/xueqiu/cookies/qr/:id", cookieCtrl.QRStatus)
			auth.POST("/xueqiu/cookies/qr/:id/cancel", cookieCtrl.QRCancel)

			wechatCookieCtrl := NewWechatCookiePoolController()
			auth.GET("/wechat/cookies", wechatCookieCtrl.GetList)
			auth.DELETE("/wechat/cookies/:id", wechatCookieCtrl.Delete)
			auth.PUT("/wechat/cookies/:id/status", wechatCookieCtrl.ToggleStatus)
			auth.POST("/wechat/cookies/:id/check", wechatCookieCtrl.CheckHealth)
			auth.POST("/wechat/cookies/qr-start", wechatCookieCtrl.QRStart)
			auth.GET("/wechat/cookies/qr/:id", wechatCookieCtrl.QRStatus)
			auth.POST("/wechat/cookies/qr/:id/cancel", wechatCookieCtrl.QRCancel)
			auth.POST("/wechat/cookies/add-manual", wechatCookieCtrl.AddManual)

			targetCtrl := NewTargetAccountController()
			auth.GET("/target-accounts", targetCtrl.GetList)
			auth.GET("/target-accounts/by-platform", targetCtrl.GetByPlatform)
			auth.POST("/target-accounts", targetCtrl.Create)
			auth.DELETE("/target-accounts/:id", targetCtrl.Delete)
		}
	}

	return r
}
