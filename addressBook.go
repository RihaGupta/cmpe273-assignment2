package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"github.com/julienschmidt/httprouter"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2"
)

type Startresults struct {
	Results []struct {
		AddressComponents []struct {
			LongName  string   `json:"long_name"`
			ShortName string   `json:"short_name"`
			Types     []string `json:"types"`
		} `json:"address_components"`
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
			LocationType string `json:"location_type"`
			Viewport     struct {
				Northeast struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"northeast"`
				Southwest struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"southwest"`
			} `json:"viewport"`
		} `json:"geometry"`
		PlaceID string   `json:"place_id"`
		Types   []string `json:"types"`
	} `json:"results"`
	Status string `json:"status"`
}

type postresp struct {
	Id      bson.ObjectId `json:"id" bson:"_id"`
	Name    string        `json:"name" bson:"name"`
	Address string        `json:"address" bson:"address"`
	City    string        `json:"city" bson:"city"`
	State   string        `json:"state" bson:"state"`
	Zip     string        `json:"zip" bson:"zip"`
	Loc     Cord          `json:"coordinate" bson:"coordinate"`
}

type Cord struct {
	Lat float64 `json:"lat" bson:"lat"`
	Lng float64 `json:"lng" bson:"lng"`
}

type LocNavigator struct {
	session *mgo.Session
}

func NewNavigator(s *mgo.Session) *LocNavigator {
	return &LocNavigator{s}
}

func (ln LocNavigator) GetLoc(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
	id := p.ByName("id")
	if !bson.IsObjectIdHex(id) {
		rw.WriteHeader(404)
		return
	}
	oid := bson.ObjectIdHex(id)
	po := postresp{}
	if err := ln.session.DB("addressbook").C("Address").FindId(oid).One(&po); err != nil {
		rw.WriteHeader(404)
		return
	}
	json.NewDecoder(req.Body).Decode(po)
	uj, _ := json.Marshal(po)
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(200)
	fmt.Fprintf(rw, "%s", uj)
}

func (ln LocNavigator) UpdateLoc(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
	id := p.ByName("id")
	if !bson.IsObjectIdHex(id) {
		rw.WriteHeader(404)
		return
	}
	oid := bson.ObjectIdHex(id)
	po := postresp{}
	ps := postresp{}
	ps.Id = oid
	json.NewDecoder(req.Body).Decode(&ps)
	if err := ln.session.DB("addressbook").C("Address").FindId(oid).One(&po); err != nil {
		rw.WriteHeader(404)
		return
	}
	na := po.Name
	collections := ln.session.DB("addressbook").C("Address")
	po = fetchdata(&ps)
	collections.Update(bson.M{"_id": oid}, bson.M{"$set": bson.M{"address": ps.Address, "city": ps.City, "state": ps.State, "zip": ps.Zip, "coordinate": bson.M{"lat": po.Loc.Lat, "lng": po.Loc.Lng}}})
	po.Name = na
	uj, _ := json.Marshal(po)
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(201)
	fmt.Fprintf(rw, "%s", uj)
}

func (ln LocNavigator) RemoveLoc(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
	id := p.ByName("id")
	if !bson.IsObjectIdHex(id) {
		rw.WriteHeader(404)
		return
	}
	oid := bson.ObjectIdHex(id)
	if err := ln.session.DB("addressbook").C("Address").RemoveId(oid); err != nil {
		rw.WriteHeader(404)
		return
	}
	rw.WriteHeader(200)
}

func (ln LocNavigator) CreateLoc(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
	postrs := postresp{}
	json.NewDecoder(req.Body).Decode(&postrs)
	neww := fetchdata(&postrs)
	neww.Id = bson.NewObjectId()
	ln.session.DB("addressbook").C("Address").Insert(neww)
	uj, _ := json.Marshal(neww)
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(201)
	fmt.Fprintf(rw, "%s", uj)
}

func fetchdata(pr *postresp) postresp {
	add := pr.Address
	ci := pr.City
	gs := strings.Replace(pr.State, " ", "+", -1)
	gadd := strings.Replace(add, " ", "+", -1)
	gci := strings.Replace(ci, " ", "+", -1)
	uri := "http://maps.google.com/maps/api/geocode/json?address=" + gadd + "+" + gci + "+" + gs + "&sensor=false"
	resp, _ := http.Get(uri)
	body, _ := ioutil.ReadAll(resp.Body)
	C := Startresults{}
	err := json.Unmarshal(body, &C)
	if err != nil {
		panic(err)
	}
	for _, Sample := range C.Results {
		pr.Loc.Lat = Sample.Geometry.Location.Lat
		pr.Loc.Lng = Sample.Geometry.Location.Lng
	}
	return *pr
}

func getSession() *mgo.Session {
	s, err := mgo.Dial("mongodb://riha:rudransh@ds045464.mongolab.com:45464/addressbook")
	if err != nil {
		panic(err)
	}
	return s
}

func main() {
	x := httprouter.New()
	ln := NewNavigator(getSession())
	x.GET("/add/:id", ln.GetLoc)
	x.POST("/add", ln.CreateLoc)
	x.PUT("/add/:id", ln.UpdateLoc)
	x.DELETE("/add/:id", ln.RemoveLoc)
	http.ListenAndServe("localhost:8080", x)
}