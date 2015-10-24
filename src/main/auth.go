/*******************************************************************************
 * 
 */

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"io"
	"strings"
	"crypto/tls"
	"crypto/x509"
	"time"
	"errors"
)

type AuthService struct {
	Service string
	Sessions map[string]*Credentials  // map session key to Credentials.
	AuthServerName string
	AuthPort int
	AuthClient *http.Client
}

/*******************************************************************************
 * 
 */
func NewAuthService(serviceName string, authServerName string, authPort int,
	certPool *x509.CertPool) *AuthService {
	return &AuthService{
		Service: serviceName,
		Sessions: make(map[string]*Credentials),
		AuthServerName: authServerName,
		AuthPort: authPort,
		AuthClient: connectToAuthServer(certPool),
	}
}

/*******************************************************************************
 * Obtain the session token, if any; return nil otherwise.
 */
func (authSvc *AuthService) authenticateRequest(httpReq *http.Request) *SessionToken {
	var sessionToken *SessionToken = nil
	
	fmt.Println("authenticating request...")
	var sessionId = getSessionId(httpReq)
	fmt.Println("obtained session id:", sessionId)
	if sessionId != "" {
		sessionToken = authSvc.validateSessionId(sessionId)
	}
	//if sessionToken == nil {
		//var creds *Credentials = getSessionBasicAuthCreds(httpReq)
		//if creds != nil {
		//	sessionToken = authSvc.authenticateCredentials(creds)
		//}
	//}

	// Temporary code - 
	//var sessionId string = authSvc.createUniqueSessionId()
	//sessionToken = NewSessionToken(sessionId, "testuser1")
	//........Remove the above two lines!!!!!!!!!
	
	return sessionToken
}

/*******************************************************************************
 * Verify that the credentials match a registered user. If so, return a session
 * token that can be used to validate subsequent requests.
 */
func (authSvc *AuthService) authenticateCredentials(creds *Credentials) *SessionToken {
	
	/***************
	// Access the auth server to authenticate the credentials.
	if ! authSvc.sendQueryToAuthServer(creds, authSvc.Service,
		creds.userid, "", "", []string{}) { return nil }
	***************/
	
	var sessionId string = authSvc.createUniqueSessionId()
	var token *SessionToken = NewSessionToken(sessionId, creds.UserId)
	
	// Cache the new session token, so that this Server can recognize it in future
	// exchanges during this session.
	authSvc.Sessions[sessionId] = creds
	
	return token
}

/*******************************************************************************
 * Remove the specified session Id from the set of authenticated session Ids.
 */
func (authSvc *AuthService) invalidateSessionId(sessionId string) {
	authSvc.Sessions[sessionId] = nil
}

/*******************************************************************************
 * Check if the specified account is allowed to have access to the specified
 * resource. This function does not authenticate the
 * account - that is done by authenticateCredentials().
 * https://stackoverflow.com/questions/24496344/golang-send-http-request-with-certificate
 */
func (authSvc *AuthService) authorized(creds *Credentials, account string, 
	scope_type string, scope_name string, scope_actions []string) bool {

	return true  //....Remove!!!!!!!!!!!!!!
	

//	return authSvc.sendQueryToAuthServer(creds, authSvc.Service,
//		creds.userid, scope_type, scope_name, scope_actions)
}

/*******************************************************************************
 * Mask constants, for convenience.
 */
var CreateMask []bool = []bool{true, false, false, false, false}
var ReadMask []bool = []bool{false, true, false, false, false}
var WriteMask []bool = []bool{false, false, true, false, false}
var ExecuteMask []bool = []bool{false, false, false, true, false}
var DeleteMask []bool = []bool{false, false, false, false, true}


/*******************************************************************************
 * At most one field of the actionMask may be true.
 */
func authorized(server *Server, sessionToken *SessionToken, actionMask []bool,
	resourceId string) (bool, error) {

	/* Rules:
	
	A party can access a resource if the party,
		has an ACL entry for the resource; or,
		the resource belongs to a repo or realm for which the party has an ACL entry.
	
	In this context, a user is a party if the user is explicitly the party or if
	the user belongs to a group that is explicitly the party.
	
	Groups may not belong to other groups.
	
	The user must have the required access mode (Create, Read, Write, Exec, Delete).
	No access mode implies any other access mode.
	
	*/
	
	// Identify the user.
	var userId string = sessionToken.AuthenticatedUserid
	fmt.Println("userid=", userId)
	var user User = server.dbClient.dbGetUserByUserId(userId)
	if user == nil {
		return false, errors.New("user object cannot be identified from user id " + userId)
	}

	// Verify that at most one field of the actionMask is true.
	var nTrue = 0
	for _, b := range actionMask {
		if b {
			if nTrue == 1 {
				return false, errors.New("More than one field in mask may not be true")
			}
			nTrue++
		}
	}
	
	// Check if the user or a group that the user belongs to has the permission
	// that is specified by the actionMask.
	var party Party = user  // start with the user.
	var resource Resource = server.dbClient.getResource(resourceId)
	if resource == nil {
		return false, errors.New("Resource with Id " + resourceId + " not found")
	}
	var groupIds []string = user.getGroupIds()
	var i = -1
	for {
		var parentId string = resource.getParentId()
		var parent Resource
		if parentId != "" { parent = server.dbClient.getResource(parentId) }
		if server.partyHasAccess(party, actionMask, resource) ||
			(
				(parent != nil) && 
				(
					(parent.isRepo() && server.partyHasAccess(party, actionMask, parent)) ||
					(parent.isRealm() && server.partyHasAccess(party, actionMask, parent)))) {
			return true, nil
		}
		
		i++
		if i == len(groupIds) { return false, nil }
		party = server.dbClient.getParty(groupIds[i])  // check user's groups
		if party == nil {
			return false, errors.New("Internal error: Party with Id " + groupIds[i] + " not found")
		}
	}
}

/*******************************************************************************
 * Return true if the party has all of the rights implied by the actionMask, for
 * the specified Resource, based on the ACLEntries that the resource has. Do not
 * attempt to determine if the resource's owning Resource has applicable ACLEntries.
 */
func (server *Server) partyHasAccess(party Party, actionMask []bool, resource Resource) bool {
	
	var entries []string = party.getACLEntryIds()
	for _, entryId := range entries {
		
		if entryId == resource.getId() {
			var entry ACLEntry = server.dbClient.getACLEntry(entryId)
			if entry == nil {
				fmt.Println("Internal error: ACLEntry with Id " + entryId + " not found")
				return false
			}
			var mask []bool = entry.getPermissionMask()
			
			for i, b := range mask {
				if actionMask[i] && b {
					return true
				}
			}
			return false
		}
	}
	return false
}




/***************************** Internal Functions ******************************
 *******************************************************************************
 * Returns the session id header value, or "" if there is none.
 */
func getSessionId(httpReq *http.Request) string {
	assertThat(httpReq != nil, "In getSessionId, httpReq is nil")
	assertThat(httpReq.Header != nil, "In getSessionId, httpReq.Header is nil")
	
	if httpReq.Header["Session-Id"] == nil { // No authenticated session has been established.
		fmt.Println("No Session-Id header found; headers are:")
		for key, val := range httpReq.Header {
			fmt.Println("\t" + key + ": " + val[0])
		}
		return ""
	}
	assertThat(len(httpReq.Header["Session-Id"]) > 0, "In getSessionId, len(httpReq.Header[Session-Id]) == 0")
	var sessionId string = httpReq.Header["Session-Id"][0]
	if len(sessionId) == 0 { return "" }
	return sessionId
}

/*******************************************************************************
 * Return the userid and password from the HTTP header, or nil if not present.
 */
func getSessionBasicAuthCreds(httpReq *http.Request) *Credentials {
	userId, password, ok := httpReq.BasicAuth()
	if !ok { return nil }
	return NewCredentials(userId, password)
}

/*******************************************************************************
 * Validate the specified session id. If valid, return a SessionToken with
 * the identity of the session's owner.
 */
func (authSvc *AuthService) validateSessionId(sessionId string) *SessionToken {
	
	var credentials *Credentials = authSvc.Sessions[sessionId]
	
	if credentials == nil {
		fmt.Println("No credentials found for session id", sessionId)
		return nil
	}
	
	return NewSessionToken(sessionId, credentials.UserId)
}

/*******************************************************************************
 * Establish a TLS connection with the authentication/authorization server.
 * This connection is maintained.
 */
func connectToAuthServer(certPool *x509.CertPool) *http.Client {
	
	var tr *http.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: certPool},
		DisableCompression: true,
	}
	return &http.Client{Transport: tr}
}

/*******************************************************************************
 * Send an authentication or authorization request to the auth server. If successful,
 * return true, otherwise return false. This function encapsulates the HTTP messsage
 * format required by the auth server.
 */
func (authSvc *AuthService) sendQueryToAuthServer(creds *Credentials, 
	service string, account string,
	scope_type string, scope_name string, scope_actions []string) bool {
	
	// https://github.com/cesanta/docker_auth/blob/master/auth_server/server/server.go
	var urlstr string = fmt.Sprintf(
		"https://%s:%s/auth",
		authSvc.AuthServerName, authSvc.AuthPort)
	
	var request *http.Request
	var err error
	var actions string = strings.Join(scope_actions, ",")
	var scope string = fmt.Sprintf("%s%s%s", scope_type, scope_name, actions)
	var data url.Values = url.Values {
		"service": []string{service},
		"scope": []string{scope},
		"account": []string{creds.UserId},
	}
	var reader io.Reader = strings.NewReader(data.Encode())
	
	request, err = http.NewRequest("POST", urlstr, reader)
		if err != nil { panic(err) }
	request.SetBasicAuth(creds.UserId, creds.Password)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var resp *http.Response
	resp, err = authSvc.AuthClient.Do(request)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	if resp.StatusCode != 200 {
		fmt.Println(fmt.Sprintf("Response code %s", resp.StatusCode))
		return false
	}
	
	defer resp.Body.Close()
	
	return true
}

/*******************************************************************************
 * Return a session id that is guaranteed to be unique, and that is completely
 * opaque and unforgeable. ....To do: append a monotonically increasing value
 * (created atomically) to the string prior to encryption.
 */
func (authSvc *AuthService) createUniqueSessionId() string {
	return encrypt(time.Now().Local().String())
}

/*******************************************************************************
 * Encrypt the specified string. For now, just return the string.
 * ....To do: Need to complete this to use the Server's private key.
 */
func encrypt(s string) string {
	return s
}
