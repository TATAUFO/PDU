// Copyright 2019 The PDU Authors
// This file is part of the PDU library.
//
// The PDU library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The PDU library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the PDU library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"errors"
	"github.com/pdupub/go-pdu/common"
	"github.com/pdupub/go-pdu/dag"
)

var (
	ErrMsgFromInvalidUser = errors.New("msg from invalid user")
	ErrMsgAlreadyExist    = errors.New("msg already exist")
	ErrMsgNotFound        = errors.New("msg not found")
	ErrTPAlreadyExist     = errors.New("time proof already exist")
)

// UniverseGroup
// value of vertex contain in dag is user status
type UniverseGroup struct {
	dag *dag.DAG
}

const (
	UserStatusNormal = iota
)

// UserState contain the information can be modified by local user
// such as local user want to block some user
type UserState struct {
	publicState int //
}

type SpaceTime struct {
	maxTimeSequence uint64
	timeProofD      *dag.DAG // msg.id  : time sequence
	userStateD      *dag.DAG // user.id : state of user
}

// Universe
// Vertex of utD is time proof, ID of Vertex is the ID of user which msg set as the time proof,
// Reference of Vertex is source which this time proof split from
// Vertex of ugD is group, ID of Vertex is the ID of time proof which this group valid,
// Reference of Vertex is same with time proof reference
type Universe struct {
	msgD  *dag.DAG `json:"messageDAG"`       // contain all messages valid in any universe (time proof)
	group *Group   `json:"group"`            // contain all users valid in any universe (time proof)
	stD   *dag.DAG `json:"spaceTimeDAG"`     // contain all space time
	ugD   *dag.DAG `json:"universeGroupDAG"` // contain all user group
}

// NewUniverse create Universe
// the msg will also be used to create time proof as msg.SenderID
func NewUniverse(group *Group, msg *Message) (*Universe, error) {
	// check msg sender from valid user
	if nil == group.GetUserByID(msg.SenderID) {
		return nil, ErrMsgFromInvalidUser
	}
	// build msg dag
	msgVertex, err := dag.NewVertex(msg.ID(), msg)
	if err != nil {
		return nil, err
	}
	msgD, err := dag.NewDAG(msgVertex)
	if err != nil {
		return nil, err
	}
	// build time proof
	st, err := createSpaceTime(msg)
	if err != nil {
		return nil, err
	}
	stVertex, err := dag.NewVertex(msg.SenderID, st)
	if err != nil {
		return nil, err
	}
	stD, err := dag.NewDAG(stVertex)
	if err != nil {
		return nil, err
	}
	// build user group
	ug, err := createUniverseGroup(group, msg)
	ugVertex, err := dag.NewVertex(msg.SenderID, ug)
	if err != nil {
		return nil, err
	}
	ugD, err := dag.NewDAG(ugVertex)
	if err != nil {
		return nil, err
	}

	Universe := Universe{
		msgD:  msgD,
		group: group,
		stD:   stD,
		ugD:   ugD}
	return &Universe, nil
}

// CheckUserValid check if the user valid in this Universe
// the msg.SenderID must valid in at least one tpDAG
func (md *Universe) CheckUserValid(userID common.Hash) bool {
	if nil != md.group.GetUserByID(userID) {
		return true
	}
	return false
}

// findValidUniverse return
func (md *Universe) findValidUniverse(senderID common.Hash) []interface{} {
	var ugs []interface{}
	for _, k := range md.ugD.GetIDs() {
		if v := md.ugD.GetVertex(k); v != nil {
			ugs = append(ugs, k)
		}
	}
	return ugs
}

// AddTimeProof will get all messages save in Universe with same msg.SenderID
// and build the time proof by those messages
func (md *Universe) AddSpaceTime(msg *Message) error {
	if md.GetMsgByID(msg.ID()) == nil {
		return ErrMsgNotFound
	}
	if nil != md.stD.GetVertex(msg.SenderID) {
		return ErrTPAlreadyExist
	}
	if !md.CheckUserValid(msg.SenderID) {
		return ErrMsgFromInvalidUser
	}
	// update time proof
	initialize := true
	for _, id := range md.msgD.GetIDs() {
		if msgTP := md.GetMsgByID(id); msgTP != nil && msgTP.SenderID == msg.SenderID {
			if initialize {
				tp, err := createSpaceTime(msgTP)
				if err != nil {
					return err
				}
				tpVertex, err := dag.NewVertex(msg.SenderID, tp, md.findValidUniverse(msg.SenderID)...)
				if err != nil {
					return err
				}
				if err = md.stD.AddVertex(tpVertex); err != nil {
					return err
				}
				initialize = false
			} else {
				if err := md.updateTimeProof(msgTP); err != nil {
					return err
				}
			}
		}
	}
	// update user group
	group := md.createUniverseGroup(msg.SenderID)
	ugVertex, err := dag.NewVertex(msg.SenderID, group, md.findValidUniverse(msg.SenderID)...)
	if err != nil {
		return err
	}
	md.ugD.AddVertex(ugVertex)
	return nil
}

//
func (md *Universe) createUniverseGroup(userID common.Hash) *UniverseGroup {
	// todo : the new UserDAG should contain all parent users
	// todo : in all userDag which this userID is valid
	// todo : need deep copy
	return nil
}

// GetUserDAG return userDAG by time proof userID
func (md *Universe) GetUserDAG() *Group {
	return md.group
}

// GetMsgByID will return the msg by msg.ID()
// nil will be return if msg not exist
func (md *Universe) GetMsgByID(mid interface{}) *Message {
	if v := md.msgD.GetVertex(mid); v != nil {
		return v.Value().(*Message)
	} else {
		return nil
	}
}

// Add will check if the msg from valid user,
// add new msg into Universe, and update time proof if
// msg.SenderID is belong to time proof
func (md *Universe) Add(msg *Message) error {
	// check
	if md.GetMsgByID(msg.ID()) != nil {
		return ErrMsgAlreadyExist
	}
	if !md.CheckUserValid(msg.SenderID) {
		return ErrMsgFromInvalidUser
	}
	// update dag
	var refs []interface{}
	for _, r := range msg.Reference {
		refs = append(refs, r.MsgID)
	}
	msgVertex, err := dag.NewVertex(msg.ID(), msg, refs...)
	if err != nil {
		return err
	}
	err = md.msgD.AddVertex(msgVertex)
	if err != nil {
		return err
	}
	// update tp
	err = md.updateTimeProof(msg)
	if err != nil {
		return err
	}
	// process the msg
	err = md.processMsg(msg)
	if err != nil {
		return err
	}
	return nil
}

func (md *Universe) processMsg(msg *Message) error {
	switch msg.Value.ContentType {
	case TypeText:
		return nil
	case TypeDOB:
		user, err := CreateNewUser(msg)
		if err != nil {
			return err
		}
		// todo :check the valid time proof for parents in each timeproof
		// user may not can be add to all userMap
		err = md.group.Add(user)
		if err != nil {
			return err
		}
		//for _, v := range md.ugD {
		/*
			for _, k := range md.ugD.GetIDs() {
				err = md.ugD.GetVertex(k).Value().(*Group).Add(user)
				if err != nil {
					return err
				}
			}*/
	}
	return nil
}

func createSpaceTime(msg *Message, users ...*User) (*SpaceTime, error) {
	timeVertex, err := dag.NewVertex(msg.ID(), uint64(1))
	if err != nil {
		return nil, err
	}
	timeProofDag, err := dag.NewDAG(timeVertex)
	if err != nil {
		return nil, err
	}
	userStateD, err := dag.NewDAG()
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		userStateVertex, err := dag.NewVertex(user.ID(), UserState{publicState: UserStatusNormal})
		if err != nil {
			return nil, err
		}
		userStateD.AddVertex(userStateVertex)
	}

	return &SpaceTime{maxTimeSequence: timeVertex.Value().(uint64), timeProofD: timeProofDag, userStateD: userStateD}, nil
}

func createUniverseGroup(group *Group, msg *Message) (*UniverseGroup, error) {
	var ugDag *dag.DAG
	if group != nil {
		for i, k := range group.dag.GetIDs() {
			ugV, err := dag.NewVertex(k, UserStatusNormal)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				ugDag, err = dag.NewDAG(ugV)
				if err != nil {
					return nil, err
				}
			} else {
				err = ugDag.AddVertex(ugV)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if msg != nil {
		ugV, err := dag.NewVertex(msg.SenderID, UserStatusNormal)
		if err != nil {
			return nil, err
		}
		if ugDag == nil {
			ugDag, err = dag.NewDAG(ugV)
			if err != nil {
				return nil, err
			}
		} else if ugDag.GetVertex(msg.SenderID) == nil {
			err = ugDag.AddVertex(ugV)
			if err != nil {
				return nil, err
			}
		}
	}
	return &UniverseGroup{dag: ugDag}, nil
}

func (md *Universe) updateTimeProof(msg *Message) error {
	if vertex := md.stD.GetVertex(msg.SenderID); vertex != nil {

		tp := vertex.Value().(*SpaceTime)
		var currentSeq uint64 = 1
		for _, r := range msg.Reference {
			if r.SenderID == msg.SenderID {
				refSeq := tp.timeProofD.GetVertex(r.MsgID).Value().(uint64)
				if currentSeq <= refSeq {
					currentSeq = refSeq + 1
				}
			}
		}
		timeVertex, err := dag.NewVertex(msg.ID(), currentSeq)
		if err != nil {
			return err
		}

		if err := tp.timeProofD.AddVertex(timeVertex); err != nil {
			return err
		} else if currentSeq > tp.maxTimeSequence {
			tp.maxTimeSequence = currentSeq
		}
	}
	return nil
}

// GetMaxSeq will return the max time proof sequence for
// time proof by the userID
func (md *Universe) GetMaxSeq(userID common.Hash) uint64 {
	if vertex := md.stD.GetVertex(userID); vertex != nil {
		return vertex.Value().(*SpaceTime).maxTimeSequence
	} else {
		return 0
	}
}
