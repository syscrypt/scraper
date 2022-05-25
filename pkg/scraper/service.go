package scraper

import (
	"github.com/syscrypt/scraper/pkg/model"
)

type Logger interface {
	Infoln(args ...interface{})
	Warnln(args ...interface{})

	Info(args ...interface{})
	Warn(args ...interface{})

	Error(args ...interface{})
	Errorln(args ...interface{})
}

/*
  The scraper interface defines the base interface of all
  additionally added scrapers
*/
type Scraper interface {
	Execute() ([]*model.Contact, error)
	GetName() string
	SetLogger(Logger)
}
