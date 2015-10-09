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
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
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
	
	return NewResult(200, "Persistent state reset")
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
	return server.authService.authenticateCredentials(creds)
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
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var userInfo *UserInfo
	userInfo, err = GetUserInfo(values)
	
	// Authorize the request, based on the authenticated identity.
	if ! server.authService.authorized(server.sessions[sessionToken.UniqueSessionId],
		"admin",  // this 'resource' is onwed by the admin account
		"repository",
		"*",  // the scope is the entire repository
		[]string{"create-user"}) { // this is the action that is being requested
	
		//"registry.docker.com", "repository:samalba/my-app:push", "jlhawn")
		fmt.Println("Unauthorized: %s, %s, %s")
		return nil
	}
	
	// Create the user account.
	var newUserId string = userInfo.UserId
	var newUserName string = userInfo.UserName
	var realmId string = userInfo.RealmId
	var newUser User
	newUser, err = server.dbClient.dbCreateUser(newUserId, newUserName, realmId)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return &UserDesc{
		Id: newUser.getId(),
		UserId: newUserId,
		UserName: newUserName,
		RealmId: realmId,
	}
}

/*******************************************************************************
 * Arguments: UserObjId
 * Returns: Result
 */
func deleteUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }
	
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
 * Arguments: 
 * Returns: UserDesc
 */
func getMyInfo(server *Server, sessionToken *SessionToken, values url.Values,
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

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }
	
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
 * Arguments: RealmId, <name>
 * Returns: GroupDesc
 */
func createGroup(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var groupName string
	groupName, err = GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return NewFailureDesc(err.Error()) }

	var group Group
	group, err = server.dbClient.dbCreateGroup(realmId, groupName)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	// Add ACL entry to enable the current user to access what he/she just created.
	server.dbClient.dbCreateACLEntry(group.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	
	return group.asGroupDesc()
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: Result
 */
func deleteGroup(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: GroupId
 * Returns: []*UserDesc
 */
func getGroupUsers(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var groupId string
	groupId, err = GetRequiredPOSTFieldValue(values, "GroupId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
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
	
	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var groupId string
	groupId, err = GetRequiredPOSTFieldValue(values, "GroupId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var userObjId string
	userObjId, err = GetRequiredPOSTFieldValue(values, "UserObjId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var group Group = server.dbClient.getGroup(groupId)
	if group == nil { return NewFailureDesc(fmt.Sprintf(
		"No group with Id %s", groupId))
	}

	err = group.addUserId(userObjId)
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

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }
	
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
 * Arguments: none
 * Returns: RealmDesc
 */
func createRealm(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var realmInfo *RealmInfo
	realmInfo, err = GetRealmInfo(values)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	fmt.Println("Creating realm ", realmInfo.Name)
	var realm Realm
	realm, err = server.dbClient.dbCreateRealm(realmInfo)
	if err != nil { return NewFailureDesc(err.Error()) }

	// Add ACL entry to enable the current user to access what he/she just created.
	server.dbClient.dbCreateACLEntry(realm.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	
	fmt.Println("Created realm", realmInfo.Name)
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: Result
 */
func deleteRealm(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: RealmId, UserObjId
 * Returns: Result
 */
func addRealmUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var realmId string
	var userObjId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	userObjId, err = GetRequiredPOSTFieldValue(values, "UserObjId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
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

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: RealmId, UserId
 * Returns: UserDesc
 */
func getRealmUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var realmId string
	var realmUserId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	realmUserId, err = GetRequiredPOSTFieldValue(values, "UserId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
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
 * Returns: []*GroupDesc
 */
func getRealmGroups(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: RealmId
 * Returns: []*RepoDesc
 */
func getRealmRepos(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
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
 * Arguments: 
 * Returns: []*RealmDesc
 */
func getMyRealms(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var realms map[string]Realm = make(map[string]Realm)
	
	var dbClient DBClient = server.dbClient
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
 * Arguments: (none)
 * Returns: []*RealmDesc
 */
func getAllRealms(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

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
 * Arguments: ImageId
 * Returns: ScanResultDesc
 */
func scanImage(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	// https://github.com/baude/image-scanner
	// https://github.com/baude
	
	// Perform a Lynis scan.
	// https://cisofy.com/lynis/
	// https://cisofy.com/lynis/plugins/docker-containers/
	// /usr/local/lynis/lynis -c --checkupdate --quiet --auditor "SafeHarbor" > ....

	return nil
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: RepoDesc
 */
func createRepo(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	fmt.Println("Creating repo...")
	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }

	var repoName string
	repoName, err = GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return NewFailureDesc(err.Error()) }

	fmt.Println("Creating repo", repoName)
	var repo Repo
	repo, err = server.dbClient.dbCreateRepo(realmId, repoName)
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println("Created repo")

	// Add ACL entry to enable the current user to access what he/she just created.
	server.dbClient.dbCreateACLEntry(repo.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: Result
 */
func deleteRepo(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: 
 * Returns: []*RepoDesc
 */
func getMyRepos(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}
	
	// Traverse the user's ACL entries; form the union of the repos that the user
	// has explicit access to, and the repos that belong to the realms that the user
	// has access to.
	
	var realms map[string]Realm = make(map[string]Realm)
	var repos map[string]Repo = make(map[string]Repo)
	
	var dbClient DBClient = server.dbClient
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
 * Arguments: RepoId
 * Returns: []*DockerfileDesc
 */
func getDockerfiles(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var repoId string
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
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

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	var err error
	var repoId string
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
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
	
	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	fmt.Println("addDockerfile handler")
	
	//printMap(values)
	//printFileMap(files)
	
	var headers []*multipart.FileHeader = files["filename"]
	if len(headers) == 0 { return NewFailureDesc("No file posted") }
	if len(headers) > 1 { return NewFailureDesc("Too many files posted") }
	
	var header *multipart.FileHeader = headers[0]
	var filename string = header.Filename	
	fmt.Println("Filename:", filename)
	
	var file multipart.File
	var err error
	file, err = header.Open()
	if err != nil { return NewFailureDesc(err.Error()) }
	if file == nil { return NewFailureDesc("Internal Error") }	
	
	// Identify the repo.
	var repoId string
	repoId, err = GetRequiredPOSTFieldValue(values, "RepoId")
	if err != nil { return NewFailureDesc(err.Error()) }
	if repoId == "" { return NewFailureDesc("No HTTP parameter found for RepoId") }
	var dbClient = server.dbClient
	var repo Repo = dbClient.getRepo(repoId)
	if repo == nil { return NewFailureDesc("Repo does not exist") }
	
	// Verify that the user is authorized to add to the repo.
	//....TBD
	
	

	
	// Create a filename for the new file.
	var filepath = repo.getFileDirectory() + "/" + filename
	if fileExists(filepath) {
		filepath, err = createUniqueFilename(repo.getFileDirectory(), filename)
		if err != nil {
			fmt.Println(err.Error())
			return NewFailureDesc(err.Error())
		}
	}
	if fileExists(filepath) {
		fmt.Println("********Internal error: file exists but it should not:" + filepath)
		return NewFailureDesc("********Internal error: file exists but it should not:" + filepath)
	}
	
	// Save the file data to a permanent file.
	var bytes []byte
	bytes, err = ioutil.ReadAll(file)
	err = ioutil.WriteFile(filepath, bytes, os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
		return NewFailureDesc(err.Error())
	}
	fmt.Println(strconv.FormatInt(int64(len(bytes)), 10), "bytes written to file", filepath)
	
	// Add the file to the specified repo's set of Dockerfiles.
	var dockerfile Dockerfile
	dockerfile, err = dbClient.dbCreateDockerfile(repo.getId(), filename, filepath)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	// Create an ACL entry for the new file, to allow access by the current user.
	fmt.Println("Adding ACL entry")
	dbClient.dbCreateACLEntry(dockerfile.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	fmt.Println("Created ACL entry")
	
	return dockerfile.asDockerfileDesc()
}

/*******************************************************************************
 * Arguments: RepoId, DockerfileId, File
 * Returns: Result
 */
func replaceDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: RepoId, DockerfileId, ImageName
 * Returns: DockerfileImageDesc
 */
func execDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

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
	var dbClient = server.dbClient
	var dockerfile Dockerfile = dbClient.getDockerfile(dockerfileId)
	var dockerfileName string = dockerfile.getName()
	fmt.Println("Dockerfile name =", dockerfileName)

	var imageName string
	imageName, err = GetRequiredPOSTFieldValue(values, "ImageName")
	if err != nil { return NewFailureDesc(err.Error()) }
	if imageName == "" { return NewFailureDesc("No HTTP parameter found for ImageName") }
	if ! localDockerImageNameIsValid(imageName) {
		return NewFailureDesc(fmt.Sprintf("Image name '%s' is not valid - must be " +
			"of format <name>[:<tag>]", imageName))
	}
	fmt.Println("Image name =", imageName)
	
	// Check if am image with that name already exists.
	var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "inspect", imageName)
	var output []byte
	output, err = cmd.CombinedOutput()
	var outputStr string = string(output)
	if ! strings.HasPrefix(outputStr, "Error") {
		return NewFailureDesc("An image with name " + imageName + " already exists.")
	}
	
	// Verify that the image name conforms to Docker's requirements.
	err = nameConformsToSafeHarborImageNameRules(imageName)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	// Create a temporary directory to serve as the build context.
	var tempDirPath string
	tempDirPath, err = ioutil.TempDir("", "")
	defer os.RemoveAll(tempDirPath)
	fmt.Println("Temp directory = ", tempDirPath)

	// Copy dockerfile to that directory.
	var in, out *os.File
	in, err = os.Open(dockerfile.getFilePath())
	if err != nil { return NewFailureDesc(err.Error()) }
	out, err = os.Create(tempDirPath + "/" + dockerfileName)
	if err != nil { return NewFailureDesc(err.Error()) }
	_, err = io.Copy(out, in)
	if err != nil { return NewFailureDesc(err.Error()) }
	err = out.Close()
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println("Copied Dockerfile to temp directory")
		
	err = os.Chdir(tempDirPath)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	// Create a the docker build command.
	// https://docs.docker.com/reference/commandline/build/
	// REPOSITORY                      TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
	// docker.io/cesanta/docker_auth   latest              3d31749deac5        3 months ago        528 MB
	// Image id format: <hash>[:TAG]
	
	cmd = exec.Command("/usr/bin/docker", "build",
    	"--file", dockerfileName, "--tag", imageName, tempDirPath)
	
	// Execute the command in the temporary directory.
	// This initiates processing of the dockerfile.
	output, err = cmd.CombinedOutput()
	outputStr = string(output)
	fmt.Println("...finished processing dockerfile.")
	fmt.Println(outputStr)
	if err != nil { return NewFailureDesc(err.Error() + ", " + outputStr) }
	fmt.Println("Performed docker build command successfully.")
	
	// Add a record for the image to the database.
	var image DockerImage
	image, err = dbClient.dbCreateDockerImage(repoId, imageName)
	fmt.Println("Created docker image object.")
	
	return image.asDockerImageDesc()
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: io.Reader
 */
func downloadImage(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: userObjId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func setPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }
	
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
 * Arguments: userObjId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func addPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

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
 * Arguments: userObjId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func remPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	if sessionToken == nil { return NewFailureDesc("Unauthenticated") }

	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return NewFailureDesc("user object cannot be identified from user id " + userId)
	}

	return nil
}
