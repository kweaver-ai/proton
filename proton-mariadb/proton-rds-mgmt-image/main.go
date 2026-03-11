package main

import (
	"fmt"

	"proton-rds-mgmt/common"
	"proton-rds-mgmt/server"

	"github.com/gin-gonic/gin"
)

type rdsMgmt struct {
	cHandler server.RESTHandler
}

func (m *rdsMgmt) Start() {
	logger := common.NewLogger()
	logger.Infoln("Start Proton RDS Mgmt")

	config := common.NewConfig()

	// 设置错误码语言
	common.SetLang(config.Lang)

	gin.SetMode(gin.ReleaseMode)

	m.cHandler.SetConfig(config)

	go func() {
		engine := gin.New()
		engine.Use(gin.Recovery())
		engine.UseRawPath = true

		// 注册公共开放API
		m.cHandler.RegisterPublic(engine)

		if err := engine.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
			logger.Errorln(err)
		}
	}()
}

func main() {
	server := &rdsMgmt{
		cHandler: server.NewRESTHandler(),
	}
	server.Start()

	select {}
}
