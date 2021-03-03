module kubesphere.io/edge-watcher

go 1.13

require (
	github.com/coreos/go-iptables v0.5.0
	github.com/emicklei/go-restful v2.9.6+incompatible
	github.com/emicklei/go-restful-openapi v1.4.1
	github.com/fsnotify/fsnotify v1.4.7
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/sony/sonyflake v1.0.0
	github.com/speps/go-hashids v2.0.0+incompatible
	github.com/stretchr/testify v1.4.0
	golang.org/x/text v0.3.2
	gopkg.in/yaml.v2 v2.2.4
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
)
