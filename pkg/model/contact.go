package model

type Contact struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Address   string `json:"address"`
}

type ContactInfo struct {
	ZipCode string   `json:"zip_code"`
	English *Contact `json:"english"`
	Russian *Contact `json:"russian"`
}

type CityContacts struct {
	CityNameRussian string         `json:"city_name_russian"`
	CityNameEnglish string         `json:"city_name_english"`
	Contacts        []*ContactInfo `json:"contacts"`
}
