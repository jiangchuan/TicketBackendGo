package main

import (
 "log"
 "io/ioutil"
 "os"
 "gopkg.in/mgo.v2"

 "golang.org/x/crypto/bcrypt"
)

type PoliceDoc struct {
	UserID         string   `json:"userid"`
	PasswordHash   []byte   `bson:"passwordhash"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	City           string   `json:"city"`
	Dept           string   `json:"dept"`
	Squad          string   `json:"squad"`
	Section        string   `json:"section"`
	Portrait       []byte   `bson:"portrait"`
	Thumbnail      []byte   `bson:"thumbnail"`
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

func readFile2Bytes(filePath string) []byte {
	f, err := os.Open(filePath)
	if err != nil {
		log.Println("Failed to open %s", filePath, err)
		return nil
	}
	defer f.Close()
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		log.Println("Failed to read %s", filePath, err)
		return nil
	}
	return bs
}

func insertPolice(mUserID, mPassword, mName, mType, mCity, mDept, mSquad, mSection, mPortraitPath, mThumbnailPath, db, dbdocs string, session *mgo.Session) {
    // Generate "hash" to store from user password
    mHash, err := bcrypt.GenerateFromPassword([]byte(mPassword), bcrypt.DefaultCost)
    if err != nil {
		log.Println("Failed to hash password ", err)
    }

	// Insert to MongoDB
	var p = PoliceDoc{UserID: mUserID, PasswordHash: mHash, Name: mName, Type: mType, City: mCity, Dept: mDept, Squad: mSquad, Section: mSection, Portrait: readFile2Bytes(mPortraitPath), Thumbnail: readFile2Bytes(mThumbnailPath)}
	c := session.DB(db).C(dbdocs)
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
	insertPolice("X285301", "123",   "张警官", "协警", "成都", "成都公安局交警4分局", "第一大队", "第三警区", "./portraits/p_X285301.jpg", "./portraits/t_X285301.jpg", "polices", "policedocs", session)
	insertPolice("X285302", "123",   "王警官", "协警", "成都", "成都公安局交警4分局", "第一大队", "第三警区", "./portraits/p_X285302.jpg", "./portraits/t_X285302.jpg", "polices", "policedocs", session)
	insertPolice("X285303", "123",   "李警官", "协警", "成都", "成都公安局交警4分局", "第一大队", "第三警区", "./portraits/p_X285303.jpg", "./portraits/t_X285303.jpg", "polices", "policedocs", session)
	insertPolice("X285401", "123",   "赵警官", "协警", "成都", "成都公安局交警5分局", "第二大队", "第四警区", "./portraits/p_X285401.jpg", "./portraits/t_X285401.jpg", "polices", "policedocs", session)
	insertPolice("X285402", "123",   "钱警官", "协警", "成都", "成都公安局交警5分局", "第二大队", "第四警区", "./portraits/p_X285402.jpg", "./portraits/t_X285402.jpg", "polices", "policedocs", session)
	insertPolice("X285403", "123",   "孙警官", "协警", "成都", "成都公安局交警5分局", "第二大队", "第四警区", "./portraits/p_X285403.jpg", "./portraits/t_X285403.jpg", "polices", "policedocs", session)

	insertPolice("005697", "123",   "余警官", "领导", "成都", "成都公安局交警5分局", "第二大队", "大队长", "./portraits/p_005697.jpg", "./portraits/t_005697.jpg", "polices", "officerdocs", session)

	insertPolice("Admin", "123",   "黄江龙", "系统管理员", "成都", "成都公安局交警5分局", "第二大队", "系统管理员", "./portraits/p_Admin.jpg", "./portraits/t_Admin.jpg", "polices", "officerdocs", session)

}









