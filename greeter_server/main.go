package main

import (
 "flag"
 "fmt"
 "log"
 "os"
 "io"
 "net"
 "net/http"
 "sync"
 "archive/zip"
 "path/filepath"
 "strings"
 "time"

 "golang.org/x/net/context"
 "google.golang.org/grpc"

 "google.golang.org/grpc/credentials"
 "google.golang.org/grpc/grpclog"

 pb "../ticket"
 // "google.golang.org/grpc/reflection"

 "gopkg.in/mgo.v2"
 "gopkg.in/mgo.v2/bson"
)

const (
	// port = ":50051"
	INTERVAL_PERIOD time.Duration = 24 * time.Hour
	HOUR_TO_TICK int = 23
	MINUTE_TO_TICK int = 59
	SECOND_TO_TICK int = 30
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
	Station     string   `json:"station"`
}

type TicketStatsDoc struct {
	UserID                 string   `json:"userid"`
	SavedTicketCount       int32     `json:"savedticketcount"`
	UploadedTicketCount    int32     `json:"uploadedticketcount"`
}

type PoliceLocDoc struct {
	UserID       string    `json:"userid"`
	Longitude    float64   `json:"longitude"`
	Latitude     float64   `json:"latitude"`
}

type PoliceAnchorDoc struct {
	UserID         string    `json:"userid"`
	AnchorCount    int32     `json:"anchorcount"`
	Anchor0Lng     float64   `json:"anchor0lng"`
	Anchor0Lat     float64   `json:"anchor0lat"`
	Anchor1Lng     float64   `json:"anchor1lng"`
	Anchor1Lat     float64   `json:"anchor1lat"`
	Anchor2Lng     float64   `json:"anchor2lng"`
	Anchor2Lat     float64   `json:"anchor2lat"`
	Anchor3Lng     float64   `json:"anchor3lng"`
	Anchor3Lat     float64   `json:"anchor3lat"`
	Anchor4Lng     float64   `json:"anchor4lng"`
	Anchor4Lat     float64   `json:"anchor4lat"`
	Anchor5Lng     float64   `json:"anchor5lng"`
	Anchor5Lat     float64   `json:"anchor5lat"`
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
	Day             int32     `json:"day"`
	Hour            int32     `json:"hour"`
	Minute          int32     `json:"minute"`
	Address         string    `json:"address"`
	Longitude       float64   `json:"longitude"`
	Latitude        float64   `json:"latitude"`
	MapImage        []byte    `json:"mapimage"`
	FarImage        []byte    `json:"farimage"`
	CloseImage      []byte    `json:"closeimage"`
	TicketImage     []byte    `json:"ticketimage"`
}

type TicketRangeDoc struct {
	UserID           string    `json:"userid"`
	TicketIDStart    int64     `json:"ticketidstart"`
	TicketIDEnd      int64     `json:"ticketidend"`
}

var masterReceiveMap = make(map[string]chan pb.SlaveLoc, 100)
var slaveSubmitMap = make(map[string]chan pb.MasterOrder, 100)

func hasMasterReceive(name string) bool {
	usersLock.Lock()
	defer usersLock.Unlock()
	_, exists := masterReceiveMap[name]
	return exists
}
func addMasterReceive(name string, msgQ chan pb.SlaveLoc) {
	usersLock.Lock()
	defer usersLock.Unlock()
	masterReceiveMap[name] = msgQ
}
func removeMasterReceive(name string) {
	usersLock.Lock()
	defer usersLock.Unlock()
	delete(masterReceiveMap, name)
}

func hasSlaveSubmit(name string) bool {
	usersLock.Lock()
	defer usersLock.Unlock()
	_, exists := slaveSubmitMap[name]
	return exists
}
func addSlaveSubmit(name string, msgQ chan pb.MasterOrder) {
	usersLock.Lock()
	defer usersLock.Unlock()
	slaveSubmitMap[name] = msgQ
}
func removeSlaveSubmit(name string) {
	usersLock.Lock()
	defer usersLock.Unlock()
	delete(slaveSubmitMap, name)
}


func (s *ticketServer) SlaveLocSubmit(ctx context.Context, slaveLoc *pb.SlaveLoc) (*pb.MasterOrder, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("policelocs").C("policelocdocs")
	var p = PoliceLocDoc{UserID: slaveLoc.Sid,
		Longitude: slaveLoc.Longitude,
		Latitude: slaveLoc.Latitude}
  	_, err := c.UpsertId(p.UserID, &p)
	if err != nil {
		return &pb.MasterOrder{MasterOrder: "Insert/update loc failed!"}, err
	}
	fmt.Println("Sid = " + slaveLoc.Sid)
	fmt.Println("Longitude = %d", slaveLoc.Longitude)
	fmt.Println("Latitude = %d", slaveLoc.Latitude)
	return &pb.MasterOrder{MasterOrder: "Submit Loc Success!"}, nil
}

func Min(x, y int32) int32 {
    if x < y {
        return x
    }
    return y
}

func (s *ticketServer) SlaveAnchorSubmit(ctx context.Context, slaveLoc *pb.SlaveLoc) (*pb.MasterOrder, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("policelocs").C("policeanchordocs")

	var policeAnchorDoc PoliceAnchorDoc
	err := c.Find(bson.M{"userid": slaveLoc.Sid}).One(&policeAnchorDoc)
	if err != nil || policeAnchorDoc.UserID == "" {
		var p = PoliceAnchorDoc{UserID: slaveLoc.Sid,
			AnchorCount: 1,
			Anchor0Lng: 0.0,
			Anchor0Lat: 0.0,
			Anchor1Lng: 0.0,
			Anchor1Lat: 0.0,
			Anchor2Lng: 0.0,
			Anchor2Lat: 0.0,
			Anchor3Lng: 0.0,
			Anchor3Lat: 0.0,
			Anchor4Lng: 0.0,
			Anchor4Lat: 0.0,
			Anchor5Lng: slaveLoc.Longitude,
			Anchor5Lat: slaveLoc.Latitude}
	  	_, err = c.UpsertId(p.UserID, &p)
		if err != nil {
			return &pb.MasterOrder{MasterOrder: "First insert/update anchor failed!"}, err
		}
		return &pb.MasterOrder{MasterOrder: "First Submit Anchor Success!"}, nil
	}

	var p = PoliceAnchorDoc{UserID: slaveLoc.Sid,
		AnchorCount: Min(policeAnchorDoc.AnchorCount + 1, 6),
		Anchor0Lng: policeAnchorDoc.Anchor1Lng,
		Anchor0Lat: policeAnchorDoc.Anchor1Lat,
		Anchor1Lng: policeAnchorDoc.Anchor2Lng,
		Anchor1Lat: policeAnchorDoc.Anchor2Lat,
		Anchor2Lng: policeAnchorDoc.Anchor3Lng,
		Anchor2Lat: policeAnchorDoc.Anchor3Lat,
		Anchor3Lng: policeAnchorDoc.Anchor4Lng,
		Anchor3Lat: policeAnchorDoc.Anchor4Lat,
		Anchor4Lng: policeAnchorDoc.Anchor5Lng,
		Anchor4Lat: policeAnchorDoc.Anchor5Lat,
		Anchor5Lng: slaveLoc.Longitude,
		Anchor5Lat: slaveLoc.Latitude}

  	_, err = c.UpsertId(p.UserID, &p)
	if err != nil {
		return &pb.MasterOrder{MasterOrder: "Second - Sixth insert/update anchor failed!"}, err
	}
	return &pb.MasterOrder{MasterOrder: "Second - Sixth Submit Anchor Success!"}, nil
}


func (s *ticketServer) MasterReceive(stream pb.Ticket_MasterReceiveServer) error {
	in, err := stream.Recv()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	mReceiveMailbox := make(chan pb.SlaveLoc, 100)
	if in.Mid == "" {
		return fmt.Errorf("No MasterReceive ID!")
	} else {
		if !hasMasterReceive(in.Mid) {
			addMasterReceive(in.Mid, mReceiveMailbox)		
		}
	}
	slaveLoc := <-mReceiveMailbox
	if err := stream.Send(&slaveLoc); err != nil {
		return err
	}
	return nil
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
		PoliceStation: policeDoc.Station}, nil
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
		Station: policeInfo.PoliceStation})
	if err != nil {
		if mgo.IsDup(err) {
			return &pb.AccountReply{CreateSuccess: false}, err
		}
		return &pb.AccountReply{CreateSuccess: false}, err
	}
	return &pb.AccountReply{CreateSuccess: true}, nil
}

func (s *ticketServer) HareLogin(ctx context.Context, loginInfo *pb.LoginRequest) (*pb.LoginReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("policedocs")
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
		log.Println("Wrong password!")
		return &pb.LoginReply{LoginSuccess: false}, err
	}
	return &pb.LoginReply{  LoginSuccess: true,
		PoliceName: policeDoc.Name,
		PoliceType: policeDoc.Type,
		PoliceCity: policeDoc.City,
		PoliceDept: policeDoc.Dept,
		PoliceStation: policeDoc.Station}, nil
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
		Station: policeInfo.PoliceStation})
	if err != nil {
		if mgo.IsDup(err) {
			return &pb.AccountReply{CreateSuccess: false}, err
		}
		return &pb.AccountReply{CreateSuccess: false}, err
	}
	return &pb.AccountReply{CreateSuccess: true}, nil
}

func (s *ticketServer) RecordTicket(ctx context.Context, ticketInfo *pb.TicketDetails) (*pb.RecordReply, error) {
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

	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("tickets").C("ticketdocs")
	err := c.Insert(&TicketDoc{TicketID: ticketInfo.TicketId,
		UserID: ticketInfo.UserId,
		LicenseNum: ticketInfo.LicenseNum,
		LicenseColor: ticketInfo.LicenseColor,
		VehicleType: ticketInfo.VehicleType,
		VehicleColor: ticketInfo.VehicleColor,
		Year: ticketInfo.Year,
		Month: ticketInfo.Month,
		Day: ticketInfo.Day,
		Hour: ticketInfo.Hour,
		Minute: ticketInfo.Minute,
		Address: ticketInfo.Address,
		Longitude: ticketInfo.Longitude,
		Latitude: ticketInfo.Latitude,
		MapImage: ticketInfo.MapImage,
		FarImage: ticketInfo.FarImage,
		CloseImage: ticketInfo.CloseImage,
		TicketImage: ticketInfo.TicketImage})
	if err != nil {
		if mgo.IsDup(err) {
			return &pb.RecordReply{RecordSuccess: false}, err
		}
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

func (s *ticketServer) SubmitTicketStats(ctx context.Context, ticketStats *pb.TicketStats) (*pb.StatsReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("ticketstatsdocs")
	var p = TicketStatsDoc{UserID: ticketStats.Sid,
		SavedTicketCount: ticketStats.SavedTicketCount,
		UploadedTicketCount: ticketStats.UploadedTicketCount}
  	_, err := c.UpsertId(p.UserID, &p)
	if err != nil {
		return &pb.StatsReply{StatsSuccess: false}, err
	}
	return &pb.StatsReply{StatsSuccess: true}, nil
}

func (s *ticketServer) PullLocation(rect *pb.PullLocRequest, stream pb.Ticket_PullLocationServer) error {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("policelocs").C("policelocdocs")

    var slaveLocs []PoliceLocDoc
    err := c.Find(bson.M{}).All(&slaveLocs)
    if err != nil {
		log.Println("Failed find police locations: ", err)
		return err
    }
	for _, slaveLoc := range slaveLocs {
		if err := stream.Send(&pb.SlaveLoc{  Sid: slaveLoc.UserID,
					Longitude: slaveLoc.Longitude,
					Latitude: slaveLoc.Latitude}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ticketServer) PullAnchors(ctx context.Context, pullAnchorRequest *pb.PullAnchorRequest) (*pb.SlaveAnchors, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("policelocs").C("policeanchordocs")
	var slaveAnchorsDoc PoliceAnchorDoc
	err := c.Find(bson.M{"userid": pullAnchorRequest.Sid}).One(&slaveAnchorsDoc)
	if err != nil {
		log.Println("Failed find police anchors: ", err)
		return &pb.SlaveAnchors{AnchorCount: -1}, err
	}
	if slaveAnchorsDoc.UserID == "" {
		log.Println("Police not found:!")
		return &pb.SlaveAnchors{AnchorCount: -1}, err
	}
	return &pb.SlaveAnchors{  Sid: slaveAnchorsDoc.UserID,
					AnchorCount: slaveAnchorsDoc.AnchorCount,
					Anchor0Lng: slaveAnchorsDoc.Anchor0Lng,
					Anchor0Lat: slaveAnchorsDoc.Anchor0Lat,
					Anchor1Lng: slaveAnchorsDoc.Anchor1Lng,
					Anchor1Lat: slaveAnchorsDoc.Anchor1Lat,
					Anchor2Lng: slaveAnchorsDoc.Anchor2Lng,
					Anchor2Lat: slaveAnchorsDoc.Anchor2Lat,
					Anchor3Lng: slaveAnchorsDoc.Anchor3Lng,
					Anchor3Lat: slaveAnchorsDoc.Anchor3Lat,
					Anchor4Lng: slaveAnchorsDoc.Anchor4Lng,
					Anchor4Lat: slaveAnchorsDoc.Anchor4Lat,
					Anchor5Lng: slaveAnchorsDoc.Anchor5Lng,
					Anchor5Lat: slaveAnchorsDoc.Anchor5Lat}, nil
}

func (s *ticketServer) PullTicketStats(ctx context.Context, pullTicketStatsRequest *pb.PullTicketStatsRequest) (*pb.TicketStats, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("polices").C("ticketstatsdocs")
	var ticketStatsDoc TicketStatsDoc
	err := c.Find(bson.M{"userid": pullTicketStatsRequest.Sid}).One(&ticketStatsDoc)
	if err != nil {
		log.Println("Failed find ticket stats: ", err)
		return &pb.TicketStats{SavedTicketCount: -1}, err
	}
	if ticketStatsDoc.UserID == "" {
		log.Println("Police not found:!")
		return &pb.TicketStats{SavedTicketCount: -1}, err
	}
	return &pb.TicketStats{  Sid: ticketStatsDoc.UserID,
					SavedTicketCount: ticketStatsDoc.SavedTicketCount,
					UploadedTicketCount: ticketStatsDoc.UploadedTicketCount}, nil
}

func (s *ticketServer) PullTicket(rect *pb.PullTicketRequest, stream pb.Ticket_PullTicketServer) error {
	return nil
}

////////// Read and Write Files //////////
func createDirectory(directoryPath string) {
	//choose your permissions well
	err := os.MkdirAll(directoryPath, 0777)
 
	//check if you need to panic, fallback or report
	if err != nil {
		fmt.Println(err.Error())
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
		fmt.Println(err.Error())
		return //must return here for defer statements to be called
	}

	// save changes
	err = file.Sync()
	if err != nil {
		fmt.Println(err.Error())
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
		fmt.Println(err.Error())
		return //must return here for defer statements to be called
	}

	// save changes
	err = file.Sync()
	if err != nil {
		fmt.Println(err.Error())
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
		fmt.Println(err.Error())
		os.Exit(0)
	}
}


func zipit(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
	return err
}

func unzip(archive, target string) error {
    reader, err := zip.OpenReader(archive)
    if err != nil {
        return err
    }

    if err := os.MkdirAll(target, 0755); err != nil {
        return err
    }

    for _, file := range reader.File {
        path := filepath.Join(target, file.Name)
        if file.FileInfo().IsDir() {
            os.MkdirAll(path, file.Mode())
            continue
        }

        fileReader, err := file.Open()
        if err != nil {
            if fileReader != nil {
                fileReader.Close()
            }
            return err
        }

        targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
        if err != nil {
            fileReader.Close()
            if targetFile != nil {
                targetFile.Close()
            }
            return err
        }

        if _, err := io.Copy(targetFile, fileReader); err != nil {
            fileReader.Close()
            targetFile.Close()
            return err
        }

        fileReader.Close()
        targetFile.Close()
    }

    return nil
}

func zipDayTickets(dayName string) {
	os.Chdir(fmt.Sprintf("./tickets/%s", dayName))
	zipit(".", fmt.Sprintf("../%s.zip", dayName))
	os.Chdir("../..")
}


////////// Main //////////
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

	c = session.DB("policelocs").C("policelocdocs")
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

	c = session.DB("policelocs").C("policeanchordocs")
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


}

func newServer() *ticketServer {
	s := new(ticketServer)
	// s.loadFeatures(*jsonDBFile)
	// s.routeNotes = make(map[string][]*pb.RouteNote)
	return s
}

func updateTicker() *time.Ticker {
	nextTick := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), HOUR_TO_TICK, MINUTE_TO_TICK, SECOND_TO_TICK, 0, time.Local)
	if !nextTick.After(time.Now()) {
		nextTick = nextTick.Add(INTERVAL_PERIOD)
	}
	fmt.Println(nextTick, "- next zip")
	diff := nextTick.Sub(time.Now())
	return time.NewTicker(diff)
}

func main() {

	// zipDayTickets("2017-8-6")
	go func() {
		ticker := updateTicker()
	    for {
			<-ticker.C
			folderName := fmt.Sprintf("%d-%d-%d", time.Now().Year(), int(time.Now().Month()), time.Now().Day())
			zipDayTickets(folderName)
			fmt.Println(time.Now(), fmt.Sprintf("- just zipped %s", folderName))
			ticker = updateTicker()
	    }
    }()

	go func() {
		http.Handle("/", http.FileServer(http.Dir("./")))
		http.ListenAndServe(":8080", nil)
	}()


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
