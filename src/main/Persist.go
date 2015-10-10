/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */

package main

import (
)

type DBClient interface {
	dbGetUserByUserId(string) User
	dbCreateGroup(string, string) (Group, error)
	dbCreateUser(string, string, string, string, string) (User, error)
	dbCreateACLEntry(string, string, []bool) (ACLEntry, error)
	dbCreateRealm(*RealmInfo) (Realm, error)
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
	init()
}

type PersistObj interface {
	getId() string
}

/* A Party is a User or a Group. Parties act on Resources. */
type Party interface {
	PersistObj
	getName() string
	getACLEntryIds() []string
	addACLEntry(ACLEntry)
}

type Group interface {
	Party
	getPurpose() string
	getUserObjIds() []string
	hasUserWithId(string) bool
	addUserId(string) error
	addUser(User)
	asGroupDesc() *GroupDesc
}

type User interface {
	Party
	getRealmId() string
	getUserId() string
	asUserDesc() *UserDesc
}

type ACLEntry interface {
	PersistObj
	getResourceId() string
	getPartyId() string
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
}

type Dockerfile interface {
	Resource
	getFilePath() string
	asDockerfileDesc() *DockerfileDesc
}

type DockerImage interface {
	Resource
	getDockerImageId() string
	asDockerImageDesc() *DockerImageDesc
}
