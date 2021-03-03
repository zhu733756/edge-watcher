package ipset

import (
	"net"
	"os/exec"
	"strings"
	"sync"

	"k8s.io/klog"
)

// IPSet represents a struct for named ipset.
type Ipset struct {
	// Name is the set name.
	Name string
	// a map to store legal ips built from clients.
	UsedIpMap map[string]bool
	// a read-write lock
	sync.RWMutex
}

// NewIpset represents a inited Ipset pointer
func NewIpset(name string) *Ipset {
	if name == "" {
		name = "default"
	}
	return &Ipset{
		Name:      name,
		UsedIpMap: make(map[string]bool),
	}
}

// CheckIp checks if the given IPs is valid.
func checkIP(ip string) bool {
	if net.ParseIP(ip) == nil {
		return false
	}
	return true
}

// NotInArpTable method checks if the given ips are on the node.
// Todo: use singeFlight to avoid arp cache breakdown;
func (e *Ipset) NotInArpTable(ips ...string) bool {
	number := len(ips)
	if number == 0 {
		klog.Errorf("Cannot get a ip for %v", ips)
		return false
	}

	cmd := exec.Command("bash", "-c", "arp -a | awk  '{print $2}' | sed -e \"s/[()]//g\"")
	out, err := cmd.Output()
	if err != nil {
		klog.Errorf("Error exec cmd:%v %v (%v)", cmd, err, out)
		return false
	}

	results := make(map[string]bool)
	for _, aip := range strings.Split(string(out), "\n") {
		if _, ok := results[aip]; !ok && aip != "" {
			results[aip] = true
		}
	}

	e.RLock()
	for _, uip := range ips {
		if _, ok := results[uip]; ok {
			klog.Errorf("Ip already exsits in arp cache: %v", uip)
			e.RUnlock()
			return false
		}
	}
	e.RUnlock()
	return true
}

// Get method tries to get the ip
func (e *Ipset) Get(ip string) bool {
	e.RLock()
	_, ok := e.UsedIpMap[ip]
	e.RUnlock()
	return ok
}

// Set method represents set a ip to the named map
// Set method should be after Get method
func (e *Ipset) Set(ip string) bool {
	if ok := checkIP(ip); !ok {
		klog.Errorf("Error parsing ip address(%v) for ipset %v", ip, e.Name)
		return false
	}
	e.Lock()
	e.UsedIpMap[ip] = true
	e.Unlock()
	klog.Info("Successfully stored a ip ", ip, " to ipset ", e.Name)
	return true
}

//SetOrGet represents calling set method if the ip not exsits
func (e *Ipset) SetOrGet(ip string) bool {
	if ok := checkIP(ip); !ok {
		klog.Errorf("Error parsing ip address(%v) for ipset %v", ip, e.Name)
		return false
	}
	if ok := e.Get(ip); ok {
		klog.Errorf("ip already exsits.")
		return false
	}

	e.Lock()
	e.UsedIpMap[ip] = true
	e.Unlock()
	klog.Info("Successfully stored a ip ", ip, " to ipset ", e.Name)
	return true
}

// GetWithChan will write bool values to the chan
func (e *Ipset) GetWithChan(ip string, ch chan bool) {
	ch <- e.Get(ip)
}

// GetMany
func (e *Ipset) GetMany(ips ...string) bool {
	number := len(ips)
	if number == 0 {
		klog.Errorf("Cannot get a ip for %v", ips)
		return false
	}

	ch := make(chan bool, number)
	for _, ip := range ips {
		go e.GetWithChan(ip, ch)
	}

	flag := true
	for i := 0; i < number; i++ {
		flag = <-ch
		if !flag {
			return false
		}
	}
	return true
}

// SetMany
func (e *Ipset) SetMany(ips ...string) bool {
	number := len(ips)
	if number == 0 {
		klog.Errorf("Cannot get a ip for %v", ips)
		return false
	}

	var wg sync.WaitGroup
))
	for _, ip := range ips {
		wg.Add(1)
		go func(ip string, wg *sync.WaitGroup)  {
			if ok := checkIP(ip); !ok {
				klog.Errorf("Error parsing ip address(%v) for ipset %v", ip, e.Name)
				return 
			}
			e.Lock()
			e.UsedIpMap[ip] = true
			e.Unlock()
			klog.Info("Successfully stored a ip ", ip, " to ipset ", e.Name)
			wg.Done()
		}(ip, &wg)
	}
	wg.Wait()
	return true
}

// SetManyIFNotExsits
func (e *Ipset) SetManyOrGetMany(ips ...string) bool {
	number := len(ips)
	if number == 0 {
		klog.Errorf("Cannot get a ip for %v", ips)
		return false
	}

	if ok := e.GetMany(ips...); ok {
		return false
	}

	return e.SetMany(ips...)
}

// DeleteOne
func (e *Ipset) DeleteOne(ip string) bool {
	if ok := checkIP(ip); !ok {
		klog.Errorf("Error parsing ip address(%v) for ipset %v", ip, e.Name)
		return false
	}
	if ok := e.Get(ip); ok {
		klog.Errorf("ip already exsits.")
		return false
	}

	e.Lock()
	delete(e.UsedIpMap, ip)
	e.Unlock()
	klog.Info("Successfully deleted a ip ", ip, " from ipset ", e.Name)
	return true
}

// DeleteAll
func (e *Ipset) DeleteAll() bool {
	m := make(map[string]bool)
	e.Lock()
	e.UsedIpMap = m
	e.Unlock()
	klog.Info("Successfully deleted all ips from ipset ", e.Name)
	return true
}

// List method gets a list of already used ips
func (e *Ipset) List() []string {
	results := make([]string, 0)
	for k, _ := range e.UsedIpMap {
		results = append(results, k)
	}
	return results
}
