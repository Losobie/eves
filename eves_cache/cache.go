package eves_cache

type Cache struct {
	charMap map[int]string
}

func NewCache() *Cache {
	return &Cache{make(map[int]string)}
}

func (cache *Cache) Lookup(charName string) (*int, error) {
	
}
