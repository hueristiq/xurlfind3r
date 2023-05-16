package sources

type Source interface {
	Run(config Configuration, domain string) (URLs chan URL)
	Name() string
}
