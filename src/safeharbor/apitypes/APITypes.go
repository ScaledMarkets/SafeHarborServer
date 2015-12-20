/*******************************************************************************
 * The data types needed by the handler functions.
 * This file implements the types defined in slide
 *    "Type Definitions For REST Calls and Responses"
 * of the design,
 *    https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 * These types are serializable via JSON.
 * All types have these:
 *    A New<type> function - Creates a new instance of the type.
 *    A Get<type> function - Constructs an instance from data provided in a map.
 *    A AsResponse method - Returns a string representation of the instance,
 *      suitable for writing to an HTTP response body. The format is defined in
 *      the design in the slide "API REST Binding". We could use go's built-in
 *		JSON formatting for this, but we do it manually to have better control
 *		of what gets sent.
 */

package apitypes

import (
	"net/url"
	"net/http"
	"fmt"
	"errors"
	"time"
	"io"
	"strings"
	//"runtime/debug"
	
	// My packages:
	//"safeharbor/rest"
)

/*******************************************************************************
 * Authorization model: <User> Can<capability> the <Resource>.
 * A capability pertains to a Resource and the Resource''s child Resources.
 * Note: For the purpose of authorization, Users and Groups are treated like Resources
 * of the owning Realm; thus, e.g., a User must have CanWrite for a Realm in order
 * to be able to modify User accounts or Groups for that Realm.
 */
const (
	CanCreateIn uint = iota	// Create new child resources.
	CanRead					// Read or download.
	CanWrite				// Modify.
	CanExecute				// Execute a dockerfile or a scan config.
	CanDelete				// Delete or inactivate.
)

// Mask constants for convenience.
var CreateInMask []bool = []bool{true, false, false, false, false}
var ReadMask []bool = []bool{false, true, false, false, false}
var WriteMask []bool = []bool{false, false, true, false, false}
var ExecuteMask []bool = []bool{false, false, false, true, false}
var DeleteMask []bool = []bool{false, false, false, false, true}

/*******************************************************************************
 * All types defined here include this type as a go "anonymous field".
 */
type BaseType struct {
}

type RespIntfTp interface {  // response interface type
	AsJSON() string
	SendFile() string
}

func (b *BaseType) AsJSON() string {
	return ""
}

func (b *BaseType) SendFile() string {
	return ""
}

var _ RespIntfTp = &BaseType{}

/*******************************************************************************
 * 
 */
type Result struct {
	BaseType
	Status int  // HTTP status code (e.g., 200 is success)
	Message string
}

func NewResult(status int, message string) *Result {
	return &Result{
		Status: status,
		Message: message,
	}
}

func (result *Result) AsJSON() string {
	return fmt.Sprintf("{\"Status\": \"%d\", \"Message\": \"%s\"}",
		result.Status, result.Message)
}

/*******************************************************************************
 * 
 */
type FileResponse struct {
	BaseType
	Status int  // HTTP status code (e.g., 200 is success)
	TempFilePath string  // should be removed after content is retrieved
}

func NewFileResponse(status int, tempFilePath string) *FileResponse {
	return &FileResponse{
		Status: status,
		TempFilePath: tempFilePath,
	}
}

func (response *FileResponse) SendFile() string {
	return response.TempFilePath
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

func (failureDesc *FailureDesc) AsJSON() string {
	return NewFailureMessage(failureDesc.Reason, failureDesc.HTTPCode)
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
	userid, err = GetRequiredHTTPParameterValue(values, "UserId")
	if err != nil { return nil, err }
	
	var pswd string
	pswd, err = GetRequiredHTTPParameterValue(values, "Password")
	if err != nil { return nil, err }
	
	return NewCredentials(userid, pswd), nil
}

func (creds *Credentials) AsJSON() string {
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
	IsAdmin bool
}

func NewSessionToken(sessionId string, userId string) *SessionToken {
	return &SessionToken{
		UniqueSessionId: sessionId,
		AuthenticatedUserid: userId,
		RealmId: "",
		IsAdmin: false,
	}
}

func (sessionToken *SessionToken) SetRealmId(id string) {
	sessionToken.RealmId = id
}

func (sessionToken *SessionToken) SetIsAdminUser(isAdmin bool) {
	sessionToken.IsAdmin = isAdmin
}

func (sessionToken *SessionToken) AsJSON() string {
	return fmt.Sprintf("{\"UniqueSessionId\": \"%s\", \"AuthenticatedUserid\": \"%s\", " +
		"\"RealmId\": \"%s\", \"IsAdmin\": %s}",
		sessionToken.UniqueSessionId, sessionToken.AuthenticatedUserid,
		sessionToken.RealmId, BoolToString(sessionToken.IsAdmin))
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

func (groupDesc *GroupDesc) AsJSON() string {
	return fmt.Sprintf("{\"RealmId\": \"%s\", \"GroupName\": \"%s\", \"CreationDate\": %s, \"GroupId\": \"%s\", \"Description\": \"%s\"}",
		groupDesc.RealmId, groupDesc.GroupName, groupDesc.CreationDate, groupDesc.GroupId, groupDesc.Description)
}

type GroupDescs []*GroupDesc

func (groupDescs GroupDescs) AsJSON() string {
	var response string = "[\n"
	var firstTime bool = true
	for _, desc := range groupDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

func (groupDescs GroupDescs) SendFile() string {
	return ""
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
	userid, err = GetRequiredHTTPParameterValue(values, "UserId")
	if err != nil { return nil, err }
	
	var name string
	name, err = GetRequiredHTTPParameterValue(values, "UserName")
	if err != nil { return nil, err }
	
	var email string
	email, err = GetRequiredHTTPParameterValue(values, "EmailAddress")
	if err != nil { return nil, err }
	
	var pswd string
	pswd, err = GetRequiredHTTPParameterValue(values, "Password")
	if err != nil { return nil, err }
	
	var realmId string
	realmId, err = GetHTTPParameterValue(values, "RealmId") // optional
	if err != nil { return nil, err }
	
	return NewUserInfo(userid, name, email, pswd, realmId), nil
}

func (userInfo *UserInfo) AsJSON() string {
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

func (userDesc *UserDesc) AsJSON() string {
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

func (userDescs UserDescs) AsJSON() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range userDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

func (userDescs UserDescs) SendFile() string {
	return ""
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

func (realmDesc *RealmDesc) AsJSON() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"RealmName\": \"%s\", \"OrgFullName\": \"%s\", \"AdminUserId\": \"%s\"}",
		realmDesc.Id, realmDesc.RealmName, realmDesc.OrgFullName, realmDesc.AdminUserId)
}

type RealmDescs []*RealmDesc

func (realmDescs RealmDescs) AsJSON() string {
	
	var response string = "["
	var firstTime bool = true
	for _, desc := range realmDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

func (realmDescs RealmDescs) SendFile() string {
	return ""
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
	name, err = GetRequiredHTTPParameterValue(values, "RealmName")
	if err != nil { return nil, err }
	orgFullName, err = GetRequiredHTTPParameterValue(values, "OrgFullName")
	if err != nil { return nil, err }
	desc, err = GetHTTPParameterValue(values, "Description")
	if err != nil { return nil, err }
	return NewRealmInfo(name, orgFullName, desc)
}

func (realmInfo *RealmInfo) AsJSON() string {
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

func (repoDesc *RepoDesc) AsJSON() string {
	var resp string = fmt.Sprintf("{\"Id\": \"%s\", \"RealmId\": \"%s\", " +
		"\"RepoName\": \"%s\", \"Description\": \"%s\", \"CreationDate\": %s, " +
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

func (repoDescs RepoDescs) AsJSON() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range repoDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

func (repoDescs RepoDescs) SendFile() string {
	return ""
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

func (dockerfileDesc *DockerfileDesc) AsJSON() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"RepoId\": \"%s\", \"DockerfileName\": \"%s\", \"Description\": \"%s\"}",
		dockerfileDesc.Id, dockerfileDesc.RepoId, dockerfileDesc.DockerfileName, dockerfileDesc.Description)
}

type DockerfileDescs []*DockerfileDesc

func (dockerfileDescs DockerfileDescs) AsJSON() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range dockerfileDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

func (dockerfileDescs DockerfileDescs) SendFile() string {
	return ""
}

/*******************************************************************************
 * 
 */
type ImageDesc struct {
	BaseType
	ObjId string
	RepoId string
	Name string
	Description string
	CreationDate string
}

func NewImageDesc(objId, repoId, name, desc string, creationTime time.Time) *ImageDesc {
	return &ImageDesc{
		ObjId: objId,
		RepoId: repoId,
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

func NewDockerImageDesc(objId, repoId, name, desc string, creationTime time.Time) *DockerImageDesc {
	return &DockerImageDesc{
		ImageDesc: *NewImageDesc(objId, repoId, name, desc, creationTime),
	}
}

func (imageDesc *DockerImageDesc) getDockerImageTag() string {
	return imageDesc.Name
}

func (imageDesc *DockerImageDesc) AsJSON() string {
	return fmt.Sprintf("{\"ObjId\": \"%s\", \"RepoId\": \"%s\", \"Name\": \"%s\", " +
		"\"Description\": \"%s\", \"CreationDate\": %s}",
		imageDesc.ObjId, imageDesc.RepoId, imageDesc.Name, imageDesc.Description, imageDesc.CreationDate)
}

type DockerImageDescs []*DockerImageDesc

func (imageDescs DockerImageDescs) AsJSON() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range imageDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

func (imageDescs DockerImageDescs) SendFile() string {
	return ""
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

func (mask *PermissionMask) CanCreateIn() bool { return mask.Mask[0] }
func (mask *PermissionMask) CanRead() bool { return mask.Mask[1] }
func (mask *PermissionMask) CanWrite() bool { return mask.Mask[2] }
func (mask *PermissionMask) CanExecute() bool { return mask.Mask[3] }
func (mask *PermissionMask) CanDelete() bool { return mask.Mask[4] }

func (mask *PermissionMask) SetCanCreateIn(can bool) { mask.Mask[0] = can }
func (mask *PermissionMask) SetCanRead(can bool) { mask.Mask[1] = can }
func (mask *PermissionMask) SetCanWrite(can bool) { mask.Mask[2] = can }
func (mask *PermissionMask) SetCanExecute(can bool) { mask.Mask[3] = can }
func (mask *PermissionMask) SetCanDelete(can bool) { mask.Mask[4] = can }

func (mask *PermissionMask) AsJSON() string {
	return fmt.Sprintf(
		"{\"CanCreateIn\": %d, \"CanRead\": %d, \"CanWrite\": %d, \"CanExecute\": %d, \"CanDelete\": %d}",
		mask.CanCreateIn, mask.CanRead, mask.CanWrite, mask.CanExecute, mask.CanDelete)
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

func (desc *PermissionDesc) AsJSON() string {
	return fmt.Sprintf(
		"{\"ACLEntryId\": \"%s\", \"ResourceId\": \"%s\", \"PartyId\": \"%s\", " +
		"\"CanCreateIn\": %s, \"CanRead\": %s, \"CanWrite\": %s, \"CanExecute\": %s, \"CanDelete\": %s}",
		desc.ACLEntryId, desc.ResourceId, desc.PartyId,
		BoolToString(desc.CanCreateIn()), BoolToString(desc.CanRead()),
		BoolToString(desc.CanWrite()), BoolToString(desc.CanExecute()),
		BoolToString(desc.CanDelete()))
}

/*******************************************************************************
 * 
 */
type ParameterInfo struct {
	Name string
	Description string
}

func NewParameterInfo(name string, desc string) *ParameterInfo {
	return &ParameterInfo{
		Name: name,
		Description: desc,
	}
}

func (parameterInfo *ParameterInfo) AsJSON() string {
	return fmt.Sprintf("{\"Name\": \"%s\", \"Description\": \"%s\"}",
		parameterInfo.Name, parameterInfo.Description)
}

/*******************************************************************************
 * 
 */
type ScanProviderDesc struct {
	BaseType
	ProviderName string
	Parameters []ParameterInfo
}

func NewScanProviderDesc(name string, params []ParameterInfo) *ScanProviderDesc {
	return &ScanProviderDesc{
		ProviderName: name,
		Parameters: params,
	}
}

func (scanProviderDesc *ScanProviderDesc) AsJSON() string {
	var response string = fmt.Sprintf("{\"ProviderName\": \"%s\", \"Parameters\": [",
		scanProviderDesc.ProviderName)
	var firstTime bool = true
	for _, paramInfo := range scanProviderDesc.Parameters {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + paramInfo.AsJSON()
	}
	response = response + "]}"
	return response
}

type ScanProviderDescs []*ScanProviderDesc

func (scanProviderDescs ScanProviderDescs) AsJSON() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range scanProviderDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

func (providerDescs ScanProviderDescs) SendFile() string {
	return ""
}

/*******************************************************************************
 * 
 */
type ParameterValueDesc struct {
	Name string
	//Type string
	StringValue string
}

func NewParameterValueDesc(name string, strValue string) *ParameterValueDesc {
	return &ParameterValueDesc{
		Name: name,
		//Type: tp,
		StringValue: strValue,
	}
}

func (desc *ParameterValueDesc) AsJSON() string {
	return fmt.Sprintf("{\"Name\": \"%s\", \"Value\": \"%s\"}",
		desc.Name, desc.StringValue)
}

/*******************************************************************************
 * 
 */
type ScanConfigDesc struct {
	BaseType
	Id string
	ProviderName string
	SuccessExpression string
	FlagId string
	ParameterValueDescs []*ParameterValueDesc
}

func NewScanConfigDesc(id, provName, expr, flagId string, paramValueDescs []*ParameterValueDesc) *ScanConfigDesc {
	return &ScanConfigDesc{
		Id: id,
		ProviderName: provName,
		SuccessExpression: expr,
		ParameterValueDescs: paramValueDescs,
	}
}

func (scanConfig *ScanConfigDesc) AsJSON() string {
	var s string = fmt.Sprintf("{\"Id\": \"%s\", \"ProviderName\": \"%s\", " +
		"\"SuccessExpression\": \"%s\", \"FlagId\": \"%s\", " +
		"\"ParameterValueDescs\": [", scanConfig.Id, scanConfig.ProviderName,
		scanConfig.SuccessExpression, scanConfig.FlagId)
	for i, paramValueDesc := range scanConfig.ParameterValueDescs {
		if i > 0 { s = s + ",\n" }
		s = s + paramValueDesc.AsJSON()
	}
	return s + "\n]}"
}

/*******************************************************************************
 * 
 */
type FlagDesc struct {
	BaseType
	RepoId string
	ImageURL string
}

func NewFlagDesc(repoId, imageURL string) *FlagDesc {
	return &FlagDesc{
		RepoId: repoId,
		ImageURL: imageURL,
	}
}

func (flagDesc *FlagDesc) AsJSON() string {
	return fmt.Sprintf("{\"RepoId\": \"%s\", \"ImageURL\": \"%s\"}",
		flagDesc.RepoId, flagDesc.ImageURL)
}

/*******************************************************************************
 * 
 */
type EventDesc struct {
	BaseType
	EventId string
	When time.Time
	UserObjId string
}

func NewEventDesc(objId string, when time.Time, userObjId string) *EventDesc {
	return &EventDesc{
		EventId: objId,
		When: when,
		UserObjId: userId,
	}
}

func (eventDesc *EventDesc) AsJSON() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"When\": %s, \"UserId\": \"%s\"}",
		eventDesc.EventId, FormatTimeAsJavascriptDate(eventDesc.When), eventDesc.UserId)
}

type EventDescs []*EventDesc

func (eventDescs EventDescs) AsJSON() string {
	var response string = "["
	var firstTime bool = true
	for _, desc := range eventDescs {
		if firstTime { firstTime = false } else { response = response + ",\n" }
		response = response + desc.AsJSON()
	}
	response = response + "]"
	return response
}

/*******************************************************************************
 * 
 */
type ScanEventDesc struct {
	EventDesc
	ScanConfigId string
	ProviderName string
    ParameterValueDescs []ParameterValueDesc
	Score string
}

func NewScanEventDesc(objId string, when time.Time, userId string,
	scanConfigId, providerName string, paramValueDescs []string, score string) *ScanEventDesc {
	return &ScanEventDesc{
		EventDesc: *NewEventDesc(objId, when, userId),
		ScanConfigId: scanConfigId,
		ProviderName: providerName,
		ParameterValueDescs: paramValuesDescs
		Score: score,
	}
}

func (eventDesc *ScanEventDesc) AsJSON() string {
	var s = fmt.Sprintf("{\"Id\": \"%s\", \"When\": %s, \"UserId\": \"%s\", " +
		"\"ScanConfigId\": \"%s\", \"ProviderName\": \"%s\", \"Score\": \"%s\",",
		eventDesc.EventId, FormatTimeAsJavascriptDate(eventDesc.When), eventDesc.UserId,
		eventDesc.ScanConfigId, eventDesc.ProviderName, eventDesc.Score)
	
	for i, valueDesc := range eventDesc.ParameterValueDescs {
		if i > 0 { s = s + ", " }
		s = s + valueDesc.AsJSON()
	}
	s = s + "}"
}

/*******************************************************************************
 * 
 */
type DockerfileExecEventDesc struct {
	EventDesc
	DockerfileId string
}

func NewDockerfileExecEventDesc(objId string, when time.Time, userId string,
	dockerfileId string) *DockerfileExecEventDesc {
	return &DockerfileExecEventDesc{
		EventDesc: *NewEventDesc(objId, when, userId),
		DockerfileId: dockerfileId,
	}
}

func (eventDesc *DockerfileExecEventDesc) AsJSON() string {
	return fmt.Sprintf("{\"Id\": \"%s\", \"When\": %s, \"UserId\": \"%s\", " +
		"\"DockefileId\": \"%s\"}",
		eventDesc.EventId, FormatTimeAsJavascriptDate(eventDesc.When), eventDesc.UserId,
		eventDesc.DockerfileId)
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
func GetHTTPParameterValue(values url.Values, name string) (string, error) {
	valuear, found := values[name]
	if ! found { return "", nil }
	if len(valuear) == 0 { return "", nil }
	return sanitize(valuear[0])
}

/*******************************************************************************
 * 
 */
func GetRequiredHTTPParameterValue(values url.Values, name string) (string, error) {
	var value string
	var err error
	value, err = GetHTTPParameterValue(values, name)
	if err != nil { return "", err }
	if value == "" { return "", errors.New(fmt.Sprintf("POST field not found: %s", name)) }
	return value, nil
}

/*******************************************************************************
 * 
 */
func (mask *PermissionMask) ToStringArray() []string {
	var strAr []string = make([]string, len(mask.Mask))
	for i, val := range mask.Mask {
		if val { strAr[i] = "true" } else { strAr[i] = "false" }
	}
	return strAr
}

/*******************************************************************************
 * 
 */
func ToBoolAr(mask []string) ([]bool, error) {
	if len(mask) != 5 { return nil, errors.New("Length of mask != 5") }
	var boolAr []bool = make([]bool, 5)
	for i, val := range mask {
		if val == "true" { boolAr[i] = true } else { boolAr[i] = false }
	}
	return boolAr, nil
}

/*******************************************************************************
 * 
 */
func RespondMethodNotSupported(writer http.ResponseWriter, methodName string) {
	writer.WriteHeader(405)
	io.WriteString(writer, "HTTP method not supported:" + methodName)
}

/*******************************************************************************
 * 
 */
func RespondWithClientError(writer http.ResponseWriter, err string) {
	writer.WriteHeader(400)
	io.WriteString(writer, err)
}

/*******************************************************************************
 * 
 */
func RespondWithServerError(writer http.ResponseWriter, err string) {
	writer.WriteHeader(500)
	io.WriteString(writer, err)
}

/*******************************************************************************
 * 
 */
func NewFailureMessage(reason string, httpCode int) string {
	return fmt.Sprintf("{\"Reason\": \"%s\", \"HTTPCode\": \"%d\"}", reason, httpCode)
}

/*******************************************************************************
 * 
 */
func BoolToString(b bool) string {
	if b { return "true" } else { return "false" }	
}

/*******************************************************************************
 * Check if the input contains any character sequences that could result in
 * a scripting attack if rendered in a response to a client. Simply limit characters
 * to letters, numbers, period, hyphen, and underscore.
 */
func sanitize(value string) (string, error) {
	//return value, nil
	
	var allowed string = " ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-@:/"
	if len(strings.TrimLeft(value, allowed)) == 0 { return value, nil }
	return "", errors.New("Value '" + value + "' may only have letters, numbers, and .-_@:/")
	
}

/*******************************************************************************
 * Utility to remove a value from an array of strings.
 */
func RemoveFrom(value string, originalList []string) []string {
	var newList []string
	copy(newList, originalList)
	for index, s := range originalList {
		if s == value { newList = RemoveAt(index, newList) }
	}
	return newList
}

/*******************************************************************************
 * Utility to remove a value from a specified location in an array of strings.
 */
func RemoveAt(position int, originalList []string) []string {
	return append(originalList[0:position], originalList[position+1:]...)
}
