package plugins

import (
	"github.com/syscrypt/scraper/pkg/scraper"
)

func CreatePlugins() []scraper.Scraper {
	return []scraper.Scraper{
		CreateSpravkaruPlugin(),
	}
}
