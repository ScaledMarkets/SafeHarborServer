/*******************************************************************************
 * Provide abstract functions that we need from docker and docker registry.
 */
package docker

import (
	"fmt"
	"os"
	"io"
	"io/ioutil"
	"bufio"
	"strings"
	"encoding/json"
	//"os/exec"
	//"errors"
	"regexp"
	
	// SafeHarbor packages:
	"safeharbor/util"
)

/* Replace with REST calls.
Registry 2.0:
https://github.com/docker/distribution/blob/master/docs/spec/api.md

SSL config:
https://www.digitalocean.com/community/tutorials/how-to-set-up-a-private-docker-registry-on-ubuntu-14-04

Registry 1.4:
https://docs.docker.com/apidocs/v1.4.0/
	
Engine:
https://github.com/docker/docker/blob/master/docs/reference/api/docker_remote_api_v1.24.md

Image format:
https://github.com/docker/docker/blob/master/image/spec/v1.md
*/

type DockerServices struct {
	Registry *DockerRegistry
	Engine *DockerEngine
}

/*******************************************************************************
 * 
 */
func NewDockerServices(registry *DockerRegistry, engine *DockerEngine) *DockerServices {
	return &DockerServices{
		Registry: registry,
		Engine: engine,
	}
}

/*******************************************************************************
 * 
 */
func (dockerSvcs *DockerServices) BuildDockerfile(dockerfileExternalFilePath,
	dockerfileName, realmName, repoName, imageName string) (string, error) {
	
	if ! localDockerImageNameIsValid(imageName) {
		return "", util.ConstructError(fmt.Sprintf("Image name '%s' is not valid - must be " +
			"of format <name>[:<tag>]", imageName))
	}
	fmt.Println("Image name =", imageName)
	
	// Check if an image with that name already exists.
	var exists bool = false
	var err error = nil
	fmt.Println("BuildDockerfile: 0")  // debug
	if dockerSvcs.Registry != nil {
		exists, err = dockerSvcs.Registry.ImageExists(realmName + "/" + repoName, imageName)
	}
	fmt.Println("BuildDockerfile: A")  // debug
	if exists {
		return "", util.ConstructError(
			"Image with name " + realmName + "/" + repoName + ":" + imageName + " already exists.")
	}
	fmt.Println("BuildDockerfile: B")  // debug
	
		/* Obsolete: -----------------
		var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "inspect", imageName)
			// GET /containers/4fa6e0f0c678/json HTTP/1.1
		var output []byte
		output, err = cmd.CombinedOutput()
		var outputStr string = string(output)
		if ! strings.Contains(outputStr, "Error") {
			return "", util.ConstructError(
				"'" + outputStr + "'; perhaps an image with name " + imageName + " already exists.")
		}
		------------------------------- */
	
	// Verify that the image name conforms to Docker's requirements.
	err = NameConformsToDockerRules(imageName)
	fmt.Println("BuildDockerfile: C")  // debug
	if err != nil { return "", err }
	
	// Create a temporary directory to serve as the build context.
	var tempDirPath string
	tempDirPath, err = ioutil.TempDir("", "")
	fmt.Println("BuildDockerfile: D")  // debug
	//....TO DO: Is the above a security problem? Do we need to use a private
	// directory? I think so.
	defer func() {
		fmt.Println("Removing all files at " + tempDirPath)
		os.RemoveAll(tempDirPath)
	}()
	fmt.Println("Temp directory = ", tempDirPath)

	// Copy dockerfile to that directory.
	var in, out *os.File
	in, err = os.Open(dockerfileExternalFilePath)
	fmt.Println("BuildDockerfile: E")  // debug
	if err != nil { return "", err }
	var dockerfileCopyPath string = tempDirPath + "/" + dockerfileName
	out, err = os.Create(dockerfileCopyPath)
	fmt.Println("BuildDockerfile: F")  // debug
	if err != nil { return "", err }
	_, err = io.Copy(out, in)
	if err != nil { return "", err }
	err = out.Close()
	if err != nil { return "", err }
	fmt.Println("Copied Dockerfile to " + dockerfileCopyPath)
	
//	fmt.Println("Changing directory to '" + tempDirPath + "'")
//	err = os.Chdir(tempDirPath)
//	if err != nil { return apitypes.NewFailureDesc(err.Error()) }
	
	// Create a the docker build command.
	// https://docs.docker.com/reference/commandline/build/
	// REPOSITORY                      TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
	// docker.io/cesanta/docker_auth   latest              3d31749deac5        3 months ago        528 MB
	// Image id format: <hash>[:TAG]
	
	var imageFullName string = realmName + "/" + repoName + ":" + imageName
	
	var outputStr string
	outputStr, err = dockerSvcs.Engine.BuildImage(tempDirPath, imageFullName)
	fmt.Println("BuildDockerfile: G")  // debug
	
		/* Obsolete: -----------------
	
		var cmd *exec.Cmd
		var output []byte
		cmd = exec.Command("docker", "build", 
			"--file", tempDirPath + "/" + dockerfileName,
			"--tag", imageFullName, tempDirPath)
		
			// https://docs.docker.com/engine/reference/api/docker_remote_api_v1.23/#build-image-from-a-dockerfile
			// POST /build HTTP/1.1
			//
			// {{ TAR STREAM }} (this is the contents of the "build context")
			
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
		------------------------------- */
	
	return outputStr, err
}

/*******************************************************************************
 * Parse the string that is returned by the docker build command.
 * Partial results are returned, but with an error.
 *
 * Parse algorithm:
	States:
	1. Looking for next step:
		When no more lines, done but incomplete.
		When encounter "Step ",
			Set step no.
			Set command.
			Read next line
			If no more lines,
				Then done but incomplete.
				Else go to state 2.
		When encounter "Successfully built"
			Set final image id
			Done and complete.
		When encounter "Error"
			Done with error
		Otherwise read line (i.e., skip the line) and go to state 1.
	2. Looking for step parts:
		When encounter " ---> ",
			Recognize and (if recognized) add part.
			Read next line.
			if no more lines,
				Then done but incomplete.
				Else go to state 2
		Otherwise go to state 1

 * Sample output:
	Sending build context to Docker daemon  2.56 kB\rSending build context to Docker daemon  2.56 kB\r\r
	Step 0 : FROM ubuntu:14.04
	 ---> ca4d7b1b9a51
	Step 1 : MAINTAINER Steve Alexander <steve@scaledmarkets.com>
	 ---> Using cache
	 ---> 3b6e27505fc5
	Step 2 : ENV REFRESHED_AT 2015-07-13
	 ---> Using cache
	 ---> 5d6cdb654470
	Step 3 : RUN apt-get -yqq update
	 ---> Using cache
	 ---> c403414c8254
	Step 4 : RUN apt-get -yqq install apache2
	 ---> Using cache
	 ---> aa3109896080
	Step 5 : VOLUME /var/www/html
	 ---> Using cache
	 ---> 138c71e28dc1
	Step 6 : WORKDIR /var/www/html
	 ---> Using cache
	 ---> 8aa5cb29ae1d
	Step 7 : ENV APACHE_RUN_USER www-data
	 ---> Using cache
	 ---> 7f721c24718d
	Step 8 : ENV APACHE_RUN_GROUP www-data
	 ---> Using cache
	 ---> 05a094d0d47f
	Step 9 : ENV APACHE_LOG_DIR /var/log/apache2
	 ---> Using cache
	 ---> 30424d879506
	Step 10 : ENV APACHE_PID_FILE /var/run/apache2.pid
	 ---> Using cache
	 ---> d163597446d6
	Step 11 : ENV APACHE_RUN_DIR /var/run/apache2
	 ---> Using cache
	 ---> 065c69b4a35c
	Step 12 : ENV APACHE_LOCK_DIR /var/lock/apache2
	 ---> Using cache
	 ---> 937eb3fd1f42
	Step 13 : RUN mkdir -p $APACHE_RUN_DIR $APACHE_LOCK_DIR $APACHE_LOG_DIR
	 ---> Using cache
	 ---> f0aebcae65d4
	Step 14 : EXPOSE 80
	 ---> Using cache
	 ---> 5f139d64c08f
	Step 15 : ENTRYPOINT /usr/sbin/apache2
	 ---> Using cache
	 ---> 13cf0b9469c1
	Step 16 : CMD -D FOREGROUND
	 ---> Using cache
	 ---> 6a959754ab14
	Successfully built 6a959754ab14
	
 * Another sample:
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
 */
func ParseBuildCommandOutput(buildOutputStr string) (*DockerBuildOutput, error) {
	
	fmt.Println("ParseBuildCommandOutput: A")  // debug
	fmt.Println("buildOutputStr=" + buildOutputStr)  // debug
	
	var output *DockerBuildOutput = NewDockerBuildOutput()
	
	var lines = strings.Split(buildOutputStr, "\n")
	var state int = 1
	var step *DockerBuildStep
	var lineNo int = 0
	for {
		
		if lineNo >= len(lines) {
			return output, util.ConstructError("Incomplete")
		}
		
		var line string = lines[lineNo]
		
		fmt.Println(line)  // debug
			
		switch state {
			
		case 1: // Looking for next step
			
			var therest = strings.TrimPrefix(line, "Step ")
			if len(therest) < len(line) {
				// Syntax is: number space colon space command
				var stepNo int
				var cmd string
				fmt.Sscanf(therest, "%d", &stepNo)
				
				var separator = " : "
				var seppos int = strings.Index(therest, separator)
				if seppos != -1 { // found
					cmd = therest[seppos + len(separator):] // portion from seppos on
					step = output.addStep(stepNo, cmd)
				}
				
				lineNo++
				state = 2
				continue
			}
			
			therest = strings.TrimPrefix(line, "Successfully built ")
			if len(therest) < len(line) {
				var id = therest
				output.setFinalImageId(id)
				return output, nil
			}
			
			therest = strings.TrimPrefix(line, "Error")
			if len(therest) < len(line) {
				output.ErrorMessage = therest
				return output, util.ConstructError(output.ErrorMessage)
			}
			
			lineNo++
			state = 1
			continue
			
		case 2: // Looking for step parts
			
			if step == nil {
				output.ErrorMessage = "Internal error: should not happen"
				return output, util.ConstructError(output.ErrorMessage)
			}

			var therest = strings.TrimPrefix(line, " ---> ")
			if len(therest) < len(line) {
				if strings.HasPrefix(therest, "Using cache") {
					step.setUsedCache()
				} else {
					if strings.Contains(" ", therest) {
						// Unrecognized line - skip it but stay in the current state.
					} else {
						step.setProducedImageId(therest)
					}
				}
				lineNo++
				continue
			}
			
			state = 1
			
		default:
			output.ErrorMessage = "Internal error: Unrecognized state"
			return output, util.ConstructError(output.ErrorMessage)
		}
	}
	output.ErrorMessage = "Did not find a final image Id"
	fmt.Println("ParseBuildCommandOutput: Z")  // debug
	return output, util.ConstructError(output.ErrorMessage)
}

/*******************************************************************************
 * Parse the string that is returned by the docker daemon REST build function.
 * Partial results are returned, but with an error.
 */
func ParseBuildRESTOutput(restResponse string) (*DockerBuildOutput, error) {
	
	var outputstr string
	var err error
	fmt.Println("ParseBuildRESTOutput: A")  // debug
	outputstr, err = extractBuildOutputFromRESTResponse(restResponse)
	fmt.Println("ParseBuildRESTOutput: B")  // debug
	if err != nil { return nil, err }
	return ParseBuildCommandOutput(outputstr)
}

/*******************************************************************************
 * The docker daemon build function - a REST function - returns a series of
 * JSON objects that encode the output stream of the build operation. We need to
 * parse the JSON and extract/decode the build operation output stream.
 *
 * Sample REST response:
	{"stream": "Step 1..."}
	{"stream": "..."}
	{"error": "Error...", "errorDetail": {"code": 123, "message": "Error..."}}
	
 * Another sample:
	{"stream":"Step 1 : FROM centos\n"}
	{"stream":" ---\u003e 968790001270\n"}
	{"stream":"Step 2 : RUN echo moo \u003e oink\n"}
	{"stream":" ---\u003e Using cache\n"}
	{"stream":" ---\u003e cb0948362f97\n"}
	{"stream":"Successfully built cb0948362f97\n"}
 */
func extractBuildOutputFromRESTResponse(restResponse string) (string, error) {
	
	fmt.Println("extractBuildOutputFromRESTResponse: A; restResponse=" + restResponse)  // debug
	var reader = bufio.NewReader(strings.NewReader(restResponse))
	fmt.Println("extractBuildOutputFromRESTResponse: B")  // debug
	
	type Message struct {
		stream string
	}

	var output = ""
	for {
		var lineBytes []byte
		var isPrefix bool
		var err error
		lineBytes, isPrefix, err = reader.ReadLine()
		if err == io.EOF { break }
		if err != nil { return "", err }
		if isPrefix { fmt.Println("Warning - only part of string was read") }
		fmt.Println("extractBuildOutputFromRESTResponse: C; lineBytes=" + string(lineBytes))  // debug
		
		var message *Message
		err = json.Unmarshal(lineBytes, &message)
		if err != nil { return "", err }
		
		fmt.Println("extractBuildOutputFromRESTResponse: D; stream=" + message.stream)  // debug
		output = output + message.stream
	}
	
	fmt.Println("extractBuildOutputFromRESTResponse: Z")  // debug
	return output, nil
}

/*******************************************************************************
 * 
 */
func (dockerSvcs *DockerServices) SaveImage(imageNamespace, imageName, tag string) (string, error) {
	
	fmt.Println("Creating temp file to save the image to...")
	var tempFile *os.File
	var err error
	tempFile, err = ioutil.TempFile("", "")
	// TO DO: Is the above a security issue?
	if err != nil { return "", err }
	var tempFilePath = tempFile.Name()
	
	var imageFullName string
	if imageNamespace == "" {
		imageFullName = imageName
	} else {
		imageFullName = imageNamespace + "/" + imageName
	}
	err = dockerSvcs.Registry.GetImage(imageFullName, tag, tempFilePath)
	//var cmd *exec.Cmd = exec.Command("docker", "save", "-o", tempFilePath, imageFullName)
	// need to use registry API
	//fmt.Println(fmt.Sprintf("Running docker save -o%s %s", tempFilePath, imageFullName))
	//err = cmd.Run()
	if err != nil { return "", err }
	return tempFilePath, nil
}

/*******************************************************************************
 * Return the hash of the specified Docker image, as computed by the file''s registry.
 */
func GetDigest(imageId string) ([]byte, error) {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, nil
}

/*******************************************************************************
 * 
 */
func (dockerSvcs *DockerServices) RemoveDockerImage(imageName, tag string) error {
	
	return dockerSvcs.Registry.DeleteImage(imageName, tag)
	//var cmd *exec.Cmd = exec.Command("docker", "rmi", imageFullName)
	// DELETE /images/test HTTP/1.1
	//fmt.Println(fmt.Sprintf("Running docker rmi %s", imageFullName))
	//return cmd.Run()
}

/*******************************************************************************
 * Check that repository name component matches "[a-z0-9]+(?:[._-][a-z0-9]+)*".
 * I.e., first char is a-z or 0-9, and remaining chars (if any) are those or
 * a period, underscore, or dash. If rules are satisfied, return nil; otherwise,
 * return an error.
 */
func NameConformsToDockerRules(name string) error {
	var a = strings.TrimLeft(name, "abcdefghijklmnopqrstuvwxyz0123456789")
	var b = strings.TrimRight(a, "abcdefghijklmnopqrstuvwxyz0123456789._-")
	if len(b) == 0 { return nil }
	return util.ConstructError("Name '" + name + "' does not conform to docker name rules: " +
		"[a-z0-9]+(?:[._-][a-z0-9]+)*  Offending fragment: '" + b + "'")
}

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
		if err != nil { panic(util.ConstructError("Unexpected internal error")) }
		if ! matched { return false }
	}
	
	return true
}
