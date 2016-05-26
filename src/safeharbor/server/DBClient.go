/*******************************************************************************
 * These interfaces define the persistent object model for SafeHarbor, as also
 * defined in the Access Control Model and Docker Image Workflow Model - see
 * https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 * The implementations should perform complete actions - i.e., maintain referential
 * integrity and satisfy all constraints and relationships.
 * Authorization (access control) is not part of the contract, however.
 * Methods are assumed to be called in the context of a transaction - an
 * implementation is expected to provide the transaction context. The methods
 * 'commit' and 'abort' should be used to finalize the transaction.
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
	"safeharbor/providers"
)

/*******************************************************************************
 * Custom error type that indicates that data inconsistency was detected.
 * Should return an HTTP status of 409 to the client.
 */
type DataError interface {
	asFailureDesc() *apitypes.FailureDesc
}

/*******************************************************************************
 * 
 */
type TxnContext interface {
	commit() error
	abort() error
}

/*******************************************************************************
 * 
 */
type DBClient interface {
	
	getPersistence() *Persistence
	getServer() *Server
	
	getTransactionContext() TxnContext
	commit() error
	abort() error
	
	updateObject(obj PersistObj) error
		/** Update the object in the database. If object does not exist, create it.
			Merely delegates to <Persistence>.updateObject(TxnContext, PersistObj). */
	
	deleteObject(obj PersistObj) error
		/** Remove an object from the database. Error results if the object is not
			in the database. */
	
	getPersistentObject(id string) (PersistObj, error)
		/** Return the database object identified by the id, or error if not found. */
	
	// Superfluous - eliminate:
	writeBack(PersistObj) error
		/** Update the state of the object in the database. If the object exists,
			then update it. Note: this method is superfluous since updateObject is equivalent. */
	
	asJSON(PersistObj) string
		/** Externalize the object as a JSON-formatted string. */
	
	addRealm(newRealm Realm) error
	dbGetAllRealmIds() ([]string, error)
	addUser(user User) error

	dbGetUserByUserId(string) (User, error)
	dbCreateGroup(string, string, string) (Group, error)
	dbCreateUser(string, string, string, string, string) (User, error)
	dbCreateACLEntry(resourceId string, partyId string, permissionMask []bool) (ACLEntry, error)
	dbCreateRealm(*apitypes.RealmInfo, string) (Realm, error)
	dbCreateRepo(string, string, string) (Repo, error)
	dbCreateDockerfile(string, string, string, string) (Dockerfile, error)
	dbCreateDockerImage(string, string, string) (DockerImage, error)
	dbCreateDockerImageVersion(dockerImageObjId string, creationDate time.Time,
		buildOutput string, digest, signature []byte) (DockerImageVersion, error)
	dbCreateScanConfig(name, desc, repoId, providerName string, paramValueIds []string, successExpr, flagId string) (ScanConfig, error)
	dbCreateScanParameterValue(name, value, configId string) (ScanParameterValue, error)
	dbCreateFlag(name, desc, repoId, successImagePath string) (Flag, error)
	dbCreateScanEvent(scanConfigId, providerName string, paramNames, paramValues []string, imageId,
		userObjId, score string, result *providers.ScanResult) (ScanEvent, error)
	dbCreateDockerfileExecEvent(dockerfileId string, paramNames, paramValues []string,
		imageId, userObjId string) (DockerfileExecEvent, error)
	dbCreateDockerfileExecParameterValue(name, value string) (DockerfileExecParameterValue, error)
	dbDeactivateRealm(realmId string) error
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
	getScanParameterValue(string) (ScanParameterValue, error)
	getDockerfileExecParameterValue(string) (DockerfileExecParameterValue, error)
	getFlag(string) (Flag, error)
	getEvent(string) (Event, error)
	getScanEvent(string) (ScanEvent, error)
	getRealmsAdministeredByUser(string) ([]string, error)  // those realms for which user can edit the realm
		
	// From Party
	setActive(Party, bool) error
	addACLEntryForParty(Party, ACLEntry) error
	deleteACLEntryForParty(party Party, entry ACLEntry) error
	
	// From ACL
	addACLEntry(ACL, ACLEntry) error
	
	// From Resource
	setName(Resource, string) error
	setDescription(Resource, string) error
	setAccess(resource Resource, party Party, permissionMask []bool) (ACLEntry, error)
	addAccess(resource Resource, party Party, permissionMask []bool) (ACLEntry, error)
	deleteAccess(Resource, Party) error
	deleteAllAccessToResource(Resource) error
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
	getPersistence() *Persistence
	writeBack(DBClient) error
	asJSON() string
}

/* A Party is a User or a Group. Parties act on Resources. */
type Party interface {  // abstract
	PersistObj
	setActive(bool)
	isActive() bool
	getRealmId() string
	getRealm(DBClient) (Realm, error)
	getName() string
	getCreationTime() time.Time
	getACLEntryIds() []string
	addACLEntry(ACLEntry)  // does not write to db
	deleteACLEntry(dbClient DBClient, entry ACLEntry) error
	getACLEntryForResourceId(DBClient, string) (ACLEntry, error)
}

type ACL interface {  // abstract
	PersistObj
	addACLEntry(ACLEntry)  // does not write to db
	getACLEntryIds() []string
	setACLEntryIds([]string)  // does not write to db
}

/* A Resource is something that a party can act upon. */
type Resource interface {  // abstract
	ACL
	getName() string
	//setName(string) error
	setNameDeferredUpdate(string)
	getCreationTime() time.Time
	getDescription() string
	//setDescription(string) error
	setDescriptionDeferredUpdate(string)
	getACLEntryForPartyId(DBClient, string) (ACLEntry, error)
	getParentId() string
	isRealm() bool
	isRepo() bool
	isDockerfile() bool
	isDockerImage() bool
	isScanConfig() bool
	isFlag() bool
	//setAccess(party Party, permissionMask []bool) (ACLEntry, error)
	//addAccess(party Party, permissionMask []bool) (ACLEntry, error)
	//deleteAccess(Party) error
	//deleteAllAccess() error
	
	removeACLEntryIdAt(index int)  // does not write to db
	clearAllACLEntryIds()  // does not write to db
	deleteAllChildResources(DBClient) error
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

type Group interface {
	Party
	getDescription() string
	getUserObjIds() []string
	hasUserWithId(DBClient, string) bool
	addUserId(DBClient, string) error
	addUser(DBClient, User) error
	removeUser(DBClient, User) error
	asGroupDesc() *apitypes.GroupDesc
}

type User interface {
	Party
	getUserId() string
	setPassword(DBClient, string) error
	validatePassword(dbClient DBClient, pswd string) bool
	hasGroupWithId(DBClient, string) bool
	addGroupId(DBClient, string) error
	getGroupIds() []string
	addLoginAttempt(DBClient)
	getMostRecentLoginAttempts() []string // each in seconds, Unix time
	addEventId(DBClient, string)
	getEventIds() []string
	deleteEvent(DBClient, Event) error
	asUserDesc(DBClient) *apitypes.UserDesc
}

type ACLEntry interface {
	PersistObj
	getResourceId() string
	getPartyId() string
	getParty(DBClient) (Party, error)
	getPermissionMask() []bool
	setPermissionMask(DBClient, []bool) error
	asPermissionDesc() *apitypes.PermissionDesc
}

type Realm interface {
	Resource
	getAdminUserId() string
	getFileDirectory() string
	hasUserWithId(DBClient, string) bool
	hasGroupWithId(DBClient, string) bool
	hasRepoWithId(DBClient, string) bool
	getUserByName(DBClient, string) (User, error)
	getGroupByName(DBClient, string) (Group, error)
	getRepoByName(DBClient, string) (Repo, error)
	getUserObjIds() []string
	getRepoIds() []string
	addUserId(DBClient, string) error
	removeUserId(DBClient, string) (User, error)
	deleteUserId(DBClient, string) error
	getUserByUserId(DBClient, string) (User, error)
	getGroupIds() []string
	addGroup(DBClient, Group) error
	addUser(DBClient, User) error
	addRepo(DBClient, Repo) error
	deleteGroup(DBClient, Group) error
	deleteRepo(DBClient, Repo) error
	asRealmDesc() *apitypes.RealmDesc
}

type Repo interface {
	Resource
	getFileDirectory() string
	getRealmId() string
	getRealm(DBClient) (Realm, error)
	getDockerfileIds() []string
	getDockerImageIds() []string
	getScanConfigIds() []string
	getFlagIds() []string
	addDockerfile(DBClient, Dockerfile) error
	addDockerImage(DBClient, DockerImage) error
	addScanConfig(DBClient, ScanConfig) error
	deleteScanConfig(DBClient, ScanConfig) error
	addFlag(DBClient, Flag) error
	deleteFlag(DBClient, Flag) error
	deleteDockerfile(DBClient, Dockerfile) error
	deleteDockerImage(DBClient, DockerImage) error
	getDockerfileByName(DBClient, string) (Dockerfile, error)
	getFlagByName(DBClient, string) (Flag, error)
	getDockerImageByName(DBClient, string) (DockerImage, error)
	getScanConfigByName(DBClient, string) (ScanConfig, error)
	asRepoDesc() *apitypes.RepoDesc
	asRepoPlusDockerfileDesc(dockerfileId string) *apitypes.RepoPlusDockerfileDesc
}

type Dockerfile interface {
	Resource
	getExternalFilePath() string
	getRepoId() string
	getRepo(DBClient) (Repo, error)
	getDockerfileExecEventIds() []string
	addEventId(DBClient, string) error
	replaceDockerfileFile(filepath, desc string) error
	getParameterValueIds() string
	asDockerfileDesc() *apitypes.DockerfileDesc
}

type Image interface {  // abstract
	Resource
	getRepoId() string
	getRepo(DBClient) (Repo, error)
	getImageVersionIds() []string
	getUniqueVersion() (string, error)
	addVersionId(DBClient, string) error
	getMostRecentVersionId() (string, error)
}

type ImageVersion interface {  // abstract
	PersistObj
	getVersion() string
	getImageObjId() string
	getCreationDate() time.Time
	getImageCreationEventId() string
	setImageCreationEventId(string)
	getFullName(dbClient DBClient) (string, error)
	getFullNameParts(dbClient DBClient) (string, string, string, error)
}

type DockerImage interface {
	Image
	getDockerImageTag() string  // Return same as getName().
	getFullName(DBClient) (string, error)  // Return the fully qualified docker image path.
	getFullNameParts(DBClient) (namespace, name, tag string, err error)
	//addScanEventId(dbClient DBClient, id string)
	//getScanEventIds() []string // ordered from oldest to newest
	//getMostRecentScanEventId() string
	//getImageCreationEventId() string
	//setImageCreationEventId(string)
	asDockerImageDesc() *apitypes.DockerImageDesc
	//getSignature() []byte
	//computeSignature() ([]byte, error)
	//getOutputFromBuild() string
}

type DockerImageVersion interface {
	ImageVersion
	getDockerImageTag() string
	addScanEventId(dbClient DBClient, id string) error
	getScanEventIds() []string // ordered from oldest to newest
	getMostRecentScanEventId() string
	getDigest() []byte
    getSignature() []byte
    getDockerBuildOutput() string
}

type ParameterValue interface {
	PersistObj
	getName() string
	getStringValue() string
	setStringValue(DBClient, string) error
	parameterValueFieldsAsJSON() string
	//asParameterValueDesc() *apitypes.ParameterValueDesc
}

type ScanConfig interface {
	Resource
	getSuccessExpr() string
	setSuccessExpression(DBClient, string) error
	setSuccessExpressionDeferredUpdate(string)
	getRepoId() string
	getProviderName() string
	setProviderName(DBClient, string) error
	setProviderNameDeferredUpdate(string)
	getParameterValueIds() []string
	setParameterValue(DBClient, string, string) (ParameterValue, error)
	setParameterValueDeferredUpdate(DBClient, string, string) (ParameterValue, error)
	deleteParameterValue(dbClient DBClient, name string) error
	deleteAllParameterValues(DBClient) error
	setFlagId(DBClient, string) error
	getFlagId() string
	addParameterValueId(dbClient DBClient, id string)
	addScanEventId(dbClient DBClient, id string)
	getScanEventIds() []string
	deleteScanEventId(DBClient, string) error
	asScanConfigDesc(DBClient) *apitypes.ScanConfigDesc
}

type ScanParameterValue interface {
	ParameterValue
	getConfigId() string
	asScanParameterValueDesc(DBClient) *apitypes.ScanParameterValueDesc
	scanParameterValueFieldsAsJSON() string
}

type DockerfileExecParameterValue interface {
	ParameterValue
	getDockerfileId() string
	asDockerfileExecParameterValueDesc(DBClient) *apitypes.DockerfileExecParameterValueDesc
	dockerfileExecParameterValueFieldsAsJSON() string
}

type Flag interface {
	Resource
	getRepoId() string
	getSuccessImagePath() string
	getSuccessImageURL() string
	addScanConfigRef(DBClient, string) error
	removeScanConfigRef(DBClient, string) error
	usedByScanConfigIds() []string
	asFlagDesc() *apitypes.FlagDesc
}

type Event interface {  // abstract
	PersistObj
	getWhen() time.Time
	getUserObjId() string
	asEventDesc(DBClient) apitypes.EventDesc
}

type ScanEvent interface {
	Event
	getScore() string
	//getDockerImageId() string  // may be empty (if Dockerfile has been deleted).
	getDockerImageVersionId() string  // may be empty (if Dockerfile has been deleted).
	getScanConfigId() string  // may be empty (if ScanConfig has been deleted).
	getActualParameterValueIds() []string
	deleteAllParameterValues(DBClient) error
	asScanEventDesc(DBClient) *apitypes.ScanEventDesc
	nullifyDockerImage(DBClient) error
	nullifyScanConfig(DBClient) error
}

type ImageCreationEvent interface {  // abstract
	Event
	nullifyImageVersion(DBClient) error
	getImageVersionId(DBClient) string
	//nullifyDockerImage(DBClient) error
}

type DockerfileExecEvent interface {
	ImageCreationEvent
	getDockerfileId() string  // may be empty - if the Dockerfile has been deleted.
	getActualParameterValueIds() []string
	deleteAllParameterValues(DBClient) error
	getDockerfileContent() string
	//getDockerfileExternalObjId() string  // may be empty.
	asDockerfileExecEventDesc(DBClient) *apitypes.DockerfileExecEventDesc
	
	/** Nullify all references to the dockerfile or its external representation. */
	nullifyDockerfile(DBClient) error
}

type ImageUploadEvent interface {
	ImageCreationEvent
}
