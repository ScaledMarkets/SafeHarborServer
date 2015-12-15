/*******************************************************************************
 * All of the REST handlers are contained here. These functions are called by
 * the ReqHandler.
 */

package server

import (
	"net/url"
	"mime/multipart"
	//"net/textproto"
	"fmt"
	//"errors"
	//"bufio"
	//"io"
	"io/ioutil"
	"os"
	"os/exec"
	//"strconv"
	"strings"
	"reflect"
	//"bytes"
	//"time"
	
	// My packages:
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
		return apitypes.NewFailureDesc("User was authenticated but not found in the database")
	}
	
	if ! user.isActive() { return apitypes.NewFailureDesc("User is not active") }
	
	// Create new user session.
	var token *apitypes.SessionToken = server.authService.createSession(creds)
	token.SetRealmId(user.getRealmId())
	
	// Flag whether the user has Write access to the realm.
	var realm Realm
	realm, err = server.dbClient.getRealm(user.getRealmId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var entry ACLEntry
	entry, err = realm.getACLEntryForPartyId(user.getId())
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if entry != nil {
		token.SetIsAdminUser(entry.getPermissionMask()[apitypes.CanWrite])
	}
	
	return token
}

/*******************************************************************************
 * Arguments: none
 * Returns: apitypes.Result
 */
func logout(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	server.authService.invalidateSessionId(sessionToken.UniqueSessionId)
	return apitypes.NewResult(200, "Logged out")
}

/*******************************************************************************
 * Arguments: apitypes.UserInfo
 * Returns: apitypes.UserDesc
 */
func createUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var userObjId string
	var err error
	userObjId, err = apitypes.GetRequiredPOSTFieldValue(values, "UserObjId")
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
 * Arguments: RealmId, <name>
 * Returns: apitypes.GroupDesc
 */
func createGroup(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var groupName string
	groupName, err = apitypes.GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var groupDescription string
	groupDescription, err = apitypes.GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var addMeStr string
	addMeStr, err = apitypes.GetPOSTFieldValue(values, "AddMe")
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var groupId string
	var err error
	groupId, err = apitypes.GetRequiredPOSTFieldValue(values, "GroupId")
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
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredPOSTFieldValue(values, "GroupId")
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
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = apitypes.GetRequiredPOSTFieldValue(values, "GroupId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var userObjId string
	userObjId, err = apitypes.GetRequiredPOSTFieldValue(values, "UserObjId")
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return apitypes.NewFailureDesc("Not implemented yet: remGroupUser")
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
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
 * Arguments: none
 * Returns: apitypes.RealmDesc
 */
func createRealm(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.DeleteMask, realmId,
		"deactivateRealm"); failMsg != nil { return failMsg }
	
	err = server.dbClient.dbDeactivateRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewResult(200, "Realm deactivated")
}

/*******************************************************************************
 * Arguments: RealmId, UserObjId
 * Returns: apitypes.Result
 */
func addRealmUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	var userObjId string
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	userObjId, err = apitypes.GetRequiredPOSTFieldValue(values, "UserObjId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, realmId,
		"addRealmUser"); failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = server.dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if realm == nil { return apitypes.NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	err = realm.addUserId(userObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	return apitypes.NewResult(200, "User added to realm")
}

/*******************************************************************************
 * Arguments: RealmId, UserObjId
 * Returns: apitypes.Result
 */
func remRealmUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return apitypes.NewFailureDesc("Not implemented yet: remRealmUser")
}

/*******************************************************************************
 * Arguments: RealmId, UserId
 * Returns: apitypes.UserDesc
 */
func getRealmUser(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	var realmUserId string
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	realmUserId, err = apitypes.GetRequiredPOSTFieldValue(values, "UserId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, realmId,
		"getRealmUser"); failMsg != nil { return failMsg }
	
	var realm Realm
	realm, err = server.dbClient.getRealm(realmId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if realm == nil { return apitypes.NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	var realmUser User
	realmUser, err = realm.getUserByUserId(realmUserId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if realmUser == nil { return apitypes.NewFailureDesc("User with user id " + realmUserId +
		" in realm " + realm.getName() + " not found.") }
	return realmUser.asUserDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*apitypes.UserDesc
 */
func getRealmUsers(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var groupDescs apitypes.GroupDescs = make([]*apitypes.GroupDesc, 0)
	var realmId string
	var err error
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
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
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, realmId,
		"getRealmRepos"); failMsg != nil { return failMsg }
	
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
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

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

	var failMsg *apitypes.FailureDesc = authenticateSession(server, sessionToken)
	if failMsg != nil { // no session Id found; see if it was sent as an HTTP parameter.
		// We do this because the client is likely to invoke this method directly
		// from a javascript app in a browser instead of from the middle tier,
		// and the browser/javascript framework is not likely going to allow
		// the javascript app to set a cookie for a domain to which _IT_ has
		// not authenticated directly. (The authenticate method is most likely
		// called by the middle tier - not a javascript app.) To get around this,
		// we allow the addAndExecDockerfile method to provide the session Id
		// as an HTTP parameter, instead of via the normal mechanism (a cookie).
		
		var sessionId string
		valuear, found := values["SessionId"]
		if ! found { return failMsg }
		sessionId = valuear[0]
		if sessionId == "" { return failMsg }
		sessionToken = server.authService.validateSessionId(sessionId)  // returns nil if invalid
		if sessionToken == nil { return failMsg }
	}

	fmt.Println("Creating repo...")
	var err error
	var realmId string
	realmId, err = apitypes.GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var repoName string
	repoName, err = apitypes.GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var repoDesc string
	repoDesc, err = apitypes.GetRequiredPOSTFieldValue(values, "Description")
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
	
	_, err = createDockerfile(sessionToken, server.dbClient, repo, repo.getDescription(), values, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: apitypes.Result
 */
func deleteRepo(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return apitypes.NewFailureDesc("Not implemented yet: deleteRepo")
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []*apitypes.DockerfileDesc
 */
func getDockerfiles(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredPOSTFieldValue(values, "RepoId")
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = apitypes.GetRequiredPOSTFieldValue(values, "RepoId")
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
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	fmt.Println("addDockerfile handler")
	
	// Identify the repo.
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var desc string
	desc, err = apitypes.GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.CreateInMask, repoId,
		"addDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if repo == nil { return apitypes.NewFailureDesc("Repo does not exist") }
	
	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, desc, values, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if dockerfile == nil { return apitypes.NewFailureDesc("No dockerfile was attached") }
	
	return dockerfile.asDockerfileDesc()
}

/*******************************************************************************
 * Arguments: DockerfileId, Description (optional), File
 * Returns: apitypes.Result
 */
func replaceDockerfile(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return apitypes.NewFailureDesc("Not implemented yet: replaceDockerfile")
}

/*******************************************************************************
 * Arguments: DockerfileId, ImageName
 * Returns: apitypes.DockerImageDesc
 */
func execDockerfile(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	fmt.Println("Entered execDockerfile")
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	// Identify the Dockerfile.
	var err error
	var dockerfileId string
	dockerfileId, err = apitypes.GetRequiredPOSTFieldValue(values, "DockerfileId")
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
	
	var failMsg *apitypes.FailureDesc = authenticateSession(server, sessionToken)
	if failMsg != nil { // no session Id found; see if it was sent as an HTTP parameter.
		// We do this because the client is likely to invoke this method directly
		// from a javascript app in a browser instead of from the middle tier,
		// and the browser/javascript framework is not likely going to allow
		// the javascript app to set a cookie for a domain to which _IT_ has
		// not authenticated directly. (The authenticate method is most likely
		// called by the middle tier - not a javascript app.) To get around this,
		// we allow the addAndExecDockerfile method to provide the session Id
		// as an HTTP parameter, instead of via the normal mechanism (a cookie).
		
		var sessionId string
		valuear, found := values["SessionId"]
		if ! found { return failMsg }
		sessionId = valuear[0]
		if sessionId == "" { return failMsg }
		sessionToken = server.authService.validateSessionId(sessionId)  // returns nil if invalid
		if sessionToken == nil { return failMsg }
	}
	
	var repoId string
	var err error
	repoId, err = apitypes.GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, repoId,
		"addAndExecDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var repo Repo
	repo, err = dbClient.getRepo(repoId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if repo == nil { return apitypes.NewFailureDesc("Repo does not exist") }
	
	var desc string
	desc, err = apitypes.GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }

	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, desc, values, files)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if dockerfile == nil { return apitypes.NewFailureDesc("No dockerfile was attached") }
	
	var image DockerImage
	image, err = buildDockerfile(server, dockerfile, sessionToken, dbClient, values)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return image.asDockerImageDesc()
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: file content
 */
func downloadImage(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var imageId string
	var err error
	imageId, err = apitypes.GetRequiredPOSTFieldValue(values, "ImageId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var dbClient DBClient = server.dbClient
	var resource Resource
	resource, err = dbClient.getResource(imageId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if resource == nil { return apitypes.NewFailureDesc("Unable to identify resource with Id " + imageId) }
	var image Image
	var isType bool
	image, isType = resource.(Image)
	if ! isType { return apitypes.NewFailureDesc("Resource with Id " + imageId + " is not an Image") }
	var dockerImage DockerImage
	dockerImage, isType = image.(DockerImage)
	if ! isType { return apitypes.NewFailureDesc("Image is not a docker image") }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.ReadMask, imageId,
		"downloadImage"); failMsg != nil { return failMsg }
	
	fmt.Println("Creating temp file to save the image to...")
	var tempFile *os.File
	tempFile, err = ioutil.TempFile("", "")
	// TO DO: Is the above a security issue?
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var tempFilePath = tempFile.Name()
	
	var imageFullName string
	imageFullName, err = dockerImage.getFullName()
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	//var stderr bytes.Buffer
	var cmd *exec.Cmd= exec.Command("docker", "save", "-o", tempFilePath, imageFullName)
	//cmd.Stderr = &stderr
	fmt.Println(fmt.Sprintf("Running docker save -o%s %s", tempFilePath, imageFullName))
	err = cmd.Run()
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return apitypes.NewFileResponse(200, tempFilePath)
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func setPermission(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	// Get the mask that we will use to overwrite the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredPOSTFieldValue(values, "PartyId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var resourceId string
	resourceId, err = apitypes.GetRequiredPOSTFieldValue(values, "ResourceId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var smask []string = make([]string, 5)
	smask[0], err = apitypes.GetRequiredPOSTFieldValue(values, "CanCreateIn")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[1], err = apitypes.GetRequiredPOSTFieldValue(values, "CanRead")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[2], err = apitypes.GetRequiredPOSTFieldValue(values, "CanWrite")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[3], err = apitypes.GetRequiredPOSTFieldValue(values, "CanExecute")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[4], err = apitypes.GetRequiredPOSTFieldValue(values, "CanDelete")
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	// Get the mask that we will be adding to the current mask.
	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredPOSTFieldValue(values, "PartyId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var resourceId string
	var smask []string = make([]string, 5)
	resourceId, err = apitypes.GetRequiredPOSTFieldValue(values, "ResourceId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[0], err = apitypes.GetRequiredPOSTFieldValue(values, "CanCreateIn")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[1], err = apitypes.GetRequiredPOSTFieldValue(values, "CanRead")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[2], err = apitypes.GetRequiredPOSTFieldValue(values, "CanWrite")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[3], err = apitypes.GetRequiredPOSTFieldValue(values, "CanExecute")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	smask[4], err = apitypes.GetRequiredPOSTFieldValue(values, "CanDelete")
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
 * Returns: PermissionDesc
 */
func remPermission(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return apitypes.NewFailureDesc("Not implemented yet: remPermission")
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId
 * Returns: PermissionDesc
 */
func getPermission(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var partyId string
	var err error
	partyId, err = apitypes.GetRequiredPOSTFieldValue(values, "PartyId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	var resourceId string
	resourceId, err = apitypes.GetRequiredPOSTFieldValue(values, "ResourceId")
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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

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

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
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
 * Returns: 
 */
func getMyDockerfiles(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var realms map[string]Realm = make(map[string]Realm)
	var repos map[string]Repo = make(map[string]Repo)
	var dockerfiles map[string]Dockerfile = make(map[string]Dockerfile)
	
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
			case Dockerfile: dockerfiles[v.getId()] = v
				fmt.Println("\t\ta Dockerfile")
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
			if r == nil { return apitypes.NewFailureDesc("No repo found for Id " + repoId) }
			repos[repoId] = r
		}
	}
	for _, repo := range repos {
		for _, dockerfileId := range repo.getDockerfileIds() {
			var d Dockerfile
			var err error
			d, err = dbClient.getDockerfile(dockerfileId)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			if d == nil { return apitypes.NewFailureDesc("Internal Error: No dockerfile found for Id " + dockerfileId) }
			dockerfiles[dockerfileId] = d
		}
	}
	
	fmt.Println("Creating result...")
	var dockerfileDescs apitypes.DockerfileDescs = make([]*apitypes.DockerfileDesc, 0)
	for _, dockerfile := range dockerfiles {
		fmt.Println("\tappending dockerfile", dockerfile.getName())
		dockerfileDescs = append(dockerfileDescs, dockerfile.asDockerfileDesc())
	}
	return dockerfileDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: 
 */
func getMyDockerImages(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var realms map[string]Realm = make(map[string]Realm)
	var repos map[string]Repo = make(map[string]Repo)
	var dockerImages map[string]DockerImage = make(map[string]DockerImage)
	
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
			case DockerImage: dockerImages[v.getId()] = v
				fmt.Println("\t\ta DockerImage")
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
			if r == nil { return apitypes.NewFailureDesc("No repo found for Id " + repoId) }
			repos[repoId] = r
		}
	}
	for _, repo := range repos {
		for _, dockerImageId := range repo.getDockerImageIds() {
			var dimg DockerImage
			var err error
			dimg, err = dbClient.getDockerImage(dockerImageId)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			if dimg == nil { return apitypes.NewFailureDesc("Internal error: No image found for Id " + dockerImageId) }
			dockerImages[dockerImageId] = dimg
		}
	}
	
	fmt.Println("Creating result...")
	var dockerImageDescs apitypes.DockerImageDescs = make([]*apitypes.DockerImageDesc, 0)
	for _, dockerImage := range dockerImages {
		fmt.Println("\tappending dockerImage", dockerImage.getName())
		dockerImageDescs = append(dockerImageDescs, dockerImage.asDockerImageDesc())
	}
	return dockerImageDescs
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: apitypes.ScanProviderDescs
 */
func getScanProviders(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var providerDescs apitypes.ScanProviderDescs = make([]*apitypes.ScanProviderDesc, 0)
	var services []providers.ScanService = server.GetScanServices()
	for _, service := range services {
		providerDescs = append(providerDescs, service.AsScanProviderDesc())
	}
	
	return providerDescs
}

/*******************************************************************************
 * Arguments: Name, Description, RepoId, ProviderName, Params..., SuccessGraphicImageURL, FailureGraphicImageURL
 * Returns: ScanConfigDesc
 */
func defineScanConfig(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var repoId string
	var name string
	var desc string
	var err error

	name, err = apitypes.GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	desc, err = apitypes.GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	repoId, err = apitypes.GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, apitypes.WriteMask, repoId,
		"defineScanConfig"); failMsg != nil { return failMsg }
	
	var providerName string
	providerName, err = apitypes.GetRequiredPOSTFieldValue(values, "ProviderName")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var successGraphicImageURL string
	successGraphicImageURL, err = apitypes.GetRequiredPOSTFieldValue(values, "SuccessGraphicImageURL")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var failureGraphicImageURL string
	failureGraphicImageURL, err = apitypes.GetRequiredPOSTFieldValue(values, "FailureGraphicImageURL")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	var scanConfig ScanConfig
	var paramValueIds []string = make([]string, 0)
	scanConfig, err = server.dbClient.dbCreateScanConfig(name, desc, repoId,
		providerName, paramValueIds, successGraphicImageURL, failureGraphicImageURL)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Look for each parameter required by the provider.
	for key, valueAr := range values {
		if strings.HasPrefix(key, "scan.") {
			if len(valueAr) != 1 { return apitypes.NewFailureDesc(
				"Parameter " + key + " is ill-formatted")
			}
			var name string = strings.TrimPrefix(key, "scan.")
			// See if the parameter is known by the scanner.
			var scanService providers.ScanService
			scanService = server.GetScanService(providerName)
			if scanService == nil { return apitypes.NewFailureDesc(
				"Unable to identify a scan service named '" + providerName + "'")
			}
			var value string = valueAr[0]
			var pval ParameterValue
			pval, err = scanConfig.setParameterValue(name, value)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			paramValueIds = append(paramValueIds, pval.getId())
		}
	}
	
	// Add ACL entry to enable the current user to access what he/she just created.
	var userId string = sessionToken.AuthenticatedUserid
	var user User = server.dbClient.dbGetUserByUserId(userId)

	var obj PersistObj = server.dbClient.getPersistentObject(user.getId())
	assertThat(obj != nil, "Internal error in defineScanConfig: obj is nil")
	
	_, err = server.dbClient.dbCreateACLEntry(scanConfig.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	return scanConfig.asScanConfigDesc()
}


/*******************************************************************************
 * Arguments: ScanConfigId, Desc, RepoId, ProviderName, Params..., optional image file
 * Returns: ScanConfigDesc
 */
func replaceScanConfig(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	//....
	return apitypes.NewFailureDesc("Not implemented yet: replaceScanConfig")
}

/*******************************************************************************
 * Arguments: ScanConfigId, ImageObjId
 * Returns: ScanEventDesc
 */
func scanImage(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var scanConfigId, imageObjId string
	var err error
	scanConfigId, err = apitypes.GetRequiredPOSTFieldValue(values, "ScanConfigId")
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	imageObjId, err = apitypes.GetRequiredPOSTFieldValue(values, "ImageObjId")
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

	// Get the current version of the ScanConfig file.
	var extObjId string = scanConfig.getCurrentExtObjId()
	var scanConfigTempFile *os.File
	scanConfigTempFile, err = scanConfig.getAsTempFile(extObjId)
	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	if scanConfigTempFile == nil {
		return apitypes.NewFailureDesc("Unable to obtain scan config as a temp file")
	}
	defer os.RemoveAll(scanConfigTempFile.Name())
	
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

	// Create a scan event.
	var userObjId string = sessionToken.AuthenticatedUserid
	var scanEvent ScanEvent
	scanEvent, err = server.dbClient.dbCreateScanEvent(scanConfig.getId(), imageObjId,
		userObjId, score, extObjId)
	
	return scanEvent.asScanEventDesc()
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: EventDesc...
 */
func getUserEvents(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	//....
	return apitypes.NewFailureDesc("Not implemented yet: getUserEvents")
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: EventDesc...
 */
func getImageEvents(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	//....
	return apitypes.NewFailureDesc("Not implemented yet: getImageEvents")
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: EventDesc (has empty fields, if there are no events)
 */
func getImageStatus(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	//....
	return apitypes.NewFailureDesc("Not implemented yet: getImageStatus")
}

/*******************************************************************************
 * Arguments: DockerfileId
 * Returns: DockerfileExecEventDesc...
 */
func getDockerfileEvents(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	//....
	return apitypes.NewFailureDesc("Not implemented yet: getDockerfileEvents")
}

/*******************************************************************************
 * Arguments: Name, image file
 * Returns: FlagDesc
 */
func defineFlag(server *Server, sessionToken *apitypes.SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) apitypes.RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	//....
	return apitypes.NewFailureDesc("Not implemented yet: defineFlag")
}