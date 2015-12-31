/*******************************************************************************
 * In-memory implementation of the methods defined in Persist.go.
 *
 * These methods do not perform any authorization - that is done by the handlers.
 * 
 * The Persistence struct implements persistence. It is extended by the Client struct,
 * which implements the Client interface from Persist.go. Below that, the remaining
 * types (structs) implement the various persistent object types from Persist.go.
 * 
 * Each type has a New<type> function. The New function merely constructs an instance
 * of the type - it does not link the type in any relationships.
 * 
 * For each concrete (non-abstract) type that has a writeBack() method, the New<type>
 * function writes the new instance to persistent storage.
 * 
 * Strategies for referential integrity:
 * -------------------------------------
 * 1. Persistent data is not cached in this layer - every handler action retrieves
 * data anew.
 * 2. Changes are not written to the database until it is known that there are no errors.
 * 3. If a consistency error is detected, a custom error type, DataError, is returned.
 * 4. For cases where consistency is important, an object level lock is used.
 */

package server

import (
	"fmt"
	"sync/atomic"
	"errors"
	"reflect"
	"os"
	//"io/ioutil"
	//"crypto/sha512"
	"time"
	//"runtime/debug"	
	
	"safeharbor/apitypes"
)

const (
	LockTimeoutSeconds int = 2
)

/*******************************************************************************
 * Implements DataError.
 */
type InMemDataError struct {
	error
}

var _ DataError = &InMemDataError{}

func NewInMemDataError(msg string) *InMemDataError {
	return &InMemDataError{
		error: errors.New(msg),
	}
}

func (dataErr *InMemDataError) asFailureDesc() *apitypes.FailureDesc {
	return apitypes.NewFailureDesc(dataErr.Error())
}

/*******************************************************************************
 * Contains all persistence functionality. Implementing these methods provides
 * persistence.
 *
 * Redis bindings for go: http://redis.io/clients#go
 * Chosen binding: https://github.com/alphazero/Go-Redis
 * Alternative binding: https://github.com/hoisie/redis
 */
type Persistence struct {
	uniqueId int64
	allObjects map[string]PersistObj
	allUsers map[string]User
	allRealmIds []string
}

func NewPersistence() (*Persistence, error) {
	var persist = &Persistence{
		uniqueId: 100000005,
		allRealmIds: make([]string, 0),
		allObjects: make(map[string]PersistObj),
		allUsers: make(map[string]User),
	}
	return persist, nil
}

// Load core database state. Database data is not cached, except for this core data.
// If the data is not present in the database, it should be created and written out.
func (persist *Persistence) load() error {
	var id int64
	var err error
	id, err = persist.readUniqueId()
	if err != nil { return err }
	if id != 0 {
		persist.uniqueId = id
	}
	return nil
}

// Create a globally unique id, to be used to uniquely identify a new persistent
// object. The creation of the id must be done atomically.
func (persist *Persistence) createUniqueDbObjectId() (string, error) {
	
	var id int64 = atomic.AddInt64(&persist.uniqueId, 1)
	var err error = persist.writeUniqueId()
	if err != nil { return "", err }
	persist.uniqueId = id
	return fmt.Sprintf("%d", id), nil
}

// Return the persistent object that is identified by the specified unique id.
// An object's Id is assigned to it by the function that creates the object.
func (persist *Persistence) getPersistentObject(id string) PersistObj {
	// TBD:
	// Read JSON from the database, using the id as the key; then deserialize
	// (unmarshall) the JSON into an object. The outermost JSON object will be
	// a field name - that field name is the name of the go object type; reflection
	// will be used to identify the go type, and set the fields in the type using
	// values from the hashmap that is built by the unmarshalling.
	return persist.allObjects[id]
	
	// Retrieve the object's value from redis.
	// Parse the JSON, using a custom parser that builds the types that are
	// defined in InMemory, using the New method of each type.
	// For fields that are string arrays or boolean arrays, the make function
	// is used to construct the required in-memory array.
	// var constructor = client.MethodByName("New" + typeName)
	// var obj = constructor(fieldValues...)
	// return obj.(PersistObj)
}

func (persist *Persistence) writeBack(obj PersistObj) error {
	// TBD:
	// Serialize (marshall) the object to JSON, and store it in redis using the
	// object's Id as the key. When the object is written out, it will be
	// written as,
	//    "<typename>": { <object fields> }
	// so that getPersistentObject will later be able to make the JSON to the
	// appropriate go type, using reflection.
	return nil
}

func (persist *Persistence) waitForLockOnObject(obj PersistObj, timeoutSeconds int) error {
	// TBD
	// If the current thread already has a lock on the specified object, merely return.
	return nil
}

func (persist *Persistence) releaseLock(obj PersistObj) {
	// TBD
}

func (persist *Persistence) addObject(obj PersistObj) error {
	persist.allObjects[obj.getId()] = obj
	return persist.writeBack(obj)
}

func (persist *Persistence) deleteObject(obj PersistObj) error {
	// TBD:
	return nil
}

func (persist *Persistence) readUniqueId() (int64, error) {
	return persist.uniqueId, nil
}

func (persist *Persistence) writeUniqueId() error {
	return nil
}

func (persist *Persistence) addRealm(newRealm Realm) error {
	persist.allRealmIds = append(persist.allRealmIds, newRealm.getId())
	return persist.addObject(newRealm)
}

func (persist *Persistence) dbGetAllRealmIds() []string {
	return persist.allRealmIds
}

func (persist *Persistence) addUser(user User) error {
	persist.allUsers[user.getUserId()] = user
	return persist.addObject(user)
}

/*******************************************************************************
 * The Client type, and methods required by the Client interface in Persist.go.
 */
type InMemClient struct {
	Persistence
	Server *Server
}

func NewInMemClient(server *Server) (DBClient, error) {
	
	// Create and return a new InMemClient.
	var pers *Persistence
	var err error
	pers, err = NewPersistence()
	if err != nil { return nil, err }
	var client = &InMemClient{
		Persistence: *pers,
		Server: server,
	}
	
	err = client.init()
	if err != nil { return nil, err }
	return client, nil
}

// Initilize the client object. This can be called later to reset the client's
// state (i.e., to erase all objects).
func (client *InMemClient) init() error {
	
	var err error = client.load()
	if err != nil { return err }
	
	// Remove the file repository - this is an in-memory implementation so we
	// want to start empty.
	err = os.RemoveAll(client.Server.Config.FileRepoRootPath)
	if err != nil { fmt.Println(err.Error()) }
	
	// Recreate the file repository, but empty.
	os.Mkdir(client.Server.Config.FileRepoRootPath, 0770)

	// For testing only:
	if client.Server.Debug {
		fmt.Println("Debug mode: creating realm testrealm")
		var realmInfo *apitypes.RealmInfo
		realmInfo, err = apitypes.NewRealmInfo("testrealm", "Test Org", "For Testing")
		if err != nil {
			fmt.Println(err.Error())
			panic(err)
		}
		var testRealm Realm
		testRealm, err = client.dbCreateRealm(realmInfo, "testuser1")
		if err != nil {
			fmt.Println(err.Error())
			panic(err)
		}
		fmt.Println("Debug mode: creating user testuser1 in realm testrealm")
		var testUser1 User
		testUser1, err = client.dbCreateUser("testuser1", "Test User", 
			"testuser@gmail.com", "Password1", testRealm.getId())
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1);
		}
		fmt.Println("User", testUser1.getName())
		fmt.Println("created, id=", testUser1.getId())
	}
	
	fmt.Println("Repository initialized")
	return nil
}

func (client *InMemClient) dbGetUserByUserId(userId string) User {
	return client.allUsers[userId]
}

// Create a directory for the Dockerfiles, images, and any other files owned
// by the specified realm.
func (client *InMemClient) assignRealmFileDir(realmId string) (string, error) {
	var path = client.Server.Config.FileRepoRootPath + "/" + realmId
	// Create the directory. (It is an error if it already exists.)
	err := os.MkdirAll(path, 0711)
	return path, err
}

// Create a directory for the Dockerfiles, images, and any other files owned
// by the specified repo. The directory will be created as a subdirectory of the
// realm's directory.
func (client *InMemClient) assignRepoFileDir(realmId string, repoId string) (string, error) {
	fmt.Println("assignRepoFileDir(", realmId, ",", repoId, ")...")
	var err error
	var realm Realm
	realm, err = client.getRealm(realmId)
	if err != nil { return "", err }
	var path = realm.getFileDirectory() + "/" + repoId
	var curdir string
	curdir, err = os.Getwd()
	if err != nil { fmt.Println(err.Error()) }
	fmt.Println("Current directory is '" + curdir + "'")
	fmt.Println("Creating directory '" + path + "'...")
	err = os.MkdirAll(path, 0711)
	return path, err
}

// Print the database to stdout. Diagnostic.
func (client *InMemClient) printDatabase() {
	fmt.Println("Not implemented yet")
}

/*******************************************************************************
 * Base type that is included in each data type as an anonymous field.
 */
type InMemPersistObj struct {  // abstract
	Id string
	Client *InMemClient
}

var _ PersistObj = &InMemPersistObj{}

func (client *InMemClient) NewInMemPersistObj() (*InMemPersistObj, error) {
	var id string
	var err error
	id, err = client.createUniqueDbObjectId()
	if err != nil { return nil, err }
	var obj *InMemPersistObj = &InMemPersistObj{
		Id: id,
		Client: client,
	}
	return obj, nil
}

func (persObj *InMemPersistObj) getId() string {
	return persObj.Id
}

func (persObj *InMemPersistObj) getDBClient() DBClient {
	return persObj.Client
}

func (persObj *InMemPersistObj) waitForLock() error {
	return persObj.Client.waitForLockOnObject(persObj, LockTimeoutSeconds)
}

func (persObj *InMemPersistObj) releaseLock() {
	persObj.Client.releaseLock(persObj)
}

func (persObj *InMemPersistObj) writeBack() error {
	return persObj.Client.writeBack(persObj)
}

/*******************************************************************************
 * 
 */
type InMemACL struct {
	InMemPersistObj
	ACLEntryIds []string
}

func (client *InMemClient) NewInMemACL() (*InMemACL, error) {
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	var acl *InMemACL = &InMemACL{
		InMemPersistObj: *pers,
		ACLEntryIds: make([]string, 0),
	}
	err = acl.writeBack()
	return acl, err
}

func (acl *InMemACL) getACLEntryIds() []string {
	return acl.ACLEntryIds
}

func (acl *InMemACL) addACLEntry(entry ACLEntry) error {
	acl.ACLEntryIds = append(acl.ACLEntryIds, entry.getId())
	return acl.writeBack()
}

/*******************************************************************************
 * 
 */
type InMemResource struct {  // abstract
	InMemACL
	Name string
	Description string
	ParentId string
	CreationTime time.Time
}

func (client *InMemClient) NewInMemResource(name string, desc string,
	parentId string) (*InMemResource, error) {
	var acl *InMemACL
	var err error
	acl, err = client.NewInMemACL()
	if err != nil { return nil, err }
	return &InMemResource{
		InMemACL: *acl,
		Name: name,
		Description: desc,
		ParentId: parentId,
		CreationTime: time.Now(),
	}, nil
}

func (resource *InMemResource) setAccess(party Party, mask []bool) (ACLEntry, error) {
	var aclEntry ACLEntry
	var err error
	aclEntry, err = party.getACLEntryForResourceId(resource.getId())
	if err != nil { return nil, err }
	if aclEntry == nil {
		aclEntry, err = resource.Client.dbCreateACLEntry(resource.getId(), party.getId(), mask)
		if err != nil { return nil, err }
	} else {
		err = aclEntry.setPermissionMask(mask)
		if err != nil { return nil, err }
	}
	
	return aclEntry, nil
}

func (resource *InMemResource) addAccess(party Party, mask []bool) (ACLEntry, error) {

	var aclEntry ACLEntry
	var err error
	aclEntry, err = party.getACLEntryForResourceId(resource.getId())
	if err != nil { return nil, err }
	if aclEntry == nil {
		aclEntry, err = resource.Client.dbCreateACLEntry(resource.getId(), party.getId(), mask)
		if err != nil { return nil, err }
	} else {
		// Add the new mask.
		var curmask []bool = aclEntry.getPermissionMask()
		for index, _ := range curmask {
			curmask[index] = curmask[index] || mask[index]
		}
		if err = aclEntry.writeBack(); err != nil { return nil, err }
	}

	return aclEntry, nil
}

func (resource *InMemResource) removeAccess(party Party) error {
	
	var aclEntriesCopy []string = make([]string, len(resource.ACLEntryIds))
	copy(aclEntriesCopy, resource.ACLEntryIds)
	fmt.Println(fmt.Sprintf("Copied %d ids", len(resource.ACLEntryIds)))
	fmt.Println(fmt.Sprintf("aclEntriesCopy has %d + elements", len(aclEntriesCopy)))
	fmt.Println("For each entry,")
	for index, entryId := range aclEntriesCopy {
		fmt.Println("\tentry entryId=" + entryId)
		var aclEntry ACLEntry
		var err error
		aclEntry, err = resource.Client.getACLEntry(entryId)
		if err != nil { return err }
		
		if aclEntry.getPartyId() == party.getId() {
			// ACL entry's resource id and party id both match.
			if aclEntry.getResourceId() != resource.getId() {
				return errors.New("Internal error: an ACL entry's resource Id does not match the resource whose list it is a member of")
			}
			
			// Remove from party's list.
			fmt.Println(fmt.Sprintf("\tRemoving ACL entry %s from party Id list", entryId))
			err = party.removeACLEntry(aclEntry)
			if err != nil { return err }
			
			// Remove the ACL entry id from the resource's ACL entry list.
			fmt.Println(fmt.Sprintf("\tRemoving ACL entry %s at position %d", entryId, index))
			resource.ACLEntryIds = apitypes.RemoveAt(index, resource.ACLEntryIds)
			
			// Remove from database.
			err = resource.Client.deleteObject(aclEntry)
			if err != nil { return err }
		}
	}
	
	return resource.writeBack()
}

func (resource *InMemResource) printACLs(party Party) {
	var curresourceId string = resource.getId()
	var curresource Resource = resource
	for {
		fmt.Println("\tACL entries for resource " + curresource.getName() + 
			" (" + curresource.getId() + ") are:")
		for _, entryId := range curresource.getACLEntryIds() {
			var aclEntry ACLEntry
			var err error
			aclEntry, err = curresource.getDBClient().getACLEntry(entryId)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			var rscId string = aclEntry.getResourceId()
			var rsc Resource
			rsc, err = resource.Client.getResource(rscId)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			var ptyId string = aclEntry.getPartyId()
			var pty Party
			pty, err = curresource.getDBClient().getParty(ptyId)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			fmt.Println("\t\tEntry Id " + entryId + ": party: " + pty.getName() + " (" + ptyId + "), resource: " +
				rsc.getName() + " (" + rsc.getId() + ")")
		}
		curresourceId = curresource.getParentId()
		if curresourceId == "" {
			fmt.Println(fmt.Sprintf("\tResource %s (%s) has not parentId",
				curresource.getName(), curresource.getId()))
			break
		}
		var err error
		curresource, err = resource.Client.getResource(curresourceId)
		if err != nil {
			fmt.Println(err.Error())
			break
		}
	}
	fmt.Println("\tACL entries for party " + party.getName() + 
		" (" + party.getId() + ") are:")
	for _, entryId := range party.getACLEntryIds() {
		var aclEntry ACLEntry
		var err error
		aclEntry, err = resource.Client.getACLEntry(entryId)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		var rscId string = aclEntry.getResourceId()
		var rsc Resource
		rsc, err = resource.Client.getResource(rscId)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		var partyId string = aclEntry.getPartyId()
		var pty Party
		pty, err = resource.Client.getParty(partyId)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		fmt.Println("\t\tEntry Id " + entryId + ": party: " + pty.getName() + " (" + partyId + "), resource: " +
			rsc.getName() + " (" + rsc.getId() + ")")
	}
}

func (resource *InMemResource) removeAllAccess() error {
	
	var aclEntriesCopy []string
	copy(aclEntriesCopy, resource.ACLEntryIds)
	for _, id := range aclEntriesCopy {
		var aclEntry ACLEntry
		var err error
		aclEntry, err = resource.Client.getACLEntry(id)
		if err != nil { return err }
		
		// Remove from party's list.
		var party Party
		party, err = resource.Client.getParty(aclEntry.getPartyId())
		if err != nil { return errors.New(err.Error()) }
		var inMemParty = party.(*InMemParty)
		inMemParty.ACLEntryIds = apitypes.RemoveFrom(id, inMemParty.ACLEntryIds)
		err = party.writeBack()
		if err != nil { return err }
		
		err = resource.Client.deleteObject(aclEntry)
		if err != nil { return err }
	}
		
	// Remove all ACL entry ids from the resource's ACL entry list.
	resource.ACLEntryIds = resource.ACLEntryIds[0:0]
	
	return resource.writeBack()
}

func (resource *InMemResource) getName() string {
	return resource.Name
}

func (resource *InMemResource) setName(name string) error {
	resource.setNameDeferredUpdate(name)
	return resource.writeBack()
}

func (resource *InMemResource) setNameDeferredUpdate(name string) {
	resource.Name = name
}

func (resource *InMemResource) getCreationTime() time.Time {
	return resource.CreationTime
}

func (resource *InMemResource) getDescription() string {
	return resource.Description
}

func (resource *InMemResource) setDescription(desc string) error {
	resource.setDescriptionDeferredUpdate(desc)
	return resource.writeBack()
}

func (resource *InMemResource) setDescriptionDeferredUpdate(desc string) {
	resource.Description = desc
}

func (resource *InMemResource) getACLEntryForPartyId(partyId string) (ACLEntry, error) {
	var err error
	for _, entryId := range resource.getACLEntryIds() {
		var obj interface{} = resource.Client.getPersistentObject(entryId)
		if obj == nil {
			err = errors.New("Internal error - no object found for Id " + entryId);
			continue
		}
		var entry ACLEntry
		var isType bool
		entry, isType = obj.(ACLEntry)
		if ! isType {
			err = errors.New("Internal error - object with Id " + entryId + " is not an ACLEntry");
			continue
		}
		if entry.getPartyId() == partyId {
			return entry, err
		}
	}
	return nil, err
}

func (client *InMemClient) getResource(resourceId string) (Resource, error) {
	var resource Resource
	var isType bool
	var obj PersistObj = client.getPersistentObject(resourceId)
	if obj == nil { return nil, errors.New("Object not found") }
	resource, isType = obj.(Resource)
	if ! isType { return nil, errors.New("Object with Id " + resourceId + " is not a Resource") }
	return resource, nil
}

func (resource *InMemResource) getParentId() string {
	return resource.ParentId
}

func (resource *InMemResource) isRealm() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Realm)
	return isType
}

func (resource *InMemResource) isRepo() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Repo)
	return isType
}

func (resource *InMemResource) isDockerfile() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Dockerfile)
	return isType
}

func (resource *InMemResource) isDockerImage() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(DockerImage)
	return isType
}

func (resource *InMemResource) isScanConfig() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(ScanConfig)
	return isType
}

func (resource *InMemResource) isFlag() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Flag)
	return isType
}

/*******************************************************************************
 * 
 */
type InMemParty struct {  // abstract
	InMemPersistObj
	IsActive bool
	Name string
	CreationTime time.Time
	RealmId string
	ACLEntryIds []string
}

func (client *InMemClient) NewInMemParty(name string, realmId string) (*InMemParty, error) {
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	return &InMemParty{
		InMemPersistObj: *pers,
		IsActive: true,
		Name: name,
		CreationTime: time.Now(),
		RealmId: realmId,
		ACLEntryIds: make([]string, 0),
	}, nil
}

func (party *InMemParty) setActive(b bool) {
	party.IsActive = b
}

func (party *InMemParty) isActive() bool {
	return party.IsActive
}

func (party *InMemParty) getName() string {
	return party.Name
}

func (party *InMemParty) getCreationTime() time.Time {
	return party.CreationTime
}

func (client *InMemClient) getParty(partyId string) (Party, error) {
	var party Party
	var isType bool
	var obj PersistObj = client.getPersistentObject(partyId)
	if obj == nil { return nil, errors.New("Object not found") }
	party, isType = obj.(Party)
	if ! isType { return nil, errors.New("Object with Id " + partyId + " is not a Party") }
	return party, nil
}

func (party *InMemParty) getRealmId() string {
	return party.RealmId
}

func (party *InMemParty) getRealm() (Realm, error) {
	return party.Client.getRealm(party.RealmId)
}

func (party *InMemParty) getACLEntryIds() []string {
	return party.ACLEntryIds
}

func (party *InMemParty) addACLEntry(entry ACLEntry) error {
	party.ACLEntryIds = append(party.ACLEntryIds, entry.getId())
	return party.writeBack()
}

func (party *InMemParty) removeACLEntry(entry ACLEntry) error {
	party.ACLEntryIds = apitypes.RemoveFrom(entry.getId(), party.ACLEntryIds)
	var err error = party.Client.deleteObject(entry)
	if err != nil { return err }
	return party.writeBack()
}

func (party *InMemParty) getACLEntryForResourceId(resourceId string) (ACLEntry, error) {
	var err error
	for _, entryId := range party.getACLEntryIds() {
		var obj interface{} = party.Client.getPersistentObject(entryId)
		if obj == nil {
			err = errors.New("Internal error - no object found for Id " + entryId);
			continue
		}
		var entry ACLEntry
		var isType bool
		entry, isType = obj.(ACLEntry)
		if ! isType {
			err = errors.New("Internal error - object with Id " + entryId + " is not an ACLEntry");
			continue
		}
		if entry.getResourceId() == resourceId {
			return entry, err
		}
	}
	return nil, err
}

/*******************************************************************************
 * 
 */
type InMemGroup struct {
	InMemParty
	Description string
	UserObjIds []string
}

func (client *InMemClient) NewInMemGroup(realmId string, name string,
	desc string) (*InMemGroup, error) {
	
	var group *InMemParty
	var err error
	group, err = client.NewInMemParty(name, realmId)
	if err != nil { return nil, err }
	var newGroup = &InMemGroup{
		InMemParty: *group,
		Description: desc,
		UserObjIds: make([]string, 0),
	}
	return newGroup, client.addObject(newGroup)
}

func (client *InMemClient) dbCreateGroup(realmId string, name string,
	description string) (Group, error) {
	
	// Check if a group with that name already exists within the realm.
	var realm Realm
	var err error
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	if realm == nil { return nil, errors.New(fmt.Sprintf(
		"Unidentified realm for realm Id %s", realmId))
	}
	var g Group
	g, err = realm.getGroupByName(name)
	if err != nil { return nil, err }
	if g != nil { return nil, errors.New(
		fmt.Sprintf("Group named %s already exists within realm %s", name,
			realm.getName()))
	}
	
	//var groupId string = createUniqueDbObjectId()
	var newGroup *InMemGroup
	newGroup, err = client.NewInMemGroup(realmId, name, description)
	if err != nil { return nil, err }
	
	// Add to parent realm's list
	err = realm.addGroup(newGroup)
	if err != nil { return nil, err }
	
	err = realm.writeBack()
	if err != nil { return nil, err }
	
	fmt.Println("Created Group")
	return newGroup, nil
}

func (client *InMemClient) getGroup(id string) (Group, error) {
	var group Group
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	group, isType = obj.(Group)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a Group") }
	return group, nil
}

func (group *InMemGroup) getDescription() string {
	return group.Description
}

func (group *InMemGroup) getUserObjIds() []string {
	return group.UserObjIds
}

func (group *InMemGroup) hasUserWithId(userObjId string) bool {
	var obj PersistObj = group.Client.getPersistentObject(userObjId)
	if obj == nil { return false }
	_, isUser := obj.(User)
	if ! isUser { return false }
	
	for _, id := range group.UserObjIds {
		if id == userObjId { return true }
	}
	return false
}

func (group *InMemGroup) addUserId(userObjId string) error {
	if group.hasUserWithId(userObjId) { return errors.New(fmt.Sprintf(
		"User with object Id %s is already in group", userObjId))
	}
	
	var obj PersistObj = group.Client.getPersistentObject(userObjId)
	if obj == nil { return errors.New(fmt.Sprintf(
		"Object with Id %s does not exist", userObjId))
	}
	user, isUser := obj.(User)
	if ! isUser { return errors.New(fmt.Sprintf(
		"Object with Id %s is not a User", userObjId))
	}
	group.UserObjIds = append(group.UserObjIds, userObjId)
	err := user.addGroupId(group.getId())
	if err != nil { return err }
	
	err = user.writeBack()
	if err != nil { return err }
	
	err = group.writeBack()
	
	return err
}

func (group *InMemGroup) removeUser(user User) error {
	group.waitForLock()
	defer group.releaseLock()
	var userId string = user.getId()
	for i, id := range group.UserObjIds {
		if id == userId {
			group.UserObjIds = append(group.UserObjIds[0:i], group.UserObjIds[i+1:]...)
			var err error = group.Client.deleteObject(user)
			if err != nil { return err }
			group.writeBack()
			return nil
		}
	}
	return errors.New("Did not find user in this group")
}

func (group *InMemGroup) addUser(user User) {
	group.UserObjIds = append(group.UserObjIds, user.getId())
}

func (group *InMemGroup) asGroupDesc() *apitypes.GroupDesc {
	return apitypes.NewGroupDesc(
		group.Id, group.RealmId, group.Name, group.Description, group.CreationTime)
}

/*******************************************************************************
 * 
 */
type InMemUser struct {
	InMemParty
	UserId string
	EmailAddress string
	PasswordHash []byte
	GroupIds []string
	MostRecentLoginAttempts []string
	EventIds []string
}

func (client *InMemClient) NewInMemUser(userId string, name string,
	email string, pswd string, realmId string) (*InMemUser, error) {
	
	var party *InMemParty
	var err error
	party, err = client.NewInMemParty(name, realmId)
	if err != nil { return nil, err }
	var newUser = &InMemUser{
		InMemParty: *party,
		UserId: userId,
		EmailAddress: email,
		PasswordHash: client.Server.authService.CreatePasswordHash(pswd),
		GroupIds: make([]string, 0),
	}
	return newUser, client.addUser(newUser)
}

func (client *InMemClient) dbCreateUser(userId string, name string,
	email string, pswd string, realmId string) (User, error) {
	
	if client.dbGetUserByUserId(userId) != nil {
		return nil, errors.New("A user with Id " + userId + " already exists")
	}
	
	var realm Realm
	var err error
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	if realm == nil { return nil, errors.New("Realm with Id " + realmId + " not found") }
	
	//var userObjId string = createUniqueDbObjectId()
	var newUser *InMemUser
	newUser, err = client.NewInMemUser(userId, name, email, pswd, realmId)
	if err != nil { return nil, err }
	
	// Add to parent realm's list.
	realm.addUser(newUser)
	
	err = realm.writeBack()
	if err != nil { return nil, err }

	fmt.Println("Created user")
	return newUser, nil
}

func (user *InMemUser) setPassword(pswd string) error {
	user.PasswordHash = user.Client.Server.authService.CreatePasswordHash(pswd)
	user.writeBack()
	return nil
}

func (user *InMemUser) validatePassword(pswd string) bool {
	var empty = []byte{}
	var authService = user.Client.Server.authService
	var prospectiveHash []byte = authService.computeHash(pswd).Sum(empty)
	return authService.compareHashValues(prospectiveHash, user.PasswordHash)
}

func (client *InMemClient) getUser(id string) (User, error) {
	var user User
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	user, isType = obj.(User)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a User") }
	return user, nil
}

func (user *InMemUser) getUserId() string {
	return user.UserId
}

func (user *InMemUser) hasGroupWithId(groupId string) bool {
	var obj PersistObj = user.Client.getPersistentObject(groupId)
	if obj == nil { return false }
	_, isGroup := obj.(Group)
	if ! isGroup { return false }
	
	for _, id := range user.GroupIds {
		if id == groupId { return true }
	}
	return false
}

func (user *InMemUser) addGroupId(groupId string) error {
	
	if user.hasGroupWithId(groupId) { return errors.New(fmt.Sprintf(
		"Group with object Id %s is already in User's set of groups", groupId))
	}
	
	var obj PersistObj = user.Client.getPersistentObject(groupId)
	if obj == nil { return errors.New(fmt.Sprintf(
		"Object with Id %s does not exist", groupId))
	}
	_, isGroup := obj.(Group)
	if ! isGroup { return errors.New(fmt.Sprintf(
		"Object with Id %s is not a Group", groupId))
	}
	user.GroupIds = append(user.GroupIds, groupId)
	return nil
}

func (user *InMemUser) getGroupIds() []string {
	return user.GroupIds
}

func (client *InMemClient) getRealmsAdministeredByUser(userObjId string) ([]string, error) {
	// those realms for which user can edit the realm
	
	var realmIds []string = make([]string, 0)
	
	// Identify the user.
	var obj PersistObj = client.getPersistentObject(userObjId)
	if obj == nil {
		return nil, errors.New("Object with Id " + userObjId + " not found")
	}
	var user User
	var isType bool
	user, isType = obj.(User)
	if ! isType {
		return nil, errors.New("Internal error: object with Id " + userObjId + " is not a User")
	}
	
	// Identify those ACLEntries that are for realms and for which the user has write access.
	var err error
	for _, entryId := range user.getACLEntryIds() {
		var entry ACLEntry
		entry, err = client.getACLEntry(entryId)
		if err != nil { return nil, err }
		if entry == nil {
			err = errors.New("Internal error: object with Id " + entryId + " is not an ACLEntry")
			continue
		}
		var resourceId string = entry.getResourceId()
		var resource Resource
		resource, err = client.getResource(resourceId)
		if err != nil { return nil, err }
		if resource == nil {
			err = errors.New("Internal error: resource with Id " + resourceId + " not found")
			continue
		}
		if resource.isRealm() {
			var realm Realm = resource.(Realm)
			var mask []bool = entry.getPermissionMask()
			if mask[apitypes.CanWrite] { // entry has write access for the realm
				realmIds = append(realmIds, realm.getId())
			}
		}
	}
	
	return realmIds, err
}

func (user *InMemUser) addLoginAttempt() {
	var num = len(user.MostRecentLoginAttempts)
	var max = user.Client.Server.MaxLoginAttemptsToRetain
	if num > max { num = num - max }
	user.MostRecentLoginAttempts = append(
		user.MostRecentLoginAttempts[num:], fmt.Sprintf("%d", time.Now().Unix()))
}

func (user *InMemUser) getMostRecentLoginAttempts() []string {
	return user.MostRecentLoginAttempts
}

func (user *InMemUser) addEventId(id string) {
	user.EventIds = append(user.EventIds, id)
	user.writeBack()
}

func (user *InMemUser) getEventIds() []string {
	return user.EventIds
}

func (user *InMemUser) removeEvent(event Event) error {
	
	// If a ScanEvent, then remove from ScanConfig and remove actual ParameterValues.
	var scanEvent ScanEvent
	var isType bool
	scanEvent, isType = event.(ScanEvent)
	if isType {
		var scanConfig ScanConfig
		var err error
		scanConfig, err = user.Client.getScanConfig(scanEvent.getScanConfigId())
		if err != nil { return err }
		err = scanConfig.removeScanEventId(scanEvent.getId())
		if err != nil { return err }
		err = scanEvent.removeAllParameterValues()
		if err != nil { return err }
	}
	
	user.EventIds = apitypes.RemoveFrom(event.getId(), user.EventIds)
	
	var err error = user.Client.deleteObject(event)
	if err != nil { return err }
	return user.writeBack()
}

func (user *InMemUser) asUserDesc() *apitypes.UserDesc {
	var adminRealmIds []string
	var err error
	adminRealmIds, err = user.getDBClient().getRealmsAdministeredByUser(user.getId())
	if err != nil {
		fmt.Println("In asUserDesc(), " + err.Error())
		adminRealmIds = make([]string, 0)
	}
	return apitypes.NewUserDesc(user.Id, user.UserId, user.Name, user.RealmId, adminRealmIds)
}

/*******************************************************************************
 * 
 */
type InMemACLEntry struct {
	InMemPersistObj
	ResourceId string
	PartyId string
	PermissionMask []bool
}

func (client *InMemClient) NewInMemACLEntry(resourceId string, partyId string,
	permissionMask []bool) (*InMemACLEntry, error) {
	
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	var newACLEntry *InMemACLEntry = &InMemACLEntry{
		InMemPersistObj: *pers,
		ResourceId: resourceId,
		PartyId: partyId,
		PermissionMask: permissionMask,
	}
	return newACLEntry, client.addObject(newACLEntry)
}

func (client *InMemClient) dbCreateACLEntry(resourceId string, partyId string,
	permissionMask []bool) (ACLEntry, error) {
	
	if resourceId == "" { return nil, errors.New("Internal error: resourceId is empty") }
	if partyId == "" { return nil, errors.New("Internal error: partyId is empty") }
	var resource Resource
	var party Party
	var isType bool
	var obj PersistObj = client.getPersistentObject(resourceId)
	if obj == nil { return nil, errors.New("Internal error: cannot identify resource: obj with Id '" + resourceId + "' not found") }
	resource, isType = obj.(Resource)
	if ! isType { return nil, errors.New("Internal error: object is not a Resource - it is a " +
		reflect.TypeOf(obj).String()) }
	obj = client.getPersistentObject(partyId)
	if obj == nil { return nil, errors.New("Internal error: cannot identify party: obj with Id '" + partyId + "' not found") }
	party, isType = obj.(Party)
	if ! isType { return nil, errors.New("Internal error: object is not a Party - it is a " +
		reflect.TypeOf(obj).String()) }
	//var aclEntryId = createUniqueDbObjectId()
	var newACLEntry ACLEntry
	var err error
	newACLEntry, err = client.NewInMemACLEntry(resourceId, partyId, permissionMask)
	if err != nil { return nil, err }
	err = resource.addACLEntry(newACLEntry)  // Add to resource's ACL
	if err != nil { return nil, err }
	err = party.addACLEntry(newACLEntry)  // Add to user or group's ACL
	if err != nil { return nil, err }
	fmt.Println("Added ACL entry for " + party.getName() + "(a " +
		reflect.TypeOf(party).String() + "), to access " +
		resource.getName() + " (a " + reflect.TypeOf(resource).String() + ")")
	return newACLEntry, nil
}

func (client *InMemClient) getACLEntry(id string) (ACLEntry, error) {
	var aclEntry ACLEntry
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	aclEntry, isType = obj.(ACLEntry)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return aclEntry, nil
}

func (entry *InMemACLEntry) getResourceId() string {
	return entry.ResourceId
}

func (entry *InMemACLEntry) getPartyId() string {
	return entry.PartyId
}

func (entry *InMemACLEntry) getParty() (Party, error) {
	var party Party
	var isType bool
	party, isType = entry.Client.getPersistentObject(entry.PartyId).(Party)
	if ! isType { return nil, errors.New("Internal error: object is not a Party") }
	return party, nil
}

func (entry *InMemACLEntry) getPermissionMask() []bool {
	return entry.PermissionMask
}

func (entry *InMemACLEntry) setPermissionMask(mask []bool) error {
	entry.PermissionMask = mask
	var err error = entry.writeBack()
	if err != nil { return err }
	return nil
}

func (entry *InMemACLEntry) asPermissionDesc() *apitypes.PermissionDesc {
	
	return apitypes.NewPermissionDesc(entry.getId(), entry.ResourceId, entry.PartyId, entry.getPermissionMask())
}

/*******************************************************************************
 * 
 */
type InMemRealm struct {
	InMemResource
	AdminUserId string
	OrgFullName string
	UserObjIds []string
	GroupIds []string
	RepoIds []string
	FileDirectory string  // where this realm's files are stored
}

func (client *InMemClient) NewInMemRealm(realmInfo *apitypes.RealmInfo, adminUserId string) (*InMemRealm, error) {
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(realmInfo.RealmName, realmInfo.Description, "")
	if err != nil { return nil, err }
	var newRealm *InMemRealm = &InMemRealm{
		InMemResource: *resource,
		AdminUserId: adminUserId,
		OrgFullName: realmInfo.OrgFullName,
		UserObjIds: make([]string, 0),
		GroupIds: make([]string, 0),
		RepoIds: make([]string, 0),
		FileDirectory: "",
	}
	
	return newRealm, client.addRealm(newRealm)
}

func (client *InMemClient) dbCreateRealm(realmInfo *apitypes.RealmInfo, adminUserId string) (Realm, error) {
	
	var realmId string
	var err error
	realmId, err = client.getRealmIdByName(realmInfo.RealmName)
	if err != nil { return nil, err }
	if realmId != "" {
		return nil, errors.New("A realm with name " + realmInfo.RealmName + " already exists")
	}
	
	err = nameConformsToSafeHarborImageNameRules(realmInfo.RealmName)
	if err != nil { return nil, err }
	
	//realmId = createUniqueDbObjectId()
	var newRealm *InMemRealm
	newRealm, err = client.NewInMemRealm(realmInfo, adminUserId)
	if err != nil { return nil, err }
	var realmFileDir string
	realmFileDir, err = client.assignRealmFileDir(newRealm.getId())
	if err != nil { return nil, err }
	newRealm.FileDirectory = realmFileDir
	err = newRealm.writeBack()
	if err != nil { return nil, err }
	
	fmt.Println("Created realm")
	//_, isType := allObjects[realmId].(Realm)
	//if ! isType {
	//	fmt.Println("*******realm", realmId, "is not a Realm")
	//	fmt.Println("newRealm is a", reflect.TypeOf(newRealm))
	//	fmt.Println("allObjects[", realmId, "] is a", reflect.TypeOf(allObjects[realmId]))
	//}
	return newRealm, nil
}

func (client *InMemClient) dbDeactivateRealm(realmId string) error {
	
	var err error
	var realm Realm
	realm, err = client.getRealm(realmId)
	if err != nil { return err }
	
	// Remove all ACL entries for the realm.
	err = realm.removeAllAccess()
	if err != nil { return err }
	
	// Remove all ACL entries for each of the realm's repos, and each of their resources.
	for _, repoId := range realm.getRepoIds() {
		var repo Repo
		repo, err = client.getRepo(repoId)
		if err != nil { return err }
		
		err = repo.removeAllAccess()
		if err != nil { return err }
		
		err = client.removeAllAccess(repo.getDockerfileIds())
		if err != nil { return err }

		err = client.removeAllAccess(repo.getDockerImageIds())
		if err != nil { return err }

		err = client.removeAllAccess(repo.getScanConfigIds())
		if err != nil { return err }

		err = client.removeAllAccess(repo.getFlagIds())
		if err != nil { return err }
	}
	
	// Inactivate all users owned by the realm.
	for _, userObjId := range realm.getUserObjIds() {
		var user User
		user, err = client.getUser(userObjId)
		if err != nil { return err }
		user.setActive(false)
	}
	
	// Inactivate all groups owned by the realm.
	for _, groupId := range realm.getGroupIds() {
		var group Group
		group, err = client.getGroup(groupId)
		if err != nil { return err }
		group.setActive(false)
	}
	
	return nil
}

func (client *InMemClient) removeAllAccess(resourceIds []string) error {
	for _, id := range resourceIds {
		var resource Resource
		var err error
		resource, err = client.getResource(id)
		if err != nil { return err }
		err = resource.removeAllAccess()
		if err != nil { return err }
	}
	return nil
}

func (client *InMemClient) getRealmIdByName(name string) (string, error) {
	for _, realmId := range client.dbGetAllRealmIds() {
		var realm Realm
		var err error
		realm, err = client.getRealm(realmId)
		if err != nil { return "", err }
		if realm.getName() == name { return realmId, nil }
	}
	return "", nil
}

func (realm *InMemRealm) getAdminUserId() string {
	return realm.AdminUserId
}

func (realm *InMemRealm) getFileDirectory() string {
	return realm.FileDirectory
}

func (client *InMemClient) getRealm(id string) (Realm, error) {
	var realm Realm
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	realm, isType = obj.(Realm)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a Realm") }
	return realm, nil
}

func (realm *InMemRealm) getUserObjIds() []string {
	return realm.UserObjIds
}

func (realm *InMemRealm) getRepoIds() []string {
	return realm.RepoIds
}

func (realm *InMemRealm) addUserId(userObjId string) error {
	
	var user User
	var isType bool
	user, isType = realm.Client.getPersistentObject(userObjId).(User)
	if ! isType { return errors.New("Internal error: object is an unexpected type") }
	if user == nil { return errors.New("Could not identify user with obj Id " + userObjId) }
	if user.getRealmId() != "" {
		return errors.New("User with obj Id " + userObjId + " belongs to another realm")
	}
	realm.UserObjIds = append(realm.UserObjIds, userObjId)
	var err error = realm.writeBack()
	return err
}

func (realm *InMemRealm) removeUserId(userObjId string) error {
	
	var user User
	var isType bool
	user, isType = realm.Client.getPersistentObject(userObjId).(User)
	if ! isType { return errors.New("Internal error: object is an unexpected type") }
	if user == nil { return errors.New("Could not identify user with obj Id " + userObjId) }
	if user.getRealmId() != "" {
		return errors.New("User with obj Id " + userObjId + " belongs to another realm")
	}
	realm.UserObjIds = apitypes.RemoveFrom(userObjId, realm.UserObjIds)
	var err error = realm.Client.deleteObject(user)
	if err != nil { return err }
	err = realm.writeBack()
	return err
}

func (realm *InMemRealm) getGroupIds() []string {
	return realm.GroupIds
}

func (realm *InMemRealm) addUser(user User) error {
	realm.Client.addUser(user)
	realm.UserObjIds = append(realm.UserObjIds, user.getId())
	return realm.writeBack()
}

func (realm *InMemRealm) addGroup(group Group) error {
	realm.GroupIds = append(realm.GroupIds, group.getId())
	return realm.writeBack()
}

func (realm *InMemRealm) addRepo(repo Repo) error {
	realm.RepoIds = append(realm.RepoIds, repo.getId())
	return realm.writeBack()
}

func (realm *InMemRealm) asRealmDesc() *apitypes.RealmDesc {
	return apitypes.NewRealmDesc(realm.Id, realm.Name, realm.OrgFullName, realm.AdminUserId)
}

func (realm *InMemRealm) hasUserWithId(userObjId string) bool {
	var obj PersistObj = realm.Client.getPersistentObject(userObjId)
	if obj == nil { return false }
	_, isUser := obj.(User)
	if ! isUser { return false }
	
	for _, id := range realm.UserObjIds {
		if id == userObjId { return true }
	}
	return false
}

func (realm *InMemRealm) hasGroupWithId(groupId string) bool {
	var obj PersistObj = realm.Client.getPersistentObject(groupId)
	if obj == nil { return false }
	_, isGroup := obj.(Group)
	if ! isGroup { return false }
	
	for _, id := range realm.GroupIds {
		if id == groupId { return true }
	}
	return false
}

func (realm *InMemRealm) hasRepoWithId(repoId string) bool {
	var obj PersistObj = realm.Client.getPersistentObject(repoId)
	if obj == nil { return false }
	_, isRepo := obj.(Repo)
	if ! isRepo { return false }
	
	for _, id := range realm.RepoIds {
		if id == repoId { return true }
	}
	return false
}

func (realm *InMemRealm) getUserByName(userName string) (User, error) {
	for _, id := range realm.UserObjIds {
		var obj PersistObj = realm.Client.getPersistentObject(id)
		if obj == nil { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		user, isUser := obj.(User)
		if ! isUser { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a User", id))
		}
		if user.getName() == userName { return user, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) getUserByUserId(userId string) (User, error) {
	for _, id := range realm.UserObjIds {
		var obj PersistObj = realm.Client.getPersistentObject(id)
		if obj == nil { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		user, isUser := obj.(User)
		if ! isUser { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a User", id))
		}
		if user.getUserId() == userId { return user, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) getGroupByName(groupName string) (Group, error) {
	for _, id := range realm.GroupIds {
		var obj PersistObj = realm.Client.getPersistentObject(id)
		if obj == nil { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		group, isGroup := obj.(Group)
		if ! isGroup { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a Group", id))
		}
		if group.getName() == groupName { return group, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) getRepoByName(repoName string) (Repo, error) {
	for _, id := range realm.RepoIds {
		var obj PersistObj = realm.Client.getPersistentObject(id)
		if obj == nil { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		repo, isRepo := obj.(Repo)
		if ! isRepo { return nil, errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a Repo", id))
		}
		if repo.getName() == repoName { return repo, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) deleteGroup(group Group) error {

	// Remove users from the group.
	for _, userObjId := range group.getUserObjIds() {
		var user User
		var err error
		user, err = realm.Client.getUser(userObjId)
		if err != nil { return err }
		err = group.removeUser(user)
		if err != nil { return err }
	}
	
	// Remove ACL entries referenced by the group.
	var entryIds []string = group.getACLEntryIds()
	var entryIdsCopy []string = make([]string, len(entryIds))
	copy(entryIdsCopy, entryIds)
	for _, entryId := range entryIdsCopy {
		var err error
		var entry ACLEntry
		entry, err = realm.Client.getACLEntry(entryId)
		if err != nil { return err }
		var resource Resource
		resource, err = realm.Client.getResource(entry.getResourceId())
		if err != nil { return err }
		var party Party
		party, err = realm.Client.getParty(entry.getPartyId())
		if err != nil { return err }
		err = resource.removeAccess(party)
		if err != nil { return err }
	}
	
	// Remove the group from its realm.
	realm.GroupIds = apitypes.RemoveFrom(group.getId(), realm.GroupIds)
	
	return realm.writeBack()
}

func (realm *InMemRealm) isRealm() bool { return true }

/*******************************************************************************
 * 
 */
type InMemRepo struct {
	InMemResource
	DockerfileIds []string
	DockerImageIds []string
	ScanConfigIds []string
	FlagIds []string
	FileDirectory string  // where this repo's files are stored
}

func (client *InMemClient) NewInMemRepo(realmId, name, desc string) (*InMemRepo, error) {
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, realmId)
	if err != nil { return nil, err }
	var newRepo *InMemRepo = &InMemRepo{
		InMemResource: *resource,
		DockerfileIds: make([]string, 0),
		DockerImageIds: make([]string, 0),
		ScanConfigIds: make([]string, 0),
		FlagIds: make([]string, 0),
		FileDirectory: "",
	}
	return newRepo, client.addObject(newRepo)
}

func (client *InMemClient) dbCreateRepo(realmId, name, desc string) (Repo, error) {
	var realm Realm
	var err error
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	
	err = nameConformsToSafeHarborImageNameRules(name)
	if err != nil { return nil, err }
	
	//var repoId string = createUniqueDbObjectId()
	var newRepo *InMemRepo
	newRepo, err = client.NewInMemRepo(realmId, name, desc)
	if err != nil { return nil, err }

	var repoFileDir string
	repoFileDir, err = client.assignRepoFileDir(realmId, newRepo.getId())
	if err != nil { return nil, err }
	newRepo.FileDirectory = repoFileDir
	err = newRepo.writeBack()
	if err != nil { return nil, err }
	fmt.Println("Created repo")
	err = realm.addRepo(newRepo)  // Add it to the realm.
	return newRepo, err
}

func (repo *InMemRepo) getFileDirectory() string {
	return repo.FileDirectory
}

func (client *InMemClient) getRepo(id string) (Repo, error) {
	var repo Repo
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	repo, isType = obj.(Repo)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a Repo") }
	return repo, nil
}

func (repo *InMemRepo) getRealmId() string { return repo.ParentId }

func (repo *InMemRepo) getRealm() (Realm, error) {
	var realm Realm
	var isType bool
	realm, isType = repo.Client.getPersistentObject(repo.getRealmId()).(Realm)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return realm, nil
}

func (repo *InMemRepo) getDockerfileIds() []string {
	return repo.DockerfileIds
}

func (repo *InMemRepo) getDockerImageIds() []string {
	return repo.DockerImageIds
}

func (repo *InMemRepo) getScanConfigIds() []string {
	return repo.ScanConfigIds
}

func (repo *InMemRepo) getFlagIds() []string {
	return repo.FlagIds
}

func (repo *InMemRepo) addDockerfile(dockerfile Dockerfile) error {
	repo.DockerfileIds = append(repo.DockerfileIds, dockerfile.getId())
	return repo.writeBack()
}

func (repo *InMemRepo) addDockerImage(image DockerImage) error {
	repo.DockerImageIds = append(repo.DockerImageIds, image.getId())
	return repo.writeBack()
}

func (repo *InMemRepo) addScanConfig(config ScanConfig) error {
	repo.ScanConfigIds = append(repo.ScanConfigIds, config.getId())
	return repo.writeBack()
}

func (repo *InMemRepo) removeScanConfig(config ScanConfig) error {
	if len(config.getScanEventIds()) > 0 { return errors.New(
		"Cannot remove ScanConfig: it is referenced by ScanEvents; the associated " +
		"dockerfile(s) would have to be removed first")
	}
	// Remove config's parameter values.
	config.removeAllParameterValues()
	
	// Remove reference from the flag.
	var flagId string = config.getFlagId()
	if flagId != "" {
		var err error
		var flag Flag
		flag, err = repo.Client.getFlag(flagId)
		if err != nil { return err }
		err = flag.removeScanConfigRef(config.getId())
		if err != nil { return err }
	}

	// Remove from repo.
	repo.ScanConfigIds = apitypes.RemoveFrom(config.getId(), repo.ScanConfigIds)

	// Remove from database.
	var err error = repo.Client.deleteObject(config)
	if err != nil { return err }
	
	return repo.writeBack()
}

func (repo *InMemRepo) removeFlag(flag Flag) error {
	if len(flag.usedByScanConfigIds()) > 0 { return errors.New(
		"Cannot remove Flag: it is referenced by one or more ScanConfigs")
	}

	// Remove the graphic file associated with the flag.
	var err error = os.Remove(flag.getSuccessImagePath())
	if err != nil { return err }
	
	// Remove from repo.
	repo.FlagIds = apitypes.RemoveFrom(flag.getId(), repo.FlagIds)
	
	// Remove from database.
	err = repo.Client.deleteObject(flag)
	if err != nil { return err }
	
	return repo.writeBack()
}

func (repo *InMemRepo) removeDockerImage(image DockerImage) error {
	
	// Remove events.
	for _, eventId := range image.getScanEventIds() {
		var event Event
		var err error
		event, err = repo.Client.getEvent(eventId)
		if err != nil { return err }
		var user User
		user, err = repo.Client.getUser(event.getUserObjId())
		if err != nil { return err }
		err = user.removeEvent(event)
		if err != nil { return err }
	}
	
	// Remove ACL entries.
	var err error = image.removeAllAccess()
	if err != nil { return err }
	
	// Remove from docker.
	err = repo.Client.Server.DockerService.RemoveDockerImage(image)
	if err != nil { return err }
	
	// Remove from database.
	err = repo.Client.deleteObject(image)
	if err != nil { return err }
	return repo.writeBack()
}

func (repo *InMemRepo) addFlag(flag Flag) error {
	repo.FlagIds = append(repo.FlagIds, flag.getId())
	return repo.writeBack()
}

func (repo *InMemRepo) getScanConfigByName(name string) (ScanConfig, error) {
	for _, configId := range repo.ScanConfigIds {
		var config ScanConfig
		var err error
		config, err = repo.getDBClient().getScanConfig(configId)
		if err != nil { return nil, err }
		if config == nil {
			return nil, errors.New("Internal error: list ScanConfigIds contains an invalid entry")
		}
		if config.getName() == name { return config, nil }
	}
	return nil, nil
}

func (repo *InMemRepo) isRepo() bool { return true }

func (repo *InMemRepo) asRepoDesc() *apitypes.RepoDesc {
	return apitypes.NewRepoDesc(repo.Id, repo.getRealmId(), repo.Name, repo.Description,
		repo.CreationTime, repo.getDockerfileIds())
}

/*******************************************************************************
 * 
 */
type InMemDockerfile struct {
	InMemResource
	FilePath string
	DockerfileExecEventIds []string
}

func (client *InMemClient) NewInMemDockerfile(repoId, name, desc,
	filepath string) (*InMemDockerfile, error) {
	
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	var newDockerfile *InMemDockerfile = &InMemDockerfile{
		InMemResource: *resource,
		FilePath: filepath,
		DockerfileExecEventIds: make([]string, 0),
	}
	return newDockerfile, client.addObject(newDockerfile)
}

func (client *InMemClient) dbCreateDockerfile(repoId, name,
	desc, filepath string) (Dockerfile, error) {
	
	var newDockerfile Dockerfile
	var err error
	newDockerfile, err = client.NewInMemDockerfile(repoId, name, desc, filepath)
	if err != nil { return nil, err }
	fmt.Println("Created Dockerfile")
	
	// Add to the Repo's list of Dockerfiles.
	var repo Repo
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	if repo == nil {
		fmt.Println("Repo with Id " + repoId + " not found")
		return nil, errors.New(fmt.Sprintf("Repo with Id %s not found", repoId))
	}
	err = repo.addDockerfile(newDockerfile)
	if err != nil { return nil, err }
	
	return newDockerfile, nil
}

func (client *InMemClient) getDockerfile(id string) (Dockerfile, error) {
	var dockerfile Dockerfile
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	dockerfile, isType = obj.(Dockerfile)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a Dockerfile") }
	return dockerfile, nil
}

func (dockerfile *InMemDockerfile) replaceDockerfileFile(filepath, desc string) error {
	
	if desc == "" { desc = dockerfile.getDescription() }  // use current description.
	
	var oldFilePath = dockerfile.getExternalFilePath()
	
	dockerfile.FilePath = filepath
	dockerfile.Description = desc
	dockerfile.CreationTime = time.Now()
	
	// Delete old file.
	return os.Remove(oldFilePath)
}

func (dockerfile *InMemDockerfile) getRepoId() string {
	return dockerfile.ParentId
}

func (dockerfile *InMemDockerfile) getRepo() (Repo, error) {
	var repo Repo
	var isType bool
	repo, isType = dockerfile.Client.getPersistentObject(dockerfile.getRepoId()).(Repo)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return repo, nil
}

func (dockerfile *InMemDockerfile) getDockerfileExecEventIds() []string {
	return dockerfile.DockerfileExecEventIds
}

func (dockerfile *InMemDockerfile) addEventId(eventId string) error {
	dockerfile.DockerfileExecEventIds = append(dockerfile.DockerfileExecEventIds, eventId)
	return dockerfile.writeBack()
}

func (dockerfile *InMemDockerfile) getExternalFilePath() string {
	return dockerfile.FilePath
}

func (dockerfile *InMemDockerfile) asDockerfileDesc() *apitypes.DockerfileDesc {
	return apitypes.NewDockerfileDesc(dockerfile.Id, dockerfile.getRepoId(), dockerfile.Name, dockerfile.Description)
}

func (dockerfile *InMemDockerfile) isDockerfile() bool { return true }

/*******************************************************************************
 * 
 */
type InMemImage struct {  // abstract
	InMemResource
}

func (client *InMemClient) NewInMemImage(name, desc, repoId string) (*InMemImage, error) {
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	return &InMemImage{
		InMemResource: *resource,
	}, nil
}

func (image *InMemImage) getName() string {
	return image.Name
}

func (image *InMemImage) getRepoId() string {
	return image.ParentId
}

func (image *InMemImage) getRepo() (Repo, error) {
	var repo Repo
	var isType bool
	repo, isType = image.Client.getPersistentObject(image.getRepoId()).(Repo)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return repo, nil
}

/*******************************************************************************
 * 
 */
type InMemDockerImage struct {
	InMemImage
	ScanEventIds []string
	Signature []byte
	OutputFromBuild string
}

func (client *InMemClient) NewInMemDockerImage(name, desc, repoId string,
	signature []byte, outputFromBuild string) (*InMemDockerImage, error) {
	var image *InMemImage
	var err error
	image, err = client.NewInMemImage(name, desc, repoId)
	if err != nil { return nil, err }
	var newDockerImage = &InMemDockerImage{
		InMemImage: *image,
		ScanEventIds: []string{},
		OutputFromBuild: outputFromBuild,
	}
	return newDockerImage, client.addObject(newDockerImage)
}

func (client *InMemClient) dbCreateDockerImage(repoId, dockerImageTag, desc string,
	outputFromBuild string) (DockerImage, error) {
	
	var repo Repo
	var isType bool
	repo, isType = client.getPersistentObject(repoId).(Repo)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	
	//var imageObjId string = createUniqueDbObjectId()
	var newDockerImage *InMemDockerImage
	var err error
	newDockerImage, err = client.NewInMemDockerImage(dockerImageTag, desc, repoId, nil,
		outputFromBuild)
	if err != nil { return nil, err }
	fmt.Println("Created DockerImage")
	err = repo.addDockerImage(newDockerImage)  // Add to repo's list.

	var signature []byte
	signature, err = newDockerImage.computeSignature()
	if err != nil { return newDockerImage, err }
	newDockerImage.Signature = signature
	
	return newDockerImage, err
}

func (client *InMemClient) getDockerImage(id string) (DockerImage, error) {
	var image DockerImage
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	image, isType = obj.(DockerImage)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a DockerImage") }
	return image, nil
}

func (image *InMemDockerImage) getSignature() []byte {
	return image.Signature
}

func (image *InMemDockerImage) computeSignature() ([]byte, error) {
	var err error
	var tempFilePath string
	tempFilePath, err = image.Client.Server.DockerService.SaveImage(image)
	if err != nil { return nil, err }
	defer os.RemoveAll(tempFilePath)
	return image.Client.Server.authService.ComputeFileSignature(tempFilePath)
}

func (image *InMemDockerImage) getOutputFromBuild() string {
	return image.OutputFromBuild
}

func (image *InMemDockerImage) getDockerImageTag() string {
	return image.Name
}

func (image *InMemDockerImage) getFullName() (string, error) {
	// See http://blog.thoward37.me/articles/where-are-docker-images-stored/
	var repo Repo
	var realm Realm
	var err error
	repo, err = image.Client.getRepo(image.getRepoId())
	if err != nil { return "", err }
	realm, err = image.Client.getRealm(repo.getRealmId())
	if err != nil { return "", err }
	return (realm.getName() + "/" + repo.getName() + ":" + image.Name), nil
}

func (image *InMemDockerImage) getScanEventIds() []string {
	return image.ScanEventIds
}

func (image *InMemDockerImage) getMostRecentScanEventId() string {
	var numOfIds = len(image.ScanEventIds)
	if numOfIds == 0 {
		return ""
	} else {
		return image.ScanEventIds[numOfIds-1]
	}
}

func (image *InMemDockerImage) asDockerImageDesc() *apitypes.DockerImageDesc {
	return apitypes.NewDockerImageDesc(image.Id, image.getRepoId(), image.Name,
		image.Description, image.CreationTime, image.Signature, image.OutputFromBuild)
}

func (image *InMemDockerImage) isDockerImage() bool { return true }

/*******************************************************************************
 * 
 */
type InMemParameterValue struct {
	InMemPersistObj
	Name string
	//TypeName string
	StringValue string
	ConfigId string
}

func (client *InMemClient) NewInMemParameterValue(name, value, configId string) (*InMemParameterValue, error) {
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	var paramValue = &InMemParameterValue{
		InMemPersistObj: *pers,
		Name: name,
		StringValue: value,
		ConfigId: configId,
	}
	return paramValue, client.addObject(paramValue)
}

func (client *InMemClient) getParameterValue(id string) (ParameterValue, error) {
	var pv ParameterValue
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	pv, isType = obj.(ParameterValue)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a ParameterValue") }
	return pv, nil
}

func (paramValue *InMemParameterValue) getName() string {
	return paramValue.Name
}

//func (paramValue *InMemParameterValue) getTypeName() string {
//	return paramValue.TypeName
//}

func (paramValue *InMemParameterValue) getStringValue() string {
	return paramValue.StringValue
}

func (paramValue *InMemParameterValue) setStringValue(value string) error {
	paramValue.StringValue = value
	return paramValue.writeBack()
}

func (paramValue *InMemParameterValue) getConfigId() string {
	return paramValue.ConfigId
}

func (paramValue *InMemParameterValue) asParameterValueDesc() *apitypes.ParameterValueDesc {
	return apitypes.NewParameterValueDesc(paramValue.Name, //paramValue.TypeName,
		paramValue.StringValue)
}

/*******************************************************************************
 * 
 */
type InMemScanConfig struct {
	InMemResource
	SuccessExpression string
	ProviderName string
	ParameterValueIds []string
	FlagId string
	ScanEventIds []string
}

func (client *InMemClient) NewInMemScanConfig(name, desc, repoId,
	providerName string, paramValueIds []string, successExpr string,
	flagId string) (*InMemScanConfig, error) {
	
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	var scanConfig = &InMemScanConfig{
		InMemResource: *resource,
		SuccessExpression: successExpr,
		ProviderName: providerName,
		ParameterValueIds: paramValueIds,
		FlagId: flagId,
	}
	return scanConfig, client.addObject(scanConfig)
}

func (client *InMemClient) dbCreateScanConfig(name, desc, repoId,
	providerName string, paramValueIds []string, successExpr, flagId string) (ScanConfig, error) {
	
	// Check if a scanConfig with that name already exists within the repo.
	var repo Repo
	var err error
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	if repo == nil { return nil, errors.New(fmt.Sprintf(
		"Unidentified repo for repo Id %s", repoId))
	}
	var sc ScanConfig
	sc, err = repo.getScanConfigByName(name)
	if err != nil { return nil, err }
	if sc != nil { return nil, errors.New(
		fmt.Sprintf("ScanConfig named %s already exists within repo %s", name,
			repo.getName()))
	}
	
	//var scanConfigId string = createUniqueDbObjectId()
	var scanConfig *InMemScanConfig
	scanConfig, err = client.NewInMemScanConfig(name, desc, repoId, providerName,
		paramValueIds, successExpr, flagId)
	if flagId != "" {
		var flag Flag
		flag, err = scanConfig.Client.getFlag(flagId)
		if err != nil { return nil, err }
		err = flag.addScanConfigRef(scanConfig.getId())
		if err != nil { return nil, err }
	}
	err = scanConfig.writeBack()
	if err != nil { return nil, err }
	
	// Link to repo
	repo.addScanConfig(scanConfig)
	
	fmt.Println("Created ScanConfig")
	return scanConfig, nil
}

func (client *InMemClient) getScanConfig(id string) (ScanConfig, error) {
	var scanConfig ScanConfig
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	scanConfig, isType = obj.(ScanConfig)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return scanConfig, nil
}

func (scanConfig *InMemScanConfig) getSuccessExpr() string {
	return scanConfig.SuccessExpression
}

func (scanConfig *InMemScanConfig) setSuccessExpression(expr string) error {
	scanConfig.setSuccessExpressionDeferredUpdate(expr)
	return scanConfig.writeBack()
}

func (scanConfig *InMemScanConfig) setSuccessExpressionDeferredUpdate(expr string) {
	scanConfig.SuccessExpression = expr
}

func (scanConfig *InMemScanConfig) getRepoId() string {
	return scanConfig.ParentId
}

func (scanConfig *InMemScanConfig) getProviderName() string {
	return scanConfig.ProviderName
}

func (scanConfig *InMemScanConfig) setProviderName(name string) error {
	scanConfig.setProviderNameDeferredUpdate(name)
	return scanConfig.writeBack()
}

func (scanConfig *InMemScanConfig) setProviderNameDeferredUpdate(name string) {
	scanConfig.ProviderName = name
}

func (scanConfig *InMemScanConfig) getParameterValueIds() []string {
	return scanConfig.ParameterValueIds
}

func (scanConfig *InMemScanConfig) setParameterValue(name, strValue string) (ParameterValue, error) {
	var paramValue ParameterValue
	var err error
	paramValue, err = scanConfig.setParameterValueDeferredUpdate(name, strValue)
	if err != nil { return paramValue, err }
	err = paramValue.writeBack()
	if err != nil { return paramValue, err }
	return paramValue, scanConfig.writeBack()
}

func (scanConfig *InMemScanConfig) setParameterValueDeferredUpdate(name, strValue string) (ParameterValue, error) {
	
	// Check if a parameter value already exist for the parameter. If so, replace the value.
	for _, id := range scanConfig.ParameterValueIds {
		var pv ParameterValue
		var err error
		pv, err = scanConfig.getDBClient().getParameterValue(id)
		if err != nil { return nil, err }
		if pv == nil {
			fmt.Println("Internal ERROR: broken ParameterValue list for scan config " + scanConfig.getName())
			continue
		}
		if pv.getName() == name {
			pv.setStringValue(strValue)
			return pv, nil
		}
	}
	
	// Did not find a value for a parameter of that name - create a new ParameterValue.
	var paramValue *InMemParameterValue
	var err error
	paramValue, err = scanConfig.Client.NewInMemParameterValue(name, strValue, scanConfig.getId())
	if err != nil { return nil, err }
	scanConfig.ParameterValueIds = append(scanConfig.ParameterValueIds, paramValue.getId())
	return paramValue, nil
}

func (scanConfig *InMemScanConfig) removeParameterValue(name string) error {
	for i, id := range scanConfig.ParameterValueIds {
		var pv ParameterValue
		var err error
		pv, err = scanConfig.getDBClient().getParameterValue(id)
		if err != nil { return err }
		if pv == nil {
			fmt.Println("Internal ERROR: broken ParameterValue list for scan config " + scanConfig.getName())
			continue
		}
		if pv.getName() == name {
			scanConfig.ParameterValueIds = apitypes.RemoveAt(i, scanConfig.ParameterValueIds)
			err = scanConfig.Client.deleteObject(pv)
			if err != nil { return err }
			return scanConfig.writeBack()
		}
	}
	return errors.New("Did not find parameter named '" + name + "'")
}

func (scanConfig *InMemScanConfig) removeAllParameterValues() error {
	for _, paramValueId := range scanConfig.getParameterValueIds() {
		var err error
		var paramValue ParameterValue
		paramValue, err = scanConfig.Client.getParameterValue(paramValueId)
		if err != nil { return err }
		scanConfig.Client.deleteObject(paramValue)
	}
	scanConfig.ParameterValueIds = make([]string, 0)
	return scanConfig.writeBack()
}

func (scanConfig *InMemScanConfig) setFlagId(id string) error {
	if scanConfig.FlagId == id { return nil } // nothing to do
	var flag Flag
	var err error
	flag, err = scanConfig.Client.getFlag(id)
	if err != nil { return err }
	if scanConfig.FlagId != "" { // already set
		flag.removeScanConfigRef(scanConfig.getId())
	}
	scanConfig.FlagId = id
	err = flag.addScanConfigRef(scanConfig.getId())  // adds non-redundantly
	if err != nil { return err }
	return scanConfig.writeBack()
}

func (scanConfig *InMemScanConfig) getFlagId() string {
	return scanConfig.FlagId
}

func (scanConfig *InMemScanConfig) addScanEventId(id string) {
	scanConfig.ScanEventIds = append(scanConfig.ScanEventIds, id)
	scanConfig.writeBack()
}

func (scanConfig *InMemScanConfig) getScanEventIds() []string {
	return scanConfig.ScanEventIds
}

func (scanConfig *InMemScanConfig) removeScanEventId(eventId string) error {
	scanConfig.ScanEventIds = apitypes.RemoveFrom(eventId, scanConfig.ScanEventIds)
	return scanConfig.writeBack()
}

func (resource *InMemScanConfig) isScanConfig() bool {
	return true
}

func (scanConfig *InMemScanConfig) asScanConfigDesc() *apitypes.ScanConfigDesc {
	
	fmt.Println("asScanConfigDesc: FlagId=" + scanConfig.FlagId)  // debug
	
	
	
	var paramValueDescs []*apitypes.ParameterValueDesc = make([]*apitypes.ParameterValueDesc, 0)
	for _, valueId := range scanConfig.ParameterValueIds {
		var paramValue ParameterValue
		var err error
		paramValue, err = scanConfig.Client.getParameterValue(valueId)
		if err != nil {
			fmt.Println("Internal error: " + err.Error())
			continue
		}
		if paramValue == nil {
			fmt.Println("Internal error: Could not find ParameterValue with Id " + valueId)
			continue
		}
		paramValueDescs = append(paramValueDescs, paramValue.asParameterValueDesc())
	}
	
	fmt.Println("asScanConfigDesc 2: FlagId=" + scanConfig.FlagId)  // debug
	fmt.Println("asScanConfigDesc 2a: getFlagId()=" + scanConfig.getFlagId())  // debug

	return apitypes.NewScanConfigDesc(scanConfig.Id, scanConfig.ProviderName,
		scanConfig.SuccessExpression, scanConfig.FlagId, paramValueDescs)
}

/*******************************************************************************
 * 
 */
type InMemFlag struct {
	InMemResource
	SuccessImagePath string
	UsedByScanConfigIds []string
}

func (client *InMemClient) NewInMemFlag(name, desc, repoId,
	successImagePath string) (*InMemFlag, error) {
	
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	var flag = &InMemFlag{
		InMemResource: *resource,
		SuccessImagePath: successImagePath,
		UsedByScanConfigIds: make([]string, 0),
	}
	return flag, client.addObject(flag)
}

func (client *InMemClient) dbCreateFlag(name, desc, repoId, successImagePath string) (Flag, error) {
	var flag Flag
	var err error
	flag, err = client.NewInMemFlag(name, desc, repoId, successImagePath)
	if err != nil { return nil, err }
	
	var repo Repo
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	
	// Add to repo's list of flags.
	err = repo.addFlag(flag)
	if err != nil { return nil, err }

	// Make persistent.
	err = flag.writeBack()
	
	return flag, err
}

func (client *InMemClient) getFlag(id string) (Flag, error) {
	var flag Flag
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	flag, isType = obj.(Flag)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return flag, nil
}

func (flag *InMemFlag) getRepoId() string {
	return flag.ParentId
}

func (flag *InMemFlag) setSuccessImagePath(path string) {
	flag.SuccessImagePath = path
}

func (flag *InMemFlag) getSuccessImagePath() string {
	return flag.SuccessImagePath
}

func (flag *InMemFlag) getSuccessImageURL() string {
	return flag.Client.Server.GetHTTPResourceScheme() + "://getFlagImage/?Id=" + flag.getId()
}

func (flag *InMemFlag) addScanConfigRef(scanConfigId string) error {
	fmt.Println("addScanConfigRef:A")
	flag.UsedByScanConfigIds = apitypes.AddUniquely(scanConfigId, flag.UsedByScanConfigIds)
	return flag.writeBack()
}

func (flag *InMemFlag) removeScanConfigRef(scanConfigId string) error {
	flag.UsedByScanConfigIds = apitypes.RemoveFrom(scanConfigId, flag.UsedByScanConfigIds)
	return flag.writeBack()
}

func (flag *InMemFlag) usedByScanConfigIds() []string {
	return flag.UsedByScanConfigIds
}

func (resource *InMemFlag) isFlag() bool {
	return true
}

func (flag *InMemFlag) asFlagDesc() *apitypes.FlagDesc {
	return apitypes.NewFlagDesc(flag.getId(), flag.getRepoId(), flag.getName(),
		flag.getSuccessImageURL())
}

/*******************************************************************************
 * 
 */
type InMemEvent struct {  // abstract
	InMemPersistObj
	When time.Time
	UserObjId string
}

func (client *InMemClient) NewInMemEvent(userObjId string) (*InMemEvent, error) {
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	return &InMemEvent{
		InMemPersistObj: *pers,
		When: time.Now(),
		UserObjId: userObjId,
	}, nil
}

func (client *InMemClient) getEvent(id string) (Event, error) {
	var event Event
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	event, isType = obj.(Event)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return event, nil
}

func (event *InMemEvent) getWhen() time.Time {
	return event.When
}

func (event *InMemEvent) getUserObjId() string {
	return event.UserObjId
}

func (event *InMemEvent) asEventDesc() *apitypes.EventDescBase {
	return apitypes.NewEventDesc(event.Id, event.When, event.UserObjId)
}

/*******************************************************************************
 * 
 */
type InMemScanEvent struct {
	InMemEvent
	ScanConfigId string
	DockerImageId string
	ProviderName string
	ActualParameterValueIds []string
	Score string
}

func (client *InMemClient) NewInMemScanEvent(scanConfigId, imageId, userObjId,
	providerName string, score string, actParamValueIds []string) (*InMemScanEvent, error) {
	
	var event *InMemEvent
	var err error
	event, err = client.NewInMemEvent(userObjId)
	if err != nil { return nil, err }
	var scanEvent *InMemScanEvent = &InMemScanEvent{
		InMemEvent: *event,
		ScanConfigId: scanConfigId,
		DockerImageId: imageId,
		ProviderName: providerName,
		ActualParameterValueIds: actParamValueIds,
		Score: score,
	}
	return scanEvent, client.addObject(scanEvent)
}

func (client *InMemClient) dbCreateScanEvent(scanConfigId, imageId,
	userObjId, score string) (ScanEvent, error) {
	
	// Create actual ParameterValues for the Event, using the current ParameterValues
	// that exist for the ScanConfig.
	var scanConfig ScanConfig
	var err error
	scanConfig, err = client.getScanConfig(scanConfigId)
	if err != nil { return nil, err }
	var actParamValueIds []string = make([]string, 0)
	for _, paramId := range scanConfig.getParameterValueIds() {
		var param ParameterValue
		param, err = client.getParameterValue(paramId)
		if err != nil { return nil, err }
		var name string = param.getName()
		var value string = param.getStringValue()
		//var pvId string = createUniqueDbObjectId()
		var actParamValue *InMemParameterValue
		actParamValue, err = client.NewInMemParameterValue(name, value, scanConfigId)
		if err != nil { return nil, err }
		actParamValueIds = append(actParamValueIds, actParamValue.getId())
	}

	//var id string = createUniqueDbObjectId()
	var scanEvent *InMemScanEvent
	scanEvent, err = client.NewInMemScanEvent(scanConfigId, imageId, userObjId,
		scanConfig.getProviderName(), score, actParamValueIds)
	if err != nil { return nil, err }
	err = scanEvent.writeBack()
	if err != nil { return nil, err }
	
	// Link to user.
	var user User
	user, err = client.getUser(userObjId)
	if err != nil { return nil, err }
	user.addEventId(scanEvent.getId())
	
	// Link to ScanConfig.
	scanConfig.addScanEventId(scanEvent.getId())

	fmt.Println("Created ScanEvent")
	return scanEvent, nil
}

func (client *InMemClient) getScanEvent(id string) (ScanEvent, error) {
	var scanEvent ScanEvent
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, errors.New("Object not found") }
	scanEvent, isType = obj.(ScanEvent)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return scanEvent, nil
}

func (event *InMemScanEvent) getScore() string {
	return event.Score
}

func (event *InMemScanEvent) getDockerImageId() string {
	return event.DockerImageId
}

func (event *InMemScanEvent) getScanConfigId() string {
	return event.ScanConfigId
}

func (event *InMemScanEvent) getActualParameterValueIds() []string {
	return event.ActualParameterValueIds
}

func (event *InMemScanEvent) removeAllParameterValues() error {
	for _, paramId := range event.ActualParameterValueIds {
		var param ParameterValue
		var err error
		param, err = event.Client.getParameterValue(paramId)
		if err != nil { return err }
		event.Client.deleteObject(param)
	}
	event.ActualParameterValueIds = make([]string, 0)
	return event.writeBack()
}

func (event *InMemScanEvent) asScanEventDesc() *apitypes.ScanEventDesc {
	var paramValueDescs []*apitypes.ParameterValueDesc = make([]*apitypes.ParameterValueDesc, 0)
	for _, valueId := range event.ActualParameterValueIds {
		var value ParameterValue
		var err error
		value, err = event.Client.getParameterValue(valueId)
		if err != nil {
			fmt.Println("Internal error:", err.Error())
			continue
		}
		paramValueDescs = append(paramValueDescs, value.asParameterValueDesc())
	}
	
	return apitypes.NewScanEventDesc(event.Id, event.When, event.UserObjId,
		event.ScanConfigId, event.ProviderName, paramValueDescs,
		event.Score)
}

func (event *InMemScanEvent) asEventDesc() apitypes.EventDesc {
	return event.asScanEventDesc()
}

/*******************************************************************************
 * 
 */
type InMemImageCreationEvent struct {  // abstract
	InMemEvent
	ImageId string
}

func (client *InMemClient) NewInMemImageCreationEvent(userObjId, 
	imageId string) (*InMemImageCreationEvent, error) {
	var event *InMemEvent
	var err error
	event, err = client.NewInMemEvent(userObjId)
	if err != nil { return nil, err }
	return &InMemImageCreationEvent{
		InMemEvent: *event,
		ImageId: imageId,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemDockerfileExecEvent struct {
	InMemImageCreationEvent
	DockerfileId string
	DockerfileExternalObjId string
}

func (client *InMemClient) NewInMemDockerfileExecEvent(dockerfileId, imageId,
	userObjId string) (*InMemDockerfileExecEvent, error) {
	
	var ev *InMemImageCreationEvent
	var err error
	ev, err = client.NewInMemImageCreationEvent(userObjId, imageId)
	if err != nil { return nil, err }
	var event = &InMemDockerfileExecEvent{
		InMemImageCreationEvent: *ev,
		DockerfileId: dockerfileId,
		DockerfileExternalObjId: "",  // for when we add git
	}
	return event, client.addObject(event)
}

func (client *InMemClient) dbCreateDockerfileExecEvent(dockerfileId, imageId,
	userObjId string) (DockerfileExecEvent, error) {
	
	//var id string = createUniqueDbObjectId()
	var newDockerfileExecEvent *InMemDockerfileExecEvent
	var err error
	newDockerfileExecEvent, err =
		client.NewInMemDockerfileExecEvent(dockerfileId, imageId, userObjId)
	
	// Link to Dockerfile.
	var dockerfile Dockerfile
	dockerfile, err = client.getDockerfile(dockerfileId)
	if err != nil { return nil, err }
	dockerfile.addEventId(newDockerfileExecEvent.getId())
	
	// Link to user.
	var user User
	user, err = client.getUser(userObjId)
	if err != nil { return nil, err }
	user.addEventId(newDockerfileExecEvent.getId())
	
	return newDockerfileExecEvent, nil
}

func (execEvent *InMemDockerfileExecEvent) getDockerfileId() string {
	return execEvent.DockerfileId
}

func (execEvent *InMemDockerfileExecEvent) getDockerfileExternalObjId() string {
	return execEvent.DockerfileExternalObjId
}

func (event *InMemDockerfileExecEvent) asDockerfileExecEventDesc() *apitypes.DockerfileExecEventDesc {
	return apitypes.NewDockerfileExecEventDesc(event.getId(), event.When, event.UserObjId,
		event.DockerfileId)
}

func (event *InMemDockerfileExecEvent) asEventDesc() apitypes.EventDesc {
	return event.asDockerfileExecEventDesc()
}
