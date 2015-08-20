/*******************************************************************************
 * In-memory implementation of the methods defined in Persist.go.
 *
 * There are three parts to this file:
 *    Definition of the Client type "InMemClient".
 *    Definition of each data object type, as defined in the slide "Access Control Model"
 *      of the design,
 *      https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 *    Some utility methods.
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
)

func NewInMemClient() *InMemClient {
	return &InMemClient{}
}

/****************************** Client Type ************************************
 *******************************************************************************
 * The Client type, and methods required by the Client interface in Persist.go.
 */
type InMemClient struct {
}

func (client *InMemClient) dbCreateGroup() *InMemGroup {
	return &InMemGroup{}
}

func (client *InMemClient) dbCreateUser() *InMemUser {
	return nil
}

func (client *InMemClient) dbCreateACLEntry() *InMemACLEntry {
	return nil
}

func (client *InMemClient) dbCreateACL() *InMemACL {
	return nil
}

func (client *InMemClient) dbCreateRealm(realmInfo *RealmInfo) *InMemRealm {
	var newRealm *InMemRealm = &InMemRealm{
		InMemPersistObj: InMemPersistObj{Id: createUniqueDbObjectId()},
		Name: realmInfo.Name,
	}
	fmt.Println("Created realm")
	return newRealm
}

func (client *InMemClient) dbCreateRepo(realmId string, name string) *InMemRepo {
	var newRepo *InMemRepo = &InMemRepo{
		InMemPersistObj: InMemPersistObj{Id: createUniqueDbObjectId()},
		RealmId: realmId,
		Name: name,
	}
	fmt.Println("Created repo")
	return newRepo
}

func (client *InMemClient) dbCreateDockerfile() *InMemDockerfile {
	return nil
}

func (client *InMemClient) dbCreateDockerImage() *InMemDockerImage {
	return nil
}

/******************************** Data Types ***********************************
 ******************************************************************************/

/*******************************************************************************
 * Base type that is included in each data type as an anonymous field.
 */
type InMemPersistObj struct {
	Id string
}

func (obj InMemPersistObj) Delete() {
}

func (obj InMemPersistObj) Clone() *InMemPersistObj {
	return nil
}

/*******************************************************************************
 * 
 */
type InMemGroup struct {
	InMemPersistObj
}

func (group *InMemGroup) Delete() {
}

func (group *InMemGroup) Clone() *InMemGroup {
	return &InMemGroup{}
}

/*******************************************************************************
 * 
 */
type InMemUser struct {
	InMemPersistObj
}

func (user *InMemUser) Clone() *InMemUser {
	return &InMemUser{}
}

/*******************************************************************************
 * 
 */
type InMemACLEntry struct {
	InMemPersistObj
}

func (aclEntry *InMemACLEntry) Clone() *InMemACLEntry {
	return &InMemACLEntry{}
}

/*******************************************************************************
 * 
 */
type InMemACL struct {
	InMemPersistObj
}

func (acl *InMemACL) Clone() *InMemACL {
	return &InMemACL{}
}

/*******************************************************************************
 * 
 */
type InMemRealm struct {
	InMemPersistObj
	Name string
}

func (realm *InMemRealm) asRealmDesc() *RealmDesc {
	return NewRealmDesc(realm.Id, realm.Name)
}

func (realm *InMemRealm) Clone() *InMemRealm {
	return &InMemRealm{
		Name: "",
	}
}

/*******************************************************************************
 * 
 */
type InMemRepo struct {
	InMemPersistObj
	RealmId string
	Name string
}

func (repo *InMemRepo) asRepoDesc() *RepoDesc {
	return NewRepoDesc(repo.Id, repo.RealmId, repo.Name)
}

func (repo *InMemRepo) Clone() *InMemRepo {
	return &InMemRepo{}
}

/*******************************************************************************
 * 
 */
type InMemDockerfile struct {
	InMemPersistObj
}

func (dockerfile *InMemDockerfile) Clone() *InMemDockerfile {
	return &InMemDockerfile{}
}

/*******************************************************************************
 * 
 */
type InMemDockerImage struct {
	InMemPersistObj
}

func (dockerImage *InMemDockerImage) Clone() *InMemDockerImage {
	return &InMemDockerImage{}
}

/****************************** Utility Methods ********************************
 ******************************************************************************/

/*******************************************************************************
 * 
 */
func createUniqueDbObjectId() string {
	return fmt.Sprintf("%d", atomic.AddInt64(&uniqueId, 1))
}

var uniqueId int64 = 0
