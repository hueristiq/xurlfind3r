package sources

import (
	"github.com/signedsecurity/sigurlfind3r/pkg/session"
)

type URLs struct {
	Source string
	Value  string
}

type Source interface {
	Run(string, *session.Session, bool) chan URLs
	Name() string
}

type Keys struct {
	GitHub []string `json:"github"`
}
