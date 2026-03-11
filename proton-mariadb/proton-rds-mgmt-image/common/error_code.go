// Package errors 服务错误码
package common

// SharedLink服务错误码
const (
	// BadRequest 通用错误码，客户端请求错误
	BadRequest = 400012001
	// Unauthorized 通用错误码，未授权或者授权已过期
	Unauthorized = 401012002
	// InternalError 通用错误码，服务端内部错误
	InternalError = 500012003

	// AuthenticationFailed, 身份验证失败
	AuthenticationFailed = 403012004
	// AccessDenied, 权限错误
	AccessDenied = 403012005
	// DatabaseAlreadyExist, 数据库已存在
	DatabaseAlreadyExist = 403012006
	// DatabaseNotExist, 数据库不存在
	DatabaseNotExist = 404012007
	// UserAlreadyExist, 用户已存在
	UserAlreadyExist = 403012008
	// UserNotExist, 用户不存在
	UserNotExist = 404012009
	// 硬盘空间不足
	InsufficientDiskSpace = 403012010
	// 备份不存在
	BackupNotExist = 404012011
	// 备份正在进行
	ExistBackupTask = 403012012
	// 无法恢复
	RecTaskDenied = 403012013
)

var (
	commonErrorI18n = map[int]map[string]string{
		BadRequest: {
			Languages[0]: "参数不合法。",
			Languages[1]: "參數不合法。",
			Languages[2]: "Invalid parameter.",
		},
		Unauthorized: {
			Languages[0]: "授权无效",
			Languages[1]: "授權無效",
			Languages[2]: "Unauthorized",
		},
		InternalError: {
			Languages[0]: "内部错误",
			Languages[1]: "內部錯誤",
			Languages[2]: "Internal Server Error",
		},
		AuthenticationFailed: {
			Languages[0]: "身份验证失败",
			Languages[1]: "身份驗證失敗",
			Languages[2]: "Authentication Failed",
		},
		AccessDenied: {
			Languages[0]: "权限错误",
			Languages[1]: "權限錯誤",
			Languages[2]: "Access Denied",
		},
		DatabaseAlreadyExist: {
			Languages[0]: "数据库已存在",
			Languages[1]: "數據庫已存在",
			Languages[2]: "Database Already Exist",
		},
		DatabaseNotExist: {
			Languages[0]: "数据库不存在",
			Languages[1]: "數據庫不存在",
			Languages[2]: "Database Not Exist",
		},
		UserAlreadyExist: {
			Languages[0]: "用户已存在",
			Languages[1]: "用戶已存在",
			Languages[2]: "User ALready Exist",
		},
		UserNotExist: {
			Languages[0]: "用户不存在",
			Languages[1]: "用戶不存在",
			Languages[2]: "User Not Exist",
		},
		InsufficientDiskSpace: {
			Languages[0]: "硬盘可用空间不足",
			Languages[1]: "硬盤可用空間不足",
			Languages[2]: "Insufficient Disk Space",
		},
		BackupNotExist: {
			Languages[0]: "备份不存在",
			Languages[1]: "備份不存在",
			Languages[2]: "Backup Not Exist",
		},
		ExistBackupTask: {
			Languages[0]: "存在进行中的备份任务",
			Languages[1]: "存在進行中的備份任務",
			Languages[2]: "Exist Backup Task",
		},
		RecTaskDenied: {
			Languages[0]: "无法创建恢复任务",
			Languages[1]: "无法创建恢复任务",
			Languages[2]: "Restorations Task Denied",
		},
	}
)

func init() {
	Register(commonErrorI18n)
}
