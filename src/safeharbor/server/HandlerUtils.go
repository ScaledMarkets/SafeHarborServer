/*******************************************************************************
 * Functions needed to implement the handlers in Handlers.go.
 */

package server

import (
	"mime/multipart"
	"fmt"
	"errors"
	"os"
	"strconv"
	"strings"
	"regexp"
	"net/url"
	"io/ioutil"
	"runtime/debug"	
	
	// Our packages:
	"safeharbor/apitypes"
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
	return "", errors.New("Unable to create unique file name in directory " + dir)
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
 * Parse the stdout output from a docker BUILD command. Sample output:
 
	Sending build context to Docker daemon 20.99 kB
	Sending build context to Docker daemon 
	Step 0 : FROM docker.io/cesanta/docker_auth:latest
	 ---> 3d31749deac5
	Step 1 : RUN echo moo > oink
	 ---> Using cache
	 ---> 0b8dd7a477bb
	Step 2 : FROM 41477bd9d7f9
	 ---> 41477bd9d7f9
	Step 3 : RUN echo blah > afile
	 ---> Running in 3bac4e50b6f9
	 ---> 03dcea1bc8a6
	Removing intermediate container 3bac4e50b6f9
	Successfully built 03dcea1bc8a6
 *
func ParseDockerOutput(in []byte) ([]string, error) {
	var images []string = make([]string, 1)
	var errorlist []error = make([]error, 0)
	for () {
		var line = in.getNextLine()
		if eof break
		if line begins with "Step" {
			continue
		}
		if line begins with " ---> " {
			if data following arrow only contains a hex number {
				images.append(images, <hex number>)
			}
			continue
		}
		if line begins with "Successfully built <imageid>" {
			if <imageid> != <last element of images> {
				errorlist.append(errorlist, errors.New(....))
			}
		}
		continue
	}
	if errorlist.size == 0 { errorlist = nil }
	return images, errorlist
}*/

/*******************************************************************************
 * Verify that the specified image name is valid, for an image stored within
 * the SafeHarborServer repository. Local images must be of the form,
     NAME[:TAG]
 */
func localDockerImageNameIsValid(name string) bool {
	var parts [] string = strings.Split(name, ":")
	if len(parts) > 2 { return false }
	
	for _, part := range parts {
		matched, err := regexp.MatchString("^[a-zA-Z0-9\\-_]*$", part)
		if err != nil { panic(errors.New("Unexpected internal error")) }
		if ! matched { return false }
	}
	
	return true
}

/*******************************************************************************
 * Parse the output of a docker build command and return the ID of the image
 * that was created at the end. If none found, return "".
 */
func parseImageIdFromDockerBuildOutput(outputStr string) string {
	var lines []string = strings.Split(outputStr, "\n")
	for i := len(lines)-1; i >= 0; i-- {
		var parts = strings.Split(lines[i], " ")
		if len(parts) != 3 { continue }
		if parts[0] != "Successfully" { continue }
		if parts[1] != "built" { continue }
		return strings.Trim(parts[2], " \r")
	}
	return ""
}

/*******************************************************************************
 * Verify that the specified name conforms to the name rules for images that
 * users attempt to store. We also require that a name not contain periods,
 * because we use periods to separate images into SafeHarbore namespaces within
 * a realm. If rules are satisfied, return nil; otherwise, return an error.
 */
func nameConformsToSafeHarborImageNameRules(name string) error {
	var err error = nameConformsToDockerRules(name)
	if err != nil { return err }
	if strings.Contains(name, ".") { return errors.New(
		"SafeHarbor does not allow periods in names: " + name)
	}
	return nil
}

/*******************************************************************************
 * Check that repository name component matches "[a-z0-9]+(?:[._-][a-z0-9]+)*".
 * I.e., first char is a-z or 0-9, and remaining chars (if any) are those or
 * a period, underscore, or dash. If rules are satisfied, return nil; otherwise,
 * return an error.
 */
func nameConformsToDockerRules(name string) error {
	var a = strings.TrimLeft(name, "abcdefghijklmnopqrstuvwxyz0123456789")
	var b = strings.TrimRight(a, "abcdefghijklmnopqrstuvwxyz0123456789._-")
	if len(b) == 0 { return nil }
	return errors.New("Name '" + name + "' does not conform to docker name rules: " +
		"[a-z0-9]+(?:[._-][a-z0-9]+)*  Offending fragment: '" + b + "'")
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
func authenticateSession(server *Server, sessionToken *apitypes.SessionToken,
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
		sessionToken = server.authService.identifySession(sessionId)  // returns nil if invalid
		if sessionToken == nil { return nil, apitypes.NewFailureDesc("Unauthenticated - session Id is invalid") }
	}

	if ! server.authService.sessionIdIsValid(sessionToken.UniqueSessionId) {
		return nil, apitypes.NewFailureDesc("Invalid session Id")
	}
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return nil, apitypes.NewFailureDesc("user object cannot be identified from user id " + userId)
	}
	
	return sessionToken, nil
}

/*******************************************************************************
 * Get the current authenticated user. If no one is authenticated, return nil. If
 * any other error, return an error.
 */
func getCurrentUser(server *Server, sessionToken *apitypes.SessionToken) (User, error) {
	if sessionToken == nil { return nil, nil }
	
	if ! server.authService.sessionIdIsValid(sessionToken.UniqueSessionId) {
		return nil, errors.New("Session is not valid")
	}
	
	var userId string = sessionToken.AuthenticatedUserid
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return nil, errors.New("user object cannot be identified from user id " + userId)
	}
	
	return user, nil
}

/*******************************************************************************
 * Authorize the request, based on the authenticated identity.
 */
func authorizeHandlerAction(server *Server, sessionToken *apitypes.SessionToken,
	mask []bool, resourceId, attemptedAction string) *apitypes.FailureDesc {
	
	if server.Authorize {
		
		isAuthorized, err := server.authService.authorized(server.dbClient,
			sessionToken, mask, resourceId)
		if err != nil { return apitypes.NewFailureDesc(err.Error()) }
		if ! isAuthorized {
			var resource Resource
			resource, err = server.dbClient.getResource(resourceId)
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
	if err != nil { return nil, errors.New(err.Error()) }
	
	// Create an ACL entry for the new file, to allow access by the current user.
	fmt.Println("Adding ACL entry")
	var user User = dbClient.dbGetUserByUserId(sessionToken.AuthenticatedUserid)
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
	if len(headers) > 1 { return "", "", errors.New("Too many files posted") }
	var header *multipart.FileHeader = headers[0]
	var filename string = header.Filename	
	fmt.Println("Filename:", filename)
	
	var file multipart.File
	file, err = header.Open()
	if err != nil { return "", "", errors.New(err.Error()) }
	if file == nil { return "", "", errors.New("Internal Error") }	
	
	// Create a filename for the new file.
	var filepath = repo.getFileDirectory() + "/" + filename
	if fileExists(filepath) {
		filepath, err = createUniqueFilename(repo.getFileDirectory(), filename)
		if err != nil {
			fmt.Println(err.Error())
			return "", "", errors.New(err.Error())
		}
	}
	if fileExists(filepath) {
		fmt.Println("********Internal error: file exists but it should not:" + filepath)
		return "", "", errors.New("********Internal error: file exists but it should not:" + filepath)
	}
	
	// Save the file data to a permanent file.
	var bytes []byte
	bytes, err = ioutil.ReadAll(file)
	err = ioutil.WriteFile(filepath, bytes, os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
		return "", "", errors.New("While writing dockerfile, " + err.Error())
	}
	fmt.Println(strconv.FormatInt(int64(len(bytes)), 10), "bytes written to file", filepath)
	return filename, filepath, nil
}

/*******************************************************************************
 * 
 */
func buildDockerfile(server *Server, dockerfile Dockerfile, sessionToken *apitypes.SessionToken,
	dbClient DBClient, values url.Values) (DockerImage, error) {

	var repo Repo
	var err error
	repo, err = dockerfile.getRepo()
	if err != nil { return nil, err }
	var realm Realm
	realm, err = repo.getRealm()
	if err != nil { return nil, err }
	
	var user User
	user, err = getCurrentUser(server, sessionToken)
	if err != nil { return nil, err }

	var imageName string
	imageName, err = apitypes.GetRequiredHTTPParameterValue(values, "ImageName")
	if err != nil { return nil, err }
	if imageName == "" { return nil, errors.New("No HTTP parameter found for ImageName") }
	
	var outputStr string
	outputStr, err = server.DockerService.BuildDockerfile(dockerfile, realm, repo, imageName)
	if err != nil { return nil, err }
	
	// Add a record for the image to the database.
	// (This automatically computes the signature.)
	var image DockerImage
	image, err = dbClient.dbCreateDockerImage(repo.getId(),
		imageName, dockerfile.getDescription(), outputStr)
	fmt.Println("Created docker image object.")
	
	// Create an event to record that this happened.
	_, err = dbClient.dbCreateDockerfileExecEvent(dockerfile.getId(), image.getId(), user.getId())
	
	return image, nil
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
			default: return nil, errors.New("Internal error: unexpected repository object type")
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
			if r == nil { return nil, errors.New("No repo found for Id " + repoId) }
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
			default: return nil, errors.New("Internal error: unexpected repository object type")
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
		if d == nil { return errors.New("Internal Error: No dockerfile found for Id " + dockerfileId) }
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
		if d == nil { return errors.New("Internal Error: No docker image found for Id " + id) }
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
		if d == nil { return errors.New("Internal Error: No scan config found for Id " + id) }
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
		if d == nil { return errors.New("Internal Error: No flag found for Id " + id) }
		leaves[id] = d
	}
	return nil
}
