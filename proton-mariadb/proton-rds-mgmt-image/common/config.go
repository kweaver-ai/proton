package common

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// svcConfig 服务配置信息
var configFileName string = "/etc/rds-mgmt-conf/rds-mgmt.yaml"
var svcConfig *Config
var viperCfg *viper.Viper = viper.New()
var logger Logger = NewLogger()

// Config 配置文件
type Config struct {
	Lang           string `yaml:"lang"`
	LogLevel       string `yaml:"logLevel"`
	Port           int
	AgentPort      int
	UseEncryption  bool
	OAuthON        bool
	HydraURL       string
	CronCheckTable bool
	CheckTableSpec string
	RDSHost        string
	RDSPort        int
	RootUserName   string
	RootPassword   string
	CRName         string
	Namespace      string
}

var configOnce sync.Once

// NewConfig 读取服务配置
func NewConfig() *Config {
	configOnce.Do(func() {
		svcConfig = &Config{}
		initConfig(viperCfg)
	})

	return svcConfig
}

func initConfig(v *viper.Viper) {
	logger.Infoln(fmt.Sprintf("Init Config File %s", configFileName))

	v.AddConfigPath("/etc/rds-mgmt-conf")
	v.SetConfigName("rds-mgmt")
	v.SetConfigType("yaml")

	loadConfig(v)

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		logger.Infoln("Config file changed:", e)
		loadConfig(v)
	})
	svcConfig.RDSHost = os.Getenv("MYSQL_HOST")
	port, err := strconv.Atoi(os.Getenv("MYSQL_PORT"))
	if err != nil {
		logger.Fatalln(fmt.Sprintf("err:%s\n", err))
	}
	svcConfig.RDSPort = port
	svcConfig.RootUserName = os.Getenv("MYSQL_ROOT_USER")
	svcConfig.RootPassword = os.Getenv("MYSQL_ROOT_PASSWORD")
	svcConfig.UseEncryption = false
	svcConfig.OAuthON = false
	svcConfig.HydraURL = "xxx"
	svcConfig.CronCheckTable = false
	svcConfig.CheckTableSpec = "0 1 ? * 0"
	svcConfig.Port = 8888
	svcConfig.CRName = os.Getenv("CR_NAME")
	svcConfig.Namespace = os.Getenv("NAMESPACE")
}

func loadConfig(v *viper.Viper) {
	logger.Infoln(fmt.Sprintf("Load Config File %s", configFileName))

	if err := v.ReadInConfig(); err != nil {
		logger.Fatalln(fmt.Sprintf("err:%s\n", err))
	}

	if err := v.Unmarshal(&svcConfig); err != nil {
		logger.Fatalln(fmt.Sprintf("err:%s\n", err))
	}

	logger.Infoln(svcConfig)
	logger.SetLogLevel(svcConfig.LogLevel)
}
