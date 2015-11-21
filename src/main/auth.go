/*******************************************************************************
 * Authentication and authorization.
 */

package main

import (
	"fmt"
	"net/http"
	//"net/url"
	//"io"
	"strings"
	//"crypto/tls"
	"crypto/x509"
	"time"
	"errors"
	"crypto/sha512"
	"hash"
)

type AuthService struct {
	Service string
	Sessions map[string]*Credentials  // map session key to Credentials.
	//DockerRegistry2AuthServerName string
	//DockerRegistry2AuthPort int
	//DockerRegistry2AuthSvc *http.Client
	secretSalt []byte
}

/*******************************************************************************
 * 
 */
func NewAuthService(serviceName string, authServerName string, authPort int,
	certPool *x509.CertPool, secretSalt string) *AuthService {

	return &AuthService{
		Service: serviceName,
		Sessions: make(map[string]*Credentials),
		//DockerRegistry2AuthServerName: authServerName,
		//DockerRegistry2AuthPort: authPort,
		//DockerRegistry2AuthSvc: connectToAuthServer(certPool),
		secretSalt: []byte(secretSalt),
	}
}

/*******************************************************************************
 * Clear all sessions that are cached in the auth service.
 */
func (authSvc *AuthService) clearAllSessions() {
	authSvc.Sessions = make(map[string]*Credentials)
}

/*******************************************************************************
 * Create a new user session. This presumes that the credentials have been verified.
 */
func (authSvc *AuthService) createSession(creds *Credentials) *SessionToken {
	
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
 * Verify that a request belongs to a valid session:
 * Obtain the session Id, if any, and validate it; return nil if no Id found
 * or the Id is not valid.
 */
func (authSvc *AuthService) authenticateRequest(httpReq *http.Request) *SessionToken {
	
	var sessionToken *SessionToken = nil
	
	fmt.Println("authenticating request...")
	var sessionId = getSessionId(httpReq)
	fmt.Println("obtained session id:", sessionId)
	if sessionId != "" {
		sessionToken = authSvc.validateSessionId(sessionId)  // returns nil if invalid
	}
	
	return sessionToken
}

/*******************************************************************************
 * 
 */

func (authSvc *AuthService) addSessionIdToResponse(sessionToken *SessionToken,
	writer http.ResponseWriter) {

	authSvc.setSessionId(sessionToken, writer)
}

/*******************************************************************************
 * Determine if a specified action is allowed on a specified resource.
 * The ACL set for the resource is used to make the determination.
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


/***************************** Internal Functions ******************************/


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

/*******************************************************************************
 * Returns the session id header value, or "" if there is none.
 * Used by authenticateRequest.
 */
func getSessionId(httpReq *http.Request) string {
	assertThat(httpReq != nil, "In getSessionId, httpReq is nil")
	assertThat(httpReq.Header != nil, "In getSessionId, httpReq.Header is nil")
	
	var cookie *http.Cookie
	var err error
	cookie, err = httpReq.Cookie("SessionId")
	if err != nil {
		fmt.Println("No SessionId cookie found.")
		return ""
	}
	
	var sessionId = cookie.String()
	
	//if httpReq.Header["Session-Id"] == nil { // No authenticated session has been established.
	//	fmt.Println("No Session-Id header found.")
	//	return ""
	//}
	//assertThat(len(httpReq.Header["Session-Id"]) > 0, "In getSessionId, len(httpReq.Header[Session-Id]) == 0")
	//var sessionId string = httpReq.Header["Session-Id"][0]
	
	
	if len(sessionId) == 0 { return "" }
	return sessionId
}

/*******************************************************************************
 * Used by addSessionIdToResponse.
 */
func (authService *AuthService) setSessionId(sessionToken *SessionToken,
	writer http.ResponseWriter) {
	
	// Set cookie containing the session Id.
	var cookie = &http.Cookie{
		Name: "SessionId",
		Value: sessionToken.UniqueSessionId,
		//Path: 
		//Domain: 
		//Expires: 
		//RawExpires: 
		MaxAge: 86400,
		Secure: false,  //....change to true later.
		HttpOnly: true,
		//Raw: 
		//Unparsed: 
	}
	
	http.SetCookie(writer, cookie)
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
 * Return a session id that is guaranteed to be unique, and that is completely
 * opaque and unforgeable.
 */
func (authSvc *AuthService) createUniqueSessionId() string {
	
	var uniqueNonRandomValue string = fmt.Sprintf("%d", time.Now().UnixNano())
	var saltedHashBytes []byte =
		authSvc.computeHash(uniqueNonRandomValue).Sum(authSvc.secretSalt)
	return uniqueNonRandomValue + ":" + fmt.Sprintf("%x", saltedHashBytes)
}

/*******************************************************************************
 * Validate session Id: return true if valid, false otherwise.
 */
func (authSvc *AuthService) sessionIdIsValid(sessionId string) bool {
	
	var parts []string = strings.Split(sessionId, ":")
	if len(parts) != 2 {
		fmt.Println("Illegally formatted sessionId:", sessionId)
		return false
	}
	
	var uniqueNonRandomValue string = parts[0]
	var untrustedHash string = parts[1]
	var actualSaltedHashBytes []byte =
		authSvc.computeHash(uniqueNonRandomValue).Sum(authSvc.secretSalt)
	
	return untrustedHash == fmt.Sprintf("%x", actualSaltedHashBytes)
}

/*******************************************************************************
 * 
 */
func (authSvc *AuthService) computeHash(s string) hash.Hash {
	
	var hash hash.Hash = sha512.New()
	var bytes []byte = []byte(s)
	hash.Write(bytes)
	return hash
}
