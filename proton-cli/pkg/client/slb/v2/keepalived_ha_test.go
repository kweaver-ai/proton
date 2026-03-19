package v2

import (
	"context"
	"encoding/json"
	"testing"

	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/rest"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/core/global"
)

func init() {
	global.LoggerLevel = "debug"
}

func TestList(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"

	cfg := &rest.Config{Host: "http://10.4.15.105:9202"}
	c, err := NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	names, err := c.KeepalivedHAs().List(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("len(names) = %d", len(names))
	for _, n := range names {
		t.Logf("keepalived/ha/%s", n)
	}
}

func TestGet(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"

	cfg := &rest.Config{Host: "http://10.4.15.105:9202"}
	c, err := NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	kha, err := c.KeepalivedHAs().Get(context.TODO(), "eceph_vip")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(kha, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("keepalived HA eceph_vip:\n%v", string(b))
}

func TestCreateKeepalivedHA(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"

	c, err := NewForConfig(&rest.Config{Host: "10.4.15.71:9202"})
	if err != nil {
		t.Fatal(err)
	}

	kha := &KeepalivedHA{
		Interface:     "ens160",
		UnicastSRC_IP: "10.4.15.71",
		UnicastPeer: []string{
			"10.4.15.72",
			"10.4.15.73",
		},
		VirtualRouterID: "111",
		Priority:        "71",
		VirtualIPAddress: map[string]string{
			"10.4.15.74": "label ens160:xxx dev ens160",
		},
		NotifyMaster: "logger xxx enter master",
		NotifyBackup: "logger xxx enter backup",
	}

	if err := c.KeepalivedHAs().Create(context.TODO(), "xxx", kha); err != nil {
		t.Log(err)
	}
}

func TestUpdateKeepalivedHA(t *testing.T) {
	t.Skip("unimplemented")

	global.LoggerLevel = "debug"

	c, err := NewForConfig(&rest.Config{Host: "10.4.15.71:9202"})
	if err != nil {
		t.Fatal(err)
	}

	kha := &KeepalivedHA{
		Interface:     "ens160",
		UnicastSRC_IP: "10.4.15.71",
		UnicastPeer: []string{
			"10.4.15.72",
			"10.4.15.73",
		},
		VirtualRouterID: "111",
		Priority:        "71",
		VirtualIPAddress: map[string]string{
			"10.4.15.74": "label ens160:xxx dev ens160",
		},
		NotifyMaster: "/root/notify-master",
		NotifyBackup: "/root/notify-backup",
	}

	if err := c.KeepalivedHAs().Update(context.TODO(), "xxx", kha); err != nil {
		t.Log(err)
	}
}
