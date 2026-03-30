package models

type Response struct {
	Countries []Country `json:"countries"`
}

type Country struct {
	Cities []City `json:"cities"`
}

type City struct {
	UID    int     `json:"uid"`
	Name   string  `json:"name"`
	Places []Place `json:"places"`
}

type Place struct {
	UID                  int     `json:"uid"`
	Name                 string  `json:"name"`
	Lat                  float64 `json:"lat"`
	Lng                  float64 `json:"lng"`
	BikesAvailableToRent int     `json:"bikes_available_to_rent"`
}
