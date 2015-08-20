/*******************************************************************************
 * Dispatch incoming HTTP requests to the appropriate function.
 */

package main

import (
	"net/http"
	"net/url"
	"io"
	"fmt"
)

/*******************************************************************************
 * All request handlers are of this type.
 * The string arguments are in pairs, where the first is the name of the arg,
 * and the second is the string value.
 */
type ReqHandlerFuncType func (*Server, *SessionToken, url.Values) ResponseInterfaceType

/*******************************************************************************
 * The Dispatcher is a singleton struct that contains a map from request name
 * to request handler function.
 */
type Dispatcher struct {
	server *Server
	handlers map[string]ReqHandlerFuncType
}

/*******************************************************************************
 * 
 */
func NewDispatcher() *Dispatcher {

	hdlrs := map[string]ReqHandlerFuncType{
		"authenticate": authenticate,
		"logout": logout,
		"createUser": createUser,
		"deleteUser": deleteUser,
		"getMyGroups": getMyGroups,
		"createGroup": createGroup,
		"deleteGroup": deleteGroup,
		"getGroupUsers": getGroupUsers,
		"addGroupUser": addGroupUser,
		"remGroupUser": remGroupUser,
		//"createRealm": xyz,
		"createRealm": createRealm,
		"deleteRealm": deleteRealm,
		"addRealmUser": addRealmUser,
		"getRealmGroups": getRealmGroups,
		"addRealmGroup": addRealmGroup,
		"getRealmRepos": getRealmRepos,
		"getMyRealms": getMyRealms,
		"scanImage": scanImage,
		"createRepo": createRepo,
		"deleteRepo": deleteRepo,
		"getMyRepos": getMyRepos,
		"getDockerfiles": getDockerfiles,
		"getImages": getImages,
		"addDockerfile": addDockerfile,
		"replaceDockerfile": replaceDockerfile,
		"buildDockerfile": buildDockerfile,
		"downloadImage": downloadImage,
		"sendImage": sendImage,
	}
	
	var dispatcher *Dispatcher = &Dispatcher{
		server: nil,  // must be filled in by server
		handlers: hdlrs,
	}
	
	return dispatcher
}


/*******************************************************************************
 * Invoke the method specified by the REST request.
 */
func (dispatcher *Dispatcher) handleRequest(sessionToken *SessionToken,
	w http.ResponseWriter, reqName string, values url.Values) {

	fmt.Printf("Dispatcher: handleRequest for '%s'\n", reqName)
	var handler, found = dispatcher.handlers[reqName]
	if ! found {
		fmt.Printf("No method found, %s\n", reqName)
		respondNoSuchMethod(w, reqName)
		return
	}
	if handler == nil {
		fmt.Println("Handler is nil!!!")
		return
	}
	fmt.Println("Calling handler")
	var result ResponseInterfaceType = handler(dispatcher.server, sessionToken, values)
	fmt.Println("Returning result:", result.asResponse())
	returnOkResponse(w, result)
	fmt.Printf("Handled %s\n", reqName)
}

/*******************************************************************************
 * Generate a 200 HTTP response by converting the result into a
 * string consisting of name=value lines.
 */
func returnOkResponse(writer http.ResponseWriter, result ResponseInterfaceType) {
	var response string = result.asResponse()
	fmt.Println("Response:")
	fmt.Println(response)
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	io.WriteString(writer, response)
}

/*******************************************************************************
 * 
 */
func respondNoSuchMethod(w http.ResponseWriter, methodName string) {
	//....
}
