package datasource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-weathermap/internal/config"

	"math/big"

	"github.com/gosnmp/gosnmp"
)

type snmpCacheEntry struct {
	Value     *big.Int
	Timestamp time.Time
}

type SNMPClient struct {
	cache map[string]snmpCacheEntry // key: host:oid:interface
	mu    sync.Mutex
}

func NewSNMPClient() *SNMPClient {
	return &SNMPClient{
		cache: make(map[string]snmpCacheEntry),
	}
}

var globalSNMPClient *SNMPClient
var once sync.Once

func GetGlobalSNMPClient() *SNMPClient {
	once.Do(func() {
		globalSNMPClient = NewSNMPClient()
	})
	return globalSNMPClient
}

func (c *SNMPClient) Get(ctx context.Context, ds config.DataSourceConfig, metricIdentifier string) (interface{}, error) {
	host, _ := ds.Params["host"].(string)
	port, _ := ds.Params["port"].(int)
	community, _ := ds.Params["community"].(string)

	fmt.Printf("[SNMP DEBUG] Target=%s Port=%d Community=%s OID=%s\n", host, port, community, metricIdentifier)
	g := &gosnmp.GoSNMP{
		Target:    host,
		Port:      uint16(port),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
		Retries:   0,
	}
	if err := g.Connect(); err != nil {
		fmt.Printf("[SNMP DEBUG] Connect error: %v\n", err)
		return nil, fmt.Errorf("snmp connect error: %w", err)
	}
	defer func() {
		if err := g.Conn.Close(); err != nil {
			fmt.Printf("[SNMP DEBUG] Close connection error: %v\n", err)
		}
	}()

	result, err := g.Get([]string{metricIdentifier})
	if err != nil {
		fmt.Printf("[SNMP DEBUG] Get error: %v\n", err)
		return nil, fmt.Errorf("snmp get error: %w", err)
	}
	if len(result.Variables) == 0 {
		fmt.Printf("[SNMP DEBUG] No SNMP data for OID %s\n", metricIdentifier)
		return nil, fmt.Errorf("no SNMP data for OID %s", metricIdentifier)
	}
	val := gosnmp.ToBigInt(result.Variables[0].Value)
	fmt.Printf("[SNMP DEBUG] SNMP value for OID %s: %v\n", metricIdentifier, val)

	cacheKey := fmt.Sprintf("%s:%d:%s", host, port, metricIdentifier)
	c.mu.Lock()
	c.cache[cacheKey] = snmpCacheEntry{Value: val, Timestamp: time.Now()}
	c.mu.Unlock()

	return val.Int64(), nil
}
