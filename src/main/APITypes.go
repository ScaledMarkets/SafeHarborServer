/*******************************************************************************
 * The data types needed by the handler functions.
 * This file implements the types defined in slide
 *    "Type Definitions For REST Calls and Responses"
 * of the design,
 *    https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 * All types have these:
 *    A New<type> function - Creates a new instance of the type.
 *    A Get<type> function - Constructs an instance from data provided in a map.
 *    A asResponse method - Returns a string representation of the instance,
 *      suitable for writing to an HTTP response body. The format is defined in
 *      the design in the slide "API REST Binding".
 */

package main

import (
	"net/url"
	"fmt"
	"errors"
)

/*******************************************************************************
 * All types defined here include this type as a go "anonymous field".
 */
type BaseType struct {
}

type RespIntfTp interface {  // response interface type
	asResponse() string
}

func (b *BaseType) asResponse() string {
	return ""
}

var _ RespIntfTp = &BaseType{}

/*******************************************************************************
 * 
 */
type Result struct {
	BaseType
	Status int
	Message string
}

func (result *Result) asResponse() string {
	return fmt.Sprintf("Status=%d\r\nMessage=%s", result.Status, result.Message)
}

/*******************************************************************************
 * All handlers return a FailureDesc if they detect an error.
 */
type FailureDesc struct {
	BaseType
	Reason string
	HTTPCode int
}

func NewFailureDesc(reason string) *FailureDesc {
	return &FailureDesc{
		Reason: reason,
		HTTPCode: 500,
	}
}

func (failureDesc *FailureDesc) asResponse() string {
	return fmt.Sprintf("Reason=%s\r\nHTTPCode=%d", failureDesc.Reason, failureDesc.HTTPCode)
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

func GetCredentials(values url.Values) (*Credentials, error) {
	var err error
	var userid string
	userid, err = GetRequiredPOSTFieldValue(values, "userid")
	if err != nil { return nil, err }
	
	var pswd string
	pswd, err = GetRequiredPOSTFieldValue(values, "pswd")
	if err != nil { return nil, err }
	
	return NewCredentials(userid, pswd), nil
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
	RealmId string
	Name string
	GroupId string
}

func (groupDesc *GroupDesc) asResponse() string {
	return fmt.Sprintf("RealmId=%s\r\nName=%s\r\nGroupId=%s\r\n",
		groupDesc.RealmId, groupDesc.Name, groupDesc.GroupId)
}

type GroupDescs []*GroupDesc

func (groupDescs GroupDescs) asResponse() string {
	var response string = ""
	var firstTime bool = true
	for _, desc := range groupDescs {
		if firstTime { firstTime = false } else { response = "\r\n" + response }
		response = response + desc.asResponse()
	}
	return response
}

/*******************************************************************************
 * 
 */
type UserInfo struct {
	BaseType
	UserId string
	UserName string
	RealmId string
}

func NewUserInfo(userid string, name string, realmId string) *UserInfo {
	return &UserInfo{
		UserId: userid,
		UserName: name,
		RealmId: realmId,
	}
}

func GetUserInfo(values url.Values) (*UserInfo, error) {
	var err error
	var userid string
	userid, err = GetRequiredPOSTFieldValue(values, "UserId")
	if err != nil { return nil, err }
	
	var name string
	name, err = GetRequiredPOSTFieldValue(values, "UserName")
	if err != nil { return nil, err }
	
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return nil, err }
	
	return NewUserInfo(userid, name, realmId), nil
}

func (userInfo *UserInfo) asResponse() string {
	return fmt.Sprintf("UserId=%s\r\nUserName=%s\r\nRealmId=%s\r\n",
		userInfo.UserId, userInfo.UserName, userInfo.RealmId)
}

/*******************************************************************************
 * 
 */
type UserDesc struct {
	BaseType
	Id string
	UserId string
	UserName string
	RealmId string
}

func (userDesc *UserDesc) asResponse() string {
	return fmt.Sprintf("Id=%s\r\nUserId=%s\r\nUserName=%s\r\nGroupId=%s\r\n",
		userDesc.Id, userDesc.UserId, userDesc.UserName, userDesc.RealmId)
}

type UserDescs []*UserDesc

func (userDescs UserDescs) asResponse() string {
	var response string = ""
	var firstTime bool = true
	for _, desc := range userDescs {
		if firstTime { firstTime = false } else { response = "\r\n" + response }
		response = response + desc.asResponse()
	}
	return response
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
	return fmt.Sprintf("Id=%s\r\nName=%s\r\n", realmDesc.Id, realmDesc.Name)
}

type RealmDescs []*RealmDesc

func (realmDescs RealmDescs) asResponse() string {
	var response string = ""
	var firstTime bool = true
	for _, desc := range realmDescs {
		if firstTime { firstTime = false } else { response = "\r\n" + response }
		response = response + desc.asResponse()
	}
	return response
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

func GetRealmInfo(values url.Values) (*RealmInfo, error) {
	var err error
	var name string
	name, err = GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return nil, err }
	return NewRealmInfo(name), nil
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
	return fmt.Sprintf("Id=%s\r\nRealmId=%s\r\nName=%s\r\n",
		repoDesc.Id, repoDesc.RealmId, repoDesc.Name)
}

type RepoDescs []*RepoDesc

func (repoDescs RepoDescs) asResponse() string {
	var response string = ""
	var firstTime bool = true
	for _, desc := range repoDescs {
		if firstTime { firstTime = false } else { response = "\r\n" + response }
		response = response + desc.asResponse()
	}
	return response
}

/*******************************************************************************
 * 
 */
type DockerfileDesc struct {
	BaseType
	Id string
	RepoId string
	Name string
}

func NewDockerfileDesc(id string, repoId string, name string) *DockerfileDesc {
	return &DockerfileDesc{
		Id: id,
		RepoId: repoId,
		Name: name,
	}
}

func (dockerfileDesc *DockerfileDesc) asResponse() string {
	return fmt.Sprintf("Id=%s\r\nRepoId=%s\r\nName=%s\r\n",
		dockerfileDesc.Id, dockerfileDesc.RepoId, dockerfileDesc.Name)
}

type DockerfileDescs []*DockerfileDesc

func (dockerfileDescs DockerfileDescs) asResponse() string {
	var response string = ""
	var firstTime bool = true
	for _, desc := range dockerfileDescs {
		if firstTime { firstTime = false } else { response = "\r\n" + response }
		response = response + desc.asResponse()
	}
	return response
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

type ImageDescs []*ImageDesc

func (imageDescs ImageDescs) asResponse() string {
	var response string = ""
	var firstTime bool = true
	for _, desc := range imageDescs {
		if firstTime { firstTime = false } else { response = "\r\n" + response }
		response = response + desc.asResponse()
	}
	return response
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
func GetRequiredPOSTFieldValue(values url.Values, name string) (string, error) {
	var value string = GetPOSTFieldValue(values, name)
	if value == "" { return "", errors.New(fmt.Sprintf("POST field not found: %s", name)) }
	return value, nil
}
