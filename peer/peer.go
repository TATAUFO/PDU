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

package peer

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/pdupub/go-pdu/common"
	"github.com/pdupub/go-pdu/core"
	"github.com/pdupub/go-pdu/galaxy"
	"golang.org/x/net/websocket"
)

var (
	errPeerNotReachable = errors.New("this peer not reachable right now")
	errArgsNotSupport   = errors.New("arguments not support")
	errMsgsNeedSplit    = errors.New("messages need split into waves")
)

const (
	// MaxMsgCountPerWave is the max number of msg per wave
	MaxMsgCountPerWave = 2
)

// Peer contain the info of websocket connection
type Peer struct {
	IP       string      `json:"ip"`
	Port     uint64      `json:"port"`
	NodeKey  string      `json:"nodeKey"`
	UserID   common.Hash `json:"userID"`
	Verified bool        `json:"verified"`
	Conn     *websocket.Conn
}

// New create new Peer
func New(ip string, port uint64, nodeKey string) (*Peer, error) {
	return &Peer{IP: ip, Port: port, NodeKey: nodeKey}, nil
}

// ID return key of peer
func (p *Peer) ID() common.Hash {
	hash := sha256.New()
	hash.Reset()
	hash.Write([]byte(p.Url()))
	return common.Bytes2Hash(hash.Sum(nil))
}

// SetUserID set author of this peer
func (p *Peer) SetUserID(userID common.Hash) {
	if p.UserID != userID {
		p.UserID = userID
		p.Verified = false
	}
}

// SetVerified set verified is true
func (p *Peer) SetVerified() {
	p.Verified = true
}

// Dial build ws connection
func (p *Peer) Dial() error {
	conn, err := websocket.Dial(p.Url(), "", p.origin())
	if err != nil {
		return err
	}
	p.Conn = conn
	return nil
}

// Close the ws connection,
func (p *Peer) Close() error {
	if p.Conn != nil {
		return p.Conn.Close()
	}
	return nil
}

// Url show the Peer ws url address
func (p Peer) Url() string {
	return fmt.Sprintf("ws://%s:%d/%s", p.IP, p.Port, p.NodeKey)
}

// Address is UserID@IP:port/nodeKey
func (p Peer) Address() string {
	// todo : address without p.UserID or not verified
	return fmt.Sprintf("%s@%s:%d/%s", common.Hash2String(p.UserID), p.IP, p.Port, p.NodeKey)
}

// Connected return true if this peer is connected right now
func (p *Peer) Connected() bool {
	if p.Conn != nil {
		return true
	}
	return false
}

func (p *Peer) send(wave galaxy.Wave) error {
	_, err := galaxy.SendWave(p.Conn, wave)
	if err != nil {
		p.Conn = nil
		return err
	}
	return nil
}

// SendQuestion is used to send question to peer
func (p *Peer) SendQuestion(waveID common.Hash, cmd string, args ...interface{}) error {
	if !p.Connected() {
		return errPeerNotReachable
	}

	newArgs, err := p.buildArgs(args...)
	if err != nil {
		return err
	}
	wave := &galaxy.WaveQuestion{
		WaveID: waveID,
		Cmd:    cmd,
		Args:   newArgs,
	}
	return p.send(wave)
}

func (p Peer) buildArgs(args ...interface{}) (result [][]byte, err error) {
	for _, arg := range args {
		var item []byte
		switch arg.(type) {
		case uint64:
			item = new(big.Int).SetUint64(arg.(uint64)).Bytes()
		case string:
			item = []byte(arg.(string))
		case *big.Int:
			item = arg.(*big.Int).Bytes()
		case []byte:
			item = arg.([]byte)
		case common.Hash:
			item = common.Hash2Bytes(arg.(common.Hash))
		default:
			return nil, errArgsNotSupport
		}
		result = append(result, item)
	}
	return result, nil
}

// SendMsg is used to send msg to peer
func (p *Peer) SendMsg(waveID common.Hash, msg *core.Message) error {
	return p.SendMsgs(waveID, []*core.Message{msg})
}

// SendMsgs is used to send mulitiple msgs
func (p *Peer) SendMsgs(waveID common.Hash, msgs []*core.Message) error {
	if len(msgs) > MaxMsgCountPerWave {
		msgs = msgs[:MaxMsgCountPerWave]
	}
	if !p.Connected() {
		return errPeerNotReachable
	}
	var msgsB [][]byte
	for _, msg := range msgs {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		msgsB = append(msgsB, msgBytes)
	}
	wave := &galaxy.WaveMessages{
		WaveID: waveID,
		Msgs:   msgsB,
	}
	return p.send(wave)
}

// SendPeers is used to send peers of local node
func (p *Peer) SendPeers(waveID common.Hash, pm map[common.Hash]*Peer, localPeer *Peer) error {
	if !p.Connected() {
		return errPeerNotReachable
	}
	var targetPeers [][]byte
	for _, item := range pm {
		if item.Conn == nil {
			continue
		}
		nodeAddress, err := json.Marshal(item)
		if err != nil {
			return err
		}
		targetPeers = append(targetPeers, nodeAddress)
	}

	localAddress, err := json.Marshal(localPeer)
	if err != nil {
		return err
	}
	targetPeers = append(targetPeers, localAddress)
	wave := &galaxy.WavePeers{
		WaveID: waveID,
		Peers:  targetPeers,
	}
	return p.send(wave)
}

// SendRoots is used to send 2 roots to peer
func (p *Peer) SendRoots(waveID common.Hash, user0, user1 *core.User) error {
	if !p.Connected() {
		return errPeerNotReachable
	}
	var users [2]*core.User
	users[0] = user0
	users[1] = user1
	wave := &galaxy.WaveRoots{
		WaveID: waveID,
		Users:  users,
	}

	return p.send(wave)
}

// SendPing is used for ping pong, send ping to peer
func (p *Peer) SendPing(waveID common.Hash) error {
	if !p.Connected() {
		return errPeerNotReachable
	}
	wave := &galaxy.WavePing{WaveID: waveID}
	return p.send(wave)
}

// SendPong is used for ping pong, send pong back to peer
func (p *Peer) SendPong(waveID common.Hash) error {
	if !p.Connected() {
		return errPeerNotReachable
	}
	wave := &galaxy.WavePong{WaveID: waveID}
	return p.send(wave)
}

// origin used when peer dial
func (p Peer) origin() string {
	return fmt.Sprintf("http://%s:%d/", p.IP, p.Port)
}

// SendErr is send the error of request back
func (p *Peer) SendErr(waveID common.Hash, err error) error {
	if !p.Connected() {
		return errPeerNotReachable
	}
	wave := &galaxy.WaveErr{
		WaveID: waveID,
		Err:    err.Error(),
	}
	return p.send(wave)
}
