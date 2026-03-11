package modules

import (
	"database/sql"
	"fmt"
	"proton-rds-mgmt/common"
	"sync"

	"github.com/robfig/cron/v3"
)

//go:generate mockgen -package mock -source ./check_table.go -destination ./mock/mock_check_table.go

var (
	cOnce sync.Once
	w     CheckTableWorker
)

type CheckTableWorker interface {
	SetConfig(config *common.Config)
	GetLastCheckResult() []string
	ResetCheckResult()
	Check()
	InitDB() (db *sql.DB, err error)
}

type checkTableWorker struct {
	lock          *sync.Mutex
	config        *common.Config
	logger        common.Logger
	corruptTables []string
}

func (c *checkTableWorker) GetLastCheckResult() []string {
	c.logger.Debugln("GetLastCheckResult: ", c.corruptTables)
	return c.corruptTables
}

func (c *checkTableWorker) ResetCheckResult() {
	c.logger.Debugln("Enter ResetCheckResult")
	c.corruptTables = []string{}
}

func (c *checkTableWorker) CheckTableOK(tableName string, db *sql.DB) (bool, error) {
	c.logger.Debugln("Enter CheckTableOK")
	c.logger.Debugln("check table", tableName)
	rows, err := db.Query("CHECK TABLE " + tableName)
	if err != nil {
		err = common.ParseMySQLError(err)
		c.logger.Errorln(err)
		return true, err
	}
	var table, op, msgType, msgText string
	for rows.Next() {
		if err = rows.Scan(&table, &op, &msgType, &msgText); err != nil {
			err = common.ParseMySQLError(err)
			c.logger.Errorln(err)
			return true, err
		}
		c.logger.Debugln("    msgText: ", msgText)
		if msgType == "error" {
			return false, nil
		}
	}
	return true, nil
}

func (c *checkTableWorker) ListTables(db *sql.DB) ([]string, error) {
	var tableName string
	var tableList []string
	rows, err := db.Query("select  concat(table_schema, '.', table_name) " +
		"from information_schema.tables " +
		"where table_schema not in ('mysql', 'information_schema', 'performance_schema')")
	if err != nil {
		return []string{}, err
	}
	for rows.Next() {
		if err = rows.Scan(&tableName); err != nil {
			err = common.ParseMySQLError(err)
			c.logger.Errorln(err)
			return []string{}, err
		}
		tableList = append(tableList, tableName)
	}
	return tableList, nil
}

func (c *checkTableWorker) InitDB() (db *sql.DB, err error) {
	//构建连接："用户名:密码@tcp(IP:端口)/数据库?charset=utf8"
	path := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema?charset=utf8", c.config.RootUserName, c.config.RootPassword, c.config.RDSHost, c.config.RDSPort)
	db, err = sql.Open("mysql", path)
	if err != nil {
		err = common.ParseMySQLError(err)
		return
	}

	err = db.Ping()
	if err != nil {
		err = common.ParseMySQLError(err)
	}

	return
}

func (c *checkTableWorker) Check() {
	c.logger.Infoln("CheckTable begin")
	c.lock.Lock()
	defer c.lock.Unlock()

	db, err := c.InitDB()
	if err != nil {
		c.logger.Errorln(err)
		return
	}

	tableList, err := c.ListTables(db)
	if err != nil {
		c.logger.Errorln(err)
		return
	}

	c.corruptTables = []string{}
	for _, table := range tableList {
		tableOK, err := c.CheckTableOK(table, db)
		if err != nil {
			c.logger.Errorln(err)
			continue
		}
		if !tableOK {
			c.corruptTables = append(c.corruptTables, table)
		}
	}
	c.logger.Infoln("CheckTable end")
}

func (c *checkTableWorker) WatchAndCheck() {
	if c.config.CronCheckTable {
		c.Check()
	} else {
		c.logger.Infoln("Skip Check Table")
	}
}

func (c *checkTableWorker) CronCheck(spec string) {
	c.logger.Infoln("CronCheck")
	cr := cron.New()
	cr.AddFunc(spec, c.WatchAndCheck)
	cr.Start()
}

func (c *checkTableWorker) SetConfig(config *common.Config) {
	c.logger.Infoln("Set Config")
	c.config = config
	c.CronCheck(c.config.CheckTableSpec)
}

func NewCheckTableWorker() CheckTableWorker {
	cOnce.Do(func() {
		w = &checkTableWorker{
			logger:        common.NewLogger(),
			corruptTables: []string{},
			lock:          new(sync.Mutex),
		}
	})
	return w
}
