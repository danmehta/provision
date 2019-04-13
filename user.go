/*
   Copyright 2019 txn2

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
package provision

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/txn2/ack"
	"github.com/txn2/es"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const IdxUser = "user"
const EncCost = 12

// User defines a DCP user object
type User struct {
	Id            string   `json:"id"`
	Description   string   `json:"description"`
	DisplayName   string   `json:"display_name"`
	Active        bool     `json:"active"`
	Sysop         bool     `json:"sysop"`
	Password      string   `json:"password"`
	Sections      []string `json:"sections"`
	SectionsAll   bool     `json:"sections_all"`
	Accounts      []string `json:"accounts"`
	AdminAccounts []string `json:"admin_accounts"`
}

// UserResult returned from Elastic
type UserResult struct {
	es.Result
	Source User `json:"_source"`
}

// Auth for authenticating users
type Auth struct {
	Id       string `json:"id"`
	Password string `json:"password"`
}

// AccessCheck is used to configure an access check
type AccessCheck struct {
	Sections []string `json:"sections"`
	Accounts []string `json:"accounts"`
}

// UpsertUser inserts or updates a user record. Elasticsearch
// treats documents as immutable.
func (a *Api) UpsertUser(user *User) (int, es.Result, error) {
	a.Logger.Info("Upsert user record", zap.String("id", user.Id), zap.String("display_name", user.DisplayName))

	// attempt to encrypt the password if one was provided
	// otherwise populate with existing
	err := user.CheckEncryptPassword(a)
	if err != nil {
		return 500, es.Result{}, err
	}

	return a.Elastic.PutObj(fmt.Sprintf("%s/_doc/%s", a.IdxPrefix+IdxUser, user.Id), user)
}

// UpsertUserHandler
func (a *Api) UpsertUserHandler(c *gin.Context) {
	ak := ack.Gin(c)

	user := &User{}
	err := ak.UnmarshalPostAbort(user)
	if err != nil {
		a.Logger.Error("Upsert failure.", zap.Error(err))
		return
	}

	code, esResult, err := a.UpsertUser(user)
	if err != nil {
		a.Logger.Error("Upsert failure.", zap.Error(err))
		ak.SetPayloadType("ErrorMessage")
		ak.SetPayload("there was a problem upserting the user")
		ak.GinErrorAbort(500, "UpsertError", err.Error())
		return
	}

	if code < 200 || code >= 300 {

		a.Logger.Error("Es returned a non 200")
		ak.SetPayloadType("EsError")
		ak.SetPayload(esResult)
		ak.GinErrorAbort(500, "EsError", "Es returned a non 200")
		return
	}

	ak.SetPayloadType("EsResult")
	ak.GinSend(esResult)
}

// AuthUser authenticates a user with bt id and password
func (a *Api) AuthUser(auth Auth) (bool, bool, error) {

	code, userResult, err := a.GetUser(auth.Id)
	if err != nil {
		a.Logger.Error("EsError", zap.Error(err))
		return false, false, err
	}

	if code >= 400 && code < 500 {
		a.Logger.Warn("User " + auth.Id + " not found")
		return false, false, nil
	}

	if code >= 500 {
		a.Logger.Error("Received 500 code from database.")
		return false, false, errors.New("received 500 code from database")
	}

	err = bcrypt.CompareHashAndPassword([]byte(userResult.Source.Password), []byte(auth.Password))
	if err != nil {
		return true, false, nil
	}

	return true, true, nil
}

func (a *Api) AuthUserHandler(c *gin.Context) {
	ak := ack.Gin(c)

	auth := &Auth{}
	err := ak.UnmarshalPostAbort(auth)
	if err != nil {
		a.Logger.Error("AuthUser failure.", zap.Error(err))
		return
	}

	found, ok, err := a.AuthUser(*auth)
	if err != nil {
		a.Logger.Error("Auth error", zap.Error(err))
		ak.GinErrorAbort(500, "AuthError", err.Error())
		return
	}

	if !found {
		ak.SetPayloadType("AuthFailResult")
		ak.GinErrorAbort(404, "AuthFailure", "User account not found.")
		return
	}

	if ok {
		ak.SetPayloadType("AuthResult")
		ak.GinSend(true)
		return
	}

	ak.GinErrorAbort(400, "AuthFailure", "Bad password.")
}

// GetUser
func (a *Api) GetUser(id string) (int, *UserResult, error) {

	userResult := &UserResult{}

	code, ret, err := a.Elastic.Get(fmt.Sprintf("%s/_doc/%s", a.IdxPrefix+IdxUser, id))
	if err != nil {
		a.Logger.Error("EsError", zap.Error(err))
		return code, userResult, err
	}

	err = json.Unmarshal(ret, userResult)
	if err != nil {
		return code, userResult, err
	}

	return code, userResult, nil
}

// GetUserHandler gets a user by ID
func (a *Api) GetUserHandler(c *gin.Context) {
	ak := ack.Gin(c)

	id := c.Param("id")
	code, userResult, err := a.GetUser(id)
	if err != nil {
		a.Logger.Error("EsError", zap.Error(err))
		ak.SetPayloadType("EsError")
		ak.SetPayload("Error communicating with database.")
		ak.GinErrorAbort(500, "EsError", err.Error())
		return
	}

	userResult.Source.Password = "REDACTED"

	if code >= 400 && code < 500 {
		ak.SetPayload("User " + id + " not found.")
		ak.GinErrorAbort(404, "UserNotFound", "User not found")
		return
	}

	ak.SetPayloadType("UserResult")
	ak.GinSend(userResult)
}

// CheckEncryptPassword checks and encrypts the password inthe user
// object.
func (u *User) CheckEncryptPassword(api *Api) error {

	// if empty or redacted check to see if we have an
	// existing user record
	if u.Password == "" || u.Password == "REDACTED" {
		code, existingUser, err := api.GetUser(u.Id)
		if err != nil {
			return err
		}

		if code == 200 {
			// user has a password, assign it
			u.Password = existingUser.Source.Password
			return nil
		}

		if code >= 500 {
			return errors.New("bad response from Es while looking up user")
		}
	}

	// check the password
	if len(u.Password) < 10 {
		return errors.New("password must be over ten characters")
	}

	// encrypt the password
	// hash the password
	encPw, err := bcrypt.GenerateFromPassword([]byte(u.Password), EncCost)
	if err != nil {
		return err
	}

	// set the hashed password
	u.Password = string(encPw)

	return nil
}
