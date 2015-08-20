/*******************************************************************************
 * In-memory implementation of the methods defined in Persist.go.
 */

package main

import (
	"fmt"
	"sync/atomic"
)

func NewInMemClient() *InMemClient {
	return &InMemClient{}
}

/*******************************************************************************
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

/*******************************************************************************
 * 
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

/*******************************************************************************
 * 
 */
func createUniqueDbObjectId() string {
	return fmt.Sprintf("%d", atomic.AddInt64(&uniqueId, 1))
}

var uniqueId int64 = 0
