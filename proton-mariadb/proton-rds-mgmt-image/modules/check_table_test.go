package modules

import (
	"database/sql"
	"errors"
	"proton-rds-mgmt/common"
	"reflect"
	"sync"
	"testing"

	"bou.ke/monkey"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
)

func MockNewCheckTableWorker() *checkTableWorker {
	c := &checkTableWorker{
		logger:        common.NewLogger(),
		corruptTables: []string{},
		lock:          new(sync.Mutex),
		config:        common.NewConfig(),
	}
	return c
}

func Test_checkTableWorker_GetLastCheckResult(t *testing.T) {
	Convey("GetLastCheckResult", t, func() {
		c := MockNewCheckTableWorker()
		c.corruptTables = []string{}
		assert.Equal(t, 0, len(c.GetLastCheckResult()))
	})
}

func Test_checkTableWorker_ResetCheckResult(t *testing.T) {
	Convey("ResetCheckResult", t, func() {
		c := MockNewCheckTableWorker()
		c.corruptTables = []string{"t1"}
		c.ResetCheckResult()
		assert.Equal(t, 0, len(c.GetLastCheckResult()))
	})
}

func Test_checkTableWorker_InitDB(t *testing.T) {
	Convey("InitDB", t, func() {

		c := MockNewCheckTableWorker()

		Convey("SQL Open Failed\n", func() {
			guard := monkey.Patch(sql.Open,
				func(driverName, dataSourceName string) (*sql.DB, error) {
					return nil, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()

			_, err := c.InitDB()
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

			_, err = c.InitDB()
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

			_, err = c.InitDB()
			assert.Equal(t, nil, err)
		})
	})
}

func Test_checkTableWorker_CheckTableOK(t *testing.T) {
	Convey("CheckTableOK", t, func() {
		c := MockNewCheckTableWorker()
		mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer mockdb.Close()
		Convey("Failed to run SQL\n", func() {
			mock.ExpectQuery("CHECK TABLE t1").WillReturnError(&mysql.MySQLError{
				Number:  1044,
				Message: "AccessDenied",
			})
			_, err = c.CheckTableOK("t1", mockdb)
			assert.NotEqual(t, nil, err)
		})
		Convey("Success, table ok\n", func() {
			mock.ExpectQuery("CHECK TABLE t1").WillReturnRows(
				sqlmock.NewRows([]string{"table", "op", "msgType", "msgText"}).AddRow("t1", "check", "status", "ok"),
			)
			ok, err := c.CheckTableOK("t1", mockdb)
			assert.Nil(t, err)
			assert.Equal(t, true, ok)
		})
		Convey("Success, table not ok\n", func() {
			mock.ExpectQuery("CHECK TABLE t1").WillReturnRows(
				sqlmock.NewRows([]string{"table", "op", "msgType", "msgText"}).AddRow("t1", "check", "warning", "....").AddRow("t1", "check", "error", "corrupt"),
			)
			ok, err := c.CheckTableOK("t1", mockdb)
			assert.Nil(t, err)
			assert.Equal(t, false, ok)
		})
	})
}

func Test_checkTableWorker_ListTables(t *testing.T) {
	Convey("ListTables", t, func() {
		c := MockNewCheckTableWorker()
		mockdb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer mockdb.Close()
		Convey("Failed to run SQL\n", func() {
			mock.ExpectQuery("select  concat(table_schema, '.', table_name) " +
				"from information_schema.tables " +
				"where table_schema not in ('mysql', 'information_schema', 'performance_schema')").WillReturnError(&mysql.MySQLError{
				Number:  1044,
				Message: "AccessDenied",
			})
			_, err = c.ListTables(mockdb)
			assert.NotEqual(t, nil, err)
		})
		Convey("Success,\n", func() {
			mock.ExpectQuery("select  concat(table_schema, '.', table_name) " +
				"from information_schema.tables " +
				"where table_schema not in ('mysql', 'information_schema', 'performance_schema')").WillReturnRows(
				sqlmock.NewRows([]string{"table"}).AddRow("t1"),
			)
			tableList, err := c.ListTables(mockdb)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(tableList))
		})
	})
}

func Test_checkTableWorker_Check(t *testing.T) {
	Convey("Check", t, func() {
		c := MockNewCheckTableWorker()
		Convey("Failed to init db\n", func() {
			guard := monkey.PatchInstanceMethod(reflect.TypeOf(c), "InitDB",
				func(*checkTableWorker) (*sql.DB, error) {
					return &sql.DB{}, errors.New("Failed to connect mysql")
				},
			)
			defer guard.Unpatch()
			c.Check()
		})
		Convey("init db ok\n", func() {
			guard := monkey.PatchInstanceMethod(reflect.TypeOf(c), "InitDB",
				func(_ *checkTableWorker) (*sql.DB, error) {
					return &sql.DB{}, nil
				},
			)
			defer guard.Unpatch()
			Convey("ListTables error\n", func() {
				guard := monkey.PatchInstanceMethod(reflect.TypeOf(c), "ListTables",
					func(*checkTableWorker, *sql.DB) ([]string, error) {
						return []string{}, errors.New("Failed to connect mysql")
					},
				)
				defer guard.Unpatch()
				c.Check()
			})
			Convey("ListTables ok\n", func() {
				guard := monkey.PatchInstanceMethod(reflect.TypeOf(c), "ListTables",
					func(*checkTableWorker, *sql.DB) ([]string, error) {
						return []string{"t1", "t2"}, nil
					},
				)
				defer guard.Unpatch()
				Convey("CheckTableOK error\n", func() {
					guard := monkey.PatchInstanceMethod(reflect.TypeOf(c), "CheckTableOK",
						func(_ *checkTableWorker, _ string, _ *sql.DB) (bool, error) {
							return true, errors.New("Failed to connect mysql")
						},
					)
					defer guard.Unpatch()
					c.Check()
					assert.Equal(t, 0, len(c.GetLastCheckResult()))
				})
				Convey("CheckTableOK ok, no corrupt table\n", func() {
					guard := monkey.PatchInstanceMethod(reflect.TypeOf(c), "CheckTableOK",
						func(_ *checkTableWorker, _ string, _ *sql.DB) (bool, error) {
							return true, nil
						},
					)
					defer guard.Unpatch()
					c.Check()
					assert.Equal(t, 0, len(c.GetLastCheckResult()))
				})
				Convey("CheckTableOK ok, exists corrupt table\n", func() {
					guard := monkey.PatchInstanceMethod(reflect.TypeOf(c), "CheckTableOK",
						func(_ *checkTableWorker, _ string, _ *sql.DB) (bool, error) {
							return false, nil
						},
					)
					defer guard.Unpatch()
					c.Check()
					assert.Equal(t, 2, len(c.GetLastCheckResult()))
				})
			})
		})
	})
}
