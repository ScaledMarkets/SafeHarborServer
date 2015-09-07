/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */

package main

import (
)

type Client interface {
	CreateGroup() Group
	CreateUser() User
	CreateACLEntry() ACLEntry
	CreateACL() ACL
	CreateRealm() Realm
	CreateRepo() Repo
	CreateDockerfile() Dockerfile
	CreateDockerImage() DockerImage
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
}

type Repo interface {
	Resource
	//getName() string
	getFileDirectory() string
	getRealm() Realm
	getDockerfileIds() []string
	getDockerImageIds() []string
	addDockerfile(Dockerfile)
}

type Dockerfile interface {
	Resource
	getFilePath() string
	asDockerfileDesc() *DockerfileDesc
}

type DockerImage interface {
	Resource
	getDockerImageId()
}
