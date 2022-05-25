package plugins

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"

	"github.com/syscrypt/scraper/pkg/model"
	"github.com/syscrypt/scraper/pkg/scraper"
)

const (
	pluginName = "Spravkaru"
)

type Spravkaru struct {
	lg           scraper.Logger
	ignoredNames []string
}

func CreateSpravkaruPlugin() *Spravkaru {
	ignore := []string{"проезд", "ул", "пер", "д", "Ул"}
	return &Spravkaru{
		ignoredNames: ignore,
	}
}

func (p *Spravkaru) SetLogger(lg scraper.Logger) {
	p.lg = lg
}

const (
	engHost        = "http://english.spravkaru.net"
	rusHost        = "http://spra.vkaru.net"
	zipCodeBaseUrl = "/post/russia"
	delay          = 500
)

func (p *Spravkaru) Execute() ([]*model.Contact, error) {
	cityInfos, err := p.parsePhonePrefixes()
	if err != nil {
		return nil, err
	}

	fileName := pluginName + "_contacts.json"
	fileNameTmp := fileName + ".tmp"

	for _, cityInfo := range cityInfos {
		p.lg.Infoln("retreiving information for city: " + cityInfo.City + " (" + cityInfo.English + "), with phone prefix: " + cityInfo.Phone + ", and page index of " + cityInfo.Index)

		postalCodes, err := p.getPostalCodes(cityInfo.Index)
		if err != nil {
			return nil, err
		}

		p.lg.Infoln("fetching streets")
		resp, err := http.Get(engHost + "/streets/7/" + cityInfo.Phone)
		if err != nil {
			return nil, err
		}
		node, err := htmlquery.Parse(resp.Body)
		if err != nil {
			return nil, err
		}

		options := htmlquery.Find(node, "/html/body/div[2]/div/div[4]/div[2]/p[3]/a")
		retContacts, err := p.parseStreets(options, postalCodes, cityInfo.Phone)
		if err != nil {
			p.lg.Errorln("error while parsing streets of city \"\"")
			continue
		}

		cityContact := &model.CityContacts{
			CityNameRussian: cityInfo.City,
			CityNameEnglish: cityInfo.English,
			Contacts:        retContacts,
		}

		j, err := json.MarshalIndent(cityContact, "", "  ")
		if err != nil {
			p.lg.Errorln(err)
			continue
		}
		cityFileName := cityContact.CityNameEnglish + "_" + fileName
		cityFileNameTmp := cityContact.CityNameEnglish + "_" + fileNameTmp

		err = ioutil.WriteFile(cityFileNameTmp, j, 0644)
		if err != nil {
			p.lg.Errorln(err)
			os.Remove(cityFileNameTmp)
			continue
		}

		err = os.Rename(cityFileNameTmp, cityFileName)
		if err != nil {
			p.lg.Errorln(err)
		}
	}

	p.lg.Infoln("finished plugin \"" + pluginName + "\" successfully")
	return nil, err
}

func (p *Spravkaru) parsePhonePrefixes() ([]*model.CityInfo, error) {
	ret := []*model.CityInfo{}

	resp, err := http.Get(rusHost)
	if err != nil {
		return nil, err
	}
	root, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	resp, err = http.Get(engHost)
	if err != nil {
		return nil, err
	}
	rootEng, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	phoneRows := htmlquery.Find(root, "/html/body/div[2]/div/div[4]/div/div[3]/div")
	phoneRowsEng := htmlquery.Find(rootEng, "/html/body/div[2]/div/div[4]/div/div[3]/div")

	cityPhoneInfosRus, err := p.parseSinglePhonePrefix(rusHost, phoneRows)
	if err != nil {
		return nil, err
	}

	cityPhoneInfosEng, err := p.parseSinglePhonePrefix(engHost, phoneRowsEng)
	if err != nil {
		return nil, err
	}

	for k, v := range cityPhoneInfosEng {
		if cityPhoneInfosRus[k] == nil {
			continue
		}

		cityPhoneInfosRus[k].English = v.City
		ret = append(ret, cityPhoneInfosRus[k])
	}

	return ret, nil
}

func (p *Spravkaru) parseStreets(streets []*html.Node, postalCodes map[string]string, phonePrefix string) ([]*model.ContactInfo, error) {
	retContacts := []*model.ContactInfo{}
	p.lg.Infoln("parsing", len(streets), "streets")
	for _, v := range streets {
		href := htmlquery.SelectAttr(v, "href")
		if href == "" {
			continue
		}

		p.lg.Infoln("fetching street under url: " + href)
		resp, err := http.Get(engHost + href)
		if err != nil {
			p.lg.Errorln("error while fetching buildings of street with url: " + href + ", err: " + err.Error())
			continue
		}
		buildings, err := htmlquery.Parse(resp.Body)
		if err != nil {
			p.lg.Errorln("error while parsing buildings of street with url: " + href + ", err: " + err.Error())
			continue
		}
		buildingNodes := htmlquery.Find(buildings, "/html/body/div[2]/div/div[4]/div[2]/a")
		contacts, err := p.parseBuildingPage(postalCodes, buildingNodes, buildings, href)
		if err != nil {
			p.lg.Errorln("error while parsing addresses of street with url: " + href + ", err: " + err.Error())
		}
		retContacts = append(retContacts, contacts...)

		time.Sleep(delay * time.Millisecond)
	}
	return retContacts, nil
}

func (p *Spravkaru) parseBuildingPage(postalCodes map[string]string, buildingNodes []*html.Node, buildings *html.Node, streetHref string) ([]*model.ContactInfo, error) {
	var contacts []*model.ContactInfo
	if len(buildingNodes) == 0 {
		p.lg.Infoln("only small entry count found, continue direct address parsing...")
		resp, err := http.Get(rusHost + streetHref)
		if err != nil {
			return nil, err
		}
		buildingsRus, err := htmlquery.Parse(resp.Body)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, p.getAddresses(postalCodes, buildings, buildingsRus)...)
	}

	for _, b := range buildingNodes {
		href := htmlquery.SelectAttr(b, "href")
		p.lg.Infoln("fetching building page 1: " + href)

		addressPage, addressPageRus, err := p.getEngRusAddressPages(href)
		if err != nil {
			p.lg.Errorln("error fetching russian and english address page, retrying with next url...")
			continue
		}
		contacts = append(contacts, p.getAddresses(postalCodes, addressPage, addressPageRus)...)

		pageNodes := htmlquery.Find(addressPage, "/html/body/div[2]/div/div[4]/div[2]/ul[1]/li")
		for i, n := range pageNodes {
			if i == 0 {
				continue
			}

			if i == len(pageNodes)-1 {
				break
			}

			href = htmlquery.SelectAttr(n, "href")
			p.lg.Infoln("fetching address page nr", i+1, "with query:", href)
			addressPage, addressPageRus, err = p.getEngRusAddressPages(href)
			if err != nil {
				return contacts, err
			}
			contacts = append(contacts, p.getAddresses(postalCodes, addressPage, addressPageRus)...)
		}
	}
	return contacts, nil
}

func (p *Spravkaru) getEngRusAddressPages(href string) (*html.Node, *html.Node, error) {
	resp, err := http.Get(engHost + href)
	if err != nil {
		return nil, nil, err
	}
	addressPage, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	resp, err = http.Get(rusHost + href)
	if err != nil {
		return nil, nil, err
	}
	addressPageRus, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return addressPage, addressPageRus, nil
}

func (p *Spravkaru) getPostalCodes(postalIndex string) (map[string]string, error) {
	ret := make(map[string]string)
	resp, err := http.Get(rusHost + zipCodeBaseUrl + "/" + postalIndex)
	if err != nil {
		return nil, err
	}

	root, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	allCodeTabs := htmlquery.Find(root, "/html/body/div[2]/div/div[4]/div[2]/div[1]/div")
	for _, tab := range allCodeTabs {
		allCodes := htmlquery.Find(tab, "/table/tbody/tr")
		for _, code := range allCodes {
			addr := strings.TrimSpace(htmlquery.InnerText(htmlquery.FindOne(code, "/td[1]")))
			zip := strings.TrimSpace(htmlquery.InnerText(htmlquery.FindOne(code, "/td[2]")))
			if addr != "" && zip != "" {
				ret[addr] = zip
			}
		}
	}

	return ret, nil
}

func (p *Spravkaru) getAddresses(zipCodes map[string]string, addressPage *html.Node, addressPageRussian *html.Node) []*model.ContactInfo {
	var contacts []*model.ContactInfo

	addresses := htmlquery.Find(addressPage, "/html/body/div[2]/div/div[4]/div[2]/table/tbody/tr")
	addressesRus := htmlquery.Find(addressPageRussian, "/html/body/div[2]/div/div[4]/div[2]/table/tbody/tr")
	for i, address := range addresses {
		name := htmlquery.InnerText(htmlquery.FindOne(address, "/td[2]/a"))
		nameRus := htmlquery.InnerText(htmlquery.FindOne(addressesRus[i], "/td[2]/a"))
		streetRus := strings.TrimSpace(htmlquery.InnerText(htmlquery.FindOne(addressesRus[i], "/td[3]/a[1]")))
		streetRus = strings.ReplaceAll(streetRus, ".", "")
		streetRus = strings.ReplaceAll(streetRus, ",", "")

		appt := htmlquery.InnerText(htmlquery.FindOne(address, "/td[3]"))
		apptRus := htmlquery.InnerText(htmlquery.FindOne(addressesRus[i], "/td[3]"))

		names := strings.Split(name, " ")
		if len(names) <= 1 {
			continue
		}

		namesRus := strings.Split(nameRus, " ")
		if len(namesRus) <= 1 {
			continue
		}

		zipCode := p.extendedStringSearch(zipCodes, streetRus)
		if zipCode == "" {
			continue
		}

		contact := &model.ContactInfo{
			ZipCode: zipCode,
			English: &model.Contact{
				FirstName: strings.Join(names[1:], " "),
				LastName:  names[0],
				Address:   appt,
			},
			Russian: &model.Contact{
				FirstName: strings.Join(namesRus[1:], " "),
				LastName:  namesRus[0],
				Address:   apptRus,
			},
		}
		contacts = append(contacts, contact)
	}
	return contacts
}

func (p *Spravkaru) extendedStringSearch(stringMap map[string]string, search string) string {
	str := ""
	str = stringMap[search]

	if str == "" {
		splittet := strings.Split(search, " ")
		if len(splittet) > 0 {
			maxEntropy := 0
			candidate := ""
			for k, v := range stringMap {
				entropy := 0
				for i := 0; i < len(splittet); i++ {
					part := strings.TrimSpace(splittet[i])
					part = strings.ReplaceAll(part, "(", "")
					part = strings.ReplaceAll(part, ")", "")
					part = strings.ReplaceAll(part, ".", "")
					part = strings.ReplaceAll(part, ",", "")

					ignore := false
					for _, s := range p.ignoredNames {
						ignore = part == s
						if ignore {
							break
						}
					}
					if ignore || len(part) <= 2 {
						continue
					}

					if strings.Contains(k, part) {
						entropy++
					}
				}
				if entropy > maxEntropy {
					if v != "" {
						maxEntropy = entropy
						candidate = k
					}
				}
			}
			if candidate != "" && maxEntropy > 0 {
				str = stringMap[candidate]
			}
		}
	}

	return str
}

func (p *Spravkaru) parsePostalCodeRows(host string) (map[string]string, error) {
	cityPostalIndex := make(map[string]string)

	resp, err := http.Get(host + zipCodeBaseUrl)
	if err != nil {
		return nil, err
	}

	rootPostalCodes, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, err
	}
	postalCodeRows := htmlquery.Find(rootPostalCodes, "/html/body/div[2]/div/div[4]/div[2]/div[1]/div/div/div")
	if len(postalCodeRows) == 0 {
		return nil, errors.New("no postal codes found")
	}

	for _, pc := range postalCodeRows {
		raw := htmlquery.InnerText(pc)
		re := regexp.MustCompile(`\([0-9]+\) .*\n`)
		list := re.FindAllStringSubmatch(raw, -1)
		for _, l := range list {
			for _, a := range l {
				indexCity := strings.Split(a, ")")
				if len(indexCity) == 2 {
					indexStr := strings.TrimSpace(indexCity[0])
					indexStr = strings.ReplaceAll(indexStr, "(", "")
					indexStr = strings.ReplaceAll(indexStr, ")", "")

					cityStr := strings.TrimSpace(indexCity[1])

					cityPostalIndex[cityStr] = indexStr
				}
			}
		}
	}
	return cityPostalIndex, nil
}

func (p *Spravkaru) parseSinglePhonePrefix(host string, phoneRows []*html.Node) (map[string]*model.CityInfo, error) {
	indexCityInfos := make(map[string]*model.CityInfo)

	cityPostalIndex, err := p.parsePostalCodeRows(host)
	if err != nil {
		return nil, err
	}

	for _, n := range phoneRows {
		citiesWithPhone := htmlquery.Find(n, "/a")
		for _, c := range citiesWithPhone {
			regexPhone := regexp.MustCompile(`\(* [0-9]+`)
			regexCity := regexp.MustCompile(`.*\(`)
			matchesPhone := regexPhone.FindStringSubmatch(htmlquery.InnerText(c))
			matchesCity := regexCity.FindStringSubmatch(htmlquery.InnerText(c))

			phone := ""
			city := ""

			if len(matchesPhone) > 0 {
				phone = strings.TrimSpace(matchesPhone[0])
			}
			if len(matchesCity) > 0 {
				city = strings.ReplaceAll(matchesCity[0], "(", "")
				city = strings.ReplaceAll(city, ")", "")
				city = strings.TrimSpace(city)
			}

			if city != "" {
				index := ""
				if host == rusHost {
					index = p.extendedStringSearch(cityPostalIndex, city)
				} else {
					index = "-1"
				}

				if phone != "" && index != "" {
					indexCityInfos[phone] = &model.CityInfo{
						City:  city,
						Phone: phone,
						Index: index,
					}
				}
			}
		}
	}
	return indexCityInfos, nil
}

func (p *Spravkaru) GetName() string {
	return pluginName
}
