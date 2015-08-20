/*******************************************************************************
 * Functions needed to implement the handlers in Handlers.go.
 */

package main

import (
	"time"
)

/*******************************************************************************
 * Return a session id that is guaranteed to be unique, and that is completely
 * opaque and unforgeable.
 */
func (server *Server) createUniqueSessionId() string {
	return encrypt(time.Now().Local().String())
}

/*******************************************************************************
 * Encrypt the specified string. For now, just return the string. Need to complete
 * this to use the Server's private key.
 */
func encrypt(s string) string {
	return s
}
