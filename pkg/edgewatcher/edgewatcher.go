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

package edgewatcher

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	edgewatcherv1alpha1 "kubesphere.io/edge-watcher/api/v1alpha1"
	edgewatcherclient "kubesphere.io/edge-watcher/pkg/client/edgewatcher"
	k8sclient "kubesphere.io/edge-watcher/pkg/client/kubernetes"
	"kubesphere.io/edge-watcher/pkg/constants"
)

type EdgeNodeStatus struct {
	LogRouted     bool
	MetricsRouted bool
}

type IPTablesStatus struct {
	Table           string
	Chain           string
	Jump            string
	Protocol        string
	SourceIP        string
	SourcePort      string
	DestinationIP   string
	DestinationPort string
	Valid           bool
}

type EdgeNode struct {
	Labels map[string]string `yaml:"labels"`
}

type SourceConfig struct {
	MetricsPort string `yaml:"metrics_port"`
	LogPort     string `yaml:"log_port"`
}

type DestinationConfig struct {
	Address string `yaml:"address"`
	Port    string `yaml:"port"`
}

type IPTablesRules struct {
	Name        string            `yaml:"name"`
	Source      SourceConfig      `yaml:"source"`
	Destination DestinationConfig `yaml:"destination"`
}

type EdgeWatcherConfig struct {
	EdgeNode      EdgeNode      `yaml:"edgenode"`
	IPTablesRules IPTablesRules `yaml:"iptablesrules"`
}

type EdgeWatcher struct {
	sync.RWMutex
	SignalCh              chan string
	K8sClient             *kubernetes.Clientset
	K8sClientStop         chan struct{}
	EdgeWatcherClient     *rest.RESTClient
	EdgeWatcherClientStop chan struct{}
	EdgeNodes             map[string]*EdgeNodeStatus
	IPTables              []*IPTablesStatus
	NeedSyncIPTables      bool

	Namespace string
	Config    EdgeWatcherConfig
}

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	_, err := os.Stat(inClusterNamespacePath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please run in-cluster")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %v", err)
	}

	// Load the namespace file and return its content
	namespace, err := ioutil.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %v", err)
	}
	return string(namespace), nil
}

const configFilePath = "/etc/edgewatcher/config/edgewatcher.yaml"

func loadConfig() (*EdgeWatcherConfig, error) {
	config, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Println("Read edge watcher config file error", err)
		return nil, err
	}

	var edgeWatcherConfig EdgeWatcherConfig
	err = yaml.Unmarshal(config, &edgeWatcherConfig)
	if err != nil {
		log.Println("Unmarshal edge watcher config file error", err)
		return nil, err
	}

	return &edgeWatcherConfig, nil
}

func NewEdgeWatcher(kubeconfig string) (*EdgeWatcher, error) {
	//Read config
	edgeWatcherConfig, err := loadConfig()
	if err != nil {
		log.Println("Load edge watcher config error", err)
		return nil, err
	}

	log.Printf("Edge watcher config %v\n", edgeWatcherConfig)

	//Create clients
	k8sClient, err := k8sclient.NewK8sClient(kubeconfig)
	if err != nil {
		log.Println("Create K8s client failed", err)
		return nil, err
	}

	edgeWatcherClient, err := edgewatcherclient.NewEdgeWatcherClient(kubeconfig)
	if err != nil {
		log.Println("Create IPTables Operator client failed", err)
		return nil, err
	}

	namespace, err := getInClusterNamespace()
	if err != nil {
		log.Println("Get incluster namespace failed", err)
		return nil, err
	}

	log.Printf("Working Namespace %s\n", namespace)

	er := &EdgeWatcher{
		SignalCh:              make(chan string, 10),
		K8sClient:             k8sClient,
		K8sClientStop:         make(chan struct{}, 10),
		EdgeWatcherClient:     edgeWatcherClient,
		EdgeWatcherClientStop: make(chan struct{}, 10),
		EdgeNodes:             make(map[string]*EdgeNodeStatus),
		IPTables:              make([]*IPTablesStatus, 0),
		NeedSyncIPTables:      false,
		Namespace:             namespace,
		Config:                *edgeWatcherConfig,
	}

	return er, nil
}

func (er *EdgeWatcher) getIPTablesRules() {
	result := edgewatcherv1alpha1.IPTablesRules{}
	err := er.EdgeWatcherClient.
		Get().
		Namespace(er.Namespace).
		Resource(constants.IPTablesRulesResource).
		Name(er.Config.IPTablesRules.Name).
		Do().
		Into(&result)

	er.clearIPTablesStatus()

	if err != nil {
		log.Printf("Get IPTablesRules %s error %+v\n", er.Config.IPTablesRules.Name, err)
		return
	}

	er.Lock()
	defer er.Unlock()

	for _, rule := range result.Spec.Rules {
		ipTablesStatus := &IPTablesStatus{
			Table:           rule.Table,
			Chain:           rule.Chain,
			Jump:            rule.Jump,
			Protocol:        rule.Protocol,
			SourceIP:        rule.SourceIP,
			SourcePort:      rule.SourcePort,
			DestinationIP:   rule.DestinationIP,
			DestinationPort: rule.DestinationPort,
			Valid:           false,
		}

		er.IPTables = append(er.IPTables, ipTablesStatus)
	}
}

func (er *EdgeWatcher) watchIPTablesRules() {
	watchlist := cache.NewListWatchFromClient(
		er.EdgeWatcherClient,
		constants.IPTablesRulesResource,
		er.Namespace,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchlist,
		&edgewatcherv1alpha1.IPTablesRules{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Printf("IPTables rules added: %v\n", obj)
				er.Lock()
				er.NeedSyncIPTables = true
				er.Unlock()
			},
			DeleteFunc: func(obj interface{}) {
				log.Printf("IPTables rules deleted: %v\n", obj)
				er.Lock()
				er.NeedSyncIPTables = true
				er.Unlock()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				log.Printf("IPTables rules updated %v\n", newObj)
				er.Lock()
				er.NeedSyncIPTables = true
				er.Unlock()
			},
		},
	)
	go controller.Run(er.EdgeWatcherClientStop)

	<-er.EdgeWatcherClientStop

	log.Println("Watch IPTablesRules stop")
}

func (er *EdgeWatcher) clearIPTablesStatus() {
	er.Lock()
	er.IPTables = make([]*IPTablesStatus, 0)
	er.Unlock()
}

func (er *EdgeWatcher) generateIPTablesRules() *edgewatcherv1alpha1.IPTablesRules {
	rules := make([]edgewatcherv1alpha1.IPTablesRule, 0)

	for ipAddress, _ := range er.EdgeNodes {
		ruleLog := edgewatcherv1alpha1.IPTablesRule{
			Table:           constants.IPTablesTableNat,
			Chain:           constants.IPTablesChainOutput,
			Jump:            constants.IPTablesJumpDnat,
			Protocol:        constants.IPTablesProtocolTcp,
			SourceIP:        ipAddress,
			SourcePort:      er.Config.IPTablesRules.Source.LogPort,
			DestinationIP:   er.Config.IPTablesRules.Destination.Address,
			DestinationPort: er.Config.IPTablesRules.Destination.Port,
		}
		rules = append(rules, ruleLog)
		ruleMetrics := edgewatcherv1alpha1.IPTablesRule{
			Table:           constants.IPTablesTableNat,
			Chain:           constants.IPTablesChainOutput,
			Jump:            constants.IPTablesJumpDnat,
			Protocol:        constants.IPTablesProtocolTcp,
			SourceIP:        ipAddress,
			SourcePort:      er.Config.IPTablesRules.Source.MetricsPort,
			DestinationIP:   er.Config.IPTablesRules.Destination.Address,
			DestinationPort: er.Config.IPTablesRules.Destination.Port,
		}
		rules = append(rules, ruleMetrics)
	}

	ipTablesRules := &edgewatcherv1alpha1.IPTablesRules{
		TypeMeta: metav1.TypeMeta{
			APIVersion: constants.IPTablesOperatorAPIVersion,
			Kind:       constants.IPTablesRulesKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      er.Config.IPTablesRules.Name,
			Namespace: er.Namespace,
		},
		Spec: edgewatcherv1alpha1.IPTablesRulesSpec{
			Rules: rules,
		},
	}

	return ipTablesRules
}

func (er *EdgeWatcher) createIPTablesRules() error {
	ipTablesRules := er.generateIPTablesRules()

	result := edgewatcherv1alpha1.IPTablesRules{}
	err := er.EdgeWatcherClient.
		Post().
		Namespace(er.Namespace).
		Resource(constants.IPTablesRulesResource).
		Body(ipTablesRules).
		Do().
		Into(&result)

	if err != nil {
		log.Println("Create IPTablesRules error", err)
	}

	return err
}

func (er *EdgeWatcher) updateIPTablesRules() error {
	ipTablesRules := er.generateIPTablesRules()

	ipTablesRulesJsonBytes, err := json.Marshal(ipTablesRules)
	if err != nil {
		log.Printf("Marshal IPTablesRules error %+v\n", err)
		return err
	}

	result := edgewatcherv1alpha1.IPTablesRules{}
	err = er.EdgeWatcherClient.
		Patch(types.MergePatchType).
		Namespace(er.Namespace).
		Resource(constants.IPTablesRulesResource).
		Name(er.Config.IPTablesRules.Name).
		Body(ipTablesRulesJsonBytes).
		Do().
		Into(&result)

	if err != nil {
		log.Println("Update IPTablesRules error", err)
	}

	return err
}

func (er *EdgeWatcher) watchEdgeNodes() {
	labelSelector := labels.Set(er.Config.EdgeNode.Labels).AsSelector()

	//Init Informer with label selector
	factory := informers.NewSharedInformerFactoryWithOptions(er.K8sClient, 0, informers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.LabelSelector = labelSelector.String()
	}))
	nodeInformer := factory.Core().V1().Nodes()
	informer := nodeInformer.Informer()
	defer runtime.HandleCrash()

	er.clearEdgeNodesStatus()

	// Start Informer
	go factory.Start(er.K8sClientStop)

	// Sync from APIServer
	if !cache.WaitForCacheSync(er.K8sClientStop, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("WatchEdgeNodes timed out waiting for caches to sync"))
		return
	}

	// Register edge node add delete handler
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    er.onEdgeNodeAdd,
		DeleteFunc: er.onEdgeNodeDel,
	})

	<-er.K8sClientStop

	log.Println("Watch edge nodes stop")
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

func (er *EdgeWatcher) onEdgeNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	ipAddress := getNodeInternalIP(node)
	log.Println("Add edge node:", node.Name, ipAddress)

	er.Lock()
	defer er.Unlock()

	_, ok := er.EdgeNodes[ipAddress]
	if ok {
		log.Printf("Added edge node %s(%s) already exist\n", node.Name, ipAddress)
		return
	}

	er.EdgeNodes[ipAddress] = &EdgeNodeStatus{
		LogRouted:     false,
		MetricsRouted: false,
	}

	er.NeedSyncIPTables = true
}

func (er *EdgeWatcher) onEdgeNodeDel(obj interface{}) {
	node := obj.(*corev1.Node)
	ipAddress := getNodeInternalIP(node)
	log.Println("Delete edge node:", node.Name, ipAddress)

	er.Lock()
	defer er.Unlock()

	_, ok := er.EdgeNodes[ipAddress]
	if !ok {
		log.Printf("Deleted edge node %s(%s) not exist\n", node.Name, ipAddress)
		return
	}

	delete(er.EdgeNodes, ipAddress)

	er.NeedSyncIPTables = true
}

func (er *EdgeWatcher) clearEdgeNodesStatus() {
	er.Lock()
	for ipAddress, _ := range er.EdgeNodes {
		delete(er.EdgeNodes, ipAddress)
	}
	er.Unlock()
}

func (er *EdgeWatcher) syncIPTables() {
	er.Lock()
	defer er.Unlock()
	ipTablesAllValid := true

	// Clear edge node status to prepare a full check
	for ipAddress, _ := range er.EdgeNodes {
		er.EdgeNodes[ipAddress].LogRouted = false
		er.EdgeNodes[ipAddress].MetricsRouted = false
	}

	// Check IPTablesRules
	for i, ipTablesStatus := range er.IPTables {
		//Check basic info valid
		if !(ipTablesStatus.Table == constants.IPTablesTableNat && ipTablesStatus.Chain == constants.IPTablesChainOutput && ipTablesStatus.Jump == constants.IPTablesJumpDnat && ipTablesStatus.Protocol == constants.IPTablesProtocolTcp && ipTablesStatus.DestinationIP == er.Config.IPTablesRules.Destination.Address && ipTablesStatus.DestinationPort == er.Config.IPTablesRules.Destination.Port) {
			log.Printf("IPTablesRule %d basic info invalid\n", i)
			ipTablesStatus.Valid = false
			ipTablesAllValid = false
			continue
		}
		//Check source IP valid
		edgeNode, ok := er.EdgeNodes[ipTablesStatus.SourceIP]
		if !ok {
			log.Printf("IPTablesRule %d source IP invalid\n", i)
			ipTablesStatus.Valid = false
			ipTablesAllValid = false
			continue
		}

		if ipTablesStatus.SourcePort == er.Config.IPTablesRules.Source.LogPort {
			if !edgeNode.LogRouted {
				edgeNode.LogRouted = true
				ipTablesStatus.Valid = true
			} else {
				log.Printf("IPTablesRule %d duplicate log rule\n", i)
				ipTablesStatus.Valid = false
				ipTablesAllValid = false
			}
		} else if ipTablesStatus.SourcePort == er.Config.IPTablesRules.Source.MetricsPort {
			if !edgeNode.MetricsRouted {
				edgeNode.MetricsRouted = true
				ipTablesStatus.Valid = true
			} else {
				log.Printf("IPTablesRule %d duplicate metrics rule\n", i)
				ipTablesStatus.Valid = false
				ipTablesAllValid = false
			}
		} else {
			log.Printf("IPTablesRule %d wrong port\n", i)
			ipTablesStatus.Valid = false
			ipTablesAllValid = false
		}
	}

	// Check edge nodes all routed
	for ipAddress, _ := range er.EdgeNodes {
		if !er.EdgeNodes[ipAddress].LogRouted {
			log.Printf("Edge node %s log not routed\n", ipAddress)
			ipTablesAllValid = false
		}
		if !er.EdgeNodes[ipAddress].MetricsRouted {
			log.Printf("Edge node %s metrics not routed\n", ipAddress)
			ipTablesAllValid = false
		}
	}

	if ipTablesAllValid {
		log.Println("IPTablesRules match edge nodes, no need to sync")
		er.NeedSyncIPTables = false
		return
	}

	//Sync IPTablesRules according to edge nodes
	log.Println("Syncing IPTablesRules")
	syncAllSuccess := true

	err := er.updateIPTablesRules()
	if err != nil {
		log.Printf("Update IPTablesRules failed %+v\n", err)
		err = er.createIPTablesRules()
		if err != nil {
			log.Printf("Create IPTablesRules failed %+v\n", err)
			syncAllSuccess = false
		} else {
			log.Println("Create IPTablesRules success")
		}
	} else {
		log.Println("Update IPTablesRules success")
	}

	if syncAllSuccess {
		er.NeedSyncIPTables = false
	}
}

func (er *EdgeWatcher) Stop() {
	er.SignalCh <- "Stop"
	close(er.K8sClientStop)
	close(er.EdgeWatcherClientStop)
}

func (er *EdgeWatcher) Run() {
	go er.watchEdgeNodes()
	go er.watchIPTablesRules()

	timer := time.NewTicker(time.Second * 15)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			if er.NeedSyncIPTables {
				er.getIPTablesRules()
				er.syncIPTables()
			}
		case operation := <-er.SignalCh:
			switch operation {
			case "Stop":
				log.Printf("EdgeWatcher stop\n")
			}
		}
	}
}
