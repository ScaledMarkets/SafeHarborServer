/*******************************************************************************
 * Program configuration. A configuration is built by parsing a configuration file. The
 * getConfiguration function (in Server.go) retrieves the path of the configuration 
 * file and creates a File that is passed to the NewConfiguration function here. That
 * function parses the file, creates, and returns a Configuration struct.
 */

package server

import (
	"os"
	"io"
	"fmt"
	"encoding/json"
	"strings"
	"strconv"
	"reflect"
	
	//"safeharbor/apitypes"
)

type Configuration struct {
	service string
	ipaddr string
	netIntfName string // e.g., eth0, en1, etc.
	port int
	RedisHost string
	RedisPort int
	RedisPswd string
	LocalAuthCertPath string
	LocalRootCertPath string // may be null
	AuthServerName string
	AuthPort int
	AuthCertPath string
	AuthKeyPath string
	FileRepoRootPath string // where Dockerfiles, images, etc. are stored
	ScanServices map[string]interface{}
}

/*******************************************************************************
 * Parse JSON input to build a configuration structure.
 * See https://stackoverflow.com/questions/16465705/how-to-handle-configuration-in-go
 */
func NewConfiguration(file *os.File) (*Configuration, error) {
	config := new(Configuration)
	
	fileInfo, err := file.Stat()
	if err != nil { return nil, err }
	
	var size int64 = fileInfo.Size()
	var data = make([]byte, size)
	n, err := io.ReadFull(file, data)
	if err != nil { return nil, err }
	if int64(n) != size { return nil, fmt.Errorf("Num bytes read does not match file size") }
	fmt.Println(fmt.Sprintf("Read %d bytes from configuration file", size))
	
	var entries = make(map[string]interface{})
	err = json.Unmarshal(data, &entries)
	if err != nil { return nil, err }
	fmt.Println("Parsed configuration file")

	var exists bool
	
	config.netIntfName, exists = entries["INTFNAME"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find INTFNAME in configuration") }
	
	var portStr string
	portStr, exists = entries["PORT"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find PORT in configuration") }
	config.port, err = strconv.Atoi(portStr)
	if err != nil { return nil, fmt.Errorf("PORT value in configuration is not an integer") }
	
	config.LocalAuthCertPath, exists = entries["LOCAL_AUTH_CERT_PATH"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find LOCAL_AUTH_CERT_PATH in configuration") }
	
	config.LocalRootCertPath, exists = entries["LOCAL_ROOT_CERT_PATH"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find LOCAL_ROOT_CERT_PATH in configuration") }
	
	config.FileRepoRootPath, exists = entries["FILE_REPOSITORY_ROOT"].(string)
	if ! exists { config.FileRepoRootPath = "Repository" }
	config.FileRepoRootPath = strings.TrimRight(config.FileRepoRootPath, "/ ")
	
	config.RedisHost, _ = entries["REDIS_HOST"].(string)
	
	var redisPortStr string
	redisPortStr, exists = entries["REDIS_PORT"].(string)
	if exists {
		config.RedisPort, err = strconv.Atoi(redisPortStr)
		if err != nil { return nil, fmt.Errorf("REDIS_PORT value in configuration is not an integer") }
	} else {
		config.RedisPort = 0
	}
	
	config.RedisPswd, exists = entries["REDIS_PASSWORD"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find REDIS_PASSWORD in configuration") }
	
	//config.service, exists = entries["SERVICE"]
	//if ! exists { return nil, fmt.Errorf("Did not find SERVICE in configuration") }
	
	//config.AuthServerName, exists = entries["AUTH_SERVER_DNS_NAME"]
	//if ! exists { return nil, fmt.Errorf("Did not find AUTH_SERVER_DNS_NAME in configuration") }
	
	//portStr, exists = entries["AUTH_PORT"]
	//if ! exists { return nil, fmt.Errorf("Did not find AUTH_PORT in configuration") }
	//config.AuthPort, err = strconv.Atoi(portStr)
	//if err != nil { return nil, fmt.Errorf("AUTH_PORT value in configuration is not an integer") }
	
	//config.AuthCertPath, exists = entries["AUTH_CERT_PATH"]
	//if ! exists { return nil, fmt.Errorf("Did not find AUTH_CERT_PATH in configuration") }
	
	//config.AuthKeyPath, exists = entries["AUTH_PRIVATE_KEY_PATH"]
	//if ! exists { return nil, fmt.Errorf("Did not find AUTH_PRIVATE_KEY_PATH in configuration") }
	
	var obj interface{}
	obj, exists = entries["ScanServices"]
	if ! exists { return nil, fmt.Errorf("Did not find ScanServices in configuration") }
	var isType bool
	config.ScanServices, isType = obj.(map[string]interface{})
	if ! isType {
		fmt.Println("ScanServices is a", reflect.TypeOf(obj))
		return nil, fmt.Errorf("Scan configuration is ill-formatted")
	}
	
	fmt.Println("Configuration values obtained")
	return config, nil
}
