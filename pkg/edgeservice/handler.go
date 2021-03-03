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
	//"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"

	"gopkg.in/yaml.v2"

	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/emicklei/go-restful"

	k8sclient "kubesphere.io/edge-watcher/pkg/client/kubernetes"
)

const (
	KubeEdgeNamespace           = "kubeedge"
	KubeEdgeCloudCoreConfigName = "cloudcore"
	KubeEdgeTokenSecretName     = "tokensecret"
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

var k8sClient *kubernetes.Clientset

func InitK8sClient(kubeconfig string) {
	var err error
	k8sClient, err = k8sclient.NewK8sClient(kubeconfig)
	if err != nil {
		log.Println("Create K8s client failed", err)
	}
}

func EdgeNodeJoin(request *restful.Request, response *restful.Response) {
	nodeName := request.QueryParameter("node_name")
	nodeIP := request.QueryParameter("node_ip")

	//Validate Node name
	msgs := validation.NameIsDNSSubdomain(nodeName, false)
	if len(msgs) != 0 {
		log.Printf("Invalid node name: %s\n", msgs[0])
		response.AddHeader("Content-Type", "text/plain")
		errMsg := fmt.Sprintf("Invalid node name: %s", msgs[0])
		response.WriteErrorString(http.StatusInternalServerError, errMsg)
		return
	}

	//Validate IP address
	ip := net.ParseIP(nodeIP)
	if ip == nil {
		log.Printf("Invalid node IP: %s\n", nodeIP)
		response.AddHeader("Content-Type", "text/plain")
		errMsg := fmt.Sprintf("Invalid node IP: %s", nodeIP)
		response.WriteErrorString(http.StatusInternalServerError, errMsg)
		return
	}

	configMap, err := k8sClient.CoreV1().ConfigMaps(KubeEdgeNamespace).Get(KubeEdgeCloudCoreConfigName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Read cloudcore configmap error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}

	var cloudCoreConfig CloudCoreConfig
	err = yaml.Unmarshal([]byte(configMap.Data["cloudcore.yaml"]), &cloudCoreConfig)
	if err != nil {
		log.Printf("Unmarshal cloudcore configmap error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}

	secret, err := k8sClient.CoreV1().Secrets(KubeEdgeNamespace).Get(KubeEdgeTokenSecretName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Read cloudcore token secret error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}

	modules := cloudCoreConfig.Modules
	advertiseAddress := modules.CloudHub.AdvertiseAddress[0]
	webSocketPort := modules.CloudHub.WebSocket.Port
	quicPort := modules.CloudHub.Quic.Port
	certPort := modules.CloudHub.HTTPS.Port
	tunnelPort := modules.CloudStream.TunnelPort

	cmd := fmt.Sprintf("arch=$(uname -m) curl -O https://kubeedge.pek3b.qingstor.com/bin/v1.5.0/$arch/keadm && chmod +x keadm && ./keadm join --kubeedge-version=1.5.0 --cloudcore-ipport=%s:%d --quicport %d --certport %d --tunnelport %d --edgenode-name %s --edgenode-ip %s --token %s", advertiseAddress, webSocketPort, quicPort, certPort, tunnelPort, nodeName, nodeIP, string(secret.Data["tokendata"]))

	response.WriteHeaderAndEntity(http.StatusOK, cmd)
}
