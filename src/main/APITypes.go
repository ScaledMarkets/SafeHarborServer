/*******************************************************************************
 * The data types needed by the handler functions.
 */

package main

import (
	"net/url"
	"fmt"
	"errors"
)

/*******************************************************************************
 * 
 */
type BaseType struct {
}

type ResponseInterfaceType interface {
	asResponse() string
}

func (b *BaseType) asResponse() string {
	return ""
}

var _ ResponseInterfaceType = &BaseType{}

/*******************************************************************************
 * 
 */
type Result struct {
	BaseType
	Status int
	Message string
}

func (result *Result) asResponse() string {
	return fmt.Sprintf("Status=%s\nMessage=%s", result.Status, result.Message)
}

/*******************************************************************************
 * 
 */
type FailureDesc struct {
	BaseType
	reason string
}

func (failureDesc *FailureDesc) asResponse() string {
	return ""
}

/*******************************************************************************
 * Types and functions for credentials.
 */
type Credentials struct {
	BaseType
	userid string
	pswd string
}

func NewCredentials(uid string, pwd string) *Credentials {
	return &Credentials{
		userid: uid,
		pswd: pwd,
	}
}

func GetCredentials(values url.Values) *Credentials {
	var userid string = GetRequiredPOSTFieldValue(values, "userid")
	var pswd string = GetRequiredPOSTFieldValue(values, "pswd")
	return NewCredentials(userid, pswd)
}

func (creds *Credentials) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
type SessionToken struct {
	BaseType
	UniqueSessionId string
	authenticatedUserid string
}

func NewSessionToken(sessionId string, userid string) *SessionToken {
	return &SessionToken{
		UniqueSessionId: sessionId,
		authenticatedUserid: userid,
	}
}

func (sessionToken *SessionToken) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
type GroupDesc struct {
	BaseType
}

func (groupDesc *GroupDesc) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
type UserInfo struct {
	BaseType
	Id string
}

func NewUserInfo(id string) *UserInfo {
	return &UserInfo{
		Id: id,
	}
}

func GetUserInfo(values url.Values) *UserInfo {
	var id string = GetRequiredPOSTFieldValue(values, "Id")
	return NewUserInfo(id)
}

func (userInfo *UserInfo) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
type UserDesc struct {
	BaseType
	UserId string
}

func (userDesc *UserDesc) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
type RealmDesc struct {
	BaseType
	Id string
	Name string
}

func NewRealmDesc(id string, name string) *RealmDesc {
	return &RealmDesc{
		Id: id,
		Name: name,
	}
}

func (realmDesc *RealmDesc) asResponse() string {
	return fmt.Sprintf("Id=%s\nName=%s\n", realmDesc.Id, realmDesc.Name)
}

/*******************************************************************************
 * 
 */
type RealmInfo struct {
	BaseType
	Name string
}

func NewRealmInfo(name string) *RealmInfo {
	return &RealmInfo{
		Name: name,
	}
}

func GetRealmInfo(values url.Values) *RealmInfo {
	var name = GetRequiredPOSTFieldValue(values, "Name")
	return NewRealmInfo(name)
}

func (realmInfo *RealmInfo) asResponse() string {
	return fmt.Sprintf("Name=%s", realmInfo.Name)
}

/*******************************************************************************
 * 
 */
type RepoDesc struct {
	BaseType
	Id string
	RealmId string
	Name string
}

func NewRepoDesc(id string, realmId string, name string) *RepoDesc {
	return &RepoDesc{
		Id: id,
		RealmId: realmId,
		Name: name,
	}
}

func (repoDesc *RepoDesc) asResponse() string {
	return fmt.Sprintf("Id=%s\nRealmId=%s\nName=%s\n",
		repoDesc.Id, repoDesc.RealmId, repoDesc.Name)
}

/*******************************************************************************
 * 
 */
type DockerfileDesc struct {
	BaseType
}

func (dockerfileDesc *DockerfileDesc) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
type ImageDesc struct {
	BaseType
}

func (imageDesc *ImageDesc) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
type ScanResultDesc struct {
	BaseType
}

func (scanResultDesc *ScanResultDesc) asResponse() string {
	return ""
}

/*******************************************************************************
 * 
 */
func GetPOSTFieldValue(values url.Values, name string) string {
	valuear, found := values[name]
	if ! found { return "" }
	if len(valuear) == 0 { return "" }
	return valuear[0]
}

/*******************************************************************************
 * 
 */
func GetRequiredPOSTFieldValue(values url.Values, name string) string {
	var value string = GetPOSTFieldValue(values, name)
	if value == "" { panic(errors.New(fmt.Sprintf("POST field not found: %s", name))) }
	return value
}
