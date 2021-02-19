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

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	edgewatcherv1alpha1 "kubesphere.io/edge-watcher/api/v1alpha1"
)

var edgeWatcherClient *rest.RESTClient

func NewEdgeWatcherClient(kubeconfig string) (*rest.RESTClient, error) {
	if edgeWatcherClient != nil {
		return edgeWatcherClient, nil
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
		log.Printf("EdgeWatcherClient create config error: %v\n", err)
		return edgeWatcherClient, err
	}

	edgewatcherv1alpha1.AddToScheme(scheme.Scheme)

	crdConfig := *config
	crdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: edgewatcherv1alpha1.GroupVersion.Group, Version: edgewatcherv1alpha1.GroupVersion.Version}
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	edgeWatcherClient, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		log.Printf("EdgeWatcherClient create client error: %v\n", err)
		return edgeWatcherClient, err
	}

	return edgeWatcherClient, nil
}
