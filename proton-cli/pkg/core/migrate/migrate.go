package migrate

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/clock"

	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/node/v1alpha1"
	v1 "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/slb/v1"
	c "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/configuration"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/core/global"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/core/logger"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/proton/eceph"
	ecephCompletion "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/proton/eceph/completion"
	ecephValidation "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/proton/eceph/validation"
)

type (
	I   = []interface{}
	MSI = map[string]interface{}
	MII = map[interface{}]interface{}
)

func MII2MSI(miiin interface{}) map[string]interface{} {
	msiout := make(map[string]interface{})
	_, ok := miiin.(MSI)
	if ok {
		return miiin.(MSI)
	}
	_, ok = miiin.(MII)
	if !ok {
		panic("MII2MSI: input is not of Map[string]interface{} type.")
	}
	for key, value := range miiin.(MII) {
		switch key := key.(type) {
		case string:
			msiout[key] = value
		default:
			panic(fmt.Sprintf("encountered non-string key: %v", key))
		}
	}
	return msiout
}

func validateFieldExists(m MSI, fields []string) string {
	for _, f := range fields {
		if _, ok := m[f]; !ok {
			return f
		}
	}
	return ""
}

func migrateProtonCLIConfig4ECeph(protonDeployConf c.ProtonDeployClusterConfig, olClusterConf *c.ClusterConfig, tlsSecretName string) error {
	// use eceph>hosts as eceph>hosts
	actualECeph := MII2MSI(protonDeployConf[c.FnProtonDeployECeph])
	h := actualECeph[c.FnProtonDeployECephHosts]
	for _, hostitem := range h.([]interface{}) {
		olClusterConf.ECeph.Hosts = append(olClusterConf.ECeph.Hosts, hostitem.(string))
	}

	// use slb>ha[0]>vip as keepalived internal vip and as_vip as external vip
	olClusterConf.ECeph.Keepalived = &c.ECephKeepalived{}
	if _, ok := protonDeployConf[c.FnProtonDeploySLB]; ok {
		actualSLB := MII2MSI(protonDeployConf[c.FnProtonDeploySLB])

		if _, ok := actualSLB[c.FnProtonDeployHighlyAvailable]; ok && actualSLB[c.FnProtonDeployHighlyAvailable] != nil {
			actualSLBHAVIP := MII2MSI(actualSLB[c.FnProtonDeployHighlyAvailable].([]interface{})[0])[c.FnProtonDeployHAVirtualIP]
			if actualSLBHAVIP != nil {
				olClusterConf.ECeph.Keepalived.Internal = actualSLBHAVIP.(string)
			}
		}
	}

	if tlsSecretName != "" {
		olClusterConf.ECeph.TLS.Secret = tlsSecretName
	} else {
		olClusterConf.ECeph.TLS.Secret = new(clock.RealClock).Now().Format("eceph-2006-01-02-15-04-05")
	}

	//	load the existing key file, if there are none, return error
	certPath := c.PDDefaultDstSSLPath
	crtFileName := c.PDDefaultCrtFileName
	keyFileName := c.PDDefaultKeyFileName
	self_cert, ok := actualECeph[c.FnProtonDeployECephSelfCert]
	if ok && self_cert.(bool) {
		certPath = actualECeph[c.FnPDECephDstSSLPath].(string)
		actualCrtFile, ok := actualECeph[c.FnPDECephCertFile]
		if ok {
			crtFileName = actualCrtFile.(string)
		}
		actualKeyFile, ok := actualECeph[c.FnPDECephKeyFile]
		if ok {
			keyFileName = actualKeyFile.(string)
		}
	}
	crtFullPath := filepath.Join(certPath, crtFileName)
	keyFullPath := filepath.Join(certPath, keyFileName)
	crtBinary, err := os.ReadFile(crtFullPath)
	if err != nil {
		return fmt.Errorf("failed to read ECeph TLS cert file: %v", err)
	}
	keyBinary, err := os.ReadFile(keyFullPath)
	if err != nil {
		return fmt.Errorf("failed to read ECeph TLS key file: %v", err)
	}
	olClusterConf.ECeph.TLS.CertificateData = crtBinary
	olClusterConf.ECeph.TLS.KeyData = keyBinary
	return nil
}

func EnforceDefaultPDConfig4ECeph(protonDeployConf c.ProtonDeployClusterConfig) error {
	// validate that slb>slb_listen either does not exist or matches the SLB client default port
	if _, ok := protonDeployConf[c.FnProtonDeploySLB]; ok {
		actualSLBListen, ok := MII2MSI(protonDeployConf[c.FnProtonDeploySLB])[c.FnProtonDeploySLBListenPort]
		if ok && fmt.Sprintf("%v", actualSLBListen) != fmt.Sprintf("%v", v1.DefaultPort) {
			return fmt.Errorf("slb_listen value must be %v", v1.DefaultPort)
		}
	}

	// validate that slb>ha>label == eceph>lb>vip == "ivip" and eceph>namespace either does not exist or is "default"
	actualECeph := MII2MSI(protonDeployConf[c.FnProtonDeployECeph])
	actualECephNamespace, ok := actualECeph[c.FnProtonDeployECephNamespace]
	if ok && actualECephNamespace.(string) != c.PDDefaultECephNamespace {
		return fmt.Errorf("eceph>namespace value must be %v", c.PDDefaultECephNamespace)
	}
	var actualECephLBVIP interface{}
	if _, ok := actualECeph[c.FnPDECephLoadBalancer]; ok {
		actualECephLBVIP = MII2MSI(actualECeph[c.FnPDECephLoadBalancer])[c.FnPDECephLoadBalancerVirtualIP]
	} else {
		actualECephLBVIP = eceph.KeepalivedVirtualAddressLabelECephInternalVIP
	}
	var actualSLBHALabel interface{}
	if _, ok := protonDeployConf[c.FnProtonDeploySLB]; ok {
		actualSLB := MII2MSI(protonDeployConf[c.FnProtonDeploySLB])
		if _, ok := actualSLB[c.FnProtonDeployHighlyAvailable]; ok && actualSLB[c.FnProtonDeployHighlyAvailable] != nil {
			actualSLBHALabel = MII2MSI(actualSLB[c.FnProtonDeployHighlyAvailable].([]interface{})[0])[c.FnProtonDeployHALabel]
		} else {
			actualSLBHALabel = eceph.KeepalivedVirtualAddressLabelECephInternalVIP
		}
	} else {
		actualSLBHALabel = eceph.KeepalivedVirtualAddressLabelECephInternalVIP
	}
	if !(actualECephLBVIP.(string) == actualSLBHALabel.(string) && actualSLBHALabel.(string) == eceph.KeepalivedVirtualAddressLabelECephInternalVIP) {
		return fmt.Errorf("slb>ha[0]>vip should be equal to eceph>lb>vip and also equal to %v", eceph.KeepalivedVirtualAddressLabelECephInternalVIP)
	}
	return nil
}

func validatePDConfig4ECeph(protonDeployConf c.ProtonDeployClusterConfig, olClusterConf *c.ClusterConfig) error {
	if protonDeployConf == nil {
		return fmt.Errorf("proton-deploy config was empty")
	}
	// validate that required top level fields exist
	topLevelFields := []string{
		c.FnProtonDeployAPIVersion,
		c.FnProtonDeployHosts,
		c.FnProtonDeployECeph,
	}

	if missingField := validateFieldExists(protonDeployConf, topLevelFields); missingField != "" {
		return fmt.Errorf("Missing field %v in proton-deploy config file provided", missingField)
	}

	// validate types of top level elements
	if reflect.ValueOf(protonDeployConf[c.FnProtonDeployAPIVersion]).Kind() != reflect.String ||
		reflect.ValueOf(protonDeployConf[c.FnProtonDeployHosts]).Kind() != reflect.Map ||
		reflect.ValueOf(protonDeployConf[c.FnProtonDeployECeph]).Kind() != reflect.Map {
		return fmt.Errorf("One of top level elements in proton-deploy config file is of wrong type")
	}

	// validate hosts
	h := MII2MSI(protonDeployConf[c.FnProtonDeployHosts])
	hostFields := []string{
		c.FnProtonDeployHostsSSHIP,
		c.FnPDHostsInternalIP,
	}
	hostCount := 0
	hostList := []string{}
	for k, v := range h {
		errHosts := validateFieldExists(MII2MSI(v), hostFields)
		if errHosts != "" {
			return fmt.Errorf("Missing field %v in hosts item %v", errHosts, k)
		}
		if reflect.ValueOf(MII2MSI(v)[c.FnProtonDeployHostsSSHIP]).Kind() != reflect.String ||
			reflect.ValueOf(MII2MSI(v)[c.FnPDHostsInternalIP]).Kind() != reflect.String {
			return fmt.Errorf("One of elements in the hosts section of provided proton-deploy config file is of wrong type")
		}
		// validate that all hosts in proton-deploy config file existed in proton-cli config file and the ip4 address is the same
		flagNodeExists := false
		ip, _, err := net.ParseCIDR(MII2MSI(v)[c.FnProtonDeployHostsSSHIP].(string))
		if err != nil {
			return fmt.Errorf("Node %v's IP address is invalid:%v", k, err)
		}
		for _, protonCliNode := range olClusterConf.Nodes {
			pnip, _, _ := net.ParseCIDR(MII2MSI(v)[c.FnProtonDeployHostsSSHIP].(string))
			if k == protonCliNode.Name && (ip.String() == pnip.String()) {
				flagNodeExists = true
				hostList = append(hostList, k)
			}
		}
		if !flagNodeExists {
			return fmt.Errorf("The nodes %v described in proton-deploy config's node name or IP address differs from those in proton-cli config", k)
		}
		hostCount++
	}
	if hostCount > c.ProtonDeployMaxHostsAllowed {
		return fmt.Errorf("There are too many hosts in the proton-deploy config file, maximum hosts allowed:%v", c.ProtonDeployMaxHostsAllowed)
	}
	if hostCount == 0 {
		return fmt.Errorf("There are no hosts in the proton-deploy config file")
	}

	// validate eceph
	actualECeph := MII2MSI(protonDeployConf[c.FnProtonDeployECeph])
	if _, ok := actualECeph[c.FnProtonDeployECephHosts]; !ok {
		return fmt.Errorf("missing field %v in eceph section of proton-deploy config file", c.FnProtonDeployECephHosts)
	} else {
		if len(actualECeph[c.FnProtonDeployECephHosts].([]interface{})) < 1 {
			return fmt.Errorf("There are no nodes to deploy ECeph as indicated by the proton-deploy config file")
		}
		for _, h := range actualECeph[c.FnProtonDeployECephHosts].([]interface{}) {
			flagNodeExistsInHost := false
			for _, hh := range hostList {
				if h.(string) == hh {
					flagNodeExistsInHost = true
				}
			}
			if !flagNodeExistsInHost {
				return fmt.Errorf("The host %v to deploy ECeph does not exist in Hosts section", h)
			}
		}
	}

	return nil
}

func Migrate(protonDeployConf c.ProtonDeployClusterConfig, migrateMode string, tlsSecretName string, tlsCertificatePath string, tlsKeyPath string) error {
	var log = logger.NewLogger()
	switch migrateMode {
	case global.MigrateECephAndAnyShare:
		log.Infoln("Migrate ECeph and AnyShare fusion deployment to proton-cli start")
		var olClusterConf *c.ClusterConfig
		_, k := client.NewK8sClient()

		//phase 1: get current conf from kubernetes
		if k == nil {
			return fmt.Errorf("unable to create Kubernetes client")
		}
		con, err := c.LoadFromKubernetes(context.Background(), k)
		if err != nil {
			return fmt.Errorf("unable load old cluster conf, cannot migrate without an existing Proton Runtime installation: %v", err)
		} else {
			olClusterConf = con
		}
		if olClusterConf.ECeph == nil {
			olClusterConf.ECeph = &c.ECeph{
				Hosts:      make([]string, 0),
				Keepalived: nil,
				TLS:        c.ECephTLS{},
			}
		}
		if len(olClusterConf.ECeph.Hosts) > 0 {
			return fmt.Errorf("Cannot migrate external ECeph installation when there already exists ECeph instance installed by Proton-CLI")
		}
		log.Infoln("Successfully got the current Proton-CLI cluster config")

		//phase 2: validate proton-deploy config structure
		err = validatePDConfig4ECeph(protonDeployConf, olClusterConf)
		if err != nil {
			return err
		}
		log.Infoln("Proton-Deploy config structure validation succeeded")

		//phase 3: enforce default values that is not supported in proton-cli implementation
		err = EnforceDefaultPDConfig4ECeph(protonDeployConf)
		if err != nil {
			return err
		}
		log.Infoln("Proton-Deploy config immutable fields validation succeeded")

		//phase 4: update the existing proton-cli config with the information from proton-deploy config, any unsupported fields are ignored in this phase
		err = migrateProtonCLIConfig4ECeph(protonDeployConf, olClusterConf, tlsSecretName)
		if err != nil {
			return err
		}
		if !strings.Contains(olClusterConf.ECeph.Keepalived.External, "/") {
			var nodes []v1alpha1.Interface
			for _, node := range olClusterConf.Nodes {
				n, err := v1alpha1.New(&node)
				if err != nil {
					return fmt.Errorf("create node/v1alpha1 %v fail: %w", node.Name, err)
				}
				nodes = append(nodes, n)
			}
			if !strings.Contains(olClusterConf.ECeph.Keepalived.External, "/") &&
				(strings.Contains(olClusterConf.ECeph.Keepalived.External, ".") || strings.Contains(olClusterConf.ECeph.Keepalived.External, ":")) &&
				len(olClusterConf.ECeph.Keepalived.External) > 1 {
				if net.ParseIP(strings.Split(olClusterConf.ECeph.Keepalived.External, "/")[0]) != nil {
					ecephCompletion.CompleteExtVIP(olClusterConf.ECeph, nodes)
				}
			}
		}
		log.Infoln("Construct Proton-CLI ECeph config from Proton-Deploy config succeeded")

		//phase 5: validate the new generated ECeph section
		var allErrs field.ErrorList
		allErrs = append(allErrs, ecephValidation.Validate(olClusterConf.ECeph, olClusterConf.Nodes, olClusterConf.ResourceConnectInfo, field.NewPath("eceph"))...)
		allErrs = append(allErrs, ecephValidation.ValidatePost(olClusterConf.ECeph, olClusterConf.Nodes, olClusterConf.ResourceConnectInfo, field.NewPath("eceph"))...)
		if allErrs != nil {
			return fmt.Errorf("invalid eceph cluster config during vaildation: %v", allErrs)
		}
		log.Infoln("Validate constructed Proton-CLI ECeph config from Proton-Deploy config succeeded")

		//phase 6: upload updated cluster config to kubernetes.
		if err := c.UploadToKubernetes(context.Background(), olClusterConf, k); err != nil {
			return fmt.Errorf("unable to upload cluster config to kubernetes: %w", err)
		}
		log.Infoln("Save constructed Proton-CLI config succeeded")
	case global.MigrateUpdateECephCert:
		log.Infoln("Migrate update ECeph TLS Certificate information start")
		if len(tlsCertificatePath) < 1 || len(tlsKeyPath) < 1 {
			return fmt.Errorf("You must provide ECeph certificate and key file path when updating ECeph TLS certificate")
		}
		var olClusterConf *c.ClusterConfig
		_, k := client.NewK8sClient()

		//phase 1: get current conf from kubernetes
		if k == nil {
			return fmt.Errorf("unable to create Kubernetes client")
		}
		con, err := c.LoadFromKubernetes(context.Background(), k)
		if err != nil {
			return fmt.Errorf("unable load old cluster conf, cannot migrate without an existing Proton Runtime installation: %v", err)
		} else {
			olClusterConf = con
		}
		if olClusterConf.ECeph == nil {
			return fmt.Errorf("Cannot update ECeph certificate information when there is no existing ECeph installation")
		}
		log.Infoln("Successfully got the current Proton-CLI cluster config")

		// phase 2: update certificate information with the provided file
		if len(tlsSecretName) > 0 {
			olClusterConf.ECeph.TLS.Secret = tlsSecretName
		}
		crtBinary, err := os.ReadFile(tlsCertificatePath)
		if err != nil {
			return fmt.Errorf("failed to read ECeph TLS cert file: %v", err)
		}
		keyBinary, err := os.ReadFile(tlsKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read ECeph TLS key file: %v", err)
		}
		olClusterConf.ECeph.TLS.CertificateData = crtBinary
		olClusterConf.ECeph.TLS.KeyData = keyBinary

		var allErrs field.ErrorList
		allErrs = append(allErrs, ecephValidation.Validate(olClusterConf.ECeph, olClusterConf.Nodes, olClusterConf.ResourceConnectInfo, field.NewPath("eceph"))...)
		allErrs = append(allErrs, ecephValidation.ValidatePost(olClusterConf.ECeph, olClusterConf.Nodes, olClusterConf.ResourceConnectInfo, field.NewPath("eceph"))...)
		if allErrs != nil {
			return fmt.Errorf("invalid eceph cluster config during vaildation: %v", allErrs)
		}
		log.Infoln("Validate updated Proton-CLI ECeph config succeeded")

		// phase 3: upload configuration into proton-cli-config
		if err := c.UploadToKubernetes(context.Background(), olClusterConf, k); err != nil {
			return fmt.Errorf("unable to upload cluster config to kubernetes: %w", err)
		}
		log.Infoln("Save Proton-CLI config with updated ECeph TLS certificate succeeded")
	default:
		panic("unsupported migrateMode, this should never happen:" + migrateMode)
	}
	log.Debug("Migrate success")
	fmt.Printf("\033[1;37;42m%s\033[0m\n", "Migrate success")
	return nil
}
