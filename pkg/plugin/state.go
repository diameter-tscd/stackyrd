package plugin

import "sync"

type StateBag interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	Clear()
	Keys() []string
}

type pluginStateBag struct {
	data sync.Map
}

func (s *pluginStateBag) Get(key string) (interface{}, bool) {
	return s.data.Load(key)
}

func (s *pluginStateBag) Set(key string, value interface{}) {
	s.data.Store(key, value)
}

func (s *pluginStateBag) Delete(key string) {
	s.data.Delete(key)
}

func (s *pluginStateBag) Clear() {
	s.data.Range(func(key, _ interface{}) bool {
		s.data.Delete(key)
		return true
	})
}

func (s *pluginStateBag) Keys() []string {
	var keys []string
	s.data.Range(func(key, _ interface{}) bool {
		if k, ok := key.(string); ok {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

var _ StateBag = (*pluginStateBag)(nil)
