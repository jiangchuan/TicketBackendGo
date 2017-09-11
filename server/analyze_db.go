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

	c = session.DB("performances").C("dayallperformancedocs")
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

	c = session.DB("performances").C("dayperformancedocs")
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

	c = session.DB("performances").C("weekperformancedocs")
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

	c = session.DB("performances").C("monthperformancedocs")
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
	_, thisWeek := mNow.ISOWeek() // Monday - Sunday

	// if mNow.Weekday() == 0 {
	// 	_, thisWeek = mNow.Add(24*time.Hour).ISOWeek()
	// }
	// fmt.Println(mNow.Weekday() == 0)
	// fmt.Println(thisWeek)

	stage_match_day_all := bson.M{"$match": bson.M{"year": mNow.Year(), "month": int(mNow.Month()), "day": mNow.Day()}}
	stage_match_day := bson.M{"$match": bson.M{"year": mNow.Year(), "month": int(mNow.Month()), "day": mNow.Day(), "isuploaded": true}}
	stage_match_week := bson.M{"$match": bson.M{"year": mNow.Year(), "month": int(mNow.Month()), "week": thisWeek, "isuploaded": true}}
	stage_match_month := bson.M{"$match": bson.M{"year": mNow.Year(), "month": int(mNow.Month()), "isuploaded": true}}
    stage_aggregate := bson.M{"$group": bson.M{
	        "_id": "$userid",
	        "userid": bson.M{"$max": "$userid"},
	        "ticketcount": bson.M{"$sum": 1},
	    }}

	pipe_day_all := c.Pipe([]bson.M{stage_match_day_all, stage_aggregate})
	pipe_day := c.Pipe([]bson.M{stage_match_day, stage_aggregate})
	pipe_week := c.Pipe([]bson.M{stage_match_week, stage_aggregate})
	pipe_month := c.Pipe([]bson.M{stage_match_month, stage_aggregate})
	var result PerformanceDoc

	iter := pipe_day_all.Iter()
	c = session.DB("performances").C("dayallperformancedocs")
    for iter.Next(&result) {
    	fmt.Println(result)
	  	_, err = c.UpsertId(result.UserID, &result)
		if err != nil {
			log.Println("Failed to insert or update day all record: ", err)
		}
    }

	iter = pipe_day.Iter()
	c = session.DB("performances").C("dayperformancedocs")
    for iter.Next(&result) {
    	fmt.Println(result)
	  	_, err = c.UpsertId(result.UserID, &result)
		if err != nil {
			log.Println("Failed to insert or update day record: ", err)
		}
    }

	iter = pipe_week.Iter()
	c = session.DB("performances").C("weekperformancedocs")
    for iter.Next(&result) {
    	fmt.Println(result)
	  	_, err = c.UpsertId(result.UserID, &result)
		if err != nil {
			log.Println("Failed to insert or update week record: ", err)
		}
    }

	iter = pipe_month.Iter()
	c = session.DB("performances").C("monthperformancedocs")
    for iter.Next(&result) {
    	fmt.Println(result)
	  	_, err = c.UpsertId(result.UserID, &result)
		if err != nil {
			log.Println("Failed to insert or update month record: ", err)
		}
    }
}

func main() {
	analyzedb()
	for range time.Tick(3 * time.Second) {
		analyzedb()
	}
}




