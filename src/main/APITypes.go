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
	"time"
	//"runtime/debug"
)

/*******************************************************************************
 * Mask constants, for convenience.
 */
var CreateMask []bool = []bool{true, false, false, false, false}
var ReadMask []bool = []bool{false, true, false, false, false}
var WriteMask []bool = []bool{false, false, true, false, false}
var ExecuteMask []bool = []bool{false, false, false, true, false}
var DeleteMask []bool = []bool{false, false, false, false, true}

var CanCreate int = 0
var CanRead int = 1
var CanWrite int = 2
var CanExecute int = 3
var CanDelete int = 4

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
	RealmId string
}

func NewSessionToken(sessionId string, userId string) *SessionToken {
	return &SessionToken{
		UniqueSessionId: sessionId,
		AuthenticatedUserid: userId,
		RealmId: "",
	}
}

func (sessionToken *SessionToken) setRealmId(id string) {
	sessionToken.RealmId = id
}

func (sessionToken *SessionToken) asResponse() string {
	return fmt.Sprintf("{\"UniqueSessionId\": \"%s\", \"AuthenticatedUserid\": \"%s\", \"RealmId\": \"%s\"}",
		sessionToken.UniqueSessionId, sessionToken.AuthenticatedUserid, sessionToken.RealmId)
}

/*******************************************************************************
 * 
 */
type GroupDesc struct {
	BaseType
	GroupId string
	RealmId string
	GroupName string
	CreationDate string
	Description string
}

func NewGroupDesc(groupId, realmId, groupName, desc string, creationDate time.Time) *GroupDesc {
	return &GroupDesc{
		GroupId: groupId,
		RealmId: realmId,
		GroupName: groupName,
		CreationDate: FormatTimeAsJavascriptDate(creationDate),
		Description: desc,
	}
}

func (groupDesc *GroupDesc) asResponse() string {
	return fmt.Sprintf("{\"RealmId\": \"%s\", \"GroupName\": \"%s\", \"CreationDate\": %s, \"GroupId\": \"%s\", \"Description\": \"%s\"}",
		groupDesc.RealmId, groupDesc.GroupName, groupDesc.CreationDate, groupDesc.GroupId, groupDesc.Description)
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
	CanModifyTheseRealms []string
}

func NewUserDesc(id, userId, userName, realmId string, canModRealms []string) *UserDesc {
	return &UserDesc{
		Id: id,
		UserId: userId,
		UserName: userName,
		RealmId: realmId,
		CanModifyTheseRealms: canModRealms,
	}
}

func (userDesc *UserDesc) asResponse() string {
	var response string = fmt.Sprintf("{\"Id\": \"%s\", \"UserId\": \"%s\", \"UserName\": \"%s\", \"RealmId\": \"%s\", \"CanModifyTheseRealms\": [",
		userDesc.Id, userDesc.UserId, userDesc.UserName, userDesc.RealmId)
	for i, adminRealmId := range userDesc.CanModifyTheseRealms {
		if i > 0 { response = response + ", " }
		response = response + "\"" + adminRealmId + "\""
	}
	response = response + "]}"
	return response
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
	RealmName string
	OrgFullName string
	AdminUserId string
}

func NewRealmDesc(id string, name string, orgName string, adminUserId string) *RealmDesc {
	return &RealmDesc{
		Id: id,
		RealmName: name,
		OrgFullName: orgName,
		AdminUserId: adminUserId,
	}
}

func (realmDesc *RealmDesc) asResponse() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"RealmName\": \"%s\", \"OrgFullName\": \"%s\", \"AdminUserId\": \"%s\"}",
		realmDesc.Id, realmDesc.RealmName, realmDesc.OrgFullName, realmDesc.AdminUserId)
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
	RealmName string
	OrgFullName string
	Description string
}

func NewRealmInfo(realmName string, orgName string, desc string) (*RealmInfo, error) {
	if realmName == "" { return nil, errors.New("realmName is empty") }
	if orgName == "" { return nil, errors.New("orgName is empty") }
	return &RealmInfo{
		RealmName: realmName,
		OrgFullName: orgName,
		Description: desc,
	}, nil
}

func GetRealmInfo(values url.Values) (*RealmInfo, error) {
	var err error
	var name, orgFullName, desc string
	name, err = GetRequiredPOSTFieldValue(values, "RealmName")
	orgFullName, err = GetRequiredPOSTFieldValue(values, "OrgFullName")
	if err != nil { return nil, err }
	desc = GetPOSTFieldValue(values, "Description")
	return NewRealmInfo(name, orgFullName, desc)
}

func (realmInfo *RealmInfo) asResponse() string {
	return fmt.Sprintf("{\"RealmName\": \"%s\", \"OrgFullName\": \"%s\"}",
		realmInfo.RealmName, realmInfo.OrgFullName)
}

/*******************************************************************************
 * 
 */
type RepoDesc struct {
	BaseType
	Id string
	RealmId string
	RepoName string
	Description string
	CreationDate string
	DockerfileIds []string
}

func NewRepoDesc(id string, realmId string, name string, desc string,
	creationTime time.Time, dockerfileIds []string) *RepoDesc {

	return &RepoDesc{
		Id: id,
		RealmId: realmId,
		RepoName: name,
		Description: desc,
		CreationDate: FormatTimeAsJavascriptDate(creationTime),
		DockerfileIds: dockerfileIds,
	}
}

func (repoDesc *RepoDesc) asResponse() string {
	var resp string = fmt.Sprintf("{\"Id\": \"%s\", \"RealmId\": \"%s\", " +
		"\"RepoName\": \"%s\", \"Description\": \"%s\", \"CreationDate\": \"%s\", " +
		"\"DockerfileIds\": [",
		repoDesc.Id, repoDesc.RealmId, repoDesc.RepoName, repoDesc.Description,
		repoDesc.CreationDate)
	fmt.Println("1: resp=%s", resp)
	fmt.Println(fmt.Sprintf("len(DockerfileIds)=%d", len(repoDesc.DockerfileIds)))
	//fmt.Println("Printing stack:")
	//debug.PrintStack()
	for i, id := range repoDesc.DockerfileIds {
		if i > 0 { resp = resp + ", " }
		resp = resp + id
		fmt.Println("Added " + id + " to resp")
	}
	fmt.Println("2: resp=%s", resp)
	resp = resp + "]}"
	fmt.Println("3: resp=%s", resp)
	return resp
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
	Description string
	DockerfileName string
}

func NewDockerfileDesc(id string, repoId string, name string, desc string) *DockerfileDesc {
	return &DockerfileDesc{
		Id: id,
		RepoId: repoId,
		DockerfileName: name,
		Description: desc,
	}
}

func (dockerfileDesc *DockerfileDesc) asResponse() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"RepoId\": \"%s\", \"DockerfileName\": \"%s\", \"Description\": \"%s\"}",
		dockerfileDesc.Id, dockerfileDesc.RepoId, dockerfileDesc.DockerfileName, dockerfileDesc.Description)
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
type ImageDesc struct {
	BaseType
	ObjId string
	Name string
	Description string
	CreationDate string
}

func NewImageDesc(objId, name, desc string, creationTime time.Time) *ImageDesc {
	return &ImageDesc{
		ObjId: objId,
		Name: name,
		Description: desc,
		CreationDate: FormatTimeAsJavascriptDate(creationTime),
	}
}

/*******************************************************************************
 * 
 */
type DockerImageDesc struct {
	ImageDesc
}

func NewDockerImageDesc(objId, name, desc string, creationTime time.Time) *DockerImageDesc {
	return &DockerImageDesc{
		ImageDesc: *NewImageDesc(objId, name, desc, creationTime),
	}
}

func (imageDesc *DockerImageDesc) getDockerImageTag() string {
	return imageDesc.Name
}

func (imageDesc *DockerImageDesc) asResponse() string {
	return fmt.Sprintf("{\"ObjId\": \"%s\", \"DockerImageTag\": \"%s\"}",
		imageDesc.ObjId, imageDesc.Name)
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

func NewPermissionMask(mask []bool) *PermissionMask {
	return &PermissionMask{
		Mask: mask,
	}
}

func (mask *PermissionMask) GetMask() []bool { return mask.Mask }

func (mask *PermissionMask) CanCreate() bool { return mask.Mask[0] }
func (mask *PermissionMask) CanRead() bool { return mask.Mask[1] }
func (mask *PermissionMask) CanWrite() bool { return mask.Mask[2] }
func (mask *PermissionMask) CanExecute() bool { return mask.Mask[3] }
func (mask *PermissionMask) CanDelete() bool { return mask.Mask[4] }

func (mask *PermissionMask) SetCanCreate(can bool) { mask.Mask[0] = can }
func (mask *PermissionMask) SetCanRead(can bool) { mask.Mask[1] = can }
func (mask *PermissionMask) SetCanWrite(can bool) { mask.Mask[2] = can }
func (mask *PermissionMask) SetCanExecute(can bool) { mask.Mask[3] = can }
func (mask *PermissionMask) SetCanDelete(can bool) { mask.Mask[4] = can }

func (mask *PermissionMask) ToStringArray() []string {
	var strAr []string = make([]string, len(mask.Mask))
	for i, val := range mask.Mask {
		if val { strAr[i] = "true" } else { strAr[i] = "false" }
	}
	return strAr
}

func ToBoolAr(mask []string) ([]bool, error) {
	if len(mask) != 5 { return nil, errors.New("Length of mask != 5") }
	var boolAr []bool = make([]bool, 5)
	for i, val := range mask {
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
	PermissionMask
	ACLEntryId string
	ResourceId string
	PartyId string
}

func NewPermissionDesc(aclEntryId string, resourceId string, partyId string,
	permissionMask []bool) *PermissionDesc {

	return &PermissionDesc{
		ACLEntryId: aclEntryId,
		ResourceId: resourceId,
		PartyId: partyId,
		PermissionMask: PermissionMask{Mask: permissionMask},
	}
}

func (desc *PermissionDesc) asResponse() string {
	return fmt.Sprintf(
		"{\"ACLEntryId\": \"%s\", \"ResourceId\": \"%s\", \"PartyId\": \"%s\", " +
		"\"Create\": %s, \"Read\": %s, \"Write\": %s, \"Execute\": %s, \"Delete\": %s}",
		desc.ACLEntryId, desc.ResourceId, desc.PartyId,
		boolToString(desc.CanCreate()), boolToString(desc.CanRead()),
		boolToString(desc.CanWrite()), boolToString(desc.CanExecute()),
		boolToString(desc.CanDelete()))
}

/*******************************************************************************
 * 
 */
type ScanResultDesc struct {
	BaseType
	Message string
}

func NewScanResultDesc(msg string) *ScanResultDesc {
	return &ScanResultDesc{
		Message: msg,
	}
}

func (scanResultDesc *ScanResultDesc) asResponse() string {
	return fmt.Sprintf("{\"Message\": \"%s\"}", scanResultDesc.Message)
}

/*******************************************************************************
 * 
 */
type EventDesc struct {
	BaseType
	Id string
	When time.Time
	UserId string
}

func NewEventDesc(objId string, when time.Time, userId string) *EventDesc {
	return &EventDesc{
		Id: objId,
		When: when,
		UserId: userId,
	}
}

func (eventDesc *EventDesc) asResponse() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"When\": %s, \"UserId\": \"%s\"}",
		eventDesc.Id, FormatTimeAsJavascriptDate(eventDesc.When), eventDesc.UserId)
}



/****************************** Utility Methods ********************************
 ******************************************************************************/

/*******************************************************************************
 * Format the specified Time value into a string that Javascript will parse as
 * a valid date/time. The string must be in this format:
 *    2015-10-09 14:45:25.641890879 / YYYY-MM-DD HH:MM:SS
 */
func FormatTimeAsJavascriptDate(curTime time.Time) string {
	b, err := curTime.MarshalJSON()
	if err != nil {
		fmt.Println(err.Error())
		return ""
	}
	return string(b)  // Note: this outputs RFC 3339 format date/time.
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
