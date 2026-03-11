package server

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"proton-rds-mgmt/common"
	cmock "proton-rds-mgmt/common/mock"
	"proton-rds-mgmt/modules"
	dmock "proton-rds-mgmt/modules/mock"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	jsoniter "github.com/json-iterator/go"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
)

func setGinMode() func() {
	old := gin.Mode()
	gin.SetMode(gin.TestMode)
	return func() {
		gin.SetMode(old)
	}
}

func newRESTHandler(rMgmt modules.RDSMgmt, hClient common.HTTPClient, bMgmt modules.BackupMgmt, checkTableWorker modules.CheckTableWorker) *restHandler {
	r := &restHandler{
		rdsMgmt:        rMgmt,
		httpClient:     hClient,
		logger:         common.NewLogger(),
		backupMgmt:     bMgmt,
		checkTableMgmt: checkTableWorker,
	}
	r.SetConfig(common.NewConfig())
	return r
}

func CheckToken(t *testing.T, engine *gin.Engine, hClient *cmock.MockHTTPClient, r *restHandler, target string, method string) {
	r.config.OAuthON = true
	r.config.HydraURL = "http://127.0.0.1:30001/oauth2/introspect"

	Convey("HydraURL is empty\n", func() {
		origHydraURL := r.config.HydraURL
		r.config.HydraURL = ""
		req := httptest.NewRequest(method, target, nil)

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
		r.config.HydraURL = origHydraURL
	})

	Convey("Authorization is empty\n", func() {
		req := httptest.NewRequest(method, target, nil)

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Access Token is empty\n", func() {
		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("Authorization", "Basic ")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Post Token failed\n", func() {
		hClient.EXPECT().PostText(gomock.Any(), gomock.Any(), gomock.Any()).Return(http.StatusInternalServerError, nil, errors.New("xxx"))

		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("Authorization", "Basic xxx")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusUnauthorized, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Active False\n", func() {
		resp := map[string]interface{}{
			"active": false,
		}
		hClient.EXPECT().PostText(gomock.Any(), gomock.Any(), gomock.Any()).Return(http.StatusOK, resp, nil)

		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("Authorization", "Basic xxx")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusUnauthorized, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	r.config.OAuthON = false
}

func CheckAdminKey(t *testing.T, engine *gin.Engine, r *restHandler, target string, method string) {
	r.config.UseEncryption = false

	Convey("Admin-key is empty\n", func() {
		reqParamByte, _ := jsoniter.Marshal(map[string]int{"error": 1})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Admin-key decode failed\n", func() {
		reqParamByte, _ := jsoniter.Marshal(map[string]int{"error": 1})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=scvzxds")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Admin-key is invalid\n", func() {
		reqParamByte, _ := jsoniter.Marshal(map[string]int{"error": 1})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "cm9vdAo=")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Admin-pwd decrypt failed\n", func() {
		r.config.UseEncryption = true

		reqParamByte, _ := jsoniter.Marshal(map[string]int{"error": 1})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "cm9vdDpGRUpFREZQRVBFT0FERVBFTg==")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
		r.config.UseEncryption = false
	})
}

func CheckJSONValue(t *testing.T, engine *gin.Engine, target string, method string) {
	Convey("GetJSONValue Failed\n", func() {
		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("admin-key", "cm9vdDpGRUpFREZQRVBFT0FERVBFTg==")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("CheckJSONValue 'body' Failed\n", func() {
		reqParamByte, _ := jsoniter.Marshal(map[string]int{"error": 1})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "cm9vdDpGRUpFREZQRVBFT0FERVBFTg==")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})
}

func CheckPassword(t *testing.T, engine *gin.Engine, r *restHandler, target string, method string) {
	r.config.UseEncryption = false

	Convey("Password is empty\n", func() {
		reqParamByte, _ := jsoniter.Marshal(map[string]string{"password": ""})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Password decode failed\n", func() {
		reqParamByte, _ := jsoniter.Marshal(map[string]string{"password": "c=afc"})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
	})

	Convey("Password decrypt failed\n", func() {
		r.config.UseEncryption = true

		reqParamByte, _ := jsoniter.Marshal(map[string]string{"password": "a2Vu"})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "dGVzdDpGRUpFREZQRVBFT0FERVBFTkU=")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}
		r.config.UseEncryption = false
	})
}

func Test_restHandler_verifyDBNameRule(t *testing.T) {

	Convey("Verify DBName Rule\n", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()

		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()

		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()

		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)

		Convey("以数字开始\n", func() {
			match := handler.verifyDBNameRule("12345")
			assert.Equal(t, false, match)
		})
		Convey("以大写字母开始\n", func() {
			match := handler.verifyDBNameRule("A12345")
			assert.Equal(t, false, match)
		})
		Convey("包含大写字母\n", func() {
			match := handler.verifyDBNameRule("a123A45")
			assert.Equal(t, false, match)
		})
		Convey("以大写字母结尾\n", func() {
			match := handler.verifyDBNameRule("a12345A")
			assert.Equal(t, false, match)
		})
		Convey("以下划线开始\n", func() {
			match := handler.verifyDBNameRule("_12345")
			assert.Equal(t, false, match)
		})
		Convey("以下划线结尾\n", func() {
			match := handler.verifyDBNameRule("a2345_")
			assert.Equal(t, false, match)
		})
		Convey("包含特殊字符\n", func() {
			match := handler.verifyDBNameRule("a!@#$%^&*()+-=. ")
			assert.Equal(t, false, match)
		})
		Convey("包含中文\n", func() {
			match := handler.verifyDBNameRule("a你好")
			assert.Equal(t, false, match)
		})
		Convey("空串\n", func() {
			match := handler.verifyDBNameRule("")
			assert.Equal(t, false, match)
		})
		Convey("长度超过64\n", func() {
			match := handler.verifyDBNameRule("abcdefghijklmnopqrstuvwxyz1234567890_abcdefghijklmnopqrstuvwxyz1234567890")
			assert.Equal(t, false, match)
		})
	})
}

func Test_restHandler_verifyUserNameRule(t *testing.T) {

	Convey("Verify UserName Rule\n", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)

		Convey("以数字开始\n", func() {
			match := handler.verifyUserNameRule("12345")
			assert.Equal(t, false, match)
		})
		Convey("以下划线开始\n", func() {
			match := handler.verifyUserNameRule("_12345")
			assert.Equal(t, false, match)
		})
		Convey("以下划线结尾\n", func() {
			match := handler.verifyUserNameRule("a2345_")
			assert.Equal(t, false, match)
		})
		Convey("包含特殊字符\n", func() {
			match := handler.verifyUserNameRule("a!@#$%^&*()+-=. ")
			assert.Equal(t, false, match)
		})
		Convey("包含中文\n", func() {
			match := handler.verifyUserNameRule("a你好")
			assert.Equal(t, false, match)
		})
		Convey("空串\n", func() {
			match := handler.verifyUserNameRule("")
			assert.Equal(t, false, match)
		})
		Convey("长度超过32\n", func() {
			match := handler.verifyUserNameRule("abcdefghijklmnopqrstuvwxyz1234567890")
			assert.Equal(t, false, match)
		})
	})
}

func Test_restHandler_verifyPasswordRule(t *testing.T) {

	Convey("Verify Password Rule\n", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)

		Convey("空串\n", func() {
			match := handler.verifyPasswordRule("")
			assert.Equal(t, false, match)
		})
		Convey("长度不足8位\n", func() {
			match := handler.verifyPasswordRule("aA12345")
			assert.Equal(t, false, match)
		})
		Convey("长度超过32位\n", func() {
			match := handler.verifyPasswordRule("abcdefghijklmnopqrstuvwxyz1234567890")
			assert.Equal(t, false, match)
		})
		Convey("包含非法特殊字符\n", func() {
			match := handler.verifyPasswordRule("fakepassword<>")
			assert.Equal(t, false, match)
		})
		Convey("包含中文\n", func() {
			match := handler.verifyPasswordRule("fakepassword你好")
			assert.Equal(t, false, match)
		})
		Convey("只有数字\n", func() {
			match := handler.verifyPasswordRule("1234567890")
			assert.Equal(t, false, match)
		})
		Convey("只有小写字母\n", func() {
			match := handler.verifyPasswordRule("abcdefghijklmn")
			assert.Equal(t, false, match)
		})
		Convey("只有大写字母\n", func() {
			match := handler.verifyPasswordRule("ABCDEFGHIJKLMN")
			assert.Equal(t, false, match)
		})
		Convey("只有特殊字符\n", func() {
			match := handler.verifyPasswordRule("!@#$%^&*()+-=")
			assert.Equal(t, false, match)
		})
		Convey("只有数字和小写字母\n", func() {
			match := handler.verifyPasswordRule("1234567890abc")
			assert.Equal(t, false, match)
		})
		Convey("只有数字和大写字母\n", func() {
			match := handler.verifyPasswordRule("1234567890ABC")
			assert.Equal(t, false, match)
		})
		Convey("只有数字和特殊字符\n", func() {
			match := handler.verifyPasswordRule("1234567890!@#")
			assert.Equal(t, false, match)
		})
		Convey("只有小写字母和大写字母\n", func() {
			match := handler.verifyPasswordRule("abcdefghijklmnABC")
			assert.Equal(t, false, match)
		})
		Convey("只有小写字母和特殊字符\n", func() {
			match := handler.verifyPasswordRule("abcdefghijklmn!@#")
			assert.Equal(t, false, match)
		})
		Convey("只有大写字母和特殊字符\n", func() {
			match := handler.verifyPasswordRule("ABCDEFGHIJKLMN!@#")
			assert.Equal(t, false, match)
		})
	})
}

func Test_restHandler_ListDB(t *testing.T) {

	Convey("ListDB\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/dbs"
		method := "GET"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		Convey("Failed to get dbs\n", func() {
			rMgmt.EXPECT().ListDB(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to get dbs"))

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			resp := []modules.DBInfo{
				{
					DBName:  "test",
					Charset: "utf8mb4",
					Collate: "utf8mb4_unicode_ci",
				},
			}
			rMgmt.EXPECT().ListDB(gomock.Any(), gomock.Any()).Return(resp, nil)

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusOK, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func Test_restHandler_CreateDB(t *testing.T) {

	Convey("CreateDB\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/dbs/test"
		method := "PUT"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckJSONValue(t, engine, target, method)

		Convey("VerifyDBNameRule Failed\n", func() {
			errTarget := "/api/proton-rds-mgmt/v2/dbs/Atest"

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"charset": "utf8mb4"})
			req := httptest.NewRequest(method, errTarget, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusBadRequest, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Charset is empty\n", func() {
			reqParamByte, _ := jsoniter.Marshal(map[string]string{"charset": ""})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusBadRequest, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("CreateDB failed\n", func() {
			rMgmt.EXPECT().CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed to create db"))

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"charset": "utf8mb4", "collate": ""})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			rMgmt.EXPECT().CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"charset": "utf8mb4"})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusCreated, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func Test_restHandler_DeleteDB(t *testing.T) {

	Convey("DeleteDB\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/dbs/test"
		method := "DELETE"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		Convey("DeleteDB failed\n", func() {
			rMgmt.EXPECT().DeleteDB(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed to delete db"))

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			rMgmt.EXPECT().DeleteDB(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusNoContent, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func Test_restHandler_ListUser(t *testing.T) {

	Convey("ListUser\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/users"
		method := "GET"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		Convey("Failed to get users\n", func() {
			rMgmt.EXPECT().ListUser(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to get users"))

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			resp := []modules.UserInfo{
				{
					UserName: "test",
					Privileges: []modules.PrivilegeItem{
						{
							DBName:        "test",
							PrivilegeType: "ReadOnly",
						},
					},
				},
			}
			rMgmt.EXPECT().ListUser(gomock.Any(), gomock.Any()).Return(resp, nil)

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusOK, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func Test_restHandler_CreateOrUpdateUser(t *testing.T) {

	Convey("CreateOrUpdateUser\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/users/test"
		method := "PUT"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckJSONValue(t, engine, target, method)

		CheckPassword(t, engine, handler, target, method)

		Convey("verifyUserNameRule Failed\n", func() {
			errTarget := "/api/proton-rds-mgmt/v2/users/_Atest"

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"password": "YWJjZGU="})
			req := httptest.NewRequest(method, errTarget, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusBadRequest, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("verifyPasswordRule Failed\n", func() {
			reqParamByte, _ := jsoniter.Marshal(map[string]string{"password": "YWJjZGU="})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusBadRequest, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("CreateOrUpdateUser failed\n", func() {
			rMgmt.EXPECT().CreateOrUpdateUser(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed to create user"))

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"password": "ZWlzb28uY29tMTIz"})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			rMgmt.EXPECT().CreateOrUpdateUser(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"password": "ZWlzb28uY29tMTIz"})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusOK, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func Test_restHandler_DeleteUser(t *testing.T) {

	Convey("DeleteUser\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/users/test"
		method := "DELETE"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		Convey("DeleteUser failed\n", func() {
			rMgmt.EXPECT().DeleteUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed to delete user"))

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			rMgmt.EXPECT().DeleteUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			req := httptest.NewRequest(method, target, nil)
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusNoContent, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func Test_restHandler_ModifyUserPrivilege(t *testing.T) {

	Convey("ModifyUserPrivilege\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/users/test/privileges"
		method := "PUT"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckJSONValue(t, engine, target, method)

		Convey("DBName is empty\n", func() {
			reqParams := []map[string]interface{}{
				{
					"db_name":        "",
					"privilege_type": "ReadOnly",
				},
			}
			reqParamByte, _ := jsoniter.Marshal(reqParams)
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusBadRequest, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("PrivilegeType is empty\n", func() {
			reqParams := []map[string]interface{}{
				{
					"db_name":        "test",
					"privilege_type": "",
				},
			}
			reqParamByte, _ := jsoniter.Marshal(reqParams)
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusBadRequest, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("DBName is duplicated\n", func() {
			reqParams := []map[string]interface{}{
				{
					"db_name":        "test",
					"privilege_type": "ReadOnly",
				}, {
					"db_name":        "test",
					"privilege_type": "ReadOnly",
				},
			}
			reqParamByte, _ := jsoniter.Marshal(reqParams)
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusBadRequest, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("ModifyUserPrivilege Failed\n", func() {
			rMgmt.EXPECT().ModifyUserPrivilege(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed to modify user privileges"))

			reqParams := []map[string]interface{}{
				{
					"db_name":        "test",
					"privilege_type": "ReadOnly",
				},
			}
			reqParamByte, _ := jsoniter.Marshal(reqParams)
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			rMgmt.EXPECT().ModifyUserPrivilege(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			reqParams := []map[string]interface{}{
				{
					"db_name":        "test",
					"privilege_type": "ReadOnly",
				},
			}
			reqParamByte, _ := jsoniter.Marshal(reqParams)
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusNoContent, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Patch Success\n", func() {
			rMgmt.EXPECT().ModifyUserPrivilege(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			reqParams := []map[string]interface{}{
				{
					"db_name":        "test",
					"privilege_type": "None",
				},
			}
			method := "PATCH"
			reqParamByte, _ := jsoniter.Marshal(reqParams)
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusNoContent, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func Test_restHandler_Health(t *testing.T) {

	Convey("Health\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/health"
		method := "GET"

		Convey("Success\n", func() {
			req := httptest.NewRequest(method, target, nil)

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusOK, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

func CheckPermission(t *testing.T, engine *gin.Engine, target string, method string, rMgmt *dmock.MockRDSMgmt) {
	Convey("failed\n", func() {
		rMgmt.EXPECT().CheckPermission(gomock.Any(), gomock.Any()).Return(common.NewHTTPError("need root user", common.BadRequest, nil))

		reqParamByte, _ := jsoniter.Marshal(map[string]string{})
		req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
		req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		result := w.Result()
		assert.Equal(t, http.StatusBadRequest, result.StatusCode)

		if err := result.Body.Close(); err != nil {
			assert.Equal(t, nil, err)
		}

	})
}
func Test_restHandler_CreateBackup(t *testing.T) {

	Convey("CreateBackup\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/backups"
		method := "POST"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckPermission(t, engine, target, method, rMgmt)

		Convey("permission ok\n", func() {
			rMgmt.EXPECT().CheckPermission(gomock.Any(), gomock.Any()).Return(nil)
			reqParams := []map[string]interface{}{
				{
					"backup_dir": "/data/backup",
				},
			}
			reqParamByte, _ := jsoniter.Marshal(reqParams)
			Convey("Backup Failed\n", func() {
				bMgmt.EXPECT().CreateBackup(gomock.Any(), gomock.Any()).Return(modules.BackupInfo{}, errors.New("failed to backup"))
				req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})
			Convey("Success\n", func() {
				bMgmt.EXPECT().CreateBackup(gomock.Any(), gomock.Any()).Return(modules.BackupInfo{}, nil)
				req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusAccepted, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})
		})

	})
}

func Test_restHandler_DeleteBackup(t *testing.T) {
	Convey("DeleteBackup\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/backups/1"
		method := "DELETE"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckPermission(t, engine, target, method, rMgmt)

		Convey("permission ok\n", func() {
			rMgmt.EXPECT().CheckPermission(gomock.Any(), gomock.Any()).Return(nil)

			Convey("Delete failed\n", func() {
				bMgmt.EXPECT().DeleteBackup(gomock.Any()).Return(errors.New("failed to delete"))
				req := httptest.NewRequest(method, target, nil)
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})

			Convey("Success\n", func() {
				bMgmt.EXPECT().DeleteBackup(gomock.Any()).Return(nil)
				req := httptest.NewRequest(method, target, nil)
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusNoContent, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})
		})
	})
}

func Test_restHandler_ListBackup(t *testing.T) {
	Convey("ListBackup\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/backups"
		method := "GET"

		backupInfo := modules.BackupInfo{
			Id: "123",
		}

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckPermission(t, engine, target, method, rMgmt)

		Convey("permission ok\n", func() {
			rMgmt.EXPECT().CheckPermission(gomock.Any(), gomock.Any()).Return(nil)

			Convey("list failed\n", func() {
				bMgmt.EXPECT().ListBackup().Return(modules.BackupInfos{}, errors.New("failed to list"))
				req := httptest.NewRequest(method, target, nil)
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})

			Convey("Success\n", func() {

				bMgmt.EXPECT().ListBackup().Return(modules.BackupInfos{backupInfo}, nil)
				req := httptest.NewRequest(method, target, nil)
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusOK, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})
		})

	})
}

func Test_restHandler_GetBackupSize(t *testing.T) {
	Convey("GetBackupSize\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/backup_size"
		method := "GET"

		size := 1024

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckPermission(t, engine, target, method, rMgmt)

		Convey("permission ok\n", func() {
			rMgmt.EXPECT().CheckPermission(gomock.Any(), gomock.Any()).Return(nil)

			Convey("get failed\n", func() {
				bMgmt.EXPECT().GetBackupDataSize().Return(size, errors.New("failed to get"))
				req := httptest.NewRequest(method, target, nil)
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})

			Convey("Success\n", func() {

				bMgmt.EXPECT().GetBackupDataSize().Return(size, nil)
				req := httptest.NewRequest(method, target, nil)
				req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

				w := httptest.NewRecorder()
				engine.ServeHTTP(w, req)

				result := w.Result()
				assert.Equal(t, http.StatusOK, result.StatusCode)

				if err := result.Body.Close(); err != nil {
					assert.Equal(t, nil, err)
				}
			})
		})

	})
}

func Test_restHandler_UpdateUserSSL(t *testing.T) {

	Convey("UpdateUserSSL\n", t, func() {

		test := setGinMode()
		defer test()
		engine := gin.New()
		engine.Use(gin.Recovery())

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		rMgmt := dmock.NewMockRDSMgmt(ctrl)
		hClient := cmock.NewMockHTTPClient(ctrl)
		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		bMgmt := dmock.NewMockBackupMgmt(ctrl)
		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
		handler.RegisterPublic(engine)

		target := "/api/proton-rds-mgmt/v2/users/test/ssls"
		method := "PUT"

		CheckToken(t, engine, hClient, handler, target, method)

		CheckAdminKey(t, engine, handler, target, method)

		CheckJSONValue(t, engine, target, method)

		CheckPassword(t, engine, handler, target, method)

		Convey("UpdateUserSsl failed\n", func() {
			rMgmt.EXPECT().ModifyUserSsl(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed to create user"))

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"ssl_type": "Any"})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})

		Convey("Success\n", func() {
			rMgmt.EXPECT().ModifyUserSsl(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			reqParamByte, _ := jsoniter.Marshal(map[string]string{"ssl_type": "Any"})
			req := httptest.NewRequest(method, target, bytes.NewReader(reqParamByte))
			req.Header.Set("admin-key", "cm9vdDplaXNvby5jb20=")

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			result := w.Result()
			assert.Equal(t, http.StatusNoContent, result.StatusCode)

			if err := result.Body.Close(); err != nil {
				assert.Equal(t, nil, err)
			}
		})
	})
}

// func Test_restHandler_CheckDbAvailability(t *testing.T) {
// 	Convey("CheckDbAvailability\n", t, func() {

// 		test := setGinMode()
// 		defer test()
// 		engine := gin.New()
// 		engine.Use(gin.Recovery())

// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		rMgmt := dmock.NewMockRDSMgmt(ctrl)
// 		hClient := cmock.NewMockHTTPClient(ctrl)
// 		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		bMgmt := dmock.NewMockBackupMgmt(ctrl)
// 		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
// 		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
// 		handler.RegisterPublic(engine)

// 		target := "/api/proton-rds-mgmt/v2/healthcheck"
// 		method := "POST"

// 		Convey("Not Healthy\n", func() {
// 			rMgmt.EXPECT().CheckDbAvailability().Return(errors.New("failed"))
// 			req := httptest.NewRequest(method, target, nil)

// 			w := httptest.NewRecorder()
// 			engine.ServeHTTP(w, req)

// 			result := w.Result()
// 			assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

// 			if err := result.Body.Close(); err != nil {
// 				assert.Equal(t, nil, err)
// 			}
// 		})

// 		Convey("Healthy\n", func() {

// 			rMgmt.EXPECT().CheckDbAvailability().Return(nil)
// 			req := httptest.NewRequest(method, target, nil)

// 			w := httptest.NewRecorder()
// 			engine.ServeHTTP(w, req)

// 			result := w.Result()
// 			assert.Equal(t, http.StatusOK, result.StatusCode)

// 			if err := result.Body.Close(); err != nil {
// 				assert.Equal(t, nil, err)
// 			}
// 		})
// 	})
// }

// func Test_restHandler_GetLastCheckTableResult(t *testing.T) {
// 	Convey("CheckDbAvailability\n", t, func() {

// 		test := setGinMode()
// 		defer test()
// 		engine := gin.New()
// 		engine.Use(gin.Recovery())

// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		rMgmt := dmock.NewMockRDSMgmt(ctrl)
// 		hClient := cmock.NewMockHTTPClient(ctrl)
// 		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		bMgmt := dmock.NewMockBackupMgmt(ctrl)
// 		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
// 		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
// 		handler.RegisterPublic(engine)

// 		target := "/api/proton-rds-mgmt/v2/corrupt_table_list"
// 		method := "GET"

// 		Convey("have not corrupt table\n", func() {
// 			cMgmt.EXPECT().GetLastCheckResult().Return([]string{})
// 			req := httptest.NewRequest(method, target, nil)

// 			w := httptest.NewRecorder()
// 			engine.ServeHTTP(w, req)

// 			result := w.Result()
// 			assert.Equal(t, http.StatusOK, result.StatusCode)
// 			var b []string
// 			jsoniter.Unmarshal(w.Body.Bytes(), &b)
// 			assert.Equal(t, 0, len(b))
// 			if err := result.Body.Close(); err != nil {
// 				assert.Equal(t, nil, err)
// 			}
// 		})

// 		Convey("have corrupt table\n", func() {
// 			cMgmt.EXPECT().GetLastCheckResult().Return([]string{"t1", "t2"})
// 			req := httptest.NewRequest(method, target, nil)

// 			w := httptest.NewRecorder()
// 			engine.ServeHTTP(w, req)

// 			result := w.Result()
// 			assert.Equal(t, http.StatusOK, result.StatusCode)
// 			var b []string
// 			jsoniter.Unmarshal(w.Body.Bytes(), &b)
// 			assert.Equal(t, 2, len(b))
// 			if err := result.Body.Close(); err != nil {
// 				assert.Equal(t, nil, err)
// 			}
// 		})
// 	})
// }

// func Test_restHandler_ClearCheckTableResult(t *testing.T) {
// 	Convey("ClearCheckTableResult\n", t, func() {

// 		test := setGinMode()
// 		defer test()
// 		engine := gin.New()
// 		engine.Use(gin.Recovery())

// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		rMgmt := dmock.NewMockRDSMgmt(ctrl)
// 		hClient := cmock.NewMockHTTPClient(ctrl)
// 		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		bMgmt := dmock.NewMockBackupMgmt(ctrl)
// 		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
// 		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
// 		handler.RegisterPublic(engine)

// 		target := "/api/proton-rds-mgmt/v2/corrupt_table_list"
// 		method := "DELETE"

// 		Convey("do anyway\n", func() {
// 			cMgmt.EXPECT().ResetCheckResult()
// 			req := httptest.NewRequest(method, target, nil)

// 			w := httptest.NewRecorder()
// 			engine.ServeHTTP(w, req)

// 			result := w.Result()
// 			assert.Equal(t, http.StatusOK, result.StatusCode)
// 			if err := result.Body.Close(); err != nil {
// 				assert.Equal(t, nil, err)
// 			}
// 		})
// 	})
// }

// func Test_restHandler_GenCheckTableResult(t *testing.T) {
// 	Convey("GenCheckTableResult\n", t, func() {

// 		test := setGinMode()
// 		defer test()
// 		engine := gin.New()
// 		engine.Use(gin.Recovery())

// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		rMgmt := dmock.NewMockRDSMgmt(ctrl)
// 		hClient := cmock.NewMockHTTPClient(ctrl)
// 		rMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		bMgmt := dmock.NewMockBackupMgmt(ctrl)
// 		bMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		cMgmt := dmock.NewMockCheckTableWorker(ctrl)
// 		cMgmt.EXPECT().SetConfig(gomock.Any()).Return()
// 		handler := newRESTHandler(rMgmt, hClient, bMgmt, cMgmt)
// 		handler.RegisterPublic(engine)

// 		target := "/api/proton-rds-mgmt/v2/check_table_task"
// 		method := "POST"

// 		Convey("do anyway\n", func() {
// 			cMgmt.EXPECT().Check()
// 			req := httptest.NewRequest(method, target, nil)

// 			w := httptest.NewRecorder()
// 			engine.ServeHTTP(w, req)

// 			result := w.Result()
// 			assert.Equal(t, http.StatusOK, result.StatusCode)
// 			if err := result.Body.Close(); err != nil {
// 				assert.Equal(t, nil, err)
// 			}
// 		})
// 	})
// }
