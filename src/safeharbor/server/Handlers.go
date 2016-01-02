/*******************************************************************************
 * All of the REST handlers are contained here. These functions are called by
 * the handleRequest method in Dispatcher.go.
 */

package server

import (
	"net/url"
	"mime/multipart"
	"fmt"
	"os/exec"
	"strings"
	"reflect"
	"time"
	
	// Our packages:
	"safeharbor/providers"
	"safeharbor/apitypes"
)

/*******************************************************************************
 * Arguments: (none)
 * Returns: apitypes.Result
 */
func ping(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	fmt.Println("ping request received")
	return &apitypes.Result{
		Status: 200,
		Message: "Server is up",
	}
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: apitypes.Result
 */
func clearAll(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	fmt.Println("clearAll")
	
	if ! server.Debug {
		return apitypes.NewFailureDesc("Not in debug mode - returning from clearAll")
	}
	
	// Kill all docker containers:
	// docker kill $(docker ps -a -q)
	// docker rm $(docker ps -a -q)
	var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "ps", "-a", "-q")
	var output []byte
	output, _ = cmd.CombinedOutput()
	var containers string = string(output)
	if strings.HasPrefix(containers, "Error") {
		return apitypes.NewFailureDesc(containers)
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
	var dbClient DBClient = server.dbClient
	for _, realmId := range dbClient.dbGetAllRealmIds() {
		var realm Realm
		var err error
		realm, err = dbClient.getRealm(realmId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		fmt.Println("For realm " + realm.getName() + ":")
		
		for _, repoId := range realm.getRepoIds() {
			var repo Repo
			repo, err = dbClient.getRepo(repoId)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			fmt.Println("\tFor repo " + repo.getName() + ":")
			
			for _, imageId := range repo.getDockerImageIds() {
				
				var image DockerImage
				image, err = dbClient.getDockerImage(imageId)
				if err != nil { return apitypes.NewFailureDesc(err.Error()) }
				var imageName string = image.getName()
				fmt.Println("\t\tRemoving image " + imageName + ":")
				
				// Remove the image.
				cmd = exec.Command("/usr/bin/docker", "rmi", imageName)
				output, _ = cmd.CombinedOutput()
				outputStr = string(output)
				if ! strings.HasPrefix(outputStr, "Error") {
					fmt.Println("While removing image " + imageName + ": " + outputStr)
				} else {
					fmt.Println("\t\tRemoved image", imageName)
				}
			}
		}
	}
	
	// Remove and re-create the repository directory.
	fmt.Println("Initializing database...")
	server.dbClient.init()
	
	// Clear all session state.
	server.authService.clearAllSessions()
	
	return apitypes.NewResult(200, "Persistent state reset")
}

/*******************************************************************************
 * Arguments: apitypes.Credentials
 * Returns: apitypes.SessionToken
 */
func printDatabase(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	fmt.Println("printDatabase")
	
	if ! server.Debug {
		return apitypes.NewFailureDesc("Not in debug mode - returning from printDatabase")
	}
	
	server.dbClient.printDatabase()
	
	return apitypes.NewResult(200, "Database printed to stdout on server.")
}

/*******************************************************************************
 * Arguments: apitypes.Credentials
 * Returns: apitypes.SessionToken
 */
func authenticate(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	var creds *apitypes.Credentials
	var err error
	creds, err = apitypes.GetCredentials(values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Verify credentials.
	var user User = server.dbClient.dbGetUserByUserId(creds.UserId)
	if user == nil {
		return apitypes.NewFailureDesc("User not found in the database")
	}
	
	if ! user.isActive() { return apitypes.NewFailureDesc("User is not active") }
	
	// Check against brute force password attack.
	var attempts []string = user.getMostRecentLoginAttempts()
	if len(attempts) >= server.MaxLoginAttemptsToRetain &&
		apitypes.AreAllWithinTimePeriod(attempts, 600) {
		server.LoginAlert(creds.UserId)
		return apitypes.NewFailureDesc("Too many login attempts")
	}
	user.addLoginAttempt()
	
	// Verify password.
	if ! user.validatePassword(creds.Password) {
		return apitypes.NewFailureDesc("Invalid password")
	}
	
	// Create new user session.
	var newSessionToken *apitypes.SessionToken = server.authService.createSession(creds)
	newSessionToken.SetRealmId(user.getRealmId())
	
	// If the user had a prior session, invalidate it.
	if sessionToken != nil {
		server.authService.invalidateSessionId(sessionToken.UniqueSessionId)
	}
	
	// Flag whether the user has Write access to the realm.
	var realm Realm
	realm, err = server.dbClient.getRealm(user.getRealmId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var entry ACLEntry
	entry, err = realm.getACLEntryForPartyId(user.getId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if entry != nil {
		newSessionToken.SetIsAdminUser(entry.getPermissionMask()[apitypes.CanWrite])
	}
	
	return newSessionToken
}

/*******************************************************************************
 * Arguments: none
 * Returns: apitypes.Result
 */
func logout(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	server.authService.invalidateSessionId(sessionToken.UniqueSessionId)
	return apitypes.NewResult(200, "Logged out")
}

/*******************************************************************************
 * Arguments: apitypes.UserInfo
 * Returns: apitypes.UserDesc
 */
func createUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var err error
	var userInfo *apitypes.UserInfo
	userInfo, err = apitypes.GetUserInfo(values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.CreateInMask, userInfo.RealmId,
		"createUser"); failMsg != nil { return failMsg }
	
	// Legacy - uses Cesanta. Probably remove this.
//	if ! server.authService.authorized(server.sessions[sessionToken.UniqueSessionId],
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
	newUser, err = server.dbClient.dbCreateUser(newUserId, newUserName, email, pswd, realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return newUser.asUserDesc()
}

/*******************************************************************************
 * Arguments: UserObjId
 * Returns: apitypes.Result
 */
func disableUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var userObjId string
	var err error
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "UserObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var user User
	user, err = server.dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask,
		user.getRealmId(), "disableUser"); failMsg != nil { return failMsg }
	
	// Prevent the user from authenticating.
	user.setActive(false)
	
	return apitypes.NewResult(200, "User with user Id '" + user.getUserId() + "' disabled")
}

/*******************************************************************************
 * Arguments: UserObjId
 * Returns: apitypes.Result
 */
func reenableUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	var userObjId string
	var err error
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "UserObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var user User
	user, err = server.dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask,
		user.getRealmId(), "reenableUser"); failMsg != nil { return failMsg }
	
	if user.isActive() { return apitypes.NewFailureDesc("User " + user.getUserId() + " is already active") }
	
	// Enable the user to authenticate.
	user.setActive(true)
	
	return apitypes.NewResult(200, "User with user Id '" + user.getUserId() + "' reenabled")
}

/*******************************************************************************
 * Arguments: UserId, OldPassword, NewPassword
 * Returns: apitypes.Result
 */
func changePassword(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var userId string
	var err error
	userId, err = apitypes.GetRequiredHTTPParameterValue(values, "UserId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Check that the userId is for the user who is currently logged in.
	if sessionToken.AuthenticatedUserid != userId {
		return apitypes.NewFailureDesc("Only an account owner may change their own password")
	}
	
	var user User
	user = server.dbClient.dbGetUserByUserId(userId)
	if user == nil { return apitypes.NewFailureDesc("User unidentified") }
	
	var oldPswd string
	oldPswd, err = apitypes.GetRequiredHTTPParameterValue(values, "OldPassword")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if ! user.validatePassword(oldPswd) {
		return apitypes.NewFailureDesc("Invalid password")
	}
	
	var newPswd string
	newPswd, err = apitypes.GetRequiredHTTPParameterValue(values, "NewPassword")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask,
		user.getId(), "changePassword"); failMsg != nil { return failMsg }
	
	err = user.setPassword(newPswd)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewResult(200, "Password changed")
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: apitypes.GroupDesc
 */
func createGroup(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var groupName string
	groupName, err = apitypes.GetRequiredHTTPParameterValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var groupDescription string
	groupDescription, err = apitypes.GetRequiredHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var addMeStr string
	addMeStr, err = apitypes.GetHTTPParameterValue(values, "AddMe")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var addMe bool = false
	if addMeStr == "true" { addMe = true }
	fmt.Println(fmt.Sprintf("AddMe=%s", addMeStr))

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.CreateInMask, realmId,
		"createGroup"); failMsg != nil { return failMsg }
	
	var group Group
	group, err = server.dbClient.dbCreateGroup(realmId, groupName, groupDescription)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if addMe {
		var userId string = sessionToken.AuthenticatedUserid
		var user User = server.dbClient.dbGetUserByUserId(userId)
		err = group.addUserId(user.getId())
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	}
	
	return group.asGroupDesc()
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: apitypes.Result
 */
func deleteGroup(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var groupId string
	var err error
	groupId, err = apitypes.GetRequiredHTTPParameterValue(values, "GroupId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var group Group
	group, err = server.dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask,
		group.getRealmId(), "deleteGroup"); failMsg != nil { return failMsg }
	
	var groupName string = group.getName()
	var realm Realm
	realm, err = group.getRealm()
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	err = realm.deleteGroup(group)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return apitypes.NewResult(200, "Group '" + groupName + "' deleted")
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: []*apitypes.UserDesc
 */
func getGroupUsers(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredHTTPParameterValue(values, "GroupId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, groupId,
		"getGroupUsers"); failMsg != nil { return failMsg }
	
	var group Group
	group, err = server.dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if group == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
		"No group with Id %s", groupId))
	}
	var userObjIds []string = group.getUserObjIds()
	var userDescs apitypes.UserDescs = make([]*apitypes.UserDesc, 0)
	for _, id := range userObjIds {
		var user User
		user, err = server.dbClient.getUser(id)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if user == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
			"Internal error: No user with Id %s", id))
		}
		var userDesc *apitypes.UserDesc = user.asUserDesc()
		userDescs = append(userDescs, userDesc)
	}
	
	return userDescs
}

/*******************************************************************************
 * Arguments: GroupId, UserObjId
 * Returns: apitypes.Result
 */
func addGroupUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredHTTPParameterValue(values, "GroupId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var userObjId string
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "UserObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, groupId,
		"addGroupUser"); failMsg != nil { return failMsg }
	
	var group Group
	group, err = server.dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if group == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
		"No group with Id %s", groupId))
	}

	err = group.addUserId(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var user User
	user, err = server.dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if user == nil { return apitypes.NewFailureDesc("User with Id " + userObjId + " unidentified") }
	user.addGroupId(groupId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return &apitypes.Result{
		Status: 200,
		Message: "User added to group",
	}
}

/*******************************************************************************
 * Arguments: GroupId, UserObjId
 * Returns: apitypes.Result
 */
func remGroupUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredHTTPParameterValue(values, "GroupId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var userObjId string
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "UserObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, groupId,
		"remGroupUser"); failMsg != nil { return failMsg }
	
	var group Group
	group, err = server.dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var user User
	user, err = server.dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	err = group.removeUser(user)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewResult(200, "User " + user.getName() + " removed from group " + group.getName())
}

/*******************************************************************************
 * Arguments: apitypes.RealmInfo, apitypes.UserInfo
 * Returns: apitypes.UserDesc
 */
func createRealmAnon(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	// Create new administrative user.
	var err error
	var userInfo *apitypes.UserInfo
	userInfo, err = apitypes.GetUserInfo(values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var newUserId string = userInfo.UserId
	var newUserName string = userInfo.UserName
	//var realmId string = userInfo.RealmId  // ignored
	var email string = userInfo.EmailAddress
	var pswd string = userInfo.Password

	var dbClient DBClient = server.dbClient
	
	// Create a realm.
	var newRealmInfo *apitypes.RealmInfo
	newRealmInfo, err = apitypes.GetRealmInfo(values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	fmt.Println("Creating realm ", newRealmInfo.RealmName)
	var newRealm Realm
	newRealm, err = server.dbClient.dbCreateRealm(newRealmInfo, newUserId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	fmt.Println("Created realm", newRealmInfo.RealmName)
	
	// Create a user
	var newUser User
	newUser, err = dbClient.dbCreateUser(newUserId, newUserName, email, pswd, newRealm.getId())
	if err != nil { return apitypes.NewFailureDesc("Unable to create user: " + err.Error()) }

	// Add ACL entry to enable the current user (if any) to access what he/she just created.
	var curUser User
	var sessionError error
	curUser, sessionError = getCurrentUser(server, sessionToken)
	if (curUser != nil) && (sessionError == nil) {
		_, err = server.dbClient.dbCreateACLEntry(newRealm.getId(), curUser.getId(),
			[]bool{ true, true, true, true, true } )
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	}

	// Add ACL entry to enable the new user to access what was just created.
	_, err = server.dbClient.dbCreateACLEntry(newRealm.getId(), newUser.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if sessionError != nil { return apitypes.NewFailureDesc(sessionError.Error()) }
	return newUser.asUserDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: apitypes.RealmDesc
 */
func getRealmDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, realmId,
		"getRealmDesc"); failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = server.dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if realm == nil { return apitypes.NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmInfo
 * Returns: apitypes.RealmDesc
 */
func createRealm(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var realmInfo *apitypes.RealmInfo
	realmInfo, err = apitypes.GetRealmInfo(values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	fmt.Println("Creating realm ", realmInfo.RealmName)
	var realm Realm
	realm, err = server.dbClient.dbCreateRealm(realmInfo, user.getId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	fmt.Println("Created realm", realmInfo.RealmName)

	// Add ACL entry to enable the current user to access what he/she just created.
	_, err = server.dbClient.dbCreateACLEntry(realm.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: apitypes.Result
 */
func deactivateRealm(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.DeleteMask, realmId,
		"deactivateRealm"); failMsg != nil { return failMsg }
	
	err = server.dbClient.dbDeactivateRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewResult(200, "Realm deactivated")
}

/*******************************************************************************
 * Arguments: UserObjId, RealmId
 * Returns: apitypes.Result
 */
func moveUserToRealm(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var realmId string
	var userObjId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	userObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "UserObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, realmId,
		"moveUserToRealm"); failMsg != nil { return failMsg }
	
	var destRealm Realm
	destRealm, err = server.dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if destRealm == nil { return apitypes.NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	var user User
	user, err = server.dbClient.getUser(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var origRealm Realm
	origRealm, err = server.dbClient.getRealm(user.getRealmId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if origRealm.getId() == destRealm.getId() {
		return apitypes.NewFailureDesc("The user is already in the destination realm")
	}
	
	_, err = origRealm.removeUserId(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	err = destRealm.addUserId(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewResult(200, "User moved to realm")
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: apitypes.UserDesc
 */
func getUserDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var userId string
	userId, err = apitypes.GetRequiredHTTPParameterValue(values, "UserId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var user User
	user = server.dbClient.dbGetUserByUserId(userId)
	if user == nil { return apitypes.NewFailureDesc("User with user id " + userId +
		" not found.") }
	
	var realm Realm
	realm, err = user.getRealm()
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, realm.getId(),
		"getUserDesc"); failMsg != nil { return failMsg }
	
	return user.asUserDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*apitypes.UserDesc
 */
func getRealmUsers(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, realmId,
		"getRealmUsers"); failMsg != nil { return failMsg }
		
	var realm Realm
	realm, err = server.dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if realm == nil { return apitypes.NewFailureDesc("Realm with Id " + realmId + " not found") }
	var userObjIds []string = realm.getUserObjIds()
	var userDescs apitypes.UserDescs = make([]*apitypes.UserDesc, 0)
	for _, userObjId := range userObjIds {
		var user User
		user, err = server.dbClient.getUser(userObjId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if user == nil { return apitypes.NewFailureDesc("User with obj Id " + userObjId + " not found") }
		var userDesc *apitypes.UserDesc = user.asUserDesc()
		userDescs = append(userDescs, userDesc)
	}
	return userDescs
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*apitypes.GroupDesc
 */
func getRealmGroups(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var groupDescs apitypes.GroupDescs = make([]*apitypes.GroupDesc, 0)
	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, realmId,
		"getRealmGroups"); failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = server.dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if realm == nil { return apitypes.NewFailureDesc("Realm with Id " + realmId + " not found") }
	var groupIds []string = realm.getGroupIds()
	for _, groupId := range groupIds {
		var group Group
		group, err = server.dbClient.getGroup(groupId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
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
func getRealmRepos(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	fmt.Println("getRealmRepos: A")
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, realmId,
		"getRealmRepos"); failMsg != nil { return failMsg }
	fmt.Println("getRealmRepos: B")
	
	var realm Realm
	realm, err = server.dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if realm == nil { return apitypes.NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	var repoIds []string = realm.getRepoIds()
	
	var result apitypes.RepoDescs = make([]*apitypes.RepoDesc, 0)
	for _, id := range repoIds {
		
		var repo Repo
		repo, err = server.dbClient.getRepo(id)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if repo == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
			"Internal error: no Repo found for Id %s", id))
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
func getAllRealms(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var realmIds []string = server.dbClient.dbGetAllRealmIds()
	
	var result apitypes.RealmDescs = make([]*apitypes.RealmDesc, 0)
	for _, realmId := range realmIds {
		
		var realm Realm
		var err error
		realm, err = server.dbClient.getRealm(realmId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if realm == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
			"Internal error: no Realm found for Id %s", realmId))
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
func createRepo(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	fmt.Println("Creating repo...")
	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredHTTPParameterValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var repoName string
	repoName, err = apitypes.GetRequiredHTTPParameterValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var repoDesc string
	repoDesc, err = apitypes.GetRequiredHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.CreateInMask, realmId,
		"createRepo"); failMsg != nil { return failMsg }
	
	fmt.Println("Creating repo", repoName)
	var repo Repo
	repo, err = server.dbClient.dbCreateRepo(realmId, repoName, repoDesc)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	// Add ACL entry to enable the current user to access what he/she just created.
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	_, err = server.dbClient.dbCreateACLEntry(repo.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var name string
	var filepath string
	name, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if filepath != "" { // a file was attached - presume that it is a dockerfile
		_, err = createDockerfile(sessionToken, server.dbClient, repo,
			name, filepath, repo.getDescription())
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	}
	
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: apitypes.Result
 */
func deleteRepo(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	return apitypes.NewFailureDesc("Not implemented yet: deleteRepo")
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []*apitypes.DockerfileDesc
 */
func getDockerfiles(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, repoId,
		"getDockerfiles"); failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if repo == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
		"Repo with Id %s not found", repoId)) }
	
	var dockerfileIds []string = repo.getDockerfileIds()	
	var result apitypes.DockerfileDescs = make([]*apitypes.DockerfileDesc, 0)
	for _, id := range dockerfileIds {
		
		var dockerfile Dockerfile
		dockerfile, err = server.dbClient.getDockerfile(id)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if dockerfile == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
			"Internal error: no Dockerfile found for Id %s", id))
		}
		var desc *apitypes.DockerfileDesc = dockerfile.asDockerfileDesc()
		// Add to result
		result = append(result, desc)
	}

	return result
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []*apitypes.DockerImageDesc
 */
func getImages(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, repoId,
		"getImages"); failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if repo == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
		"Repo with Id %s not found", repoId)) }
	
	var imageIds []string = repo.getDockerImageIds()
	var result apitypes.DockerImageDescs = make([]*apitypes.DockerImageDesc, 0)
	for _, id := range imageIds {
		
		var dockerImage DockerImage
		dockerImage, err = server.dbClient.getDockerImage(id)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if dockerImage == nil { return apitypes.NewFailureDesc(fmt.Sprintf(
			"Internal error: no DockerImage found for Id %s", id))
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
func addDockerfile(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Identify the repo.
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var desc string
	desc, err = apitypes.GetRequiredHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.CreateInMask, repoId,
		"addDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if repo == nil { return apitypes.NewFailureDesc("Repo does not exist") }
	
	var name string
	var filepath string
	name, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if filepath == "" { return apitypes.NewFailureDesc("No file was found") }
	
	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, name, filepath, desc)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if dockerfile == nil { return apitypes.NewFailureDesc("No dockerfile was attached") }
	
	return dockerfile.asDockerfileDesc()
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: GroupDesc
 */
func getGroupDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Identify the group.
	var groupId string
	var err error
	groupId, err = apitypes.GetRequiredHTTPParameterValue(values, "GroupId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var group Group
	group, err = server.dbClient.getGroup(groupId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return group.asGroupDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: RepoDesc
 */
func getRepoDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Identify the repo.
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: DockerImageId
 * Returns: DockerImageDesc
 */
func getDockerImageDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Identify the repo.
	var imageId string
	var err error
	imageId, err = apitypes.GetRequiredHTTPParameterValue(values, "DockerImageId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var image DockerImage
	image, err = server.dbClient.getDockerImage(imageId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return image.asDockerImageDesc()
}

/*******************************************************************************
 * Arguments: DockerfileId
 * Returns: DockerfileDesc
 */
func getDockerfileDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Identify the dockerfile.
	var dockerfileId string
	var err error
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var dockerfile Dockerfile
	dockerfile, err = server.dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return dockerfile.asDockerfileDesc()
}

/*******************************************************************************
 * Arguments: DockerfileId, Description (optional), File
 * Returns: apitypes.Result
 */
func replaceDockerfile(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var dockerfileId string
	var desc string
	var err error
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	desc, err = apitypes.GetHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var dockerfile Dockerfile
	dockerfile, err = server.dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask,
		dockerfileId, "replaceDockerfile"); failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = dockerfile.getRepo()
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var filepath string
	_, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if filepath == "" { return apitypes.NewFailureDesc("No file was found") }
	
	err = dockerfile.replaceDockerfileFile(filepath, desc)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return apitypes.NewResult(200, "Dockerfile file replaced")
}

/*******************************************************************************
 * Arguments: DockerfileId, ImageName
 * Returns: apitypes.DockerImageDesc
 */
func execDockerfile(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	fmt.Println("Entered execDockerfile")
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Identify the Dockerfile.
	var err error
	var dockerfileId string
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if dockerfileId == "" { return apitypes.NewFailureDesc("No HTTP parameter found for DockerfileId") }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ExecuteMask, dockerfileId,
		"execDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var dockerfile Dockerfile
	dockerfile, err = dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	fmt.Println("Dockerfile name =", dockerfile.getName())
	
	var image DockerImage
	image, err = buildDockerfile(server, dockerfile, sessionToken, dbClient, values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return image.asDockerImageDesc()
}

/*******************************************************************************
 * Arguments: RepoId, Description, ImageName, <File attachment>
 * Returns: apitypes.DockerImageDesc
 */
func addAndExecDockerfile(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	fmt.Println("Entered addAndExecDockerfile")
	
	var failMsg *apitypes.FailureDesc = nil
	sessionToken, failMsg = authenticateSession(server, sessionToken, values)
	if failMsg != nil { return failMsg }
	
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, repoId,
		"addAndExecDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if repo == nil { return apitypes.NewFailureDesc("Repo does not exist") }
	
	var desc string
	desc, err = apitypes.GetRequiredHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var name string
	var filepath string
	name, filepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if filepath == "" { return apitypes.NewFailureDesc("No file was found") }
	
	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, name, filepath, desc)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if dockerfile == nil { return apitypes.NewFailureDesc("No dockerfile was attached") }
	
	var image DockerImage
	image, err = buildDockerfile(server, dockerfile, sessionToken, dbClient, values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return image.asDockerImageDesc()
}

/*******************************************************************************
 * Arguments: ImageObjId
 * Returns: file content
 */
func downloadImage(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var imageObjId string
	var err error
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var dbClient DBClient = server.dbClient
	var resource Resource
	resource, err = dbClient.getResource(imageObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if resource == nil { return apitypes.NewFailureDesc("Unable to identify resource with Id " + imageObjId) }
	var image Image
	var isType bool
	image, isType = resource.(Image)
	if ! isType { return apitypes.NewFailureDesc("Resource with Id " + imageObjId + " is not an Image") }
	var dockerImage DockerImage
	dockerImage, isType = image.(DockerImage)
	if ! isType { return apitypes.NewFailureDesc("Image is not a docker image") }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, imageObjId,
		"downloadImage"); failMsg != nil { return failMsg }

	var tempFilePath string
	tempFilePath, err = server.DockerService.SaveImage(dockerImage)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewFileResponse(200, tempFilePath, true)
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func setPermission(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Get the mask that we will use to overwrite the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(values, "PartyId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var resourceId string
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(values, "ResourceId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var smask []string = make([]string, 5)
	smask[0], err = apitypes.GetRequiredHTTPParameterValue(values, "CanCreateIn")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[1], err = apitypes.GetRequiredHTTPParameterValue(values, "CanRead")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[2], err = apitypes.GetRequiredHTTPParameterValue(values, "CanWrite")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[3], err = apitypes.GetRequiredHTTPParameterValue(values, "CanExecute")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[4], err = apitypes.GetRequiredHTTPParameterValue(values, "CanDelete")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var mask []bool
	mask, err = apitypes.ToBoolAr(smask)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, resourceId,
		"setPermission"); failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var dbClient DBClient = server.dbClient
	var resource Resource
	resource, err = dbClient.getResource(resourceId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if resource == nil { return apitypes.NewFailureDesc("Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if party == nil { return apitypes.NewFailureDesc("Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	var aclEntry ACLEntry
	aclEntry, err = resource.setAccess(party, mask)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return aclEntry.asPermissionDesc()
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func addPermission(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Get the mask that we will be adding to the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(values, "PartyId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var resourceId string
	var smask []string = make([]string, 5)
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(values, "ResourceId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[0], err = apitypes.GetRequiredHTTPParameterValue(values, "CanCreateIn")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[1], err = apitypes.GetRequiredHTTPParameterValue(values, "CanRead")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[2], err = apitypes.GetRequiredHTTPParameterValue(values, "CanWrite")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[3], err = apitypes.GetRequiredHTTPParameterValue(values, "CanExecute")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[4], err = apitypes.GetRequiredHTTPParameterValue(values, "CanDelete")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var mask []bool
	mask, err = apitypes.ToBoolAr(smask)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, resourceId,
		"addPermission"); failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var dbClient DBClient = server.dbClient
	var resource Resource
	resource, err = dbClient.getResource(resourceId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if resource == nil { return apitypes.NewFailureDesc("Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if party == nil { return apitypes.NewFailureDesc("Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	var aclEntry ACLEntry
	aclEntry, err = resource.addAccess(party, mask)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return aclEntry.asPermissionDesc()
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: Result
 */
func remPermission(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	// Get the mask that we will be subracting from the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(values, "PartyId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var resourceId string
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(values, "ResourceId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, resourceId,
		"remPermission"); failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var dbClient DBClient = server.dbClient
	var resource Resource
	resource, err = dbClient.getResource(resourceId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if resource == nil { return apitypes.NewFailureDesc(
		"Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if party == nil { return apitypes.NewFailureDesc("Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	err = resource.deleteAccess(party)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewResult(200, "All permission removed")
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId
 * Returns: PermissionDesc
 */
func getPermission(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredHTTPParameterValue(values, "PartyId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var resourceId string
	resourceId, err = apitypes.GetRequiredHTTPParameterValue(values, "ResourceId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, resourceId,
		"getPermission"); failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var dbClient DBClient = server.dbClient
	//var resource Resource = dbClient.getResource(resourceId)
	//if resource == nil { return apitypes.NewFailureDesc("Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party
	party, err = dbClient.getParty(partyId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if party == nil { return apitypes.NewFailureDesc("Unable to identify party with Id " + partyId) }
	
	// Return the ACLEntry.
	var aclEntry ACLEntry
	aclEntry, err = party.getACLEntryForResourceId(resourceId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
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
func getMyDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	if sessionToken == nil { return apitypes.NewFailureDesc("User not authenticated") }
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return apitypes.NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	return user.asUserDesc()
}

/*******************************************************************************
 * Arguments: 
 * Returns: []*apitypes.GroupDesc
 */
func getMyGroups(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var groupDescs apitypes.GroupDescs = make([]*apitypes.GroupDesc, 0)
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	var groupIds []string = user.getGroupIds()
	for _, groupId := range groupIds {
		var group Group
		var err error
		group, err = server.dbClient.getGroup(groupId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
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
func getMyRealms(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var realms map[string]Realm = make(map[string]Realm)
	
	var dbClient DBClient = server.dbClient
	var user User = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	var aclEntrieIds []string = user.getACLEntryIds()
	fmt.Println("For each acl entry...")
	for _, aclEntryId := range aclEntrieIds {
		fmt.Println("\taclEntryId:", aclEntryId)
		var err error
		var aclEntry ACLEntry
		aclEntry, err = dbClient.getACLEntry(aclEntryId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		var resourceId string = aclEntry.getResourceId()
		var resource Resource
		resource, err = dbClient.getResource(resourceId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
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
func getMyRepos(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	// Traverse the user's ACL entries; form the union of the repos that the user
	// has explicit access to, and the repos that belong to the realms that the user
	// has access to.
	
	var realms map[string]Realm = make(map[string]Realm)
	var repos map[string]Repo = make(map[string]Repo)
	
	var dbClient DBClient = server.dbClient
	var user User = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	var aclEntrieIds []string = user.getACLEntryIds()
	fmt.Println("For each acl entry...")
	for _, aclEntryId := range aclEntrieIds {
		fmt.Println("\taclEntryId:", aclEntryId)
		var err error
		var aclEntry ACLEntry
		aclEntry, err = dbClient.getACLEntry(aclEntryId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		var resourceId string = aclEntry.getResourceId()
		var resource Resource
		resource, err = dbClient.getResource(resourceId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		switch v := resource.(type) {
			case Realm: realms[v.getId()] = v
				fmt.Println("\t\ta Realm")
			case Repo: repos[v.getId()] = v
				fmt.Println("\t\ta Repo")
		}
	}
	fmt.Println("For each realm...")
	for _, realm := range realms {
		fmt.Println("For each repo of realm id", realm.getId(), "...")
		// Add all of the repos belonging to realm.
		for _, repoId := range realm.getRepoIds() {
			fmt.Println("\tadding repoId", repoId)
			var r Repo
			var err error
			r, err = dbClient.getRepo(repoId)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			if r == nil { return apitypes.NewFailureDesc("Internal error: No repo found for Id " + repoId) }
			repos[repoId] = r
		}
	}
	fmt.Println("Creating result...")
	var repoDescs apitypes.RepoDescs = make([]*apitypes.RepoDesc, 0)
	for _, repo := range repos {
		fmt.Println("\tappending repo", repo.getName())
		repoDescs = append(repoDescs, repo.asRepoDesc())
	}
	
	return repoDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: DockerfileDescs
 */
func getMyDockerfiles(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	
	var err error
	var leaves map[string]Resource
	leaves, err = getLeafResources(server.dbClient, user, ADockerfile)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	fmt.Println("Creating result...")
	var dockerfileDescs apitypes.DockerfileDescs = make([]*apitypes.DockerfileDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending dockerfile", leaf.getName())
		var dockerfile Dockerfile
		var isType bool
		dockerfile, isType = leaf.(Dockerfile)
		if ! isType { return apitypes.NewFailureDesc("Internal error: type of resource is unexpected") }
		dockerfileDescs = append(dockerfileDescs, dockerfile.asDockerfileDesc())
	}
	return dockerfileDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: DockerImageDescs
 */
func getMyDockerImages(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	
	var err error
	var leaves map[string]Resource
	leaves, err = getLeafResources(server.dbClient, user, ADockerImage)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	fmt.Println("Creating result...")
	var dockerImageDescs apitypes.DockerImageDescs = make([]*apitypes.DockerImageDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending docker image", leaf.getName())
		var dockerImage DockerImage
		var isType bool
		dockerImage, isType = leaf.(DockerImage)
		if ! isType { return apitypes.NewFailureDesc("Internal error: type of resource is unexpected") }
		dockerImageDescs = append(dockerImageDescs, dockerImage.asDockerImageDesc())
	}
	return dockerImageDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: ScanConfigDesc...
 */
func getMyScanConfigs(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	
	var err error
	var leaves map[string]Resource
	leaves, err = getLeafResources(server.dbClient, user, AScanConfig)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	fmt.Println("Creating result...")
	var dcanConfigDescs apitypes.ScanConfigDescs = make([]*apitypes.ScanConfigDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending scan config", leaf.getName())
		var dcanConfig ScanConfig
		var isType bool
		dcanConfig, isType = leaf.(ScanConfig)
		if ! isType { return apitypes.NewFailureDesc("Internal error: type of resource is unexpected") }
		dcanConfigDescs = append(dcanConfigDescs, dcanConfig.asScanConfigDesc())
	}
	return dcanConfigDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: FlagDesc...
 */
func getMyFlags(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	
	var err error
	var leaves map[string]Resource
	leaves, err = getLeafResources(server.dbClient, user, AFlag)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	fmt.Println("Creating result...")
	var flagDescs apitypes.FlagDescs = make([]*apitypes.FlagDesc, 0)
	for _, leaf := range leaves {
		fmt.Println("\tappending docker image", leaf.getName())
		var flag Flag
		var isType bool
		flag, isType = leaf.(Flag)
		if ! isType { return apitypes.NewFailureDesc("Internal error: type of resource is unexpected") }
		flagDescs = append(flagDescs, flag.asFlagDesc())
	}
	return flagDescs
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: apitypes.ScanProviderDescs
 */
func getScanProviders(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var providerDescs apitypes.ScanProviderDescs = make([]*apitypes.ScanProviderDesc, 0)
	var services []providers.ScanService = server.GetScanServices()
	for _, service := range services {
		providerDescs = append(providerDescs, service.AsScanProviderDesc())
	}
	
	return providerDescs
}

/*******************************************************************************
 * Arguments: Name, Description, RepoId, ProviderName, Params..., SuccessExpression, <optional File>
 * Returns: ScanConfigDesc
 */
func defineScanConfig(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var err error
	var providerName string
	providerName, err = apitypes.GetRequiredHTTPParameterValue(values, "ProviderName")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var scanService providers.ScanService
	scanService = server.GetScanService(providerName)
	if scanService == nil { return apitypes.NewFailureDesc(
		"Unable to identify a scan service named '" + providerName + "'")
	}
	
	var name string
	name, err = apitypes.GetRequiredHTTPParameterValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var desc string
	desc, err = apitypes.GetRequiredHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, repoId,
		"defineScanConfig"); failMsg != nil { return failMsg }
	
	var successExpr string = ""
	successExpr, err = apitypes.GetHTTPParameterValue(values, "SuccessExpression")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var scanConfig ScanConfig
	var paramValueIds []string = make([]string, 0)
	scanConfig, err = server.dbClient.dbCreateScanConfig(name, desc, repoId,
		providerName, paramValueIds, successExpr, "")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Retrieve and set the provider parameters.
	for key, valueAr := range values {
		if strings.HasPrefix(key, "scan.") {
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(
				"Parameter " + key + " is ill-formatted")
			}
			var paramName string = strings.TrimPrefix(key, "scan.")
			// See if the parameter is known by the scanner.
			_, err = scanService.GetParameterDescription(paramName)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			// Create a ParameterValue and attach it to the ScanConfig.
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(
				"Value for scan parameter '" + paramName + "' is ill-formed") }
			var value string = valueAr[0]
			_, err = scanConfig.setParameterValue(paramName, value)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		}
	}
	
	// Add ACL entry to enable the current user to access what he/she just created.
	var userId string = sessionToken.AuthenticatedUserid
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil { return apitypes.NewFailureDesc(
		"Internal error - could not identify user after use has been authenticated") }

	_, err = server.dbClient.dbCreateACLEntry(scanConfig.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Add success image, if one was attached.
	var imageFilepath string
	var repo Repo
	repo, err = server.dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	_, imageFilepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if imageFilepath != "" { // a file was attached - presume that it is an image
		fmt.Println("file attached...")
		var flag Flag
		flag, err = server.dbClient.dbCreateFlag(name, desc, repoId, imageFilepath)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		err = scanConfig.setFlagId(flag.getId())
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if scanConfig.getFlagId() == "" { return apitypes.NewFailureDesc("Flag set failed") }
		// Add ACL entry.
		_, err = server.dbClient.dbCreateACLEntry(flag.getId(), user.getId(),
			[]bool{ true, true, true, true, true } )
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	}
	
	return scanConfig.asScanConfigDesc()
}

/*******************************************************************************
 * Arguments: ScanConfigId, Name, Description, ProviderName, Params..., SuccessExpression, file
 * Returns: ScanConfigDesc
 */
func updateScanConfig(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
		
	// Update only the fields that are specified and that are not empty strings.
	var err error

	var scanConfigId string
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var scanConfig ScanConfig
	scanConfig, err = server.dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask,
		scanConfig.getRepoId(), "defineScanConfig"); failMsg != nil { return failMsg }
	
	var name string
	name, err = apitypes.GetHTTPParameterValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if name != "" { scanConfig.setNameDeferredUpdate(name) }
	
	var desc string
	desc, err = apitypes.GetHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if desc != "" { scanConfig.setDescriptionDeferredUpdate(desc) }
	
	var providerName string
	providerName, err = apitypes.GetHTTPParameterValue(values, "ProviderName")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if providerName != "" { scanConfig.setProviderNameDeferredUpdate(providerName) }
	
	var successExpr string = ""
	successExpr, err = apitypes.GetHTTPParameterValue(values, "SuccessExpression")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if successExpr != "" { scanConfig.setSuccessExpressionDeferredUpdate(successExpr) }
	
	var scanService providers.ScanService
	scanService = server.GetScanService(scanConfig.getProviderName())
	if scanService == nil { return apitypes.NewFailureDesc(
		"Unable to identify a scan service named '" + providerName + "'")
	}
	
	err = scanConfig.writeBack()
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Retrieve and set the provider parameters.
	for key, valueAr := range values {
		if strings.HasPrefix(key, "scan.") {
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(
				"Parameter " + key + " is ill-formatted")
			}
			var paramName string = strings.TrimPrefix(key, "scan.")
			// See if the parameter is known by the scanner.
			_, err = scanService.GetParameterDescription(paramName)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			// Create a ParameterValue and attach it to the ScanConfig.
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(
				"Value for scan parameter '" + paramName + "' is ill-formed") }
			var value string = valueAr[0]
			_, err = scanConfig.setParameterValue(paramName, value)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		}
	}
	
	// Add success image, if one was attached.
	var userId string = sessionToken.AuthenticatedUserid
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil { return apitypes.NewFailureDesc(
		"Internal error - could not identify user after use has been authenticated") }
	var imageFilepath string
	var repo Repo
	repo, err = server.dbClient.getRepo(scanConfig.getRepoId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	_, imageFilepath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if imageFilepath != "" { // a file was attached - presume that it is an image
		fmt.Println("file attached...")
		var flag Flag
		flag, err = server.dbClient.dbCreateFlag(name, desc, repo.getId(), imageFilepath)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		err = scanConfig.setFlagId(flag.getId())
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		// Add ACL entry.
		_, err = server.dbClient.dbCreateACLEntry(flag.getId(), user.getId(),
			[]bool{ true, true, true, true, true } )
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	}
	
	return scanConfig.asScanConfigDesc()
}

/*******************************************************************************
 * Arguments: FlagId
 * Returns: image file
 */
func getFlagImage(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var flagId string
	var err error
	flagId, err = apitypes.GetRequiredHTTPParameterValue(values, "FlagId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var flag Flag
	flag, err = server.dbClient.getFlag(flagId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, flagId,
		"getFlagImage"); failMsg != nil { return failMsg }

	var path string = flag.getSuccessImagePath()
	
	return apitypes.NewFileResponse(200, path, false)
}

/*******************************************************************************
 * Arguments: RepoId, Name, Description, image file
 * Returns: FlagDesc
 */
func defineFlag(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }
	
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var name string
	name, err = apitypes.GetRequiredHTTPParameterValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var desc string
	desc, err = apitypes.GetRequiredHTTPParameterValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var successImagePath string
	_, successImagePath, err = captureFile(repo, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if successImagePath == "" { return apitypes.NewFailureDesc("No file attached") }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.CreateInMask, repoId,
		"defineFlag"); failMsg != nil { return failMsg }

	var flag Flag
	flag, err = server.dbClient.dbCreateFlag(name, desc, repoId, successImagePath)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return flag.asFlagDesc()
}

/*******************************************************************************
 * Arguments: ScanConfigId, ImageObjId
 * Returns: ScanEventDesc
 */
func scanImage(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var scanConfigId, imageObjId string
	var err error
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	fmt.Println(scanConfigId)
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, imageObjId,
		"scanImage"); failMsg != nil { return failMsg }
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ExecuteMask, scanConfigId,
		"scanImage"); failMsg != nil { return failMsg }
	
	var dockerImage DockerImage
	dockerImage, err = server.dbClient.getDockerImage(imageObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if dockerImage == nil {
		return apitypes.NewFailureDesc("Docker image with object Id " + imageObjId + " not found")
	}
	
	var scanConfig ScanConfig
	scanConfig, err = server.dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if scanConfig == nil {
		return apitypes.NewFailureDesc("Scan Config with object Id " + scanConfigId + " not found")
	}

	fmt.Println("Getting scan parameters from configuration")
	var params = map[string]string{}
	var paramValueIds []string = scanConfig.getParameterValueIds()
	for _, id := range paramValueIds {
		var paramValue ParameterValue
		paramValue, err = server.dbClient.getParameterValue(id)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		params[paramValue.getName()] = paramValue.getStringValue()
	}

	// Identify the requested scan provider.
	// For now, just hard-code each provider.
	var scanProviderName = scanConfig.getProviderName()
	//var paramValues []string = scanConfig.getParameterValueIds()
	//var cmd *exec.Cmd
	
	var score string
	
	fmt.Println("Getting scan service...")
	var scanService providers.ScanService
	scanService = server.GetScanService(scanProviderName)
	if scanService == nil { return apitypes.NewFailureDesc(
		"Unable to identify a scan service named '" + scanProviderName + "'")
	}
	var scanContext providers.ScanContext
	scanContext, err = scanService.CreateScanContext(params)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var imageName string
	imageName, err = dockerImage.getFullName()
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var result *providers.ScanResult
	fmt.Println("Contacting scan service...")
	result, err = scanContext.ScanImage(imageName)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	fmt.Println("Scanner service completed")
	
	score = fmt.Sprintf("%d", len(result.Vulnerabilities))
	
	// TBD: Here we should use the scanConfig.SuccessExpression to compute the score.
	

	// Create a scan event.
	var userObjId string = sessionToken.AuthenticatedUserid
	var scanEvent ScanEvent
	scanEvent, err = server.dbClient.dbCreateScanEvent(scanConfig.getId(), imageObjId,
		userObjId, score)
	
	return scanEvent.asScanEventDesc()
}

/*******************************************************************************
 * Arguments: ImageObjId
 * Returns: ScanEventDesc (has empty fields, if there are no events)
 */
func getDockerImageStatus(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var imageObjId string
	var err error
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var image DockerImage
	image, err = server.dbClient.getDockerImage(imageObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, imageObjId,
		"getDockerImageStatus"); failMsg != nil { return failMsg }
	
	// Get the most recent ScanEvent, if any.
	var eventId string = image.getMostRecentScanEventId()
	if eventId == "" {
		return apitypes.NewScanEventDesc("", time.Now(), "", "", "", nil, "") // an empty ScanEventDesc
	} else {
		var event ScanEvent
		event, err = server.dbClient.getScanEvent(eventId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		return event.asScanEventDesc()
	}
}

/*******************************************************************************
 * Arguments: ScanConfigId
 * Returns: ScanConfigDesc
 */
func getScanConfigDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var scanConfigId string
	var err error
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var scanConfig ScanConfig
	scanConfig, err = server.dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, scanConfigId,
		"getScanConfigDesc"); failMsg != nil { return failMsg }
	
	return scanConfig.asScanConfigDesc()
}

/*******************************************************************************
 * Arguments: FlagId
 * Returns: FlagDesc
 */
func getFlagDesc(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var flagId string
	var err error
	flagId, err = apitypes.GetRequiredHTTPParameterValue(values, "FlagId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var flag Flag
	flag, err = server.dbClient.getFlag(flagId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, flagId,
		"getFlagDesc"); failMsg != nil { return failMsg }
	
	return flag.asFlagDesc()
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: EventDescBase...
 */
func getUserEvents(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var userId string = sessionToken.AuthenticatedUserid
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil { return apitypes.NewFailureDesc("Unidentified user, " + userId) }
	var eventIds []string = user.getEventIds()
	
	var eventDescs apitypes.EventDescs = make([]apitypes.EventDesc, 0)
	for _, eventId := range eventIds {
		var event Event
		var err error
		event, err = server.dbClient.getEvent(eventId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		eventDescs = append(eventDescs, event.asEventDesc())
	}
	
	return eventDescs
}

/*******************************************************************************
 * Arguments: ImageObjId
 * Returns: EventDescBase...
 */
func getDockerImageEvents(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var imageObjId string
	var err error
	imageObjId, err = apitypes.GetRequiredHTTPParameterValue(values, "ImageObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask,
		imageObjId, "getDockerImageEvents"); failMsg != nil { return failMsg }
	
	var image DockerImage
	image, err = server.dbClient.getDockerImage(imageObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var eventIds []string = image.getScanEventIds()
	var eventDescs apitypes.EventDescs = make([]apitypes.EventDesc, 0)
	for _, eventId := range eventIds {
		var event Event
		event, err = server.dbClient.getEvent(eventId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		eventDescs = append(eventDescs, event.asEventDesc())
	}
	
	return eventDescs
}

/*******************************************************************************
 * Arguments: DockerfileId
 * Returns: DockerfileExecEventDesc...
 */
func getDockerfileEvents(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var dockerfileId string
	var err error
	dockerfileId, err = apitypes.GetRequiredHTTPParameterValue(values, "DockerfileId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask,
		dockerfileId, "getDockerfileEvents"); failMsg != nil { return failMsg }
	
	var dockerfile Dockerfile
	dockerfile, err = server.dbClient.getDockerfile(dockerfileId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var eventIds []string = dockerfile.getDockerfileExecEventIds()
	var eventDescs apitypes.EventDescs = make([]apitypes.EventDesc, 0)
	for _, eventId := range eventIds {
		var event Event
		event, err = server.dbClient.getEvent(eventId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		eventDescs = append(eventDescs, event.asEventDesc())
	}
	
	return eventDescs
}

/*******************************************************************************
 * Arguments: RepoId, ScanConfigName
 * Returns: ScanConfigDesc
 */
func getScanConfigDescByName(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var configName string
	configName, err = apitypes.GetRequiredHTTPParameterValue(values, "ScanConfigName")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask,
		repoId, "getScanConfigDescByName"); failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	for _, scanConfigId := range repo.getScanConfigIds() {
		var scanConfig ScanConfig
		scanConfig, err = server.dbClient.getScanConfig(scanConfigId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if scanConfig.getName() == configName {
			return scanConfig.asScanConfigDesc()
		}
	}
	
	return apitypes.NewFailureDesc("ScanConfig with name " + configName +
		" not found in repo " + repo.getName())
}

/*******************************************************************************
 * Arguments: ScanConfigId
 * Returns: Result
 */
func remScanConfig(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var scanConfigId string
	scanConfigId, err = apitypes.GetRequiredHTTPParameterValue(values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.DeleteMask,
		scanConfigId, "remScanConfig"); failMsg != nil { return failMsg }
	
	var scanConfig ScanConfig
	scanConfig, err = server.dbClient.getScanConfig(scanConfigId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(scanConfig.getRepoId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		
	err = repo.deleteScanConfig(scanConfig)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return apitypes.NewResult(200, "Scan config removed")
}

/*******************************************************************************
 * Arguments: RepoId, FlagName
 * Returns: FlagDesc
 */
func getFlagDescByName(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredHTTPParameterValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var flagName string
	flagName, err = apitypes.GetRequiredHTTPParameterValue(values, "FlagName")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask,
		repoId, "getFlagDescByName"); failMsg != nil { return failMsg }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	for _, flagId := range repo.getFlagIds() {
		var flag Flag
		flag, err = server.dbClient.getFlag(flagId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if flag.getName() == flagName {
			return flag.asFlagDesc()
		}
	}
	
	return apitypes.NewFailureDesc("Flag with name " + flagName +
		" not found in repo " + repo.getName())
}

/*******************************************************************************
 * Arguments: FlagId
 * Returns: Result
 */
func remFlag(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var flagId string
	flagId, err = apitypes.GetRequiredHTTPParameterValue(values, "FlagId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.DeleteMask,
		flagId, "remFlag"); failMsg != nil { return failMsg }
	
	var flag Flag
	flag, err = server.dbClient.getFlag(flagId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(flag.getRepoId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	err = repo.deleteFlag(flag)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return apitypes.NewResult(200, "Flag removed")
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: Result
 */
func remDockerImage(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if _, failMsg := authenticateSession(server, sessionToken, values); failMsg != nil { return failMsg }

	var err error
	var imageId string
	imageId, err = apitypes.GetRequiredHTTPParameterValue(values, "ImageId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.DeleteMask,
		imageId, "remDockerImage"); failMsg != nil { return failMsg }
	
	var image DockerImage
	image, err = server.dbClient.getDockerImage(imageId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var repo Repo
	repo, err = server.dbClient.getRepo(image.getRepoId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	err = repo.deleteDockerImage(image)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return apitypes.NewResult(200, "Docker image removed")
}
