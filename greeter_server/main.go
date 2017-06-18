/*
 *
 * Copyright 2015, Google Inc.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *     * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *     * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

package main

import (
 "flag"
 "fmt"
 "log"
 "io"
 "net"
 "sync"

 "golang.org/x/net/context"
 "google.golang.org/grpc"

 "google.golang.org/grpc/credentials"
 "google.golang.org/grpc/grpclog"

 pb "../ticket"
 // "google.golang.org/grpc/reflection"

 "gopkg.in/mgo.v2"
 "gopkg.in/mgo.v2/bson"
)

// const (
// 	port = ":50051"
// )

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
	fmt.Println("Entered PullAnchors")
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

// func (s *ticketServer) PullAnchors(rect *pb.PullLocRequest, stream pb.Ticket_PullAnchorsServer) error {
// 	session := dbSession.Copy()
// 	defer session.Close()
// 	c := session.DB("policelocs").C("policeanchordocs")

//     var slaveAnchorsDoc []PoliceAnchorDoc
//     err := c.Find(bson.M{}).All(&slaveAnchorsDoc)
//     if err != nil {
// 		log.Println("Failed find police anchors: ", err)
// 		return err
//     }
// 	for _, slaveAnchor := range slaveAnchorsDoc {
// 		if err := stream.Send(&pb.SlaveAnchors{  Sid: slaveAnchor.UserID,
// 					AnchorCount: slaveAnchor.AnchorCount,
// 					Anchor0Lng: slaveAnchor.Anchor0Lng,
// 					Anchor0Lat: slaveAnchor.Anchor0Lat,
// 					Anchor1Lng: slaveAnchor.Anchor1Lng,
// 					Anchor1Lat: slaveAnchor.Anchor1Lat,
// 					Anchor2Lng: slaveAnchor.Anchor2Lng,
// 					Anchor2Lat: slaveAnchor.Anchor2Lat,
// 					Anchor3Lng: slaveAnchor.Anchor3Lng,
// 					Anchor3Lat: slaveAnchor.Anchor3Lat,
// 					Anchor4Lng: slaveAnchor.Anchor4Lng,
// 					Anchor4Lat: slaveAnchor.Anchor4Lat,
// 					Anchor5Lng: slaveAnchor.Anchor5Lng,
// 					Anchor5Lat: slaveAnchor.Anchor5Lat}); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

func (s *ticketServer) PullTicket(rect *pb.PullTicketRequest, stream pb.Ticket_PullTicketServer) error {
	return nil
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


}

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
