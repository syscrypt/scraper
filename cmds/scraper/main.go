package main

import (
	"github.com/sirupsen/logrus"

	"github.com/syscrypt/scraper/plugins"
)

func main() {
	lg := logrus.New()
	allPlugins := plugins.CreatePlugins()

	for _, p := range allPlugins {
		p.SetLogger(lg.WithField("plugin", p.GetName()))

		_, err := p.Execute()
		if err != nil {
			lg.WithField("plugin", p.GetName()).Errorln(err)
			continue
		}
	}
}
