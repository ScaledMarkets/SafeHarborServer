package main

/*
Program configuration. A configuration is built by parsing a configuration. The
main function retrieves the path of the configuration file and creates a reader
that is passed to the newConfiguration function here. That function parses the
file, creates, and returns a configuration struct.
*/


import (
	"os"
	"io"
	"fmt"
	"encoding/json"
)

type Configuration struct {
	ipaddr string
	port string
	LocalAuthCertPath string
	AuthServerName string
	AuthPort string
	AuthCertPath string
	AuthKeyPath string
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
	
	config.ipaddr, exists = entries["IPADDR"]
	if ! exists { return nil, fmt.Errorf("Did not find IPADDR in configuration") }
	
	config.port, exists = entries["PORT"]
	if ! exists { return nil, fmt.Errorf("Did not find PORT in configuration") }
	
	config.LocalAuthCertPath, exists = entries["LOCAL_AUTH_CERT_PATH"]
	if ! exists { return nil, fmt.Errorf("Did not find LOCAL_AUTH_CERT_PATH in configuration") }
	
	config.AuthServerName, exists = entries["AUTH_SERVER_DNS_NAME"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_SERVER_DNS_NAME in configuration") }
	
	config.AuthPort, exists = entries["AUTH_PORT"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_PORT in configuration") }
	
	config.AuthCertPath, exists = entries["AUTH_CERT_PATH"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_CERT_PATH in configuration") }
	
	config.AuthKeyPath, exists = entries["AUTH_KEY_PATH"]
	if ! exists { return nil, fmt.Errorf("Did not find AUTH_KEY_PATH in configuration") }
	
	fmt.Println("Configuration values obtained")
	return config, nil
}
