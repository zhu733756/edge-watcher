/*
Copyright 2020 The KubeSphere Authors.
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

package client

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var k8sClient *kubernetes.Clientset

func NewK8sClient(kubeconfig string) (*kubernetes.Clientset, error) {
	if k8sClient != nil {
		return k8sClient, nil
	}

	var config *rest.Config
	var err error

	if kubeconfig == "" {
		log.Printf("using in-cluster configuration\n")
		config, err = rest.InClusterConfig()
	} else {
		log.Printf("using configuration from '%s'\n", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		log.Printf("K8sClient create config error: %v\n", err)
		return k8sClient, err
	}

	k8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("K8sClient create client error: %v\n", err)
		return k8sClient, err
	}

	return k8sClient, nil
}
