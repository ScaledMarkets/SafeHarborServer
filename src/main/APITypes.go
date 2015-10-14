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

func NewResult(status int, message string) *Result {
	return &Result{
		Status: status,
		Message: message,
	}
}

func (result *Result) asResponse() string {
	return fmt.Sprintf("{\"Status\": \"%d\",\"Message\": \"%s\"}",
		result.Status, result.Message)
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
	fmt.Println("Creating FailureDesc; reason=" + reason)
	return &FailureDesc{
		Reason: reason,
		HTTPCode: 500,
	}
}

func (failureDesc *FailureDesc) asResponse() string {
	return fmt.Sprintf("{\"Reason\": \"%s\", \"HTTPCode\": \"%d\"}",
		failureDesc.Reason, failureDesc.HTTPCode)
}

/*******************************************************************************
 * Types and functions for credentials.
 */
type Credentials struct {
	BaseType
	UserId string
	Password string
}

func NewCredentials(uid string, pwd string) *Credentials {
	return &Credentials{
		UserId: uid,
		Password: pwd,
	}
}

func GetCredentials(values url.Values) (*Credentials, error) {
	var err error
	var userid string
	userid, err = GetRequiredPOSTFieldValue(values, "UserId")
	if err != nil { return nil, err }
	
	var pswd string
	pswd, err = GetRequiredPOSTFieldValue(values, "Password")
	if err != nil { return nil, err }
	
	return NewCredentials(userid, pswd), nil
}

func (creds *Credentials) asResponse() string {
	return fmt.Sprintf("{\"UserId\": \"%s\"}", creds.UserId)
}

/*******************************************************************************
 * 
 */
type SessionToken struct {
	BaseType
	UniqueSessionId string
	AuthenticatedUserid string
}

func NewSessionToken(sessionId string, userId string) *SessionToken {
	return &SessionToken{
		UniqueSessionId: sessionId,
		AuthenticatedUserid: userId,
	}
}

func (sessionToken *SessionToken) asResponse() string {
	return fmt.Sprintf("{\"UniqueSessionId\": \"%s\", \"AuthenticatedUserid\": \"%s\"}",
		sessionToken.UniqueSessionId, sessionToken.AuthenticatedUserid)
}

/*******************************************************************************
 * 
 */
type GroupDesc struct {
	BaseType
	GroupId string
	RealmId string
	Name string
	Purpose string
}

func (groupDesc *GroupDesc) asResponse() string {
	return fmt.Sprintf("{\"RealmId\": \"%s\", \"Name\": \"%s\", \"GroupId\": \"%s\", \"Purpose\": \"%s\"}",
		groupDesc.RealmId, groupDesc.Name, groupDesc.GroupId, groupDesc.Purpose)
}

type GroupDescs []*GroupDesc

func (groupDescs GroupDescs) asResponse() string {
	var response string = "[\n"
	var firstTime bool = true
	for _, desc := range groupDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.asResponse()
	}
	response = response + "]"
	return response
}

/*******************************************************************************
 * 
 */
type UserInfo struct {
	BaseType
	UserId string
	UserName string
	EmailAddress string
	Password string
	RealmId string  // may be ""
}

func NewUserInfo(userid, name, email, pswd, realmId string) *UserInfo {
	return &UserInfo{
		UserId: userid,
		UserName: name,
		EmailAddress: email,
		Password: pswd,
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
	
	var email string
	email, err = GetRequiredPOSTFieldValue(values, "EmailAddress")
	if err != nil { return nil, err }
	
	var pswd string
	pswd, err = GetRequiredPOSTFieldValue(values, "Password")
	if err != nil { return nil, err }
	
	var realmId string
	realmId = GetPOSTFieldValue(values, "RealmId") // optional
	if err != nil { return nil, err }
	
	return NewUserInfo(userid, name, email, pswd, realmId), nil
}

func (userInfo *UserInfo) asResponse() string {
	return fmt.Sprintf("{\"UserId\": \"%s\", \"UserName\": \"%s\", \"EmailAddress\": \"%s\", \"RealmId\": \"%s\"}",
		userInfo.UserId, userInfo.UserName, userInfo.EmailAddress, userInfo.RealmId)
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
	return fmt.Sprintf("{\"Id\": \"%s\", \"UserId\": \"%s\", \"UserName\": \"%s\", \"RealmId\": \"%s\"}",
		userDesc.Id, userDesc.UserId, userDesc.UserName, userDesc.RealmId)
}

type UserDescs []*UserDesc

func (userDescs UserDescs) asResponse() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range userDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.asResponse()
	}
	response = response + "]"
	return response
}

/*******************************************************************************
 * 
 */
type RealmDesc struct {
	BaseType
	Id string
	Name string
	OrgFullName string
	AdminUserId string
}

func NewRealmDesc(id string, name string, orgName string, adminUserId string) *RealmDesc {
	return &RealmDesc{
		Id: id,
		Name: name,
		OrgFullName: orgName,
		AdminUserId: adminUserId,
	}
}

func (realmDesc *RealmDesc) asResponse() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"Name\": \"%s\", \"OrgFullName\": \"%s\", \"AdminUserId\": \"%s\"}",
		realmDesc.Id, realmDesc.Name, realmDesc.OrgFullName, realmDesc.AdminUserId)
}

type RealmDescs []*RealmDesc

func (realmDescs RealmDescs) asResponse() string {
	
	var response string = "["
	var firstTime bool = true
	for _, desc := range realmDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.asResponse()
	}
	response = response + "]"
	return response
}

/*******************************************************************************
 * 
 */
type RealmInfo struct {
	BaseType
	Name string
	OrgFullName string
	AdminUserId string
}

func NewRealmInfo(realmName string, orgName string, adminUserId string) *RealmInfo {
	return &RealmInfo{
		Name: realmName,
		OrgFullName: orgName,
		AdminUserId: adminUserId,
	}
}

func GetRealmInfo(values url.Values) (*RealmInfo, error) {
	var err error
	var name, orgFullName, adminUserId string
	name, err = GetRequiredPOSTFieldValue(values, "Name")
	orgFullName, err = GetRequiredPOSTFieldValue(values, "OrgFullName")
	adminUserId, err = GetRequiredPOSTFieldValue(values, "AdminUserId")
	if err != nil { return nil, err }
	return NewRealmInfo(name, orgFullName, adminUserId), nil
}

func (realmInfo *RealmInfo) asResponse() string {
	return fmt.Sprintf("{\"Name\": \"%s\", \"OrgFullName\": \"%s\", \"AdminUserId\": \"%s\"}",
		realmInfo.Name, realmInfo.OrgFullName, realmInfo.AdminUserId)
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
	return fmt.Sprintf("{\"Id\": \"%s\", \"RealmId\": \"%s\", \"Name\": \"%s\"}",
		repoDesc.Id, repoDesc.RealmId, repoDesc.Name)
}

type RepoDescs []*RepoDesc

func (repoDescs RepoDescs) asResponse() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range repoDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.asResponse()
	}
	response = response + "]"
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
	return fmt.Sprintf("{\"Id\": \"%s\", \"RepoId\": \"%s\", \"Name\": \"%s\"}",
		dockerfileDesc.Id, dockerfileDesc.RepoId, dockerfileDesc.Name)
}

type DockerfileDescs []*DockerfileDesc

func (dockerfileDescs DockerfileDescs) asResponse() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range dockerfileDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.asResponse()
	}
	response = response + "]"
	return response
}

/*******************************************************************************
 * 
 */
type DockerImageDesc struct {
	BaseType
	ObjId string
	DockerImageTag string
}

func NewDockerImageDesc(objId string, dockerId string) *DockerImageDesc {
	return &DockerImageDesc{
		ObjId: objId,
		DockerImageTag: dockerId,
	}
}

func (imageDesc *DockerImageDesc) getDockerImageId() string {
	return imageDesc.DockerImageTag
}

func (imageDesc *DockerImageDesc) asResponse() string {
	return fmt.Sprintf("{\"ObjId\": \"%s\", \"DockerImageTag\": \"%s\"}",
		imageDesc.ObjId, imageDesc.DockerImageTag)
}

type DockerImageDescs []*DockerImageDesc

func (imageDescs DockerImageDescs) asResponse() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range imageDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.asResponse()
	}
	response = response + "]"
	return response
}

/*******************************************************************************
 * 
 */
type PermissionMask struct {
	BaseType
	Mask []bool
}

func NewPermissionMask(canCreate, canRead, canWrite, canExec, canDel bool) PermissionMask {
	return &PermissionMask{
		Mask: { canCreate, canRead, canWrite, canExec, canDel },
	}
}

func (mask *PermissionMask) GetMask() []bool { return Mask }

func (mask *PermissionMask) CanCreate() bool { return Mask[0] }
func (mask *PermissionMask) CanRead() bool { return Mask[1] }
func (mask *PermissionMask) CanWrite() bool { return Mask[2] }
func (mask *PermissionMask) CanExecute() bool { return Mask[3] }
func (mask *PermissionMask) CanDelete() bool { return Mask[4] }

func (mask *PermissionMask) SetCanCreate(can bool) { Mask[0] }
func (mask *PermissionMask) SetCanRead(can bool) { Mask[1] }
func (mask *PermissionMask) SetCanWrite(can bool) { Mask[2] }
func (mask *PermissionMask) SetCanExecute(can bool) { Mask[3] }
func (mask *PermissionMask) SetCanDelete(can bool) { Mask[4] }

func (mask *PermissionMask) ToStringArray() []string {
	var strAr []string = make([]string, len(mask.Mask))
	for i, val := range mask.Mask {
		if val { strAr[i] = "true" } else {strAr[i] = false }
	}
	return strAr
}

func ToBoolAr(mask []string) ([]bool, error) {
	if len(mask) != 5 { return nil, errors.New("Length of mask != 5") }
	var boolAr []bool = make([]bool, 5)
	for i, val := range boolAr {
		if val == "true" { boolAr[i] = true } else { boolAr[i] = false }
	}
	return boolAr, nil
}

func (mask *PermissionMask) asResponse() string {
	return fmt.Sprintf(
		"{\"CanCreate\": %d, \"CanRead\": %d, \"CanWrite\": %d, \"CanExecute\": %d, \"CanDelete\": %d}",
		mask.CanCreate, mask.CanRead, mask.CanWrite, mask.CanExecute, mask.CanDelete)
}

/*******************************************************************************
 * 
 */
type PermissionDesc struct {
	BaseType
	ACLEntryId string
	ResourceId string
	PartyId string
	PermissionMask PermissionMask
}

func NewPermissionDesc(aclEntryId string, resourceId string, partyId string, permissionMask PermissionMask) {
	return &PermissionDesc{
		ACLEntryId: aclEntryId,
		ResourceId: resourceId,
		PartyId: partyId,
		PermissionMask: permissionMask,
}

func (desc *PermissionDesc) asResponse() string {
	return fmt.Sprintf(
		"{\"ACLEntryId\": \"%s\", \"ResourceId\": \"%s\", \"PartyId\": \"%s\", \"CanCreate\": %d, \"CanRead\": %d, \"CanWrite\": %d, \"CanExecute\": %d, \"CanDelete\": %d}",
		desc.ACLEntryId, desc.ResourceId, desc.PartyId, desc.CanCreate(), desc.CanRead(), desc.CanWrite(), desc.CanExecute(), desc.CanDelete())
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
