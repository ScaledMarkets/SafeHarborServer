package docker

import (
	"fmt"
	"os"
	"io"
	"io/ioutil"
	"strings"
	"os/exec"
	"errors"
	
	// SafeHarbor packages:
	"rest"
)

/*******************************************************************************
 * Provide abstract functions that we need from docker and docker registry.
 */
type DockerService struct {
}

func NewDockerService() *DockerService {
	return &DockerService{}
}

/*******************************************************************************
 * 
 */
func (docker *DockerService) BuildDockerfile(dockerfile Dockerfile, realm Realm, repo Repo,
	imageName string) (string, error) {
	
	if ! localDockerImageNameIsValid(imageName) {
		return "", errors.New(fmt.Sprintf("Image name '%s' is not valid - must be " +
			"of format <name>[:<tag>]", imageName))
	}
	fmt.Println("Image name =", imageName)
	
	// Check if am image with that name already exists.
	var cmd *exec.Cmd = exec.Command("/usr/bin/docker", "inspect", imageName)
	var output []byte
	var err error
	output, err = cmd.CombinedOutput()
	var outputStr string = string(output)
	if ! strings.HasPrefix(outputStr, "Error") {
		return "", errors.New("An image with name " + imageName + " already exists.")
	}
	
	// Verify that the image name conforms to Docker's requirements.
	err = nameConformsToSafeHarborImageNameRules(imageName)
	if err != nil { return "", err }
	
	// Create a temporary directory to serve as the build context.
	var tempDirPath string
	tempDirPath, err = ioutil.TempDir("", "")
	//....TO DO: Is the above a security problem? Do we need to use a private
	// directory? I think so.
	defer func() {
		fmt.Println("Removing all files at " + tempDirPath)
		os.RemoveAll(tempDirPath)
	}()
	fmt.Println("Temp directory = ", tempDirPath)

	// Copy dockerfile to that directory.
	var in, out *os.File
	in, err = os.Open(dockerfile.getExternalFilePath())
	if err != nil { return "", err }
	var dockerfileCopyPath string = tempDirPath + "/" + dockerfile.getName()
	out, err = os.Create(dockerfileCopyPath)
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
	
	var imageFullName string = realm.getName() + "/" + repo.getName() + ":" + imageName
	cmd = exec.Command("docker", "build", 
		"--file", tempDirPath + "/" + dockerfile.getName(),
		"--tag", imageFullName, tempDirPath)
	
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
		return "", errors.New(err.Error() + ", " + outputStr)
	}
	fmt.Println("Performed docker build command successfully.")
	return outputStr, nil
}

/*******************************************************************************
 * Parse docker build output - i.e., parses the output string that is returned
 * by BuildDockerfile.
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
func (docker *DockerService) ParseBuildOutput(buildOutputStr string) (*DockerBuildOutput, error) {
	
	var output *DockerBuildOutput = NewDockerBuildOutput(id)
	
	var lines = strings.Split(buildOutputStr, "\n")
	var state int = 1
	var step DockerBuildStep
	var int lineNo = 0
	for {
		
		if lineNo >= len(lines) {
			return output, errors.New("Incomplete")
		}
		
		switch state {
			
		case 1: // Looking for next step
			
			if lineNo >= len(lines) {
				output.ErrorMessage = "Incomplete"
				return output, errors.New(output.ErrorMessage)
			}
			
			var therest = strings.TrimPrefix(line, "Step ")
			if len(therest) < len(line) {
				// Syntax is: number space colon space command
				var stepNo int
				var cmd = string
				fmt.Sscanf(therest, "%d", &stepNo)
				
				var separator = " : "
				inv seppos = strings.Index(therest, separator)
				if seppos != -1 { // found
					cmd = therest[seppos + len(separator):] // portion from seppos on
					step = output.addStep(stepNo, cmd)
				}
				
				lineNo++
				state = 2
				continue
			}
			
			var therest = strings.TrimPrefix(line, "Successfully built ")
			if len(therest) < len(line) {
				var id = therest
				output.setFinalImageId(id)
				return output, nil
			}
			
			lineNo++
			state = 1
			continue
			
		case 2: // Looking for step parts
			
			if step == nil {
				output.ErrorMessage = "Internal error: should not happen"
				return output, errors.New(output.ErrorMessage)
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
			return output, errors.New(output.ErrorMessage)
		}
	}
	output.ErrorMessage = "Did not find a final image Id"
	return output, errors.New(output.ErrorMessage)
}

/*******************************************************************************
 * 
 */
func (docker *DockerService) SaveImage(dockerImage DockerImage) (string, error) {
	
	fmt.Println("Creating temp file to save the image to...")
	var tempFile *os.File
	var err error
	tempFile, err = ioutil.TempFile("", "")
	// TO DO: Is the above a security issue?
	if err != nil { return "", err }
	var tempFilePath = tempFile.Name()
	
	var imageFullName string
	imageFullName, err = dockerImage.getFullName()
	if err != nil { return "", err }
	var cmd *exec.Cmd = exec.Command("docker", "save", "-o", tempFilePath, imageFullName)
	fmt.Println(fmt.Sprintf("Running docker save -o%s %s", tempFilePath, imageFullName))
	err = cmd.Run()
	if err != nil { return "", err }
	return tempFilePath, nil
}

/*******************************************************************************
 * Return the hash of the specified Docker image, as computed by the file''s registry.
 */
func (docker *DockerService) GetDigest(imageId string) ([]byte, error) {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, nil
}

/*******************************************************************************
 * 
 */
func (docker *DockerService) RemoveDockerImage(dockerImage DockerImage) error {
	var imageFullName string
	var err error
	imageFullName, err = dockerImage.getFullName()
	if err != nil { return err }
	var cmd *exec.Cmd = exec.Command("docker", "rmi", imageFullName)
	fmt.Println(fmt.Sprintf("Running docker rmi %s", imageFullName))
	return cmd.Run()
}
