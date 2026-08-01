package registry

import (
	"stackyrd/pkg/infrastructure"
	"sync"
	"time"
)

type Dependencies struct {
	components  map[string]interface{}
	mu          sync.RWMutex
	sealed      bool
	cachedAll   map[string]interface{}
	cacheExpiry time.Time
	cacheTTL    time.Duration
}

func NewDependencies() *Dependencies {
	return &Dependencies{
		components: make(map[string]interface{}),
		cacheTTL:   2 * time.Second,
	}
}

func (d *Dependencies) Set(name string, component interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sealed {
		return
	}
	d.components[name] = component
	d.cachedAll = nil
	d.cacheExpiry = time.Time{}
}

func (d *Dependencies) Seal() {
	d.mu.Lock()
	d.sealed = true
	d.mu.Unlock()
}

func (d *Dependencies) IsSealed() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sealed
}

func (d *Dependencies) Get(name string) (interface{}, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	comp, ok := d.components[name]
	return comp, ok
}

func typed[T any](d *Dependencies, name string) *T {
	comp, ok := d.Get(name)
	if !ok {
		return nil
	}
	v, ok := comp.(*T)
	if !ok {
		return nil
	}
	return v
}

func (d *Dependencies) Redis() *infrastructure.RedisManager {
	return typed[infrastructure.RedisManager](d, "redis")
}

func (d *Dependencies) Postgres() *infrastructure.PostgresConnectionManager {
	return typed[infrastructure.PostgresConnectionManager](d, "postgres")
}

func (d *Dependencies) Mongo() *infrastructure.MongoConnectionManager {
	return typed[infrastructure.MongoConnectionManager](d, "mongo")
}

func (d *Dependencies) Kafka() *infrastructure.KafkaManager {
	return typed[infrastructure.KafkaManager](d, "kafka")
}

func (d *Dependencies) Grafana() *infrastructure.GrafanaManager {
	return typed[infrastructure.GrafanaManager](d, "grafana")
}

func (d *Dependencies) MinIO() *infrastructure.MinIOManager {
	return typed[infrastructure.MinIOManager](d, "minio")
}

func (d *Dependencies) Cron() *infrastructure.CronManager {
	return typed[infrastructure.CronManager](d, "cron")
}

func (d *Dependencies) GetAll() map[string]interface{} {
	d.mu.RLock()
	if time.Now().Before(d.cacheExpiry) && d.cachedAll != nil {
		result := d.cachedAll
		d.mu.RUnlock()
		return result
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()
	// Re-check after acquiring write lock
	if time.Now().Before(d.cacheExpiry) && d.cachedAll != nil {
		return d.cachedAll
	}
	result := make(map[string]interface{}, len(d.components))
	for k, v := range d.components {
		result[k] = v
	}
	d.cachedAll = result
	d.cacheExpiry = time.Now().Add(d.cacheTTL)
	return result
}
