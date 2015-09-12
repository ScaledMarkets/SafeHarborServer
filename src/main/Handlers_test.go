/*******************************************************************************
 * Unit level testing of the handlers. Can be run without any external services
 * such as Cesanta auth serve, Redis, or external storage.
 */

package main

import (
	"testing"
	//"os"
)

func Test_AuthenticateHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}

func Test_CreateUserHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}

func Test_CreateGroupHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}

func Test_AddGroupUserHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}

func Test_CreateRealmHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}

func Test_AddRealmUserHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}

func Test_AddRealmGroupHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}

func Test_CreateRepoHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
	//var server = ....
	//var sessionToken = ....
	//var values = ....
	//var repoDesc RepoDesc = createRepo(server, sessionToken, values)
	
}

func Test_AddDockerfileHandler(t *testing.T) {
	if testing.Short() { t.Skip("skipping test in short mode.") }
	
}
