package docker



import (
	"fmt"
	"io"
	"os"
	"io/ioutil"
	"net/http"
	"archive/tar"
	"errors"
	"reflect"
	
	"safeharbor/rest"
)

type DockerEngine struct {
	rest.RestContext
}

/*******************************************************************************
 * 
 */
func OpenDockerEngineConnection(ssl bool, host string, port int, userId string,
	password string, sessionIdSetter func(*http.Request, string)) (*Engine, error) {

	var engine *DockerEngine = &DockerEngine{
		RestContext: *rest.CreateRestContext(false, host, port, userId, password, sessionIdSetter),
	}
	
	var err error = engine.Ping()
	if err != nil {
		return nil, err
	}
	
	return engine, nil
}

/*******************************************************************************
 * 
 */
func (engine *DockerEngine) BuildImage(buildDirPath, imageFullName string) error {

	cmd = exec.Command("docker", "build", 
		"--file", buildDirPath + "/" + dockerfileName,
		"--tag", imageFullName, buildDirPath)

	// https://docs.docker.com/engine/reference/api/docker_remote_api_v1.23/#build-image-from-a-dockerfile
	// POST /build HTTP/1.1
	//
	// {{ TAR STREAM }} (this is the contents of the "build context")
	var resp *http.Response
	var err error
	resp, err = engine.SendBasicPost("/build", )
}

