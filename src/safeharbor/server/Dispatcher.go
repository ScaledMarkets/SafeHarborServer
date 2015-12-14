/*******************************************************************************
 * Dispatch incoming HTTP requests to the appropriate function.
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
)

/*******************************************************************************
 * All request handler functions are of this type.
 * The string arguments are in pairs, where the first is the name of the arg,
 * and the second is the string value.
 */
type ReqHandlerFuncType func (*Server, *apitypes.SessionToken, url.Values,
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
		"createGroup": createGroup,
		"deleteGroup": deleteGroup,
		"getGroupUsers": getGroupUsers,
		"addGroupUser": addGroupUser,
		"remGroupUser": remGroupUser,
		"createRealmAnon": createRealmAnon,
		"createRealm": createRealm,
		"getRealmDesc": getRealmDesc,
		"deactivateRealm": deactivateRealm,
		"addRealmUser": addRealmUser,
		"getRealmUsers": getRealmUsers,
		"remRealmUser": remRealmUser,
		"getRealmUser": getRealmUser,
		"getRealmGroups": getRealmGroups,
		"getRealmRepos": getRealmRepos,
		"getAllRealms": getAllRealms,
		"createRepo": createRepo,
		"deleteRepo": deleteRepo,
		"getDockerfiles": getDockerfiles,
		"getImages": getImages,
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
		"replaceScanConfig": replaceScanConfig,
		"scanImage": scanImage,
		"getUserEvents": getUserEvents,
		"getImageEvents": getImageEvents,
		"getImageStatus": getImageStatus,
		"getDockerfileEvents": getDockerfileEvents,
		"defineFlag": defineFlag,
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

	fmt.Println("------------------------")
	fmt.Printf("Dispatcher: handleRequest for '%s'\n", reqName)
	var handler, found = dispatcher.handlers[reqName]
	if ! found {
		fmt.Printf("No method found, %s\n", reqName)
		respondNoSuchMethod(headers, w, reqName)
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
		printHTTPParameters(values)
	}
	var result apitypes.RespIntfTp = handler(dispatcher.server, sessionToken, values, files)
	fmt.Println("Returning result:", result.AsJSON())
	
	// Detect whether an error occurred.
	failureDesc, isType := result.(*apitypes.FailureDesc)
	if isType {
		http.Error(w, failureDesc.AsJSON(), failureDesc.HTTPCode)
		fmt.Printf("Error:", failureDesc.Reason)
		return
	}
	
	returnOkResponse(headers, w, result)
	fmt.Printf("Handled %s\n", reqName)
	fmt.Println()
}

/*******************************************************************************
 * Generate a 200 HTTP response by converting the result into a
 * string consisting of name=value lines.
 */
func returnOkResponse(headers http.Header, writer http.ResponseWriter, result apitypes.RespIntfTp) {
	var jsonResponse string = result.AsJSON()
	
	if jsonResponse == "" {
		var filePath string = result.SendFile()
		if filePath == "" {
			io.WriteString(writer, "Internal error: No JSON response or file path in result")
			return
		}
		// Write the file to the response writer. It is assumed that the file is
		// a temp file.
		f, err := os.Open(path)
		defer os.Remove(filePath)
		if err != nil {
			io.WriteString(writer, err.Error())
			return
		}
		_, err = io.Copy(writer, f)
		
		if err != nil {
			io.WriteString(writer, err.Error())
			return
		}
	} else {
		fmt.Println("Response:")
		fmt.Println(jsonResponse)
		writer.WriteHeader(http.StatusOK)
		io.WriteString(writer, jsonResponse)
	}
}

/*******************************************************************************
 * 
 */
func respondNoSuchMethod(headers http.Header, writer http.ResponseWriter, methodName string) {
	
	writer.WriteHeader(404)
	io.WriteString(writer, "No such method," + methodName)
}

/*******************************************************************************
 * 
 */
func printHTTPParameters(values url.Values) {
	// Values is a map[string][]string
	fmt.Println("HTTP parameters:")
	for k, v := range values {
		fmt.Println(k + ": '" + v[0] + "'")
	}
}
