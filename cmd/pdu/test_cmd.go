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

package main

import (
	"encoding/json"
	"errors"
	"github.com/pdupub/go-pdu/common"
	"github.com/pdupub/go-pdu/common/log"
	"github.com/pdupub/go-pdu/core"
	"github.com/pdupub/go-pdu/crypto"
	"github.com/pdupub/go-pdu/crypto/pdu"
	"github.com/spf13/cobra"
)

var (
	errUserNotExist       = errors.New("user not exist in this system")
	errCreateRootUserFail = errors.New("create root user fail")
)

func TestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test on pdu",
		RunE: func(_ *cobra.Command, args []string) error {
			// create root users
			retryCnt := 100
			var err error
			var Adam, Eve *core.User
			var privKeyAdam, privKeyEve *crypto.PrivateKey
			for i := 0; i < retryCnt; i++ {
				if Adam == nil {
					privKeyAdam, Adam, _ = createRootUser(true)
				}
				if Eve == nil {
					privKeyEve, Eve, _ = createRootUser(false)
				}
				if Adam != nil && Eve != nil {
					log.Trace("Adam ID :", common.Hash2String(Adam.ID()))
					log.Trace("Eve ID  :", common.Hash2String(Eve.ID()))
					break
				}
			}

			// add root user into dag
			userDAG, err := core.NewUserDag(Eve, Adam)
			if err != nil {
				log.Error("create new user dag fail, err :", err)
			} else {
				log.Info("user dag :", userDAG)
			}

			// try get Adam
			newAdam := userDAG.GetUserByID(Adam.ID())
			if newAdam != nil {
				log.Trace("get Adam from userDAG :", common.Hash2String(newAdam.ID()))
			}

			// create msg
			value := core.MsgValue{
				ContentType: core.TypeText,
				Content:     []byte("hello world!"),
			}
			msg, err := core.CreateMsg(Adam, &value, privKeyAdam)
			if err != nil {
				log.Info("create msg fail , err :", err)
			} else {
				log.Info("first msg from Adam ", "sender", common.Hash2String(msg.SenderID))
				if msg.Value.ContentType == core.TypeText {
					log.Info("first msg from Adam ", "value.content", string(msg.Value.Content))
				}
				log.Info("first msg from Adam ", "reference", msg.Reference)
				//log.Info("first msg from Adam ", "signature", msg.Signature)
			}

			// create msgDaG
			msgDAG, err := core.NewMsgDag(msg)
			if err != nil {
				log.Info("create msg dag fail, err:", err)
			} else {
				log.Trace("msg dag add msg", common.Hash2String(msgDAG.GetMsgByID(msg.ID()).ID()))
			}

			// verify msg
			verifyMsg(userDAG, msg)

			// new msg reference first msg
			value2 := core.MsgValue{
				ContentType: core.TypeText,
				Content:     []byte("hey u!"),
			}
			ref := core.MsgReference{SenderID: Adam.ID(), MsgID: msg.ID()}
			msg2, err := core.CreateMsg(Eve, &value2, privKeyEve, &ref)
			if err != nil {
				log.Error("create msg fail , err :", err)
			} else {
				log.Info("first msg from Eve ", "sender", common.Hash2String(msg2.SenderID))
				if msg2.Value.ContentType == core.TypeText {
					log.Info("first msg from Eve ", "value.content", string(msg2.Value.Content))
				}
				log.Info("first msg from Eve ", "reference", msg2.Reference)
			}

			// add msg2
			if err := msgDAG.Add(msg2); err != nil {
				log.Error("add msg2 fail, err:", err)
			} else {
				log.Trace("msg dag add msg2", common.Hash2String(msgDAG.GetMsgByID(msg2.ID()).ID()))
			}

			// verify msg
			verifyMsg(userDAG, msg2)

			// new msg reference first & second msg
			// create bod msg
			value3 := core.MsgValue{
				ContentType: core.TypeDOB,
			}
			_, pubKeyA2, err := pdu.GenKey(pdu.MultipleSignatures, 5)

			auth := core.Auth{PublicKey: *pubKeyA2}
			content, err := core.CreateDOBMsgContent("A2", "1234", &auth)
			if err != nil {
				log.Error("create bod content fail, err:", err)
			}
			content.SignByParent(Adam, *privKeyAdam)
			content.SignByParent(Eve, *privKeyEve)

			value3.Content, err = json.Marshal(content)
			log.Info()
			if err != nil {
				log.Error("content marshal fail , err:", err)
			}

			ref2 := core.MsgReference{SenderID: Eve.ID(), MsgID: msg2.ID()}
			msg3, err := core.CreateMsg(Eve, &value3, privKeyEve, &ref, &ref2)
			if err != nil {
				log.Error("create msg fail , err :", err)
			} else {
				log.Info("first dob msg ", "sender", common.Hash2String(msg3.SenderID))
				if msg3.Value.ContentType == core.TypeText {
					log.Info("first dob msg ", "value.content", string(msg3.Value.Content))
				} else if msg3.Value.ContentType == core.TypeDOB {
					log.Info("first dob msg ", "bod.content", string(msg3.Value.Content))
				}
				log.Info("first dob msg ", "reference", msg3.Reference)
				log.Info("first dob msg ", "signature", msg3.Signature)
			}

			verifyMsg(userDAG, msg3)

			// marshal and unmarshal the msg
			msgBytes, err := json.Marshal(msg3)
			//log.Info(common.Bytes2String(msgBytes))

			var msg4 core.Message
			if err != nil {
				log.Error("marshal fail err :", err)
			} else {
				err = json.Unmarshal(msgBytes, &msg4)
				if err != nil {
					log.Error("unmarshal fail err:", err)
				}
				verifyMsg(userDAG, &msg4)
				msgBytes, err = json.Marshal(msg4)
				if err != nil {
					log.Error("marshal fail err:", err)
				}
				//log.Info(common.Bytes2String(msgBytes))
			}

			// verify the signature in the content of DOBMsg
			if msg4.Value.ContentType == core.TypeDOB {
				var dobContent core.DOBMsgContent
				err = json.Unmarshal(msg4.Value.Content, &dobContent)
				if err != nil {
					log.Error("dob message can not be unmarshl, err:", err)
				}

				jsonBytes, err := json.Marshal(dobContent.User)
				if err != nil {
					log.Error("user to json fail , err:", err)
				}

				sigAdam := crypto.Signature{Signature: dobContent.Parents[1].Sig,
					PublicKey: userDAG.GetUserByID(dobContent.Parents[1].PID).Auth.PublicKey}
				sigEve := crypto.Signature{Signature: dobContent.Parents[0].Sig,
					PublicKey: userDAG.GetUserByID(dobContent.Parents[0].PID).Auth.PublicKey}

				if res, err := pdu.Verify(jsonBytes, sigAdam); err != nil || res == false {
					log.Error("verify Adam fail, err", err)
				} else {
					log.Trace("verify Adam true")
				}

				if res, err := pdu.Verify(jsonBytes, sigEve); err != nil || res == false {
					log.Trace("verify Eve fail, err", err)
				} else {
					log.Info("verify Eve true")
				}
			} else {
				log.Error("should be dob msg")
			}

			// reate new User from dob message
			// user create from msg3 and msg4 should be same user
			newUser1, err := core.CreateNewUser(msg3)
			if err != nil {
				log.Error("create user1 fail , err:", err)
			} else {
				log.Info("user1 be created, ID :", common.Hash2String(newUser1.ID()))
			}
			newUser2, err := core.CreateNewUser(&msg4)
			if err != nil {
				log.Error("create user2 fail, err:", err)
			} else {
				log.Info("user2 be created, ID :", common.Hash2String(newUser2.ID()))
			}

			if err := userDAG.Add(newUser1); err != nil {
				log.Error("dag add user1 fail, err:", err)
			} else {
				log.Trace("dag add user1 success :", common.Hash2String(userDAG.GetUserByID(newUser1.ID()).ID()))
			}

			if err := userDAG.Add(newUser2); err != core.ErrUserAlreadyExist {
				log.Error("dag add user2 fail, err should be %s, but now err : %s", core.ErrUserAlreadyExist, err)
			}

			if err := msgDAG.Add(msg3); err != nil {
				log.Error("add msg3 fail , err", err)
			} else {
				log.Trace("add msg3 success")
			}

			if err := msgDAG.Add(&msg4); err != core.ErrMsgAlreadyExist {
				log.Error("add msg4 fail, err should be %s, but now err : %s", core.ErrMsgAlreadyExist, err)
			}

			return nil
		},
	}

	return cmd
}

func verifyMsg(userDAG *core.UserDAG, msg *core.Message) {
	// verify msg
	sender := userDAG.GetUserByID(msg.SenderID)
	if sender != nil {
		msg.Signature.PubKey = sender.Auth.PubKey
		res, err := core.VerifyMsg(*msg)
		if err != nil {
			log.Error("verfiy fail, err :", err)
		} else {
			log.Trace("verify result is: ", res)
		}
	} else {
		log.Error("verify fail, err:", errUserNotExist)
	}

}

func createRootUser(male bool) (*crypto.PrivateKey, *core.User, error) {
	keyCnt := 7
	if !male {
		keyCnt = 3
	}
	privKey, pubKey, err := pdu.GenKey(pdu.MultipleSignatures, keyCnt)
	if err != nil {
		return nil, nil, err
	}

	users, err := core.CreateRootUsers(*pubKey)
	if err != nil {
		return nil, nil, err
	}

	if male && users[1] != nil {
		return privKey, users[1], nil
	} else if !male && users[0] != nil {
		return privKey, users[0], nil
	}
	return nil, nil, errCreateRootUserFail
}
