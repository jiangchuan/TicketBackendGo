package main

import (
	"log"
	// "strings"
	"net/http"

	"github.com/goji/httpauth"
	"github.com/gorilla/mux"

	"gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
)

////////// MongoDB //////////
var dbSession *mgo.Session

type PoliceDoc struct {
	UserID      string   `json:"userid"`
	Password    string   `json:"password"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	City        string   `json:"city"`
	Dept        string   `json:"dept"`
	Station     string   `json:"station"`
}

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
}

func main() {

	var dialErr error

	dbSession, dialErr = mgo.Dial("localhost")
	if dialErr != nil {
		panic(dialErr)
	}
	defer dbSession.Close()

	dbSession.SetMode(mgo.Monotonic, true)
	ensureIndex(dbSession)

	/////////////////////////////////////////////////////////////////////

	authOpts := httpauth.AuthOptions{
        Realm: "ChiziTech",
        AuthFunc: myAuthFunc,
        UnauthorizedHandler: http.HandlerFunc(myUnauthorizedHandler),
    }

	r := mux.NewRouter()
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("tickets/"))))
	http.Handle("/", httpauth.BasicAuth(authOpts)(r))

	http.ListenAndServe(":8080", nil)
}

func myAuthFunc(user, pass string, r *http.Request) bool {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("officerdocs")
	var policeDoc PoliceDoc
	err := c.Find(bson.M{"userid": user}).One(&policeDoc)
	if err != nil {
		log.Println("Failed find police: ", err)
		return false
	}
	if policeDoc.UserID == "" {
		log.Println("Police not found:!")
		return false	
	}
	if policeDoc.Password != pass {
		log.Println("Wrong password! ")
		return false
	}
	return true
    // return pass == strings.Repeat(user, 3)
}

func myUnauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	// http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	http.Error(w, "用户名或密码不正确!", http.StatusUnauthorized)
}




// import (
// 	"net/http"
// )

// func main() {
// 	http.Handle("/", http.FileServer(http.Dir("./tickets")))
// 	http.ListenAndServe(":8080", nil)
// }
