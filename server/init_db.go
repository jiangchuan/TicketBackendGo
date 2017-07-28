package main

import (
 // "fmt"
 "log"
 "io/ioutil"
 "os"
 "gopkg.in/mgo.v2"
 // "gopkg.in/mgo.v2/bson"

)

type PoliceDoc struct {
	UserID      string   `json:"userid"`
	Password    string   `json:"password"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	City        string   `json:"city"`
	Dept        string   `json:"dept"`
	Squad       string   `json:"squad"`
	Section     string   `json:"section"`
	Portrait     []byte   `bson:"portrait"`
}


////////// MongoDB //////////
func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	c := session.DB("polices").C("officerdocs")
	index := mgo.Index{
		Key:        []string{"userid"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err := c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}

	c = session.DB("polices").C("policedocs")
	index = mgo.Index{
		Key:        []string{"userid"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}

}





func insertPolice(mUserID, mPassword, mName, mType, mCity, mDept, mSquad, mSection, mPortraitPath, db, dbdocs string, session *mgo.Session) {

	// Read image to []byte
	f, err := os.Open(mPortraitPath)
	if err != nil {
		log.Fatalln("Failed to open image: ", err.Error())
	}
	defer f.Close()
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalln("Failed to read image: ", err.Error())		
	}

	// Insert to MongoDB
	var p = PoliceDoc{UserID: mUserID, Password: mPassword, Name: mName, Type: mType, City: mCity, Dept: mDept, Squad: mSquad, Section: mSection, Portrait: bs}

	// session := s.Copy()
	// defer session.Close()

	c := session.DB(db).C(dbdocs)
	// c := session.DB("polices").C("policedocs")
  	_, err = c.UpsertId(p.UserID, &p)
	if err != nil {
		log.Println("Failed to insert or update record: ", err)
	}

}

func main() {

	/////////////////////////////////////////////////
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)
	ensureIndex(session)


	//           UserID    Password  Name     Type   City   Dept                Squad      Section   PortraitPath  db         dbdocs        session
	insertPolice("X285301", "123",   "张警官", "协警", "成都", "成都公安局交警五分局", "第二大队", "第三警区", "./portrait/X285301.jpg", "polices", "policedocs", session)
	insertPolice("X285302", "123",   "王警官", "协警", "成都", "成都公安局交警五分局", "第二大队", "第三警区", "./portrait/X285302.jpg", "polices", "policedocs", session)






















}
