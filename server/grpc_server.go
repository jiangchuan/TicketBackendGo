package main

import (
 "flag"
 "fmt"
 "log"
 "os"
 "net"
 "sync"
 "time"

 // "io/ioutil"

 "golang.org/x/net/context"
 "google.golang.org/grpc"

 "google.golang.org/grpc/credentials"
 "google.golang.org/grpc/grpclog"

 pb "../proto"
 // "google.golang.org/grpc/reflection"

 "gopkg.in/mgo.v2"
 "gopkg.in/mgo.v2/bson"
)

const (
   // CHINA_WEST float64 = 73.0
   // CHINA_EAST float64 = 135.0
   // CHINA_SOUTH float64 = 20.0
   // CHINA_NORTH float64 = 54.0

    CHINA_WEST float64 = -125.0
    CHINA_EAST float64 = -64.0
    CHINA_SOUTH float64 = 24.0
    CHINA_NORTH float64 = 49.5
)


var (
	tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "testdata/server1.pem", "The TLS cert file")
	keyFile    = flag.String("key_file", "testdata/server1.key", "The TLS key file")
	jsonDBFile = flag.String("json_db_file", "testdata/route_guide_db.json", "A json file containing a list of features")
	port       = flag.Int("port", 50051, "The server port")
)

var dbSession *mgo.Session

var usersLock = &sync.Mutex{} // Added chat

// server is used to implement ticket.TicketServer.
type ticketServer struct {
	// savedFeatures []*pb.Feature
	// routeNotes    map[string][]*pb.RouteNote
}

type PoliceDoc struct {
	UserID      string   `json:"userid"`
	Password    string   `json:"password"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	City        string   `json:"city"`
	Dept        string   `json:"dept"`
	Squad       string   `json:"squad"`
	Section     string   `json:"section"`
	Portrait    []byte   `bson:"portrait"`
	Thumbnail   []byte   `bson:"thumbnail"`
}

type TicketStatsDoc struct {
	UserID                 string   `json:"userid"`
	SavedTicketCount       int32     `json:"savedticketcount"`
	UploadedTicketCount    int32     `json:"uploadedticketcount"`
}

type PoliceLocDoc struct {
	UserID       string      `json:"userid"`
	Dept         string      `json:"dept"`
	Longitude    float64     `json:"longitude"`
	Latitude     float64     `json:"latitude"`
	Timestamp    time.Time   `json:"timestamp"`
}

// type PoliceAnchorDoc struct {
// 	UserID         string    `json:"userid"`
// 	AnchorCount    int32     `json:"anchorcount"`
// 	Anchor0Lng     float64   `json:"anchor0lng"`
// 	Anchor0Lat     float64   `json:"anchor0lat"`
// 	Anchor1Lng     float64   `json:"anchor1lng"`
// 	Anchor1Lat     float64   `json:"anchor1lat"`
// 	Anchor2Lng     float64   `json:"anchor2lng"`
// 	Anchor2Lat     float64   `json:"anchor2lat"`
// 	Anchor3Lng     float64   `json:"anchor3lng"`
// 	Anchor3Lat     float64   `json:"anchor3lat"`
// 	Anchor4Lng     float64   `json:"anchor4lng"`
// 	Anchor4Lat     float64   `json:"anchor4lat"`
// 	Anchor5Lng     float64   `json:"anchor5lng"`
// 	Anchor5Lat     float64   `json:"anchor5lat"`
// }

type PoliceAnchorDoc struct {
	UserID       string      `json:"userid"`
	Longitude    float64     `json:"longitude"`
	Latitude     float64     `json:"latitude"`
	Timestamp    time.Time   `json:"timestamp"`
}


type TicketDoc struct {
	TicketID        int64     `json:"ticketid"`
	UserID          string    `json:"userid"`
	LicenseNum      string    `json:"licensenum"`
	LicenseColor    string    `json:"licensecolor"`
	VehicleType     string    `json:"vehicletype"`
	VehicleColor    string    `json:"vehiclecolor"`
	Year            int32     `json:"year"`
	Month           int32     `json:"month"`
	Week            int32     `json:"week"`
	Day             int32     `json:"day"`
	Hour            int32     `json:"hour"`
	Minute          int32     `json:"minute"`
	TicketTime      int64     `json:"tickettime"`
	Address         string    `json:"address"`
	Longitude       float64   `json:"longitude"`
	Latitude        float64   `json:"latitude"`
	MapImage        []byte    `bson:"mapimage"`
	FarImage        []byte    `bson:"farimage"`
	CloseImage      []byte    `bson:"closeimage"`
	TicketImage     []byte    `bson:"ticketimage"`
	IsUploaded      bool      `json:"isuploaded"`
}

type TicketRangeDoc struct {
	UserID           string    `json:"userid"`
	TicketIDStart    int64     `json:"ticketidstart"`
	TicketIDEnd      int64     `json:"ticketidend"`
}

type PerformanceDoc struct {
	UserID         string   `json:"userid"`
	TicketCount    int32    `json:"ticketcount"`
}

func (s *ticketServer) SlaveLocSubmit(ctx context.Context, slaveLoc *pb.SlaveLoc) (*pb.MasterOrder, error) {
	if !lonLatInChina(slaveLoc.Longitude, slaveLoc.Latitude) {
		log.Println("Lat-Lon out of China!")
		return &pb.MasterOrder{MasterOrder: "Lat-Lon out of China!"}, nil
	}

	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policelocdocs")
	var p = PoliceLocDoc{UserID: slaveLoc.Sid,
		Dept: slaveLoc.PoliceDept,
		Longitude: slaveLoc.Longitude,
		Latitude: slaveLoc.Latitude,
		Timestamp: time.Now()}
  	_, err := c.UpsertId(p.UserID, &p)
	if err != nil {
		log.Println("Insert/update loc failed!", err)
		return &pb.MasterOrder{MasterOrder: "Insert/update loc failed!"}, err
	}
	// fmt.Println("Sid = " + slaveLoc.Sid)
	// fmt.Println("Longitude = %d", slaveLoc.Longitude)
	// fmt.Println("Latitude = %d", slaveLoc.Latitude)
	return &pb.MasterOrder{MasterOrder: "Submit Loc Success!"}, nil
}

func Min(x, y int32) int32 {
    if x < y {
        return x
    }
    return y
}

func (s *ticketServer) RhinoLogin(ctx context.Context, loginInfo *pb.LoginRequest) (*pb.LoginReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("officerdocs")
	var policeDoc PoliceDoc
	err := c.Find(bson.M{"userid": loginInfo.UserId}).One(&policeDoc)
	if err != nil {
		log.Println("Failed find police: ", err)
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.UserID == "" {
		log.Println("Police not found:!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.Password != loginInfo.Password {
		log.Println("Wrong password! ")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	return &pb.LoginReply{  LoginSuccess: true,
		PoliceName: policeDoc.Name,
		PoliceType: policeDoc.Type,
		PoliceCity: policeDoc.City,
		PoliceDept: policeDoc.Dept,
		PoliceSquad: policeDoc.Squad,
		PoliceSection: policeDoc.Section,
		PolicePortrait: policeDoc.Portrait}, nil
}

func (s *ticketServer) RhinoCreateAccount(ctx context.Context, policeInfo *pb.AccountRequest) (*pb.AccountReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("officerdocs")
	err := c.Insert(&PoliceDoc{UserID: policeInfo.UserId,
		Password: policeInfo.Password,
		Name: policeInfo.PoliceName,
		Type: policeInfo.PoliceType,
		City: policeInfo.PoliceCity,
		Dept: policeInfo.PoliceDept,
		Squad: policeInfo.PoliceSquad,
		Section: policeInfo.PoliceSection,
		Portrait: policeInfo.PolicePortrait})
	if err != nil {
		if mgo.IsDup(err) {
			return &pb.AccountReply{CreateSuccess: false}, err
		}
		return &pb.AccountReply{CreateSuccess: false}, err
	}
	return &pb.AccountReply{CreateSuccess: true}, nil
}

func (s *ticketServer) RhinoChangePassword(ctx context.Context, loginInfo *pb.PasswordRequest) (*pb.LoginReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("officerdocs")
	var policeDoc PoliceDoc
	err := c.Find(bson.M{"userid": loginInfo.UserId}).One(&policeDoc)
	if err != nil {
		log.Println("Failed to find police: ", err)
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.UserID == "" {
		log.Println("Police not found:!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.Password != loginInfo.Password {
		log.Println("Wrong password!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	// Update password
	err = c.Update(bson.M{"userid": loginInfo.UserId}, bson.M{"$set": bson.M{"password": loginInfo.NewPassword}})
	if err != nil {
		log.Println("Failed to change password: ", err)
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	return &pb.LoginReply{  LoginSuccess: true,
		PoliceName: policeDoc.Name,
		PoliceType: policeDoc.Type,
		PoliceCity: policeDoc.City,
		PoliceDept: policeDoc.Dept,
		PoliceSquad: policeDoc.Squad,
		PoliceSection: policeDoc.Section,
		PolicePortrait: policeDoc.Portrait}, nil
}

func (s *ticketServer) PullHareOne(ctx context.Context, loginInfo *pb.HareRequest) (*pb.HareProfiles, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policedocs")
	var policeDoc PoliceDoc
	err := c.Find(bson.M{"userid": loginInfo.Sid}).One(&policeDoc)
	if err != nil {
		log.Println("Failed find police: ", err)
		return &pb.HareProfiles{HareSuccess: false}, err
	}
	if policeDoc.UserID == "" {
		log.Println("Police not found:!")
		return &pb.HareProfiles{HareSuccess: false}, err
	}
	return &pb.HareProfiles{HareSuccess: true,
		UserId: policeDoc.UserID,
		PoliceName: policeDoc.Name,
		PoliceType: policeDoc.Type,
		PoliceCity: policeDoc.City,
		PoliceDept: policeDoc.Dept,
		PoliceSquad: policeDoc.Squad,
		PoliceSection: policeDoc.Section,
		PolicePortrait: policeDoc.Portrait,
		PoliceThumbnail: policeDoc.Thumbnail}, nil
}

func (s *ticketServer) PullHareAll(pullProfileRequest *pb.HareAllRequest, stream pb.Ticket_PullHareAllServer) error {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policedocs")

	var allPolices []PoliceDoc
	err := c.Find(nil).All(&allPolices)
	// err = c.Find(bson.M{"userid": pullProfileRequest.Mid}).Sort("-timestamp").All(&allPolices)
	if err != nil {
		log.Println("Failed find police profiles: ", err)
		return nil
	}

	for _, profile := range allPolices {
		var p = pb.HareProfiles{HareSuccess: true,
					UserId: profile.UserID,
					PoliceName: profile.Name,
					PoliceType: profile.Type,
					PoliceCity: profile.City,
					PoliceDept: profile.Dept,
					PoliceSquad: profile.Squad,
					PoliceSection: profile.Section,
					PolicePortrait: profile.Portrait,
					PoliceThumbnail: profile.Thumbnail}
		if err := stream.Send(&p); err != nil {
			return err
		}
	}
	return nil
}

func (s *ticketServer) HareLogin(ctx context.Context, loginInfo *pb.LoginRequest) (*pb.LoginReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policedocs")
	var policeDoc PoliceDoc
	err := c.Find(bson.M{"userid": loginInfo.UserId}).One(&policeDoc)
	if err != nil {
		log.Println("Failed to find police: ", err)
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.UserID == "" {
		log.Println("Police not found:!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.Password != loginInfo.Password {
		log.Println("Wrong password!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}

	c = session.DB("polices").C("policelocdocs")
	var slaveLoc PoliceLocDoc
	err = c.Find(bson.M{"userid": loginInfo.UserId}).One(&slaveLoc)
	if err == nil && slaveLoc.UserID == loginInfo.UserId && slaveLoc.Timestamp.After(time.Now().Add(-5*time.Second)) {
		return &pb.LoginReply{LoginSuccess: false}, nil
	}
	return &pb.LoginReply{LoginSuccess: true,
		PoliceName: policeDoc.Name,
		PoliceType: policeDoc.Type,
		PoliceCity: policeDoc.City,
		PoliceDept: policeDoc.Dept,
		PoliceSquad: policeDoc.Squad,
		PoliceSection: policeDoc.Section,
		PolicePortrait: policeDoc.Portrait}, nil
}

func (s *ticketServer) HareLogout(ctx context.Context, logoutInfo *pb.LogoutRequest) (*pb.LogoutReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policelocdocs")
	_, err := c.RemoveAll(bson.M{"userid": logoutInfo.UserId})
	if err != nil {
		log.Println(fmt.Sprintf("Failed to logout %s", logoutInfo.UserId), err)
		return &pb.LogoutReply{LogoutSuccess: false}, err
	}
	return &pb.LogoutReply{LogoutSuccess: true}, nil
}

func (s *ticketServer) HareCreateAccount(ctx context.Context, policeInfo *pb.AccountRequest) (*pb.AccountReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policedocs")
	err := c.Insert(&PoliceDoc{UserID: policeInfo.UserId,
		Password: policeInfo.Password,
		Name: policeInfo.PoliceName,
		Type: policeInfo.PoliceType,
		City: policeInfo.PoliceCity,
		Dept: policeInfo.PoliceDept,
		Squad: policeInfo.PoliceSquad,
		Section: policeInfo.PoliceSection,
		Portrait: policeInfo.PolicePortrait})
	if err != nil {
		if mgo.IsDup(err) {
			return &pb.AccountReply{CreateSuccess: false}, err
		}
		return &pb.AccountReply{CreateSuccess: false}, err
	}
	return &pb.AccountReply{CreateSuccess: true}, nil
}

func (s *ticketServer) HareChangePassword(ctx context.Context, loginInfo *pb.PasswordRequest) (*pb.LoginReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policedocs")
	var policeDoc PoliceDoc
	err := c.Find(bson.M{"userid": loginInfo.UserId}).One(&policeDoc)
	if err != nil {
		log.Println("Failed to find police: ", err)
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.UserID == "" {
		log.Println("Police not found:!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	if policeDoc.Password != loginInfo.Password {
		log.Println("Wrong password!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	// Update password
	err = c.Update(bson.M{"userid": loginInfo.UserId}, bson.M{"$set": bson.M{"password": loginInfo.NewPassword}})
	if err != nil {
		log.Println("Failed to change password: ", err)
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	return &pb.LoginReply{  LoginSuccess: true,
		PoliceName: policeDoc.Name,
		PoliceType: policeDoc.Type,
		PoliceCity: policeDoc.City,
		PoliceDept: policeDoc.Dept,
		PoliceSquad: policeDoc.Squad,
		PoliceSection: policeDoc.Section,
		PolicePortrait: policeDoc.Portrait}, nil
}


func (s *ticketServer) RecordTicket(ctx context.Context, ticketInfo *pb.TicketDetails) (*pb.RecordReply, error) {
	//////////////////// Save Ticket to Folder ////////////////////
	if ticketInfo.IsUploaded {
		dayName := fmt.Sprintf("%d-%d-%d", ticketInfo.Year, ticketInfo.Month, ticketInfo.Day)
		timeNumName := fmt.Sprintf("%d-%d_%d", ticketInfo.Hour, ticketInfo.Minute, ticketInfo.TicketId)
		directoryPath := fmt.Sprintf("./tickets/%s/%s_%s", dayName, dayName, timeNumName)
		createDirectory(directoryPath)

		// Write ticket.txt
		ticketPath := fmt.Sprintf("%s/ticket_%d.txt", directoryPath, ticketInfo.TicketId)
		ticketContent := fmt.Sprintf("罚单编号: %d\n车辆牌号: %s\n车身颜色: %s\n车辆类型: %s\n号牌颜色: %s\n停车时间: %d年%d月%d日%d时%d分\n停车地点: %s", 
				ticketInfo.TicketId, ticketInfo.LicenseNum, ticketInfo.VehicleColor, ticketInfo.VehicleType, ticketInfo.LicenseColor,
				ticketInfo.Year, ticketInfo.Month, ticketInfo.Day, ticketInfo.Hour, ticketInfo.Minute, ticketInfo.Address)
		createFile(ticketPath)
		writeFile(ticketPath, ticketContent)

		// Write img.jpg
		imgPath := fmt.Sprintf("%s/image1_%d.jpg", directoryPath, ticketInfo.TicketId)
		createFile(imgPath)
		writeImage(imgPath, ticketInfo.FarImage)

		imgPath = fmt.Sprintf("%s/image2_%d.jpg", directoryPath, ticketInfo.TicketId)
		createFile(imgPath)
		writeImage(imgPath, ticketInfo.CloseImage)

		imgPath = fmt.Sprintf("%s/image3_%d.jpg", directoryPath, ticketInfo.TicketId)
		createFile(imgPath)
		writeImage(imgPath, ticketInfo.TicketImage)
	}


	//////////////////// Record Lat-Lon Anchors ////////////////////
	session := dbSession.Copy()
	defer session.Close()

	if lonLatInChina(ticketInfo.Longitude, ticketInfo.Latitude) {
		c := session.DB("polices").C("policeanchordocs")
		var p = PoliceAnchorDoc{UserID: ticketInfo.UserId,
			Longitude: ticketInfo.Longitude,
			Latitude: ticketInfo.Latitude,
			Timestamp: time.Now()}
	  	// _, err := c.UpsertId(p.TicketID, &p)
	  	err := c.Insert(&p)

		if err != nil {
			log.Println("Insert/update anchor failed!", err)
		}
		log.Println("Submit Anchor Success!")
		fmt.Println("Sid = " + ticketInfo.UserId)
		fmt.Println("Anchor Longitude = %d", ticketInfo.Longitude)
		fmt.Println("Anchor Latitude = %d", ticketInfo.Latitude)
	} else {
		log.Println("Lat-Lon out of China!")
	}

	// //////////////////// Record Ticket Statistics ////////////////////
	// c := session.DB("polices").C("ticketstatsdocs")
	// var p = TicketStatsDoc{UserID: ticketInfo.UserId,
	// 	SavedTicketCount: ticketInfo.SavedTicketCount,
	// 	UploadedTicketCount: ticketInfo.UploadedTicketCount}
 //  	_, err := c.UpsertId(p.UserID, &p)
	// if err != nil {
	// 	log.Println("Failed to insert/update ticket statistics!", err)
	// }

	//////////////////// Record Ticket to DB ////////////////////
	c := session.DB("tickets").C("ticketdocs")
	var p = TicketDoc{TicketID: ticketInfo.TicketId,
		UserID: ticketInfo.UserId,
		LicenseNum: ticketInfo.LicenseNum,
		LicenseColor: ticketInfo.LicenseColor,
		VehicleType: ticketInfo.VehicleType,
		VehicleColor: ticketInfo.VehicleColor,
		Year: ticketInfo.Year,
		Month: ticketInfo.Month,
		Week: ticketInfo.Week,
		Day: ticketInfo.Day,
		Hour: ticketInfo.Hour,
		Minute: ticketInfo.Minute,
		TicketTime: ticketInfo.TicketTime,
		Address: ticketInfo.Address,
		Longitude: ticketInfo.Longitude,
		Latitude: ticketInfo.Latitude,
		MapImage: ticketInfo.MapImage,
		FarImage: ticketInfo.FarImage,
		CloseImage: ticketInfo.CloseImage,
		TicketImage: ticketInfo.TicketImage,
		IsUploaded: ticketInfo.IsUploaded}
  	_, err := c.UpsertId(p.TicketID, &p)
	if err != nil {
		return &pb.RecordReply{RecordSuccess: false}, err
	}
	return &pb.RecordReply{RecordSuccess: true}, err
}

func (s *ticketServer) SetTicketRange(ctx context.Context, ticketRange *pb.TicketRange) (*pb.TicketRangeReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("tickets").C("ticketrangedocs")
	var p = TicketRangeDoc{UserID: "Admin",
		TicketIDStart: ticketRange.TicketIdStart,
		TicketIDEnd: ticketRange.TicketIdEnd}
  	_, err := c.UpsertId(p.UserID, &p)
	if err != nil {
		return &pb.TicketRangeReply{RangeSuccess: false}, err
	}
	return &pb.TicketRangeReply{RangeSuccess: true}, err
}

func (s *ticketServer) PullTicketRange(ctx context.Context, ticketRangeSid *pb.TicketRangeSid) (*pb.TicketRange, error) {
	const DELTA_RANGE int64 = 200
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("tickets").C("ticketrangedocs")

	var ticketRangeDoc TicketRangeDoc
	err := c.Find(bson.M{"userid": "Admin"}).One(&ticketRangeDoc)
	if err != nil {
		log.Println("Failed to find Admin: ", err)
		return &pb.TicketRange{TicketIdStart: -1, TicketIdEnd: -1}, err
	}
	if ticketRangeDoc.UserID == "" {
		log.Println("Admin not found!")
		return &pb.TicketRange{TicketIdStart: -1, TicketIdEnd: -1}, err
	}

	var theStart = ticketRangeDoc.TicketIDStart
	var newStart = ticketRangeDoc.TicketIDStart + DELTA_RANGE
	var theEnd = ticketRangeDoc.TicketIDEnd
	if newStart > theEnd {
		log.Println("Out of range!")
		return &pb.TicketRange{TicketIdStart: -1, TicketIdEnd: -1}, err		
	}

	var pAdmin = TicketRangeDoc{UserID: "Admin",
		TicketIDStart: newStart,
		TicketIDEnd: theEnd}
  	_, err = c.UpsertId(pAdmin.UserID, &pAdmin)
	if err != nil {
		log.Println("Fail to update overall range!")
		return &pb.TicketRange{TicketIdStart: -1, TicketIdEnd: -1}, err		
	}

	var pSid = TicketRangeDoc{UserID: ticketRangeSid.Sid,
		TicketIDStart: theStart,
		TicketIDEnd: (newStart - 1)}
  	_, err = c.UpsertId(pSid.UserID, &pSid)
	if err != nil {
		log.Println("Fail to update sid range!")
	}
	return &pb.TicketRange{TicketIdStart: pSid.TicketIDStart, TicketIdEnd: pSid.TicketIDEnd}, err
}

func (s *ticketServer) PullLocation(pullLocRequest *pb.PullLocRequest, stream pb.Ticket_PullLocationServer) error {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policelocdocs")

    var slaveLocs []PoliceLocDoc
	var err error
    if pullLocRequest.PoliceDept == "全部" {
	    err = c.Find(bson.M{"timestamp": bson.M{"$gt": time.Now().Add(-10*time.Second)},}).All(&slaveLocs)
    } else {
	    err = c.Find(bson.M{"dept": pullLocRequest.PoliceDept,
	    					"timestamp": bson.M{"$gt": time.Now().Add(-10*time.Second)},}).All(&slaveLocs)
    }
    if err != nil {
		log.Println("Failed find police locations: ", err)
		return err
    }

	var policeDoc PoliceDoc
	c = session.DB("polices").C("policedocs")
	for _, slaveLoc := range slaveLocs {
    	err = c.Find(bson.M{"userid": slaveLoc.UserID}).One(&policeDoc)
		if err != nil || policeDoc.UserID == "" {
			log.Println("No police found: ", err)
			if err = stream.Send(&pb.SlaveNameLoc{Sid: slaveLoc.UserID,
					PoliceName: slaveLoc.UserID,
					Longitude: slaveLoc.Longitude,
					Latitude: slaveLoc.Latitude}); err != nil {
				return err
			}
		}
		if err = stream.Send(&pb.SlaveNameLoc{  Sid: slaveLoc.UserID,
				PoliceName: policeDoc.Name,
				Longitude: slaveLoc.Longitude,
				Latitude: slaveLoc.Latitude}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ticketServer) PullTicketStats(ctx context.Context, pullTicketStatsRequest *pb.PullTicketStatsRequest) (*pb.TicketStats, error) {
	session := dbSession.Copy()
	defer session.Close()
	
    var ticket_count_day_all int32 = 0
    var ticket_count_day int32 = 0
	var performanceDoc PerformanceDoc
	c_day_all := session.DB("performances").C("dayallperformancedocs")
	c_day := session.DB("performances").C("dayperformancedocs")

	err := c_day_all.Find(bson.M{"userid": pullTicketStatsRequest.Sid}).One(&performanceDoc)
	if err != nil || performanceDoc.UserID == "" {
		// log.Println("No police performance day all: ", err)
		ticket_count_day_all = 0
	} else {
		ticket_count_day_all = performanceDoc.TicketCount	
	}

	err = c_day.Find(bson.M{"userid": pullTicketStatsRequest.Sid}).One(&performanceDoc)
	if err != nil || performanceDoc.UserID == "" {
		// log.Println("No police performance day: ", err)
		ticket_count_day = 0
	} else {
		ticket_count_day = performanceDoc.TicketCount	
	}

	return &pb.TicketStats{Sid: pullTicketStatsRequest.Sid,
					SavedTicketCount: ticket_count_day_all,
					UploadedTicketCount: ticket_count_day}, nil
}

func (s *ticketServer) PullPerformance(pullPerformanceRequest *pb.PullPerformanceRequest, stream pb.Ticket_PullPerformanceServer) error {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policedocs")

    var slaves []PoliceDoc
	var err error
    if pullPerformanceRequest.PoliceDept == "全部" {
	    err = c.Find(bson.M{}).All(&slaves)
    } else {
    	err = c.Find(bson.M{"dept": pullPerformanceRequest.PoliceDept}).All(&slaves)
    }
    if err != nil {
		log.Println("Failed to find polices: ", err)
		return err
    }

    var ticket_count_day_all int32 = 0
    var ticket_count_day int32 = 0
    var ticket_count_week int32 = 0
    var ticket_count_month int32 = 0
	var performanceDoc PerformanceDoc
	c_day_all := session.DB("performances").C("dayallperformancedocs")
	c_day := session.DB("performances").C("dayperformancedocs")
	c_week := session.DB("performances").C("weekperformancedocs")
	c_month := session.DB("performances").C("monthperformancedocs")

	for _, slave := range slaves {
		err = c_day_all.Find(bson.M{"userid": slave.UserID}).One(&performanceDoc)
		if err != nil || performanceDoc.UserID == "" {
			// log.Println("No police performance day all: ", err)
			ticket_count_day_all = 0
		} else {
			ticket_count_day_all = performanceDoc.TicketCount	
		}

		err = c_day.Find(bson.M{"userid": slave.UserID}).One(&performanceDoc)
		if err != nil || performanceDoc.UserID == "" {
			// log.Println("No police performance day: ", err)
			ticket_count_day = 0
		} else {
			ticket_count_day = performanceDoc.TicketCount	
		}

		err = c_week.Find(bson.M{"userid": slave.UserID}).One(&performanceDoc)
		if err != nil || performanceDoc.UserID == "" {
			// log.Println("No police performance week: ", err)
			ticket_count_week = 0
		} else {
			ticket_count_week = performanceDoc.TicketCount	
		}

		err = c_month.Find(bson.M{"userid": slave.UserID}).One(&performanceDoc)
		if err != nil || performanceDoc.UserID == "" {
			// log.Println("No police performance month: ", err)
			ticket_count_month = 0
		} else {
			ticket_count_month = performanceDoc.TicketCount	
		}

		if err := stream.Send(&pb.SlavePerformance{  Sid: slave.UserID,
					PoliceName: slave.Name,
					PoliceDept: slave.Dept,
					SavedTicketCount: ticket_count_day_all,
					UploadedTicketCount: ticket_count_day,
					TicketCountDay: ticket_count_day,
					TicketCountWeek: ticket_count_week,
					TicketCountMonth: ticket_count_month}); err != nil {
			return err
		}
	}

	log.Println("Finished PullPerformance")
	return nil
}

func lonLatInChina(longitude, latitude float64) bool {
    if (longitude > CHINA_WEST && longitude < CHINA_EAST && latitude > CHINA_SOUTH && latitude < CHINA_NORTH) {
        return true;
    }
    return false;
}

func (s *ticketServer) PullAnchors(pullAnchorRequest *pb.PullAnchorRequest, stream pb.Ticket_PullAnchorsServer) error {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policeanchordocs")
    var slaveLocs []PoliceAnchorDoc
    err := c.Find(bson.M{"userid": pullAnchorRequest.Sid}).Sort("timestamp").All(&slaveLocs)

    if err != nil {
		log.Println("Failed find police anchors: ", err)
		return err
    }

	for _, slaveLoc := range slaveLocs {
		fmt.Println("anchor entered")
		if err = stream.Send(&pb.SlaveLoc{Sid: slaveLoc.UserID,
				Longitude: slaveLoc.Longitude,
				Latitude: slaveLoc.Latitude}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ticketServer) PullTickets(pullTicketsRequest *pb.PullTicketsRequest, stream pb.Ticket_PullTicketsServer) error {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("tickets").C("ticketdocs")

	var allTickets []TicketDoc
	err := c.Find(bson.M{"userid": pullTicketsRequest.Sid,
						"year": pullTicketsRequest.Year,
						"week": pullTicketsRequest.Week,
						"tickettime": bson.M{"$gt": pullTicketsRequest.LastTime},}).All(&allTickets)
	// err = c.Find(bson.M{"userid": pullTicketsRequest.Sid}).Sort("-timestamp").All(&allTickets)
	if err != nil {
		log.Println(fmt.Sprintf("Failed find police tickets for week %d", pullTicketsRequest.Week), err)
		return nil
	}

	for _, details := range allTickets {
		var p = pb.TicketDetails{TicketId: details.TicketID,
					UserId: details.UserID,
					LicenseNum: details.LicenseNum,
					LicenseColor: details.LicenseColor,
					VehicleType: details.VehicleType,
					VehicleColor: details.VehicleColor,
					Year: details.Year,
					Month: details.Month,
					Week: details.Week,
					Day: details.Day,
					Hour: details.Hour,
					Minute: details.Minute,
					TicketTime: details.TicketTime,
					Address: details.Address,
					Longitude: details.Longitude,
					Latitude: details.Latitude,
					MapImage: details.MapImage,
					FarImage: details.FarImage,
					CloseImage: details.CloseImage,
					TicketImage: details.TicketImage,
					IsUploaded: details.IsUploaded}
		if err := stream.Send(&p); err != nil {
			return err
		}
	}
	return nil
}


////////// Read and Write Files //////////
func createDirectory(directoryPath string) {
	//choose your permissions well
	err := os.MkdirAll(directoryPath, 0777)
	//check if you need to panic, fallback or report
	if err != nil {
		log.Println("Failed to create %s", directoryPath, err)
	}
}

func createFile(path string) {
	// detect if file exists
	var _, err = os.Stat(path)

	// create file if not exists
	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		checkError(err) //okay to call os.exit() 
		defer file.Close()
	}
}

func writeFile(path string, content string) {
	// open file using READ & WRITE permission
	var file, err = os.OpenFile(path, os.O_RDWR, 0644)
	checkError(err)
	defer file.Close()

	// write some text to file
	_, err = file.WriteString(content)
	if err != nil {
		log.Println("Failed to write string ", err)
		return //must return here for defer statements to be called
	}

	// save changes
	err = file.Sync()
	if err != nil {
		log.Println("Failed to save changes ", err)
		return //same as above
	}
}

func writeImage(path string, content []byte) {
	// open file using READ & WRITE permission
	var file, err = os.OpenFile(path, os.O_RDWR, 0644)
	checkError(err)
	defer file.Close()

	// write some text to file
	_, err = file.Write(content)
	if err != nil {
		log.Println("Failed to write files ", err)
		return //must return here for defer statements to be called
	}

	// save changes
	err = file.Sync()
	if err != nil {
		log.Println("Failed to save changes ", err)
		return //same as above
	}
}

func readFile(path string) {
	// re-open file
	var file, err = os.OpenFile(path, os.O_RDWR, 0644)
	checkError(err)
	defer file.Close()

	// read file
	var text = make([]byte, 1024)
	n, err := file.Read(text)
	if n > 0 {
		fmt.Println(string(text))
	}
	//if there is an error while reading
	//just print however much was read if any
	//at return file will be closed
}

func deleteFile(path string) {
	// delete file
	var err = os.Remove(path)
	checkError(err)
}

func checkError(err error) {
	if err != nil {
		log.Println(err)
		os.Exit(0)
	}
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

	c = session.DB("polices").C("ticketstatsdocs")
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

	c = session.DB("polices").C("policelocdocs")
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

	c = session.DB("polices").C("policeanchordocs")
	index = mgo.Index{
		Key:        []string{"userid"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}

	c = session.DB("tickets").C("ticketdocs")
	index = mgo.Index{
		Key:        []string{"ticketid"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}

	c = session.DB("tickets").C("ticketrangedocs")
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

////////// Main //////////
func newServer() *ticketServer {
	s := new(ticketServer)
	// s.loadFeatures(*jsonDBFile)
	// s.routeNotes = make(map[string][]*pb.RouteNote)
	return s
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

	// lis, err := net.Listen("tcp", port)
	// if err != nil {
	// 	log.Fatalf("failed to listen: %v", err)
	// }
	// s := grpc.NewServer()
	// pb.RegisterTicketServer(s, &server{})
	// // Register reflection service on gRPC server.
	// reflection.Register(s)
	// if err := s.Serve(lis); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }

	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterTicketServer(grpcServer, newServer())
	grpcServer.Serve(lis)













}
