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
 "log"
 "net"

 "golang.org/x/net/context"
 "google.golang.org/grpc"
 pb "../ticket"
 "google.golang.org/grpc/reflection"

 "gopkg.in/mgo.v2"
 "gopkg.in/mgo.v2/bson"
)

const (
	port = ":50051"
)

var dbSession *mgo.Session

// server is used to implement ticket.TicketServer.
type server struct{}

type PoliceDoc struct {
	UserID      string   `json:"userid"`
	Password    string   `json:"password"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	City        string   `json:"city"`
	Dept        string   `json:"dept"`
	Station     string   `json:"station"`
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

func (s *server) Login(ctx context.Context, loginInfo *pb.LoginRequest) (*pb.LoginReply, error) {
	session := dbSession.Copy()
	defer session.Close()
	c := session.DB("police").C("policedocs")
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

	func (s *server) CreateAccount(ctx context.Context, policeInfo *pb.AccountRequest) (*pb.AccountReply, error) {
		session := dbSession.Copy()
		defer session.Close()
		c := session.DB("police").C("policedocs")
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

	func (s *server) RecordTicket(ctx context.Context, ticketInfo *pb.TicketRequest) (*pb.TicketReply, error) {
		session := dbSession.Copy()
		defer session.Close()
		c := session.DB("ticket").C("ticketdocs")
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
				return &pb.TicketReply{RecordSuccess: false}, err
			}
			return &pb.TicketReply{RecordSuccess: false}, err
		}
		return &pb.TicketReply{RecordSuccess: true}, err
	}

	func ensureIndex(s *mgo.Session) {  
		session := s.Copy()
		defer session.Close()

		c := session.DB("police").C("policedocs")

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
		lis, err := net.Listen("tcp", port)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterTicketServer(s, &server{})
	// Register reflection service on gRPC server.
		reflection.Register(s)
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}
