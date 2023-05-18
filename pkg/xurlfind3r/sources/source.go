package sources

type Source interface {
	Run(config *Configuration) (URLs chan URL)
	Name() string
}
