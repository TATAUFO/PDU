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
	"encoding/json"

	"github.com/pdupub/go-pdu/common"
	"github.com/pdupub/go-pdu/crypto"
	"github.com/pdupub/go-pdu/crypto/utils"
)

// ContentBirth is the birth msg content, which can create new user
type ContentBirth struct {
	User    User
	Parents [2]ParentSig
}

// ParentSig contains the signature from both parents
type ParentSig struct {
	UserID    common.Hash
	Signature []byte
}

// CreateContentBirth create the birth msg content , which usually from the new user, not sign by parents yet
func CreateContentBirth(name string, extra string, auth *Auth) (*ContentBirth, error) {
	user := User{Name: name, BirthExtra: extra, Auth: auth}
	return &ContentBirth{User: user}, nil

}

// SignByParent used to sign the birth msg by both parents
func (mv *ContentBirth) SignByParent(user *User, privKey crypto.PrivateKey) error {

	jsonByte, err := json.Marshal(mv.User)
	if err != nil {
		return err
	}
	var signature *crypto.Signature
	engine, err := utils.SelectEngine(privKey.Source)
	if err != nil {
		return err
	}

	signature, err = engine.Sign(jsonByte, &privKey)
	if err != nil {
		return err
	}

	if user.Gender() {
		mv.Parents[1] = ParentSig{UserID: user.ID(), Signature: signature.Signature}
	} else {
		mv.Parents[0] = ParentSig{UserID: user.ID(), Signature: signature.Signature}
	}
	return nil
}
