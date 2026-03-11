package modules

import (
	"database/sql"
	"errors"
	"fmt"
	"proton-rds-mgmt/common"
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
)

func MockNewRDSMgmt() RDSMgmt {
	r := &rdsMgmt{
		logger: common.NewLogger(),
	}
	r.SetConfig(common.NewConfig())
	return r
}

func Test_RDSMgmt_InitDB(t *testing.T) {

	Convey("InitDB", t, func() {

		adminName := "root"
		adminPwd := "fakepassword"

		mgmt := MockNewRDSMgmt()

		Convey("SQL Open Failed\n", func() {
			guard := monkey.Patch(sql.Open,
				func(driverName, dataSourceName string) (*sql.DB, error) {
					return nil, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			_, err := mgmt.InitDB(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Failed to connect mysql\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectPing().WillReturnError(&mysql.MySQLError{
				Number:  1045,
				Message: "AuthenticationFailed",
			})

			guard := monkey.Patch(sql.Open,
				func(driverName, dataSourceName string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			_, err = mgmt.InitDB(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectPing().WillReturnError(nil)

			guard := monkey.Patch(sql.Open,
				func(driverName, dataSourceName string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			_, err = mgmt.InitDB(adminName, adminPwd)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_RDSMgmt_CheckDBExist(t *testing.T) {

	Convey("CheckDBExist", t, func() {

		dbName := "test"

		mgmt := MockNewRDSMgmt()

		Convey("Failed to run SQL\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectQuery("SELECT COUNT(1) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?").WithArgs(dbName).WillReturnError(errors.New("failed to run SQL"))

			_, err = mgmt.CheckDBExist(mockdb, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success, db not exist\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectQuery("SELECT COUNT(1) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?").WithArgs(dbName).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			exist, _ := mgmt.CheckDBExist(mockdb, dbName)
			assert.Equal(t, false, exist)
		})

		Convey("Success, db already exist\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectQuery("SELECT COUNT(1) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?").WithArgs(dbName).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			exist, _ := mgmt.CheckDBExist(mockdb, dbName)
			assert.Equal(t, true, exist)
		})
	})
}

func Test_RDSMgmt_CheckUserExist(t *testing.T) {

	Convey("CheckUserExist", t, func() {

		userName := "test"

		mgmt := MockNewRDSMgmt()

		Convey("Failed to run SQL, AccessDenied, 1044\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectQuery("SELECT COUNT(1) FROM mysql.user WHERE User=? and Host=?").WithArgs(userName, "%").WillReturnError(&mysql.MySQLError{
				Number:  1044,
				Message: "AccessDenied",
			})

			_, err = mgmt.CheckUserExist(mockdb, userName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Failed to run SQL, AccessDenied, 1142\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectQuery("SELECT COUNT(1) FROM mysql.user WHERE User=? and Host=?").WithArgs(userName, "%").WillReturnError(&mysql.MySQLError{
				Number:  1142,
				Message: "AccessDenied",
			})

			_, err = mgmt.CheckUserExist(mockdb, userName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success, user not exist\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectQuery("SELECT COUNT(1) FROM mysql.user WHERE User=? and Host=?").WithArgs(userName, "%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			exist, _ := mgmt.CheckUserExist(mockdb, userName)
			assert.Equal(t, false, exist)
		})

		Convey("Success, user already exist\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			mock.ExpectQuery("SELECT COUNT(1) FROM mysql.user WHERE User=? and Host=?").WithArgs(userName, "%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			exist, _ := mgmt.CheckUserExist(mockdb, userName)
			assert.Equal(t, true, exist)
		})
	})
}

func Test_RDSMgmt_ListDB(t *testing.T) {

	Convey("ListDB", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"

		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			_, err = mgmt.ListDB(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Query Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			mock.ExpectQuery("SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA").WillReturnError(errors.New("failed to run SQL"))

			_, err = mgmt.ListDB(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Scan Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName", "charset"})
			rows.AddRow("test", "utf8mb4")
			mock.ExpectQuery("SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA").WillReturnRows(rows)

			_, err = mgmt.ListDB(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName", "charset", "collate"})
			rows.AddRow("mysql", "utf8mb4", "utf8mb4_unicode_ci")
			rows.AddRow("test", "utf8mb4", "utf8mb4_unicode_ci")
			mock.ExpectQuery("SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA").WillReturnRows(rows)

			_, err = mgmt.ListDB(adminName, adminPwd)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_RDSMgmt_CreateDB(t *testing.T) {

	Convey("CreateDB", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"
		dbInfo := DBInfo{
			DBName:  "test",
			Charset: "utf8mb4",
			Collate: "",
		}

		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			err = mgmt.CreateDB(adminName, adminPwd, dbInfo)
			assert.NotEqual(t, nil, err)
		})

		Convey("DB is system database\n", func() {
			errDBInfo := DBInfo{
				DBName:  "mysql",
				Charset: "utf8mb4",
				Collate: "",
			}
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			err = mgmt.CreateDB(adminName, adminPwd, errDBInfo)
			assert.NotEqual(t, nil, err)
		})

		Convey("CheckDBExist failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, errors.New("CheckDBExist failed")
				},
			)
			defer guard2.Unpatch()

			err = mgmt.CreateDB(adminName, adminPwd, dbInfo)
			assert.NotEqual(t, nil, err)
		})

		Convey("DB Already exist\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			err = mgmt.CreateDB(adminName, adminPwd, dbInfo)
			assert.NotEqual(t, nil, err)
		})

		Convey("Exec Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET %s", dbInfo.DBName, dbInfo.Charset)
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.CreateDB(adminName, adminPwd, dbInfo)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET %s", dbInfo.DBName, dbInfo.Charset)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.CreateDB(adminName, adminPwd, dbInfo)
			assert.Equal(t, nil, err)
		})

		Convey("Success, Collate is not empty\n", func() {
			dbInfo.Collate = "utf8mb4_unicode_ci"

			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET %s COLLATE %s", dbInfo.DBName, dbInfo.Charset, dbInfo.Collate)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.CreateDB(adminName, adminPwd, dbInfo)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_RDSMgmt_DeleteDB(t *testing.T) {

	Convey("DeleteDB", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"
		dbName := "test"

		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("DB is system database\n", func() {
			errDBName := "mysql"
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			err = mgmt.DeleteDB(adminName, adminPwd, errDBName)
			assert.NotEqual(t, nil, err)
		})

		Convey("CheckDBExist Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, errors.New("CheckDBExist Failed")
				},
			)
			defer guard2.Unpatch()

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("DB is not exist\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Exec Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("DROP DATABASE `%s`", dbName)
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Select Priv Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("DROP DATABASE `%s`", dbName)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			sql = fmt.Sprintf("SELECT User, Host FROM mysql.db WHERE Db = %s", dbName)
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Scan Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("DROP DATABASE `%s`", dbName)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			rows := sqlmock.NewRows([]string{"User"})
			rows.AddRow("test")
			mock.ExpectQuery("SELECT User, Host FROM mysql.db WHERE Db = ?").WithArgs(dbName).WillReturnRows(rows)

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Delete Priv Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("DROP DATABASE `%s`", dbName)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			rows := sqlmock.NewRows([]string{"User", "Host"})
			rows.AddRow("test", "127.0.0.1")
			mock.ExpectQuery("SELECT User, Host FROM mysql.db WHERE Db = ?").WithArgs(dbName).WillReturnRows(rows)

			sql = fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `test`@'127.0.0.1'", dbName)
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("DROP DATABASE `%s`", dbName)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			rows := sqlmock.NewRows([]string{"User", "Host"})
			rows.AddRow("test", "127.0.0.1")
			mock.ExpectQuery("SELECT User, Host FROM mysql.db WHERE Db = ?").WithArgs(dbName).WillReturnRows(rows)

			sql = fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `test`@'127.0.0.1'", dbName)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.DeleteDB(adminName, adminPwd, dbName)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_RDSMgmt_ListUser(t *testing.T) {

	Convey("ListUser", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"

		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			_, err = mgmt.ListUser(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Query Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			mock.ExpectQuery("select User, ssl_type, null as Db, References_priv, Insert_priv, Create_priv, Select_priv, Super_priv, null as Delete_history_priv  from mysql.user where HOST=?"+
				" union select User, null as ssl_type, Db, References_priv, Insert_priv, Create_priv, Select_priv, null as Super_priv, Delete_history_priv from mysql.db where host=?").WithArgs("%", "%").WillReturnError(errors.New("failed to run SQL"))

			_, err = mgmt.ListUser(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Scan Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName", "charset"})
			rows.AddRow("test", "utf8mb4")

			mock.ExpectQuery("select User, ssl_type, null as Db, References_priv, Insert_priv, Create_priv, Select_priv, Super_priv, null as Delete_history_priv  from mysql.user where HOST=?"+
				" union select User, null as ssl_type, Db, References_priv, Insert_priv, Create_priv, Select_priv, null as Super_priv, Delete_history_priv from mysql.db where host=?").WithArgs("%", "%").WillReturnRows(rows)

			_, err = mgmt.ListUser(adminName, adminPwd)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()
			rows := sqlmock.NewRows([]string{"user", "ssl_type", "db", "referencesPriv", "insertPriv", "createPriv", "selectPriv", "superPriv", "deleteHistoryPriv"})
			rows.AddRow("root", "ANY", nil, "N", "N", "N", "N", "N", nil)
			rows.AddRow("test1", "ANY", nil, "N", "N", "N", "N", "N", nil)
			rows.AddRow("test1", "ANY", nil, "N", "N", "N", "N", "Y", nil)
			rows.AddRow("test1", "ANY", "ecms", "N", "N", "N", "N", "N", nil)
			rows.AddRow("test1", "ANY", "ecms", "N", "N", "N", "N", "Y", nil)
			rows.AddRow("test1", "ANY", "ecms", "N", "N", "N", "N", "N", "Y")
			rows.AddRow("test1", "ANY", "ecms", "Y", "N", "N", "N", "N", nil)
			rows.AddRow("test1", "ANY", "ecms", "N", "Y", "N", "N", "N", nil)
			rows.AddRow("test1", "ANY", "ecms", "N", "N", "Y", "N", "N", nil)
			rows.AddRow("test1", "ANY", "ecms", "N", "N", "N", "Y", "N", nil)
			rows.AddRow("test1", nil, "ecms", "N", "N", "N", "Y", "N", nil)

			mock.ExpectQuery("select User, ssl_type, null as Db, References_priv, Insert_priv, Create_priv, Select_priv, Super_priv, null as Delete_history_priv  from mysql.user where HOST=?"+
				" union select User, null as ssl_type, Db, References_priv, Insert_priv, Create_priv, Select_priv, null as Super_priv, Delete_history_priv from mysql.db where host=?").WithArgs("%", "%").WillReturnRows(rows)

			_, err = mgmt.ListUser(adminName, adminPwd)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_RDSMgmt_CreateOrUpdateUser(t *testing.T) {

	Convey("CreateOrUpdateUser", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"
		userName := "test"
		password := "fakepassword"

		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			err = mgmt.CreateOrUpdateUser(adminName, adminPwd, userName, password)
			assert.NotEqual(t, nil, err)
		})

		Convey("User is system user\n", func() {
			errUserName := "root"
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			sql := fmt.Sprintf("CREATE USER `%s`@'%s' IDENTIFIED BY '%s'", userName, "%", password)
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.CreateOrUpdateUser(adminName, adminPwd, errUserName, password)
			assert.NotEqual(t, nil, err)
		})

		Convey("CheckUserExist Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, errors.New("CheckUserExist Failed")
				},
			)
			defer guard2.Unpatch()

			err = mgmt.CreateOrUpdateUser(adminName, adminPwd, userName, password)
			assert.NotEqual(t, nil, err)
		})

		Convey("Exec Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("CREATE USER `%s`@'%s' IDENTIFIED BY '%s'", userName, "%", password)
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.CreateOrUpdateUser(adminName, adminPwd, userName, password)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("CREATE USER `%s`@'%s' IDENTIFIED BY '%s'", userName, "%", password)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.CreateOrUpdateUser(adminName, adminPwd, userName, password)
			assert.Equal(t, nil, err)
		})

		Convey("Success, user already exist\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("SET PASSWORD FOR `%s`@'%s' = PASSWORD('%s')", userName, "%", password)
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.CreateOrUpdateUser(adminName, adminPwd, userName, password)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_RDSMgmt_DeleteUser(t *testing.T) {

	Convey("DeleteUser", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"
		userName := "test"

		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			err = mgmt.DeleteUser(adminName, adminPwd, userName)
			assert.NotEqual(t, nil, err)
		})

		Convey("User is admin user\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			err = mgmt.DeleteUser("admin", adminPwd, "admin")
			assert.NotEqual(t, nil, err)
		})

		Convey("User is system user\n", func() {
			errUserName := "root"
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			err = mgmt.DeleteUser("admin", adminPwd, errUserName)
			assert.NotEqual(t, nil, err)
		})

		Convey("CheckUserExist Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, errors.New("CheckUserExist Failed")
				},
			)
			defer guard2.Unpatch()

			err = mgmt.DeleteUser(adminName, adminPwd, userName)
			assert.NotEqual(t, nil, err)
		})

		Convey("User is not exist\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			err = mgmt.DeleteUser(adminName, adminPwd, userName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Exec Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("DROP USER `%s`@'%s'", userName, "%")
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.DeleteUser(adminName, adminPwd, userName)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			sql := fmt.Sprintf("DROP USER `%s`@'%s'", userName, "%")
			mock.ExpectExec(sql).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.DeleteUser(adminName, adminPwd, userName)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_RDSMgmt_ModifyUserPrivilege(t *testing.T) {

	Convey("ModifyUserPrivilege", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"
		userInfo := UserInfo{
			UserName: "test",
			Privileges: []PrivilegeItem{
				{
					DBName:        "test",
					PrivilegeType: "ReadOnly",
				},
			},
		}

		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("User is admin user\n", func() {
			errUserInfo := UserInfo{
				UserName: "admin",
				Privileges: []PrivilegeItem{
					{
						DBName:        "test",
						PrivilegeType: "ReadOnly",
					},
				},
			}
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			err = mgmt.ModifyUserPrivilege("admin", adminPwd, errUserInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("User is system user\n", func() {
			errUserInfo := UserInfo{
				UserName: "root",
				Privileges: []PrivilegeItem{
					{
						DBName:        "test",
						PrivilegeType: "ReadOnly",
					},
				},
			}
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			err = mgmt.ModifyUserPrivilege("admin", adminPwd, errUserInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("CheckUserExist Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, errors.New("CheckUserExist Failed")
				},
			)
			defer guard2.Unpatch()

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("User is not exist\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("PrivilegeType is invalid\n", func() {
			errUserInfo := UserInfo{
				UserName: "test",
				Privileges: []PrivilegeItem{
					{
						DBName:        "test",
						PrivilegeType: "haha",
					},
				},
			}

			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, errUserInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("CheckDBExist Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, errors.New("CheckDBExist Failed")
				},
			)
			defer guard3.Unpatch()

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("DB is not exist\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard3.Unpatch()

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("Revoke all privilege failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			mock.ExpectExec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%")).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("Query old privilege failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			mock.ExpectExec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%")).WillReturnResult(sqlmock.NewResult(1, 1))

			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("Scan Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName", "userName"})
			rows.AddRow("test", "test")

			mock.ExpectExec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%")).WillReturnResult(sqlmock.NewResult(1, 1))

			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("Revoke Failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName"})
			rows.AddRow("test1")

			mock.ExpectExec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%")).WillReturnResult(sqlmock.NewResult(1, 1))

			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			sql := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", "test1", userInfo.UserName, "%")
			mock.ExpectExec(sql).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("Grant failed\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName"})
			rows.AddRow("test1")

			mock.ExpectExec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%")).WillReturnResult(sqlmock.NewResult(1, 1))

			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			sql1 := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", "test1", userInfo.UserName, "%")
			mock.ExpectExec(sql1).WillReturnResult(sqlmock.NewResult(1, 1))

			sql2 := fmt.Sprintf("GRANT %s ON `%s`.* TO `%s`@'%s'", PrivilegeTypes["ReadOnly"], "test", userInfo.UserName, "%")
			mock.ExpectExec(sql2).WillReturnError(errors.New("failed to run SQL"))

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName"})
			rows.AddRow("test1")

			mock.ExpectExec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%")).WillReturnResult(sqlmock.NewResult(1, 1))

			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			sql1 := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", "test1", userInfo.UserName, "%")
			mock.ExpectExec(sql1).WillReturnResult(sqlmock.NewResult(1, 1))

			sql2 := fmt.Sprintf("GRANT %s ON `%s`.* TO `%s`@'%s'", PrivilegeTypes["ReadOnly"], "test", userInfo.UserName, "%")
			mock.ExpectExec(sql2).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.Equal(t, nil, err)
		})

		Convey("Set Global Priv, Success\n", func() {

			userInfo := UserInfo{
				UserName: "test",
				Privileges: []PrivilegeItem{
					{
						DBName:        "*",
						PrivilegeType: "ReadOnly",
					},
				},
			}

			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			mock.ExpectExec(fmt.Sprintf("REVOKE ALL ON *.* FROM `%s`@'%s'", userInfo.UserName, "%")).WillReturnResult(sqlmock.NewResult(1, 1))

			rows := sqlmock.NewRows([]string{"dbName"})
			rows.AddRow("test1")
			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			sql1 := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", "test1", userInfo.UserName, "%")
			mock.ExpectExec(sql1).WillReturnResult(sqlmock.NewResult(1, 1))

			sql2 := fmt.Sprintf("GRANT %s ON *.* TO `%s`@'%s'", PrivilegeTypes["ReadOnly"], userInfo.UserName, "%")
			mock.ExpectExec(sql2).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, true)
			assert.Equal(t, nil, err)
		})

		Convey("Patch, add priv, Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName"})
			rows.AddRow("test2")
			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			sql2 := fmt.Sprintf("GRANT %s ON `%s`.* TO `%s`@'%s'", PrivilegeTypes["ReadOnly"], "test", userInfo.UserName, "%")
			mock.ExpectExec(sql2).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, false)
			assert.Equal(t, nil, err)
		})

		Convey("Patch, del priv, Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName"})
			rows.AddRow("test")
			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			sql1 := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", "test", userInfo.UserName, "%")
			mock.ExpectExec(sql1).WillReturnResult(sqlmock.NewResult(1, 1))

			userInfo.Privileges[0].PrivilegeType = "None"
			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, false)
			assert.Equal(t, nil, err)
		})

		Convey("Patch, update priv, Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			rows := sqlmock.NewRows([]string{"dbName"})
			rows.AddRow("test")
			mock.ExpectQuery("SELECT Db FROM mysql.db WHERE User=? AND Host = ?").WithArgs(userInfo.UserName, "%").WillReturnRows(rows)

			sql1 := fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`@'%s'", "test", userInfo.UserName, "%")
			mock.ExpectExec(sql1).WillReturnResult(sqlmock.NewResult(1, 1))

			sql2 := fmt.Sprintf("GRANT %s ON `%s`.* TO `%s`@'%s'", PrivilegeTypes["ReadOnly"], "test", userInfo.UserName, "%")
			mock.ExpectExec(sql2).WillReturnResult(sqlmock.NewResult(1, 1))

			userInfo.Privileges[0].PrivilegeType = "ReadOnly"

			err = mgmt.ModifyUserPrivilege(adminName, adminPwd, userInfo, false)
			assert.Equal(t, nil, err)
		})
	})
}

func Test_rdsMgmt_ModifyUserSsl(t *testing.T) {

	Convey("ModifyUserSsl", t, func() {
		adminName := "root"
		adminPwd := "fakepassword"
		mgmt := MockNewRDSMgmt()

		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			err = mgmt.ModifyUserSsl(adminName, adminPwd, "test", "Any")
			assert.NotEqual(t, nil, err)
		})

		Convey("User is system user\n", func() {

			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			err = mgmt.ModifyUserSsl(adminName, adminPwd, "root", "Any")
			assert.NotEqual(t, nil, err)
		})

		Convey("CheckUserExist Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, errors.New("CheckUserExist Failed")
				},
			)
			defer guard2.Unpatch()

			err = mgmt.ModifyUserSsl(adminName, adminPwd, "test", "Any")
			assert.NotEqual(t, nil, err)
		})

		Convey("User is not exist\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return false, nil
				},
			)
			defer guard2.Unpatch()

			err = mgmt.ModifyUserSsl(adminName, adminPwd, "test", "Any")
			assert.NotEqual(t, nil, err)
		})

		Convey("sslType is invalid\n", func() {

			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			err = mgmt.ModifyUserSsl(adminName, adminPwd, "test", "xxx")
			assert.NotEqual(t, nil, err)
		})

		Convey("Success\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard1.Unpatch()

			guard2 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckUserExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard2.Unpatch()

			guard3 := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "CheckDBExist",
				func(_ *rdsMgmt, db *sql.DB, dbName string) (bool, error) {
					return true, nil
				},
			)
			defer guard3.Unpatch()

			sql1 := fmt.Sprintf("ALTER USER `%s`@`%s` REQUIRE %s", "test", "%", SslTypes["Any"])
			mock.ExpectExec(sql1).WillReturnResult(sqlmock.NewResult(1, 1))

			err = mgmt.ModifyUserSsl(adminName, adminPwd, "test", "Any")
			assert.Equal(t, nil, err)
		})
	})

}

func Test_rdsMgmt_CheckDbAvailability(t *testing.T) {
	Convey("CheckDbAvailability\n", t, func() {
		mgmt := MockNewRDSMgmt()
		Convey("replica 1, get wsrep_on failed", func() {
			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "GetDBVariables",
				func(_ *rdsMgmt, variableType string, variable string) (string, error) {
					return "OFF", errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()
			err := mgmt.CheckDbAvailability()
			assert.NotEqual(t, nil, err)
		})
		Convey("replica 1, ready", func() {
			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "GetDBVariables",
				func(_ *rdsMgmt, variableType string, variable string) (string, error) {
					return "OFF", nil
				},
			)
			defer guard.Unpatch()
			err := mgmt.CheckDbAvailability()
			assert.Equal(t, nil, err)
		})

		Convey("replica 3, get wsrep_ready failed", func() {
			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "GetDBVariables",
				func(_ *rdsMgmt, variableType string, variable string) (string, error) {
					if variable == "wsrep_on" {
						return "ON", nil
					}
					if variable == "wsrep_ready" {
						return "ON", errors.New("Failed to connect mysql")
					}
					return "", nil
				},
			)
			defer guard.Unpatch()
			err := mgmt.CheckDbAvailability()
			assert.NotEqual(t, nil, err)
		})

		Convey("replica 3, ready", func() {
			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "GetDBVariables",
				func(_ *rdsMgmt, variableType string, variable string) (string, error) {
					if variable == "wsrep_on" {
						return "ON", nil
					}
					if variable == "wsrep_ready" {
						return "ON", nil
					}
					return "", nil
				},
			)
			defer guard.Unpatch()
			err := mgmt.CheckDbAvailability()
			assert.Equal(t, nil, err)
		})

		Convey("replica 3, not ready", func() {
			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "GetDBVariables",
				func(_ *rdsMgmt, variableType string, variable string) (string, error) {
					if variable == "wsrep_on" {
						return "ON", nil
					}
					if variable == "wsrep_ready" {
						return "OFF", nil
					}
					return "not sync", nil
				},
			)
			defer guard.Unpatch()
			err := mgmt.CheckDbAvailability()
			assert.NotEqual(t, nil, err)
		})

	})
}

func Test_rdsMgmt_GetDBVariables(t *testing.T) {
	Convey("GetDBVariables\n", t, func() {
		mgmt := MockNewRDSMgmt()
		Convey("InitDB Failed\n", func() {
			mockdb, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			_, err = mgmt.GetDBVariables("", "")
			assert.NotEqual(t, nil, err)
		})
		Convey("InitDB OK\n", func() {
			mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer mockdb.Close()

			guard := monkey.PatchInstanceMethod(reflect.TypeOf(mgmt), "InitDB",
				func(_ *rdsMgmt, adminName string, adminPwd string) (*sql.DB, error) {
					return mockdb, nil
				},
			)
			defer guard.Unpatch()

			Convey("Query failed\n", func() {
				mock.ExpectQuery("show global status like 'wsrep_ready'").WillReturnError(errors.New("failed to run SQL"))
				_, err = mgmt.GetDBVariables("status", "wsrep_ready")
				assert.NotEqual(t, nil, err)
			})

			Convey("Query ok\n", func() {
				mock.ExpectQuery("show global status like 'wsrep_ready'").WillReturnRows(sqlmock.NewRows([]string{"variable_name", "value"}).AddRow("", ""))
				_, err = mgmt.GetDBVariables("status", "wsrep_ready")
				assert.Equal(t, nil, err)
			})
		})
	})
}
