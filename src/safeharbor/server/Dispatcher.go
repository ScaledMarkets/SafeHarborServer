/*******************************************************************************
 * Dispatch incoming HTTP requests to the appropriate REST function.
 */

package server

import (
	"net/http"
	"mime/multipart"
	"net/url"
	"io"
	"fmt"
	"os"
	//"errors"
	
	"safeharbor/apitypes"
	//"safeharbor/util"
)

/*******************************************************************************
 * All request handler functions are of this type.
 * The string arguments are in pairs, where the first is the name of the arg,
 * and the second is the string value.
 */
type ReqHandlerFuncType func (*InMemClient, *apitypes.SessionToken, url.Values,
	map[string][]*multipart.FileHeader) apitypes.RespIntfTp

/*******************************************************************************
 * The Dispatcher is a singleton struct that contains a map from request name
 * to request handler function.
 */
type Dispatcher struct {
	server *Server
	handlers map[string]ReqHandlerFuncType
}

/*******************************************************************************
 * Create a new dispatcher for dispatching to REST handlers. This is often
 * called "muxing", but the implementation here is simpler, clearer and more
 * maintainable, and faster.
 */
func NewDispatcher() *Dispatcher {

	// Map of REST request names to handler functions. These functions are all
	// defined in Handlers.go.
	hdlrs := map[string]ReqHandlerFuncType{
		"ping": ping,
		"clearAll": clearAll,
		"printDatabase": printDatabase,
		"authenticate": authenticate,
		"logout": logout,
		"createUser": createUser,
		"disableUser": disableUser,
		"reenableUser": reenableUser,
		"changePassword": changePassword,
		"createGroup": createGroup,
		"deleteGroup": deleteGroup,
		"getGroupUsers": getGroupUsers,
		"addGroupUser": addGroupUser,
		"remGroupUser": remGroupUser,
		"createRealmAnon": createRealmAnon,
		"createRealm": createRealm,
		"getRealmDesc": getRealmDesc,
		"deactivateRealm": deactivateRealm,
		"moveUserToRealm": moveUserToRealm,
		"getRealmUsers": getRealmUsers,
		"getUserDesc": getUserDesc,
		"getRealmGroups": getRealmGroups,
		"getRealmRepos": getRealmRepos,
		"getAllRealms": getAllRealms,
		"createRepo": createRepo,
		"deleteRepo": deleteRepo,
		"getDockerfiles": getDockerfiles,
		"getDockerImages": getDockerImages,
		"addDockerfile": addDockerfile,
		"replaceDockerfile": replaceDockerfile,
		"execDockerfile": execDockerfile,
		"addAndExecDockerfile": addAndExecDockerfile,
		"downloadImage": downloadImage,
		"setPermission": setPermission,
		"addPermission": addPermission,
		"remPermission": remPermission,
		"getPermission": getPermission,
		"getMyDesc": getMyDesc,
		"getMyGroups": getMyGroups,
		"getMyRealms": getMyRealms,
		"getMyRepos": getMyRepos,
		"getMyDockerfiles": getMyDockerfiles,
		"getMyDockerImages": getMyDockerImages,
		"getScanProviders": getScanProviders,
		"defineScanConfig": defineScanConfig,
		"updateScanConfig": updateScanConfig,
		"scanImage": scanImage,
		"getUserEvents": getUserEvents,
		"getDockerImageEvents": getDockerImageEvents,
		"getDockerImageStatus": getDockerImageStatus,
		"getDockerfileEvents": getDockerfileEvents,
		"defineFlag": defineFlag,
		"getGroupDesc": getGroupDesc,
		"getRepoDesc": getRepoDesc,
		"getDockerImageDesc": getDockerImageDesc,
		"getDockerfileDesc": getDockerfileDesc,
		"getScanConfigDesc": getScanConfigDesc,
		"getFlagDesc": getFlagDesc,
		"getFlagImage": getFlagImage,
		"getMyScanConfigs": getMyScanConfigs,
		"getScanConfigDescByName": getScanConfigDescByName,
		"remScanConfig": remScanConfig,
		"getMyFlags": getMyFlags,
		"getFlagDescByName": getFlagDescByName,
		"remFlag": remFlag,
		"remDockerImage": remDockerImage,
	}
	
	var dispatcher *Dispatcher = &Dispatcher{
		server: nil,  // must be filled in by server
		handlers: hdlrs,
	}
	
	return dispatcher
}

/*******************************************************************************
 * Invoke the method specified by the REST request. This is called by the
 * Server dispatch method.
 */
func (dispatcher *Dispatcher) handleRequest(sessionToken *apitypes.SessionToken,
	headers http.Header, w http.ResponseWriter, reqName string, values url.Values,
	files map[string][]*multipart.FileHeader) {

	fmt.Printf("Dispatcher: handleRequest for '%s'\n", reqName)
	var handler, found = dispatcher.handlers[reqName]
	if ! found {
		fmt.Printf("No method found, %s\n", reqName)
		dispatcher.respondNoSuchMethod(headers, w, reqName)
		return
	}
	if handler == nil {
		fmt.Println("Handler is nil!!!")
		return
	}
	var curdir string
	var err error
	curdir, err = os.Getwd()
	if err != nil { fmt.Println(err.Error()) }
	if dispatcher.server.Debug { fmt.Println("Cur dir='" + curdir + "'") }
	fmt.Println("Calling handler")
	if sessionToken == nil { fmt.Println("handleRequest: Session token is nil") }
	if dispatcher.server.Debug {
		dispatcher.printHTTPParameters(values)
	}
	
	// Start a transaction.
	var server = dispatcher.server
	var inMemClient *InMemClient
	inMemClient, err = NewInMemClient(server)
	if err != nil {
		dispatcher.returnSystemErrorResponse(headers, w, err.Error())
		return
	}
	var inMemClients = []*InMemClient{ inMemClient }
		// We created an array, because that is the only way to get the defer
		// statement to defer evaluating inMemClient in the function below:
	defer func() {
		var inMemClient = inMemClients[0]
		if inMemClient != nil {
			inMemClient.abort()
			inMemClient = nil
		}
	}()
	
	var result apitypes.RespIntfTp = handler(inMemClients[0], sessionToken, values, files)
	fmt.Println("Returning result:", result.AsJSON())
	
	// Detect whether an error occurred.
	failureDesc, isType := result.(*apitypes.FailureDesc)
	if isType {
		fmt.Printf("Error:", failureDesc.Reason)
		http.Error(w, failureDesc.AsJSON(), failureDesc.HTTPCode)
		
		// Abort transaction.
		inMemClients[0].abort()
		inMemClient = nil
		
		return
	}
	
	// Commit transaction.
	err = inMemClients[0].commit()
	inMemClients[0] = nil
	inMemClients = nil
	if err != nil {
		dispatcher.returnSystemErrorResponse(headers, w, err.Error())
		return
	}
	
	dispatcher.returnOkResponse(headers, w, result)
	
	fmt.Printf("Handled %s\n", reqName)
}

/*******************************************************************************
 * Generate a 200 HTTP response by converting the result into a
 * string consisting of name=value lines.
 */
func (dispatcher *Dispatcher) returnOkResponse(headers http.Header, writer http.ResponseWriter, result apitypes.RespIntfTp) {
	var jsonResponse string = result.AsJSON()
	
	if jsonResponse == "" {
		var filePath string
		var deleteAfter bool
		filePath, deleteAfter = result.SendFile()
		if filePath == "" {
			io.WriteString(writer, "Internal error: No JSON response or file path in result")
			return
		}
		// Write the file to the response writer. It is assumed that the file is
		// a temp file.
		f, err := os.Open(filePath)
		if deleteAfter {
			if dispatcher.server.Debug {
				// Copy file to a scratch area before deleting it.
				defer func() {
					err = os.MkdirAll("temp", os.ModePerm)
					if err != nil { fmt.Println(err.Error()); return }
					
					var fileToCopy *os.File
					fileToCopy, err = os.Open(filePath)
					defer os.Remove(filePath)
					if err != nil { fmt.Println(err.Error()); return }
					var fileInfo os.FileInfo
					fileInfo, err = fileToCopy.Stat()
					if err != nil { fmt.Println(err.Error()); return }
					var scratchFilePath = "temp/" + fileInfo.Name()
					
					var scratchFile *os.File
					scratchFile, err = os.Create(scratchFilePath)
					defer scratchFile.Close()
					if err != nil { fmt.Println(err.Error()); return }
					
					var buf = make([]byte, 10000)
					for {
						var numBytesRead int
						numBytesRead, err = fileToCopy.Read(buf)
						if (numBytesRead == 0) || (err != nil) { break }

						_, err = scratchFile.Write(buf[0:numBytesRead])
						if (err != nil) { fmt.Println(err.Error()); return }
					}
				}()
			} else {
				defer func() {
					fmt.Println("Removing file " + filePath)
					os.Remove(filePath)
				}()
			}
		}
		if err != nil {
			io.WriteString(writer, err.Error())
			return
		}
		
		writer.Header().Set("Content-Type", "application/octet-stream")
		
		_, err = io.Copy(writer, f)
		
		if err != nil {
			io.WriteString(writer, err.Error())
			return
		}
	} else {
		fmt.Println("Response:")
		fmt.Println(jsonResponse)
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		io.WriteString(writer, jsonResponse)
	}
}

/*******************************************************************************
 * 
 */
func (dispatcher *Dispatcher) respondNoSuchMethod(headers http.Header,
	writer http.ResponseWriter, methodName string) {
	
	var msg = "No such method," + methodName
	writer.WriteHeader(404)
	io.WriteString(writer, msg)
	fmt.Println(msg)
}

/*******************************************************************************
 * 
 */
func (dispatcher *Dispatcher) returnSystemErrorResponse(headers http.Header,
	writer http.ResponseWriter, msg string) {

	writer.WriteHeader(500)
	io.WriteString(writer, msg)
	fmt.Println(msg)
}

/*******************************************************************************
 * 
 */
func (dispatcher *Dispatcher) printHTTPParameters(values url.Values) {
	// Values is a map[string][]string
	fmt.Println("HTTP parameters:")
	for k, v := range values {
		if k != "Log" { fmt.Println(k + ": '" + v[0] + "'") }
	}
}
