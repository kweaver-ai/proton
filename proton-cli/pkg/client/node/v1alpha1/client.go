package v1alpha1

import (
	"bytes"
	"fmt"
	"net"

	eceph_agent_config "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/eceph/agent_config/v1alpha1"
	ecms "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/ecms/v1alpha1"
	exec "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/exec/v1alpha1"
	firewalld "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/firewalld/v1alpha1"
	helm "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/helm/v2"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/rest"
	slb_v1 "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/slb/v1"
	slb_v2 "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/slb/v2"
	systemd "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/systemd/v1alpha1"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/configuration"
)

// SSHClient 是访问节点的客户端的实现
//
// 线程不安全
//
// TODO: close ssh client and sftp client
type Client struct {
	// node name
	name string

	ipv4, ipv6, internal net.IP

	executor exec.Executor

	slbV1 slb_v1.SLB_V1Interface
	slbV2 slb_v2.SLB_V2Interface
}

// NewSSHClient 返回 SSHClient
func New(node *configuration.Node) (*Client, error) {
	restConfig := &rest.Config{Host: net.JoinHostPort(node.IP(), fmt.Sprintf("%d", slb_v1.DefaultPort))}

	ecms := ecms.NewForHost(node.IP())

	slbV1, err := slb_v1.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	slbV2, err := slb_v2.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &Client{
		name:     node.Name,
		ipv4:     net.ParseIP(node.IP4),
		ipv6:     net.ParseIP(node.IP6),
		internal: net.ParseIP(node.Internal_ip),
		executor: exec.NewECMSExecutorForHost(ecms.Exec()),
		slbV1:    slbV1,
		slbV2:    slbV2,
	}, nil
}

// Name implements Interface.
func (c *Client) Name() string {
	return c.name
}

// IP implements Interface.
func (c *Client) IP() net.IP {
	if c.ipv4 != nil {
		return c.ipv4
	}
	return c.ipv6
}

// IPVersion return the type of returned IP by IP()
func (c *Client) IPVersion() string {
	if c.ipv4 != nil {
		return configuration.IPVersionIPV4
	}
	return configuration.IPVersionIPV6
}

// InternalIP implements Interface.
func (c *Client) InternalIP() net.IP {
	return c.internal
}

// NetworkInterfaces implements Interface.
func (c *Client) NetworkInterfaces() ([]NetworkInterface, error) {
	out, err := c.executor.Command("ip", "address").Output()
	if err != nil {
		return nil, err
	}

	return parseOutputOfIPAddress(bytes.NewBuffer(out))
}

// Systemd implements Interface.
func (c *Client) Systemd() systemd.Interface {
	return systemd.New(c.executor)
}

// Firewalld implements Interface.
func (c *Client) Firewalld() firewalld.Interface {
	return firewalld.New(c.executor)
}

// ECMS implements Interface.
func (c *Client) ECMS() ecms.Interface {
	return ecms.NewForHost(c.IP().String())
}

// ECephAgentConfig implements Interface.
func (c *Client) ECephAgentConfig() eceph_agent_config.Interface {
	return eceph_agent_config.New(c.executor)
}

// SLB_V1 implements Interface.
func (c *Client) SLB_V1() slb_v1.SLB_V1Interface {
	return c.slbV1
}

// SLB_V2 implements Interface.
func (c *Client) SLB_V2() slb_v2.SLB_V2Interface {
	return c.slbV2
}

// Deprecated: use helm/v3 instead
func (c *Client) Helm() helm.Interface {
	return helm.New(c.executor)
}

var _ Interface = (*Client)(nil)
