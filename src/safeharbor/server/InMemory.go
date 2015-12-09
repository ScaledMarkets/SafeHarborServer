/*******************************************************************************
 * In-memory implementation of the methods defined in Persist.go.
 *
 * The Group, Permission, Repo, Dockerfile, Image, User, and Realm have
 * asGroupDesc, asPermissionDesc, asRepoDesc, asDockerfileDesc, asImageDesc,
 * asUserDesc, and asRealmDesc methods, respectively - these methods construct
 * instances of apitypes.GroupDesc, apitypes.PermissionDesc, apitypes.RepoDesc, apitypes.DockerfileDesc, ImageDesc,
 * and so on. These methods are a convenient way of constructing the return values
 * that are needed by the handler methods defined in the API (slides titled
 * "SafeHarbor REST API" of the desgin), which are implemented by the functions
 * in Handlers.go.
 */

package server

import (
	"fmt"
	"sync/atomic"
	"errors"
	"reflect"
	"os"
	"io/ioutil"
	"crypto/sha1"
	"time"
	
	"safeharbor/apitypes"
)

/*******************************************************************************
 * The Client type, and methods required by the Client interface in Persist.go.
 */
type InMemClient struct {
	Server *Server
}

func NewInMemClient(server *Server) DBClient {
	
	// Create and return a new InMemClient.
	var client = &InMemClient{
		Server: server,
	}
	
	client.init()
	return client
}

/*******************************************************************************
 * Initilize the client object. This can be called later to reset the client's
 * state (i.e., to erase all objects).
 */
func (client *InMemClient) init() {
	
	// Initialize global variables.
	allRealmIds = make([]string, 0)
	allObjects = make(map[string]PersistObj)
	allUsers = make(map[string]User)

	// Remove the file repository - this is an in-memory implementation so we
	// want to start empty.
	var err error = os.RemoveAll(client.Server.Config.FileRepoRootPath)
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
}

/*******************************************************************************
 * Base type that is included in each data type as an anonymous field.
 */
type InMemPersistObj struct {
	Id string
	Client *InMemClient
}

var _ PersistObj = &InMemPersistObj{}

func (client *InMemClient) NewInMemPersistObj() *InMemPersistObj {
	return &InMemPersistObj{
		Id: createUniqueDbObjectId(),
		Client: client,
	}
}

func (persObj *InMemPersistObj) getId() string {
	return persObj.Id
}

func (persObj *InMemPersistObj) getDBClient() DBClient {
	return persObj.Client
}

/*
 * Return the persistent object that is identified by the specified unique id.
 * An object's Id is assigned to it by the function that creates the object.
 */
func (client *InMemClient) getPersistentObject(id string) PersistObj {
	return allObjects[id]
}


/*******************************************************************************
 * 
 */
type InMemACL struct {
	InMemPersistObj
	ACLEntryIds []string
}

func (client *InMemClient) NewInMemACL() *InMemACL {
	return &InMemACL{
		InMemPersistObj: *client.NewInMemPersistObj(),
		ACLEntryIds: make([]string, 0),
	}
}

func (acl *InMemACL) getACLEntryIds() []string {
	return acl.ACLEntryIds
}

func (acl *InMemACL) addACLEntry(entry ACLEntry) {
	acl.ACLEntryIds = append(acl.ACLEntryIds, entry.getId())
}

/*******************************************************************************
 * 
 */
type InMemResource struct {
	InMemACL
	Name string
	Description string
	CreationTime time.Time
}

func (client *InMemClient) NewInMemResource(name string, desc string) *InMemResource {
	return &InMemResource{
		InMemACL: *client.NewInMemACL(),
		Name: name,
		Description: desc,
		CreationTime: time.Now(),
	}
}

func (resource *InMemResource) getName() string {
	return resource.Name
}

func (resource *InMemResource) getCreationTime() time.Time {
	return resource.CreationTime
}

func (resource *InMemResource) getDescription() string {
	return resource.Description
}

func (resource *InMemResource) getACLEntryForPartyId(partyId string) (ACLEntry, error) {
	var err error
	for _, entryId := range resource.getACLEntryIds() {
		var obj interface{} = allObjects[entryId]
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
	if obj == nil { return nil, nil }
	resource, isType = obj.(Resource)
	if ! isType { return nil, errors.New("Object with Id " + resourceId + " is not a Resource") }
	return resource, nil
}

func (resource *InMemResource) getParentId() string {
	return ""
}

func (resource *InMemResource) isRealm() bool { return false }
func (resource *InMemResource) isRepo() bool { return false }
func (resource *InMemResource) isDockerfile() bool { return false }
func (resource *InMemResource) isDockerImage() bool { return false }

/*******************************************************************************
 * 
 */
type InMemParty struct {
	InMemPersistObj
	Name string
	CreationTime time.Time
	RealmId string
	ACLEntryIds []string
}

func (client *InMemClient) NewInMemParty(name string, realmId string) *InMemParty {
	return &InMemParty{
		InMemPersistObj: *client.NewInMemPersistObj(),
		Name: name,
		CreationTime: time.Now(),
		RealmId: realmId,
		ACLEntryIds: make([]string, 0),
	}
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
	if obj == nil { return nil, nil }
	party, isType = obj.(Party)
	if ! isType { return nil, errors.New("Object with Id " + partyId + " is not a Party") }
	return party, nil
}

func (party *InMemParty) getRealmId() string {
	return party.RealmId
}

func (party *InMemParty) getACLEntryIds() []string {
	return party.ACLEntryIds
}

func (party *InMemParty) addACLEntry(entry ACLEntry) {
	party.ACLEntryIds = append(party.ACLEntryIds, entry.getId())
}

func (party *InMemParty) getACLEntryForResourceId(resourceId string) (ACLEntry, error) {
	var err error
	for _, entryId := range party.getACLEntryIds() {
		var obj interface{} = allObjects[entryId]
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
	var newGroup = &InMemGroup{
		InMemParty: *client.NewInMemParty(/*groupId,*/ name, realmId),
		Description: description,
		UserObjIds: make([]string, 0),
	}
	
	// Add to parent realm's list
	realm.addGroup(newGroup)
	
	fmt.Println("Created Group")
	allObjects[newGroup.getId()] = newGroup
	return newGroup, nil
}

func (client *InMemClient) getGroup(id string) (Group, error) {
	var group Group
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, nil }
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
	var obj PersistObj = allObjects[userObjId]
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
	
	var obj PersistObj = allObjects[userObjId]
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
	return nil
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
	PasswordHash [20]byte
	GroupIds []string
}

func (client *InMemClient) dbGetUserByUserId(userId string) User {
	return allUsers[userId]
}

func (client *InMemClient) dbCreateUser(userId string, name string,
	email string, pswd string, realmId string) (User, error) {
	
	if allUsers[userId] != nil {
		return nil, errors.New("A user with Id " + userId + " already exists")
	}
	
	var realm Realm
	var err error
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	if realm == nil { return nil, errors.New("Realm with Id " + realmId + " not found") }
	
	//var userObjId string = createUniqueDbObjectId()
	var pswdAsBytes []byte = []byte(pswd)
	var newUser *InMemUser = &InMemUser{
		InMemParty: *client.NewInMemParty(/*userObjId,*/ name, realmId),
		UserId: userId,
		EmailAddress: email,
		PasswordHash: sha1.Sum(pswdAsBytes),
		GroupIds: make([]string, 0),
	}
	
	// Add to parent realm's list.
	realm.addUser(newUser)
	
	fmt.Println("Created user")
	allObjects[newUser.getId()] = newUser
	return newUser, nil
}

func (client *InMemClient) getUser(id string) (User, error) {
	var user User
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, nil }
	user, isType = obj.(User)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a User") }
	return user, nil
}

func (user *InMemUser) getUserId() string {
	return user.UserId
}

func (user *InMemUser) hasGroupWithId(groupId string) bool {
	var obj PersistObj = allObjects[groupId]
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
	
	var obj PersistObj = allObjects[groupId]
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
	var newACLEntry *InMemACLEntry = &InMemACLEntry{
		InMemPersistObj: *client.NewInMemPersistObj(),
		ResourceId: resource.getId(),
		PartyId: partyId,
		PermissionMask: permissionMask,
	}
	allObjects[newACLEntry.getId()] = newACLEntry
	resource.addACLEntry(newACLEntry)  // Add to resource's ACL
	party.addACLEntry(newACLEntry)  // Add to user or group's ACL
	fmt.Println("Added ACL entry for " + party.getName() + "(a " +
		reflect.TypeOf(party).String() + "), to access " +
		resource.getName() + " (a " + reflect.TypeOf(resource).String() + ")")
	return newACLEntry, nil
}

func (client *InMemClient) getACLEntry(id string) (ACLEntry, error) {
	var aclEntry ACLEntry
	var isType bool
	var obj PersistObj = client.getPersistentObject(id)
	if obj == nil { return nil, nil }
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
	party, isType = allObjects[entry.PartyId].(Party)
	if ! isType { return nil, errors.New("Internal error: object is not a Party") }
	return party, nil
}

func (entry *InMemACLEntry) getPermissionMask() []bool {
	return entry.PermissionMask
}

func (entry *InMemACLEntry) setPermissionMask(mask []bool) {
	entry.PermissionMask = mask
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
	var newRealm *InMemRealm = &InMemRealm{
		InMemResource: *client.NewInMemResource(realmInfo.RealmName, realmInfo.Description),
		AdminUserId: adminUserId,
		OrgFullName: realmInfo.OrgFullName,
		UserObjIds: make([]string, 0),
		GroupIds: make([]string, 0),
		RepoIds: make([]string, 0),
		FileDirectory: "",
	}
	var realmFileDir string
	realmFileDir, err = client.assignRealmFileDir(newRealm.getId())
	if err != nil { return nil, err }
	newRealm.FileDirectory = realmFileDir
	
	allRealmIds = append(allRealmIds, realmId)
	
	fmt.Println("Created realm")
	allObjects[realmId] = newRealm
	//_, isType := allObjects[realmId].(Realm)
	//if ! isType {
	//	fmt.Println("*******realm", realmId, "is not a Realm")
	//	fmt.Println("newRealm is a", reflect.TypeOf(newRealm))
	//	fmt.Println("allObjects[", realmId, "] is a", reflect.TypeOf(allObjects[realmId]))
	//}
	return newRealm, nil
}

func (client *InMemClient) dbGetAllRealmIds() []string {
	return allRealmIds
}

func (client *InMemClient) getRealmIdByName(name string) (string, error) {
	for _, realmId := range allRealmIds {
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
	var obj PersistObj = allObjects[id]
	if obj == nil { return nil, nil }
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
	user, isType = allObjects[userObjId].(User)
	if ! isType { return errors.New("Internal error: object is an unexpected type") }
	if user == nil { return errors.New("Could not identify user with obj Id " + userObjId) }
	if user.getRealmId() != "" {
		return errors.New("User with obj Id " + userObjId + " belongs to another realm")
	}
	realm.UserObjIds = append(realm.UserObjIds, userObjId)
	return nil
}

func (realm *InMemRealm) getGroupIds() []string {
	return realm.GroupIds
}

func (realm *InMemRealm) addUser(user User) {
	allUsers[user.getUserId()] = user
	realm.UserObjIds = append(realm.UserObjIds, user.getId())
}

func (realm *InMemRealm) addGroup(group Group) {
	realm.GroupIds = append(realm.GroupIds, group.getId())
}

func (realm *InMemRealm) addRepo(repo Repo) {
	realm.RepoIds = append(realm.RepoIds, repo.getId())
}

func (realm *InMemRealm) asRealmDesc() *apitypes.RealmDesc {
	return apitypes.NewRealmDesc(realm.Id, realm.Name, realm.OrgFullName, realm.AdminUserId)
}

func (realm *InMemRealm) hasUserWithId(userObjId string) bool {
	var obj PersistObj = allObjects[userObjId]
	if obj == nil { return false }
	_, isUser := obj.(User)
	if ! isUser { return false }
	
	for _, id := range realm.UserObjIds {
		if id == userObjId { return true }
	}
	return false
}

func (realm *InMemRealm) hasGroupWithId(groupId string) bool {
	var obj PersistObj = allObjects[groupId]
	if obj == nil { return false }
	_, isGroup := obj.(Group)
	if ! isGroup { return false }
	
	for _, id := range realm.GroupIds {
		if id == groupId { return true }
	}
	return false
}

func (realm *InMemRealm) hasRepoWithId(repoId string) bool {
	var obj PersistObj = allObjects[repoId]
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
		var obj PersistObj = allObjects[id]
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
		var obj PersistObj = allObjects[id]
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
		var obj PersistObj = allObjects[id]
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
		var obj PersistObj = allObjects[id]
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

func (realm *InMemRealm) isRealm() bool { return true }

/*******************************************************************************
 * 
 */
type InMemRepo struct {
	InMemResource
	RealmId string
	DockerfileIds []string
	DockerImageIds []string
	ScanConfigIds []string
	FileDirectory string  // where this repo's files are stored
}

func (client *InMemClient) dbCreateRepo(realmId string, name string, desc string) (Repo, error) {
	var realm Realm
	var err error
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	
	err = nameConformsToSafeHarborImageNameRules(name)
	if err != nil { return nil, err }
	
	//var repoId string = createUniqueDbObjectId()
	var newRepo *InMemRepo = &InMemRepo{
		InMemResource: *client.NewInMemResource(name, desc),
		RealmId: realmId,
		DockerfileIds: make([]string, 0),
		DockerImageIds: make([]string, 0),
		ScanConfigIds: make([]string, 0),
		FileDirectory: "",
	}
	var repoFileDir string
	repoFileDir, err = client.assignRepoFileDir(realmId, newRepo.getId())
	if err != nil { return nil, err }
	newRepo.FileDirectory = repoFileDir
	fmt.Println("Created repo")
	allObjects[newRepo.getId()] = newRepo
	realm.addRepo(newRepo)  // Add it to the realm.
	return newRepo, nil
}

func (repo *InMemRepo) getFileDirectory() string {
	return repo.FileDirectory
}

func (client *InMemClient) getRepo(id string) (Repo, error) {
	var repo Repo
	var isType bool
	var obj PersistObj = allObjects[id]
	if obj == nil { return nil, nil }
	repo, isType = obj.(Repo)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a Repo") }
	return repo, nil
}

func (repo *InMemRepo) getRealmId() string { return repo.RealmId }

func (repo *InMemRepo) getRealm() (Realm, error) {
	var realm Realm
	var isType bool
	realm, isType = allObjects[repo.RealmId].(Realm)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return realm, nil
}

func (repo *InMemRepo) getDockerfileIds() []string {
	return repo.DockerfileIds
}

func (repo *InMemRepo) getDockerImageIds() []string {
	return repo.DockerImageIds
}

func (repo *InMemRepo) addDockerfile(dockerfile Dockerfile) {
	repo.DockerfileIds = append(repo.DockerfileIds, dockerfile.getId())
}

func (repo *InMemRepo) addDockerImage(image DockerImage) {
	repo.DockerImageIds = append(repo.DockerImageIds, image.getId())
}

func (repo *InMemRepo) addScanConfig(config ScanConfig) {
	repo.ScanConfigIds = append(repo.ScanConfigIds, config.getId())
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

func (repo *InMemRepo) getParentId() string {
	return repo.RealmId
}

func (repo *InMemRepo) isRepo() bool { return true }

func (repo *InMemRepo) asRepoDesc() *apitypes.RepoDesc {
	return apitypes.NewRepoDesc(repo.Id, repo.RealmId, repo.Name, repo.Description,
		repo.CreationTime, repo.getDockerfileIds())
}

/*******************************************************************************
 * 
 */
type InMemDockerfile struct {
	InMemResource
	RepoId string
	FilePath string  // make immutable
	DockerfileExecEventIds []string
}

func (client *InMemClient) dbCreateDockerfile(repoId string, name string,
	desc string, filepath string) (Dockerfile, error) {
	//var dockerfileId string = createUniqueDbObjectId()
	var newDockerfile *InMemDockerfile = &InMemDockerfile{
		InMemResource: *client.NewInMemResource(name, desc),
		RepoId: repoId,
		FilePath: filepath,
		DockerfileExecEventIds: make([]string, 0),
	}
	
	fmt.Println("Created Dockerfile")
	allObjects[newDockerfile.getId()] = newDockerfile
	
	// Add to the Repo's list of Dockerfiles.
	var repo Repo
	var err error
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	if repo == nil {
		fmt.Println("Repo with Id " + repoId + " not found")
		return nil, errors.New(fmt.Sprintf("Repo with Id %s not found", repoId))
	}
	repo.addDockerfile(newDockerfile)
	
	return newDockerfile, nil
}

func (client *InMemClient) getDockerfile(id string) (Dockerfile, error) {
	var dockerfile Dockerfile
	var isType bool
	var obj PersistObj = allObjects[id]
	if obj == nil { return nil, nil }
	dockerfile, isType = obj.(Dockerfile)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a Dockerfile") }
	return dockerfile, nil
}

func (dockerfile *InMemDockerfile) getRepo() (Repo, error) {
	var repo Repo
	var isType bool
	repo, isType = allObjects[dockerfile.RepoId].(Repo)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return repo, nil
}

func (dockerfile *InMemDockerfile) getDockerfileExecEventIds() []string {
	return dockerfile.DockerfileExecEventIds
}

func (dockerfile *InMemDockerfile) addEventId(eventId string) {
	dockerfile.DockerfileExecEventIds = append(dockerfile.DockerfileExecEventIds, eventId)
}

func (dockerfile *InMemDockerfile) getExternalFilePath() string {
	return dockerfile.FilePath
}

func (dockerfile *InMemDockerfile) asDockerfileDesc() *apitypes.DockerfileDesc {
	return apitypes.NewDockerfileDesc(dockerfile.Id, dockerfile.RepoId, dockerfile.Name, dockerfile.Description)
}

func (dockerfile *InMemDockerfile) getParentId() string {
	return dockerfile.RepoId
}

func (dockerfile *InMemDockerfile) isDockerfile() bool { return true }

/*******************************************************************************
 * 
 */
type InMemImage struct {
	InMemResource
	RepoId string
}

func (client *InMemClient) NewInMemImage(name, desc, repoId string) *InMemImage {
	return &InMemImage{
		InMemResource: *client.NewInMemResource(name, desc),
		RepoId: repoId,
	}
}

func (image *InMemImage) getName() string {
	return image.Name
}

func (image *InMemImage) getRepo() (Repo, error) {
	var repo Repo
	var isType bool
	repo, isType = allObjects[image.RepoId].(Repo)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return repo, nil
}

func (image *InMemImage) getParentId() string {
	return image.RepoId
}

/*******************************************************************************
 * 
 */
type InMemDockerImage struct {
	InMemImage
}

func (client *InMemClient) dbCreateDockerImage(repoId string,
	dockerImageTag string, desc string) (DockerImage, error) {
	
	var repo Repo
	var isType bool
	repo, isType = allObjects[repoId].(Repo)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	
	//var imageObjId string = createUniqueDbObjectId()
	var newDockerImage *InMemDockerImage = &InMemDockerImage{
		InMemImage: *client.NewInMemImage(dockerImageTag, desc, repoId),
	}
	fmt.Println("Created DockerImage")
	allObjects[newDockerImage.getId()] = newDockerImage
	repo.addDockerImage(newDockerImage)  // Add to repo's list.
	return newDockerImage, nil
}

func (client *InMemClient) getDockerImage(id string) (DockerImage, error) {
	var image DockerImage
	var isType bool
	var obj PersistObj = allObjects[id]
	if obj == nil { return nil, nil }
	image, isType = obj.(DockerImage)
	if ! isType { return nil, errors.New("Object with Id " + id + " is not a DockerImage") }
	return image, nil
}

func (image *InMemDockerImage) getDockerImageTag() string {
	return image.Name
}

func (image *InMemDockerImage) getFullName() (string, error) {
	// See http://blog.thoward37.me/articles/where-are-docker-images-stored/
	var repo Repo
	var realm Realm
	var err error
	repo, err = image.Client.getRepo(image.RepoId)
	if err != nil { return "", err }
	realm, err = image.Client.getRealm(repo.getRealmId())
	if err != nil { return "", err }
	return (realm.getName() + "/" + repo.getName() + ":" + image.Name), nil
}

func (image *InMemDockerImage) asDockerImageDesc() *apitypes.DockerImageDesc {
	return apitypes.NewDockerImageDesc(image.Id, image.RepoId, image.Name, image.Description, image.CreationTime)
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

func (client *InMemClient) NewInMemParameterValue(name, value, configId string) *InMemParameterValue {
	return &InMemParameterValue{
		InMemPersistObj: *client.NewInMemPersistObj(),
		Name: name,
		StringValue: value,
		ConfigId: configId,
	}
}

func (client *InMemClient) getParameterValue(id string) (ParameterValue, error) {
	var pv ParameterValue
	var isType bool
	var obj PersistObj = allObjects[id]
	if obj == nil { return nil, nil }
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

func (paramValue *InMemParameterValue) setStringValue(value string) {
	paramValue.StringValue = value
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
	RepoId string
	ExternalObjPath string
	ProviderName string
	ParameterValueIds []string
	SuccessGraphicImageURL string
	FailureGraphicImageURL string
}

func (client *InMemClient) dbCreateScanConfig(name, desc, repoId,
	providerName string, paramValueIds []string, successGraphicImageURL,
	failureGraphicImageURL string) (ScanConfig, error) {
	
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
	var scanConfig *InMemScanConfig = &InMemScanConfig{
		InMemResource: *client.NewInMemResource(name, desc),
		RepoId: repoId,
		ExternalObjPath: "",
		ProviderName: providerName,
		ParameterValueIds: paramValueIds,
		SuccessGraphicImageURL: successGraphicImageURL,
		FailureGraphicImageURL: failureGraphicImageURL,
	}
	var externalObjPath string = repo.getFileDirectory() + "/" + scanConfig.getId()
	scanConfig.ExternalObjPath = externalObjPath
	
	// Link to repo
	repo.addScanConfig(scanConfig)

	fmt.Println("Created ScanConfig")
	allObjects[scanConfig.getId()] = scanConfig
	
	return scanConfig, nil
}

func (client *InMemClient) getScanConfig(id string) (ScanConfig, error) {
	var scanConfig ScanConfig
	var isType bool
	var obj PersistObj = allObjects[id]
	if obj == nil { return nil, nil }
	scanConfig, isType = obj.(ScanConfig)
	if ! isType { return nil, errors.New("Internal error: object is an unexpected type") }
	return scanConfig, nil
}

func (scanConfig *InMemScanConfig) getRepoId() string {
	return scanConfig.RepoId
}

func (scanConfig *InMemScanConfig) getExternalObjPath() string {
	return scanConfig.ExternalObjPath
}

func (scanConfig *InMemScanConfig) getCurrentExtObjId() string {
	return scanConfig.ExternalObjPath  // temporary - until we incorporate git.
		// Then we will replace this with the hash and path.
		// See https://stackoverflow.com/questions/2466735/how-to-checkout-only-one-file-from-git-repository
		// See also https://git-scm.com/docs/git-hash-object
}

func (scanConfig *InMemScanConfig) getAsTempFile(extObjId string) (*os.File, error) {
	var tempfile *os.File
	var err error
	tempfile, err = ioutil.TempFile("", "")
	if err != nil {
		return nil, errors.New("Error: Unable to create temp file")
	}
	
	// Copy the specified file object to the temp file.
	var bytes []byte
	var sourcefile *os.File
	sourcefile, err = os.Open(extObjId) // for now, we treat the extObjId as a file path.
	bytes, err = ioutil.ReadAll(sourcefile)
	err = ioutil.WriteFile(tempfile.Name(), bytes, os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
		return nil, errors.New("While writing dockerfile, " + err.Error())
	}
	
	return tempfile, nil
}

func (scanConfig *InMemScanConfig) getProviderName() string {
	return scanConfig.ProviderName
}

func (scanConfig *InMemScanConfig) getParameterValueIds() []string {
	return scanConfig.ParameterValueIds
}

func (scanConfig *InMemScanConfig) getSuccessGraphicImageURL() string {
	return scanConfig.SuccessGraphicImageURL
}

func (scanConfig *InMemScanConfig) getFailureGraphicImageURL() string {
	return scanConfig.FailureGraphicImageURL
}

func (scanConfig *InMemScanConfig) setParameterValue(name, strValue string) (ParameterValue, error) {
	
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
	//var pvId string = createUniqueDbObjectId()
	var paramValue = scanConfig.Client.NewInMemParameterValue(/*pvId,*/ name, strValue, scanConfig.getId())
	scanConfig.ParameterValueIds = append(scanConfig.ParameterValueIds, paramValue.getId())
	allObjects[paramValue.getId()] = paramValue
	return paramValue, nil
}

func (scanConfig *InMemScanConfig) asScanConfigDesc() *apitypes.ScanConfigDesc {
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
	
	return apitypes.NewScanConfigDesc(scanConfig.Id, scanConfig.ProviderName, paramValueDescs)
}

/*******************************************************************************
 * 
 */
type InMemEvent struct {
	InMemPersistObj
	When time.Time
	UserObjId string
}

func (client *InMemClient) NewInMemEvent(userObjId string) *InMemEvent {
	return &InMemEvent{
		InMemPersistObj: *client.NewInMemPersistObj(),
		When: time.Now(),
		UserObjId: userObjId,
	}
}

func (event *InMemEvent) getWhen() time.Time {
	return event.When
}

func (event *InMemEvent) getUserObjId() string {
	return event.UserObjId
}

func (event *InMemEvent) asEventDesc() *apitypes.EventDesc {
	return apitypes.NewEventDesc(event.Id, event.When, event.UserObjId)
}

/*******************************************************************************
 * 
 */
type InMemScanEvent struct {
	InMemEvent
	ScanConfigId string
	DockerImageId string
	Score string
	ScanConfigExternalObjId string
	ActualParameterValueIds []string
}

func (client *InMemClient) dbCreateScanEvent(scanConfigId, imageId,
	userObjId string, score string, extObjId string) (ScanEvent, error) {
	
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
		var actParamValue = client.NewInMemParameterValue(/*pvId,*/ name, value, scanConfigId)
		actParamValueIds = append(actParamValueIds, actParamValue.getId())
	}

	//var id string = createUniqueDbObjectId()
	var scanEvent *InMemScanEvent = &InMemScanEvent{
		InMemEvent: *client.NewInMemEvent(/*id,*/ userObjId),
		ScanConfigId: scanConfigId,
		DockerImageId: imageId,
		Score: score,
		ScanConfigExternalObjId: extObjId,
		ActualParameterValueIds: actParamValueIds,
	}

	fmt.Println("Created ScanEvent")
	allObjects[scanEvent.getId()] = scanEvent
	
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

func (event *InMemScanEvent) getScanConfigExternalObjId() string {
	return event.ScanConfigExternalObjId
}

func (event *InMemScanEvent) asScanEventDesc() *apitypes.ScanEventDesc {
	return apitypes.NewScanEventDesc(event.Id, event.When, event.UserObjId,
		event.ScanConfigId, event.Score)
}

/*******************************************************************************
 * 
 */
type InMemImageCreationEvent struct {
	InMemEvent
	ImageId string
}

func (client *InMemClient) NewInMemImageCreationEvent(userObjId, imageId string) *InMemImageCreationEvent {
	return &InMemImageCreationEvent{
		InMemEvent: *client.NewInMemEvent(userObjId),
		ImageId: imageId,
	}
}

/*******************************************************************************
 * 
 */
type InMemDockerfileExecEvent struct {
	InMemImageCreationEvent
	DockerfileId string
	DockerfileExternalObjId string
}

func (client *InMemClient) dbCreateDockerfileExecEvent(dockerfileId, imageId,
	userObjId string) (DockerfileExecEvent, error) {
	
	//var id string = createUniqueDbObjectId()
	var newDockerfileExecEvent *InMemDockerfileExecEvent =
		client.NewInMemDockerfileExecEvent(dockerfileId, imageId, userObjId)
	
	allObjects[newDockerfileExecEvent.getId()] = newDockerfileExecEvent

	// Link to Dockerfile.
	var dockerfile Dockerfile
	var err error
	dockerfile, err = client.getDockerfile(dockerfileId)
	if err != nil { return nil, err }
	dockerfile.addEventId(newDockerfileExecEvent.getId())
	
	return newDockerfileExecEvent, nil
}

func (client *InMemClient) NewInMemDockerfileExecEvent(dockerfileId, imageId,
	userObjId string) *InMemDockerfileExecEvent {
	
	return &InMemDockerfileExecEvent{
		InMemImageCreationEvent: *client.NewInMemImageCreationEvent(userObjId, imageId),
		DockerfileId: dockerfileId,
		DockerfileExternalObjId: "",  // for when we add git
	}
}

func (execEvent *InMemDockerfileExecEvent) getDockerfileId() string {
	return execEvent.DockerfileId
}

func (execEvent *InMemDockerfileExecEvent) getDockerfileExternalObjId() string {
	return execEvent.DockerfileExternalObjId
}



/****************************** Utility Methods ********************************
 ******************************************************************************/

/*******************************************************************************
 * Create a globally unique id, to be used to uniquely identify a new persistent
 * object. The creation of the id must be done atomically.
 */
func createUniqueDbObjectId() string {
	return fmt.Sprintf("%d", atomic.AddInt64(&uniqueId, 1))
}

var uniqueId int64 = 5

var allObjects map[string]PersistObj

var allUsers map[string]User

var allRealmIds []string

/*******************************************************************************
 * Create a directory for the Dockerfiles, images, and any other files owned
 * by the specified realm.
 */
func (client *InMemClient) assignRealmFileDir(realmId string) (string, error) {
	var path = client.Server.Config.FileRepoRootPath + "/" + realmId
	// Create the directory. (It is an error if it already exists.)
	err := os.MkdirAll(path, 0711)
	return path, err
}

/*******************************************************************************
 * Create a directory for the Dockerfiles, images, and any other files owned
 * by the specified repo. The directory will be created as a subdirectory of the
 * realm's directory.
 */
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

/*******************************************************************************
 * Print the database to stdout. Diagnostic.
 */
func (client *InMemClient) printDatabase() {
	fmt.Println("Not implemented yet")
}