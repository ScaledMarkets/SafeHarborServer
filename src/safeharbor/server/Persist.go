/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model and Docker Image
 * Workflow Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 * The implementations should perform complete actions - i.e., maintain referential
 * integrity and satisfy all constraints and relationships.
 * Authorization (access control) is not part of the contract, however.
 */ 
 
package server

import (
	"time"
	//"os"
	
	"safeharbor/apitypes"
)

// Custom error type that indicates that data inconsistency was detected.
type DataError interface {
	asFailureDesc() *apitypes.FailureDesc
}

type DBClient interface {
	dbGetUserByUserId(string) User
	dbCreateGroup(string, string, string) (Group, error)
	dbCreateUser(string, string, string, string, string) (User, error)
	dbCreateACLEntry(resourceId string, partyId string, permissionMask []bool) (ACLEntry, error)
	dbCreateRealm(*apitypes.RealmInfo, string) (Realm, error)
	dbCreateRepo(string, string, string) (Repo, error)
	dbCreateDockerfile(string, string, string, string) (Dockerfile, error)
	dbCreateDockerImage(string, string, string) (DockerImage, error)
	dbCreateScanConfig(name, desc, repoId, providerName string, paramValueIds []string, successExpr, flagId string) (ScanConfig, error)
	dbCreateFlag(name, desc, repoId, successImagePath string) (Flag, error)
	dbCreateScanEvent(string, string, string, string) (ScanEvent, error)
	dbCreateDockerfileExecEvent(dockerfileId, imageId, userObjId string) (DockerfileExecEvent, error)
	dbDeactivateRealm(realmId string) error
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
	getFlag(string) (Flag, error)
	getScanEvent(string) (ScanEvent, error)
	getRealmsAdministeredByUser(string) ([]string, error)  // those realms for which user can edit the realm
	init()
	printDatabase()
}

type PersistObj interface {
	getId() string
	getDBClient() DBClient
	writeBack() error
}

/* A Party is a User or a Group. Parties act on Resources. */
type Party interface {
	PersistObj
	setActive(bool)
	isActive() bool
	getRealmId() string
	getRealm() (Realm, error)
	getName() string
	getCreationTime() time.Time
	getACLEntryIds() []string
	addACLEntry(ACLEntry) error
	getACLEntryForResourceId(string) (ACLEntry, error)
}

type Group interface {
	Party
	getDescription() string
	getUserObjIds() []string
	hasUserWithId(string) bool
	addUserId(string) error
	addUser(User)
	remUser(User) error
	asGroupDesc() *apitypes.GroupDesc
}

type User interface {
	Party
	getUserId() string
	setPassword(string) error
	hasGroupWithId(string) bool
	addGroupId(string) error
	getGroupIds() []string
	asUserDesc() *apitypes.UserDesc
	addLoginAttempt()
	getMostRecentLoginAttempts() []string // each in seconds, Unix time
	addEventId(string)
	getEventIds() []string
	
	//getEventIds() []string
}

type ACLEntry interface {
	PersistObj
	getResourceId() string
	getPartyId() string
	getPermissionMask() []bool
	setPermissionMask([]bool) error
	asPermissionDesc() *apitypes.PermissionDesc
}

type ACL interface {
	PersistObj
	getACLEntryIds() []string
	addACLEntry(ACLEntry) error
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
	setAccess(party Party, permissionMask []bool) (ACLEntry, error)
	addAccess(party Party, permissionMask []bool) (ACLEntry, error)
	removeAccess(Party) error
	removeAllAccess() error
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
	addGroup(Group) error
	addUser(User) error
	addRepo(Repo) error
	deleteRepo(Repo) error
	deleteGroup(Group) error
}

type Repo interface {
	Resource
	//getName() string
	getFileDirectory() string
	getRealmId() string
	getRealm() (Realm, error)
	getDockerfileIds() []string
	getDockerImageIds() []string
	getScanConfigIds() []string
	getFlagIds() []string
	addDockerfile(Dockerfile) error
	addDockerImage(DockerImage) error
	addScanConfig(ScanConfig) error
	addFlag(Flag) error
	getScanConfigByName(string) (ScanConfig, error)
	asRepoDesc() *apitypes.RepoDesc
	deleteResource(Resource) error
	
	//getDatasetIds() []string
	//getFlagIds() []string
}

type Dockerfile interface {
	Resource
	getExternalFilePath() string
	asDockerfileDesc() *apitypes.DockerfileDesc
	getRepo() (Repo, error)
	getDockerfileExecEventIds() []string
	addEventId(string) error
	replaceDockerfileFile(filepath, desc string) error
}

type Image interface {
	Resource
	getRepo() (Repo, error)
}

type DockerImage interface {
	Image
	//ImageCreationEvent
	getDockerImageTag() string  // Return same as getName().
	getFullName() (string, error)  // Return the fully qualified docker image path.
	getScanEventIds() []string // ordered from oldest to newest
	getMostRecentScanEventId() string
	asDockerImageDesc() *apitypes.DockerImageDesc
}

type ParameterValue interface {
	PersistObj
	getName() string
	//getTypeName() string
	getStringValue() string
	setStringValue(string) error
	getConfigId() string
	asParameterValueDesc() *apitypes.ParameterValueDesc
}

type ScanConfig interface {
	Resource
	getSuccessExpr() string
	getRepoId() string
	getProviderName() string
	getParameterValueIds() []string
	setParameterValue(string, string) (ParameterValue, error)
	setFlagId(string) error
	getFlagId() string
	addScanEventId(id string)
	getScanEventIds() []string
	asScanConfigDesc() *apitypes.ScanConfigDesc
}

type Flag interface {
	Resource
	getRepoId() string
	getSuccessImagePath() string
	getSuccessImageURL() string
	asFlagDesc() *apitypes.FlagDesc
}

type Event interface {
	PersistObj
	getWhen() time.Time
	getUserObjId() string
	asEventDesc() EventDesc
}

type ScanEvent interface {
	Event
	getScore() string
	getDockerImageId() string
	getScanConfigId() string
	getActualParameterValueIds() []string
	asScanEventDesc() *apitypes.ScanEventDesc
}

type ImageCreationEvent interface {
	Event
}

type DockerfileExecEvent interface {
	ImageCreationEvent
	getDockerfileId() string
	getDockerfileExternalObjId() string
}

type ImageUploadEvent interface {
	ImageCreationEvent
}
