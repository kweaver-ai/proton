package modules

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/go-sql-driver/mysql"

	"proton-rds-mgmt/common"
)

//go:generate mockgen -package mock -source ./rds_mgmt.go -destination ./mock/mock_rds_mgmt.go

// RDSMgmt 数据库操作接口
type RDSMgmt interface {
	// SetConfig 设置Config
	SetConfig(config *common.Config)
	// InitDB 初始化连接
	InitDB(adminName string, adminPwd string) (db *sql.DB, err error)
	// CheckDBExist 检查数据库是否存在
	CheckDBExist(db *sql.DB, dbName string) (bool, error)
	// CheckUserExist 检查用户否存在
	CheckUserExist(db *sql.DB, userName string) (bool, error)
	// ListDB 查询数据库
	ListDB(adminName string, adminPwd string) (dbs []DBInfo, err error)
	// CreateDB 创建数据库
	CreateDB(adminName string, adminPwd string, dbinfo DBInfo) error
	// DeleteDB 删除数据库
	DeleteDB(adminName string, adminPwd string, dbName string) error
	// ListUser 查询用户
	ListUser(adminName string, adminPwd string) (users []UserInfo, err error)
	// CreateOrUpdateUser 创建或修改用户
	CreateOrUpdateUser(adminName string, adminPwd string, userName string, password string) error
	// DeleteUser 删除用户
	DeleteUser(adminName string, adminPwd string, userName string) error
	// ModifyUserPrivilege 修改用户角色
	ModifyUserPrivilege(adminName string, adminPwd string, userInfo UserInfo, resetPriv bool) error
	// CheckPermission 检查是否管理员权限
	CheckPermission(string, string) error
	// ModifyUserSsl 用户ssl设置
	ModifyUserSsl(adminName string, adminPwd string, userName string, sslType string) error
	// CheckDbAvailability
	CheckDbAvailability() error
	// GetDBVariables
	GetDBVariables(variableType string, variable string) (string, error)
}

type rdsMgmt struct {
	logger common.Logger
	config *common.Config
}

var (
	rOnce sync.Once
	r     RDSMgmt
)

// NewRDSMgmt 创建数据库操作对象
func NewRDSMgmt() RDSMgmt {
	rOnce.Do(func() {
		r = &rdsMgmt{
			logger: common.NewLogger(),
		}
	})

	return r
}

// SetConfig 设置Config
func (r *rdsMgmt) SetConfig(config *common.Config) {
	r.logger.Infoln("Set Config")
	r.config = config
	if r.config.RootUserName != "" {
		SystemDefaultUser[r.config.RootUserName] = 1
	}
}

// parseMySQLError
func (r *rdsMgmt) parseMySQLError(err error) error {
	r.logger.Errorln(err.Error())
	errCode := common.InternalError
	if e, ok := err.(*mysql.MySQLError); ok {
		switch e.Number {
		case 1045:
			errCode = common.AuthenticationFailed
		case 1044:
			errCode = common.AccessDenied
		case 1142:
			errCode = common.AccessDenied
		default:
		}
	}
	newErr := common.NewHTTPError(err.Error(), errCode, nil)
	return newErr
}

// InitDB 初始化连接
func (r *rdsMgmt) InitDB(adminName string, adminPwd string) (db *sql.DB, err error) {
	//构建连接："用户名:密码@tcp(IP:端口)/数据库?charset=utf8"
	path := fmt.Sprintf("%s:%s@tcp(%s:%d)/information_schema?charset=utf8", adminName, adminPwd, r.config.RDSHost, r.config.RDSPort)
	db, err = sql.Open("mysql", path)
	if err != nil {
		err = r.parseMySQLError(err)
		return
	}

	err = db.Ping()
	if err != nil {
		err = r.parseMySQLError(err)
	}

	return
}

// CheckDBExist 检查数据库是否存在
func (r *rdsMgmt) CheckDBExist(db *sql.DB, dbName string) (bool, error) {
	r.logger.Debugln("CheckDBExist: " + dbName)

	var count int = 0
	err := db.QueryRow("SELECT COUNT(1) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", dbName).Scan(&count)
	if err != nil {
		err = r.parseMySQLError(err)
		r.logger.Errorln("failed to CheckDBExist: " + dbName)
		return false, err
	}

	exist := (count == 1)
	return exist, nil
}

// CheckUserExist 检查用户否存在
func (r *rdsMgmt) CheckUserExist(db *sql.DB, userName string) (bool, error) {
	r.logger.Debugln("CheckUserExist: " + userName)

	var count int = 0
	err := db.QueryRow("SELECT COUNT(1) FROM mysql.user WHERE User=? and Host=?", userName, "%").Scan(&count)
	if err != nil {
		err = r.parseMySQLError(err)
		r.logger.Errorln("failed to CheckUserExist: " + userName)
		return false, err
	}

	exist := (count == 1)
	return exist, nil
}

// ListDB 查询数据库列表
func (r *rdsMgmt) ListDB(adminName string, adminPwd string) (dbs []DBInfo, err error) {
	r.logger.Debugln("ListDB")

	dbs = make([]DBInfo, 0)

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return
	}
	defer db.Close()

	// 查询数据库列表
	rows, err := db.Query("SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA")
	if err != nil {
		err = r.parseMySQLError(err)
		return
	}

	for rows.Next() {
		var dbName, charset, collate string
		if err = rows.Scan(&dbName, &charset, &collate); err != nil {
			err = r.parseMySQLError(err)
			return
		}

		// 忽略系统库
		if _, ok := SystemDefaultDB[dbName]; ok {
			continue
		}

		row := DBInfo{
			DBName:  dbName,
			Charset: charset,
			Collate: collate,
		}

		dbs = append(dbs, row)
	}

	return
}

// CreateDB 创建数据库
func (r *rdsMgmt) CreateDB(adminName string, adminPwd string, dbinfo DBInfo) error {
	r.logger.Debugln("CreateDB: " + dbinfo.DBName)

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return err
	}
	defer db.Close()

	// 检查数据库是否系统库
	if _, ok := SystemDefaultDB[dbinfo.DBName]; ok {
		r.logger.Errorln("db is system database")
		err = common.NewHTTPError("db is system database", common.BadRequest, nil)
		return err
	}

	// 检查数据库是否存在
	exist, err := r.CheckDBExist(db, dbinfo.DBName)
	if err != nil {
		return err
	}
	if exist {
		r.logger.Errorln("database already exist: " + dbinfo.DBName)
		err = common.NewHTTPError("database already exist", common.DatabaseAlreadyExist, nil)
		return err
	}

	// 创建数据库
	var sqlstr string
	if dbinfo.Collate != "" {
		sqlstr = fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET %s COLLATE %s", dbinfo.DBName, dbinfo.Charset, dbinfo.Collate)
	} else {
		sqlstr = fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET %s", dbinfo.DBName, dbinfo.Charset)
	}

	_, err = db.Exec(sqlstr)
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}
	return nil
}

// DeleteDB 删除数据库
func (r *rdsMgmt) DeleteDB(adminName string, adminPwd string, dbName string) error {
	r.logger.Debugln("DeleteDB: " + dbName)

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return err
	}
	defer db.Close()

	// 检查数据库是否系统库
	if _, ok := SystemDefaultDB[dbName]; ok {
		r.logger.Errorln("db is system database")
		err = common.NewHTTPError("db is system database", common.BadRequest, nil)
		return err
	}

	// 检查数据库是否存在
	exist, err := r.CheckDBExist(db, dbName)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.Errorln("database not exist: " + dbName)
		err = common.NewHTTPError("database not exist", common.DatabaseNotExist, nil)
		return err
	}

	// 删除数据库
	sqlstr := fmt.Sprintf("DROP DATABASE `%s`", dbName)
	_, err = db.Exec(sqlstr)
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}

	//删除该db上的权限
	rows, err := db.Query("SELECT User, Host FROM mysql.db WHERE Db = ?", dbName)
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}
	for rows.Next() {
		var userName, host string
		if err = rows.Scan(&userName, &host); err != nil {
			err = r.parseMySQLError(err)
			return err
		}
		sqlstr := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", dbName, userName, host)
		_, err = db.Exec(sqlstr)
		if err != nil {
			err = r.parseMySQLError(err)
			return err
		}

	}

	return nil
}

// ListUser 查询用户列表
func (r *rdsMgmt) ListUser(adminName string, adminPwd string) (users []UserInfo, err error) {
	r.logger.Debugln("ListUser")

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return
	}
	defer db.Close()

	//查询用户及用户权限
	rows, err := db.Query("select User, ssl_type, null as Db, References_priv, Insert_priv, Create_priv, Select_priv, Super_priv, null as Delete_history_priv  from mysql.user where HOST=?"+
		" union select User, null as ssl_type, Db, References_priv, Insert_priv, Create_priv, Select_priv, null as Super_priv, Delete_history_priv from mysql.db where host=?", "%", "%")
	if err != nil {
		err = r.parseMySQLError(err)
		return
	}

	users = make([]UserInfo, 0)
	userMap := map[string]int{}

	for rows.Next() {
		var userName string
		var dbName, referencesPriv, insertPriv, createPriv, selectPriv, sslType, superPriv, deleteHistoryPriv sql.NullString
		if err = rows.Scan(&userName, &sslType, &dbName, &referencesPriv, &insertPriv, &createPriv, &selectPriv, &superPriv, &deleteHistoryPriv); err != nil {
			err = r.parseMySQLError(err)
			return
		}

		// 忽略系统用户
		if _, ok := SystemDefaultUser[userName]; ok {
			continue
		}

		if _, ok := userMap[userName]; !ok {
			userMap[userName] = len(users)
			users = append(users, UserInfo{userName, []PrivilegeItem{}, ""})
		}

		var DBName string
		var privType string

		if dbName.String == "" {
			DBName = "*"
		} else {
			DBName = dbName.String
		}
		if superPriv.String == "Y" || deleteHistoryPriv.String == "Y" {
			privType = "All"
		} else if referencesPriv.String == "Y" {
			privType = "ReadWrite"
		} else if insertPriv.String == "Y" {
			privType = "DMLOnly"
		} else if createPriv.String == "Y" {
			privType = "DDLOnly"
		} else if selectPriv.String == "Y" {
			privType = "ReadOnly"
		} else {
			privType = "None"
		}
		priv := PrivilegeItem{
			DBName:        DBName,
			PrivilegeType: privType,
		}
		index := userMap[userName]
		users[index].Privileges = append(users[index].Privileges, priv)
		if dbName.String == "" {
			if sslType.String == "ANY" {
				users[index].SslType = "Any"
			} else {
				users[index].SslType = "None"
			}
		}
	}

	return
}

// CreateOrUpdateUser 创建或修改用户
func (r *rdsMgmt) CreateOrUpdateUser(adminName string, adminPwd string, userName string, password string) error {
	r.logger.Debugln("CreateOrUpdateUser: " + userName)

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return err
	}
	defer db.Close()

	// 检查用户是否系统用户
	if _, ok := SystemDefaultUser[userName]; ok {
		r.logger.Errorln("user is system user")
		err = common.NewHTTPError("user is system user", common.BadRequest, nil)
		return err
	}

	// 检查用户是否存在
	var sqlstr string
	exist, err := r.CheckUserExist(db, userName)
	if err != nil {
		return err
	}
	if exist {
		// 存在，修改密码
		sqlstr = fmt.Sprintf("SET PASSWORD FOR `%s`@'%s' = PASSWORD('%s')", userName, "%", password)
	} else {
		// 不存在，创建
		sqlstr = fmt.Sprintf("CREATE USER `%s`@'%s' IDENTIFIED BY '%s'", userName, "%", password)
	}

	_, err = db.Exec(sqlstr)
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}
	return nil
}

// DeleteUser 删除用户
func (r *rdsMgmt) DeleteUser(adminName string, adminPwd string, userName string) error {
	r.logger.Debugln("DeleteUser: " + userName)

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return err
	}
	defer db.Close()

	// 不允许删除admin用户
	if userName == adminName {
		r.logger.Errorln("user is admin user")
		err = common.NewHTTPError("user is admin user", common.BadRequest, nil)
		return err
	}

	// 检查用户是否系统用户
	if _, ok := SystemDefaultUser[userName]; ok {
		r.logger.Errorln("user is system user")
		err = common.NewHTTPError("user is system user", common.BadRequest, nil)
		return err
	}

	// 检查用户是否存在
	exist, err := r.CheckUserExist(db, userName)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.Errorln("user not exist: " + userName)
		err = common.NewHTTPError("user not exist", common.UserNotExist, nil)
		return err
	}

	// 删除用户
	sqlstr := fmt.Sprintf("DROP USER `%s`@'%s'", userName, "%")
	_, err = db.Exec(sqlstr)
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}
	return nil
}

// ModifyUserPrivilege 修改用户角色
func (r *rdsMgmt) ModifyUserPrivilege(adminName string, adminPwd string, userInfo UserInfo, resetPriv bool) error {
	r.logger.Debugln("ModifyUserPrivilege: " + userInfo.UserName)

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return err
	}
	defer db.Close()

	// admin用户权限不允许更新
	if userInfo.UserName == adminName {
		r.logger.Errorln("user is admin user")
		err = common.NewHTTPError("user is admin user", common.BadRequest, nil)
		return err
	}

	// 检查用户是否系统用户
	if _, ok := SystemDefaultUser[userInfo.UserName]; ok {
		r.logger.Errorln("user is system user")
		err = common.NewHTTPError("user is system user", common.BadRequest, nil)
		return err
	}

	// 检查用户是否存在
	exist, err := r.CheckUserExist(db, userInfo.UserName)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.Errorln("user not exist: " + userInfo.UserName)
		err = common.NewHTTPError("user not exist", common.UserNotExist, nil)
		return err
	}

	//PUT请求需要重置全局权限
	resetGlobPriv := false
	if resetPriv {
		resetGlobPriv = true
	}

	dbNames := map[string]int{}
	// 检查DB是否存在
	for _, item := range userInfo.Privileges {
		dbName := item.DBName
		dbNames[dbName] = 1
		privilegeType := item.PrivilegeType

		//PATCH请求变更全局权限时也需要重置全局权限
		if dbName == "*" {
			resetGlobPriv = true
		}

		// 检查权限类型
		if _, ok := PrivilegeTypes[privilegeType]; !ok {
			r.logger.Errorln("privilegeType is invalid")
			err = common.NewHTTPError("privilegeType is invalid", common.BadRequest, nil)
			return err
		}

		exist, err = r.CheckDBExist(db, dbName)
		if err != nil {
			return err
		}
		if !exist && dbName != "*" {
			r.logger.Errorln("database not exist: " + dbName)
			err = common.NewHTTPError("database not exist", common.DatabaseNotExist, nil)
			return err
		}
	}

	// 删除全局权限
	if resetGlobPriv {
		_, err = db.Exec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%"))
		if err != nil {
			err = r.parseMySQLError(err)
			return err
		}
	}

	rows, err := db.Query("SELECT Db FROM mysql.db WHERE User=? AND Host = ?", userInfo.UserName, "%")
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}

	// 删除库级别的权限
	for rows.Next() {
		var dbName string
		if err = rows.Scan(&dbName); err != nil {
			err = r.parseMySQLError(err)
			return err
		}

		if _, ok := dbNames[dbName]; ok || resetPriv {
			sqlstr := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", dbName, userInfo.UserName, "%")
			_, err = db.Exec(sqlstr)
			if err != nil {
				err = r.parseMySQLError(err)
				return err
			}

		}
	}

	// 设置新的权限
	var sqlstr string
	for _, item := range userInfo.Privileges {
		dbName := item.DBName
		privilegeType := item.PrivilegeType
		if privilegeType != "None" {
			if dbName == "*" {
				sqlstr = fmt.Sprintf("GRANT %s ON %s.* TO `%s`@'%s'", PrivilegeTypes[privilegeType], dbName, userInfo.UserName, "%")
			} else {
				sqlstr = fmt.Sprintf("GRANT %s ON `%s`.* TO `%s`@'%s'", PrivilegeTypes[privilegeType], dbName, userInfo.UserName, "%")
			}
			_, err = db.Exec(sqlstr)
			if err != nil {
				err = r.parseMySQLError(err)
				return err
			}
		}
	}
	return nil
}

// 检查是否是管理员用户
func (r *rdsMgmt) CheckPermission(user string, pass string) error {
	r.logger.Debugln("rdsMgmt.CheckPermission: " + user)
	db, err := r.InitDB(user, pass)
	if err != nil {
		return err
	}
	defer db.Close()
	// 只有管理员用户拥有mysql.user权限
	_, err = db.Query("SELECT 1 FROM mysql.user")
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}
	return nil
}

// ModifyUserSsl
func (r *rdsMgmt) ModifyUserSsl(adminName string, adminPwd string, userName string, sslType string) error {
	r.logger.Debugln("ModifyUserSsl: " + sslType)

	// 连接MySQL
	db, err := r.InitDB(adminName, adminPwd)
	if err != nil {
		return err
	}
	defer db.Close()

	// 检查用户是否系统用户
	if _, ok := SystemDefaultUser[userName]; ok {
		r.logger.Errorln("user is system user")
		err = common.NewHTTPError("user is system user", common.BadRequest, nil)
		return err
	}

	// 检查用户是否存在
	exist, err := r.CheckUserExist(db, userName)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.Errorln("user not exist: " + userName)
		err = common.NewHTTPError("user not exist", common.UserNotExist, nil)
		return err
	}

	// 只允许设置ssl为any或none
	if _, ok := SslTypes[sslType]; !ok {
		r.logger.Errorln("ssl_type is invalid")
		err = common.NewHTTPError("ssl_type is invalid", common.BadRequest, nil)
		return err
	}

	// 更改SSL Type
	sqlstr := fmt.Sprintf("ALTER USER `%s`@`%s` REQUIRE %s", userName, "%", SslTypes[sslType])
	_, err = db.Exec(sqlstr)
	if err != nil {
		err = r.parseMySQLError(err)
		return err
	}
	return nil
}

// 检查数据库是否可以对外服务
func (r *rdsMgmt) CheckDbAvailability() error {
	r.logger.Debugln("CheckDbAvailability")

	wsrepOn, err := r.GetDBVariables("variables", "wsrep_on")
	if err != nil {
		return common.NewHTTPError(err.Error(), common.InternalError, nil)
	}

	if wsrepOn == "OFF" {
		return nil
	}

	wsrepReady, err := r.GetDBVariables("status", "wsrep_ready")
	if err != nil {
		return common.NewHTTPError(err.Error(), common.InternalError, nil)
	}

	if wsrepReady == "OFF" {
		wsrepLocalStateComment, _ := r.GetDBVariables("status", "wsrep_local_state_comment")
		return common.NewHTTPError(fmt.Sprintf("wsrep_local_state_comment: %s", wsrepLocalStateComment), common.InternalError, nil)
	}

	return nil
}

// 查询数据库变量
func (r *rdsMgmt) GetDBVariables(variableType string, variable string) (string, error) {
	r.logger.Debugln("GetDBVariables")

	// 连接MySQL
	db, err := r.InitDB("exporter", "mKRb1DAwdWBCBuGs")
	if err != nil {
		return "", err
	}
	defer db.Close()

	preparedSQL := fmt.Sprintf("show global %s like '%s'", variableType, variable)

	var variableName, value string
	err = db.QueryRow(preparedSQL).Scan(&variableName, &value)
	if err != nil {
		return "", err
	}

	return value, nil
}
