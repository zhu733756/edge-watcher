/*
Copyright 2021 The KubeSphere Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package edgeservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/emicklei/go-restful"

	k8sclient "kubesphere.io/edge-watcher/pkg/client/kubernetes"
)

type CloudCoreConfig struct {
	Modules *Modules `json:"modules,omitempty"`
}

type Modules struct {
	CloudHub    *CloudHub    `yaml:"cloudHub,omitempty"`
	CloudStream *CloudStream `yaml:"cloudStream,omitempty"`
}

type CloudHub struct {
	Quic             *CloudHubQUIC      `yaml:"quic,omitempty"`
	WebSocket        *CloudHubWebSocket `json:"websocket,omitempty"`
	HTTPS            *CloudHubHTTPS     `json:"https,omitempty"`
	AdvertiseAddress []string           `yaml:"advertiseAddress,omitempty"`
}

type CloudHubQUIC struct {
	Port uint32 `yaml:"port,omitempty"`
}

type CloudHubWebSocket struct {
	Port uint32 `yaml:"port,omitempty"`
}

type CloudHubHTTPS struct {
	Port uint32 `yaml:"port,omitempty"`
}

type CloudStream struct {
	TunnelPort uint32 `yaml:"tunnelPort,omitempty"`
}

type EdgeJoinResponse struct {
	Code    uint32 `json:"code,omitempty"`
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Data    string `json:"data,omitempty"`
}

const (
	KubeEdgeNamespace           = "kubeedge"
	KubeEdgeCloudCoreConfigName = "cloudcore"
	KubeEdgeTokenSecretName     = "tokensecret"
	StatusSucceeded             = "Succeeded"
	StatusFailure               = "Failure"
	EdgeWatcherConfig           = "edge-watcher-config"
)

var (
	availableRegions = map[string]bool{"zh": true, "en": true}
)

var k8sClient *kubernetes.Clientset

func InitK8sClient(kubeconfig string) {
	var err error
	k8sClient, err = k8sclient.NewK8sClient(kubeconfig)
	if err != nil {
		log.Println("Create K8s client failed", err)
	}
}

func getNodeInternalIP(node *corev1.Node) string {
	ipAddress := ""

	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" {
			ipAddress = address.Address
			break
		}
	}

	return ipAddress
}

func EdgeNodeJoin(request *restful.Request, response *restful.Response) {
	nodeName := request.QueryParameter("node_name")
	nodeIP := request.QueryParameter("node_ip")

	//Validate Node name
	msgs := validation.NameIsDNSSubdomain(nodeName, false)
	if len(msgs) != 0 {
		log.Printf("EdgeNodeJoin: Invalid node name: %s\n", msgs[0])
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusBadRequest)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusBadRequest,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Invalid node name: %s", msgs[0]),
		})
		return
	}

	//Validate IP address
	ip := net.ParseIP(nodeIP)
	if ip == nil {
		log.Printf("EdgeNodeJoin: Invalid node IP: %s\n", nodeIP)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusBadRequest)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusBadRequest,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Invalid node IP: %s", nodeIP),
		})
		return
	}

	//Check Node name and IP used
	nodeList, err := k8sClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Printf("EdgeNodeJoin: List nodes error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("List nodes error [+%v]", err),
		})
		return
	}

	nodeNames := make(map[string]bool, 0)
	nodeIPs := make(map[string]bool, 0)

	for _, n := range nodeList.Items {
		nodeNames[n.Name] = true
		nodeIPs[getNodeInternalIP(&n)] = true
	}

	_, ok := nodeNames[nodeName]
	if ok {
		log.Printf("EdgeNodeJoin: Node name %s in use\n", nodeName)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusBadRequest)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusBadRequest,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Node name %s in use", nodeName),
		})
		return
	}

	_, ok = nodeIPs[nodeIP]
	if ok {
		log.Printf("EdgeNodeJoin: Node IP %s in use\n", nodeIP)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusBadRequest)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusBadRequest,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Node IP %s in use", nodeIP),
		})
		return
	}

	// Get configmap for edgeNodeJoin
	region := os.Getenv("REGION")
	version := os.Getenv("VERSION")
	edgeWatcherConfig, err := k8sClient.CoreV1().ConfigMaps(KubeEdgeNamespace).Get(EdgeWatcherConfig, metav1.GetOptions{})
	if err == nil {
		version, _ = edgeWatcherConfig.Data["version"]
		region, _ = edgeWatcherConfig.Data["region"]
		if _, ok := availableRegions[region]; !ok || version == "" {
			msg := fmt.Sprintf("EdgeNodeJoin: Edge-watcher-config configured, but the given values were incorrect: version=%s, region=%s\n", version, region)
			log.Println(msg)
			response.AddHeader("Content-Type", "text/json")
			response.WriteHeader(http.StatusBadRequest)
			response.WriteAsJson(&EdgeJoinResponse{
				Code:    http.StatusBadRequest,
				Status:  StatusFailure,
				Message: msg,
			})
			return
		}
	}

	uri, ok := edgeWatcherConfig.Data["uri"]
	if !ok || uri == "" {
		if region == "zh" {
			uri = fmt.Sprintf("https://kubeedge.pek3b.qingstor.com/bin/%s/$arch/keadm-%s-linux-$arch.tar.gz", version, version)
		} else {
			uri = fmt.Sprintf("https://github.com/kubesphere/kubeedge/releases/download/%s-kubesphere/keadm-%s-linux-$arch.tar.gz", version, version)
		}
	}

	// Get configmap for cloudcore
	configMap, err := k8sClient.CoreV1().ConfigMaps(KubeEdgeNamespace).Get(KubeEdgeCloudCoreConfigName, metav1.GetOptions{})
	if err != nil {
		log.Printf("EdgeNodeJoin: Read cloudcore configmap error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Read cloudcore configmap error [+%v]", err),
		})
		return
	}

	var cloudCoreConfig CloudCoreConfig
	err = yaml.Unmarshal([]byte(configMap.Data["cloudcore.yaml"]), &cloudCoreConfig)
	if err != nil {
		log.Printf("EdgeNodeJoin: Unmarshal cloudcore configmap error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Unmarshal cloudcore configmap error [+%v]", err),
		})
		return
	}

	secret, err := k8sClient.CoreV1().Secrets(KubeEdgeNamespace).Get(KubeEdgeTokenSecretName, metav1.GetOptions{})
	if err != nil {
		log.Printf("EdgeNodeJoin: Read cloudcore token secret error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Read cloudcore token secret error [+%v]", err),
		})
		return
	}

	modules := cloudCoreConfig.Modules
	advertiseAddress := modules.CloudHub.AdvertiseAddress[0]
	webSocketPort := modules.CloudHub.WebSocket.Port
	quicPort := modules.CloudHub.Quic.Port
	certPort := modules.CloudHub.HTTPS.Port
	tunnelPort := modules.CloudStream.TunnelPort

	var cmd string
	if region == "zh" {
		cmd = fmt.Sprintf("arch=$(uname -m); curl -LO %s  && tar xvf keadm-%s-linux-$arch.tar.gz && chmod +x keadm && ./keadm join --kubeedge-version=%s --region=%s --cloudcore-ipport=%s:%d --quicport %d --certport %d --tunnelport %d --edgenode-name %s --edgenode-ip %s --token %s", uri, version, strings.ReplaceAll(version, "v", ""), region, advertiseAddress, webSocketPort, quicPort, certPort, tunnelPort, nodeName, nodeIP, string(secret.Data["tokendata"]))
	} else {
		cmd = fmt.Sprintf("arch=$(uname -m); if [ $arch == 'x86_64' ]; then arch='amd64'; fi; curl -LO %s  && tar xvf keadm-%s-linux-$arch.tar.gz && chmod +x keadm && ./keadm join --kubeedge-version=%s --region=%s --cloudcore-ipport=%s:%d --quicport %d --certport %d --tunnelport %d --edgenode-name %s --edgenode-ip %s --token %s", uri, version, strings.ReplaceAll(version, "v", ""), region, advertiseAddress, webSocketPort, quicPort, certPort, tunnelPort, nodeName, nodeIP, string(secret.Data["tokendata"]))
	}

	resp := EdgeJoinResponse{
		Code:   http.StatusOK,
		Status: StatusSucceeded,
		Data:   cmd,
	}
	bf := bytes.NewBufferString("")
	jsonEncoder := json.NewEncoder(bf)
	jsonEncoder.SetEscapeHTML(false)
	jsonEncoder.Encode(resp)

	response.AddHeader("Content-Type", "text/json")
	response.WriteHeader(http.StatusOK)
	response.Write(bf.Bytes())
}
