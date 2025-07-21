package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-weathermap/internal/config"
	"go-weathermap/internal/datasource"
	"os"
)

const (
	SNMPPollerType          = "snmp"
	DefaultPollInterval     = 3 * time.Second
	SNMPTimeoutPollInterval = 10 * time.Second
)

type EmbeddedPoller struct {
	mu    sync.RWMutex
	cache map[string]int64
	tasks []dataPollTask
}

func (p *EmbeddedPoller) AddTask(task dataPollTask) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range p.tasks {
		if t.Key == task.Key {
			return
		}
	}
	p.tasks = append(p.tasks, task)
}

func (p *EmbeddedPoller) SetCache(key string, val int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[key] = val
}

func (p *EmbeddedPoller) GetCache(key string) (int64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	val, ok := p.cache[key]
	return val, ok
}

type Poller interface {
	AddTask(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string, interval time.Duration)
	Start()
	GetMetric(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string) interface{}
}

func CreatePoller(pollerType string) Poller { // Poller fabric
	switch pollerType {
	case SNMPPollerType:
		return NewSNMPPoller()
	case "mock":
		return NewMockPoller()
	case "zabbix":
		return NewZabbixPoller()
	case "prometheus":
		return NewPrometheusPoller()
	default:
		return nil
	}
}

// SNMP POLLER
type SNMPPoller struct {
	EmbeddedPoller
}

func NewSNMPPoller() *SNMPPoller {
	return &SNMPPoller{EmbeddedPoller{cache: make(map[string]int64)}}
}

func (p *SNMPPoller) AddTask(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string, interval time.Duration) {
	oids, ok := iface.Params["oids"].(map[string]interface{})
	if !ok {
		return
	}
	oid, ok := oids[metricName].(string)
	if !ok {
		return
	}

	host, _ := ds.Params["host"].(string)
	port, _ := ds.Params["port"].(int)
	community, _ := ds.Params["community"].(string)

	key := fmt.Sprintf("%s:%d:%s", host, port, oid)
	p.EmbeddedPoller.AddTask(dataPollTask{
		Host:             host,
		Port:             port,
		Community:        community,
		MetricIdentifier: oid,
		Key:              key,
		DS:               ds,
		Interval:         interval,
	})
}

func (p *SNMPPoller) Start() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, task := range p.tasks {
		go p.pollTask(task)
	}
}

func (p *SNMPPoller) pollTask(task dataPollTask) {
	snmpClient := datasource.GetGlobalSNMPClient()
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	var prevValue int64
	var prevTime time.Time

	for range ticker.C {
		valRaw, err := snmpClient.Get(context.Background(), task.DS, task.MetricIdentifier)
		if err != nil {
			fmt.Printf("[ERROR] SNMP Get failed for %s: %v\n", task.Key, err)
			continue
		}

		val, ok := valRaw.(int64)
		if !ok {
			fmt.Printf("[ERROR] SNMP value is not int64 for %s\n", task.Key)
			continue
		}

		if !prevTime.IsZero() {
			elapsed := time.Since(prevTime).Seconds()
			if elapsed > 0 {
				delta := val - prevValue
				if delta < 0 {
					delta += (1 << 32) // Counter wrap around for snmp 32 bit counter
				}
				bps := int64(float64(delta) / elapsed)
				p.SetCache(task.Key, bps)
			}
		}

		prevValue = val
		prevTime = time.Now()
	}
}

func (p *SNMPPoller) GetMetric(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string) interface{} {
	oids, ok := iface.Params["oids"].(map[string]interface{})
	if !ok {
		return nil
	}
	oid, ok := oids[metricName].(string)
	if !ok {
		return nil
	}

	host, _ := ds.Params["host"].(string)
	port, _ := ds.Params["port"].(int)
	key := fmt.Sprintf("%s:%d:%s", host, port, oid)
	val, _ := p.GetCache(key)
	return val
}

// ZABBIX POLLER
type ZabbixPoller struct {
}

func NewZabbixPoller() *ZabbixPoller {
	return &ZabbixPoller{}
}

func (p *ZabbixPoller) AddTask(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string, interval time.Duration) {
}
func (p *ZabbixPoller) Start() {}
func (p *ZabbixPoller) GetMetric(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string) interface{} {
	fmt.Println("[ZabbixPoller] Заглушка: всегда возвращает 0")
	return 0
}

// PROMETHEUS POLLER
type PrometheusPoller struct {
}

func NewPrometheusPoller() *PrometheusPoller {
	return &PrometheusPoller{}
}

func (p *PrometheusPoller) AddTask(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string, interval time.Duration) {
}
func (p *PrometheusPoller) Start() {}
func (p *PrometheusPoller) GetMetric(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string) interface{} {
	fmt.Println("[PrometheusPoller] Заглушка: всегда возвращает 0")
	return 0
}

// MOCK POLLER
type MockPoller struct {
	EmbeddedPoller
	client *datasource.MockClient
}

func NewMockPoller() *MockPoller {
	return &MockPoller{
		EmbeddedPoller: EmbeddedPoller{cache: make(map[string]int64)},
		client:         datasource.NewMockClient(),
	}
}

func (p *MockPoller) AddTask(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string, interval time.Duration) {
	key := fmt.Sprintf("%s:%s:%s", ds.Name, iface.Name, metricName)
	p.EmbeddedPoller.AddTask(dataPollTask{
		Host:             ds.Name,
		MetricIdentifier: metricName,
		Key:              key,
		DS:               ds,
		Interval:         interval,
	})
}

func (p *MockPoller) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			p.mu.RLock()
			tasks := make([]dataPollTask, len(p.tasks))
			copy(tasks, p.tasks)
			p.mu.RUnlock()

			traffic, err := p.client.GetTraffic(context.Background())
			if err != nil {
				continue
			}

			for _, task := range tasks {
				var val int64
				switch task.MetricIdentifier {
				case "in":
					val = traffic.InBytes
				case "out":
					val = traffic.OutBytes
				}
				p.SetCache(task.Key, val)
			}
		}
	}()
}

func (p *MockPoller) GetMetric(ds config.DataSourceConfig, iface config.InterfaceConfig, metricName string) interface{} {
	key := fmt.Sprintf("%s:%s:%s", ds.Name, iface.Name, metricName)
	val, _ := p.GetCache(key)
	return val
}

type dataPollTask struct {
	Host             string
	Port             int
	Community        string
	MetricIdentifier string
	Key              string // host:port:oid // ds:iface:metric
	DS               config.DataSourceConfig
	Interval         time.Duration
}

type DataSourceService struct {
	datasources map[string]config.DataSourceConfig
	pollers     map[string]Poller // key: snmp, zabbix, prometheus, mock, ...
}

func NewDataSourceService(datasources []config.DataSourceConfig) *DataSourceService {
	dsMap := make(map[string]config.DataSourceConfig)
	for _, ds := range datasources {
		dsMap[ds.Name] = ds
	}

	pollers := make(map[string]Poller)
	for _, ds := range datasources {
		if pollers[ds.Type] == nil {
			poller := CreatePoller(ds.Type)
			if poller == nil {
				fmt.Printf("[WARN] unknown poller type: %s\n", ds.Type)
				continue
			}
			pollers[ds.Type] = poller
		}
	}

	for _, ds := range datasources {
		poller, ok := pollers[ds.Type]
		if !ok {
			continue
		}
		for _, iface := range ds.Interfaces {
			metricNames := getMetricNames(ds, iface)
			for _, metricName := range metricNames {
				interval := DefaultPollInterval
				if ds.PollInterval > 0 {
					interval = time.Duration(ds.PollInterval) * time.Second
				}
				poller.AddTask(ds, iface, metricName, interval)
			}
		}
	}

	return &DataSourceService{
		datasources: dsMap,
		pollers:     pollers,
	}
}

func (s *DataSourceService) Start() {
	for _, p := range s.pollers {
		p.Start()
	}
}

func getMetricNames(ds config.DataSourceConfig, iface config.InterfaceConfig) []string {
	if ds.Type == SNMPPollerType {
		if oids, ok := iface.Params["oids"].(map[string]interface{}); ok {
			names := make([]string, 0, len(oids))
			for name := range oids {
				names = append(names, name)
			}
			return names
		}
	} else {
		if metrics, ok := iface.Params["metrics"].([]interface{}); ok {
			names := make([]string, 0, len(metrics))
			for _, m := range metrics {
				if name, ok := m.(string); ok {
					names = append(names, name)
				}
			}
			return names
		}
	}
	return nil
}

func (s *DataSourceService) GetInterfaceMetrics(ctx context.Context, dsName, ifaceName string, metrics []string) (map[string]interface{}, error) {
	ds, ok := s.datasources[dsName]
	if !ok {
		fmt.Printf("[DEBUG] datasource not found: %s\n", dsName)
		return nil, fmt.Errorf("datasource not found: %s", dsName)
	}
	var iface *config.InterfaceConfig
	for i := range ds.Interfaces {
		if ds.Interfaces[i].Name == ifaceName {
			iface = &ds.Interfaces[i]
			break
		}
	}
	if iface == nil {
		fmt.Printf("[DEBUG] interface not found: %s\n", ifaceName)
		return nil, fmt.Errorf("interface not found: %s", ifaceName)
	}
	result := make(map[string]interface{})
	pollerType := ds.Type
	if pollerType == "" {
		pollerType = SNMPPollerType
	}
	poller, ok := s.pollers[pollerType]
	if !ok {
		fmt.Printf("[DEBUG] poller for type %s not found\n", pollerType)
		return nil, fmt.Errorf("poller for type %s not found", pollerType)
	}
	fmt.Printf("[DEBUG] GetInterfaceMetrics: ds=%s iface=%s metrics=%v pollerType=%s\n", dsName, ifaceName, metrics, pollerType)
	for _, metric := range metrics {
		val := poller.GetMetric(ds, *iface, metric)
		fmt.Printf("[DEBUG] metric=%s val=%v\n", metric, val)
		result[metric] = val
	}
	return result, nil
}

func LoadAllDataSources(configDir string) ([]config.DataSourceConfig, error) {
	datasources := []config.DataSourceConfig{}
	parser := config.NewParser()
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) <= 5 || entry.Name()[len(entry.Name())-5:] != ".yaml" {
			continue
		}
		file, err := os.Open(configDir + "/" + entry.Name())
		if err != nil {
			continue
		}
		m, err := parser.ParseYAML(file)
		if err := file.Close(); err != nil {
			fmt.Printf("[WARN] failed to close file %s: %v\n", file.Name(), err)
		}
		if err == nil && m != nil {
			datasources = append(datasources, m.Datasources...)
		}
	}
	return datasources, nil
}
