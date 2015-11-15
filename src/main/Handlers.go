/*******************************************************************************
 * All of the REST handlers are contained here. These functions are called by
 * the ReqHandler.
 */

package main

import (
	"net/url"
	"mime/multipart"
	//"net/textproto"
	"fmt"
	//"errors"
	//"bufio"
	//"io"
	//"io/ioutil"
	//"os"
	"os/exec"
	//"strconv"
	"strings"
	"reflect"
)

/*******************************************************************************
 * Arguments: (none)
 * Returns: Result
 */
func ping(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	fmt.Println("ping request received")
	return &Result{
		Status: 200,
		Message: "Server is up",
	}
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: Result
 */
func clearAll(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	fmt.Println("clearAll")
	
	if ! server.Debug {
		return NewFailureDesc("Not in debug mode - returning from clearAll")
	}
	
	// Kill all docker containers:
	// docker kill $(docker ps -a -q)
	// docker rm $(docker ps -a -q)
	var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "ps", "-a", "-q")
	var output []byte
	output, _ = cmd.CombinedOutput()
	var containers string = string(output)
	if strings.HasPrefix(containers, "Error") {
		return NewFailureDesc(containers)
	}
	fmt.Println("Containers are:")
	fmt.Println(containers)
	
	cmd = exec.Command("/usr/bin/docker", "kill", containers)
	output, _ = cmd.CombinedOutput()
	var outputStr string = string(output)
	//if strings.HasPrefix(outputStr, "Error") {
	//	return NewFailureDesc(outputStr)
	//}
	fmt.Println("All containers were signalled to stop")
	
	cmd = exec.Command("/usr/bin/docker", "rm", containers)
	output, _ = cmd.CombinedOutput()
	outputStr = string(output)
	//if strings.HasPrefix(outputStr, "Error") {
	//	return NewFailureDesc(outputStr)
	//}
	fmt.Println("All containers were removed")
	
	// Remove all of the docker images that were created by SafeHarborServer.
	fmt.Println("Removing docker images that were created by SafeHarbor:")
	var dbClient DBClient = server.dbClient
	for _, realmId := range dbClient.dbGetAllRealmIds() {
		var realm Realm = dbClient.getRealm(realmId)
		fmt.Println("For realm " + realm.getName() + ":")
		
		for _, repoId := range realm.getRepoIds() {
			var repo Repo = dbClient.getRepo(repoId)
			fmt.Println("\tFor repo " + repo.getName() + ":")
			
			for _, imageId := range repo.getDockerImageIds() {
				
				var image DockerImage = dbClient.getDockerImage(imageId)
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
	
	return NewResult(200, "Persistent state reset")
}

/*******************************************************************************
 * Arguments: Credentials
 * Returns: SessionToken
 */
func printDatabase(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	fmt.Println("printDatabase")
	
	if ! server.Debug {
		return NewFailureDesc("Not in debug mode - returning from printDatabase")
	}
	
	server.dbClient.printDatabase()
	
	return NewResult(200, "Database printed to stdout on server.")
}

/*******************************************************************************
 * Arguments: Credentials
 * Returns: SessionToken
 */
func authenticate(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	var creds *Credentials
	var err error
	creds, err = GetCredentials(values)
	if err != nil { return NewFailureDesc(err.Error()) }
	var token *SessionToken = server.authService.authenticateCredentials(creds)
	var user User = server.dbClient.dbGetUserByUserId(token.AuthenticatedUserid)
	if user == nil {
		server.authService.invalidateSessionId(token.UniqueSessionId)
		return NewFailureDesc("User was authenticated but not found in the database")
	}
	token.setRealmId(user.getRealmId())
	
	// Flag whether the user has Write access to the realm.
	var realm Realm = server.dbClient.getRealm(user.getRealmId())
	var entry ACLEntry = realm.getACLEntryForPartyId(user.getId())
	if entry != nil {
		token.setIsAdminUser(entry.getPermissionMask()[CanWrite])
	}
	
	return token
}

/*******************************************************************************
 * Arguments: none
 * Returns: Result
 */
func logout(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	


	return nil
}

/*******************************************************************************
 * Arguments: UserInfo
 * Returns: UserDesc
 */
func createUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var err error
	var userInfo *UserInfo
	userInfo, err = GetUserInfo(values)
	if err != nil { return NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, CreateMask, userInfo.RealmId,
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
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return newUser.asUserDesc()
}

/*******************************************************************************
 * Arguments: UserObjId
 * Returns: Result
 */
func deleteUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: GroupDesc
 */
func createGroup(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var groupName string
	groupName, err = GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return NewFailureDesc(err.Error()) }

	var groupDescription string
	groupDescription, err = GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var addMeStr string = GetPOSTFieldValue(values, "AddMe")
	var addMe bool = false
	if addMeStr == "true" { addMe = true }
	fmt.Println(fmt.Sprintf("AddMe=%s", addMeStr))

	if failMsg := authorizeHandlerAction(server, sessionToken, CreateMask, realmId,
		"createGroup"); failMsg != nil { return failMsg }
	
	var group Group
	group, err = server.dbClient.dbCreateGroup(realmId, groupName, groupDescription)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if addMe {
		var userId string = sessionToken.AuthenticatedUserid
		var user User = server.dbClient.dbGetUserByUserId(userId)
		err = group.addUserId(user.getId())
		if err != nil { return NewFailureDesc(err.Error()) }
	}
	
	return group.asGroupDesc()
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: Result
 */
func deleteGroup(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: []*UserDesc
 */
func getGroupUsers(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = GetRequiredPOSTFieldValue(values, "GroupId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, groupId,
		"getGroupUsers"); failMsg != nil { return failMsg }
	
	var group Group = server.dbClient.getGroup(groupId)
	if group == nil { return NewFailureDesc(fmt.Sprintf(
		"No group with Id %s", groupId))
	}
	var userObjIds []string = group.getUserObjIds()
	var userDescs UserDescs = make([]*UserDesc, 0)
	for _, id := range userObjIds {
		var user User = server.dbClient.getUser(id)
		if user == nil { return NewFailureDesc(fmt.Sprintf(
			"Internal error: No user with Id %s", id))
		}
		var userDesc *UserDesc = user.asUserDesc()
		userDescs = append(userDescs, userDesc)
	}
	
	return userDescs
}

/*******************************************************************************
 * Arguments: GroupId, UserObjId
 * Returns: Result
 */
func addGroupUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var groupId string
	groupId, err = GetRequiredPOSTFieldValue(values, "GroupId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var userObjId string
	userObjId, err = GetRequiredPOSTFieldValue(values, "UserObjId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, WriteMask, groupId,
		"addGroupUser"); failMsg != nil { return failMsg }
	
	var group Group = server.dbClient.getGroup(groupId)
	if group == nil { return NewFailureDesc(fmt.Sprintf(
		"No group with Id %s", groupId))
	}

	err = group.addUserId(userObjId)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var user User = server.dbClient.getUser(userObjId)
	user.addGroupId(groupId)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return &Result{
		Status: 200,
		Message: "User added to group",
	}
}

/*******************************************************************************
 * Arguments: GroupId, UserObjId
 * Returns: Result
 */
func remGroupUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: RealmInfo, UserInfo
 * Returns: UserDesc
 */
func createRealmAnon(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	// Create new administrative user.
	var err error
	var userInfo *UserInfo
	userInfo, err = GetUserInfo(values)
	if err != nil { return NewFailureDesc(err.Error()) }
	var newUserId string = userInfo.UserId
	var newUserName string = userInfo.UserName
	//var realmId string = userInfo.RealmId  // ignored
	var email string = userInfo.EmailAddress
	var pswd string = userInfo.Password

	var dbClient DBClient = server.dbClient
	
	// Create a realm.
	var newRealmInfo *RealmInfo
	newRealmInfo, err = GetRealmInfo(values)
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println("Creating realm ", newRealmInfo.RealmName)
	var newRealm Realm
	newRealm, err = server.dbClient.dbCreateRealm(newRealmInfo, newUserId)
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println("Created realm", newRealmInfo.RealmName)
	
	// Create a user
	var newUser User
	newUser, err = dbClient.dbCreateUser(newUserId, newUserName, email, pswd, newRealm.getId())
	if err != nil { return NewFailureDesc("Unable to create user: " + err.Error()) }

	// Add ACL entry to enable the current user to access what he/she just created.
	server.dbClient.dbCreateACLEntry(newRealm.getId(), newUser.getId(),
		[]bool{ true, true, true, true, true } )
	
	return newUser.asUserDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: RealmDesc
 */
func getRealmDesc(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, realmId,
		"getRealmDesc"); failMsg != nil { return failMsg }
	
	var realm Realm = server.dbClient.getRealm(realmId)
	if realm == nil { return NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: none
 * Returns: RealmDesc
 */
func createRealm(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmInfo *RealmInfo
	realmInfo, err = GetRealmInfo(values)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	fmt.Println("Creating realm ", realmInfo.RealmName)
	var realm Realm
	realm, err = server.dbClient.dbCreateRealm(realmInfo, user.getId())
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println("Created realm", realmInfo.RealmName)

	// Add ACL entry to enable the current user to access what he/she just created.
	server.dbClient.dbCreateACLEntry(realm.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: Result
 */
func deleteRealm(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: RealmId, UserObjId
 * Returns: Result
 */
func addRealmUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	var userObjId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	userObjId, err = GetRequiredPOSTFieldValue(values, "UserObjId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, WriteMask, realmId,
		"addRealmUser"); failMsg != nil { return failMsg }
	
	var realm Realm
	realm = server.dbClient.getRealm(realmId)
	if realm == nil { return NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	err = realm.addUserId(userObjId)
	if err != nil { return NewFailureDesc(err.Error()) }
	return NewResult(200, "User added to realm")
}

/*******************************************************************************
 * Arguments: RealmId, UserObjId
 * Returns: Result
 */
func remRealmUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: RealmId, UserId
 * Returns: UserDesc
 */
func getRealmUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	var realmUserId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	realmUserId, err = GetRequiredPOSTFieldValue(values, "UserId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, realmId,
		"getRealmUser"); failMsg != nil { return failMsg }
	
	var realm Realm
	realm = server.dbClient.getRealm(realmId)
	if realm == nil { return NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	var realmUser User = realm.getUserByUserId(realmUserId)
	if realmUser == nil { return NewFailureDesc("User with user id " + realmUserId +
		" in realm " + realm.getName() + " not found.") }
	return realmUser.asUserDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*UserDesc
 */
func getRealmUsers(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var realmId string
	var err error
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, realmId,
		"getRealmUsers"); failMsg != nil { return failMsg }
		
	var realm Realm = server.dbClient.getRealm(realmId)
	if realm == nil { return NewFailureDesc("Realm with Id " + realmId + " not found") }
	var userObjIds []string = realm.getUserObjIds()
	var userDescs UserDescs = make([]*UserDesc, 0)
	for _, userObjId := range userObjIds {
		var user User = server.dbClient.getUser(userObjId)
		if user == nil {
			fmt.Println("Internal error: user with obj Id " + userObjId + " not found")
			continue
		}
		var userDesc *UserDesc = user.asUserDesc()
		userDescs = append(userDescs, userDesc)
	}
	return userDescs
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []*GroupDesc
 */
func getRealmGroups(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var groupDescs GroupDescs = make([]*GroupDesc, 0)
	var realmId string
	var err error
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, realmId,
		"getRealmGroups"); failMsg != nil { return failMsg }
	
	var realm Realm = server.dbClient.getRealm(realmId)
	if realm == nil { return NewFailureDesc("Realm with Id " + realmId + " not found") }
	var groupIds []string = realm.getGroupIds()
	for _, groupId := range groupIds {
		var group Group = server.dbClient.getGroup(groupId)
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
 * Returns: []*RepoDesc
 */
func getRealmRepos(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, realmId,
		"getRealmRepos"); failMsg != nil { return failMsg }
	
	var realm Realm
	realm = server.dbClient.getRealm(realmId)
	if realm == nil { return NewFailureDesc("Cound not find realm with Id " + realmId) }
	
	var repoIds []string = realm.getRepoIds()
	
	var result RepoDescs = make([]*RepoDesc, 0)
	for _, id := range repoIds {
		
		var repo Repo = server.dbClient.getRepo(id)
		if repo == nil { return NewFailureDesc(fmt.Sprintf(
			"Internal error: no Repo found for Id %s", id))
		}
		var desc *RepoDesc = repo.asRepoDesc()
		// Add to result
		result = append(result, desc)
	}

	return result
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: []*RealmDesc
 */
func getAllRealms(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var realmIds []string = server.dbClient.dbGetAllRealmIds()
	
	var result RealmDescs = make([]*RealmDesc, 0)
	for _, realmId := range realmIds {
		
		var realm Realm = server.dbClient.getRealm(realmId)
		if realm == nil { return NewFailureDesc(fmt.Sprintf(
			"Internal error: no Realm found for Id %s", realmId))
		}
		var desc *RealmDesc = realm.asRealmDesc()
		result = append(result, desc)
	}
	
	return result
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: RepoDesc
 */
func createRepo(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	fmt.Println("Creating repo...")
	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }

	var repoName string
	repoName, err = GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return NewFailureDesc(err.Error()) }

	var repoDesc string
	repoDesc, err = GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, CreateMask, realmId,
		"createRepo"); failMsg != nil { return failMsg }
	
	fmt.Println("Creating repo", repoName)
	var repo Repo
	repo, err = server.dbClient.dbCreateRepo(realmId, repoName, repoDesc)
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println("createRepo.A")

	// Add ACL entry to enable the current user to access what he/she just created.
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	fmt.Println("createRepo.B")
	server.dbClient.dbCreateACLEntry(repo.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	fmt.Println("createRepo.C")
	
	_, err = createDockerfile(sessionToken, server.dbClient, repo, repo.getDescription(), values, files)
	fmt.Println("createRepo.D")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: Result
 */
func deleteRepo(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []*DockerfileDesc
 */
func getDockerfiles(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, repoId,
		"getDockerfiles"); failMsg != nil { return failMsg }
	
	var repo Repo = server.dbClient.getRepo(repoId)
	if repo == nil { return NewFailureDesc(fmt.Sprintf(
		"Repo with Id %s not found", repoId)) }
	
	var dockerfileIds []string = repo.getDockerfileIds()	
	var result DockerfileDescs = make([]*DockerfileDesc, 0)
	for _, id := range dockerfileIds {
		
		var dockerfile Dockerfile = server.dbClient.getDockerfile(id)
		if dockerfile == nil { return NewFailureDesc(fmt.Sprintf(
			"Internal error: no Dockerfile found for Id %s", id))
		}
		var desc *DockerfileDesc = dockerfile.asDockerfileDesc()
		// Add to result
		result = append(result, desc)
	}

	return result
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []*DockerImageDesc
 */
func getImages(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var err error
	var repoId string
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, repoId,
		"getImages"); failMsg != nil { return failMsg }
	
	var repo Repo = server.dbClient.getRepo(repoId)
	if repo == nil { return NewFailureDesc(fmt.Sprintf(
		"Repo with Id %s not found", repoId)) }
	
	var imageIds []string = repo.getDockerImageIds()
	var result DockerImageDescs = make([]*DockerImageDesc, 0)
	for _, id := range imageIds {
		
		var dockerImage DockerImage = server.dbClient.getDockerImage(id)
		if dockerImage == nil { return NewFailureDesc(fmt.Sprintf(
			"Internal error: no DockerImage found for Id %s", id))
		}
		var imageDesc *DockerImageDesc = dockerImage.asDockerImageDesc()
		result = append(result, imageDesc)
	}
	
	return result
}

/*******************************************************************************
 * Arguments: RepoId, File
 * Returns: DockerfileDesc
 * The File argument is obtained from the values as follows:
 *    The name specified by the client is keyed on "filename".
 * The handler should move the file to a permanent name.
 */
func addDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	fmt.Println("addDockerfile handler")
	
	// Identify the repo.
	var repoId string
	var err error
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }

	var desc string
	desc, err = GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, CreateMask, repoId,
		"addDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var repo Repo = dbClient.getRepo(repoId)
	if repo == nil { return NewFailureDesc("Repo does not exist") }
	
	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, desc, values, files)
	if err != nil { return NewFailureDesc(err.Error()) }
	if dockerfile == nil { return NewFailureDesc("No dockerfile was attached") }
	
	return dockerfile.asDockerfileDesc()
}

/*******************************************************************************
 * Arguments: RepoId, DockerfileId, File
 * Returns: Result
 */
func replaceDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: RepoId, DockerfileId, ImageName
 * Returns: DockerImageDesc
 */
func execDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	fmt.Println("Entered execDockerfile")
	
	var repoId string
	var err error
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	if repoId == "" { return NewFailureDesc("No HTTP parameter found for RepoId") }

	// Identify the Dockerfile.
	var dockerfileId string
	dockerfileId, err = GetRequiredPOSTFieldValue(values, "DockerfileId")
	if err != nil { return NewFailureDesc(err.Error()) }
	if dockerfileId == "" { return NewFailureDesc("No HTTP parameter found for DockerfileId") }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ExecuteMask, dockerfileId,
		"execDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var dockerfile Dockerfile = dbClient.getDockerfile(dockerfileId)
	fmt.Println("Dockerfile name =", dockerfile.getName())
	
	var image DockerImage
	image, err = buildDockerfile(dockerfile, sessionToken, dbClient, values)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return image.asDockerImageDesc()
}
	
/*******************************************************************************
 * Arguments: RepoId, Description, ImageName, <File attachment>
 * Returns: DockerImageDesc
 */
func addAndExecDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	fmt.Println("Entered addAndExecDockerfile")
	
	var repoId string
	var err error
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	if repoId == "" { return NewFailureDesc("No HTTP parameter found for RepoId") }

	if failMsg := authorizeHandlerAction(server, sessionToken, WriteMask, repoId,
		"addAndExecDockerfile"); failMsg != nil { return failMsg }
	
	var dbClient = server.dbClient
	var repo Repo = dbClient.getRepo(repoId)
	if repo == nil { return NewFailureDesc("Repo does not exist") }
	
	var desc string
	desc, err = GetRequiredPOSTFieldValue(values, "Description")
	if err != nil { return NewFailureDesc(err.Error()) }
	if desc == "" { return NewFailureDesc("No HTTP parameter found for Description") }

	var imageName string
	imageName, err = GetRequiredPOSTFieldValue(values, "ImageName")
	if err != nil { return NewFailureDesc(err.Error()) }
	if imageName == "" { return NewFailureDesc("No HTTP parameter found for ImageName") }
	
	var dockerfile Dockerfile
	dockerfile, err = createDockerfile(sessionToken, dbClient, repo, desc, values, files)
	if err != nil { return NewFailureDesc(err.Error()) }
	if dockerfile == nil { return NewFailureDesc("No dockerfile was attached") }
	
	var image DockerImage
	image, err = buildDockerfile(dockerfile, sessionToken, dbClient, values)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return image.asDockerImageDesc()
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: io.Reader
 */
func downloadImage(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func setPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	// Get the mask that we will use to overwrite the current mask.
	var partyId string
	var err error
	partyId, err = GetRequiredPOSTFieldValue(values, "PartyId")
	if err != nil { return NewFailureDesc(err.Error()) }
	var resourceId string
	resourceId, err = GetRequiredPOSTFieldValue(values, "ResourceId")
	if err != nil { return NewFailureDesc(err.Error()) }
	var smask []string = make([]string, 5)
	smask[0], err = GetRequiredPOSTFieldValue(values, "Create")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[1], err = GetRequiredPOSTFieldValue(values, "Read")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[2], err = GetRequiredPOSTFieldValue(values, "Write")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[3], err = GetRequiredPOSTFieldValue(values, "Execute")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[4], err = GetRequiredPOSTFieldValue(values, "Delete")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var mask []bool
	mask, err = ToBoolAr(smask)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, WriteMask, resourceId,
		"setPermission"); failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var dbClient DBClient = server.dbClient
	var resource Resource = dbClient.getResource(resourceId)
	if resource == nil { return NewFailureDesc("Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party = dbClient.getParty(partyId)
	if party == nil { return NewFailureDesc("Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	var aclEntry ACLEntry = party.getACLEntryForResourceId(resourceId)
	if aclEntry == nil {
		aclEntry, err = server.dbClient.dbCreateACLEntry(resourceId, partyId, mask)
		if err != nil { return NewFailureDesc(err.Error()) }
	} else {
		aclEntry.setPermissionMask(mask)
	}
	
	return aclEntry.asPermissionDesc()
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func addPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	// Get the mask that we will be adding to the current mask.
	var partyId string
	var err error
	partyId, err = GetRequiredPOSTFieldValue(values, "PartyId")
	var resourceId string
	var smask []string = make([]string, 5)
	resourceId, err = GetRequiredPOSTFieldValue(values, "ResourceId")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[0], err = GetRequiredPOSTFieldValue(values, "Create")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[1], err = GetRequiredPOSTFieldValue(values, "Read")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[2], err = GetRequiredPOSTFieldValue(values, "Write")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[3], err = GetRequiredPOSTFieldValue(values, "Execute")
	if err != nil { return NewFailureDesc(err.Error()) }
	smask[4], err = GetRequiredPOSTFieldValue(values, "Delete")
	if err != nil { return NewFailureDesc(err.Error()) }
	var mask []bool
	mask, err = ToBoolAr(smask)
	if err != nil { return NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, WriteMask, resourceId,
		"addPermission"); failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var dbClient DBClient = server.dbClient
	var resource Resource = dbClient.getResource(resourceId)
	if resource == nil { return NewFailureDesc("Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party = dbClient.getParty(partyId)
	if party == nil { return NewFailureDesc("Unable to identify party with Id " + partyId) }
	
	// Get the current ACLEntry, if there is one.
	var aclEntry ACLEntry = party.getACLEntryForResourceId(resourceId)
	if aclEntry == nil {
		aclEntry, err = server.dbClient.dbCreateACLEntry(resourceId, partyId, mask)
		if err != nil { return NewFailureDesc(err.Error()) }
	} else {
		// Add the new mask.
		var curmask []bool = aclEntry.getPermissionMask()
		for index, _ := range curmask {
			curmask[index] = curmask[index] || mask[index]
		}
	}
	
	return aclEntry.asPermissionDesc()
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId, PermissionMask
 * Returns: PermissionDesc
 */
func remPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: PartyId, ResourceId
 * Returns: PermissionDesc
 */
func getPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var partyId string
	var err error
	partyId, err = GetRequiredPOSTFieldValue(values, "PartyId")
	if err != nil { return NewFailureDesc(err.Error()) }
	var resourceId string
	resourceId, err = GetRequiredPOSTFieldValue(values, "ResourceId")
	if err != nil { return NewFailureDesc(err.Error()) }

	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, resourceId,
		"getPermission"); failMsg != nil { return failMsg }
	
	// Identify the Resource.
	var dbClient DBClient = server.dbClient
	//var resource Resource = dbClient.getResource(resourceId)
	//if resource == nil { return NewFailureDesc("Unable to identify resource with Id " + resourceId) }
	
	// Identify the Party.
	var party Party = dbClient.getParty(partyId)
	if party == nil { return NewFailureDesc("Unable to identify party with Id " + partyId) }
	
	// Return the ACLEntry.
	var aclEntry ACLEntry = party.getACLEntryForResourceId(resourceId)
	var mask []bool
	if aclEntry == nil {
		mask = make([]bool, 5)
	} else {
		mask = aclEntry.getPermissionMask()
	}
	return NewPermissionDesc(aclEntry.getId(), resourceId, partyId, mask)
}

/*******************************************************************************
 * Arguments: 
 * Returns: UserDesc
 */
func getMyDesc(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	return user.asUserDesc()
}

/*******************************************************************************
 * Arguments: 
 * Returns: []*GroupDesc
 */
func getMyGroups(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var groupDescs GroupDescs = make([]*GroupDesc, 0)
	var user User = server.dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	var groupIds []string = user.getGroupIds()
	for _, groupId := range groupIds {
		var group Group = server.dbClient.getGroup(groupId)
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
 * Returns: []*RealmDesc
 */
func getMyRealms(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var realms map[string]Realm = make(map[string]Realm)
	
	var dbClient DBClient = server.dbClient
	var user User = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	var aclEntrieIds []string = user.getACLEntryIds()
	fmt.Println("For each acl entry...")
	for _, aclEntryId := range aclEntrieIds {
		fmt.Println("\taclEntryId:", aclEntryId)
		var aclEntry ACLEntry = dbClient.getACLEntry(aclEntryId)
		var resourceId string = aclEntry.getResourceId()
		var resource Resource = dbClient.getResource(resourceId)
		switch v := resource.(type) {
			case Realm: realms[v.getId()] = v
				fmt.Println("\t\ta Realm")
			default: fmt.Println("\t\ta " + reflect.TypeOf(v).String())
		}
	}
	fmt.Println("For each realm...")
	var realmDescs RealmDescs = make([]*RealmDesc, 0)
	for _, realm := range realms {
		fmt.Println("\tappending realm", realm.getName())
		realmDescs = append(realmDescs, realm.asRealmDesc())
	}
	return realmDescs
}

/*******************************************************************************
 * Arguments: 
 * Returns: []*RepoDesc
 */
func getMyRepos(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

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
		var aclEntry ACLEntry = dbClient.getACLEntry(aclEntryId)
		var resourceId string = aclEntry.getResourceId()
		var resource Resource = dbClient.getResource(resourceId)
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
			repos[repoId] = dbClient.getRepo(repoId)
		}
	}
	fmt.Println("Creating result...")
	var repoDescs RepoDescs = make([]*RepoDesc, 0)
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
func getMyDockerfiles(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

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
		var aclEntry ACLEntry = dbClient.getACLEntry(aclEntryId)
		var resourceId string = aclEntry.getResourceId()
		var resource Resource = dbClient.getResource(resourceId)
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
			repos[repoId] = dbClient.getRepo(repoId)
		}
	}
	for _, repo := range repos {
		for _, dockerfileId := range repo.getDockerfileIds() {
			dockerfiles[dockerfileId] = dbClient.getDockerfile(dockerfileId)
		}
	}
	
	fmt.Println("Creating result...")
	var dockerfileDescs DockerfileDescs = make([]*DockerfileDesc, 0)
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
func getMyDockerImages(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

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
		var aclEntry ACLEntry = dbClient.getACLEntry(aclEntryId)
		var resourceId string = aclEntry.getResourceId()
		var resource Resource = dbClient.getResource(resourceId)
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
			repos[repoId] = dbClient.getRepo(repoId)
		}
	}
	for _, repo := range repos {
		for _, dockerImageId := range repo.getDockerImageIds() {
			dockerImages[dockerImageId] = dbClient.getDockerImage(dockerImageId)
		}
	}
	
	fmt.Println("Creating result...")
	var dockerImageDescs DockerImageDescs = make([]*DockerImageDesc, 0)
	for _, dockerImage := range dockerImages {
		fmt.Println("\tappending dockerImage", dockerImage.getName())
		dockerImageDescs = append(dockerImageDescs, dockerImage.asDockerImageDesc())
	}
	return dockerImageDescs
}

/*******************************************************************************
 * Arguments: (none)
 * Returns: ScanProviderDescs
 */
func getScanProviders(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var providerDescs ScanProviderDescs = make([]*ScanProviderDesc, 0)
	
	// For now, hard-code the scan providers.
	var params []ParameterInfo = make([]ParameterInfo, 0)
	var providerDesc *ScanProviderDesc = NewScanProviderDesc("baude", params)
	providerDescs = append(providerDescs, providerDesc)
	
	return providerDescs
}

/*******************************************************************************
 * Arguments: Name, Desc, RepoId, ProviderName, Params..., SuccessGraphicImageURL, FailureGraphicImageURL
 * Returns: ScanConfigDesc
 */
func defineScanConfig(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }
	
	var repoId string
	var err error
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	if repoId == "" { return NewFailureDesc("No HTTP parameter found for RepoId") }
	
	if failMsg := authorizeHandlerAction(server, sessionToken, WriteMask, repoId,
		"defineScanConfig"); failMsg != nil { return failMsg }
	
	var providerName string
	providerName, err = GetRequiredPOSTFieldValue(values, "ProviderName")
	if err != nil { return NewFailureDesc(err.Error()) }
	if providerName == "" { return NewFailureDesc("No HTTP parameter found for ProviderName") }
	
	var successGraphicImageURL string
	successGraphicImageURL, err = GetRequiredPOSTFieldValue(values, "SuccessGraphicImageURL")
	if err != nil { return NewFailureDesc(err.Error()) }
	if successGraphicImageURL == "" { return NewFailureDesc("No HTTP parameter found for SuccessGraphicImageURL") }
	
	var failureGraphicImageURL string
	failureGraphicImageURL, err = GetRequiredPOSTFieldValue(values, "FailureGraphicImageURL")
	if err != nil { return NewFailureDesc(err.Error()) }
	if failureGraphicImageURL == "" { return NewFailureDesc("No HTTP parameter found for FailureGraphicImageURL") }
	
	// Look for each parameter required by the provider.
	// (Right now there are none.)
	
	var scanConfig *ScanConfig
	scanConfig, err = server.client.dbCreateScanConfig(name, desc, repoId,
		providerName, paramValueIds, successGraphicImageURL, failureGraphicImageURL)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return scanConfig.asScanConfigDesc()
}

/*******************************************************************************
 * Arguments: ScanConfigId, ImageObjId
 * Returns: ScanResultDesc
 */
func scanImage(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	var scriptId, imageObjId string
	var err error
	scriptId, err = GetRequiredPOSTFieldValue(values, "ScriptId")
	if err != nil { return NewFailureDesc(err.Error()) }
	imageObjId, err = GetRequiredPOSTFieldValue(values, "ImageObjId")
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println(scriptId)
	
	if failMsg := authorizeHandlerAction(server, sessionToken, ReadMask, imageObjId,
		"scanImage"); failMsg != nil { return failMsg }
	if failMsg := authorizeHandlerAction(server, sessionToken, ExecuteMask, scriptId,
		"scanImage"); failMsg != nil { return failMsg }
	
	var dockerImage DockerImage = server.dbClient.getDockerImage(imageObjId)
	if dockerImage == nil {
		return NewFailureDesc("Docker image with object Id " + imageObjId + " not found")
	}

	// Lynis scan:
	// https://cisofy.com/lynis/
	// https://cisofy.com/lynis/plugins/docker-containers/
	// /usr/local/lynis/lynis -c --checkupdate --quiet --auditor "SafeHarbor" > ....
	
	// OpenScap using RedHat/Baude image scanner:
	// https://github.com/baude/image-scanner
	// https://github.com/baude
	// https://developerblog.redhat.com/2015/04/21/introducing-the-atomic-command/
	// https://access.redhat.com/articles/881893#get
	// https://aws.amazon.com/partners/redhat/
	// https://aws.amazon.com/marketplace/pp/B00VIMU19E
	// https://aws.amazon.com/marketplace/library/ref=mrc_prm_manage_subscriptions
	// RHEL7.1 ami at Amazon: ami-4dbf9e7d
	
	// Clair scan:
	// https://github.com/coreos/clair
	
	var cmd *exec.Cmd = exec.Command("image-scanner-remote.py",
		"--profile", "localhost", "-s", dockerImage.getDockerImageTag())
	var output []byte
	output, err = cmd.CombinedOutput()
	var outputStr string = string(output)
	if ! strings.HasPrefix(outputStr, "Error") {
		return NewFailureDesc("Image scan failed: " + outputStr)
	}

	// Tag the image as having been scanned.
	// ....
	
	return ....NewScanResultDesc(outputStr)
}

/*******************************************************************************
 * Arguments: 
 * Returns: 
 */
func getUserEvents(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: 
 * Returns: 
 */
func getImageEvents(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: 
 * Returns: 
 */
func getImageStatus(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: 
 * Returns: 
 */
func getDockerfileEvents(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}

/*******************************************************************************
 * Arguments: 
 * Returns: 
 */
func defineFlag(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if failMsg := authenticateSession(server, sessionToken); failMsg != nil { return failMsg }

	return nil
}