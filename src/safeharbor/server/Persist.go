/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 */

package server

import (
	"time"
	"os"
	
	"safeharbor/apitypes"
)

type DBClient interface {
	dbGetUserByUserId(string) User
	dbCreateGroup(string, string, string) (Group, error)
	dbCreateUser(string, string, string, string, string) (User, error)
	dbCreateACLEntry(string, string, []bool) (ACLEntry, error)
	dbCreateRealm(*apitypes.RealmInfo, string) (Realm, error)
	dbCreateRepo(string, string, string) (Repo, error)
	dbCreateDockerfile(string, string, string, string) (Dockerfile, error)
	dbCreateDockerImage(string, string, string) (DockerImage, error)
	dbCreateScanConfig(string, string, string, string, []string, string, string) (ScanConfig, error)
	dbCreateScanEvent(string, string, string, time.Time, string, string) (ScanEvent, error)
	dbGetAllRealmIds() []string
	getPersistentObject(id string) PersistObj
	getResource(string) (Resource, error)
	getParty(string) (Party, error)
	getGroup(string) (Group, error)
	getUser(string) (User, error)
	getACLEntry(string) (ACLEntry, error)
	getRealm(string) (Realm, error)
	getRepo(string) (Repo, error)
	getDockerfile(string) (Dockerfile, error)
	getDockerImage(string) (DockerImage, error)
	getScanConfig(string) (ScanConfig, error)
	getParameterValue(string) (ParameterValue, error)
	getRealmsAdministeredByUser(string) ([]string, error)  // those realms for which user can edit the realm
	init()
	printDatabase()
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
	getACLEntryForResourceId(string) (ACLEntry, error)
}

type Group interface {
	Party
	getDescription() string
	getUserObjIds() []string
	hasUserWithId(string) bool
	addUserId(string) error
	addUser(User)
	asGroupDesc() *apitypes.GroupDesc
}

type User interface {
	Party
	getUserId() string
	hasGroupWithId(string) bool
	asUserDesc() *apitypes.UserDesc
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
	asPermissionDesc() *apitypes.PermissionDesc
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
	getDescription() string
	getACLEntryForPartyId(string) (ACLEntry, error)
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
	getUserByName(string) (User, error)
	getGroupByName(string) (Group, error)
	getRepoByName(string) (Repo, error)
	getUserObjIds() []string
	getRepoIds() []string
	addUserId(string) error
	getUserByUserId(string) (User, error)
	asRealmDesc() *apitypes.RealmDesc
	getGroupIds() []string
	addGroup(Group)
	addUser(User)
	addRepo(Repo)
}

type Repo interface {
	Resource
	//getName() string
	getFileDirectory() string
	getRealm() (Realm, error)
	getDockerfileIds() []string
	getDockerImageIds() []string
	addDockerfile(Dockerfile)
	addDockerImage(DockerImage)
	addScanConfig(ScanConfig)
	getScanConfigByName(string) (ScanConfig, error)
	asRepoDesc() *apitypes.RepoDesc
	
	//getDatasetIds() []string
	//getFlagIds() []string
}

type Dockerfile interface {
	Resource
	getExternalFilePath() string
	asDockerfileDesc() *apitypes.DockerfileDesc
	getRepo() (Repo, error)
	
	//getDockerfileExecEventIds() []string
}

type Image interface {
	Resource
	getRepo() (Repo, error)
}

type DockerImage interface {
	Image
	//ImageCreationEvent
	getDockerImageTag() string
	getFullName() string
	asDockerImageDesc() *apitypes.DockerImageDesc
	
	//getScanEventIds() []string
}



// For Image Workflow:

type ParameterValue interface {
	PersistObj
	getName() string
	getTypeName() string
	getStringValue() string
	getConfigId() string
	asParameterValueDesc() *apitypes.ParameterValueDesc
}

type ScanConfig interface {
	Resource
	getRepoId() string
	getExternalObjPath() string
	getCurrentExtObjId() string
	getAsTempFile(string) (*os.File, error)
	getProviderName() string
	getParameterValueIds() []string
	getSuccessGraphicImageURL() string
	getFailureGraphicImageURL() string
	asScanConfigDesc() *apitypes.ScanConfigDesc
}

type Event interface {
	PersistObj
	getWhen() time.Time
	getUserObjId() string
}

type ScanEvent interface {
	Event
	getScore() string
	getDockerImageId() string
	getScanConfigId() string
	asScanEventDesc() *apitypes.ScanEventDesc
	getScanConfigExternalObjId() string
}

type ImageCreationEvent interface {
	Event
}

type DockerfileExecEvent interface {
	ImageCreationEvent
	getDockerfileId() string
	getDockerfileExternalObjId() string
}

type UploadEvent interface {
	ImageCreationEvent
}

type Dataset interface {
	PersistObj
	getRepoId() string
	getRepoExternalObjPath() string
	//RepoExternalObjId string
	getScanEventIds() []string
}

type Flag interface {
	PersistObj
	getExpr() string
	getRepoId() string
	getRepoExternalObjPath() string
	getRepoExternalObjId() string
}