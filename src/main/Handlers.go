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
	"io/ioutil"
	"os"
	"strconv"
)

/*******************************************************************************
 * Arguments: Credentials
 * Returns: SessionToken
 */
func authenticate(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	//....var creds *Credentials = GetCredentials(values)

	//....return server.authenticated(creds)
	return nil
}

/*******************************************************************************
 * Arguments: none
 * Returns: Result
 */
func logout(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	


	return nil
}

/*******************************************************************************
 * Arguments: UserInfo
 * Returns: UserDesc
 */
func createUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	var err error
	var userInfo *UserInfo
	userInfo, err = GetUserInfo(values)
	
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
	var username string = userInfo.Username
	var realmId string = userInfo.RealmId
	var newUser *InMemUser
	newUser, err = server.dbClient.dbCreateUser(username, realmId)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return &UserDesc{
		Id: newUser.Id,
		Username: username,
		RealmId: realmId,
	}
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: Result
 */
func deleteUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: []GroupDesc
 */
func getMyGroups(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: GroupDesc
 */
func createGroup(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }
	
	var groupName string
	groupName, err = GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return NewFailureDesc(err.Error()) }

	var group *InMemGroup
	group, err = server.dbClient.dbCreateGroup(realmId, groupName)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	return group.asGroupDesc()
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: Result
 */
func deleteGroup(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: GroupId
 * Returns: []UserDesc
 */
func getGroupUsers(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: GroupId, UserId
 * Returns: Result
 */
func addGroupUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: GroupId, UserId
 * Returns: Result
 */
func remGroupUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: none
 * Returns: RealmDesc
 */
func createRealm(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	var err error
	var realmInfo *RealmInfo
	realmInfo, err = GetRealmInfo(values)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	fmt.Println("Creating realm ", realmInfo.Name)
	var realm *InMemRealm
	realm, err = server.dbClient.dbCreateRealm(realmInfo)
	if err != nil { return NewFailureDesc(err.Error()) }

	fmt.Println("Created realm", realmInfo.Name)
	return realm.asRealmDesc()
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: Result
 */
func deleteRealm(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId, UserId
 * Returns: Result
 */
func addRealmUser(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []GroupDesc
 */
func getRealmGroups(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId, GroupId
 * Returns: Result
 */
func addRealmGroup(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []RepoDesc
 */
func getRealmRepos(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: UserId
 * Returns: []RealmDesc
 */
func getMyRealms(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: ScanResultDesc
 */
func scanImage(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {



	// Perform a Lynis scan.
	// https://cisofy.com/lynis/
	// https://cisofy.com/lynis/plugins/docker-containers/


	return nil
}

/*******************************************************************************
 * Arguments: RealmId, <name>
 * Returns: RepoDesc
 */
func createRepo(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	fmt.Println("Creating repo...")
	var err error
	var realmId string
	realmId, err = GetRequiredPOSTFieldValue(values, "RealmId")
	if err != nil { return NewFailureDesc(err.Error()) }

	var repoName string
	repoName, err = GetRequiredPOSTFieldValue(values, "Name")
	if err != nil { return NewFailureDesc(err.Error()) }

	fmt.Println("Creating repo", repoName)
	var repo *InMemRepo
	repo, err = server.dbClient.dbCreateRepo(realmId, repoName)
	if err != nil { return NewFailureDesc(err.Error()) }
	fmt.Println("Created repo")
	return repo.asRepoDesc()
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: Result
 */
func deleteRepo(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RealmId
 * Returns: []RepoDesc
 */
func getMyRepos(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []DockerfileDesc
 */
func getDockerfiles(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId
 * Returns: []ImageDesc
 */
func getImages(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: RepoId, File
 * Returns: DockerfileDesc
 * The File argument is obtained from the values as follows:
 *    The file temp name is stored in values, keyed on "File".
 *    The name specified by the client is keyed on "Filename".
 * The handler should move the file to a permanent name.
 */
func addDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	
	fmt.Println("addDockerfile handler")
	
	printMap(values)
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
	var repoId string = values["RepoId"][0]
	if repoId == "" { return NewFailureDesc("No HTTP parameter found for RepoId") }
	var dbClient = server.dbClient
	var repo Repo = dbClient.getRepo(repoId)
	if repo == nil { return NewFailureDesc("Repo does not exist") }
	
	// Identify the user.
	var userid string = sessionToken.authenticatedUserid
	fmt.Println("userid=", userid)
	
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
		fmt.Println("********Internal error: file exists but it should not:")
		fmt.Println(filepath)
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
	var dockerfile *InMemDockerfile
	dockerfile, err = dbClient.dbCreateDockerfile(repo.getId(), filename, filepath)
	if err != nil { return NewFailureDesc(err.Error()) }
	
	// Create an ACL entry for the new file.
	dbClient.dbCreateACLEntry(dockerfile.Id, userid,
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
	return nil
}

/*******************************************************************************
 * Arguments: RepoId, DockerfileId
 * Returns: ImageDesc
 */
func buildDockerfile(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {

	// Run docker client on the server.


	return nil
}

/*******************************************************************************
 * Arguments: ImageId
 * Returns: io.Reader
 */
func downloadImage(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: userId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func setPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: userId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func addPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}

/*******************************************************************************
 * Arguments: userId or groupId, repoId or dockerfileId or imageId, PermissionMask
 * Returns: PermissionDesc
 */
func remPermission(server *Server, sessionToken *SessionToken, values url.Values,
	files map[string][]*multipart.FileHeader) RespIntfTp {
	return nil
}
