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
)

/*******************************************************************************
 * The Client type, and methods required by the Client interface in Persist.go.
 */
type InMemClient struct {
	Server *Server
}

func NewInMemClient(server *Server) *InMemClient {
	return &InMemClient{
		Server: server,
	}
}

/*******************************************************************************
 * Base type that is included in each data type as an anonymous field.
 */
type InMemPersistObj struct {
	Id string
}

var _ PersistObj = &InMemPersistObj{}

func (persObj *InMemPersistObj) getId() string {
	return persObj.Id
}

/*******************************************************************************
 * 
 */
type InMemGroup struct {
	InMemPersistObj
	RealmId string
	Name string
	aclEntryIds []string
}

func (client *InMemClient) dbCreateGroup(realmId string, name string) (*InMemGroup, error) {
	
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
		InMemPersistObj: InMemPersistObj{Id: groupId},
		RealmId: realmId,
		Name: name,
	}
	fmt.Println("Created Group")
	allObjects[groupId] = newGroup
	return newGroup, nil
}

//func (group *InMemGroup) getId() string {
//	return group.Id
//}

func (client *InMemClient) getGroup(id string) Group {
	return client.getPersistentObject(id).(Group)
}

func (group *InMemGroup) getName() string {
	return group.Name
}

func (group *InMemGroup) getACLEntryIds() []string {
	return group.aclEntryIds
}

func (group *InMemGroup) asGroupDesc() *GroupDesc {
	return &GroupDesc{
		RealmId: group.RealmId,
		Name: group.Name,
	}
}

/*******************************************************************************
 * 
 */
type InMemUser struct {
	InMemPersistObj
	RealmId string
	Name string
	ACLEntryIds []string
}

func (client *InMemClient) dbCreateUser(name string, realmId string) (*InMemUser, error) {
	var userId string = createUniqueDbObjectId()
	var newUser *InMemUser = &InMemUser{
		InMemPersistObj: InMemPersistObj{Id: userId},
		RealmId: realmId,
		Name: name,
	}
	fmt.Println("Created user")
	allObjects[userId] = newUser
	return newUser, nil
}

func (client *InMemClient) getUser(id string) User {
	return client.getPersistentObject(id).(User)
	//return User(client.getPersistentObject(id))
}

func (user *InMemUser) getName() string {
	return user.Name
}

func (user *InMemUser) getACLEntryIds() []string {
	return user.ACLEntryIds
}

/*******************************************************************************
 * 
 */
type InMemACLEntry struct {
	InMemPersistObj
	ACLId string
	IdentityId string
	PermissionMask []bool
}

func (client *InMemClient) dbCreateACLEntry(resourceId string, identityId string,
	permissionMask []bool) (*InMemACLEntry, error) {
	var obj PersistObj = client.getPersistentObject(resourceId)
	var acl ACL
	var isType bool
	acl, isType = obj.(Resource)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	var aclEntryId = createUniqueDbObjectId()
	var newACLEntry *InMemACLEntry = &InMemACLEntry{
		InMemPersistObj: InMemPersistObj{Id: aclEntryId},
		ACLId: acl.getId(),
		IdentityId: identityId,
		PermissionMask: permissionMask,
	}
	fmt.Println("Created ACLEntry")
	allObjects[aclEntryId] = newACLEntry
	return newACLEntry, nil
}

func (client *InMemClient) getACLEntry(id string) ACLEntry {
	return ACLEntry(client.getPersistentObject(id))
}

func (entry *InMemACLEntry) getACL() ACL {
	var acl ACL
	var isType bool
	acl, isType = allObjects[entry.ACLId].(ACL)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return acl
}

func (entry *InMemACLEntry) getIdentity() PersistObj {
	return allObjects[entry.IdentityId]
}

func (entry *InMemACLEntry) getPermissionMask() []bool {
	return entry.PermissionMask
}

/*******************************************************************************
 * 
 */
type InMemACL struct {
	InMemPersistObj
	ResourceId string
	ACLEntryIds []string
}

func (client *InMemClient) dbCreateACL(resourceId string) (*InMemACL, error) {
	var aclId = createUniqueDbObjectId()
	var newACL *InMemACL = &InMemACL{
		InMemPersistObj: InMemPersistObj{Id: aclId},
		ResourceId: resourceId,
	}
	fmt.Println("Created ACL, id=", aclId)
	allObjects[aclId] = newACL
	return newACL, nil
}

//func (acl *InMemACL) getId() string {
//	return acl.Id
//}

func (client *InMemClient) getACL(id string) ACL {
	var acl ACL
	var isType bool
	acl, isType = allObjects[id].(ACL)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return acl
}

func (acl *InMemACL) getResource() PersistObj {
	return allObjects[acl.ResourceId]
}

func (acl *InMemACL) getACLEntryIds() []string {
	return acl.ACLEntryIds
}

/*******************************************************************************
 * 
 */
type InMemRealm struct {
	InMemPersistObj
	Name string
	ACLId string
	UserIds []string
	GroupIds []string
	RepoIds []string
	FileDirectory string  // where this realm's files are stored
}

func (client *InMemClient) dbCreateRealm(realmInfo *RealmInfo) (*InMemRealm, error) {
	var realmId string = createUniqueDbObjectId()
	var newRealm *InMemRealm = &InMemRealm{
		InMemPersistObj: InMemPersistObj{Id: realmId},
		Name: realmInfo.Name,
		ACLId: "",
		FileDirectory: client.assignRealmFileDir(realmId),
	}
	var err error
	var acl *InMemACL
	acl, err = client.dbCreateACL(realmId)
	if err != nil { return nil, err }
	newRealm.ACLId = acl.Id
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

//func (realm *InMemRealm) getId() string {
//	return realm.Id
//}

func (realm *InMemRealm) getFileDirectory() string {
	fmt.Println("getFileDirectory...")
	return realm.FileDirectory
}

func (client *InMemClient) getRealm(id string) Realm {
	fmt.Println("getRealm(", id, ")...")
	var realm Realm
	var isType bool
	realm, isType = allObjects[id].(Realm)
	if ! isType {
		fmt.Println("realm is a", reflect.TypeOf(realm))
		fmt.Println("allObjects[", id, "] is a", reflect.TypeOf(allObjects[id]))
		panic(errors.New("Internal error: object is an unexpected type"))
	}
	return realm
}

func (realm *InMemRealm) getName() string {
	return realm.Name
}

func (realm *InMemRealm) getUserIds() []string {
	return realm.UserIds
}

func (realm *InMemRealm) getGroupIds() []string {
	return realm.GroupIds
}

func (realm *InMemRealm) getRepoIds() []string {
	return realm.RepoIds
}

func (realm *InMemRealm) getACL() ACL {
	var acl ACL
	var isType bool
	acl, isType = allObjects[realm.ACLId].(ACL)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return acl
}

func (realm *InMemRealm) asRealmDesc() *RealmDesc {
	return NewRealmDesc(realm.Id, realm.Name)
}

func (realm *InMemRealm) hasUserWithId(userId string) bool {
	var obj PersistObj = allObjects[userId]
	if obj == nil { return false }
	_, isUser := obj.(User)
	if ! isUser { return false }
	
	for _, id := range realm.UserIds {
		if id == userId { return true }
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
	for _, id := range realm.UserIds {
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

/*******************************************************************************
 * 
 */
type InMemRepo struct {
	InMemPersistObj
	RealmId string
	Name string
	ACLId string
	DockerfileIds []string
	DockerImageIds []string
	FileDirectory string  // where this repo's files are stored
}

func (client *InMemClient) dbCreateRepo(realmId string, name string) (*InMemRepo, error) {
	var repoId string = createUniqueDbObjectId()
	var newRepo *InMemRepo = &InMemRepo{
		InMemPersistObj: InMemPersistObj{Id: repoId},
		RealmId: realmId,
		Name: name,
		ACLId: "",
		FileDirectory: client.assignRepoFileDir(realmId, repoId),
	}
	var err error
	var acl *InMemACL
	acl, err = client.dbCreateACL(repoId)
	if err != nil { return nil, err }
	newRepo.ACLId = acl.Id
	fmt.Println("Created repo")
	allObjects[repoId] = newRepo
	return newRepo, nil
}

//func (repo *InMemRepo) getId() string {
//	return repo.Id
//}

func (repo *InMemRepo) getName() string {
	return repo.Name
}

func (repo *InMemRepo) getFileDirectory() string {
	return repo.FileDirectory
}

func (client *InMemClient) getRepo(id string) Repo {
	fmt.Println("getRepo(", id, ")...")
	var repo Repo
	var isType bool
	repo, isType = allObjects[id].(Repo)
	fmt.Println("getRepo.A")
	if ! isType {
		fmt.Println("***********allObjects[", id, "] is a", reflect.TypeOf(allObjects[id]))
		panic(errors.New("************Internal error: object is an unexpected type"))
	}
	fmt.Println("getRepo.B")
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

func (repo *InMemRepo) getACL() ACL {
	var acl ACL
	var isType bool
	acl, isType = allObjects[repo.ACLId].(ACL)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return acl
}

func (repo *InMemRepo) asRepoDesc() *RepoDesc {
	return NewRepoDesc(repo.Id, repo.RealmId, repo.Name)
}

/*******************************************************************************
 * 
 */
type InMemDockerfile struct {
	InMemPersistObj
	RepoId string
	Name string
	ACLId string
}

func (client *InMemClient) dbCreateDockerfile(repoId string, name string,
	filepath string) (*InMemDockerfile, error) {
	var dockerfileId string = createUniqueDbObjectId()
	var newDockerfile *InMemDockerfile = &InMemDockerfile{
		InMemPersistObj: InMemPersistObj{Id: dockerfileId},
		RepoId: repoId,
		Name: name,
		ACLId: "",
	}
	var err error
	var acl *InMemACL
	acl, err = client.dbCreateACL(dockerfileId)
	if err != nil { return nil, err }
	newDockerfile.ACLId = acl.Id
	fmt.Println("Created Dockerfile")
	allObjects[dockerfileId] = newDockerfile
	return newDockerfile, nil
}

//func (dockerfile *InMemDockerfile) getId() string {
//	return dockerfile.Id
//}

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

func (dockerfile *InMemDockerfile) getName() string {
	return dockerfile.Name
}

func (dockerfile *InMemDockerfile) getACL() ACL {
	var acl ACL
	var isType bool
	acl, isType = allObjects[dockerfile.ACLId].(ACL)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return acl
}

func (dockerfile *InMemDockerfile) asDockerfileDesc() *DockerfileDesc {
	return NewDockerfileDesc(dockerfile.Id, dockerfile.RepoId, dockerfile.Name)
}

/*******************************************************************************
 * 
 */
type InMemDockerImage struct {
	InMemPersistObj
	RepoId string
	ACLId string
}

func (client *InMemClient) dbCreateDockerImage(repoId string,
	filepath string) (*InMemDockerImage, error) {
	var imageId string = createUniqueDbObjectId()
	var newDockerImage *InMemDockerImage = &InMemDockerImage{
		InMemPersistObj: InMemPersistObj{Id: imageId},
		RepoId: repoId,
		ACLId: "",
	}
	var err error
	var acl *InMemACL
	acl, err = client.dbCreateACL(imageId)
	if err != nil { return nil, err }
	newDockerImage.ACLId = acl.Id
	fmt.Println("Created DockerImage")
	allObjects[imageId] = newDockerImage
	return newDockerImage, nil
}

//func (image *InMemDockerImage) getId() string {
//	return image.Id
//}

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

func (image *InMemDockerImage) getACL() ACL {
	var acl ACL
	var isType bool
	acl, isType = allObjects[image.ACLId].(ACL)
	if ! isType { panic(errors.New("Internal error: object is an unexpected type")) }
	return acl
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

var allObjects map[string]PersistObj = make(map[string]PersistObj)

/*******************************************************************************
 * Return the persistent object that is identified by the specified unique id.
 * An object's Id is assigned to it by the function that creates the object.
 */
func (client *InMemClient) getPersistentObject(id string) PersistObj {
	return allObjects[id]
}


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
