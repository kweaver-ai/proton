package modules

var DefaultPrivilege = "PROCESS, REPLICATION SLAVE, REPLICATION CLIENT"

var PrivilegeTypes = map[string]string{
	"ReadWrite": "SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, REFERENCES, INDEX, ALTER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, EVENT, TRIGGER",
	"ReadOnly":  "SELECT, LOCK TABLES, SHOW VIEW",
	"DDLOnly":   "CREATE, DROP, INDEX, ALTER, CREATE TEMPORARY TABLES, LOCK TABLES, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE",
	"DMLOnly":   "SELECT, INSERT, UPDATE, DELETE, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, SHOW VIEW, EVENT, TRIGGER",
	"None":      "",
	"All":       "ALL",
}

var SslTypes = map[string]string{
	"Any":  "SSL",
	"None": "NONE",
}

var SystemDefaultDB = map[string]int{
	"information_schema": 1,
	"mysql":              1,
	"performance_schema": 1,
	"sys":                1,
}

var SystemDefaultUser = map[string]int{
	"root":        1,
	"mariabackup": 1,
	"monitor":     1,
	"exporter":    1,
	"mariadb.sys": 1,
}

type DBInfo struct {
	DBName  string `json:"db_name"`
	Charset string `json:"charset"`
	Collate string `json:"collate"`
}

type PrivilegeItem struct {
	DBName        string `json:"db_name"`
	PrivilegeType string `json:"privilege_type"`
}

type UserInfo struct {
	UserName   string          `json:"username"`
	Privileges []PrivilegeItem `json:"privileges"`
	SslType    string          `json:"ssl_type"`
}

type BackupStaus struct {
	BackupTaskCount       int
	BackupDatadirFreeSize int64
}

type BackupInfo struct {
	Id          string `json:"id"`
	PackageName string `json:"package_name"`
	CreateTime  string `json:"create_time"`
	Status      string `json:"status"`
	StorageNode string `json:"storage_node"`
}

var BackupStatusSuccess string = "success"
var BackupStatusFailed string = "failed"
var BackupStatusRunning string = "running"

type BackupInfos []BackupInfo

func (b BackupInfos) Len() int {
	return len(b)
}

//逆序
func (b BackupInfos) Less(i, j int) bool {
	return b[i].Id > b[j].Id
}

func (b BackupInfos) Swap(i, j int) {
	tmp := b[i]
	b[i] = b[j]
	b[j] = tmp
}

type RecStaus struct {
	Status string `json:"status"`
	Msg    string `json:"msg"`
}

var RecStatusSuccess string = "success"
var RecStatusFailed string = "failed"
var RecStatusRunning string = "running"
