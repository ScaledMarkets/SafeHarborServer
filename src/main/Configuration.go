/*******************************************************************************
 * Program configuration. A configuration is built by parsing a configuration. The
 * getConfiguration function (in Server.go) retrieves the path of the configuration 
 * file and creates a File that is passed to the NewConfiguration function here. That
 * function parses the file, creates, and returns a configuration struct.
 */

package main

import (
	"os"
	"io"
	"fmt"
	"encoding/json"
	"strings"
)

type Configuration struct {
	service string
	ipaddr string
	port string
	LocalAuthCertPath string
	LocalRootCertPath string // may be null
	AuthServerName string
	AuthPort string
	AuthCertPath string
	AuthKeyPath string
	FileRepoRootPath string // where Dockerfiles, images, etc. are stored
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
	
	var entries = make(map[string]string)
	err = json.Unmarshal(data, &entries)
	if err != nil { return nil, err }
	fmt.Println("Parsed configuration file")

	var exists bool
	
	config.service, exists = entries["SERVICE"]
	if ! exists { return nil, fmt.Errorf("Did not find SERVICE in configuration") }
	
	config.ipaddr, exists = entries["IPADDR"]
	if ! exists { return nil, fmt.Errorf("Did not find IPADDR in configuration") }
	
	config.port, exists = entries["PORT"]
	if ! exists { return nil, fmt.Errorf("Did not find PORT in configuration") }
	
	config.LocalAuthCertPath, exists = entries["LOCAL_AUTH_CERT_PATH"]
	if ! exists { return nil, fmt.Errorf("Did not find LOCAL_AUTH_CERT_PATH in configuration") }
	
	config.LocalRootCertPath, exists = entries["LOCAL_ROOT_CERT_PATH"]
	if ! exists { return nil, fmt.Errorf("Did not find LOCAL_ROOT_CERT_PATH in configuration") }
	
	config.AuthServerName, exists = entries["AUTH_SERVER_DNS_NAME"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_SERVER_DNS_NAME in configuration") }
	
	config.AuthPort, exists = entries["AUTH_PORT"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_PORT in configuration") }
	
	config.AuthCertPath, exists = entries["AUTH_CERT_PATH"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_CERT_PATH in configuration") }
	
	config.AuthKeyPath, exists = entries["AUTH_KEY_PATH"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_KEY_PATH in configuration") }
	
	config.FileRepoRootPath, exists = entries["FILE_REPOSITORY_ROOT"]
	if ! exists { return nil, fmt.Errorf("Did not find FILE_REPOSITORY_ROOT in configuration") }
	config.FileRepoRootPath = strings.TrimRight(config.FileRepoRootPath, "/ ")
	
	fmt.Println("Configuration values obtained")
	return config, nil
}
