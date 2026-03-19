package v1

import (
	"context"
	"encoding/json"
	"testing"

	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/rest"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/core/global"
)

func TestCreateNginxHTTP(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"
	c, err := NewForConfig(&rest.Config{Host: "10.4.15.71:9202"})
	if err != nil {
		t.Fatal(err)
	}

	const name = "test-eceph"

	nh := &NginxHTTP{
		Name: name,
		Conf: NginxHTTPConf{
			Server: NginxServer{
				Listen:    "12450 default_server",
				AccessLog: "on",
			},
			Upstream: map[string]NginxUpstream{
				name: {
					CheckHTTPSend: `"HEAD / HTTP/1.1\r\nConnection: keep-alive\r\n\r\n"`,
					Check:         "interval=10000 rise=2 fall=3 timeout=1000 type=http  default_down=true",
					Servers: []string{
						"10.4.15.71:80",
						"10.4.15.72:80",
						"10.4.15.73:80",
					},
					Keepalive: "300",
				},
			},
		},
	}

	if err := c.NginxHTTPs().Create(context.TODO(), nh); err != nil {
		t.Log(err)
	}
}
func TestGetNginxHTTP(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"

	c, err := NewForConfig(&rest.Config{Host: "10.4.15.105:9202"})
	if err != nil {
		t.Fatal(err)
	}

	r, err := c.NginxHTTPs().Get(context.TODO(), "eceph_10001")
	if err != nil {
		t.Fatal(err)
	}

	j, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(j))
}

func TestListNginxHTTPs(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"

	c, err := NewForConfig(&rest.Config{Host: "10.4.15.71:9202"})
	if err != nil {
		t.Fatal(err)
	}

	r, err := c.NginxHTTPs().List(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	t.Log(r)
}

func TestUpdateNginxHTTP(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"

	c, err := NewForConfig(&rest.Config{Host: "10.4.15.71:9202"})
	if err != nil {
		t.Fatal(err)
	}

	const name = "test-eceph"

	nh := &NginxHTTP{
		Name: name,
		Conf: NginxHTTPConf{
			Server: NginxServer{
				Listen: "12450 default_server",
				Locations: []map[string]map[string]interface{}{
					{
						"/": map[string]interface{}{
							"proxy_pass": "http://test-eceph",
						},
					},
				},
				AccessLog: "on",
			},
			Upstream: map[string]NginxUpstream{
				name: {
					CheckHTTPSend: `"HEAD / HTTP/1.1\r\nConnection: keep-alive\r\n\r\n"`,
					Check:         "interval=10000 rise=2 fall=3 timeout=1000 type=http  default_down=true",
					Servers: []string{
						"10.4.15.71:80",
						"10.4.15.72:80",
						"10.4.15.73:80",
					},
					Keepalive: "300",
				},
			},
		},
	}

	if err := c.NginxHTTPs().Update(context.TODO(), nh); err != nil {
		t.Log(err)
	}
}
