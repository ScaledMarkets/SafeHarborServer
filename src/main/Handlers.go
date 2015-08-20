/*******************************************************************************
 * All of the REST handlers are contained here. These functions are called by
 * the ReqHandler.
 */

package main

import (
	"net/url"
	"fmt"
)

/*******************************************************************************
 * Arguments: Credentials
 * Returns: SessionToken
 */
func authenticate(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	
	var creds *Credentials = GetCredentials(values)

	return server.authenticated(creds)
}

/*******************************************************************************
 * Arguments: none
 * Returns: Result
 */
func logout(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	
	


	return nil
}

/*******************************************************************************
 * Arguments: UserInfo
 * Returns: UserDesc
 */
func createUser(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {

	var userInfo *UserInfo = GetUserInfo(values)
	
	// Authorize the request, based on the authenticated identity.
	if ! server.authorized(server.sessions[sessionToken.UniqueSessionId],
		"admin",  // this 'resource' is onwed by the admin account
		"repository",
		"*",  // the scope is the entire repository
		[]string{"create-user"}) { // this is the action that is being requested
	
		//"registry.docker.com", "repository:samalba/my-app:push", "jlhawn")
		fmt.Println("Unauthorized: %s, %s, %s")
		return nil
	}
	
	// Create the user account.
	var userid string = userInfo.Id
	//....
	
	return &UserDesc{
		UserId: userid,
	}
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: Result
 */
func deleteUser(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: []GroupDesc
 */
func getMyGroups(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: GroupDesc
 */
func createGroup(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: Result
 */
func deleteGroup(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: []UserDesc
 */
func getGroupUsers(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: GroupId, UserId
 * Returns: Result
 */
func addGroupUser(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: GroupId, UserId
 * Returns: Result
 */
func remGroupUser(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: none
 * Returns: RealmDesc
 */
func createRealm(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	var realmInfo *RealmInfo = GetRealmInfo(values)
	if realmInfo == nil { fmt.Println("realmInfo is nil") }
	fmt.Println("Creating realm ", realmInfo.Name)
	var realm *InMemRealm = server.dbClient.dbCreateRealm(realmInfo)
	fmt.Println("Created realm", realmInfo.Name)
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: Result
 */
func deleteRealm(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId, UserId
 * Returns: Result
 */
func addRealmUser(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []GroupDesc
 */
func getRealmGroups(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId, GroupId
 * Returns: Result
 */
func addRealmGroup(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []RepoDesc
 */
func getRealmRepos(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: []RealmDesc
 */
func getMyRealms(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: ScanResultDesc
 */
func scanImage(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: RepoDesc
 */
func createRepo(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	fmt.Println("Creating repo")
	var realmId string = GetRequiredPOSTFieldValue(values, "RealmId")
	var repoName string = GetRequiredPOSTFieldValue(values, "Name")
	fmt.Println("Creating repo ", repoName)
	var repo *InMemRepo = server.dbClient.dbCreateRepo(realmId, repoName)
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: Result
 */
func deleteRepo(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []RepoDesc
 */
func getMyRepos(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []DockerfileDesc
 */
func getDockerfiles(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []ImageDesc
 */
func getImages(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId, File
 * Returns: DockerfileDesc
 */
func addDockerfile(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId, DockerfileId, File
 * Returns: Result
 */
func replaceDockerfile(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId, DockerfileId
 * Returns: ImageDesc
 */
func buildDockerfile(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: io.Reader
 */
func downloadImage(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: ImageId, AccountDesc
 * Returns: Result
 */
func sendImage(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: userId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func setPermission(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: userId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func addPermission(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}

/*******************************************************************************
 * Arguments: userId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func remPermission(server *Server, sessionToken *SessionToken, values url.Values) ResponseInterfaceType {
	return nil
}
