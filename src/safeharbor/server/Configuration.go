/*******************************************************************************
 * Program configuration. A configuration is built by parsing a configuration file. The
 * getConfiguration function (in Server.go) retrieves the path of the configuration 
 * file and creates a File that is passed to the NewConfiguration function here. That
 * function parses the file, creates, and returns a Configuration struct.
 * If a configuration value begins with a dollar sign ($), it is assumed to be an
 * environment variable, and the system environment is checked for the variable: if
 * it is found, the value is replaced with the environment variable value; if not
 * found, an error results.
 *
 * Copyright Scaled Markets, Inc.
 */

package server

import (
	"errors"
	"os"
	"io"
	"fmt"
	"encoding/json"
	"strings"
	"strconv"
	"reflect"
	
	//"safeharbor/apitypes"
	//"utilities/utils"
)

type Configuration struct {
	service string
	PublicHostname string
	ipaddr string
	netIntfName string // e.g., eth0, en1, etc.
	port int
	RedisHost string
	RedisPort int
	RedisPswd string
	RegistryHost string
	RegistryPort int
	RegistryUserId string
	RegistryPassword string
	LocalAuthCertPath string
	LocalRootCertPath string // may be null
	AuthServerName string
	AuthPort int
	AuthCertPath string
	AuthKeyPath string
	FileRepoRootPath string // where Dockerfiles, images, etc. are stored
	ScanServices map[string]interface{}
	EmailService map[string]interface{}
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

	var rawValue string
	var stringValue string
	var exists bool
	var isType bool
	
	// INTFNAME
	rawValue, exists = entries["INTFNAME"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find INTFNAME in configuration") }
	config.netIntfName, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// PUBLIC_HOSTNAME
	rawValue, exists = entries["PUBLIC_HOSTNAME"].(string)
	if exists {
		config.PublicHostname, err = substituteEnvValue(rawValue)
		if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	}
	
	// PORT
	rawValue, exists = entries["PORT"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find PORT in configuration") }
	stringValue, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	config.port, err = strconv.Atoi(stringValue)
	if err != nil { return nil, fmt.Errorf("PORT value in configuration is not an integer") }
	
	// LOCAL_AUTH_CERT_PATH
	rawValue, exists = entries["LOCAL_AUTH_CERT_PATH"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find LOCAL_AUTH_CERT_PATH in configuration") }
	config.LocalAuthCertPath, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// LOCAL_ROOT_CERT_PATH
	rawValue, exists = entries["LOCAL_ROOT_CERT_PATH"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find LOCAL_ROOT_CERT_PATH in configuration") }
	config.LocalRootCertPath, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// FILE_REPOSITORY_ROOT
	rawValue, exists = entries["FILE_REPOSITORY_ROOT"].(string)
	if exists {
		stringValue, err = substituteEnvValue(rawValue)
		if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
		config.FileRepoRootPath = strings.TrimRight(stringValue, "/ ")
	} else {
		config.FileRepoRootPath = "Repository"
	}
	
	// REDIS_HOST
	rawValue, _ = entries["REDIS_HOST"].(string)
	config.RedisHost, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// REDIS_PORT
	rawValue, exists = entries["REDIS_PORT"].(string)
	if exists {
		var redisPortStr string
		redisPortStr, err = substituteEnvValue(rawValue)
		if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
		config.RedisPort, err = strconv.Atoi(redisPortStr)
		if err != nil { return nil, fmt.Errorf("REDIS_PORT value in configuration is not an integer") }
	} else {
		config.RedisPort = 0
	}
	
	// REDIS_PASSWORD
	rawValue, exists = entries["REDIS_PASSWORD"].(string)
	if ! exists { return nil, fmt.Errorf("Did not find REDIS_PASSWORD in configuration") }
	config.RedisPswd, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// REGISTRY_HOST
	rawValue, exists = entries["REGISTRY_HOST"].(string)
	config.RegistryHost, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// REGISTRY_PORT
	rawValue, exists = entries["REGISTRY_PORT"].(string)
	if exists {
		stringValue, err = substituteEnvValue(rawValue)
		if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
		config.RegistryPort, err = strconv.Atoi(stringValue)
		if err != nil { return nil, fmt.Errorf(
			"REGISTRY_PORT value in configuration is not an integer")
		}
	}
	
	// REGISTRY_USERID
	rawValue, exists = entries["REGISTRY_USERID"].(string)
	config.RegistryUserId, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// REGISTRY_PASSWORD
	rawValue, exists = entries["REGISTRY_PASSWORD"].(string)
	config.RegistryPassword, err = substituteEnvValue(rawValue)
	if err != nil { return nil, errors.New("Did not find environment variable " + rawValue) }
	
	// ScanServices
	var obj interface{}
	obj, exists = entries["ScanServices"]
	if ! exists { return nil, fmt.Errorf("Did not find ScanServices in configuration") }
	config.ScanServices, isType = obj.(map[string]interface{})
	if ! isType {
		fmt.Println("ScanServices is a", reflect.TypeOf(obj))
		return nil, fmt.Errorf("Scan configuration is ill-formatted")
	}
	for _, obj := range config.ScanServices { // each attribute of scanServices
		var svcParams map[string]interface{}
		svcParams, isType = obj.(map[string]interface{})
		if ! isType { return nil, errors.New("Scan service config is invalid") }
		for key, value := range svcParams { // each attribute of the object
			stringValue, isType = value.(string)
			if ! isType { return nil, errors.New("Parameter for " + key + " is not a string") }
			svcParams[key], err = substituteEnvValue(stringValue)
			if err != nil { return nil, err }
		}
	}
	
	// Email service.
	obj, exists = entries["EmailService"]
	if ! exists { return nil, fmt.Errorf("Did not find EmailService in configuration") }
	config.EmailService, isType = obj.(map[string]interface{})
	if ! isType {
		fmt.Println("EmailService is a", reflect.TypeOf(obj))
		return nil, fmt.Errorf("Email configuration is ill-formatted")
	}
	for key, value := range config.EmailService { // each attribute of emailService
		stringValue, isType = value.(string)
		if ! isType { return nil, errors.New("Parameter for " + key + " is not a string") }
		config.EmailService[key], err = substituteEnvValue(stringValue)
		if err != nil { return nil, err }
	}
	
	fmt.Println("Configuration values obtained")
	return config, nil
}

/*******************************************************************************
 * If the raw value begins with a dollar sign ($), assume that it is an environment
 * variable reference: search the environment for the variable. If found, return it,
 * otherwise, return an error.
 */
func substituteEnvValue(rawValue string) (string, error) {
	
	if strings.HasPrefix(rawValue, "$") {
		
		var clippedValue string = rawValue[1:]
		var value string
		var found bool
		value, found = os.LookupEnv(clippedValue)
		if ! found {
			return "", errors.New("Environment variable " + clippedValue + " not found")
		}
		return value, nil
		
	} else {
		return rawValue, nil
	}
}
