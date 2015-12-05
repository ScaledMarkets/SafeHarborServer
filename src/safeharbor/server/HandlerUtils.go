/*******************************************************************************
 * Functions needed to implement the handlers in Handlers.go.
 */

package server

import (
	"mime/multipart"
	"fmt"
	"errors"
	"os"
	"os/exec"
	"io"
	"strconv"
	"strings"
	"regexp"
	"net/url"
	"io/ioutil"
	
	"safeharbor/apitypes"
	//"rest"
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
func assertThat(condition bool, msg string) {
	if ! condition {
		var s string = fmt.Sprintf("ERROR: %s", msg)
		fmt.Println(s)
		panic(errors.New(s))
	}
}

/*******************************************************************************
 * 
 */
func assertErrIsNil(err error, msg string) {
	if err == nil { return }
	fmt.Print(msg)
	panic(err)
}

/*******************************************************************************
 * Authenticate the session token.
 */
func authenticateSession(server *Server, sessionToken *apitypes.SessionToken) *apitypes.FailureDesc {
	
	if sessionToken == nil { return apitypes.NewFailureDesc("Unauthenticated") }

	if ! server.authService.sessionIdIsValid(sessionToken.UniqueSessionId) {
		return apitypes.NewFailureDesc("Invalid session Id")
	}
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return apitypes.NewFailureDesc("user object cannot be identified from user id " + userId)
	}
	
	return nil
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
		isAuthorized, err := authorized(server, sessionToken, mask, resourceId)
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
func createDockerfile(sessionToken *apitypes.SessionToken, dbClient DBClient, repo Repo,
	desc string, values url.Values, files map[string][]*multipart.FileHeader) (Dockerfile, error) {
	
	var headers []*multipart.FileHeader = files["filename"]
	if len(headers) == 0 { return nil, nil }
	if len(headers) > 1 { return nil, errors.New("Too many files posted") }
	
	var header *multipart.FileHeader = headers[0]
	var filename string = header.Filename	
	fmt.Println("Filename:", filename)
	
	var file multipart.File
	var err error
	file, err = header.Open()
	if err != nil { return nil, errors.New(err.Error()) }
	if file == nil { return nil, errors.New("Internal Error") }	
	
	// Create a filename for the new file.
	var filepath = repo.getFileDirectory() + "/" + filename
	if fileExists(filepath) {
		filepath, err = createUniqueFilename(repo.getFileDirectory(), filename)
		if err != nil {
			fmt.Println(err.Error())
			return nil, errors.New(err.Error())
		}
	}
	if fileExists(filepath) {
		fmt.Println("********Internal error: file exists but it should not:" + filepath)
		return nil, errors.New("********Internal error: file exists but it should not:" + filepath)
	}
	
	// Save the file data to a permanent file.
	var bytes []byte
	bytes, err = ioutil.ReadAll(file)
	err = ioutil.WriteFile(filepath, bytes, os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
		return nil, errors.New("While writing dockerfile, " + err.Error())
	}
	fmt.Println(strconv.FormatInt(int64(len(bytes)), 10), "bytes written to file", filepath)
	
	// Add the file to the specified repo's set of Dockerfiles.
	var dockerfile Dockerfile
	dockerfile, err = dbClient.dbCreateDockerfile(repo.getId(), filename, desc, filepath)
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
func buildDockerfile(dockerfile Dockerfile, sessionToken *apitypes.SessionToken,
	dbClient DBClient, values url.Values) (DockerImage, error) {

	var repo Repo
	var err error
	repo, err = dockerfile.getRepo()
	if err != nil { return nil, err }
	var realm Realm
	realm, err = repo.getRealm()
	if err != nil { return nil, err }

	var imageName string
	imageName, err = apitypes.GetRequiredPOSTFieldValue(values, "ImageName")
	if err != nil { return nil, err }
	if imageName == "" { return nil, errors.New("No HTTP parameter found for ImageName") }
	if ! localDockerImageNameIsValid(imageName) {
		return nil, errors.New(fmt.Sprintf("Image name '%s' is not valid - must be " +
			"of format <name>[:<tag>]", imageName))
	}
	fmt.Println("Image name =", imageName)
	
	// Check if am image with that name already exists.
	var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "inspect", imageName)
	var output []byte
	output, err = cmd.CombinedOutput()
	var outputStr string = string(output)
	if ! strings.HasPrefix(outputStr, "Error") {
		return nil, errors.New("An image with name " + imageName + " already exists.")
	}
	
	// Verify that the image name conforms to Docker's requirements.
	err = nameConformsToSafeHarborImageNameRules(imageName)
	if err != nil { return nil, err }
	
	// Create a temporary directory to serve as the build context.
	var tempDirPath string
	tempDirPath, err = ioutil.TempDir("", "")
	defer os.RemoveAll(tempDirPath)
	fmt.Println("Temp directory = ", tempDirPath)

	// Copy dockerfile to that directory.
	var in, out *os.File
	in, err = os.Open(dockerfile.getExternalFilePath())
	if err != nil { return nil, err }
	var dockerfileCopyPath string = tempDirPath + "/" + dockerfile.getName()
	out, err = os.Create(dockerfileCopyPath)
	if err != nil { return nil, err }
	_, err = io.Copy(out, in)
	if err != nil { return nil, err }
	err = out.Close()
	if err != nil { return nil, err }
	fmt.Println("Copied Dockerfile to " + dockerfileCopyPath)
	
//	fmt.Println("Changing directory to '" + tempDirPath + "'")
//	err = os.Chdir(tempDirPath)
//	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Create a the docker build command.
	// https://docs.docker.com/reference/commandline/build/
	// REPOSITORY                      TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
	// docker.io/cesanta/docker_auth   latest              3d31749deac5        3 months ago        528 MB
	// Image id format: <hash>[:TAG]
	
	var imageFullName string = realm.getName() + "/" + repo.getName() + ":" + imageName
	cmd = exec.Command("/usr/bin/docker", "build",
	"--file", tempDirPath + "/" + dockerfile.getName(), "--tag", imageFullName, tempDirPath)
	
	// Execute the command in the temporary directory.
	// This initiates processing of the dockerfile.
	output, err = cmd.CombinedOutput()
	outputStr = string(output)
	fmt.Println("...finished processing dockerfile.")
	fmt.Println("Output from docker build command:")
	fmt.Println(outputStr)
	fmt.Println()
	fmt.Println("End of output from docker build command.")
	
	fmt.Println("Files in " + tempDirPath + ":")
	dirfiles, _ := ioutil.ReadDir(tempDirPath)
	for _, f := range dirfiles {
		fmt.Println("\t" + f.Name())
	}
	
	if err != nil {
		fmt.Println()
		fmt.Println("Returning from buildDockerfile, with error")
		return nil, errors.New(err.Error() + ", " + outputStr)
	}
	fmt.Println("Performed docker build command successfully.")
	
	// Add a record for the image to the database.
	var image DockerImage
	image, err = dbClient.dbCreateDockerImage(repo.getId(),
		imageName, dockerfile.getDescription())
	fmt.Println("Created docker image object.")
	
	return image, nil
}
