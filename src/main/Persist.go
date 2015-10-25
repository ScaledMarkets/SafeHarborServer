/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */

package main

import (
	"time"
)

type DBClient interface {
	dbGetUserByUserId(string) User
	dbCreateGroup(string, string, string) (Group, error)
	dbCreateUser(string, string, string, string, string) (User, error)
	dbCreateACLEntry(string, string, []bool) (ACLEntry, error)
	dbCreateRealm(*RealmInfo, string) (Realm, error)
	dbCreateRepo(string, string) (Repo, error)
	dbCreateDockerfile(string, string, string) (Dockerfile, error)
	dbCreateDockerImage(string, string) (DockerImage, error)
	dbGetAllRealmIds() []string
	getResource(string) Resource
	getParty(string) Party
	getGroup(string) Group
	getUser(string) User
	getACLEntry(string) ACLEntry
	getRealm(string) Realm
	getRepo(string) Repo
	getDockerfile(string) Dockerfile
	getDockerImage(string) DockerImage
	getRealmsAdministeredByUser(string) []string  // those realms for which user can edit the realm
	init()
}

type PersistObj interface {
	getId() string
	getDBClient() DBClient
}

/* A Party is a User or a Group. Parties act on Resources. */
type Party interface {
	PersistObj
	getRealmId() string
	getName() string
	getCreationTime() time.Time
	getACLEntryIds() []string
	addACLEntry(ACLEntry)
	getACLEntryForResourceId(string) ACLEntry
}

type Group interface {
	Party
	getDescription() string
	getUserObjIds() []string
	hasUserWithId(string) bool
	addUserId(string) error
	addUser(User)
	asGroupDesc() *GroupDesc
}

type User interface {
	Party
	getUserId() string
	hasGroupWithId(string) bool
	asUserDesc() *UserDesc
	addGroupId(string) error
	getGroupIds() []string
	
	//getEventIds() []string
}

type ACLEntry interface {
	PersistObj
	getResourceId() string
	getPartyId() string
	getPermissionMask() []bool
	setPermissionMask([]bool)
	asPermissionDesc() *PermissionDesc
}

type ACL interface {
	PersistObj
	getACLEntryIds() []string
	addACLEntry(ACLEntry)
}

/* A Resource is something that a party can act upon. */
type Resource interface {
	ACL
	getName() string
	getCreationTime() time.Time
	getACLEntryForPartyId(string) ACLEntry
	getParentId() string
	isRealm() bool
	isRepo() bool
	isDockerfile() bool
	isDockerImage() bool
}

type Realm interface {
	Resource
	//getName() string
	getAdminUserId() string
	getFileDirectory() string
	hasUserWithId(string) bool
	hasGroupWithId(string) bool
	hasRepoWithId(string) bool
	getUserByName(string) User
	getGroupByName(string) Group
	getRepoByName(string) Repo
	getUserObjIds() []string
	getRepoIds() []string
	addUserId(string) error
	getUserByUserId(string) User
	asRealmDesc() *RealmDesc
	getGroupIds() []string
	addGroup(Group)
	addUser(User)
	addRepo(Repo)
}

type Repo interface {
	Resource
	//getName() string
	getFileDirectory() string
	getRealm() Realm
	getDockerfileIds() []string
	getDockerImageIds() []string
	addDockerfile(Dockerfile)
	addDockerImage(DockerImage)
	asRepoDesc() *RepoDesc
	
	//getDatasetIds() []string
	//getFlagIds() []string
}

type Dockerfile interface {
	Resource
	getFilePath() string
	asDockerfileDesc() *DockerfileDesc
	
	//getDockerfileExecEventIds() []string
}

type DockerImage interface {
	Resource
	//ImageCreationEvent
	getDockerImageId() string
	asDockerImageDesc() *DockerImageDesc
	
	//getScanEventIds() []string
}



// For Image Workflow:

type Event interface {
	getWhen() string
	getUserId() string
}

type ScanEvent interface {
	Event
	getScore() string
	getDockerImageId() string
	getDatasetIds() []string
}

type ImageCreationEvent interface {
	Event
}

type DockerfileExecEvent interface {
	ImageCreationEvent
	getDockerfileId() string
}

type UploadEvent interface {
	ImageCreationEvent
}

type Dataset interface {
	getRepoId() string
	getRepoExternalObjPath() string
	//RepoExternalObjId string
	getScanEventIds() []string
}

type Flag interface {
	getExpr() string
	getRepoId() string
	getRepoExternalObjPath() string
	getRepoExternalObjId() string
}
