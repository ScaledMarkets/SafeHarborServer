/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */

package main

import (
)

type Client interface {
	CreateGroup() *Group
	CreateUser() *User
	CreateACLEntry() *ACLEntry
	CreateACL() *ACL
	CreateRealm() *Realm
	CreateRepo() *Repo
	CreateDockerfile() *Dockerfile
	CreateDockerImage() *DockerImage
}

type PersistObj interface {
	Delete()
	Clone() *PersistObj
}

type Group interface {
	PersistObj
}

type User interface {
	PersistObj
}

type ACLEntry interface {
	PersistObj
}

type ACL interface {
	PersistObj
}

type Realm interface {
	PersistObj
}

type Repo interface {
	PersistObj
}

type RepoItem interface {
	PersistObj
}

type Dockerfile interface {
	RepoItem
}

type DockerImage interface {
	RepoItem
}
