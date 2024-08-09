package storage

type RelEntry struct {
	Name string
	HREF string
}

var registry = make([]*RelEntry, 0)

func AddRel(name string, href string) {
	registry = append(registry, &RelEntry{
		Name: name,
		HREF: href,
	})
}

func GetRels() []*RelEntry {
	return registry
}
