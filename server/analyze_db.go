package main

import (
 "fmt"
 "log"
 "gopkg.in/mgo.v2"
 "gopkg.in/mgo.v2/bson"
 "time"
)

type PerformanceDoc struct {
	UserID         string   `json:"userid"`
	TicketCount    int32    `json:"ticketcount"`
}

////////// MongoDB //////////
func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	c := session.DB("tickets").C("ticketdocs")
	index := mgo.Index{
		Key:        []string{"ticketid"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err := c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}

	c = session.DB("polices").C("dayperformancedocs")
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

	c = session.DB("polices").C("monthperformancedocs")
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

func analyzedb() {
	fmt.Println("analyzing ... ")
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)
	ensureIndex(session)

	c := session.DB("tickets").C("ticketdocs")

	mNow := time.Now()
	stage_match_day := bson.M{"$match": bson.M{"year": mNow.Year(), "month": int(mNow.Month()), "day": mNow.Day()}}
	stage_match_month := bson.M{"$match": bson.M{"year": mNow.Year(), "month": int(mNow.Month())}}
    stage_aggregate := bson.M{"$group": bson.M{
	        "_id": "$userid",
	        "userid": bson.M{"$max": "$userid"},
	        "ticketcount": bson.M{"$sum": 1},
	    }}

	pipe_day := c.Pipe([]bson.M{stage_match_day, stage_aggregate})
	pipe_month := c.Pipe([]bson.M{stage_match_month, stage_aggregate})
	var result PerformanceDoc


	iter := pipe_day.Iter()
	c = session.DB("polices").C("dayperformancedocs")
    for iter.Next(&result) {
    	fmt.Println(result)
	  	_, err = c.UpsertId(result.UserID, &result)
		if err != nil {
			log.Println("Failed to insert or update record: ", err)
		}
    }

	iter = pipe_month.Iter()
	c = session.DB("polices").C("monthperformancedocs")
    for iter.Next(&result) {
    	fmt.Println(result)
	  	_, err = c.UpsertId(result.UserID, &result)
		if err != nil {
			log.Println("Failed to insert or update record: ", err)
		}
    }
}

func main() {
	analyzedb()
	for range time.Tick(time.Hour) {
		analyzedb()
	}
}




