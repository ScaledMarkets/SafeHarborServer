/*******************************************************************************
 * Interface for accessing a docker engine via its REST API.
 * Engine API:
 * https://github.com/docker/docker/blob/master/docs/reference/api/docker_remote_api.md
 */

package docker

import (
	"fmt"
	"io"
	"os"
	"io/ioutil"
	"net"
	"net/http"
	"archive/tar"
	"errors"
	"path/filepath"
	"encoding/base64"
	
	"safeharbor/rest"
)

type DockerEngine struct {
	rest.RestContext
}

/*******************************************************************************
 * 
 */
func OpenDockerEngineConnection() (*DockerEngine, error) {

	var engine *DockerEngine = &DockerEngine{
		// https://docs.docker.com/engine/quickstart/#bind-docker-to-another-host-port-or-a-unix-socket
		// Note: When the SafeHarborServer container is run, it must mount the
		// /var/run/docker.sock unix socket in the container:
		//		-v /var/run/docker.sock:/var/run/docker.sock
		RestContext: *rest.CreateUnixRestContext(
			unixDial,
			"", "",
			func (req *http.Request, s string) {}),
	}
	
	fmt.Println("Attempting to ping the engine...")
	var err error = engine.Ping()
	if err != nil {
		return nil, err
	}
	
	return engine, nil
}

/*******************************************************************************
 * For connecting to docker''s unix domain socket.
 */
func unixDial(network, addr string) (conn net.Conn, err error) {
	return net.Dial("unix", "/var/run/docker.sock")
}

/*******************************************************************************
 * 
 */
func (engine *DockerEngine) Ping() error {
	
	var uri = "_ping"
	var response *http.Response
	var err error
	response, err = engine.SendBasicGet(uri)
	if err != nil { return err }
	if response.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Ping returned status: %s", response.Status))
	}
	return nil
}

/*******************************************************************************
 * Retrieve a list of the images that the docker engine has.
 */
func (engine *DockerEngine) GetImages() ([]map[string]interface{}, error) {
	
	var uri = "/images/json?all=1"
	var response *http.Response
	var err error
	response, err = engine.SendBasicGet(uri)
	if err != nil { return nil, err }
	if response.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("GetImages returned status: %s", response.Status))
	}
	var imageMaps []map[string]interface{}
	imageMaps, err = rest.ParseResponseBodyToMaps(response.Body)
	if err != nil { return nil, err }
	return imageMaps, nil
}

/*******************************************************************************
 * Invoke the docker engine to build the image defined by the specified contents
 * of the build directory, which presumably contains a dockerfile. The textual
 * response from the docker engine is returned.
 */
func (engine *DockerEngine) BuildImage(buildDirPath, imageFullName string) (string, error) {

	// https://docs.docker.com/engine/reference/api/docker_remote_api_v1.23/#build-image-from-a-dockerfile
	// POST /build HTTP/1.1
	//
	// {{ TAR STREAM }} (this is the contents of the "build context")
	
	// See also the docker command line code, in docker/vendor/src/github.com/docker/engine-api/client/image_build.go:
	// https://github.com/docker/docker/blob/7fd53f7c711474791ce4292326e0b1dc7d4d6b0f/vendor/src/github.com/docker/engine-api/client/image_build.go
	
	// Create a temporary tar file of the build directory contents.
	fmt.Println("BuildImage: A")  // debug
	var tarFile *os.File
	var err error
	var tempDirPath string
	tempDirPath, err = ioutil.TempDir("", "")
	if err != nil { return "", err }
	defer os.RemoveAll(tempDirPath)
	tarFile, err = ioutil.TempFile(tempDirPath, "")
	fmt.Println("BuildImage: B")  // debug
	if err != nil { return "", errors.New(fmt.Sprintf(
		"When creating temp file '%s': %s", tarFile.Name(), err.Error()))
	}
	
	// Walk the build directory and add each file to the tar.
	var tarWriter = tar.NewWriter(tarFile)
	fmt.Println("BuildImage: C")  // debug
	err = filepath.Walk(buildDirPath,
		func(path string, info os.FileInfo, err error) error {
		
			fmt.Println("BuildImage: A.A")  // debug
			// Open the file to be written to the tar.
			if info.Mode().IsDir() { return nil }
			fmt.Println("BuildImage: A.B")  // debug
			var new_path = path[len(buildDirPath):]
			if len(new_path) == 0 { return nil }
			fmt.Println("BuildImage: A.C")  // debug
			var file *os.File
			file, err = os.Open(path)
			fmt.Println("BuildImage: A.D")  // debug
			if err != nil { return err }
			defer file.Close()
			
			// Write tar header for the file.
			var header *tar.Header
			header, err = tar.FileInfoHeader(info, new_path)
			if err != nil { return err }
			header.Name = new_path
			err = tarWriter.WriteHeader(header)
			fmt.Println("BuildImage: A.E")  // debug
			if err != nil { return err }
			
			// Write the file contents to the tar.
			_, err = io.Copy(tarWriter, file)
			fmt.Println("BuildImage: A.F")  // debug
			if err != nil { return err }
			fmt.Println("BuildImage: A.G")  // debug
			
			return nil  // success - file was written to tar.
		})
	
	if err != nil { return "", err }
	tarWriter.Close()
	
	// Send the request to the docker engine, with the tar file as the body content.
	var tarReader io.ReadCloser
	tarReader, err = os.Open(tarFile.Name())
	fmt.Println("BuildImage: D")  // debug
	defer tarReader.Close()
	if err != nil { return "", err }
	var headers = make(map[string]string)
	headers["Content-Type"] = "application/tar"
	headers["X-Registry-Config"] = base64.URLEncoding.EncodeToString([]byte("{}"))
	var response *http.Response
	response, err = engine.SendBasicStreamPost(
		fmt.Sprintf("build?t=%s", imageFullName), headers, tarReader)
	fmt.Println("BuildImage: E")  // debug
	defer response.Body.Close()
	if err != nil { return "", err }
	fmt.Println("BuildImage: E1")  // debug
	if response.StatusCode != 200 {
		fmt.Println("Response message: " + response.Status)
		return "", errors.New(response.Status)
	}
	
	var bytes []byte
	bytes, err = ioutil.ReadAll(response.Body)
	fmt.Println("BuildImage: F")  // debug
	response.Body.Close()
	if err != nil { return "", err }
	var responseStr = string(bytes)
	fmt.Println("BuildImage: Z")  // debug
	
	return responseStr, nil
}
