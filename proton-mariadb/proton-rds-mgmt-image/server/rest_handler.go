// Package document AnyShare接口处理层
package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"proton-rds-mgmt/common"
	"proton-rds-mgmt/modules"
)

// RESTHandler document RESTful api Handler接口
type RESTHandler interface {
	// SetConfig 设置Config
	SetConfig(config *common.Config)
	// RegisterPublic 注册开放API
	RegisterPublic(engine *gin.Engine)
}

var (
	restOnce sync.Once
	r        RESTHandler
)

type restHandler struct {
	rdsMgmt        modules.RDSMgmt
	backupMgmt     modules.BackupMgmt
	config         *common.Config
	httpClient     common.HTTPClient
	logger         common.Logger
	checkTableMgmt modules.CheckTableWorker
}

// NewRESTHandler 创建document handler对象
func NewRESTHandler() RESTHandler {
	restOnce.Do(func() {
		r = &restHandler{
			rdsMgmt:        modules.NewRDSMgmt(),
			backupMgmt:     modules.NewBackupMgmt(),
			httpClient:     common.NewHTTPClient(),
			logger:         common.NewLogger(),
			checkTableMgmt: modules.NewCheckTableWorker(),
		}
	})

	return r
}

// SetConfig 设置Config
func (r *restHandler) SetConfig(config *common.Config) {
	r.logger.Infoln("Set Config")
	r.config = config
	r.rdsMgmt.SetConfig(config)
	r.backupMgmt.SetConfig(config)
	r.checkTableMgmt.SetConfig(config)
}

// RegisterPublic 注册开放API
func (r *restHandler) RegisterPublic(engine *gin.Engine) {
	r.logger.Infoln("Register API")

	// 查询数据库
	engine.GET("/api/proton-rds-mgmt/v2/dbs", r.ListDB)

	// 创建数据库
	engine.PUT("/api/proton-rds-mgmt/v2/dbs/:dbName", r.CreateDB)

	// 删除数据库
	engine.DELETE("/api/proton-rds-mgmt/v2/dbs/:dbName", r.DeleteDB)

	// 查询用户
	engine.GET("/api/proton-rds-mgmt/v2/users", r.ListUser)

	// 创建或修改用户
	engine.PUT("/api/proton-rds-mgmt/v2/users/:userName", r.CreateOrUpdateUser)

	// 删除用户
	engine.DELETE("/api/proton-rds-mgmt/v2/users/:userName", r.DeleteUser)

	// 修改用户角色
	engine.PUT("/api/proton-rds-mgmt/v2/users/:userName/privileges", r.ModifyUserPrivilege)

	// 修改用户角色
	engine.PATCH("/api/proton-rds-mgmt/v2/users/:userName/privileges", r.ModifyUserPrivilege)

	// 健康检查
	engine.GET("/api/proton-rds-mgmt/v2/health", r.Health)

	// 删除指定备份
	engine.DELETE("/api/proton-rds-mgmt/v2/backups/:id", r.DeleteBackup)

	//查询备份列表
	engine.GET("/api/proton-rds-mgmt/v2/backups", r.ListBackup)

	//创建备份
	engine.POST("/api/proton-rds-mgmt/v2/backups", r.CreateBackup)

	// 获取备份需要的磁盘空间
	engine.GET("/api/proton-rds-mgmt/v2/backup_size", r.GetBackupDataSize)

	// 修改用户SSL连接类型
	engine.PUT("/api/proton-rds-mgmt/v2/users/:userName/ssls", r.ModifyUserSsl)

	// 执行健康检查
	//engine.POST("/api/proton-rds-mgmt/v2/healthcheck", r.CheckDbAvailability)

	// 获取check table结果
	//engine.GET("/api/proton-rds-mgmt/v2/corrupt_table_list", r.GetLastCheckTableResult)

	// 删除check table结果
	//engine.DELETE("/api/proton-rds-mgmt/v2/corrupt_table_list", r.ClearCheckTableResult)

	// 执行check table
	//engine.POST("/api/proton-rds-mgmt/v2/check_table_task", r.GenCheckTableResult)

	// 恢复数据
	engine.POST("/api/proton-rds-mgmt/v2/restorations", r.CreateRecovery)

	// 扩容
	engine.POST("/api/proton-rds-mgmt/v2/scales_hook", r.ScaleHook)

	//查询恢复进度
	engine.GET("/api/proton-rds-mgmt/v2/restorations", r.ListRecovery)
}

// verifyToken 解析Token
func (r *restHandler) verifyToken(c *gin.Context) error {

	// 未配置OAuthON 则认为没有集成OAuth2.0, 不进行验证
	if !r.config.OAuthON {
		return nil
	}

	url := r.config.HydraURL
	if url == "" {
		r.logger.Errorln("HydraURL is empty")
		err := common.NewHTTPError("HydraURL is empty", common.InternalError, nil)
		return err
	}

	// 获取Authorization
	authorization := c.GetHeader("Authorization")
	if authorization == "" {
		r.logger.Errorln("Authorization is empty")
		err := common.NewHTTPError("Authorization is empty", common.BadRequest, nil)
		return err
	}

	// Authorization: Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==
	// POST http://10.2.184.38:<admin-port>/oauth2/token {"token":"QWxhZGRpbjpvcGVuIHNlc2FtZQ=="}
	access_tokens := strings.Split(authorization, " ")
	access_token := access_tokens[len(access_tokens)-1]
	if access_token == "" {
		r.logger.Errorln("Access Token is empty")
		err := common.NewHTTPError("Access Token is empty", common.BadRequest, nil)
		return err
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}
	data := fmt.Sprintf("token=%s", access_token)
	respCode, resp, err := r.httpClient.PostText(url, headers, []byte(data))
	if err != nil || respCode != http.StatusOK {
		r.logger.Errorln(err.Error())
		err := common.NewHTTPError("Authorization Failed", common.Unauthorized, nil)
		return err
	}

	respObj := resp.(map[string]interface{})
	active := respObj["active"].(bool)
	if !active {
		r.logger.Errorln("Authorization Failed")
		err := common.NewHTTPError("Authorization Failed", common.Unauthorized, nil)
		return err
	}
	return nil
}

// verifyAdminKey 解析admin用户名密码
func (r *restHandler) verifyAdminKey(c *gin.Context) (adminName string, adminPwd string, err error) {
	r.logger.Debugln("verifyAdminKey")

	// 获取admin-key
	adminKey := c.GetHeader("admin-key")
	if adminKey == "" {
		r.logger.Errorln("admin-key is empty")
		err = common.NewHTTPError("admin-key is empty", common.BadRequest, nil)
		return
	}

	// base64 解码
	decodeBytes, err := base64.StdEncoding.DecodeString(adminKey)
	if err != nil {
		r.logger.Errorln("admin-key decode failed")
		err = common.NewHTTPError("admin-key decode failed", common.BadRequest, nil)
		return
	}

	// 拆分adminName, adminPwd
	str := string(decodeBytes)
	idx := strings.Index(str, ":")
	if idx <= 0 {
		r.logger.Errorln("admin-key is invalid")
		err = common.NewHTTPError("admin-key is invalid", common.BadRequest, nil)
		return
	}

	adminName = str[0:idx]
	adminPwd = str[idx+1:]

	// 解密
	if r.config.UseEncryption {
		//str := "RkVKRURGUEVQRU9BREVQRU5F"
		//str := "FEJEDFPEPEOADEPENE"
		out, outerr := common.DecryptPwd(adminPwd)
		if outerr != nil {
			r.logger.Errorln("admin-pwd decrypt failed")
			err = common.NewHTTPError("admin-pwd decrypt failed", common.BadRequest, nil)
			return
		}
		adminPwd = out
	}

	return
}

// verifyPassword 解析用户密码
func (r *restHandler) verifyPassword(originPwd string) (password string, err error) {
	r.logger.Debugln("verifyPassword")

	// 密码不能为空
	if originPwd == "" {
		r.logger.Errorln("password is empty")
		err = common.NewHTTPError("password is empty", common.BadRequest, nil)
		return
	}

	// base64 解码
	decodeBytes, err := base64.StdEncoding.DecodeString(originPwd)
	if err != nil {
		r.logger.Errorln("originPwd decode failed")
		err = common.NewHTTPError("originPwd decode failed", common.BadRequest, nil)
		return
	}

	// 解密
	password = string(decodeBytes)
	if r.config.UseEncryption {
		//str := "RkVKRURGUEVQRU9BREVQRU5F"
		//str := "FEJEDFPEPEOADEPENE"
		out, outerr := common.DecryptPwd(password)
		if outerr != nil {
			r.logger.Errorln("password decrypt failed")
			err = common.NewHTTPError("password decrypt failed", common.BadRequest, nil)
			return
		}
		password = out
	}
	return
}

// 验证数据库名
func (r *restHandler) verifyDBNameRule(dbName string) bool {
	// 数据库名, 由小写字母、数字、下划线（_）组成，以字母开头，字母或数字结尾，最多64个字符
	re := `^[a-z]([a-z0-9_]{0,62}[a-z0-9])?$`
	return regexp.MustCompile(re).MatchString(dbName)
}

// 验证用户名
func (r *restHandler) verifyUserNameRule(userName string) bool {
	// 用户名, 由大写字母、小写字母、数字、下划线（_）组成，以字母开头，以字母或数字结尾，最多32个字符
	re := `^[a-zA-Z]([a-zA-Z0-9_]{0,30}[a-zA-Z0-9])?$`
	return regexp.MustCompile(re).MatchString(userName)
}

// 验证密码
func (r *restHandler) verifyPasswordRule(password string) bool {
	// 密码内容必须包含以下三种及以上字符类型：大写字母、小写字母、数字、特殊符号。长度为8～32位。
	// 特殊字符包括：叹号 !、电子邮件符号 @、井号 #、美元符 $、百分号 %、 脱字符 ^、
	//   和号 &、星号 *、左括号 (、右括号 )、下划线 _、加号 +、减号 -、等号 =、句号 .
	re := `^[\w\!\@\#\$\%\^\&\*\(\)\+\-\=\.]{8,32}$`
	if match := regexp.MustCompile(re).MatchString(password); !match {
		return false
	}

	re_upper := `[A-Z]+`
	re_lower := `[a-z]+`
	re_number := `[0-9]+`
	re_symbol := `[\!\@\#\$\%\^\&\*\(\)\_\+\-\=\.]+`

	count := 0
	if match := regexp.MustCompile(re_upper).MatchString(password); match {
		count++
	}
	if match := regexp.MustCompile(re_lower).MatchString(password); match {
		count++
	}
	if match := regexp.MustCompile(re_number).MatchString(password); match {
		count++
	}
	if match := regexp.MustCompile(re_symbol).MatchString(password); match {
		count++
	}
	if count >= 3 {
		return true
	}
	return false
}

// ListDB 查询数据库
func (r *restHandler) ListDB(c *gin.Context) {
	r.logger.Debugln("ListDB")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 获取DB列表
	dbs, err := r.rdsMgmt.ListDB(adminName, adminPwd)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应200
	r.logger.Debugln("ListDB success")
	common.ReplyOK(c, http.StatusOK, dbs)
}

// CreateDB 创建数据库
func (r *restHandler) CreateDB(c *gin.Context) {
	r.logger.Debugln("CreateDB")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 获取请求body
	var jsonV interface{}
	if err = common.GetJSONValue(c, &jsonV); err != nil {
		r.logger.Errorln("GetJSONValue Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查请求参数与文档是否匹配
	objDesc := make(map[string]*common.JSONValueDesc)
	objDesc["charset"] = &common.JSONValueDesc{Kind: reflect.String, Required: true}
	objDesc["collate"] = &common.JSONValueDesc{Kind: reflect.String, Required: false}
	reqParamsDesc := &common.JSONValueDesc{Kind: reflect.Map, Required: true, ValueDesc: objDesc}
	if err = common.CheckJSONValue("body", jsonV, reqParamsDesc); err != nil {
		r.logger.Errorln("CheckJSONValue 'body' Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查参数是否合法
	dbName := c.Param("dbName")

	// 验证数据库名
	if !r.verifyDBNameRule(dbName) {
		r.logger.Errorln("dbName is invalid")
		err = common.NewHTTPError("dbName is invalid", common.BadRequest, nil)
		common.ReplyError(c, err)
		return
	}

	dbinfo := modules.DBInfo{
		DBName: dbName,
	}

	jsonObj := jsonV.(map[string]interface{})
	dbinfo.Charset = jsonObj["charset"].(string)
	if reqParamsDesc.ValueDesc["collate"].Exist {
		dbinfo.Collate = jsonObj["collate"].(string)
	}

	// Charset不能为空
	if dbinfo.Charset == "" {
		r.logger.Errorln("charset is empty")
		err = common.NewHTTPError("charset is empty", common.BadRequest, nil)
		common.ReplyError(c, err)
		return
	}

	// 创建数据库
	err = r.rdsMgmt.CreateDB(adminName, adminPwd, dbinfo)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应201
	r.logger.Debugln("CreateDB success")
	common.ReplyOK(c, http.StatusCreated, nil)
}

// DeleteDB 删除数据库
func (r *restHandler) DeleteDB(c *gin.Context) {
	r.logger.Debugln("DeleteDB")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 检查参数是否合法
	dbName := c.Param("dbName")

	// 删除数据库
	err = r.rdsMgmt.DeleteDB(adminName, adminPwd, dbName)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应204
	r.logger.Debugln("DeleteDB success")
	common.ReplyOK(c, http.StatusNoContent, nil)
}

// ListUser 查询用户
func (r *restHandler) ListUser(c *gin.Context) {
	r.logger.Debugln("ListUser")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 获取用户列表
	users, err := r.rdsMgmt.ListUser(adminName, adminPwd)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应200
	r.logger.Debugln("ListUser success")
	common.ReplyOK(c, http.StatusOK, users)
}

// CreateOrUpdateUser 创建或修改用户
func (r *restHandler) CreateOrUpdateUser(c *gin.Context) {
	r.logger.Debugln("CreateOrUpdateUser")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 获取请求body
	var jsonV interface{}
	if err = common.GetJSONValue(c, &jsonV); err != nil {
		r.logger.Errorln("GetJSONValue Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查请求参数与文档是否匹配
	objDesc := make(map[string]*common.JSONValueDesc)
	objDesc["password"] = &common.JSONValueDesc{Kind: reflect.String, Required: true}
	reqParamsDesc := &common.JSONValueDesc{Kind: reflect.Map, Required: true, ValueDesc: objDesc}
	if err = common.CheckJSONValue("body", jsonV, reqParamsDesc); err != nil {
		r.logger.Errorln("CheckJSONValue 'body' Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查参数是否合法
	userName := c.Param("userName")

	// 验证用户名
	if !r.verifyUserNameRule(userName) {
		r.logger.Errorln("userName is invalid")
		err = common.NewHTTPError("userName is invalid", common.BadRequest, nil)
		common.ReplyError(c, err)
		return
	}

	jsonObj := jsonV.(map[string]interface{})
	password := jsonObj["password"].(string)
	// 解析密码
	password, err = r.verifyPassword(password)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 验证密码
	if !r.verifyPasswordRule(password) {
		r.logger.Errorln("password is invalid")
		err = common.NewHTTPError("password is invalid", common.BadRequest, nil)
		common.ReplyError(c, err)
		return
	}

	// 创建或者修改用户
	err = r.rdsMgmt.CreateOrUpdateUser(adminName, adminPwd, userName, password)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应200
	r.logger.Debugln("CreateOrUpdateUser success")
	common.ReplyOK(c, http.StatusOK, nil)
}

// DeleteUser 删除用户
func (r *restHandler) DeleteUser(c *gin.Context) {
	r.logger.Debugln("DeleteUser")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 检查参数是否合法
	userName := c.Param("userName")

	// 删除用户
	err = r.rdsMgmt.DeleteUser(adminName, adminPwd, userName)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应204
	r.logger.Debugln("DeleteUser success")
	common.ReplyOK(c, http.StatusNoContent, nil)
}

// ModifyUserPrivilege 修改用户角色
func (r *restHandler) ModifyUserPrivilege(c *gin.Context) {
	r.logger.Debugln("ModifyUserPrivilege")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 获取请求body
	var jsonV interface{}
	if err = common.GetJSONValue(c, &jsonV); err != nil {
		r.logger.Errorln("GetJSONValue Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查请求参数与文档是否匹配
	itemObjDesc := make(map[string]*common.JSONValueDesc)
	itemObjDesc["db_name"] = &common.JSONValueDesc{Kind: reflect.String, Required: true}
	itemObjDesc["privilege_type"] = &common.JSONValueDesc{Kind: reflect.String, Required: true}
	objDesc := make(map[string]*common.JSONValueDesc)
	objDesc["element"] = &common.JSONValueDesc{Kind: reflect.Map, Required: true, ValueDesc: itemObjDesc}
	reqParamsDesc := &common.JSONValueDesc{Kind: reflect.Slice, Required: true, ValueDesc: objDesc}
	if err = common.CheckJSONValue("body", jsonV, reqParamsDesc); err != nil {
		r.logger.Errorln("CheckJSONValue 'body' Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查参数是否合法
	userName := c.Param("userName")

	arr := jsonV.([]interface{})
	dbNameMap := map[string]string{}
	privileges := []modules.PrivilegeItem{}
	for _, value := range arr {
		valueMap := value.(map[string]interface{})
		dbName := valueMap["db_name"].(string)
		privilegeType := valueMap["privilege_type"].(string)

		// dbName不能为空
		if dbName == "" {
			r.logger.Errorln("privilege's dbName is empty")
			err = common.NewHTTPError("privilege's dbName is empty", common.BadRequest, nil)
			common.ReplyError(c, err)
			return
		}

		// 权限类型不能为空
		if privilegeType == "" {
			r.logger.Errorln("privilegeType is empty")
			err = common.NewHTTPError("privilegeType is empty", common.BadRequest, nil)
			common.ReplyError(c, err)
			return
		}

		if _, ok := dbNameMap[dbName]; ok {
			r.logger.Errorln("dbName is duplicated")
			err = common.NewHTTPError("dbName is duplicated", common.BadRequest, nil)
			common.ReplyError(c, err)
			return
		}

		dbNameMap[dbName] = dbName
		privileges = append(privileges, modules.PrivilegeItem{
			DBName:        dbName,
			PrivilegeType: privilegeType,
		})
	}

	userInfo := modules.UserInfo{
		UserName:   userName,
		Privileges: privileges,
	}

	resetPriv := false
	if c.Request.Method == "PUT" {
		resetPriv = true
	}

	// 修改用户权限
	err = r.rdsMgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, resetPriv)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应204
	r.logger.Debugln("ModifyUserPrivilege success")
	common.ReplyOK(c, http.StatusNoContent, nil)
}

// Health 健康检查
func (r *restHandler) Health(c *gin.Context) {
	r.logger.Traceln("Health")

	// 响应200
	r.logger.Traceln("Health success")
	common.ReplyOK(c, http.StatusOK, nil)
}

// ModifyUserSsl 用户SSL设置
func (r *restHandler) ModifyUserSsl(c *gin.Context) {
	r.logger.Debugln("ModifyUserSsl")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 获取请求body
	var jsonV interface{}
	if err = common.GetJSONValue(c, &jsonV); err != nil {
		r.logger.Errorln("GetJSONValue Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查请求参数与文档是否匹配
	objDesc := make(map[string]*common.JSONValueDesc)
	objDesc["ssl_type"] = &common.JSONValueDesc{Kind: reflect.String, Required: true}
	reqParamsDesc := &common.JSONValueDesc{Kind: reflect.Map, Required: true, ValueDesc: objDesc}

	if err = common.CheckJSONValue("body", jsonV, reqParamsDesc); err != nil {
		r.logger.Errorln("CheckJSONValue 'body' Failed")
		common.ReplyError(c, err)
		return
	}

	// 检查参数是否合法
	userName := c.Param("userName")

	sslMap := jsonV.(map[string]interface{})
	sslType := sslMap["ssl_type"].(string)

	// 修改用户权限
	err = r.rdsMgmt.ModifyUserSsl(adminName, adminPwd, userName, sslType)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 响应204
	r.logger.Debugln("ModifyUserSsl success")
	common.ReplyOK(c, http.StatusNoContent, nil)
}

// 创建备份
func (r *restHandler) CreateBackup(c *gin.Context) {
	r.logger.Debugln("restHandler.CreateBackup")

	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 检查权限
	err = r.rdsMgmt.CheckPermission(adminName, adminPwd)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	var jsonV interface{}
	if err = common.GetJSONValue(c, &jsonV); err != nil {
		r.logger.Errorln("GetJSONValue Failed")
		common.ReplyError(c, err)
		return
	}

	objDesc := make(map[string]*common.JSONValueDesc)
	objDesc["backup_dir"] = &common.JSONValueDesc{Kind: reflect.String, Required: true}
	reqParamsDesc := &common.JSONValueDesc{Kind: reflect.Map, Required: true, ValueDesc: objDesc}

	if err = common.CheckJSONValue("body", jsonV, reqParamsDesc); err != nil {
		r.logger.Errorln("CheckJSONValue 'body' Failed")
		common.ReplyError(c, err)
		return
	}

	backupInfo, err := r.backupMgmt.CreateBackup(c.GetHeader("admin-key"), jsonV.(map[string]interface{})["backup_dir"].(string))
	if err != nil {
		common.ReplyError(c, err)
		return
	}
	common.ReplyOK(c, http.StatusAccepted, backupInfo)
}

// 删除备份
func (r *restHandler) DeleteBackup(c *gin.Context) {
	r.logger.Debugln("restHandler.DeleteBackup")
	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 检查权限
	err = r.rdsMgmt.CheckPermission(adminName, adminPwd)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	err = r.backupMgmt.DeleteBackup(c.Param("id"))
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	common.ReplyOK(c, http.StatusNoContent, nil)
}

// 获取备份列表
func (r *restHandler) ListBackup(c *gin.Context) {
	r.logger.Debugln("restHandler.ListBackup")
	// 解析Token
	err := r.verifyToken(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 解析获取adminName, adminPwd
	adminName, adminPwd, err := r.verifyAdminKey(c)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	// 检查权限
	err = r.rdsMgmt.CheckPermission(adminName, adminPwd)
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	backupInfos, err := r.backupMgmt.ListBackup()
	if err != nil {
		common.ReplyError(c, err)
		return
	}

	backupId := c.Query("id")
	if backupId == "" {
		common.ReplyOK(c, http.StatusOK, backupInfos)
		return
	}
	for _, backupInfo := range backupInfos {
		if backupInfo.Id == backupId {
			common.ReplyOK(c, http.StatusOK, backupInfo)
			return
		}
	}
	common.ReplyError(c, common.NewHTTPError("Backup not exists", common.BackupNotExist, nil))
}

// 获取备份包大小
func (r *restHandler) GetBackupDataSize(c *gin.Context) {
	size, err := r.backupMgmt.GetBackupDataSize()
	if err != nil {
		common.ReplyError(c, err)
		return
	}
	common.ReplyOK(c, http.StatusOK, size)
}

func (r *restHandler) CreateRecovery(c *gin.Context) {
	var jsonV interface{}
	if err := common.GetJSONValue(c, &jsonV); err != nil {
		r.logger.Errorln("GetJSONValue Failed")
		common.ReplyError(c, err)
		return
	}

	objDesc := make(map[string]*common.JSONValueDesc)
	objDesc["file"] = &common.JSONValueDesc{Kind: reflect.String, Required: true}
	reqParamsDesc := &common.JSONValueDesc{Kind: reflect.Map, Required: true, ValueDesc: objDesc}

	if err := common.CheckJSONValue("body", jsonV, reqParamsDesc); err != nil {
		r.logger.Errorln("CheckJSONValue 'body' Failed")
		common.ReplyError(c, err)
		return
	}

	filepath := jsonV.(map[string]interface{})["file"].(string)
	if filepath == "" {
		common.ReplyError(c, common.NewHTTPError("Require file", common.BadRequest, nil))
	}
	recStatus, err := r.backupMgmt.CreateRecovery(filepath)
	if err != nil {
		common.ReplyError(c, err)
		return
	}
	common.ReplyOK(c, http.StatusAccepted, recStatus)
}

func (r *restHandler) ListRecovery(c *gin.Context) {
	recStatus, err := r.backupMgmt.ListRecovery()
	if err != nil {
		common.ReplyError(c, err)
		return
	}
	common.ReplyOK(c, http.StatusOK, recStatus)
}

func (r *restHandler) ScaleHook(c *gin.Context) {
	err := r.backupMgmt.ScaleHook()
	if err != nil {
		common.ReplyError(c, err)
		return
	}
	common.ReplyOK(c, http.StatusOK, nil)
}
