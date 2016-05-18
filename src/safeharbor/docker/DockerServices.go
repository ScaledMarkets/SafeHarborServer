/*******************************************************************************
 * Provide abstract functions that we need from docker and docker registry.
 * This module relies on implementations of DockerEngine and DockerRegistry.
 */
package docker

import (
	"fmt"
	"os"
	"io"
	//"io/ioutil"
	"bufio"
	"strings"
	"encoding/json"
	//"os/exec"
	//"errors"
	"regexp"
	"reflect"
	
	// SafeHarbor packages:
	"safeharbor/utils"
	"safeharbor/rest"
)

/* Replace with REST calls.
Registry 2.0:
https://github.com/docker/distribution/tree/master/docs
https://github.com/docker/distribution/tree/master/docs/spec
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
	Registry DockerRegistry
	Engine DockerEngine
}

/*******************************************************************************
 * 
 */
func NewDockerServices(registry DockerRegistry, engine DockerEngine) *DockerServices {
	return &DockerServices{
		Registry: registry,
		Engine: engine,
	}
}

/*******************************************************************************
 * 
 */
func (dockerSvcs *DockerServices) BuildDockerfile(dockerfileExternalFilePath,
	dockerfileName, realmName, repoName, imageName string,
	paramNames, paramValues []string) (string, error) {
	
	if ! localDockerImageNameIsValid(imageName) {
		return "", utils.ConstructUserError(fmt.Sprintf("Image name '%s' is not valid - must be " +
			"of format <name>[:<tag>]", imageName))
	}
	fmt.Println("Image name =", imageName)
	
	// Check if an image with that name already exists.
	var dockerImageName, tag string
	dockerImageName, tag = dockerSvcs.ConstructDockerImageName(realmName, repoName, imageName)
	var exists bool = false
	var err error = nil
	if dockerSvcs.Registry == nil {  // no registry
		// Check if image exists in engine.
		var fullName = dockerImageName
		if imageName != "" { fullName = fullName + ":" + imageName }
		_, err = dockerSvcs.Engine.GetImageInfo(imageName)
		if err == nil { exists = true }
	} else {
		exists, err = dockerSvcs.Registry.ImageExists(dockerImageName, tag)
		//exists, err = dockerSvcs.Registry.ImageExists(realmName + "/" + repoName, imageName)
	}
	
	if exists {
		return "", utils.ConstructUserError(
			"Image with name " + realmName + "/" + repoName + ":" + imageName + " already exists.")
	}
	
	// Verify that the image name conforms to Docker's requirements.
	err = NameConformsToDockerRules(imageName)
	if err != nil { return "", err }
	
	// Create a temporary directory to serve as the build context.
	var tempDirPath string
	tempDirPath, err = utils.MakeTempDir()
	if err != nil { return "", err }
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
	if err != nil { return "", err }
	var dockerfileCopyPath string = tempDirPath + "/" + dockerfileName
	out, err = os.Create(dockerfileCopyPath)
	if err != nil { return "", err }
	_, err = io.Copy(out, in)
	if err != nil { return "", err }
	err = out.Close()
	if err != nil { return "", err }
	fmt.Println("Copied Dockerfile to " + dockerfileCopyPath)
	
//	fmt.Println("Changing directory to '" + tempDirPath + "'")
//	err = os.Chdir(tempDirPath)
//	if err != nil { return apitypes.NewFailureDescFromError(err) }
	
	// Create a the docker build command.
	// https://docs.docker.com/reference/commandline/build/
	// REPOSITORY                      TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
	// docker.io/cesanta/docker_auth   latest              3d31749deac5        3 months ago        528 MB
	// Image id format: <hash>[:TAG]
	
	var imageFullName string = realmName + "/" + repoName + ":" + imageName
	
	var outputStr string
	outputStr, err = dockerSvcs.Engine.BuildImage(tempDirPath, imageFullName, 
		dockerfileName, paramNames, paramValues)
	if err != nil { return outputStr, err }
	
	if dockerSvcs.Registry != nil {  // no registry
		// Push new image to registry. Use the engine's push image feature.
		// Have not been able to get the engine push command to work. The docker client
		// end up reporting "Pull session cancelled".
		//err = dockerSvcs.Engine.PushImage(imageRegistryTag)
		
		// Obtain image as a file.
		var tempDirPath2 string
		tempDirPath2, err = utils.MakeTempDir()
		if err != nil { return outputStr, err }
		defer os.RemoveAll(tempDirPath2)
		var imageFile *os.File
		imageFile, err = utils.MakeTempFile(tempDirPath2, "")
		if err != nil { return outputStr, err }
		var imageFilePath = imageFile.Name()
		err = dockerSvcs.Engine.GetImage(imageFullName, imageFilePath)
		if err != nil { return outputStr, err }
		
		// Obtain the image digest.
		var info map[string]interface{}
		info, err = dockerSvcs.Engine.GetImageInfo(imageFullName)
		if err != nil { return outputStr, err }
		var digest = info["Id"]
		var digestString string
		var isType bool
		digestString, isType = digest.(string)
		if digest == nil {
			fmt.Println("Digest is nil; map returned from GetImageInfo:")
			rest.PrintMap(info)
			return outputStr, utils.ConstructServerError("Digest is nil") }
		if ! isType { return outputStr, utils.ConstructServerError(
			"checksum is not a string: it is a " + reflect.TypeOf(digest).String())
		}
		if digestString == "" { return outputStr, utils.ConstructServerError(
			"No checksum field found for image")
		}
		
		// Push image to registry - all layers and manifest.
		var dockerImageName, tag string
		dockerImageName, tag = dockerSvcs.ConstructDockerImageName(realmName, repoName, imageName)
		err = dockerSvcs.Registry.PushImage(dockerImageName, tag, imageFilePath)
		if err != nil { return outputStr, err }
		
		// Tag the uploaded image with its name.
		//err = dockerSvcs.Registry.TagImage(digestString, ....repoName, ....tag)
		if err != nil { return outputStr, err }
	}
	
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
	var output *DockerBuildOutput = NewDockerBuildOutput()
	
	var lines = strings.Split(buildOutputStr, "\n")
	var state int = 1
	var step *DockerBuildStep
	var lineNo int = 0
	fmt.Println("ParseBuildCommandOutput: B")  // debug
	for {
		
		if lineNo >= len(lines) {
			fmt.Println("ParseBuildCommandOutput: C")  // debug
			return output, utils.ConstructServerError("Incomplete")
		}
		
		var line string = lines[lineNo]
		
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
				fmt.Println("ParseBuildCommandOutput: D")  // debug
				return output, nil
			}
			
			therest = strings.TrimPrefix(line, "Error")
			if len(therest) < len(line) {
				output.ErrorMessage = therest
				fmt.Println("ParseBuildCommandOutput: E")  // debug
				return output, utils.ConstructServerError(output.ErrorMessage)
			}
			
			lineNo++
			state = 1
			continue
			
		case 2: // Looking for step parts
			
			if step == nil {
				output.ErrorMessage = "Internal error: should not happen"
				fmt.Println("ParseBuildCommandOutput: F")  // debug
				return output, utils.ConstructServerError(output.ErrorMessage)
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
			fmt.Println("ParseBuildCommandOutput: G")  // debug
			return output, utils.ConstructServerError(output.ErrorMessage)
		}
	}
	fmt.Println("ParseBuildCommandOutput: H")  // debug
	output.ErrorMessage = "Did not find a final image Id"
	return output, utils.ConstructServerError(output.ErrorMessage)
}

/*******************************************************************************
 * Parse the string that is returned by the docker daemon REST build function.
 * Partial results are returned, but with an error.
 */
func ParseBuildRESTOutput(restResponse string) (*DockerBuildOutput, error) {
	
	fmt.Println("ParseBuildRESTOutput: A")  // debug
	var outputstr string
	var err error
	outputstr, err = extractBuildOutputFromRESTResponse(restResponse)
	fmt.Println("ParseBuildRESTOutput: B")  // debug
	if err != nil { return nil, err }
	fmt.Println("ParseBuildRESTOutput: C")  // debug
	return ParseBuildCommandOutput(outputstr)
}

/*******************************************************************************
 * Retrieve the specified image from the registry and store it in a file.
 * Return the file path.
 */
func (dockerSvcs *DockerServices) SaveImage(imageNamespace, imageName, tag string) (string, error) {

	fmt.Println("Creating temp file to save the image to...")
	var tempFile *os.File
	var err error
	tempFile, err = utils.MakeTempFile("", "")
	// TO DO: Is the above a security issue?
	if err != nil { return "", err }
	var tempFilePath = tempFile.Name()
	
	var imageFullName string
	if imageNamespace == "" {
		imageFullName = imageName
	} else {
		imageFullName = imageNamespace + "/" + imageName
	}

	if dockerSvcs.Registry == nil {  // no registry
		
		var repoNameAndTag = imageFullName
		if tag != "" { repoNameAndTag = repoNameAndTag + ":" + tag }
		err = dockerSvcs.Engine.GetImage(repoNameAndTag, tempFilePath)
		if err != nil { return "", err }
		
	} else {
	
		err = dockerSvcs.Registry.GetImage(imageFullName, tag, tempFilePath)
		if err != nil { return "", err }
	}
	
	return tempFilePath, nil
}

/*******************************************************************************
 * Return the hash of the specified Docker image, as computed by the file''s registry.
 */
func GetSignature(imageId string) ([]byte, error) {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, nil
}

/*******************************************************************************
 * 
 */
func (dockerSvcs *DockerServices) RemoveDockerImage(repoName, tag string) error {
	
	// Delete from registry.
	var err error
	err = dockerSvcs.Engine.DeleteImage(repoName, tag)
	if dockerSvcs.Registry != nil {
		err = dockerSvcs.Registry.DeleteImage(repoName, tag)
	}
	if err != nil { return err }
	
	// Delete local engine copy as well, if it exists.
	return nil
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
	return utils.ConstructUserError("Name '" + name + "' does not conform to docker name rules: " +
		"[a-z0-9]+(?:[._-][a-z0-9]+)*  Offending fragment: '" + b + "'")
}

/*******************************************************************************
 * 
 */
func (dockerSvcs *DockerServices) ConstructDockerImageName(shRealmName,
	shRepoName, shImageName string) (imageName, tag string) {

	return (shRealmName + "/" + shRepoName), shImageName
}



/*******************************************************************************
								Internal methods
*******************************************************************************/



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
		if err != nil { panic(utils.ConstructServerError("Unexpected internal error")) }
		if ! matched { return false }
	}
	
	return true
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
	
	fmt.Println("extractBuildOutputFromRESTResponse: A")  // debug
	var reader = bufio.NewReader(strings.NewReader(restResponse))
	
	var output = ""
	for {
		var lineBytes []byte
		var isPrefix bool
		var err error
		lineBytes, isPrefix, err = reader.ReadLine()
		if err == io.EOF { break }
		fmt.Println("extractBuildOutputFromRESTResponse: B")  // debug
		if err != nil { return "", err }
		if isPrefix { fmt.Println("Warning - only part of string was read") }
		
		var obj interface{}
		err = json.Unmarshal(lineBytes, &obj)
		fmt.Println("extractBuildOutputFromRESTResponse: C")  // debug
		if err != nil { return "", err }
		
		var isType bool
		var msgMap map[string]interface{}
		msgMap, isType = obj.(map[string]interface{})
		fmt.Println("extractBuildOutputFromRESTResponse: D")  // debug
		if ! isType { return "", utils.ConstructServerError(
			"Unexpected format for json build output: " + string(lineBytes))
		}
		obj = msgMap["stream"]
		var value string
		value, isType = obj.(string)
		fmt.Println("extractBuildOutputFromRESTResponse: E")  // debug
		if obj == nil {
			obj = msgMap["errorDetail"]
			if obj == nil {
				return "", utils.ConstructServerError(
					"Unexpected JSON field: " + string(lineBytes))
			}
			var errMsgJSON string
			errMsgJSON, isType = obj.(string)
			if ! isType { return "", utils.ConstructServerError(
				"Unexpected type in json field value: " + reflect.TypeOf(obj).String())
			}
			return "", utils.ConstructUserError(errMsgJSON)
		}
		if ! isType { return "", utils.ConstructServerError(
			"Unexpected type in json field value: " + reflect.TypeOf(obj).String())
		}
		fmt.Println("extractBuildOutputFromRESTResponse: F")  // debug

		output = output + value
	}
	
	fmt.Println("extractBuildOutputFromRESTResponse: Z")  // debug
	return output, nil
}
