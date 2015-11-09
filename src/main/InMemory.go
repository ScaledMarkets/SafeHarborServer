/*******************************************************************************
 * In-memory implementation of the methods defined in Persist.go.
 *
 * The Group, Permission, Repo, Dockerfile, Image, User, and Realm have
 * asGroupDesc, asPermissionDesc, asRepoDesc, asDockerfileDesc, asImageDesc,
 * asUserDesc, and asRealmDesc methods, respectively - these methods construct
 * instances of GroupDesc, PermissionDesc, RepoDesc, DockerfileDesc, ImageDesc,
 * and so on. These methods are a convenient way of constructing the return values
 * that are needed by the handler methods defined in the API (slides titled
 * "SafeHarbor REST API" of the desgin), which are implemented by the functions
 * in Handlers.go.
 */

package main

import (
	"fmt"
	"sync/atomic"
	"errors"
	"reflect"
	"os"
	"crypto/sha1"
	"time"
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
		var realmInfo *RealmInfo
		realmInfo, err = NewRealmInfo("testrealm", "For Testing")
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
		fmt.Println("User", testUser1.getName(), "created, id=", testUser1.getId())
	}
	
	fmt.Println("Repository initialized")
}

/*******************************************************************************
 * Base type that is included in each data type as an anonymous field.
 */
type InMemPersistObj struct {
	Id string
	Client DBClient
}

var _ PersistObj = &InMemPersistObj{}

func (client *InMemClient) NewInMemPersistObj() *InMemPersistObj {
	return &InMemPersistObj{Id: "", Client: client}
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
	ACLEntryIds []string
}

func NewInMemACL() *InMemACL {
	return &InMemACL{
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
	CreationTime time.Time
}

func NewInMemResource(name string, curTime time.Time) *InMemResource {
	return &InMemResource{
		InMemACL: *NewInMemACL(),
		Name: name,
		CreationTime: curTime,
	}
}

func (resource *InMemResource) getName() string {
	return resource.Name
}

func (resource *InMemResource) getCreationTime() time.Time {
	return resource.CreationTime
}

func (resource *InMemResource) getACLEntryForPartyId(partyId string) ACLEntry {
	for _, entryId := range resource.getACLEntryIds() {
		var obj interface{} = allObjects[entryId]
		if obj == nil {
			fmt.Println("Internal error - no object found for Id " + entryId);
			continue
		}
		var entry ACLEntry
		var isType bool
		entry, isType = obj.(ACLEntry)
		if ! isType {
			fmt.Println("Internal error - object with Id " + entryId + " is not an ACLEntry");
			continue
		}
		if entry.getPartyId() == partyId {
			return entry
		}
	}
	return nil
}

func (client *InMemClient) getResource(resourceId string) Resource {
	return client.getPersistentObject(resourceId).(Resource)
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
	Name string
	CreationTime time.Time
	RealmId string
	ACLEntryIds []string
}

func NewInMemParty(name string) *InMemParty {
	return &InMemParty{
		Name: name,
		ACLEntryIds: make([]string, 0),
	}
}

func (party *InMemParty) getName() string {
	return party.Name
}

func (party *InMemParty) getCreationTime() time.Time {
	return party.CreationTime
}

func (client *InMemClient) getParty(partyId string) Party {
	return client.getPersistentObject(partyId).(Party)
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

func (party *InMemParty) getACLEntryForResourceId(resourceId string) ACLEntry {
	for _, entryId := range party.getACLEntryIds() {
		var obj interface{} = allObjects[entryId]
		if obj == nil {
			fmt.Println("Internal error - no object found for Id " + entryId);
			continue
		}
		var entry ACLEntry
		var isType bool
		entry, isType = obj.(ACLEntry)
		if ! isType {
			fmt.Println("Internal error - object with Id " + entryId + " is not an ACLEntry");
			continue
		}
		if entry.getResourceId() == resourceId {
			return entry
		}
	}
	return nil
}

/*******************************************************************************
 * 
 */
type InMemGroup struct {
	InMemPersistObj
	InMemParty
	Description string
	RealmId string
	UserObjIds []string
}

func (client *InMemClient) dbCreateGroup(realmId string, name string,
	description string) (Group, error) {
	
	// Check if a group with that name already exists within the realm.
	var realm Realm = client.getRealm(realmId)
	if realm == nil { return nil, errors.New(fmt.Sprintf(
		"Unidentified realm for realm Id %s", realmId))
	}
	if realm.getGroupByName(name) != nil { return nil, errors.New(
		fmt.Sprintf("Group named %s already exists within realm %s", name,
			client.getRealm(realmId).getName()))
	}
	
	var groupId string = createUniqueDbObjectId()
	var newGroup = &InMemGroup{
		InMemPersistObj: InMemPersistObj{Id: groupId, Client: client},
		InMemParty: InMemParty{Name: name, CreationTime: time.Now(), ACLEntryIds: make([]string, 0)},
		Description: description,
		RealmId: realmId,
		UserObjIds: make([]string, 0),
	}
	
	// Add to parent realm's list
	realm.addGroup(newGroup)
	
	fmt.Println("Created Group")
	allObjects[groupId] = newGroup
	return newGroup, nil
}

func (client *InMemClient) getGroup(id string) Group {
	return client.getPersistentObject(id).(Group)
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

func (group *InMemGroup) asGroupDesc() *GroupDesc {
	return &GroupDesc{
		RealmId: group.RealmId,
		GroupName: group.Name,
		CreationDate: FormatTimeAsJavascriptDate(group.CreationTime),
		Description: group.Description,
		GroupId: group.Id,
	}
}

/*******************************************************************************
 * 
 */
type InMemUser struct {
	InMemPersistObj
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
	
	var realm Realm = client.getRealm(realmId)
	if realm == nil { return nil, errors.New("Realm with Id " + realmId + " not found") }
	
	var userObjId string = createUniqueDbObjectId()
	var pswdAsBytes []byte = []byte(pswd)
	var newUser *InMemUser = &InMemUser{
		InMemPersistObj: InMemPersistObj{Id: userObjId, Client: client},
		InMemParty: InMemParty{Name: name, CreationTime: time.Now(), RealmId: realmId, ACLEntryIds: make([]string, 0)},
		UserId: userId,
		EmailAddress: email,
		PasswordHash: sha1.Sum(pswdAsBytes),
		GroupIds: make([]string, 0),
	}
	
	// Add to parent realm's list.
	realm.addUser(newUser)
	
	fmt.Println("Created user")
	allObjects[userObjId] = newUser
	return newUser, nil
}

func (client *InMemClient) getUser(id string) User {
	return client.getPersistentObject(id).(User)
	//return User(client.getPersistentObject(id))
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

func (client *InMemClient) getRealmsAdministeredByUser(userObjId string) []string {
	// those realms for which user can edit the realm
	
	var realmIds []string = make([]string, 0)
	
	// Identify the user.
	var obj PersistObj = client.getPersistentObject(userObjId)
	if obj == nil {
		fmt.Println("Object with Id " + userObjId + " not found")
		return realmIds
	}
	var user User
	var isType bool
	user, isType = obj.(User)
	if ! isType {
		fmt.Println("Internal error: object with Id " + userObjId + " is not a User")
		return realmIds
	}
	
	// Identify those ACLEntries that are for realms and for which the user has write access.
	for _, entryId := range user.getACLEntryIds() {
		var entry ACLEntry = client.getACLEntry(entryId)
		if entry == nil {
			fmt.Println("Internal error: object with Id " + entryId + " is not an ACLEntry")
			continue
		}
		var resourceId string = entry.getResourceId()
		var resource Resource = client.getResource(resourceId)
		if resource == nil {
			fmt.Println("Internal error: resource with Id " + resourceId + " not found")
			continue
		}
		if resource.isRealm() {
			var realm Realm = resource.(Realm)
			var mask []bool = entry.getPermissionMask()
			if mask[CanWrite] { // entry has write access for the realm
				realmIds = append(realmIds, realm.getId())
			}
		}
	}
	
	return realmIds
}

func (user *InMemUser) asUserDesc() *UserDesc {
	var adminRealmIds []string = user.getDBClient().getRealmsAdministeredByUser(user.getId())
	return &UserDesc{
		Id: user.Id,
		UserId: user.UserId,
		UserName: user.Name,
		RealmId: user.RealmId,
		CanModifyTheseRealms: adminRealmIds,
	}
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
	
	assertThat(resourceId != "", "Internal error: resourceId is empty")
	assertThat(partyId != "", "Internal error: partyId is empty")
	var resource Resource
	var party Party
	var isType bool
	var obj PersistObj = client.getPersistentObject(resourceId)
	resource, isType = obj.(Resource)
	assertThat(obj != nil, "Internal error: obj is nil")
	assertThat(isType, "Internal error: object is not a Resource - it is a " +
		reflect.TypeOf(obj).String())
	obj = client.getPersistentObject(partyId)
	assertThat(obj != nil, "Internal error: obj is nil")
	party, isType = obj.(Party)
	assertThat(isType, "Internal error: object is not a Party - it is a " +
		reflect.TypeOf(obj).String())
	var aclEntryId = createUniqueDbObjectId()
	var newACLEntry *InMemACLEntry = &InMemACLEntry{
		InMemPersistObj: InMemPersistObj{Id: aclEntryId, Client: client},
		ResourceId: resource.getId(),
		PartyId: partyId,
		PermissionMask: permissionMask,
	}
	allObjects[aclEntryId] = newACLEntry
	resource.addACLEntry(newACLEntry)  // Add to resource's ACL
	party.addACLEntry(newACLEntry)  // Add to user or group's ACL
	fmt.Println("Added ACL entry for " + party.getName() + "(a " +
		reflect.TypeOf(party).String() + "), to access " +
		resource.getName() + " (a " + reflect.TypeOf(resource).String() + ")")
	return newACLEntry, nil
}

func (client *InMemClient) getACLEntry(id string) ACLEntry {
	var aclEntry ACLEntry
	var isType bool
	aclEntry, isType = client.getPersistentObject(id).(ACLEntry)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return aclEntry
}

func (entry *InMemACLEntry) getResourceId() string {
	return entry.ResourceId
}

func (entry *InMemACLEntry) getPartyId() string {
	return entry.PartyId
}

func (entry *InMemACLEntry) getParty() Party {
	var party Party
	var isType bool
	party, isType = allObjects[entry.PartyId].(Party)
	if ! isType { panic(errors.New("Internal error: object is not a Party")) }
	return party
}

func (entry *InMemACLEntry) getPermissionMask() []bool {
	return entry.PermissionMask
}

func (entry *InMemACLEntry) setPermissionMask(mask []bool) {
	entry.PermissionMask = mask
}

func (entry *InMemACLEntry) asPermissionDesc() *PermissionDesc {
	
	return NewPermissionDesc(entry.getId(), entry.ResourceId, entry.PartyId, entry.getPermissionMask())
}

/*******************************************************************************
 * 
 */
type InMemRealm struct {
	InMemPersistObj
	InMemResource
	AdminUserId string
	OrgFullName string
	UserObjIds []string
	GroupIds []string
	RepoIds []string
	FileDirectory string  // where this realm's files are stored
}

func (client *InMemClient) dbCreateRealm(realmInfo *RealmInfo, adminUserId string) (Realm, error) {
	
	var realmId string = client.getRealmIdByName(realmInfo.RealmName)
	if realmId != "" {
		return nil, errors.New("A realm with name " + realmInfo.RealmName + " already exists")
	}
	realmId = createUniqueDbObjectId()
	var newRealm *InMemRealm = &InMemRealm{
		InMemPersistObj: InMemPersistObj{Id: realmId, Client: client},
		InMemResource: *NewInMemResource(realmInfo.RealmName, time.Now()),
		AdminUserId: adminUserId,
		OrgFullName: realmInfo.OrgFullName,
		UserObjIds: make([]string, 0),
		GroupIds: make([]string, 0),
		RepoIds: make([]string, 0),
		FileDirectory: client.assignRealmFileDir(realmId),
	}
	
	allRealmIds = append(allRealmIds, realmId)
	
	fmt.Println("Created realm")
	allObjects[realmId] = newRealm
	_, isType := allObjects[realmId].(Realm)
	if ! isType {
		fmt.Println("*******realm", realmId, "is not a Realm")
		fmt.Println("newRealm is a", reflect.TypeOf(newRealm))
		fmt.Println("allObjects[", realmId, "] is a", reflect.TypeOf(allObjects[realmId]))
	}
	return newRealm, nil
}

func (client *InMemClient) dbGetAllRealmIds() []string {
	return allRealmIds
}

func (client *InMemClient) getRealmIdByName(name string) string {
	for _, realmId := range allRealmIds {
		var realm Realm = client.getRealm(realmId)
		if realm.getName() == name { return realmId }
	}
	return ""
}

func (realm *InMemRealm) getAdminUserId() string {
	return realm.AdminUserId
}

func (realm *InMemRealm) getFileDirectory() string {
	fmt.Println("getFileDirectory...")
	return realm.FileDirectory
}

func (client *InMemClient) getRealm(id string) Realm {
	fmt.Println("getRealm(", id, ")...")
	var realm Realm
	var isType bool
	realm, isType = allObjects[id].(Realm)
	if realm == nil { return nil }
	if ! isType {
		fmt.Println("realm is a", reflect.TypeOf(realm))
		fmt.Println("allObjects[", id, "] is a", reflect.TypeOf(allObjects[id]))
		panic(errors.New("Internal error: object is an unexpected type"))
	}
	return realm
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
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
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

func (realm *InMemRealm) asRealmDesc() *RealmDesc {
	return NewRealmDesc(realm.Id, realm.Name, realm.OrgFullName, realm.AdminUserId)
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

func (realm *InMemRealm) getUserByName(userName string) User {
	for _, id := range realm.UserObjIds {
		var obj PersistObj = allObjects[id]
		if obj == nil { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id)))
		}
		user, isUser := obj.(User)
		if ! isUser { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a User", id)))
		}
		if user.getName() == userName { return user }
	}
	return nil
}

func (realm *InMemRealm) getUserByUserId(userId string) User {
	for _, id := range realm.UserObjIds {
		var obj PersistObj = allObjects[id]
		if obj == nil { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id)))
		}
		user, isUser := obj.(User)
		if ! isUser { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a User", id)))
		}
		if user.getUserId() == userId { return user }
	}
	return nil
}

func (realm *InMemRealm) getGroupByName(groupName string) Group {
	for _, id := range realm.GroupIds {
		var obj PersistObj = allObjects[id]
		if obj == nil { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id)))
		}
		group, isGroup := obj.(Group)
		if ! isGroup { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a Group", id)))
		}
		if group.getName() == groupName { return group }
	}
	return nil
}

func (realm *InMemRealm) getRepoByName(repoName string) Repo {
	for _, id := range realm.RepoIds {
		var obj PersistObj = allObjects[id]
		if obj == nil { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id)))
		}
		repo, isRepo := obj.(Repo)
		if ! isRepo { panic(errors.New(fmt.Sprintf(
			"Internal error: obj with Id %s is not a Repo", id)))
		}
		if repo.getName() == repoName { return repo }
	}
	return nil
}

func (realm *InMemRealm) isRealm() bool { return true }

/*******************************************************************************
 * 
 */
type InMemRepo struct {
	InMemPersistObj
	InMemResource
	RealmId string
	Description string
	DockerfileIds []string
	DockerImageIds []string
	FileDirectory string  // where this repo's files are stored
}

func (client *InMemClient) dbCreateRepo(realmId string, name string, desc string) (Repo, error) {
	var realm Realm = client.getRealm(realmId)
	var repoId string = createUniqueDbObjectId()
	var newRepo *InMemRepo = &InMemRepo{
		InMemPersistObj: InMemPersistObj{Id: repoId, Client: client},
		InMemResource: *NewInMemResource(name, time.Now()),
		RealmId: realmId,
		Description: desc,
		DockerfileIds: make([]string, 0),
		DockerImageIds: make([]string, 0),
		FileDirectory: client.assignRepoFileDir(realmId, repoId),
	}
	fmt.Println("Created repo")
	allObjects[repoId] = newRepo
	realm.addRepo(newRepo)  // Add it to the realm.
	return newRepo, nil
}

func (repo *InMemRepo) getFileDirectory() string {
	return repo.FileDirectory
}

func (client *InMemClient) getRepo(id string) Repo {
	var repo Repo
	var isType bool
	repo, isType = allObjects[id].(Repo)
	if ! isType {
		fmt.Println("***********allObjects[", id, "] is a", reflect.TypeOf(allObjects[id]))
		panic(errors.New("************Internal error: object is an unexpected type"))
	}
	return repo
}

func (repo *InMemRepo) getRealm() Realm {
	var realm Realm
	var isType bool
	realm, isType = allObjects[repo.RealmId].(Realm)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return realm
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

func (repo *InMemRepo) asRepoDesc() *RepoDesc {
	return NewRepoDesc(repo.Id, repo.RealmId, repo.Name, repo.Description)
}

func (repo *InMemRepo) getParentId() string {
	return repo.RealmId
}

func (repo *InMemRepo) isRepo() bool { return true }

/*******************************************************************************
 * 
 */
type InMemDockerfile struct {
	InMemPersistObj
	InMemResource
	RepoId string
	Description string
	FilePath string
}

func (client *InMemClient) dbCreateDockerfile(repoId string, name string,
	desc string, filepath string) (Dockerfile, error) {
	var dockerfileId string = createUniqueDbObjectId()
	var newDockerfile *InMemDockerfile = &InMemDockerfile{
		InMemPersistObj: InMemPersistObj{Id: dockerfileId, Client: client},
		InMemResource: *NewInMemResource(name, time.Now()),
		RepoId: repoId,
		Description: desc,
		FilePath: filepath,
	}
	fmt.Println("Created Dockerfile")
	allObjects[dockerfileId] = newDockerfile
	
	// Add to the Repo's list of Dockerfiles.
	var repo Repo = client.getRepo(repoId)
	if repo == nil {
		fmt.Println("Repo with Id " + repoId + " not found")
		return nil, errors.New(fmt.Sprintf("Repo with Id %s not found", repoId))
	}
	repo.addDockerfile(newDockerfile)
	
	return newDockerfile, nil
}

func (client *InMemClient) getDockerfile(id string) Dockerfile {
	var dockerfile Dockerfile
	var isType bool
	dockerfile, isType = allObjects[id].(Dockerfile)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return dockerfile
}

func (dockerfile *InMemDockerfile) getRepo() Repo {
	var repo Repo
	var isType bool
	repo, isType = allObjects[dockerfile.RepoId].(Repo)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return repo
}

func (dockerfile *InMemDockerfile) getFilePath() string {
	return dockerfile.FilePath
}

func (dockerfile *InMemDockerfile) asDockerfileDesc() *DockerfileDesc {
	return NewDockerfileDesc(dockerfile.Id, dockerfile.RepoId, dockerfile.Name, dockerfile.Description)
}

func (dockerfile *InMemDockerfile) getParentId() string {
	return dockerfile.RepoId
}

func (dockerfile *InMemDockerfile) isDockerfile() bool { return true }

/*******************************************************************************
 * 
 */
type InMemDockerImage struct {
	InMemPersistObj
	InMemResource
	RepoId string
}

func (client *InMemClient) dbCreateDockerImage(repoId string,
	dockerImageTag string) (DockerImage, error) {
	
	var repo Repo
	var isType bool
	repo, isType = allObjects[repoId].(Repo)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	
	var imageId string = createUniqueDbObjectId()
	var newDockerImage *InMemDockerImage = &InMemDockerImage{
		InMemPersistObj: InMemPersistObj{Id: imageId, Client: client},
		InMemResource: *NewInMemResource(dockerImageTag, time.Now()),
		RepoId: repoId,
	}
	fmt.Println("Created DockerImage")
	allObjects[imageId] = newDockerImage
	repo.addDockerImage(newDockerImage)  // Add to repo's list.
	return newDockerImage, nil
}

func (image *InMemDockerImage) getName() string {
	return image.Name
}

func (client *InMemClient) getDockerImage(id string) DockerImage {
	var image DockerImage
	var isType bool
	image, isType = allObjects[id].(DockerImage)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return image
}

func (image *InMemDockerImage) getRepo() Repo {
	var repo Repo
	var isType bool
	repo, isType = allObjects[image.RepoId].(Repo)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return repo
}

func (image *InMemDockerImage) getDockerImageId() string {
	return image.Name
}

func (image *InMemDockerImage) asDockerImageDesc() *DockerImageDesc {
	return NewDockerImageDesc(image.Id, image.Name)
}

func (image *InMemDockerImage) getParentId() string {
	return image.RepoId
}

func (image *InMemDockerImage) isDockerImage() bool { return true }


/*******************************************************************************
 * 
 */
type InMemEvent struct {
	InMemPersistObj
	When time.Time
	UserId string
}

func (event *InMemEvent) getWhen() time.Time {
	return event.When
}

func (event *InMemEvent) getUserId() string {
	return event.UserId
}

func (event *InMemEvent) asEventDesc() *EventDesc {
	return NewEventDesc(event.Id, event.When, event.UserId)
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
func (client *InMemClient) assignRealmFileDir(realmId string) string {
	var path = client.Server.Config.FileRepoRootPath + "/" + realmId
	// Create the directory. (It is an error if it already exists.)
	err := os.MkdirAll(path, 0711)
	if err != nil { panic(err) }
	return path
}

/*******************************************************************************
 * Create a directory for the Dockerfiles, images, and any other files owned
 * by the specified repo. The directory will be created as a subdirectory of the
 * realm's directory.
 */
func (client *InMemClient) assignRepoFileDir(realmId string, repoId string) string {
	fmt.Println("assignRepoFileDir(", realmId, ",", repoId, ")...")
	var realm Realm = client.getRealm(realmId)
	var path = realm.getFileDirectory() + "/" + repoId
	err := os.MkdirAll(path, 0711)
	if err != nil { panic(err) }
	return path
}

/*******************************************************************************
 * Print the database to stdout. Diagnostic.
 */
func (client *InMemClient) printDatabase() {
	fmt.Println("Not implemented yet")
}
