/*******************************************************************************
 * Functions needed to implement the handlers in Handlers.go.
 */

package server

import (
	"mime/multipart"
	"fmt"
	//"errors"
	"os"
	"strconv"
	"strings"
	"net/url"
	"io/ioutil"
	"runtime/debug"	
	
	// SafeHarbor packages:
	"safeharbor/apitypes"
	"safeharbor/docker"
	"safeharbor/util"
)

/*******************************************************************************
 * Create a filename that is unique within the specified directory. Derive the
 * file name from the specified base name.
 */
func createUniqueFilename(dir string, basename string) (string, error) {
	var filepath = dir + "/" + basename
	for i := 0; i < 1000; i++ {
		var p string = filepath + strconv.FormatInt(int64(i), 10)
		if ! fileExists(p) {
			return p, nil
		}
	}
	return "", util.ConstructError("Unable to create unique file name in directory " + dir)
}

/*******************************************************************************
 * 
 */
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return (err == nil)
}

/*******************************************************************************
 * Write the specified map to stdout. This is a diagnostic method.
 */
func PrintMap(m map[string][]string) {
	fmt.Println("Map:")
	for k, v := range m {
		fmt.Println(k, ":")
		for i := range v {
			fmt.Println("\t", v[i])
		}
	}
}

/*******************************************************************************
 * Write the specified map to stdout. This is a diagnostic method.
 */
func printFileMap(m map[string][]*multipart.FileHeader) {
	fmt.Println("FileHeader Map:")
	for k, headers := range m {
		fmt.Println("Name:", k, "FileHeaders:")
		for i := range headers {
			fmt.Println("Filename:", headers[i].Filename)
			PrintMap(headers[i].Header)
			fmt.Println()
		}
	}
}

/*******************************************************************************
 * Verify that the specified name conforms to the name rules for images that
 * users attempt to store. We also require that a name not contain periods,
 * because we use periods to separate images into SafeHarbore namespaces within
 * a realm. If rules are satisfied, return nil; otherwise, return an error.
 */
func nameConformsToSafeHarborImageNameRules(name string) error {
	var err error = docker.NameConformsToDockerRules(name)
	if err != nil { return err }
	if strings.Contains(name, ".") { return util.ConstructError(
		"SafeHarbor does not allow periods in names: " + name)
	}
	return nil
}

/*******************************************************************************
 * Verify that the file name is a valid simple file name (not including a
 * directory path) and that it only contains characters that are a valid in a
 * file name in SafeHarbor''s various file repositories (for dockerfiles, image
 * files, etc.)
 */
func validateSimpleFileNameSyntax(name string) error {
	_, err := apitypes.Sanitize(name)
	if err != nil { return err }
	return nil
}

/*******************************************************************************
 * If the specified condition is not true, then thrown an exception with the message.
 */
func assertThat(condition bool, msg string) bool {
	if ! condition {
		var s string = fmt.Sprintf("ERROR: %s", msg)
		fmt.Println(s)
		debug.PrintStack()
	}
	return condition
}

/*******************************************************************************
 * 
 */
func AssertErrIsNil(err error, msg string) bool {
	if err == nil { return true }
	fmt.Print(msg)
	debug.PrintStack()
	return false
}

/*******************************************************************************
 * Authenticate the session, using either the session token or an HTTP parameter
 * that provides a valid session Id.
 */
func authenticateSession(dbClient *InMemClient, sessionToken *apitypes.SessionToken,
	values url.Values) (*apitypes.SessionToken, *apitypes.FailureDesc) {
	
	if sessionToken == nil {

		// no session Id found; see if it was sent as an HTTP parameter.
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
		if ! found { return nil, apitypes.NewFailureDesc("Unauthenticated - no session Id found") }
		sessionId = valuear[0]
		if sessionId == "" { return nil, apitypes.NewFailureDesc("Unauthenticated - session Id appears to be malformed") }
		sessionToken = dbClient.getServer().authService.identifySession(sessionId)  // returns nil if invalid
		if sessionToken == nil { return nil, apitypes.NewFailureDesc("Unauthenticated - session Id is invalid") }
	}

	if ! dbClient.getServer().authService.sessionIdIsValid(sessionToken.UniqueSessionId) {
		return nil, apitypes.NewFailureDesc("Invalid session Id")
	}
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return nil, apitypes.NewFailureDesc(err.Error()) }
	if user == nil {
		return nil, apitypes.NewFailureDesc("user object cannot be identified from user id " + userId)
	}
	
	return sessionToken, nil
}

/*******************************************************************************
 * Get the current authenticated user. If no one is authenticated, return nil. If
 * any other error, return an error.
 */
func getCurrentUser(dbClient DBClient, sessionToken *apitypes.SessionToken) (User, error) {
	if sessionToken == nil { return nil, nil }
	
	if ! dbClient.getServer().authService.sessionIdIsValid(sessionToken.UniqueSessionId) {
		return nil, util.ConstructError("Session is not valid")
	}
	
	var userId string = sessionToken.AuthenticatedUserid
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return nil, err }
	if user == nil {
		return nil, util.ConstructError("user object cannot be identified from user id " + userId)
	}
	
	return user, nil
}

/*******************************************************************************
 * Authorize the request, based on the authenticated identity.
 */
func authorizeHandlerAction(dbClient *InMemClient, sessionToken *apitypes.SessionToken,
	mask []bool, resourceId, attemptedAction string) *apitypes.FailureDesc {
	
	if dbClient.getServer().Authorize {
		
		isAuthorized, err := dbClient.getServer().authService.authorized(dbClient,
			sessionToken, mask, resourceId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if ! isAuthorized {
			var resource Resource
			resource, err = dbClient.getResource(resourceId)
			if err != nil { return apitypes.NewFailureDesc(err.Error()) }
			if resource == nil {
				return apitypes.NewFailureDesc("Unable to identify resource with Id " + resourceId)
			}
			return apitypes.NewFailureDesc(fmt.Sprintf(
				"Unauthorized: cannot perform %s on %s", attemptedAction, resource.getName()))
		}
	}
	
	return nil
}

/*******************************************************************************
 * 
 */
func createDockerfile(sessionToken *apitypes.SessionToken, dbClient DBClient,
	repo Repo, name, filepath, desc string) (Dockerfile, error) {
	
	// Add the file to the specified repo's set of Dockerfiles.
	var dockerfile Dockerfile
	var err error
	dockerfile, err = dbClient.dbCreateDockerfile(repo.getId(), name, desc, filepath)
	if err != nil { return nil, util.ConstructError(err.Error()) }
	
	// Create an ACL entry for the new file, to allow access by the current user.
	fmt.Println("Adding ACL entry")
	var user User
	user, err = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
	if err != nil { return dockerfile, err }
	_, err = dbClient.dbCreateACLEntry(dockerfile.getId(), user.getId(),
		[]bool{ true, true, true, true, true } )
	if err != nil { return dockerfile, err }
	fmt.Println("Created ACL entry")
	
	return dockerfile, nil
}

/*******************************************************************************
 * 
 */
func captureFile(repo Repo, files map[string][]*multipart.FileHeader) (string, string, error) {

	var err error
	var headers []*multipart.FileHeader = files["filename"]
	if len(headers) == 0 { return "", "", nil }
	if len(headers) > 1 { return "", "", util.ConstructError("Too many files posted") }
	var header *multipart.FileHeader = headers[0]
	var filename string = header.Filename	
	fmt.Println("Filename:", filename)
	
	// Validate syntax of filename: must be a simple name - no slashes, and a valid file name
	err = validateSimpleFileNameSyntax(filename)
	if err != nil { return "", "", util.ConstructError(err.Error()) }
	
	var file multipart.File
	file, err = header.Open()
	if err != nil { return "", "", util.ConstructError(err.Error()) }
	if file == nil { return "", "", util.ConstructError("Internal Error") }	
	
	// Create a filename for the new file.
	var filepath = repo.getFileDirectory() + "/" + filename
	if fileExists(filepath) {
		filepath, err = createUniqueFilename(repo.getFileDirectory(), filename)
		if err != nil {
			fmt.Println(err.Error())
			return "", "", util.ConstructError(err.Error())
		}
	}
	if fileExists(filepath) {
		fmt.Println("********Internal error: file exists but it should not:" + filepath)
		return "", "", util.ConstructError("********Internal error: file exists but it should not:" + filepath)
	}
	
	// Save the file data to a permanent file.
	var bytes []byte
	bytes, err = ioutil.ReadAll(file)
	err = ioutil.WriteFile(filepath, bytes, os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
		return "", "", util.ConstructError("While writing dockerfile, " + err.Error())
	}
	fmt.Println(strconv.FormatInt(int64(len(bytes)), 10), "bytes written to file", filepath)
	return filename, filepath, nil
}

/*******************************************************************************
 * 
 */
func buildDockerfile(dbClient DBClient, dockerfile Dockerfile, sessionToken *apitypes.SessionToken,
	values url.Values) (DockerImage, error) {

	fmt.Println("buildDockerfile: A")  // debug
	var repo Repo
	var err error
	repo, err = dockerfile.getRepo(dbClient)
	fmt.Println("buildDockerfile: B")  // debug
	if err != nil { return nil, err }
	var realm Realm
	realm, err = repo.getRealm(dbClient)
	fmt.Println("buildDockerfile: C")  // debug
	if err != nil { return nil, err }
	
	var user User
	user, err = getCurrentUser(dbClient, sessionToken)
	fmt.Println("buildDockerfile: D")  // debug
	if err != nil { return nil, err }

	var imageName string
	imageName, err = apitypes.GetRequiredHTTPParameterValue(true, values, "ImageName")
	fmt.Println("buildDockerfile: E")  // debug
	if err != nil { return nil, err }
	if imageName == "" { return nil, util.ConstructError("No HTTP parameter found for ImageName") }
	
	var outputStr string
	err = nameConformsToSafeHarborImageNameRules(imageName)
	fmt.Println("buildDockerfile: F")  // debug
	if err != nil { return nil, err }
	outputStr, err = dbClient.getServer().DockerServices.BuildDockerfile(
		dockerfile.getExternalFilePath(), dockerfile.getName(), realm.getName(),
		repo.getName(), imageName)
	fmt.Println("buildDockerfile: G")  // debug
	if err != nil { return nil, err }
	
	var dockerBuildOutput *docker.DockerBuildOutput
	dockerBuildOutput, err = docker.ParseBuildRESTOutput(outputStr)
	fmt.Println("buildDockerfile: H")  // debug
	if err != nil { return nil, err }
	fmt.Println("buildDockerfile: H1")  // debug
	fmt.Println("dockerBuildOutput=" + outputStr) // debug
	fmt.Println("buildDockerfile: H2")  // debug
	var dockerImageId string = dockerBuildOutput.GetFinalDockerImageId()
	
	var digest []byte
	digest, err = docker.GetDigest(dockerImageId)
	fmt.Println("buildDockerfile: I")  // debug
	if err != nil { return nil, err }
	
	// Add a record for the image to the database.
	// (This automatically computes the signature.)
	var image DockerImage
	image, err = dbClient.dbCreateDockerImage(repo.getId(),
		imageName, dockerfile.getDescription(), digest, outputStr)
	fmt.Println("Created docker image object.")
	
	// Create an event to record that this happened.
	_, err = dbClient.dbCreateDockerfileExecEvent(dockerfile.getId(), image.getId(), user.getId())
	fmt.Println("buildDockerfile: J")  // debug
	
	return image, err
}

/*******************************************************************************
 * Create a map of the leaf resources (resources that are not containers for other
 * resources) that the specified user has access to. The map is keyed on each
 * resource''s object Id.
 */
func getLeafResources(dbClient DBClient, user User,
	leafType ResourceType) (map[string]Resource, error) {
	
	var realms map[string]Realm = make(map[string]Realm)
	var repos map[string]Repo = make(map[string]Repo)
	var leaves map[string]Resource = make(map[string]Resource)
	
	// Add leaves for which there are direct entries, and while doing that,
	// create a list of realms and repos that the user has access to.
	var aclEntrieIds []string = user.getACLEntryIds()
	var err error
	for _, aclEntryId := range aclEntrieIds {
		fmt.Println("\taclEntryId:", aclEntryId)
		var aclEntry ACLEntry
		aclEntry, err = dbClient.getACLEntry(aclEntryId)
		if err != nil { return nil, err }
		var resourceId string = aclEntry.getResourceId()
		var resource Resource
		resource, err = dbClient.getResource(resourceId)
		if err != nil { return nil, err }
		switch v := resource.(type) {
			case Realm: realms[v.getId()] = v
				fmt.Println("\t\ta Realm")
			case Repo: repos[v.getId()] = v
				fmt.Println("\t\ta Repo")
			case Dockerfile: if leafType == ADockerfile { leaves[v.getId()] = v }
			case DockerImage: if leafType == ADockerImage { leaves[v.getId()] = v }
			case ScanConfig: if leafType == AScanConfig { leaves[v.getId()] = v }
			case Flag: if leafType == AFlag { leaves[v.getId()] = v }
			default: return nil, util.ConstructError("Internal error: unexpected repository object type")
		}
	}
	// Create composite list of repos that the user has access to, either directly
	// or as a result of having access to the owning realm.
	for _, realm := range realms {
		fmt.Println("For each repo of realm id", realm.getId(), "...")
		// Add all of the repos belonging to realm.
		for _, repoId := range realm.getRepoIds() {
			fmt.Println("\tadding repoId", repoId)
			var r Repo
			var err error
			r, err = dbClient.getRepo(repoId)
			if err != nil { return nil, err }
			if r == nil { return nil, util.ConstructError("No repo found for Id " + repoId) }
			repos[repoId] = r
		}
	}
	// Add the leaves that belong to each of those repos.
	for _, repo := range repos {
		switch leafType {
			case ADockerfile: err = mapRepoDockerfileIds(dbClient, repo, leaves)
			case ADockerImage: err = mapRepoDockerImageIds(dbClient, repo, leaves)
			case AScanConfig: err = mapRepoScanConfigIds(dbClient, repo, leaves)
			case AFlag: err = mapRepoFlagIds(dbClient, repo, leaves)
			default: return nil, util.ConstructError("Internal error: unexpected repository object type")
		}
		if err != nil { return nil, err }
	}
	
	return leaves, nil
}

func mapRepoDockerfileIds(dbClient DBClient, repo Repo, leaves map[string]Resource) error {
		
	for _, dockerfileId := range repo.getDockerfileIds() {
		var d Dockerfile
		var err error
		d, err = dbClient.getDockerfile(dockerfileId)
		if err != nil { return err }
		if d == nil { return util.ConstructError("Internal Error: No dockerfile found for Id " + dockerfileId) }
		leaves[dockerfileId] = d
	}
	return nil
}

func mapRepoDockerImageIds(dbClient DBClient, repo Repo, leaves map[string]Resource) error {
		
	for _, id := range repo.getDockerImageIds() {
		var d DockerImage
		var err error
		d, err = dbClient.getDockerImage(id)
		if err != nil { return err }
		if d == nil { return util.ConstructError("Internal Error: No docker image found for Id " + id) }
		leaves[id] = d
	}
	return nil
}

func mapRepoScanConfigIds(dbClient DBClient, repo Repo, leaves map[string]Resource) error {
		
	for _, id := range repo.getScanConfigIds() {
		var d ScanConfig
		var err error
		d, err = dbClient.getScanConfig(id)
		if err != nil { return err }
		if d == nil { return util.ConstructError("Internal Error: No scan config found for Id " + id) }
		leaves[id] = d
	}
	return nil
}

func mapRepoFlagIds(dbClient DBClient, repo Repo, leaves map[string]Resource) error {
		
	for _, id := range repo.getFlagIds() {
		var d Flag
		var err error
		d, err = dbClient.getFlag(id)
		if err != nil { return err }
		if d == nil { return util.ConstructError("Internal Error: No flag found for Id " + id) }
		leaves[id] = d
	}
	return nil
}
