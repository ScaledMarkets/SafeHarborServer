/*******************************************************************************
 * Functions needed to implement the handlers in Handlers.go.
 */

package main

import (
	"mime/multipart"
	"fmt"
	"errors"
	"os"
	"strconv"
	"strings"
	"regexp"
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
	if err == nil {
		fmt.Println("fileExists:")
		fmt.Println("\t", path)
	} else {
		fmt.Println("fileExists - NOT:")
		fmt.Println("\t", path)
		fmt.Println(err.Error())
	}
	return (err == nil)
}

/*******************************************************************************
 * Write the specified map to stdout. This is a diagnostic method.
 */
func printMap(m map[string][]string) {
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
			printMap(headers[i].Header)
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
