/*******************************************************************************
 * All of the REST handlers are contained here. These functions are called by
 * the handleRequest method in Dispatcher.go. Methods and types are defined here:
 *	https://drive.google.com/open?id=1KLRJ8u0rk3djxr5_sw0vfNnq9cVY3OLzDoQV6YWoVto
 * Error codes used are (see https://golang.org/pkg/net/http/#pkg-constants),
	StatusBadRequest (400) - Bad request.
	StatusUnauthorized (401) - User is not authenticated, and must be to perform the requested action.
	StatusForbidden (403) - User is authenticated, but is not authorized to perform the action.
	StatusConflict (409) - Contention among multiple users for update to the same data.
	StatusInternalServerError (500) - An unexpected internal server error.
 *
 * Copyright Scaled Markets, Inc.
 */

package server

import (
	"net/url"
	"net/http"
	"mime/multipart"
	"fmt"
	"os/exec"
	"strings"
	"reflect"
	"time"
	//"runtime/debug"
	
	// Our packages:
	"scanners"
	"safeharbor/apitypes"
	"docker"
)

/*******************************************************************************
 * Arguments: (none)
 * Returns: apitypes.Result
 */
func ping(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	fmt.Println("ping request received")
	return apitypes.NewResult(200, "Server is up")
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: apitypes.Result
 */
func clearAll(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	fmt.Println("clearAll")
	
	if ! dbClient.Server.Debug {
		return apitypes.NewFailureDesc(http.StatusForbidden,
			"Not in debug mode - returning from clearAll")
	}
	
	// Kill all docker containers:
	// docker kill $(docker ps -a -q)
	// docker rm $(docker ps -a -q)
	var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "ps", "-a", "-q")
	var output []byte
	output, _ = cmd.CombinedOutput()
	var containers string = string(output)
	if strings.HasPrefix(containers, "Error") {
		return apitypes.NewFailureDesc(http.StatusInternalServerError, containers)
	}
	fmt.Println("Containers are:")
	fmt.Println(containers)
	
	cmd = exec.Command("/usr/bin/docker", "kill", containers)
	output, _ = cmd.CombinedOutput()
	var outputStr string = string(output)
	//if strings.HasPrefix(outputStr, "Error") {
	//	return apitypes.NewFailureDesc(outputStr)
	//}
	fmt.Println("All containers were signalled to stop")
	
	cmd = exec.Command("/usr/bin/docker", "rm", containers)
	output, _ = cmd.CombinedOutput()
	outputStr = string(output)
	//if strings.HasPrefix(outputStr, "Error") {
	//	return apitypes.NewFailureDesc(outputStr)
	//}
	fmt.Println("All containers were removed")
	
	// Remove all of the docker images that were created by SafeHarborServer.
	fmt.Println("Removing docker images that were created by SafeHarbor:")
	var realmIds []string
	var err error
	realmIds, err = dbClient.dbGetAllRealmIds()
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	for _, realmId := range realmIds {
		var realm Realm
		var err error
		realm, err = dbClient.getRealm(realmId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		fmt.Println("For realm " + realm.getName() + ":")
		
		for _, repoId := range realm.getRepoIds() {
			var repo Repo
			repo, err = dbClient.getRepo(repoId)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
			fmt.Println("\tFor repo " + repo.getName() + ":")
			
			for _, imageId := range repo.getDockerImageIds() {
				
				var image DockerImage
				image, err = dbClient.getDockerImage(imageId)
				if err != nil { return apitypes.NewFailureDescFromError(err) }
				
				for _, versionId := range image.getImageVersionIds() {
					
					var imageVersion ImageVersion
					imageVersion, err = dbClient.getImageVersion(versionId)
					if err != nil { return apitypes.NewFailureDescFromError(err) }
					
					var imageFullName, tag string
					imageFullName, tag = docker.ConstructDockerImageName(
						realm.getName(), repo.getName(), image.getName(), imageVersion.getVersion())
					var imageFullTaggedName = imageFullName + ":" + tag
					fmt.Println("\t\tRemoving image " + imageFullTaggedName + ":")
					
					// Remove the image.
					cmd = exec.Command("/usr/bin/docker", "rmi", imageFullTaggedName)
					output, _ = cmd.CombinedOutput()
					outputStr = string(output)
					if ! strings.HasPrefix(outputStr, "Error") {
						fmt.Println("While removing image " + imageFullTaggedName + ": " + outputStr)
					} else {
						fmt.Println("\t\tRemoved image", imageFullTaggedName)
					}
				}
			}
		}
	}
	
	// Clear all session state.
	dbClient.Server.authService.clearAllSessions()
	
	// Remove and re-create the repository directory.
	err = dbClient.Persistence.resetPersistentState()
	if err != nil { return apitypes.NewResult(500, err.Error()) }
	fmt.Println("Initializing database...")
	dbClient.Persistence.init()
	
	return apitypes.NewResult(200, "Persistent state reset")
}

/*******************************************************************************
 * Arguments: apitypes.Credentials
 * Returns: apitypes.SessionToken
 */
func printDatabase(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	fmt.Println("printDatabase")
	
	if ! dbClient.Server.Debug {
		return apitypes.NewFailureDesc(http.StatusForbidden,
			"Not in debug mode - returning from printDatabase")
	}
	
	dbClient.Persistence.printDatabase()
	
	return apitypes.NewResult(200, "Database printed to stdout on server.")
}

/*******************************************************************************
 * Arguments: <any>
 * Returns: apitypes.Result
 * A File argument is obtained from the form values as follows:
 *    The name specified by the client is keyed on "filename".
 * The handler should move the file to a permanent name.
 */
func acknowledge(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	fmt.Println("acknowledge")
	
	if ! dbClient.Server.Debug {
		return apitypes.NewFailureDesc(http.StatusForbidden,
			"Not in debug mode - returning from acknowledge")
	}
	
	var msg = "Parameters:\n"
	
	for key, valueAr := range values {  // valueAr is a []string
		msg = msg + "\t" + key + ": "
		for i, value := range valueAr {
			if i > 0 { msg = msg + ";" }
			msg = msg + value
		}
		msg = msg + "\n"
	}
	
	msg = msg + "Files:\n"
	var headers []*multipart.FileHeader = files["filename"]
	
	for _, header := range headers {
		var filename string = header.Filename	
		var file multipart.File
		var err error
		file, err = header.Open()
		if err != nil { return apitypes.NewFailureDesc(http.StatusBadRequest, err.Error()) }
		if file == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError, "no file found") }	
		var buf = make([]byte, 100000)
		var size = 0
		for {
			var n int
			n, err = file.Read(buf)
			size = size + n
			if n < 100000 { break }
		}
		if err != nil { return apitypes.NewFailureDesc(http.StatusBadRequest, err.Error()) }
		msg = msg + filename + ": " + fmt.Sprintf("%d", size) + " bytes\n"
	}
	
	msg = msg + "End of data"
	
	return apitypes.NewResult(200, msg)
}

/*******************************************************************************
 * Arguments: apitypes.Credentials
 * Returns: apitypes.SessionToken
 */
func authenticate(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var creds *apitypes.Credentials
	var err error
	creds, err = apitypes.GetCredentials(values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Verify credentials.
	var user User
	user, err = dbClient.dbGetUserByUserId(creds.UserId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil {
		return apitypes.NewFailureDesc(http.StatusBadRequest, "User not found in the database")
	}
	
	if ! user.isActive() { return apitypes.NewFailureDesc(http.StatusUnauthorized,
		"User is not active") }
	
	// Check against brute force password attack.
	var attempts []string = user.getMostRecentLoginAttempts()
	if len(attempts) >= dbClient.Server.MaxLoginAttemptsToRetain &&
		apitypes.AreAllWithinTimePeriod(attempts, 600) {
		dbClient.Server.LoginAlert(creds.UserId)
		return apitypes.NewFailureDesc(http.StatusUnauthorized, "Too many login attempts")
	}
	user.addLoginAttempt(dbClient)
	
	// Verify password.
	if ! user.validatePassword(dbClient, creds.Password) {
		return apitypes.NewFailureDesc(http.StatusBadRequest, "Invalid password")
	}
	
	// Create new user session.
	var newSessionToken *apitypes.SessionToken = dbClient.Server.authService.createSession(creds)
	newSessionToken.SetRealmId(user.getRealmId())
	
	// If the user had a prior session, invalidate it.
	if sessionToken != nil {
		dbClient.Server.authService.invalidateSessionId(sessionToken.UniqueSessionId)
	}
	
	// Flag whether the user has Write access to the realm.
	var realm Realm
	realm, err = dbClient.getRealm(user.getRealmId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var entry ACLEntry
	entry, err = realm.getACLEntryForPartyId(dbClient, user.getId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if entry != nil {
		newSessionToken.SetIsAdminUser(entry.getPermissionMask()[apitypes.CanWrite])
	}
	
	return newSessionToken
}

/*******************************************************************************
 * Arguments: none
 * Returns: apitypes.Result
 */
func logout(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	_, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	dbClient.Server.authService.invalidateSessionId(sessionToken.UniqueSessionId)
	return apitypes.NewResult(200, "Logged out")
}

/*******************************************************************************
 * Arguments: apitypes.UserInfo
 * Returns: apitypes.UserDesc
 */
func createUser(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var err error
	var userInfo *apitypes.UserInfo
	userInfo, err = apitypes.GetUserInfo(values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.CreateInMask, userInfo.RealmId,
		"createUser")
	if failMsg != nil { return failMsg }
	
	// Legacy - uses Cesanta. Probably remove this.
//	if ! dbClient.Server.authService.authorized(dbClient.Server.sessions[sessionToken.UniqueSessionId],
//		"admin",  // this 'resource' is onwed by the admin account
//		"repository",
//		"*",  // the scope is the entire repository
//		[]string{"create-user"}) { // this is the action that is being requested
//	
//		//"registry.docker.com", "repository:samalba/my-app:push", "jlhawn")
//		fmt.Println("Unauthorized: %s, %s, %s")
//		return nil
//	}
	
	// Create the user account.
	var newUserId string = userInfo.UserId
	var newUserName string = userInfo.UserName
	var realmId string = userInfo.RealmId
	var email string = userInfo.EmailAddress
	var pswd string = userInfo.Password
	var newUser User
	newUser, err = dbClient.dbCreateUser(newUserId, newUserName, email, pswd, realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	if email != "" {
		err = EstablishEmail(dbClient.getServer().authService, dbClient,
			dbClient.getServer().EmailService, newUserId, email)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
	}
	
	return newUser.asUserDesc(dbClient)
}

/*******************************************************************************
 * Arguments: UserObjId
 * Returns: apitypes.Result
 */
func disableUser(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var userObjId string
	var err error
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var user User
	user, err = dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		user.getRealmId(), "disableUser")
	if failMsg != nil { return failMsg }
	
	// Prevent the user from authenticating.
	dbClient.setActive(user, false)
	
	return apitypes.NewResult(200, "User with user Id '" + user.getUserId() + "' disabled")
}

/*******************************************************************************
 * Arguments: UserObjId
 * Returns: apitypes.Result
 */
func reenableUser(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var userObjId string
	var err error
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var user User
	user, err = dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		user.getRealmId(), "reenableUser")
	if failMsg != nil { return failMsg }
	
	if user.isActive() { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"User " + user.getUserId() + " is already active") }
	
	// Enable the user to authenticate.
	dbClient.setActive(user, true)
	
	return apitypes.NewResult(200, "User with user Id '" + user.getUserId() + "' reenabled")
}

/*******************************************************************************
 * Arguments: UserId, OldPassword, NewPassword
 * Returns: apitypes.Result
 */
func changePassword(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var userId string
	var err error
	userId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Check that the userId is for the user who is currently logged in.
	if sessionToken.AuthenticatedUserid != userId {
		return apitypes.NewFailureDesc(http.StatusForbidden,
			"Only an account owner may change their own password")
	}
	
	var user User
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError, "User unidentified") }
	
	var oldPswd string
	oldPswd, err = apitypes.GetRequiredHTTPParameterValue(true, values, "OldPassword")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	if ! user.validatePassword(dbClient,oldPswd) {
		return apitypes.NewFailureDesc(http.StatusBadRequest, "Invalid password")
	}
	
	var newPswd string
	newPswd, err = apitypes.GetRequiredHTTPParameterValue(true, values, "NewPassword")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		user.getId(), "changePassword")
	if failMsg != nil { return failMsg }
	
	err = user.setPassword(dbClient, newPswd)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewResult(200, "Password changed")
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: apitypes.GroupDesc
 */
func createGroup(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var groupName string
	groupName, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Name")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	var groupDescription string
	groupDescription, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var addMeStr string
	addMeStr, err = apitypes.GetHTTPParameterValue(true, values, "AddMe")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var addMe bool = false
	if addMeStr == "true" { addMe = true }
	fmt.Println(fmt.Sprintf("AddMe=%s", addMeStr))

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.CreateInMask,
		realmId, "createGroup")
	if failMsg != nil { return failMsg }
	
	var group Group
	group, err = dbClient.dbCreateGroup(realmId, groupName, groupDescription)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	if addMe {
		var userId string = sessionToken.AuthenticatedUserid
		var user User
		user, err = dbClient.dbGetUserByUserId(userId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		err = group.addUserId(dbClient, user.getId())
		if err != nil { return apitypes.NewFailureDescFromError(err) }
	}
	
	return group.asGroupDesc()
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: apitypes.Result
 */
func deleteGroup(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var groupId string
	var err error
	groupId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "GroupId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var group Group
	group, err = dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		group.getRealmId(), "deleteGroup")
	if failMsg != nil { return failMsg }
	
	var groupName string = group.getName()
	var realm Realm
	realm, err = group.getRealm(dbClient)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	err = realm.deleteGroup(dbClient, group)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return apitypes.NewResult(200, "Group '" + groupName + "' deleted")
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: []*apitypes.UserDesc
 */
func getGroupUsers(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "GroupId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var group Group
	group, err = dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if group == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		fmt.Sprintf("No group with Id %s", groupId))
	}

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask,
		group.getRealmId(), "getGroupUsers")
	if failMsg != nil { return failMsg }
	
	var userObjIds []string = group.getUserObjIds()
	var userDescs apitypes.UserDescs = make([]*apitypes.UserDesc, 0)
	for _, id := range userObjIds {
		var user User
		user, err = dbClient.getUser(id)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if user == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			fmt.Sprintf("Internal error: No user with Id %s", id))
		}
		var userDesc *apitypes.UserDesc = user.asUserDesc(dbClient)
		userDescs = append(userDescs, userDesc)
	}
	
	return userDescs
}

/*******************************************************************************
 * Arguments: GroupId, UserObjId
 * Returns: apitypes.Result
 */
func addGroupUser(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "GroupId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var userObjId string
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var group Group
	group, err = dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if group == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		fmt.Sprintf("No group with Id %s", groupId))
	}

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		group.getRealmId(), "addGroupUser")
	if failMsg != nil { return failMsg }
	
	err = group.addUserId(dbClient, userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var user User
	user, err = dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"User with Id " + userObjId + " unidentified") }
	user.addGroupIdDeferredUpdate(dbClient, groupId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewResult(200, "User added to group")
}

/*******************************************************************************
 * Arguments: GroupId, UserObjId
 * Returns: apitypes.Result
 */
func remGroupUser(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "GroupId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var userObjId string
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var group Group
	group, err = dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		group.getRealmId(), "remGroupUser")
	if failMsg != nil { return failMsg }
	
	var user User
	user, err = dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	err = group.removeUser(dbClient, user)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewResult(200, "User " + user.getName() + " removed from group " + group.getName())
}

/*******************************************************************************
 * Arguments: apitypes.RealmInfo, apitypes.UserInfo
 * Returns: apitypes.UserDesc
 */
func createRealmAnon(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	// Create new administrative user.
	var err error
	var userInfo *apitypes.UserInfo
	userInfo, err = apitypes.GetUserInfo(values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var newUserId string = userInfo.UserId
	var newUserName string = userInfo.UserName
	//var realmId string = userInfo.RealmId  // ignored
	var email string = userInfo.EmailAddress
	var pswd string = userInfo.Password

	// Create a realm.
	var newRealmInfo *apitypes.RealmInfo
	newRealmInfo, err = apitypes.GetRealmInfo(values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Creating realm", newRealmInfo.RealmName)
	var newRealm Realm
	newRealm, err = dbClient.dbCreateRealm(newRealmInfo, newUserId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Created realm", newRealmInfo.RealmName)
	
	// Create a user
	var newUser User
	newUser, err = dbClient.dbCreateUser(newUserId, newUserName, email, pswd, newRealm.getId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if newUser != nil {
		err = EstablishEmail(dbClient.getServer().authService, dbClient,
			dbClient.getServer().EmailService, newUserId, email)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
	}
	
	// Add ACL entry to enable the current user (if any) to access what he/she just created.
	var curUser User
	var sessionError error
	curUser, sessionError = getCurrentUser(dbClient, sessionToken)
	if (curUser != nil) && (sessionError == nil) {
		_, err = dbClient.dbCreateACLEntry(newRealm.getId(), curUser.getId(),
			[]bool{ true, true, true, true, true } )
		if err != nil { return apitypes.NewFailureDescFromError(err) }
	}
	
	// Add ACL entry to enable the new user to access what was just created.
	_, err = dbClient.dbCreateACLEntry(newRealm.getId(), newUser.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	if sessionError != nil { return apitypes.NewFailureDescFromError(sessionError) }
	return newUser.asUserDesc(dbClient)
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: apitypes.RealmDesc
 */
func getRealmDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, realmId,
		"getRealmDesc")
	if failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if realm == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Cound not find realm with Id " + realmId) }
	
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmName
 * Returns: apitypes.RealmDesc
 */
func getRealmByName(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var err error
	var realmName string
	realmName, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmName")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var realmId string
	realmId, err = dbClient.getPersistence().GetRealmObjIdByRealmName(realmName)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if realmId == "" { return apitypes.NewFailureDesc(
		http.StatusBadRequest, "Realm with name " + realmName + " not found")
	}
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, realmId,
		"getRealmByName")
	if failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if realm == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Cound not find realm with Id " + realmId) }
	
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmInfo
 * Returns: apitypes.RealmDesc
 */
func createRealm(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var realmInfo *apitypes.RealmInfo
	realmInfo, err = apitypes.GetRealmInfo(values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var user User
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Creating realm", realmInfo.RealmName)
	var realm Realm
	realm, err = dbClient.dbCreateRealm(realmInfo, user.getId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Created realm", realmInfo.RealmName)

	// Add ACL entry to enable the current user to access what he/she just created.
	_, err = dbClient.dbCreateACLEntry(realm.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: apitypes.Result
 */
func deactivateRealm(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.DeleteMask, realmId,
		"deactivateRealm")
	if failMsg != nil { return failMsg }

	err = dbClient.dbDeactivateRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	return apitypes.NewResult(200, "Realm deactivated")
}

/*******************************************************************************
 * Arguments: UserObjId, RealmId
 * Returns: apitypes.Result
 */
func moveUserToRealm(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var realmId string
	var userObjId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask, realmId,
		"moveUserToRealm")
	if failMsg != nil { return failMsg }
	
	var destRealm Realm
	destRealm, err = dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if destRealm == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Cound not find realm with Id " + realmId) }
	
	var user User
	user, err = dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var origRealm Realm
	origRealm, err = dbClient.getRealm(user.getRealmId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	if origRealm.getId() == destRealm.getId() {
		return apitypes.NewFailureDesc(http.StatusBadRequest, "The user is already in the destination realm")
	}
	
	fmt.Println("Removing user Id " + userObjId + " from realm Id " + origRealm.getId())
	_, err = origRealm.removeUserId(dbClient, userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Adding user Id " + userObjId + " to realm Id " + destRealm.getId())
	err = destRealm.addUserId(dbClient, userObjId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewResult(200, "User moved to realm")
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: apitypes.UserDesc
 */
func getUserDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var userId string
	userId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var user User
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"User with user id " + userId + " not found.") }
	
	var realm Realm
	realm, err = user.getRealm(dbClient)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, realm.getId(),
		"getUserDesc")
	if failMsg != nil { return failMsg }
	
	return user.asUserDesc(dbClient)
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*apitypes.UserDesc
 */
func getRealmUsers(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, realmId,
		"getRealmUsers")
	if failMsg != nil { return failMsg }
		
	var realm Realm
	realm, err = dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if realm == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Realm with Id " + realmId + " not found") }
	var userObjIds []string = realm.getUserObjIds()
	var userDescs apitypes.UserDescs = make([]*apitypes.UserDesc, 0)
	for _, userObjId := range userObjIds {
		var user User
		user, err = dbClient.getUser(userObjId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if user == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"User with obj Id " + userObjId + " not found") }
		var userDesc *apitypes.UserDesc = user.asUserDesc(dbClient)
		userDescs = append(userDescs, userDesc)
	}
	return userDescs
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*apitypes.GroupDesc
 */
func getRealmGroups(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var groupDescs apitypes.GroupDescs = make([]*apitypes.GroupDesc, 0)
	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, realmId,
		"getRealmGroups")
	if failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if realm == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Realm with Id " + realmId + " not found") }
	var groupIds []string = realm.getGroupIds()
	for _, groupId := range groupIds {
		var group Group
		group, err = dbClient.getGroup(groupId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if group == nil {
			fmt.Println("Internal error: group with Id " + groupId + " not found")
			continue
		}
		groupDescs = append(groupDescs, group.asGroupDesc())
	}
	return groupDescs
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*apitypes.RepoDesc
 */
func getRealmRepos(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, realmId,
		"getRealmRepos")
	if failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if realm == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Cound not find realm with Id " + realmId) }
	
	var repoIds []string = realm.getRepoIds()
	
	var result apitypes.RepoDescs = make([]*apitypes.RepoDesc, 0)
	for _, id := range repoIds {
		
		var repo Repo
		repo, err = dbClient.getRepo(id)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if repo == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			fmt.Sprintf("Internal error: no Repo found for Id %s", id))
		}
		var desc *apitypes.RepoDesc = repo.asRepoDesc()
		// Add to result
		result = append(result, desc)
	}

	return result
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: []*apitypes.RealmDesc
 */
func getAllRealms(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var realmIds []string
	var err error
	realmIds, err = dbClient.dbGetAllRealmIds()
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var result apitypes.RealmDescs = make([]*apitypes.RealmDesc, 0)
	for _, realmId := range realmIds {
		
		var realm Realm
		var err error
		realm, err = dbClient.getRealm(realmId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if realm == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			fmt.Sprintf("Internal error: no Realm found for Id %s", realmId))
		}
		var desc *apitypes.RealmDesc = realm.asRealmDesc()
		result = append(result, desc)
	}
	
	return result
}

/*******************************************************************************
 * Arguments: RealmId, Name, Description, <optional: File attachment>
 * Returns: apitypes.RepoDesc
 */
func createRepo(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	fmt.Println("Creating repo...")
	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RealmId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var repoName string
	repoName, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Name")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	var repoDesc string
	repoDesc, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.CreateInMask, realmId,
		"createRepo")
	if failMsg != nil { return failMsg }
	
	fmt.Println("Creating repo", repoName)
	var repo Repo
	repo, err = dbClient.dbCreateRepo(realmId, repoName, repoDesc)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	// Add ACL entry to enable the current user to access what he/she just created.
	var user User
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	_, err = dbClient.dbCreateACLEntry(repo.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var name string
	var filepath string
	name, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if filepath != "" { // a file was attached - presume that it is a dockerfile
		var newDockerfile Dockerfile
		newDockerfile, err = createDockerfile(sessionToken, dbClient, repo,
			name, filepath, repo.getDescription())
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		var desc *apitypes.RepoPlusDockerfileDesc
		desc, err = repo.asRepoPlusDockerfileDesc(dbClient, newDockerfile.getId())
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		return desc
	}
	
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: apitypes.Result
 */
func deleteRepo(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.DeleteMask, repoId,
		"deleteRepo")
	if failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var realmId = repo.getRealmId()
	var realm Realm
	realm, err = dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	err = realm.deleteRepo(dbClient, repo)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewResult(200, "Repo deleted")
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []*apitypes.DockerfileDesc
 */
func getDockerfiles(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, repoId,
		"getDockerfiles")
	if failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if repo == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		fmt.Sprintf("Repo with Id %s not found", repoId)) }
	
	var dockerfileIds []string = repo.getDockerfileIds()	
	var result apitypes.DockerfileDescs = make([]*apitypes.DockerfileDesc, 0)
	for _, id := range dockerfileIds {
		
		var dockerfile Dockerfile
		dockerfile, err = dbClient.getDockerfile(id)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if dockerfile == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			fmt.Sprintf("Internal error: no Dockerfile found for Id %s", id))
		}
		var desc *apitypes.DockerfileDesc
		desc, err = dockerfile.asDockerfileDesc()
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		// Add to result
		result = append(result, desc)
	}

	return result
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []*apitypes.DockerImageDesc
 */
func getDockerImages(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, repoId,
		"getDockerImages")
	if failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if repo == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		fmt.Sprintf("Repo with Id %s not found", repoId)) }
	
	var imageIds []string = repo.getDockerImageIds()
	var result apitypes.DockerImageDescs = make([]*apitypes.DockerImageDesc, 0)
	for _, id := range imageIds {
		
		var dockerImage DockerImage
		dockerImage, err = dbClient.getDockerImage(id)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if dockerImage == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
				fmt.Sprintf("Internal error: no DockerImage found for Id %s", id))
		}
		var imageDesc *apitypes.DockerImageDesc = dockerImage.asDockerImageDesc()
		result = append(result, imageDesc)
	}
	
	return result
}

/*******************************************************************************
 * Arguments: RepoId, File
 * Returns: apitypes.DockerfileDesc
 * The File argument is obtained from the values as follows:
 *    The name specified by the client is keyed on "filename".
 * The handler should move the file to a permanent name.
 */
func addDockerfile(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Identify the repo.
	var repoId string
	var err error
	repoId, err = apitypes.GetHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	// If not specified, use default repo (and create if not exist).
	var repo Repo
	if repoId == "" {
		repo, err = getDefaultRepoForUser(dbClient, sessionToken.AuthenticatedUserid)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		repoId = repo.getId()
	} else {
		repo, err = dbClient.getRepo(repoId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if repo == nil { return apitypes.NewFailureDesc(http.StatusBadRequest, "Repo does not exist") }
	}

	var desc string
	desc, err = apitypes.GetHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.CreateInMask, repoId,
		"addDockerfile")
	if failMsg != nil { return failMsg }
	
	var name string
	var filepath string
	name, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if filepath == "" { return apitypes.NewFailureDesc(http.StatusBadRequest, "No file was found") }
	
	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, name, filepath, desc)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if dockerfile == nil { return apitypes.NewFailureDesc(http.StatusBadRequest, "No dockerfile was attached") }
	
	var dockerfileDesc *apitypes.DockerfileDesc
	dockerfileDesc, err = dockerfile.asDockerfileDesc()
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return dockerfileDesc
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: GroupDesc
 */
func getGroupDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Identify the group.
	var groupId string
	var err error
	groupId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "GroupId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var group Group
	group, err = dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return group.asGroupDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: RepoDesc
 */
func getRepoDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Identify the repo.
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: DockerImageId
 * Returns: DockerImageDesc or DockerImageVersionDesc
 */
func getDockerImageDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Identify the repo.
	var imageId string
	var err error
	imageId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerImageId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var image DockerImage
	image, err = dbClient.getDockerImage(imageId)
	if err != nil {
		var imageVersion DockerImageVersion
		imageVersion, err = dbClient.getDockerImageVersion(imageId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		var imageVersionDesc *apitypes.DockerImageVersionDesc
		imageVersionDesc, err = imageVersion.asDockerImageVersionDesc(dbClient)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		return imageVersionDesc
	}
	return image.asDockerImageDesc()
}

/*******************************************************************************
 * Arguments: DockerfileId
 * Returns: DockerfileDesc
 */
func getDockerfileDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Identify the dockerfile.
	var dockerfileId string
	var err error
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var dockerfile Dockerfile
	dockerfile, err = dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var dockerfileDesc *apitypes.DockerfileDesc
	dockerfileDesc, err = dockerfile.asDockerfileDesc()
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return dockerfileDesc
}

/*******************************************************************************
 * Arguments: DockerfileId, Description (optional), File
 * Returns: apitypes.Result
 */
func replaceDockerfile(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var dockerfileId string
	var desc string
	var err error
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	desc, err = apitypes.GetHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var dockerfile Dockerfile
	dockerfile, err = dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		dockerfileId, "replaceDockerfile")
	if failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = dockerfile.getRepo(dbClient)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var filepath string
	_, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if filepath == "" { return apitypes.NewFailureDesc(http.StatusBadRequest, "No file was found") }
	
	
	//....create DockerfileExecParameterValues

	
	
	err = dockerfile.replaceDockerfileFile(filepath, desc)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return apitypes.NewResult(200, "Dockerfile file replaced")
}

/*******************************************************************************
 * Arguments: DockerfileId, ImageName
 * Returns: DockerImageVersionDesc
 */
func execDockerfile(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	fmt.Println("Entered execDockerfile")
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Identify the Dockerfile.
	var err error
	var dockerfileId string
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if dockerfileId == "" { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"No HTTP parameter found for DockerfileId") }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ExecuteMask, dockerfileId,
		"execDockerfile")
	if failMsg != nil { return failMsg }
	
	var dockerfile Dockerfile
	dockerfile, err = dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Dockerfile name =", dockerfile.getName())
	
	var imageVersion DockerImageVersion
	imageVersion, err = buildDockerfile(dbClient, dockerfile, sessionToken, values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Built from dockerfile")
	
	var imageVersionDesc *apitypes.DockerImageVersionDesc
	imageVersionDesc, err = imageVersion.asDockerImageVersionDesc(dbClient)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	fmt.Println("Returning image description")
	
	return imageVersionDesc
}

/*******************************************************************************
 * Arguments: RepoId, Description, ImageName, <File attachment>
 * Returns: DockerImageVersionDesc
 */
func addAndExecDockerfile(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	fmt.Println("Entered addAndExecDockerfile")
	
	var failMsg apitypes.RespIntfTp = nil
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var repoId string
	var err error
	repoId, err = apitypes.GetHTTPParameterValue(true, values, "RepoId")
	// If not specified, use default repo (and create if not exist).
	var repo Repo
	if repoId == "" {
		repo, err = getDefaultRepoForUser(dbClient, sessionToken.AuthenticatedUserid)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		repoId = repo.getId()
	} else {
		repo, err = dbClient.getRepo(repoId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if repo == nil { return apitypes.NewFailureDesc(http.StatusBadRequest, "Repo does not exist") }
	}
	
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask, repoId,
		"addAndExecDockerfile")
	if failMsg != nil { return failMsg }
	
	var desc string
	desc, err = apitypes.GetHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	var name string
	var filepath string
	name, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if filepath == "" { return apitypes.NewFailureDesc(http.StatusBadRequest, "No file was found") }
	
	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, name, filepath, desc)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if dockerfile == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"No dockerfile was attached") }
	
	var imageVersion DockerImageVersion
	imageVersion, err = buildDockerfile(dbClient, dockerfile, sessionToken, values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	
	//....create DockerfileExecParameterValues

	var imageVersionDesc *apitypes.DockerImageVersionDesc
	imageVersionDesc, err = imageVersion.asDockerImageVersionDesc(dbClient)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return imageVersionDesc
}

/*******************************************************************************
 * Arguments: ImageObjId
 * Returns: file content
 */
func downloadImage(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var imageObjId string
	var err error
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Determine if imageObjId is a DockerImage or a DockerImageVersion.
	var dockerImage DockerImage
	var dockerImageId string
	var dockerImageVersion DockerImageVersion
	var dockerImageVersionId string
	dockerImage, err = dbClient.getDockerImage(imageObjId)
	if err == nil {
		// User specified a DockerImage - not an image version.
		dockerImageId = imageObjId
		// Get most recent image version.
		dockerImageVersionId = dockerImage.getMostRecentVersionId()
		if dockerImageVersionId == "" { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Null most recent DockerImageVersion for image " + dockerImage.getName())
		}
		dockerImageVersion, err = dbClient.getDockerImageVersion(dockerImageVersionId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Docker image version with object Id " + dockerImageVersionId + " not found")
		}
	} else {
		dockerImageVersion, err = dbClient.getDockerImageVersion(imageObjId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
			"Docker image version with object Id " + imageObjId + " not found")
		}
		// User specified a particular image version.
		dockerImageVersionId = imageObjId
		dockerImageId = dockerImageVersion.getImageObjId()
		dockerImage, err = dbClient.getDockerImage(dockerImageId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Docker image with object Id " + dockerImageId + " not found")
		}
	}
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, dockerImageId,
		"downloadImage")
	if failMsg != nil { return failMsg }

	var tempFilePath string
	var realmName, repoName, imageName, version string
	realmName, repoName, imageName, version, err = dockerImageVersion.getFullNameParts(dbClient)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var dockerImageName, tag string
	dockerImageName, tag = docker.ConstructDockerImageName(realmName, repoName, imageName, version)
	tempFilePath, err = dbClient.getServer().DockerServices.SaveImage(dockerImageName, tag)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewFileResponse(200, tempFilePath, true)
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func setPermission(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Get the mask that we will use to overwrite the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "PartyId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var resourceId string
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ResourceId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var smask []string = make([]string, 5)
	smask[0], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanCreateIn")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[1], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanRead")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[2], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanWrite")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[3], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanExecute")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[4], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanDelete")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var mask []bool
	mask, err = apitypes.ToBoolAr(smask)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask, resourceId,
		"setPermission")
	if failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var resource Resource
	resource, err = dbClient.getResource(resourceId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if resource == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if party == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	var aclEntry ACLEntry
	aclEntry, err = dbClient.setAccess(resource, party, mask)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return aclEntry.asPermissionDesc()
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func addPermission(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Get the mask that we will be adding to the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "PartyId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var resourceId string
	var smask []string = make([]string, 5)
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ResourceId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[0], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanCreateIn")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[1], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanRead")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[2], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanWrite")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[3], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanExecute")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	smask[4], err = apitypes.GetRequiredHTTPParameterValue(true, values, "CanDelete")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var mask []bool
	mask, err = apitypes.ToBoolAr(smask)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask, resourceId,
		"addPermission")
	if failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var resource Resource
	resource, err = dbClient.getResource(resourceId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if resource == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if party == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	var aclEntry ACLEntry
	aclEntry, err = dbClient.addAccess(resource, party, mask)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return aclEntry.asPermissionDesc()
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: Result
 */
func remPermission(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	// Get the mask that we will be subracting from the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "PartyId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var resourceId string
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ResourceId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask, resourceId,
		"remPermission")
	if failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var resource Resource
	resource, err = dbClient.getResource(resourceId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if resource == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if party == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	err = dbClient.deleteAccess(resource, party)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewResult(200, "All permission removed")
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId
 * Returns: PermissionDesc
 */
func getPermission(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "PartyId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var resourceId string
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ResourceId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, resourceId,
		"getPermission")
	if failMsg != nil { return failMsg }
	
	// Identify the Resource.
	//var resource Resource = dbClient.getResource(resourceId)
	//if resource == nil { return apitypes.NewFailureDesc("Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if party == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify party with Id " + partyId) }
	
	// Return the ACLEntry.
	var aclEntry ACLEntry
	aclEntry, err = party.getACLEntryForResourceId(dbClient, resourceId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var mask []bool
	var aclEntryId string = ""
	if aclEntry == nil {
		mask = make([]bool, 5)
	} else {
		mask = aclEntry.getPermissionMask()
		aclEntryId = aclEntry.getId()
	}
	return apitypes.NewPermissionDesc(aclEntryId, resourceId, partyId, mask)
}

/*******************************************************************************
 * Arguments: 
 * Returns: apitypes.UserDesc
 */
func getMyDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	if sessionToken == nil { return apitypes.NewFailureDesc(
		http.StatusUnauthorized, "User not authenticated") }
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil {
		return apitypes.NewFailureDesc(http.StatusBadRequest,
			"user object cannot be identified from user id " + userId)
	}

	return user.asUserDesc(dbClient)
}

/*******************************************************************************
 * Arguments: 
 * Returns: []*apitypes.GroupDesc
 */
func getMyGroups(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var groupDescs apitypes.GroupDescs = make([]*apitypes.GroupDesc, 0)
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var groupIds []string = user.getGroupIds()
	for _, groupId := range groupIds {
		var group Group
		var err error
		group, err = dbClient.getGroup(groupId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if group == nil {
			fmt.Println("Internal error: group with Id " + groupId + " could not be found")
			continue
		}
		groupDescs = append(groupDescs, group.asGroupDesc())
	}
	return groupDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: []*apitypes.RealmDesc
 */
func getMyRealms(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var realms map[string]Realm = make(map[string]Realm)
	
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var aclEntrieIds []string = user.getACLEntryIds()
	fmt.Println("For each acl entry...")
	for _, aclEntryId := range aclEntrieIds {
		var err error
		var aclEntry ACLEntry
		aclEntry, err = dbClient.getACLEntry(aclEntryId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		var resourceId string = aclEntry.getResourceId()
		var resource Resource
		resource, err = dbClient.getResource(resourceId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		switch v := resource.(type) {
			case Realm: realms[v.getId()] = v
				fmt.Println("\t\ta Realm")
			default: fmt.Println("\t\ta " + reflect.TypeOf(v).String())
		}
	}
	fmt.Println("For each realm...")
	var realmDescs apitypes.RealmDescs = make([]*apitypes.RealmDesc, 0)
	for _, realm := range realms {
		fmt.Println("\tappending realm", realm.getName())
		realmDescs = append(realmDescs, realm.asRealmDesc())
	}
	return realmDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: []*apitypes.RepoDesc
 */
func getMyRepos(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	// Traverse the user's ACL entries; form the union of the repos that the user
	// has explicit access to, and the repos that belong to the realms that the user
	// has access to.
	
	var realms map[string]Realm = make(map[string]Realm)
	var repos map[string]Repo = make(map[string]Repo)
	
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var aclEntrieIds []string = user.getACLEntryIds()
	for _, aclEntryId := range aclEntrieIds {
		var err error
		var aclEntry ACLEntry
		aclEntry, err = dbClient.getACLEntry(aclEntryId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		var resourceId string = aclEntry.getResourceId()
		var resource Resource
		resource, err = dbClient.getResource(resourceId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		switch v := resource.(type) {
			case Realm: realms[v.getId()] = v
			case Repo: repos[v.getId()] = v
		}
	}
	for _, realm := range realms {
		// Add all of the repos belonging to realm.
		for _, repoId := range realm.getRepoIds() {
			var r Repo
			var err error
			r, err = dbClient.getRepo(repoId)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
			if r == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
				"Internal error: No repo found for Id " + repoId) }
			repos[repoId] = r
		}
	}
	var repoDescs apitypes.RepoDescs = make([]*apitypes.RepoDesc, 0)
	for _, repo := range repos {
		repoDescs = append(repoDescs, repo.asRepoDesc())
	}
	
	return repoDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: DockerfileDescs
 */
func getMyDockerfiles(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var leaves map[string]Resource
	leaves, err = getLeafResources(dbClient, user, ADockerfile)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	fmt.Println("Creating result...")
	var dockerfileDescs apitypes.DockerfileDescs = make([]*apitypes.DockerfileDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending dockerfile", leaf.getName())
		var dockerfile Dockerfile
		var isType bool
		dockerfile, isType = leaf.(Dockerfile)
		if ! isType { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Internal error: type of resource is unexpected") }
		var dockerfileDesc *apitypes.DockerfileDesc
		dockerfileDesc, err = dockerfile.asDockerfileDesc()
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		dockerfileDescs = append(dockerfileDescs, dockerfileDesc)
	}
	return dockerfileDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: DockerImageDescs
 */
func getMyDockerImages(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var leaves map[string]Resource
	leaves, err = getLeafResources(dbClient, user, ADockerImage)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	fmt.Println("Creating result...")
	var dockerImageDescs apitypes.DockerImageDescs = make([]*apitypes.DockerImageDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending docker image", leaf.getName())
		var dockerImage DockerImage
		var isType bool
		dockerImage, isType = leaf.(DockerImage)
		if ! isType { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Internal error: type of resource is unexpected") }
		dockerImageDescs = append(dockerImageDescs, dockerImage.asDockerImageDesc())
	}
	return dockerImageDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: ScanConfigDesc...
 */
func getMyScanConfigs(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var leaves map[string]Resource
	leaves, err = getLeafResources(dbClient, user, AScanConfig)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	fmt.Println("Creating result...")
	var dcanConfigDescs apitypes.ScanConfigDescs = make([]*apitypes.ScanConfigDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending scan config", leaf.getName())
		var dcanConfig ScanConfig
		var isType bool
		dcanConfig, isType = leaf.(ScanConfig)
		if ! isType { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Internal error: type of resource is unexpected") }
		dcanConfigDescs = append(dcanConfigDescs, dcanConfig.asScanConfigDesc(dbClient))
	}
	return dcanConfigDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: FlagDesc...
 */
func getMyFlags(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var leaves map[string]Resource
	leaves, err = getLeafResources(dbClient, user, AFlag)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	fmt.Println("Creating result...")
	var flagDescs apitypes.FlagDescs = make([]*apitypes.FlagDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending docker image", leaf.getName())
		var flag Flag
		var isType bool
		flag, isType = leaf.(Flag)
		if ! isType { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Internal error: type of resource is unexpected") }
		flagDescs = append(flagDescs, flag.asFlagDesc())
	}
	return flagDescs
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: scanners.ScanProviderDescs
 */
func getScanProviders(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var providerDescs scanners.ScanProviderDescs = make([]*scanners.ScanProviderDesc, 0)
	var services []scanners.ScanService = dbClient.Server.GetScanServices()
	for _, service := range services {
		providerDescs = append(providerDescs, service.AsScanProviderDesc())
	}
	
	return providerDescs
}

/*******************************************************************************
 * Arguments: Name, Description, RepoId, ProviderName, Params..., SuccessExpression, <optional File>
 * Returns: ScanConfigDesc
 */
func defineScanConfig(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var err error
	var providerName string
	providerName, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ProviderName")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var scanService scanners.ScanService
	scanService = dbClient.Server.GetScanService(providerName)
	if scanService == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify a scan service named '" + providerName + "'")
	}
	
	var name string
	name, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Name")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var desc string
	desc, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var repoId string
	repoId, err = apitypes.GetHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	// If not specified, use default repo (and create if not exist).
	var repo Repo
	if repoId == "" {
		repo, err = getDefaultRepoForUser(dbClient, sessionToken.AuthenticatedUserid)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		repoId = repo.getId()
	} else {
		repo, err = dbClient.getRepo(repoId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if repo == nil { return apitypes.NewFailureDesc(http.StatusBadRequest, "Repo does not exist") }
	}
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask, repoId,
		"defineScanConfig")
	if failMsg != nil { return failMsg }
	
	var successExpr string = ""
	successExpr, err = apitypes.GetHTTPParameterValue(true, values, "SuccessExpression")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var scanConfig ScanConfig
	var paramValueIds []string = make([]string, 0)
	scanConfig, err = dbClient.dbCreateScanConfig(name, desc, repoId,
		providerName, paramValueIds, successExpr, "")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Retrieve and set the provider parameters.
	for key, valueAr := range values {
		if strings.HasPrefix(key, "scan.") {
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(http.StatusBadRequest,
				"Parameter " + key + " is ill-formatted")
			}
			var paramName string = strings.TrimPrefix(key, "scan.")
			// See if the parameter is known by the scanner.
			_, err = scanService.GetParameterDescription(paramName)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
			// Create a ParameterValue and attach it to the ScanConfig.
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(http.StatusBadRequest,
				"Value for scan parameter '" + paramName + "' is ill-formed") }
			var value string = valueAr[0]
			_, err = scanConfig.setParameterValue(dbClient, paramName, value)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
		}
	}
	
	// Add ACL entry to enable the current user to access what he/she just created.
	var userId string = sessionToken.AuthenticatedUserid
	var user User
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
		"Internal error - could not identify user after use has been authenticated") }

	_, err = dbClient.dbCreateACLEntry(scanConfig.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Add success image, if one was attached.
	var imageFilepath string
	_, imageFilepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if imageFilepath != "" { // a file was attached - presume that it is an image
		fmt.Println("file attached...")
		var flag Flag
		flag, err = dbClient.dbCreateFlag(name, desc, repoId, imageFilepath)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		err = scanConfig.setFlagId(dbClient, flag.getId())
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if scanConfig.getFlagId() == "" { return apitypes.NewFailureDesc(
			http.StatusInternalServerError, "Flag set failed") }
		// Add ACL entry.
		_, err = dbClient.dbCreateACLEntry(flag.getId(), user.getId(),
			[]bool{ true, true, true, true, true } )
		if err != nil { return apitypes.NewFailureDescFromError(err) }
	}
	
	return scanConfig.asScanConfigDesc(dbClient)
}

/*******************************************************************************
 * Arguments: ScanConfigId, Name, Description, ProviderName, Params..., SuccessExpression, file
 * Returns: ScanConfigDesc
 */
func updateScanConfig(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
		
	// Update only the fields that are specified and that are not empty strings.
	var err error

	var scanConfigId string
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var scanConfig ScanConfig
	scanConfig, err = dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		scanConfig.getRepoId(), "defineScanConfig")
	if failMsg != nil { return failMsg }
	
	var name string
	name, err = apitypes.GetHTTPParameterValue(true, values, "Name")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if name != "" { scanConfig.setNameDeferredUpdate(name) }
	
	var desc string
	desc, err = apitypes.GetHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if desc != "" { scanConfig.setDescriptionDeferredUpdate(desc) }
	
	var providerName string
	providerName, err = apitypes.GetHTTPParameterValue(true, values, "ProviderName")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if providerName != "" { scanConfig.setProviderNameDeferredUpdate(providerName) }
	
	var successExpr string = ""
	successExpr, err = apitypes.GetHTTPParameterValue(true, values, "SuccessExpression")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if successExpr != "" { scanConfig.setSuccessExpressionDeferredUpdate(successExpr) }
	
	var scanService scanners.ScanService
	scanService = dbClient.Server.GetScanService(scanConfig.getProviderName())
	if scanService == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unable to identify a scan service named '" + providerName + "'")
	}
	
	err = dbClient.writeBack(scanConfig)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Retrieve and set the provider parameters.
	for key, valueAr := range values {
		if strings.HasPrefix(key, "scan.") {
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(http.StatusBadRequest,
				"Parameter " + key + " is ill-formatted")
			}
			var paramName string = strings.TrimPrefix(key, "scan.")
			// See if the parameter is known by the scanner.
			_, err = scanService.GetParameterDescription(paramName)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
			// Create a ParameterValue and attach it to the ScanConfig.
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(http.StatusBadRequest,
				"Value for scan parameter '" + paramName + "' is ill-formed") }
			var value string = valueAr[0]
			_, err = scanConfig.setParameterValue(dbClient, paramName, value)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
		}
	}
	
	// Add success image, if one was attached.
	var userId string = sessionToken.AuthenticatedUserid
	var user User
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
		"Internal error - could not identify user after use has been authenticated") }
	var imageFilepath string
	var repo Repo
	repo, err = dbClient.getRepo(scanConfig.getRepoId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	_, imageFilepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if imageFilepath != "" { // a file was attached - presume that it is an image
		fmt.Println("file attached...")
		var flag Flag
		flag, err = dbClient.dbCreateFlag(name, desc, repo.getId(), imageFilepath)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		err = scanConfig.setFlagId(dbClient, flag.getId())
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		// Add ACL entry.
		_, err = dbClient.dbCreateACLEntry(flag.getId(), user.getId(),
			[]bool{ true, true, true, true, true } )
		if err != nil { return apitypes.NewFailureDescFromError(err) }
	}
	
	return scanConfig.asScanConfigDesc(dbClient)
}

/*******************************************************************************
 * Arguments: FlagId
 * Returns: image file
 */
func getFlagImage(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var flagId string
	var err error
	flagId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "FlagId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var flag Flag
	flag, err = dbClient.getFlag(flagId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, flagId,
		"getFlagImage")
	if failMsg != nil { return failMsg }

	var path string = flag.getSuccessImagePath()
	
	return apitypes.NewFileResponse(200, path, false)
}

/*******************************************************************************
 * Arguments: RepoId, Name, Description, image file
 * Returns: FlagDesc
 */
func defineFlag(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var repoId string
	var err error
	repoId, err = apitypes.GetHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	// If not specified, use default repo (and create if not exist).
	var repo Repo
	if repoId == "" {
		repo, err = getDefaultRepoForUser(dbClient, sessionToken.AuthenticatedUserid)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		repoId = repo.getId()
	} else {
		repo, err = dbClient.getRepo(repoId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if repo == nil { return apitypes.NewFailureDesc(http.StatusBadRequest, "Repo does not exist") }
	}
	
	var name string
	name, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Name")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var desc string
	desc, err = apitypes.GetRequiredHTTPParameterValue(true, values, "Description")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var successImagePath string
	_, successImagePath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if successImagePath == "" { return apitypes.NewFailureDesc(
		http.StatusBadRequest, "No file attached") }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.CreateInMask, repoId,
		"defineFlag")
	if failMsg != nil { return failMsg }

	var flag Flag
	flag, err = dbClient.dbCreateFlag(name, desc, repoId, successImagePath)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return flag.asFlagDesc()
}

/*******************************************************************************
 * Arguments: ScanConfigId..., ImageObjId
 * Returns: ScanEventDescs
 */
func scanImage(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var scanConfigIdSeq, imageObjId string
	var err error
	scanConfigIdSeq, err = apitypes.GetHTTPParameterValue(false, values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Determine if imageObjId is a DockerImage or a DockerImageVersion.
	var dockerImage DockerImage
	var dockerImageId string
	var dockerImageVersion DockerImageVersion
	var dockerImageVersionId string
	dockerImage, err = dbClient.getDockerImage(imageObjId)
	if err == nil {
		// User specified a DockerImage - not an image version.
		dockerImageId = imageObjId
		// Get most recent image version.
		dockerImageVersionId = dockerImage.getMostRecentVersionId()
		if dockerImageVersionId == "" { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Null most recent DockerImageVersion for image " + dockerImage.getName())
		}
		dockerImageVersion, err = dbClient.getDockerImageVersion(dockerImageVersionId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Docker image version with object Id " + dockerImageVersionId + " not found")
		}
	} else {
		dockerImageVersion, err = dbClient.getDockerImageVersion(imageObjId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
			"Docker image version with object Id " + imageObjId + " not found")
		}
		// User specified a particular image version.
		dockerImageVersionId = imageObjId
		dockerImageId = dockerImageVersion.getImageObjId()
		dockerImage, err = dbClient.getDockerImage(dockerImageId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Docker image with object Id " + dockerImageId + " not found")
		}
	}
	
	// Determine the ScanConfigs to use.
	var scanConfigIds []string
	if scanConfigIdSeq == "" {
		// Obtain the linked scan configs.
		scanConfigIds = dockerImage.getScanConfigsToUse()
	} else {
		// Parse the list of scan config ids.
		scanConfigIds = strings.Split(scanConfigIdSeq, ",")
		for _, scanConfigId := range scanConfigIds {
			_, err = apitypes.Sanitize(scanConfigId)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
		}
	}
	
	// Perform scan with each ScanConfig.
	var scanEventDescs apitypes.ScanEventDescs = make(apitypes.ScanEventDescs, 0)
	for _, scanConfigId := range scanConfigIds {
		// Check if user is authorized to use the DockerImage and the ScanConfig.
		failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, dockerImageId,
			"scanImage")
		if failMsg != nil { return failMsg }
		failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ExecuteMask, scanConfigId,
			"scanImage")
		if failMsg != nil { return failMsg }
		
		// Retrieve the ScanConfig.
		var scanConfig ScanConfig
		scanConfig, err = dbClient.getScanConfig(scanConfigId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if scanConfig == nil {
			return apitypes.NewFailureDesc(http.StatusBadRequest,
				"Scan Config with object Id " + scanConfigId + " not found")
		}
	
		// Obtain scan provider name and parameters from the ScanConfig.
		var scanProviderName = scanConfig.getProviderName()
		var params = map[string]string{}
		var paramValueIds []string = scanConfig.getParameterValueIds()
		for _, id := range paramValueIds {
			var paramValue ParameterValue
			paramValue, err = dbClient.getParameterValue(id)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
			params[paramValue.getName()] = paramValue.getStringValue()
		}
	
		// Locate the scan provider.
		fmt.Println("Getting scan service...")
		var scanService scanners.ScanService
		scanService = dbClient.Server.GetScanService(scanProviderName)
		if scanService == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
			"Unable to identify a scan service named '" + scanProviderName + "'")
		}
		
		// Attach to the scan provider.
		var scanContext scanners.ScanContext
		scanContext, err = scanService.CreateScanContext(params)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		var imageName string
		imageName, err = dockerImageVersion.getFullName(dbClient)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		var result *scanners.ScanResult
		fmt.Println("Contacting scan service...")
		
		// Perform scan.
		result, err = scanContext.ScanImage(imageName)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		fmt.Println("Scanner service completed")
		
		// Compute score.
		// TBD: Here we should use the scanConfig.SuccessExpression to compute the score.
		var score string
		score = fmt.Sprintf("%d", len(result.Vulnerabilities))
	
		// Construct arrays of param names and values, needed by dbCreateScanEvent.
		var paramNames = make([]string, len(params))
		var paramValues = make([]string, len(paramNames))
		var i = 0
		for name, value := range params {
			paramNames[i] = name
			paramValues[i] = value
			i++
		}
		
		// Create a scan event.
		var userId string = sessionToken.AuthenticatedUserid
		var user User
		user, err = dbClient.dbGetUserByUserId(userId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if user == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
			"User with Id " + userId + " not found") }
		var scanEvent ScanEvent
		scanEvent, err = dbClient.dbCreateScanEvent(scanConfig.getId(), scanConfig.getProviderName(),
			paramNames, paramValues, dockerImageVersionId, user.getId(), score, result)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		
		scanEventDescs = append(scanEventDescs, scanEvent.asScanEventDesc(dbClient))
	}
	
	return scanEventDescs
}

/*******************************************************************************
 * Arguments: ImageObjId
 * Returns: ScanEventDesc (has empty fields, if there are no events)
 */
func getDockerImageStatus(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var imageObjId string
	var err error
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Determine if ImageObjId is a DockerImage or a DockerImageVersion.
	
	var dockerImageId string = ""
	var dockerImage DockerImage
	var dockerImageVersionId string = ""
	var dockerImageVersion DockerImageVersion = nil
	dockerImage, err = dbClient.getDockerImage(imageObjId)
	if err == nil {
		dockerImageId = imageObjId
		// Get the most recent version.
		dockerImageVersionId = dockerImage.getMostRecentVersionId()
		dockerImageVersion, err = dbClient.getDockerImageVersion(dockerImageVersionId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Could not identify DockerImageVersion with object Id " + dockerImageVersionId)
		}
	} else {
		dockerImageVersion, err = dbClient.getDockerImageVersion(imageObjId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
			"The ImageObjId is not a DockerImage or a DockerImageVersion")
		}
		// User specified a DockerImageVersion.
		dockerImageVersionId = imageObjId
		// Identify the DockerImage, in order to check the user's authorization.
		dockerImage, err = dbClient.getDockerImage(dockerImageVersion.getImageObjId())
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Could not identify Docker Image for the DockerImageVersion")
		}
	}
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, dockerImageId,
		"getDockerImageStatus")
	if failMsg != nil { return failMsg }
	
	// Get the most recent ScanEvent, if any.
	var eventId string = dockerImageVersion.getMostRecentScanEventId()
	if eventId == "" {
		return apitypes.NewScanEventDesc("", time.Now(), "", dockerImageVersionId,
			"", "", nil, "", nil) // an empty ScanEventDesc
	} else {
		var event ScanEvent
		event, err = dbClient.getScanEvent(eventId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		return event.asScanEventDesc(dbClient)
	}
}

/*******************************************************************************
 * Arguments: ScanConfigId
 * Returns: ScanConfigDesc
 */
func getScanConfigDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var scanConfigId string
	var err error
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var scanConfig ScanConfig
	scanConfig, err = dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, scanConfigId,
		"getScanConfigDesc")
	if failMsg != nil { return failMsg }
	
	return scanConfig.asScanConfigDesc(dbClient)
}

/*******************************************************************************
 * Arguments: FlagId
 * Returns: FlagDesc
 */
func getFlagDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var flagId string
	var err error
	flagId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "FlagId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var flag Flag
	flag, err = dbClient.getFlag(flagId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask, flagId,
		"getFlagDesc")
	if failMsg != nil { return failMsg }
	
	return flag.asFlagDesc()
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: EventDescBase...
 */
func getUserEvents(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var userId string = sessionToken.AuthenticatedUserid
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if user == nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unidentified user, " + userId) }
	var eventIds []string = user.getEventIds()
	
	var eventDescs apitypes.EventDescs = make([]apitypes.EventDesc, 0)
	for _, eventId := range eventIds {
		var event Event
		var err error
		event, err = dbClient.getEvent(eventId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		eventDescs = append(eventDescs, event.asEventDesc(dbClient))
	}
	
	return eventDescs
}

/*******************************************************************************
 * Arguments: ImageObjId
 * Returns: Array of derived types of EventDescBase.
 */
func getDockerImageEvents(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var imageObjId string
	var err error
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Determine if ImageObjId is a DockerImage or a DockerImageVersion.
	//var dockerImageId string = ""
	var dockerImage DockerImage
	var dockerImageVersionId string = ""
	var dockerImageVersion DockerImageVersion = nil
	dockerImage, err = dbClient.getDockerImage(imageObjId)
	if err == nil {
		//dockerImageId = imageObjId
	} else {
		dockerImageVersion, err = dbClient.getDockerImageVersion(imageObjId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusBadRequest,
			"The ImageObjId is not a DockerImage or a DockerImageVersion")
		}
		// User specified a DockerImageVersion.
		dockerImageVersionId = imageObjId
		// Identify the DockerImage, in order to check the user's authorization.
		dockerImage, err = dbClient.getDockerImage(dockerImageVersion.getImageObjId())
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Could not identify Docker Image for the DockerImageVersion")
		}
	}
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask,
		dockerImage.getId(), "getDockerImageEvents")
	if failMsg != nil { return failMsg }
	
	var eventDescs apitypes.EventDescs = make([]apitypes.EventDesc, 0)
	var dockerImageVersionIds []string
	if dockerImageVersion == nil {
		dockerImageVersionIds = dockerImage.getImageVersionIds()
	} else {
		dockerImageVersionIds = []string{ dockerImageVersionId }
	}
	
	for _, versionId := range dockerImageVersionIds {
		dockerImageVersion, err  = dbClient.getDockerImageVersion(versionId)
		if err != nil { return apitypes.NewFailureDesc(http.StatusInternalServerError,
			"Cound not identify DockerImageVersion with object Id " + versionId)
		}
		var eventIds []string = dockerImageVersion.getScanEventIds()
		for _, eventId := range eventIds {
			var event Event
			event, err = dbClient.getEvent(eventId)
			if err != nil { return apitypes.NewFailureDescFromError(err) }
			eventDescs = append(eventDescs, event.asEventDesc(dbClient))
		}
		var eventId string = dockerImageVersion.getImageCreationEventId()
		var event Event
		event, err = dbClient.getEvent(eventId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		eventDescs = append(eventDescs, event.asEventDesc(dbClient))
	}
	
	return eventDescs
}

/*******************************************************************************
 * Arguments: DockerfileId
 * Returns: DockerfileExecEventDesc...
 */
func getDockerfileEvents(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var dockerfileId string
	var err error
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask,
		dockerfileId, "getDockerfileEvents")
	if failMsg != nil { return failMsg }
	
	var dockerfile Dockerfile
	dockerfile, err = dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var eventIds []string = dockerfile.getDockerfileExecEventIds()
	var eventDescs apitypes.EventDescs = make([]apitypes.EventDesc, 0)
	for _, eventId := range eventIds {
		var event Event
		event, err = dbClient.getEvent(eventId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		eventDescs = append(eventDescs, event.asEventDesc(dbClient))
	}
	
	return eventDescs
}

/*******************************************************************************
 * Arguments: RepoId, ScanConfigName
 * Returns: ScanConfigDesc
 */
func getScanConfigDescByName(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var configName string
	configName, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ScanConfigName")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask,
		repoId, "getScanConfigDescByName")
	if failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	for _, scanConfigId := range repo.getScanConfigIds() {
		var scanConfig ScanConfig
		scanConfig, err = dbClient.getScanConfig(scanConfigId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if scanConfig.getName() == configName {
			return scanConfig.asScanConfigDesc(dbClient)
		}
	}
	
	return apitypes.NewFailureDesc(http.StatusBadRequest, "ScanConfig with name " + configName +
		" not found in repo " + repo.getName())
}

/*******************************************************************************
 * Arguments: ScanConfigId
 * Returns: Result
 */
func remScanConfig(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var scanConfigId string
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.DeleteMask,
		scanConfigId, "remScanConfig")
	if failMsg != nil { return failMsg }
	
	var scanConfig ScanConfig
	scanConfig, err = dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var repo Repo
	repo, err = dbClient.getRepo(scanConfig.getRepoId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
		
	err = repo.deleteScanConfig(dbClient, scanConfig)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return apitypes.NewResult(200, "Scan config removed")
}

/*******************************************************************************
 * Arguments: RepoId, FlagName
 * Returns: FlagDesc
 */
func getFlagDescByName(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "RepoId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var flagName string
	flagName, err = apitypes.GetRequiredHTTPParameterValue(true, values, "FlagName")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask,
		repoId, "getFlagDescByName")
	if failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	for _, flagId := range repo.getFlagIds() {
		var flag Flag
		flag, err = dbClient.getFlag(flagId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		if flag.getName() == flagName {
			return flag.asFlagDesc()
		}
	}
	
	return apitypes.NewFailureDesc(http.StatusBadRequest, "Flag with name " + flagName +
		" not found in repo " + repo.getName())
}

/*******************************************************************************
 * Arguments: EventId
 * Returns: EventDesc
 */
func getEventDesc(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var eventId string
	eventId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "EventId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Identify the resource to which the event applies.
	var event Event
	event, err = dbClient.getEvent(eventId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var isType bool
	var scanEvent ScanEvent
	scanEvent, isType = event.(ScanEvent)
	var objId string
	if isType {
		objId = scanEvent.getScanConfigId()
	} else {
		var imageCreationEvent ImageCreationEvent
		imageCreationEvent, isType = event.(ImageCreationEvent)
		if isType {
			var imageVersion ImageVersion
			if imageCreationEvent.getImageVersionId() != "" {
				imageVersion, err = dbClient.getImageVersion(imageCreationEvent.getImageVersionId())
				if err != nil { return apitypes.NewFailureDescFromError(err) }
				objId = imageVersion.getImageObjId()
			}
		} else {
			return apitypes.NewFailureDesc(http.StatusInternalServerError,
				"Unexpected Event type: " + reflect.TypeOf(event).String())
		}
	}
	
	if objId != "" {
		failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask,
			objId, "getEventDesc")
		if failMsg != nil { return failMsg }
	}
	
	return dbClient.asEventDesc(event)
}

/*******************************************************************************
 * Arguments: FlagId
 * Returns: Result
 */
func remFlag(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var flagId string
	flagId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "FlagId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.DeleteMask,
		flagId, "remFlag")
	if failMsg != nil { return failMsg }
	
	var flag Flag
	flag, err = dbClient.getFlag(flagId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var repo Repo
	repo, err = dbClient.getRepo(flag.getRepoId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	err = repo.deleteFlag(dbClient, flag)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return apitypes.NewResult(200, "Flag removed")
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: Result
 */
func remDockerImage(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var imageId string
	imageId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ImageId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.DeleteMask,
		imageId, "remDockerImage")
	if failMsg != nil { return failMsg }
	
	var image DockerImage
	image, err = dbClient.getDockerImage(imageId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var repo Repo
	repo, err = dbClient.getRepo(image.getRepoId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if repo == nil { return apitypes.NewFailureDesc(
		http.StatusInternalServerError, "Internal error - repo is nil") }
	
	err = repo.deleteDockerImage(dbClient, image)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return apitypes.NewResult(200, "Docker image removed")
}

/*******************************************************************************
 * Arguments: DockerfileId
 * Returns: Result
 */
func remDockerfile(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var dockerfileId string
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.DeleteMask,
		dockerfileId, "remDockerfile")
	if failMsg != nil { return failMsg }
	
	// Identify the Dockerfile.
	var dockerfile Dockerfile
	dockerfile, err = dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Remove the Dockerfile's ACLEntries.
	err = dbClient.deleteAllAccessToResource(dockerfile)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Nullify the Dockerfile in each of its DockerfileExecEvents.
	var execEventIds []string = dockerfile.getDockerfileExecEventIds()
	for _, id := range execEventIds {
		var event DockerfileExecEvent
		event, err = dbClient.getDockerfileExecEvent(id)
		if err != nil {
			fmt.Println("While retrieving DockerfileExecEvent with Id " + id)
			fmt.Println(err.Error())
			continue
		}
		err = event.nullifyDockerfile(dbClient)
		if err != nil {
			fmt.Println("While calling nullifyDockerfile with DockerfileExecEvent Id " + id)
			fmt.Println(err.Error())
			continue
		}
	}
	
	// Remove the Dockerfile from its Repo.
	var repo Repo
	repo, err = dbClient.getRepo(dockerfile.getRepoId())
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	err = repo.deleteDockerfile(dbClient, dockerfile)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	return apitypes.NewResult(200, "Dockerfile removed")
}

/*******************************************************************************
 * Arguments: ImageVersionId
 * Returns: Result
 */
func remImageVersion(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var imageVersionId string
	imageVersionId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ImageVersionId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var imageId string
	var imageVersion ImageVersion
	imageVersion, err = dbClient.getImageVersion(imageVersionId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	imageId = imageVersion.getImageObjId()
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.DeleteMask,
		imageId, "remImageVersion")
	if failMsg != nil { return failMsg }
	
	// Nullify the ImageVersion Id in the version's ImageCreationEvent.
	var eventId = imageVersion.getImageCreationEventId()
	var imageCreationEvent ImageCreationEvent
	imageCreationEvent, err = dbClient.getImageCreationEvent(eventId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	imageCreationEvent.nullifyImageVersion()
	
	// Nullify the ImageVersion Id in any of the version's ScanEvents.
	var isType bool
	var dockerImageVersion DockerImageVersion
	dockerImageVersion, isType = imageVersion.(DockerImageVersion)
	if isType {
		for _, scanEventId := range dockerImageVersion.getScanEventIds() {
			var scanEvent ScanEvent
			scanEvent, err = dbClient.getScanEvent(scanEventId)
			if err != nil {
				fmt.Println("While getting ScanEvent with Id " + scanEventId)
				fmt.Println(err.Error())
				continue
			}
			err = scanEvent.nullifyDockerImageVersion(dbClient)
			if err != nil {
				fmt.Println("While nullifying DockerImageVersion for ScanEvent with Id " + scanEventId)
				fmt.Println(err.Error())
				continue
			}
		}
	}
	
	// Remove the ImageVersion from the Image.
	var image Image
	image, err = dbClient.getImage(imageId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	err = image.deleteImageVersion(dbClient, imageVersion)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	return apitypes.NewResult(200, "Image version removed")
}

/*******************************************************************************
 * Arguments: DockerImageId
 * Returns: DockerImageVersionDescs
 */
func getDockerImageVersions(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var err error
	var dockerImageId string
	dockerImageId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerImageId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ReadMask,
		dockerImageId, "getDockerImageVersions")
	if failMsg != nil { return failMsg }
	
	var dockerImage DockerImage
	dockerImage, err = dbClient.getDockerImage(dockerImageId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var descs apitypes.DockerImageVersionDescs
	descs = make([]*apitypes.DockerImageVersionDesc, 0)
	var versionIds []string = dockerImage.getImageVersionIds()
	for _, versionId := range versionIds {
		var dockerImageVersion DockerImageVersion
		dockerImageVersion, err = dbClient.getDockerImageVersion(versionId)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		
		var imageVersionDesc *apitypes.DockerImageVersionDesc
		imageVersionDesc, err = dockerImageVersion.asDockerImageVersionDesc(dbClient)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		
		descs = append(descs, imageVersionDesc)
	}
	
	return descs
}

/*******************************************************************************
 * Arguments: UserInfo
 * Returns: UserDesc
 */
func updateUserInfo(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	/* UserInfo:
	UserId
	UserName - full name (e.g., "John Smith")
	EmailAddress
	Password - ignored
	RealmId
	*/

	var newUserInfo *apitypes.UserInfo
	var err error
	newUserInfo, err = apitypes.GetUserInfoChanges(values)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	var specifiedUser User
	specifiedUser, err = dbClient.dbGetUserByUserId(newUserInfo.UserId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if specifiedUser == nil { return apitypes.NewFailureDesc(http.StatusInternalServerError, "User unidentified") }
	
	// Check that the userId is for the user who is currently logged in.
	if sessionToken.AuthenticatedUserid != newUserInfo.UserId {
		return apitypes.NewFailureDesc(http.StatusForbidden,
			"Only an account owner may change their info")
	}
	
	var changesMade = false
	if newUserInfo.UserName != "" {
		specifiedUser.setNameDeferredUpdate(newUserInfo.UserName)
		changesMade = true
	}
	
	if newUserInfo.EmailAddress != "" {
		err = EstablishEmail(dbClient.getServer().authService, dbClient,
			dbClient.getServer().EmailService, specifiedUser.getId(), newUserInfo.EmailAddress)
		if err != nil { return apitypes.NewFailureDescFromError(err) }
		changesMade = true
	}
	
	if changesMade { dbClient.updateObject(specifiedUser) }
	return specifiedUser.asUserDesc(dbClient)
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: bool
 */
func userExists(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var specifiedUserId string
	var err error
	specifiedUserId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "UserId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	// Can only be called by the special user "HighTrustClient".
	//if sessionToken.AuthenticatedUserid != "HighTrustClient" {
	//	return apitypes.NewFailureDesc(http.StatusForbidden,
	//		"Only HighTrustClient may call this method")
	//}
	
	var specifiedUser User
	specifiedUser, err = dbClient.dbGetUserByUserId(specifiedUserId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if specifiedUser == nil { return apitypes.NewFailureDesc(http.StatusNotFound,
		"User with Id " + specifiedUserId)
	}
	
	return apitypes.NewResult(200, "User with Id " + specifiedUserId + " exists")
}

/*******************************************************************************
 * Arguments: AccountVerificationToken
 * Returns: Result
 */
func validateAccountVerificationToken(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var tokenString string
	var err error
	tokenString, err = apitypes.GetRequiredHTTPParameterValue(true, values, "AccountVerificationToken")
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	var emailAddress string
	var userId string
	userId, emailAddress, err = ValidateEmailToken(dbClient, 
		dbClient.getServer().authService, tokenString)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	var user User
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	err = user.flagEmailAsVerified(dbClient, emailAddress)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	return apitypes.NewResult(200, "Email address verified")
}

/*******************************************************************************
 * Arguments: VerificationEnabled ("true" or "false")
 * Returns: Result
 */
func enableEmailVerification(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if ! dbClient.Server.AllowToggleEmailVerification { return apitypes.NewFailureDesc(
		http.StatusForbidden, "Method enableEmailVerification disallowed")
	}
	
	var flag string
	var err error
	flag, err = apitypes.GetRequiredHTTPParameterValue(true, values, "VerificationEnabled")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	if flag == "true" {
		dbClient.Server.setEmailVerification(true)
		return apitypes.NewResult(200, "Email verification enabled")
	}
	if flag == "false" {
		dbClient.Server.setEmailVerification(false)
		return apitypes.NewResult(200, "Email verification disabled")
	}
	return apitypes.NewFailureDesc(http.StatusBadRequest,
		"Unrecognized value for VerificationEnabled: " + flag)
}

/*******************************************************************************
 * Arguments: DockerImageId, ScanConfigId
 * Returns: Result
 */
func useScanConfigForImage(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var dockerImageId, scanConfigId string
	var err error
	dockerImageId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerImageId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		dockerImageId, "useScanConfigForImage")
	if failMsg != nil { return failMsg }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.ExecuteMask,
		scanConfigId, "stopUsingScanConfigForImage")
	if failMsg != nil { return failMsg }
	
	var scanConfig ScanConfig
	scanConfig, err = dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	err = scanConfig.addDockerImage(dbClient, dockerImageId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	return apitypes.NewResult(200, "Scan Config added to image")
}

/*******************************************************************************
 * Arguments: DockerImageId, ScanConfigId
 * Returns: Result
 */
func stopUsingScanConfigForImage(dbClient *InMemClient, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var failMsg apitypes.RespIntfTp
	sessionToken, failMsg = authenticateSession(dbClient, sessionToken, values)
	if failMsg != nil { return failMsg }

	var dockerImageId, scanConfigId string
	var err error
	dockerImageId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "DockerImageId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	failMsg = authorizeHandlerAction(dbClient, sessionToken, apitypes.WriteMask,
		dockerImageId, "stopUsingScanConfigForImage")
	if failMsg != nil { return failMsg }
	
	var scanConfig ScanConfig
	scanConfig, err = dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	err = scanConfig.remDockerImage(dbClient, dockerImageId)
	if err != nil { return apitypes.NewFailureDescFromError(err) }

	return apitypes.NewResult(200, "Scan Config removed from image")
}
