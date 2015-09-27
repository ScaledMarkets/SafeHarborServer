/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */

package main

import (
)

type DBClient interface {
	dbCreateGroup(string, string) (Group, error)
	dbCreateUser(string, string, string) (User, error)
	dbCreateACLEntry(string, string, []bool) (ACLEntry, error)
	dbCreateACL(string) (ACL, error)
	dbCreateRealm(*RealmInfo) (Realm, error)
	dbCreateRepo(string, string) (Repo, error)
	dbCreateDockerfile(string, string, string) (Dockerfile, error)
	dbCreateDockerImage(string, string) (DockerImage, error)
	dbGetAllRealmIds() []string
	getGroup(string) Group
	getUser(string) User
	getACLEntry(string) ACLEntry
	getACL(string) ACL
	getRealm(string) Realm
	getRepo(string) Repo
	getDockerfile(string) Dockerfile
	getDockerImage(string) DockerImage
	init()
}

type PersistObj interface {
	getId() string
}

type Group interface {
	PersistObj
	getName() string
	getACLEntryIds() []string
	getUserObjIds() []string
	hasUserWithId(string) bool
	addUser(string) error
	asGroupDesc() *GroupDesc
}

type User interface {
	PersistObj
	getRealmId() string
	getUserId() string
	getName() string
	getACLEntryIds() []string
	asUserDesc() *UserDesc
}

type ACLEntry interface {
	PersistObj
}

type ACL interface {
	PersistObj
}

type Resource interface {
	PersistObj
	getName() string
	getACL() ACL
}

type Realm interface {
	Resource
	//getName() string
	getFileDirectory() string
	hasUserWithId(string) bool
	hasGroupWithId(string) bool
	hasRepoWithId(string) bool
	getUserByName(string) User
	getGroupByName(string) Group
	getRepoByName(string) Repo
	getUserObjIds() []string
	getRepoIds() []string
	addUser(string) error
	getUserByUserId(string) User
	asRealmDesc() *RealmDesc
}

type Repo interface {
	Resource
	//getName() string
	getFileDirectory() string
	getRealm() Realm
	getDockerfileIds() []string
	getDockerImageIds() []string
	addDockerfile(Dockerfile)
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
