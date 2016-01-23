/*******************************************************************************
 * This file defines the methods that a persistence implementation should have for
 * creating the object types defined in the Access Control Model and Docker Image
 * Workflow Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 * The implementations should perform complete actions - i.e., maintain referential
 * integrity and satisfy all constraints and relationships.
 * Authorization (access control) is not part of the contract, however.
 *
 * The Group, Permission, Repo, Dockerfile, Image, User, and Realm have
 * asGroupDesc, asPermissionDesc, asRepoDesc, asDockerfileDesc, asImageDesc,
 * asUserDesc, and asRealmDesc methods, respectively - these methods construct
 * instances of apitypes.GroupDesc, apitypes.PermissionDesc, apitypes.RepoDesc, apitypes.DockerfileDesc, ImageDesc,
 * and so on. These methods are a convenient way of constructing the return values
 * that are needed by the handler methods defined in the API (slides titled
 * "SafeHarbor REST API" of the desgin), which are implemented by the functions
 * in Handlers.go.
 */ 
 
package server

import (
	"time"
	//"os"
	
	"safeharbor/apitypes"
)

// Custom error type that indicates that data inconsistency was detected.
// Should return an HTTP status of 409 to the client.
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
	dbCreateDockerImage(string, string, string, []byte, string) (DockerImage, error)
	dbCreateScanConfig(name, desc, repoId, providerName string, paramValueIds []string, successExpr, flagId string) (ScanConfig, error)
	dbCreateFlag(name, desc, repoId, successImagePath string) (Flag, error)
	dbCreateScanEvent(string, string, string, string) (ScanEvent, error)
	dbCreateDockerfileExecEvent(dockerfileId, imageId, userObjId string) (DockerfileExecEvent, error)
	dbDeactivateRealm(realmId string) error
	dbGetAllRealmIds() ([]string, error)
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
	getEvent(string) (Event, error)
	getScanEvent(string) (ScanEvent, error)
	getRealmsAdministeredByUser(string) ([]string, error)  // those realms for which user can edit the realm
	init() error
	resetPersistentState() error
	printDatabase()
	
	// From PersistObj
	writeBack(PersistObj) error
	asJSON(PersistObj) string
	
	// From Resource
	isRealm(Resource) bool
	isRepo(Resource) bool
	isDockerfile(Resource) bool
	isDockerImage(Resource) bool
	isScanConfig(Resource) bool
	isFlag(Resource) bool

	// From Event
	asEventDesc(Event) apitypes.EventDesc
}

type PersistObj interface {  // abstract
	getId() string
	getDBClient() DBClient
	writeBack() error
	asJSON() string
}

/* A Party is a User or a Group. Parties act on Resources. */
type Party interface {  // abstract
	PersistObj
	setActive(bool) error
	isActive() bool
	getRealmId() string
	getRealm() (Realm, error)
	getName() string
	getCreationTime() time.Time
	getACLEntryIds() []string
	addACLEntry(ACLEntry) error
	deleteACLEntry(entry ACLEntry) error
	getACLEntryForResourceId(string) (ACLEntry, error)
}

type Group interface {
	Party
	getDescription() string
	getUserObjIds() []string
	hasUserWithId(string) bool
	addUserId(string) error
	addUser(User) error
	removeUser(User) error
	asGroupDesc() *apitypes.GroupDesc
}

type User interface {
	Party
	getUserId() string
	setPassword(string) error
	validatePassword(pswd string) bool
	hasGroupWithId(string) bool
	addGroupId(string) error
	getGroupIds() []string
	addLoginAttempt()
	getMostRecentLoginAttempts() []string // each in seconds, Unix time
	addEventId(string)
	getEventIds() []string
	deleteEvent(Event) error
	asUserDesc() *apitypes.UserDesc
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
type Resource interface {  // abstract
	ACL
	getName() string
	setName(string) error
	setNameDeferredUpdate(string)
	getCreationTime() time.Time
	getDescription() string
	setDescription(string) error
	setDescriptionDeferredUpdate(string)
	getACLEntryForPartyId(string) (ACLEntry, error)
	getParentId() string
	isRealm() bool
	isRepo() bool
	isDockerfile() bool
	isDockerImage() bool
	isScanConfig() bool
	isFlag() bool
	setAccess(party Party, permissionMask []bool) (ACLEntry, error)
	addAccess(party Party, permissionMask []bool) (ACLEntry, error)
	deleteAccess(Party) error
	deleteAllAccess() error
}

type ResourceType int

const (
	ARealm ResourceType = iota
	ARepo
	ADockerfile
	ADockerImage
	AScanConfig
	AFlag
)

type Realm interface {
	Resource
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
	removeUserId(string) (User, error)
	deleteUserId(string) error
	getUserByUserId(string) (User, error)
	getGroupIds() []string
	addGroup(Group) error
	addUser(User) error
	addRepo(Repo) error
	deleteGroup(Group) error
	asRealmDesc() *apitypes.RealmDesc
}

type Repo interface {
	Resource
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
	deleteScanConfig(ScanConfig) error
	addFlag(Flag) error
	deleteFlag(Flag) error
	deleteDockerImage(DockerImage) error
	getScanConfigByName(string) (ScanConfig, error)
	asRepoDesc() *apitypes.RepoDesc
}

type Dockerfile interface {
	Resource
	getExternalFilePath() string
	getRepoId() string
	getRepo() (Repo, error)
	getDockerfileExecEventIds() []string
	addEventId(string) error
	replaceDockerfileFile(filepath, desc string) error
	asDockerfileDesc() *apitypes.DockerfileDesc
}

type Image interface {  // abstract
	Resource
	getRepoId() string
	getRepo() (Repo, error)
}

type DockerImage interface {
	Image
	getDockerImageTag() string  // Return same as getName().
	getFullName() (string, error)  // Return the fully qualified docker image path.
	getScanEventIds() []string // ordered from oldest to newest
	getMostRecentScanEventId() string
	asDockerImageDesc() *apitypes.DockerImageDesc
	getSignature() []byte
	//computeSignature() ([]byte, error)
	getOutputFromBuild() string
	addScanEventId(id string)
}

type ParameterValue interface {
	PersistObj
	getName() string
	getStringValue() string
	setStringValue(string) error
	getConfigId() string
	asParameterValueDesc() *apitypes.ParameterValueDesc
}

type ScanConfig interface {
	Resource
	getSuccessExpr() string
	setSuccessExpression(string) error
	setSuccessExpressionDeferredUpdate(string)
	getRepoId() string
	getProviderName() string
	setProviderName(string) error
	setProviderNameDeferredUpdate(string)
	getParameterValueIds() []string
	setParameterValue(string, string) (ParameterValue, error)
	setParameterValueDeferredUpdate(string, string) (ParameterValue, error)
	deleteParameterValue(name string) error
	deleteAllParameterValues() error
	setFlagId(string) error
	getFlagId() string
	addScanEventId(id string)
	getScanEventIds() []string
	deleteScanEventId(string) error
	asScanConfigDesc() *apitypes.ScanConfigDesc


}

type Flag interface {
	Resource
	getRepoId() string
	getSuccessImagePath() string
	getSuccessImageURL() string
	addScanConfigRef(string) error
	removeScanConfigRef(string) error
	usedByScanConfigIds() []string
	asFlagDesc() *apitypes.FlagDesc
}

type Event interface {  // abstract
	PersistObj
	getWhen() time.Time
	getUserObjId() string
	asEventDesc() apitypes.EventDesc
}

type ScanEvent interface {
	Event
	getScore() string
	getDockerImageId() string
	getScanConfigId() string
	getActualParameterValueIds() []string
	deleteAllParameterValues() error
	asScanEventDesc() *apitypes.ScanEventDesc
}

type ImageCreationEvent interface {  // abstract
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
