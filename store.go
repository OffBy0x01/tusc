package tusc

type Store interface {
	Get(fingerprint string) (string, bool)
	Set(fingerprint, url string)
	Delete(fingerprint string)
	Close()
}

type MemoryStore struct {
	m map[string]string
}

func NewMemoryStore() Store {
	return &MemoryStore{
		make(map[string]string),
	}
}

func (s *MemoryStore) Get(fingerprint string) (string, bool) {
	url, ok := s.m[fingerprint]
	return url, ok
}

func (s *MemoryStore) Set(fingerprint, url string) {
	s.m[fingerprint] = url
}

func (s *MemoryStore) Delete(fingerprint string) {
	delete(s.m, fingerprint)
}

func (s *MemoryStore) Close() {
	for k := range s.m {
		delete(s.m, k)
	}
}
