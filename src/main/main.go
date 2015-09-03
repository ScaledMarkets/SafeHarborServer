/*******************************************************************************
 * SafeHarbor REST server.
 * See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
*/

package main

import (
	"fmt"
)

func main() {
	
	fmt.Println("Creating SafeHarbor server...")
	var server *Server = NewServer()

	// Temporary for testing - remove! ********************
	var testRealm *InMemRealm
	var err error
	testRealm, err = server.dbClient.dbCreateRealm(NewRealmInfo("testrealm"))
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}
	var testUser1 *InMemUser
	testUser1, err = server.dbClient.dbCreateUser("testuser1", "Test User", testRealm.Id)
	fmt.Println("User", testUser1.Name, "created, id=", testUser1.Id)
	// ****************************************************
	
	server.start()
}
