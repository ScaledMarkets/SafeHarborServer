/*******************************************************************************
 * In-memory implementation of the methods defined in DBClient.go.
 *
 * These methods do not perform any authorization - that is done by the handlers.
 * 
 * Each type has a New<type> function. The New function merely constructs an instance
 * of the type - it does not link the type in any relationships.
 * 
 * For each concrete (non-abstract) type that has a writeBack() method, the New<type>
 * function writes the new instance to persistent storage.
 */

package server

import (
	"fmt"
	//"errors"
	"net/http"
	"reflect"
	"os"
	"io/ioutil"
	"time"
	"strings"
	"runtime/debug"	
	//"encoding/hex"
	
	//"goredis"
	
	"safeharbor/apitypes"
	"safeharbor/docker"
	"safeharbor/rest"
	"safeharbor/utils"
	"safeharbor/providers"
)

const (
	TransactionTimeoutSeconds int = 2
)

/*******************************************************************************
 * Implements DataError.
 */
type PersistDataError struct {
	utils.ServerError
}

var _ DataError = &PersistDataError{}

func NewPersistDataError(msg string) *PersistDataError {
	return &PersistDataError{
		ServerError: *utils.ConstructServerError(msg),
	}
}

func (dataErr *PersistDataError) asFailureDesc() *apitypes.FailureDesc {
	return apitypes.NewFailureDesc(http.StatusInternalServerError, dataErr.Error())
}

/*******************************************************************************
 * Implements DBClient.
 */
type InMemClient struct {
	Persistence *Persistence
	Server *Server
	txn TxnContext  // database transaction context
	
	// Private (transaction-scope) object cache.
	objectsCache map[string]PersistObj  // maps object id to PersistObj
	usersCache map[string]User  // maps user id to User obj
	realmMapCache map[string]Realm  // maps realm name to Realm obj
}

func NewInMemClient(server *Server) (*InMemClient, error) {
	
	var txn TxnContext
	var err error
	txn, err = server.persistence.NewTxnContext()
	if err != nil { return nil, err }
	
	// Create and return a new InMemClient.
	var client = &InMemClient{
		Persistence: server.persistence,
		Server: server,
		txn: txn,
	}
	
	client.resetTransactionCache()
	
	return client, nil
}

func (client *InMemClient) getPersistence() *Persistence { return client.Persistence }

func (client *InMemClient) getServer() *Server { return client.Server }

func (client *InMemClient) getTransactionContext() TxnContext { return client.txn }

func (client *InMemClient) resetTransactionCache() {
	client.objectsCache = make(map[string]PersistObj)
	client.usersCache = make(map[string]User)
	client.realmMapCache = make(map[string]Realm)
}

// Commit the database transaction - after calling this, methods on this instance
// of InMemClient can no longer be called.
func (client *InMemClient) commit() error {
	client.resetTransactionCache()
	if client.Persistence.InMemoryOnly {
		return nil
	} else {
		return client.txn.commit()
	}
}

// Abort the database transaction - after calling this, methods on this instance
// of InMemClient can no longer be called.
func (client *InMemClient) abort() error {
	client.resetTransactionCache()
	if client.Persistence.InMemoryOnly {
		return nil
	} else {
		return client.txn.abort()
	}
}

func (client *InMemClient) getPersistentObject(id string) (PersistObj, error) {
	
	if id == "" { return nil, utils.ConstructServerError("Object Id is empty") }
	
	var cachedObj PersistObj = client.objectsCache[id]
	if cachedObj != nil { return cachedObj, nil }

	var obj PersistObj
	var err error
	obj, err = client.Persistence.getObject(client.txn, client, id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Object with Id " + id + " not found") }
	client.objectsCache[id] = obj
	return obj, nil
}

func (client *InMemClient) updateObject(obj PersistObj) error {
	
	// Checks that can be removed after testing.
	if obj == nil { return utils.ConstructServerError("Object is nil") }
	if client.objectIsAbstract(obj) { return utils.ConstructServerError(
		"Abstract object: " + reflect.TypeOf(obj).String())
	}
	
	// Check cache for object.
	client.objectsCache[obj.getId()] = obj  // Add object to cache
	
	// Update database.
	return client.Persistence.updateObject(client.txn, obj)
}

func (client *InMemClient) objectIsAbstract(obj PersistObj) bool {
	var t = reflect.TypeOf(obj).String()
	switch t {
		case "*server.InMemPersistObj",
			"*server.InMemACL",
			"*server.InMemResource",
			"*server.InMemParty",
			"*server.InMemImage",
			"*server.InMemImageVersion",
			"*server.InMemParameterValue",
			"*server.InMemEvent",
			"*server.InMemImageCreationEvent": return true
		default: return false
	}
}

// Superfluous - eliminate:
func (client *InMemClient) writeBack(obj PersistObj) error {
	//client.objectsCache[obj.getId()] = obj  // update cache
	return client.updateObject(obj)
	//return obj.writeBack(client)  // update database
}

func (client *InMemClient) deleteObject(obj PersistObj) error {
	var cachedObj PersistObj = client.objectsCache[obj.getId()]
	if cachedObj != nil { client.objectsCache[obj.getId()] = nil }  // remove from cache
	
	return client.Persistence.deleteObject(client.txn, obj)
}

func (client *InMemClient) asJSON(obj PersistObj) string {
	return obj.asJSON()
}

func (client *InMemClient) dbGetAllRealmIds() ([]string, error) {
	var realmIdMap map[string]string
	var err error
	realmIdMap, err = client.Persistence.dbGetAllRealmIds(client.txn)
	if err != nil { return nil, err }
	var realmIds = make([]string, 0)
	for _, realmId := range realmIdMap {
		if client.realmMapCache[realmId] == nil { // don't add cached realms yet
			realmIds = append(realmIds, realmId)
		}
	}
	
	// Add any cached realms to the list.
	for _, realm := range client.realmMapCache {
		realmIds = append(realmIds, realm.getId())
	}
	
	return realmIds, nil
}

func (client *InMemClient) addRealm(newRealm Realm) error {
	
	// Check if realm with same name already exists.
	var rid string
	var err error
	rid, err = client.Persistence.GetRealmObjIdByRealmName(client.txn, newRealm.getName())
	if err != nil { return err }
	if rid != "" { return utils.ConstructUserError(
		"A realm with name " + newRealm.getName() + " already exists")
	}
	
	var cachedRealm Realm = client.realmMapCache[newRealm.getId()]
	if cachedRealm != nil { return utils.ConstructUserError("Realm already exists") }
	client.realmMapCache[newRealm.getId()] = newRealm  // Add the realm to the cache.
	client.objectsCache[newRealm.getId()] = newRealm  // Add object to cache
	return client.Persistence.addRealm(client.txn, newRealm)  // Add to database.
}

func (client *InMemClient) addUser(user User) error {
	
	var cachedUser User = client.usersCache[user.getId()]
	if cachedUser != nil { return utils.ConstructUserError("User already exists") }
	client.usersCache[user.getId()] = user  // Add the user to the cache.
	client.objectsCache[user.getId()] = user  // Add object to cache
	
	return client.Persistence.addUser(client.txn, user)
}

func (client *InMemClient) dbGetUserByUserId(userId string) (User, error) {
	
	var cachedUser User = client.usersCache[userId]
	if cachedUser != nil { return cachedUser, nil }
	
	var userObjId string
	var err error
	userObjId, err = client.Persistence.GetUserObjIdByUserId(client.txn, userId)
	if err != nil { return nil, err }
	if userObjId == "" { return nil, nil }
	var user User
	user, err = client.getUser(userObjId)
	if err != nil { return nil, err }
	
	// Add user to cache.
	client.usersCache[userId] = user
	client.objectsCache[userId] = user
	
	return user, nil
}

/*******************************************************************************
 * Base type that is included in each data type as an anonymous field.
 */
type InMemPersistObj struct {  // abstract
	Persistence *Persistence
	Id string
}

var _ PersistObj = &InMemPersistObj{}

func (client *InMemClient) NewInMemPersistObj() (*InMemPersistObj, error) {
	
	var id string
	var err error
	id, err = client.Persistence.createUniqueDbObjectId()
	if err != nil { return nil, err }
	var obj *InMemPersistObj = &InMemPersistObj{
		Persistence: client.Persistence,
		Id: id,
	}
	return obj, nil
}

func (persObj *InMemPersistObj) getId() string {
	return persObj.Id
}

func (persObj *InMemPersistObj) getPersistence() *Persistence {
	return persObj.Persistence
}

func (persObj *InMemPersistObj) writeBack(dbClient DBClient) error {
	panic("Abstract method should not be called")
}

func (persObj *InMemPersistObj) persistObjFieldsAsJSON() string {
	return fmt.Sprintf("\"Id\": \"%s\"", persObj.Id)
}

func (persObj *InMemPersistObj) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstitutePersistObj(id string) (*InMemPersistObj, error) {
	return &InMemPersistObj{
		Persistence: client.Persistence,
		Id: id,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemACL struct {  // abstract
	InMemPersistObj
	ACLEntryIds []string
}

var _ ACL = &InMemACL{}

func (client *InMemClient) NewInMemACL() (*InMemACL, error) {
	var persistobj *InMemPersistObj
	var err error
	persistobj, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	var acl *InMemACL = &InMemACL{
		InMemPersistObj: *persistobj,
		ACLEntryIds: make([]string, 0),
	}
	return acl, nil
}

func (acl *InMemACL) getACLEntryIds() []string {
	return acl.ACLEntryIds
}

func (acl *InMemACL) setACLEntryIds(ids []string) {
	acl.ACLEntryIds = ids
}

func (client *InMemClient) addACLEntry(acl ACL, entry ACLEntry) error {
	acl.addACLEntry(entry)
	return client.writeBack(acl)
}

func (acl *InMemACL) addACLEntry(entry ACLEntry) {
	acl.ACLEntryIds = append(acl.ACLEntryIds, entry.getId())
}

func (acl *InMemACL) writeBack(dbClient DBClient) error {
	panic("Abstract method should not be called")
}

func (acl *InMemACL) aclFieldsAsJSON() string {
	var json = acl.persistObjFieldsAsJSON()
	json = json + ", \"ACLEntryIds\": ["
	for i, entryId := range acl.ACLEntryIds {
		if i != 0 { json = json + ", " }
		json = json + fmt.Sprintf("\"%s\"", entryId)
	}
	json = json + "]"
	return json
}

func (acl *InMemACL) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstituteACL(id string, aclEntryIds []string) (*InMemACL, error) {
	var persistObj *InMemPersistObj
	var err error
	persistObj, err = client.ReconstitutePersistObj(id)
	if err != nil { return nil, err }
	var acl = &InMemACL{
		InMemPersistObj: *persistObj,
		ACLEntryIds: aclEntryIds,
	}
	return acl, nil
}

/*******************************************************************************
 * 
 */
type InMemResource struct {  // abstract
	InMemACL
	Name string
	Description string
	ParentId string
	CreationTime time.Time
}

var _ Resource = &InMemResource{}

func (client *InMemClient) NewInMemResource(name string, desc string,
	parentId string) (*InMemResource, error) {
	var acl *InMemACL
	var err error
	acl, err = client.NewInMemACL()
	if err != nil { return nil, err }
	return &InMemResource{
		InMemACL: *acl,
		Name: name,
		Description: desc,
		ParentId: parentId,
		CreationTime: time.Now(),
	}, nil
}

func (client *InMemClient) setAccess(resource Resource, party Party, mask []bool) (ACLEntry, error) {
	var aclEntry ACLEntry
	var err error
	aclEntry, err = party.getACLEntryForResourceId(client, resource.getId())
	if err != nil { return nil, err }
	if aclEntry == nil {
		aclEntry, err = client.dbCreateACLEntry(resource.getId(), party.getId(), mask)
		if err != nil { return nil, err }
	} else {
		err = aclEntry.setPermissionMask(client, mask)
		if err != nil { return nil, err }
	}
	
	return aclEntry, nil
}

func (client *InMemClient) addAccess(resource Resource, party Party, mask []bool) (ACLEntry, error) {

	var aclEntry ACLEntry
	var err error
	aclEntry, err = party.getACLEntryForResourceId(client, resource.getId())
	if err != nil { return nil, err }
	if aclEntry == nil {
		aclEntry, err = client.dbCreateACLEntry(resource.getId(), party.getId(), mask)
		if err != nil { return nil, err }
	} else {
		// Add the new mask.
		var curmask []bool = aclEntry.getPermissionMask()
		for index, _ := range curmask {
			curmask[index] = curmask[index] || mask[index]
		}
		err = aclEntry.setPermissionMask(client, curmask)
		if err != nil { return nil, err }
		//if err = client.writeBack(aclEntry); err != nil { return nil, err }
	}

	return aclEntry, nil
}

func (client *InMemClient) deleteAccess(resource Resource, party Party) error {
	
	var aclEntriesCopy []string = make([]string, len(resource.getACLEntryIds()))
	copy(aclEntriesCopy, resource.getACLEntryIds())
	for index, entryId := range aclEntriesCopy {
		var aclEntry ACLEntry
		var err error
		aclEntry, err = client.getACLEntry(entryId)
		if err != nil { return err }
		
		if aclEntry.getPartyId() == party.getId() {
			// ACL entry's resource id and party id both match.
			if aclEntry.getResourceId() != resource.getId() {
				return utils.ConstructServerError("Internal error: an ACL entry's resource Id does not match the resource whose list it is a member of")
			}
			
			// Remove from party's list.
			err = client.deleteACLEntryForParty(party, aclEntry)
			if err != nil { return err }
			
			// Remove the ACL entry id from the resource's ACL entry list.
			resource.removeACLEntryIdAt(index)
			
			// Remove from database.
			err = client.Persistence.deleteObject(client.txn, aclEntry)
			if err != nil { return err }
		}
	}
	
	return client.writeBack(resource)
}

func (resource *InMemResource) removeACLEntryIdAt(index int) {
	resource.ACLEntryIds = apitypes.RemoveAt(index, resource.ACLEntryIds)
}

func (resource *InMemResource) printACLs(dbClient DBClient, party Party) {
	var curresourceId string = resource.getId()
	var curresource Resource = resource
	for {
		fmt.Println("\tACL entries for resource " + curresource.getName() + 
			" (" + curresource.getId() + ") are:")
		for _, entryId := range curresource.getACLEntryIds() {
			var aclEntry ACLEntry
			var err error
			aclEntry, err = dbClient.getACLEntry(entryId)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			var rscId string = aclEntry.getResourceId()
			var rsc Resource
			rsc, err = dbClient.getResource(rscId)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			var ptyId string = aclEntry.getPartyId()
			var pty Party
			pty, err = dbClient.getParty(ptyId)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			fmt.Println("\t\tEntry Id " + entryId + ": party: " + pty.getName() + " (" + ptyId + "), resource: " +
				rsc.getName() + " (" + rsc.getId() + ")")
		}
		curresourceId = curresource.getParentId()
		if curresourceId == "" {
			fmt.Println(fmt.Sprintf("\tResource %s (%s) has not parentId",
				curresource.getName(), curresource.getId()))
			break
		}
		var err error
		curresource, err = dbClient.getResource(curresourceId)
		if err != nil {
			fmt.Println(err.Error())
			break
		}
	}
	fmt.Println("\tACL entries for party " + party.getName() + 
		" (" + party.getId() + ") are:")
	for _, entryId := range party.getACLEntryIds() {
		var aclEntry ACLEntry
		var err error
		aclEntry, err = dbClient.getACLEntry(entryId)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		var rscId string = aclEntry.getResourceId()
		var rsc Resource
		rsc, err = dbClient.getResource(rscId)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		var partyId string = aclEntry.getPartyId()
		var pty Party
		pty, err = dbClient.getParty(partyId)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		fmt.Println("\t\tEntry Id " + entryId + ": party: " + pty.getName() + " (" + partyId + "), resource: " +
			rsc.getName() + " (" + rsc.getId() + ")")
	}
}

func (client *InMemClient) deleteAllAccessToResource(resource Resource) error {
	
	var aclEntriesCopy []string = make([]string, len(resource.getACLEntryIds()))
	copy(aclEntriesCopy, resource.getACLEntryIds())
	for _, id := range aclEntriesCopy {
		var aclEntry ACLEntry
		var err error
		aclEntry, err = client.getACLEntry(id)
		if err != nil { return err }
		
		// Remove from party's list.
		var party Party
		party, err = client.getParty(aclEntry.getPartyId())
		if err != nil { return err }
		
		err = client.deleteACLEntryForParty(party, aclEntry)
		if err != nil { return err }
		
		err = client.writeBack(party)
		if err != nil { return err }
		
		err = client.deleteObject(aclEntry)
		if err != nil { return err }
	}
		
	// Remove all ACL entry ids from the resource's ACL entry list.
	resource.clearAllACLEntryIds()
	
	return client.writeBack(resource)
}

func (repo *InMemResource) deleteAllChildResources(dbClient DBClient) error {
	panic("Call to method that should be abstract")
}

func (resource *InMemResource) clearAllACLEntryIds() {
	resource.ACLEntryIds = resource.ACLEntryIds[0:0]
}

func (resource *InMemResource) getName() string {
	return resource.Name
}

func (client *InMemClient) setName(resource Resource, name string) error {
	resource.setNameDeferredUpdate(name)
	return client.writeBack(resource)
}

func (resource *InMemResource) setNameDeferredUpdate(name string) {
	resource.Name = name
}

func (resource *InMemResource) getCreationTime() time.Time {
	return resource.CreationTime
}

func (resource *InMemResource) getDescription() string {
	return resource.Description
}

func (client *InMemClient) setDescription(resource Resource, desc string) error {
	resource.setDescriptionDeferredUpdate(desc)
	return client.writeBack(resource)
}

func (resource *InMemResource) setDescriptionDeferredUpdate(desc string) {
	resource.Description = desc
}

func (resource *InMemResource) getACLEntryForPartyId(dbClient DBClient, partyId string) (ACLEntry, error) {
	var err error
	for _, entryId := range resource.getACLEntryIds() {
		var entry ACLEntry
		entry, err = dbClient.getACLEntry(entryId)
		if err != nil {
			continue
		}
		if entry.getPartyId() == partyId {
			return entry, err
		}
	}
	return nil, err
}

func (client *InMemClient) getResource(resourceId string) (Resource, error) {
	var resource Resource
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(resourceId)
	if err != nil { return nil, err }
	if obj == nil {
		var err = utils.ConstructUserError("Resource with Id " + resourceId + " not found")
		fmt.Println(err.Error())
		debug.PrintStack()
		return nil, err
	}
	resource, isType = obj.(Resource)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + resourceId + " is not a Resource") }
	return resource, nil
}

func (resource *InMemResource) getParentId() string {
	return resource.ParentId
}

func (resource *InMemResource) isRealm() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Realm)
	return isType
}

func (resource *InMemResource) isRepo() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Repo)
	return isType
}

func (resource *InMemResource) isDockerfile() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Dockerfile)
	return isType
}

func (resource *InMemResource) isDockerImage() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(DockerImage)
	return isType
}

func (resource *InMemResource) isScanConfig() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(ScanConfig)
	return isType
}

func (resource *InMemResource) isFlag() bool {
	var res Resource = resource
	var isType bool
	_, isType = res.(Flag)
	return isType
}

func (resource *InMemResource) resourceFieldsAsJSON() string {
	var json = resource.aclFieldsAsJSON()
	return json + fmt.Sprintf(", \"Name\": \"%s\", \"Description\": \"%s\", " +
		"\"ParentId\": \"%s\", \"CreationTime\": time %s",
		resource.Name, resource.Description, resource.ParentId,
		apitypes.FormatTimeAsJavascriptDate(resource.CreationTime))
}

func (resource *InMemResource) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstituteResource(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time) (*InMemResource, error) {

	var acl *InMemACL
	var err error
	acl, err = client.ReconstituteACL(id, aclEntryIds)
	if err != nil { return nil, err }
	var resource = &InMemResource{
		InMemACL: *acl,
		Name: name,
		Description: desc,
		ParentId: parentId,
		CreationTime: creationTime,
	}
	return resource, nil
}

func (client *InMemClient) isRealm(res Resource) bool {
	return res.isRealm()
}

func (client *InMemClient) isRepo(res Resource) bool {
	return res.isRepo()
}

func (client *InMemClient) isDockerfile(res Resource) bool {
	return res.isDockerfile()
}

func (client *InMemClient) isDockerImage(res Resource) bool {
	return res.isDockerImage()
}

func (client *InMemClient) isScanConfig(res Resource) bool {
	return res.isScanConfig()
}

func (client *InMemClient) isFlag(res Resource) bool {
	return res.isFlag()
}

/*******************************************************************************
 * 
 */
type InMemParty struct {  // abstract
	InMemPersistObj
	IsActive bool
	Name string
	CreationTime time.Time
	RealmId string
	ACLEntryIds []string
}

var _ Party = &InMemParty{}

func (client *InMemClient) NewInMemParty(name string, realmId string) (*InMemParty, error) {
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	return &InMemParty{
		InMemPersistObj: *pers,
		IsActive: true,
		Name: name,
		CreationTime: time.Now(),
		RealmId: realmId,
		ACLEntryIds: make([]string, 0),
	}, nil
}

func (client *InMemClient) setActive(party Party, b bool) error {
	party.setActive(b)
	return client.writeBack(party)
}

func (party *InMemParty) setActive(b bool) {
	party.IsActive = b
}

func (party *InMemParty) isActive() bool {
	return party.IsActive
}

func (party *InMemParty) setNameDeferredUpdate(name string) {
	party.Name = name
}

func (party *InMemParty) getName() string {
	return party.Name
}

func (party *InMemParty) getCreationTime() time.Time {
	return party.CreationTime
}

func (client *InMemClient) getParty(partyId string) (Party, error) {
	var party Party
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(partyId)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Party not found") }
	party, isType = obj.(Party)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + partyId + " is not a Party") }
	return party, nil
}

func (party *InMemParty) getRealmId() string {
	return party.RealmId
}

func (party *InMemParty) getRealm(dbClient DBClient) (Realm, error) {
	return dbClient.getRealm(party.RealmId)
}

func (party *InMemParty) getACLEntryIds() []string {
	return party.ACLEntryIds
}

func (client *InMemClient) addACLEntryForParty(party Party, entry ACLEntry) error {
	party.addACLEntry(entry)
	return client.writeBack(party)
}

func (party *InMemParty) addACLEntry(entry ACLEntry) {
	party.ACLEntryIds = append(party.ACLEntryIds, entry.getId())
}

func (client *InMemClient) deleteACLEntryForParty(party Party, entry ACLEntry) error {
	party.deleteACLEntry(client, entry)
	return client.writeBack(party)
}

func (party *InMemParty) deleteACLEntry(dbClient DBClient, entry ACLEntry) error {
	party.ACLEntryIds = apitypes.RemoveFrom(entry.getId(), party.ACLEntryIds)
	var err error = dbClient.deleteObject(entry)
	return err
}

func (party *InMemParty) getACLEntryForResourceId(dbClient DBClient, resourceId string) (ACLEntry, error) {
	var err error
	for _, entryId := range party.getACLEntryIds() {
		var entry ACLEntry
		entry, err = dbClient.getACLEntry(entryId)
		if err != nil {
			continue
		}
		if entry.getResourceId() == resourceId {
			return entry, err
		}
	}
	return nil, err
}

func (party *InMemParty) partyFieldsAsJSON() string {
	var json = party.persistObjFieldsAsJSON()
	json = json + fmt.Sprintf(", \"IsActive\": %s, \"Name\": \"%s\", " +
		"\"CreationTime\": time %s, \"RealmId\": \"%s\", \"ACLEntryIds\": [",
		apitypes.BoolToString(party.IsActive),
		party.Name,
		apitypes.FormatTimeAsJavascriptDate(party.CreationTime),
		party.RealmId)
	for i, entryId := range party.ACLEntryIds {
		if i != 0 { json = json + ", " }
		json = json + fmt.Sprintf("\"%s\"", entryId)
	}
	json = json + "]"
	return json
}

func (party *InMemParty) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstituteParty(id string, isActive bool,
	name string, creationTime time.Time, realmId string, aclEntryIds []string) (*InMemParty, error) {
	
	var persObj *InMemPersistObj
	var err error
	persObj, err = client.ReconstitutePersistObj(id)
	if err != nil { return nil, err }
	
	return &InMemParty{
		InMemPersistObj: *persObj,
		IsActive: isActive,
		Name: name,
		CreationTime: creationTime,
		RealmId: realmId,
		ACLEntryIds: aclEntryIds,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemGroup struct {
	InMemParty
	Description string
	UserObjIds []string
}

var _ Group = &InMemGroup{}

func (client *InMemClient) NewInMemGroup(realmId string, name string,
	desc string) (*InMemGroup, error) {
	
	var group *InMemParty
	var err error
	group, err = client.NewInMemParty(name, realmId)
	if err != nil { return nil, err }
	var newGroup = &InMemGroup{
		InMemParty: *group,
		Description: desc,
		UserObjIds: make([]string, 0),
	}
	return newGroup, client.updateObject(newGroup)
}

func (client *InMemClient) dbCreateGroup(realmId string, name string,
	description string) (Group, error) {
	
	// Check if a group with that name already exists within the realm.
	var realm Realm
	var err error
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	if realm == nil { return nil, utils.ConstructUserError(fmt.Sprintf(
		"Unidentified realm for realm Id %s", realmId))
	}
	var g Group
	g, err = realm.getGroupByName(client, name)
	if err != nil { return nil, err }
	if g != nil { return nil, utils.ConstructUserError(
		fmt.Sprintf("Group named %s already exists within realm %s", name,
			realm.getName()))
	}
	
	//var groupId string = createUniqueDbObjectId()
	var newGroup *InMemGroup
	newGroup, err = client.NewInMemGroup(realmId, name, description)
	if err != nil { return nil, err }
	
	// Add to parent realm's list
	err = realm.addGroup(client, newGroup)
	if err != nil { return nil, err }
	
	err = client.writeBack(realm)
	if err != nil { return nil, err }
	
	fmt.Println("Created Group")
	return newGroup, nil
}

func (client *InMemClient) getGroup(id string) (Group, error) {
	var group Group
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Group not found") }
	group, isType = obj.(Group)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + id + " is not a Group") }
	return group, nil
}

func (group *InMemGroup) getDescription() string {
	return group.Description
}

func (group *InMemGroup) getUserObjIds() []string {
	return group.UserObjIds
}

func (group *InMemGroup) hasUserWithId(dbClient DBClient, userObjId string) bool {
	var err error 
	var user User
	user, err = dbClient.getUser(userObjId)
	if err != nil { return false }
	if user == nil { return false }
	
	for _, id := range group.UserObjIds {
		if id == userObjId { return true }
	}
	return false
}

func (group *InMemGroup) addUserId(dbClient DBClient, userObjId string) error {
	
	if group.hasUserWithId(dbClient, userObjId) {
		return utils.ConstructUserError(fmt.Sprintf(
			"User with object Id %s is already in group", userObjId))
	}
	
	var err error
	var user User
	user, err = dbClient.getUser(userObjId)
	if err != nil { return err }
	group.UserObjIds = append(group.UserObjIds, userObjId)
	err = user.addGroupId(dbClient, group.getId())
	if err != nil { return err }
	
	err = dbClient.writeBack(user)
	if err != nil { return err }
	
	err = dbClient.writeBack(group)
	
	return err
}

func (group *InMemGroup) removeUser(dbClient DBClient, user User) error {
	var userId string = user.getId()
	for i, id := range group.UserObjIds {
		if id == userId {
			group.UserObjIds = append(group.UserObjIds[0:i], group.UserObjIds[i+1:]...)
			dbClient.writeBack(group)
			return nil
		}
	}
	return utils.ConstructUserError("Did not find user in this group")
}

func (group *InMemGroup) addUser(dbClient DBClient, user User) error {
	group.UserObjIds = append(group.UserObjIds, user.getId())
	return dbClient.writeBack(group)
}

func (group *InMemGroup) asGroupDesc() *apitypes.GroupDesc {
	return apitypes.NewGroupDesc(
		group.Id, group.RealmId, group.Name, group.Description, group.CreationTime)
}

func (group *InMemGroup) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(group)
}

func (group *InMemGroup) asJSON() string {
	var json = "\"Group\": {"
	json = json + group.partyFieldsAsJSON()
	json = json + fmt.Sprintf(", \"Description\": \"%s\", \"UserObjIds\": [",
		group.Description)
	for i, id := range group.UserObjIds {
		if i != 0 { json = json + ", " }
		json = json + fmt.Sprintf("\"%s\"", id)
	}
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteGroup(id string, isActive bool,
		name string, creationTime time.Time, realmId string, aclEntryIds []string,
		desc string, userObjIds []string) (*InMemGroup, error) {
	
	var party *InMemParty
	var err error
	party, err = client.ReconstituteParty(id, isActive, name, creationTime, realmId, aclEntryIds)
	if err != nil { return nil, err }

	return &InMemGroup{
		InMemParty: *party,
		Description: desc,
		UserObjIds: userObjIds,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemUser struct {
	InMemParty
	UserId string
	EmailAddress string
	PasswordHash []byte
	GroupIds []string
	MostRecentLoginAttempts []string
	EventIds []string
}

var _ User = &InMemUser{}

func (client *InMemClient) NewInMemUser(userId string, name string,
	email string, pswd string, realmId string) (*InMemUser, error) {
	
	var party *InMemParty
	var err error
	party, err = client.NewInMemParty(name, realmId)
	if err != nil { return nil, err }
	var passwordHash []byte = client.Server.authService.CreatePasswordHash(pswd)
	var newUser = &InMemUser{
		InMemParty: *party,
		UserId: userId,
		EmailAddress: email,
		PasswordHash: passwordHash,
		GroupIds: make([]string, 0),
		MostRecentLoginAttempts: make([]string, 0),
		EventIds: make([]string, 0),
	}
	
	return newUser, client.addUser(newUser)
}

func (client *InMemClient) dbCreateUser(userId string, name string,
	email string, pswd string, realmId string) (User, error) {
	
	var user User
	var err error
	user, err = client.dbGetUserByUserId(userId)
	if err != nil { return nil, err }
	if user != nil {
		return nil, utils.ConstructUserError("A user with Id " + userId + " already exists")
	}
	
	var realm Realm
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	if realm == nil { return nil, utils.ConstructUserError("Realm with Id " + realmId + " not found") }
	
	//var userObjId string = createUniqueDbObjectId()
	var newUser *InMemUser
	newUser, err = client.NewInMemUser(userId, name, email, pswd, realmId)
	if err != nil { return nil, err }
	
	// Add to parent realm's list.
	err = realm.addUser(client, newUser)
	if err != nil { return nil, err }
	
	err = client.writeBack(realm)
	if err != nil { return nil, err }

	fmt.Println("Created user")
	return newUser, nil
}

func (user *InMemUser) setPassword(dbClient DBClient, pswd string) error {
	user.PasswordHash = dbClient.getServer().authService.CreatePasswordHash(pswd)
	dbClient.writeBack(user)
	return nil
}

func (user *InMemUser) validatePassword(dbClient DBClient, pswd string) bool {
	var empty = []byte{}
	var authService = dbClient.getServer().authService
	var prospectiveHash []byte = authService.computeHash(pswd).Sum(empty)
	return authService.compareHashValues(prospectiveHash, user.PasswordHash)
}

func (client *InMemClient) getUser(id string) (User, error) {
	var user User
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("User with Id " + id + " not found") }
	user, isType = obj.(User)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + id + " is not a User") }
	return user, nil
}

func (user *InMemUser) getUserId() string {
	return user.UserId
}

func (user *InMemUser) setEmailAddressDeferredUpdate(emailAddress string) {
	user.EmailAddress = emailAddress
}

func (user *InMemUser) getEmailAddress() string {
	return user.EmailAddress
}

func (user *InMemUser) hasGroupWithId(dbClient DBClient, groupId string) bool {
	var err error
	var group Group
	group, err = dbClient.getGroup(groupId)
	if err != nil { return false }
	if group == nil { return false }
	for _, id := range user.GroupIds {
		if id == groupId { return true }
	}
	return false
}

func (user *InMemUser) addGroupId(dbClient DBClient, groupId string) error {
	
	if user.hasGroupWithId(dbClient, groupId) { return utils.ConstructUserError(fmt.Sprintf(
		"Group with object Id %s is already in User's set of groups", groupId))
	}
	
	var obj PersistObj
	var err error
	obj, err = dbClient.getPersistentObject(groupId)
	if err != nil { return err }
	if obj == nil { return utils.ConstructUserError(fmt.Sprintf(
		"Object with Id %s does not exist", groupId))
	}
	_, isGroup := obj.(Group)
	if ! isGroup { return utils.ConstructUserError(fmt.Sprintf(
		"Object with Id %s is not a Group", groupId))
	}
	user.GroupIds = append(user.GroupIds, groupId)
	return nil
}

func (user *InMemUser) getGroupIds() []string {
	return user.GroupIds
}

func (client *InMemClient) getRealmsAdministeredByUser(userObjId string) ([]string, error) {
	// those realms for which user can edit the realm
	
	var realmIds []string = make([]string, 0)
	
	// Identify the user.
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(userObjId)
	if err != nil { return nil, err }
	if obj == nil {
		return nil, utils.ConstructUserError("Object with Id " + userObjId + " not found")
	}
	var user User
	var isType bool
	user, isType = obj.(User)
	if ! isType {
		return nil, utils.ConstructServerError("Internal error: object with Id " + userObjId + " is not a User")
	}
	
	// Identify those ACLEntries that are for realms and for which the user has write access.
	for _, entryId := range user.getACLEntryIds() {
		var entry ACLEntry
		entry, err = client.getACLEntry(entryId)
		if err != nil { return nil, err }
		if entry == nil {
			err = utils.ConstructServerError("Internal error: object with Id " + entryId + " is not an ACLEntry")
			continue
		}
		var resourceId string = entry.getResourceId()
		var resource Resource
		resource, err = client.getResource(resourceId)
		if err != nil { return nil, err }
		if resource == nil {
			err = utils.ConstructServerError("Internal error: resource with Id " + resourceId + " not found")
			continue
		}
		if resource.isRealm() {
			var realm Realm = resource.(Realm)
			var mask []bool = entry.getPermissionMask()
			if mask[apitypes.CanWrite] { // entry has write access for the realm
				realmIds = append(realmIds, realm.getId())
			}
		}
	}
	
	return realmIds, err
}

func (user *InMemUser) addLoginAttempt(dbClient DBClient) {
	var num = len(user.MostRecentLoginAttempts)
	var max = dbClient.getServer().MaxLoginAttemptsToRetain
	if num > max { num = num - max }
	user.MostRecentLoginAttempts = append(
		user.MostRecentLoginAttempts[num:], fmt.Sprintf("%d", time.Now().Unix()))
}

func (user *InMemUser) getMostRecentLoginAttempts() []string {
	return user.MostRecentLoginAttempts
}

func (user *InMemUser) addEventId(dbClient DBClient, id string) {
	user.EventIds = append(user.EventIds, id)
	dbClient.writeBack(user)
}

func (user *InMemUser) getEventIds() []string {
	return user.EventIds
}

func (user *InMemUser) deleteEvent(dbClient DBClient, event Event) error {
	
	// If a ScanEvent, then remove from ScanConfig and remove actual ParameterValues.
	var scanEvent ScanEvent
	var isType bool
	scanEvent, isType = event.(ScanEvent)
	if isType {
		var scanConfig ScanConfig
		var err error
		scanConfig, err = dbClient.getScanConfig(scanEvent.getScanConfigId())
		if err != nil { return err }
		err = scanConfig.deleteScanEventId(dbClient, scanEvent.getId())
		if err != nil { return err }
		err = scanEvent.deleteAllParameterValues(dbClient)
		if err != nil { return err }
	}
	
	user.EventIds = apitypes.RemoveFrom(event.getId(), user.EventIds)
	
	var err error = dbClient.deleteObject(event)
	if err != nil { return err }
	return dbClient.writeBack(user)
}

func (user *InMemUser) asUserDesc(dbClient DBClient) *apitypes.UserDesc {
	var adminRealmIds []string
	var err error
	adminRealmIds, err = dbClient.getRealmsAdministeredByUser(user.getId())
	if err != nil {
		fmt.Println("In asUserDesc(), " + err.Error())
		adminRealmIds = make([]string, 0)
	}
	return apitypes.NewUserDesc(user.Id, user.UserId, user.Name, user.RealmId, adminRealmIds)
}

func (user *InMemUser) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(user)
}

func (user *InMemUser) asJSON() string {
	
	var json = "\"User\": {"
	json = json + user.partyFieldsAsJSON()
	json = json + fmt.Sprintf(", \"UserId\": \"%s\", \"EmailAddress\": \"%s\", " +
		"\"PasswordHash\": [", user.UserId, user.EmailAddress)
	for i, b := range user.PasswordHash {
		if i != 0 { json = json + ", " }
		json = json + fmt.Sprintf("%d", b)
	}
	json = json + "], \"GroupIds\": ["
	for i, id := range user.GroupIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "], \"MostRecentLoginAttempts\": ["
	for i, a := range user.MostRecentLoginAttempts {
		if i != 0 { json = json + ", " }
		json = json + "\"" + a + "\""
	}
	json = json + "], \"EventIds\": ["
	for i, id := range user.EventIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteUser(id string, isActive bool,
		name string, creationTime time.Time, realmId string, aclEntryIds []string,
		userId, emailAddr string, pswdHash []byte, groupIds []string,
		loginAttmpts []string, eventIds []string) (*InMemUser, error) {
	
	var party *InMemParty
	var err error
	party, err = client.ReconstituteParty(id, isActive, name, creationTime, realmId, aclEntryIds)
	if err != nil { return nil, err }

	return &InMemUser{
		InMemParty: *party,
		UserId: userId,
		EmailAddress: emailAddr,
		PasswordHash: pswdHash,
		GroupIds: groupIds,
		MostRecentLoginAttempts: loginAttmpts,
		EventIds: eventIds,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemACLEntry struct {
	InMemPersistObj
	ResourceId string
	PartyId string
	PermissionMask []bool
}

var _ ACLEntry = &InMemACLEntry{}

func (client *InMemClient) NewInMemACLEntry(resourceId string, partyId string,
	permissionMask []bool) (*InMemACLEntry, error) {
	
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	var newACLEntry *InMemACLEntry = &InMemACLEntry{
		InMemPersistObj: *pers,
		ResourceId: resourceId,
		PartyId: partyId,
		PermissionMask: permissionMask,
	}
	return newACLEntry, client.updateObject(newACLEntry)
}

func (client *InMemClient) dbCreateACLEntry(resourceId string, partyId string,
	permissionMask []bool) (ACLEntry, error) {
	
	if resourceId == "" { return nil, utils.ConstructServerError("Internal error: resourceId is empty") }
	if partyId == "" { return nil, utils.ConstructServerError("Internal error: partyId is empty") }
	var resource Resource
	var party Party
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(resourceId)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructServerError("Internal error: cannot identify resource: obj with Id '" + resourceId + "' not found") }
	resource, isType = obj.(Resource)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is not a Resource - it is a " +
		reflect.TypeOf(obj).String()) }
	obj, err = client.getPersistentObject(partyId)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructServerError("Internal error: cannot identify party: obj with Id '" + partyId + "' not found") }
	party, isType = obj.(Party)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is not a Party - it is a " +
		reflect.TypeOf(obj).String()) }
	var newACLEntry ACLEntry
	newACLEntry, err = client.NewInMemACLEntry(resourceId, partyId, permissionMask)
	if err != nil { return nil, err }
	err = client.addACLEntry(resource, newACLEntry)  // Add to resource's ACL
	if err != nil { return nil, err }
	
	err = client.addACLEntryForParty(party, newACLEntry)  // Add to user or group's ACL
	if err != nil { return nil, err }
	fmt.Println("\tdbCreateACLEntry: Added ACL entry with Id " + newACLEntry.getId() + " for " + party.getName() + "(a " +
		reflect.TypeOf(party).String() + "), to access " +
		resource.getName() + " (a " + reflect.TypeOf(resource).String() + ")")
	
	return newACLEntry, nil
}

func (client *InMemClient) getACLEntry(id string) (ACLEntry, error) {
	var aclEntry ACLEntry
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("ACLEntry with Id " + id + " not found") }
	aclEntry, isType = obj.(ACLEntry)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return aclEntry, nil
}

func (entry *InMemACLEntry) getResourceId() string {
	return entry.ResourceId
}

func (entry *InMemACLEntry) getPartyId() string {
	return entry.PartyId
}

func (entry *InMemACLEntry) getParty(dbClient DBClient) (Party, error) {
	var party Party
	var obj PersistObj
	var err error
	obj, err = dbClient.getPersistentObject(entry.PartyId)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Party with Id " + entry.PartyId + " not found") }
	var isType bool
	party, isType = obj.(Party)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is not a Party") }
	return party, nil
}

func (entry *InMemACLEntry) getPermissionMask() []bool {
	return entry.PermissionMask
}

func (entry *InMemACLEntry) setPermissionMask(dbClient DBClient, mask []bool) error {
	entry.PermissionMask = mask
	var err error = dbClient.writeBack(entry)
	if err != nil { return err }
	return nil
}

func (entry *InMemACLEntry) asPermissionDesc() *apitypes.PermissionDesc {
	
	return apitypes.NewPermissionDesc(entry.getId(), entry.ResourceId, entry.PartyId, entry.getPermissionMask())
}

func (entry *InMemACLEntry) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(entry)
}

func (entry *InMemACLEntry) asJSON() string {
	var json = "\"ACLEntry\": {"
	json = json + entry.persistObjFieldsAsJSON()
	json = json + fmt.Sprintf(
		", \"ResourceId\": \"%s\", \"PartyId\": \"%s\", \"PermissionMask\": [",
		entry.ResourceId, entry.PartyId)
	for i, b := range entry.PermissionMask {
		if i != 0 { json = json + ", " }
		json = json + apitypes.BoolToString(b)
	}
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteACLEntry(id, resourceId, partyId string,
	permMask []bool) (*InMemACLEntry, error) {

	var persistObj *InMemPersistObj
	var err error
	persistObj, err = client.ReconstitutePersistObj(id)
	if err != nil { return nil, err }

	return &InMemACLEntry{
		InMemPersistObj: *persistObj,
		ResourceId: resourceId,
		PartyId: partyId,
		PermissionMask: permMask,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemRealm struct {
	InMemResource
	AdminUserId string
	OrgFullName string
	UserObjIds []string
	GroupIds []string
	RepoIds []string
	FileDirectory string  // where this realm's files are stored
}

var _ Realm = &InMemRealm{}

func (client *InMemClient) NewInMemRealm(realmInfo *apitypes.RealmInfo, adminUserId string) (*InMemRealm, error) {
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(realmInfo.RealmName, realmInfo.Description, "")
	if err != nil { return nil, err }
	var newRealm *InMemRealm = &InMemRealm{
		InMemResource: *resource,
		AdminUserId: adminUserId,
		OrgFullName: realmInfo.OrgFullName,
		UserObjIds: make([]string, 0),
		GroupIds: make([]string, 0),
		RepoIds: make([]string, 0),
		FileDirectory: "",
	}
	return newRealm, client.addRealm(newRealm)
}

func (client *InMemClient) dbCreateRealm(realmInfo *apitypes.RealmInfo, adminUserId string) (Realm, error) {
	
	var realmId string
	var err error
	realmId, err = client.getRealmIdByName(realmInfo.RealmName)
	if err != nil { return nil, err }
	if realmId != "" {
		return nil, utils.ConstructUserError("A realm with name " + realmInfo.RealmName + " already exists")
	}
	
	err = nameConformsToSafeHarborImageNameRules(realmInfo.RealmName)
	if err != nil { return nil, err }
	
	//realmId = createUniqueDbObjectId()
	var newRealm *InMemRealm
	newRealm, err = client.NewInMemRealm(realmInfo, adminUserId)
	if err != nil { return nil, err }
	var realmFileDir string
	realmFileDir, err = client.Persistence.assignRealmFileDir(client.txn, newRealm.getId())
	if err != nil { return nil, err }
	newRealm.FileDirectory = realmFileDir
	err = client.writeBack(newRealm)
	if err != nil { return nil, err }
	
	fmt.Println("Created realm")
	//_, isType := allObjects[realmId].(Realm)
	//if ! isType {
	//	fmt.Println("*******realm", realmId, "is not a Realm")
	//	fmt.Println("newRealm is a", reflect.TypeOf(newRealm))
	//	fmt.Println("allObjects[", realmId, "] is a", reflect.TypeOf(allObjects[realmId]))
	//}
	return newRealm, nil
}

func (realm *InMemRealm) setNameDeferredUpdate(name string) {
	realm.Name = name
}

func (client *InMemClient) dbDeactivateRealm(realmId string) error {
	
	var err error
	var realm Realm
	realm, err = client.getRealm(realmId)
	if err != nil { return err }
	
	// Remove all ACL entries for the realm.
	err = client.deleteAllAccessToResource(realm)
	if err != nil { return err }
	
	// Remove all ACL entries for each of the realm's repos, and each of their resources.
	for _, repoId := range realm.getRepoIds() {
		var repo Repo
		repo, err = client.getRepo(repoId)
		if err != nil { return err }
		
		err = client.deleteAllAccessToResource(repo)
		if err != nil { return err }
		
		err = client.deleteAllAccess(repo.getDockerfileIds())
		if err != nil { return err }

		err = client.deleteAllAccess(repo.getDockerImageIds())
		if err != nil { return err }

		err = client.deleteAllAccess(repo.getScanConfigIds())
		if err != nil { return err }

		err = client.deleteAllAccess(repo.getFlagIds())
		if err != nil { return err }
	}
	
	// Inactivate all users owned by the realm.
	for _, userObjId := range realm.getUserObjIds() {
		var user User
		user, err = client.getUser(userObjId)
		if err != nil { return err }
		client.setActive(user, false)
	}
	
	// Inactivate all groups owned by the realm.
	for _, groupId := range realm.getGroupIds() {
		var group Group
		group, err = client.getGroup(groupId)
		if err != nil { return err }
		client.setActive(group, false)
	}
	
	return nil
}

func (client *InMemClient) deleteAllAccess(resourceIds []string) error {
	for _, id := range resourceIds {
		var resource Resource
		var err error
		resource, err = client.getResource(id)
		if err != nil { return err }
		err = client.deleteAllAccessToResource(resource)
		if err != nil { return err }
	}
	return nil
}

func (realm *InMemRealm) deleteAllChildResources(dbClient DBClient) error {
	
	// Delete the Realm's Repos.
	var err error
	for _, repoId := range realm.getRepoIds() {
		
		var repo Repo
		repo, err = dbClient.getRepo(repoId)
		if err != nil { return err }
		
		err = realm.deleteRepo(dbClient, repo)
		if err != nil { return err }
	}
	
	return dbClient.writeBack(realm)
}

func (client *InMemClient) getRealmIdByName(name string) (string, error) {
	var realmIds []string
	var err error
	realmIds, err = client.dbGetAllRealmIds()
	if err != nil { return "", err }
	for _, realmId := range realmIds {
		var realm Realm
		var err error
		realm, err = client.getRealm(realmId)
		if err != nil { return "", err }
		if realm.getName() == name { return realmId, nil }
	}
	return "", nil
}

func (realm *InMemRealm) getAdminUserId() string {
	return realm.AdminUserId
}

func (realm *InMemRealm) getFileDirectory() string {
	return realm.FileDirectory
}

func (client *InMemClient) getRealm(id string) (Realm, error) {
	var realm Realm
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Realm not found") }
	realm, isType = obj.(Realm)
	if ! isType {
		fmt.Println("Not a realm")
		debug.PrintStack()
		return nil, utils.ConstructUserError(
		"Object with Id " + id + " is not a Realm - it is a " + reflect.TypeOf(obj).String()) }
	return realm, nil
}

func (realm *InMemRealm) getUserObjIds() []string {
	return realm.UserObjIds
}

func (realm *InMemRealm) getRepoIds() []string {
	return realm.RepoIds
}

func (realm *InMemRealm) addUserId(dbClient DBClient, userObjId string) error {
	
	var user User
	var err error
	user, err = dbClient.getUser(userObjId)
	if err != nil { return err }
	if user == nil { return utils.ConstructUserError("Could not identify user with obj Id " + userObjId) }
	if user.getRealmId() != "" {
		return utils.ConstructUserError("User with obj Id " + userObjId + " belongs to another realm")
	}
	
	// Check if user with same name already exists within the realm.
	var u User
	u, err = realm.getUserByName(dbClient, user.getName())
	if err != nil { return err }
	if u != nil { return utils.ConstructUserError(
		"A user with name " + user.getName() + " already exists in realm " + realm.getName())
	}
	
	realm.UserObjIds = append(realm.UserObjIds, userObjId)
	var inMemUser = user.(*InMemUser)
	inMemUser.RealmId = realm.getId()
	err = dbClient.writeBack(realm)
	if err != nil { return err }
	err = dbClient.writeBack(inMemUser)
	return err
}

func (realm *InMemRealm) removeUserId(dbClient DBClient, userObjId string) (User, error) {
	
	var user User
	var err error
	user, err = dbClient.getUser(userObjId)
	if err != nil { return nil, err }
	if user == nil { return nil, utils.ConstructUserError("User with obj Id " + userObjId + " not found") }
	if user.getRealmId() != realm.getId() {
		return nil, utils.ConstructUserError("User with obj Id " + userObjId + " belongs to another realm")
	}
	realm.UserObjIds = apitypes.RemoveFrom(userObjId, realm.UserObjIds)
	var inMemUser = user.(*InMemUser)
	inMemUser.RealmId = ""
	err = dbClient.writeBack(realm)
	if err != nil { return user, err }
	err = dbClient.writeBack(inMemUser)
	return user, err
}

func (realm *InMemRealm) deleteUserId(dbClient DBClient, userObjId string) error {
	
	var user User
	var err error
	user, err = realm.removeUserId(dbClient, userObjId)
	if err != nil { return err }
	err = dbClient.deleteObject(user)
	if err != nil { return err }
	err = dbClient.writeBack(realm)
	return err
}

func (realm *InMemRealm) getGroupIds() []string {
	return realm.GroupIds
}

func (realm *InMemRealm) addUser(dbClient DBClient, user User) error {
	
	// Check if user with same name already exists.
	var u User
	var err error
	u, err = realm.getUserByName(dbClient, user.getName())
	if err != nil { return err }
	if u != nil { return utils.ConstructUserError(
		"A user with name " + user.getName() + " already exists in realm " + realm.getName())
	}
	
	realm.UserObjIds = append(realm.UserObjIds, user.getId())
	var inMemUser = user.(*InMemUser)
	inMemUser.RealmId = realm.getId()
	return dbClient.writeBack(realm)
}

func (realm *InMemRealm) addGroup(dbClient DBClient, group Group) error {
	
	// Check if group with same name already exists in this realm.
	var g Group
	var err error
	g, err = realm.getGroupByName(dbClient, group.getName())
	if err != nil { return err }
	if g != nil { return utils.ConstructUserError(
		"A group with name " + group.getName() + " already exists in realm " + realm.getName())
	}
	
	realm.GroupIds = append(realm.GroupIds, group.getId())
	return dbClient.writeBack(realm)
}

func (realm *InMemRealm) addRepo(dbClient DBClient, repo Repo) error {
	
	// Check if repo with same name already exists.
	var r Repo
	var err error
	r, err = realm.getRepoByName(dbClient, repo.getName())
	if err != nil { return err }
	if r != nil { return utils.ConstructUserError(
		"A repo with name " + repo.getName() + " already exists in realm " + realm.getName())
	}
	
	realm.RepoIds = append(realm.RepoIds, repo.getId())
	return dbClient.writeBack(realm)
}

func (realm *InMemRealm) asRealmDesc() *apitypes.RealmDesc {
	return apitypes.NewRealmDesc(realm.Id, realm.Name, realm.OrgFullName, realm.AdminUserId)
}

func (realm *InMemRealm) hasUserWithId(dbClient DBClient, userObjId string) bool {
	var obj PersistObj
	var err error
	obj, err = dbClient.getPersistentObject(userObjId)
	if err != nil { return false }
	if obj == nil { return false }
	_, isUser := obj.(User)
	if ! isUser { return false }
	
	for _, id := range realm.UserObjIds {
		if id == userObjId { return true }
	}
	return false
}

func (realm *InMemRealm) hasGroupWithId(dbClient DBClient, groupId string) bool {
	var obj PersistObj
	var err error
	obj, err = dbClient.getPersistentObject(groupId)
	if err != nil { return false }
	if obj == nil { return false }
	_, isGroup := obj.(Group)
	if ! isGroup { return false }
	
	for _, id := range realm.GroupIds {
		if id == groupId { return true }
	}
	return false
}

func (realm *InMemRealm) hasRepoWithId(dbClient DBClient, repoId string) bool {
	var obj PersistObj
	var err error
	obj, err = dbClient.getPersistentObject(repoId)
	if err != nil { return false }
	if obj == nil { return false }
	_, isRepo := obj.(Repo)
	if ! isRepo { return false }
	
	for _, id := range realm.RepoIds {
		if id == repoId { return true }
	}
	return false
}

func (realm *InMemRealm) getUserByName(dbClient DBClient, userName string) (User, error) {
	for _, id := range realm.UserObjIds {
		var obj PersistObj
		var err error
		obj, err = dbClient.getPersistentObject(id)
		if err != nil { return nil, err }
		if obj == nil { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		user, isUser := obj.(User)
		if ! isUser { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s is not a User", id))
		}
		if user.getName() == userName { return user, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) getUserByUserId(dbClient DBClient, userId string) (User, error) {
	for _, id := range realm.UserObjIds {
		var obj PersistObj
		var err error
		obj, err = dbClient.getPersistentObject(id)
		if err != nil { return nil, err }
		if obj == nil { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		user, isUser := obj.(User)
		if ! isUser { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s is not a User", id))
		}
		if user.getUserId() == userId { return user, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) getGroupByName(dbClient DBClient, groupName string) (Group, error) {
	for _, id := range realm.GroupIds {
		var obj PersistObj
		var err error
		obj, err = dbClient.getPersistentObject(id)
		if err != nil { return nil, err }
		if obj == nil { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		group, isGroup := obj.(Group)
		if ! isGroup { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s is not a Group", id))
		}
		if group.getName() == groupName { return group, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) getRepoByName(dbClient DBClient, repoName string) (Repo, error) {
	for _, id := range realm.RepoIds {
		var obj PersistObj
		var err error
		obj, err = dbClient.getPersistentObject(id)
		if err != nil { return nil, err }
		if obj == nil { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s does not exist", id))
		}
		repo, isRepo := obj.(Repo)
		if ! isRepo { return nil, utils.ConstructServerError(fmt.Sprintf(
			"Internal error: obj with Id %s is not a Repo", id))
		}
		if repo.getName() == repoName { return repo, nil }
	}
	return nil, nil
}

func (realm *InMemRealm) deleteGroup(dbClient DBClient, group Group) error {
	
	// Remove the group from its realm. Do this first so that no new actions
	// will be taken on the group.
	realm.GroupIds = apitypes.RemoveFrom(group.getId(), realm.GroupIds)
	var err error
	err = dbClient.writeBack(realm)
	if err != nil { return err }
	
	// Remove users from the group.
	for _, userObjId := range group.getUserObjIds() {
		var user User
		var err error
		user, err = dbClient.getUser(userObjId)
		if err != nil { return err }
		err = group.removeUser(dbClient, user)
		if err != nil { return err }
	}
	
	// Remove ACL entries referenced by the group.
	var entryIds []string = group.getACLEntryIds()
	var entryIdsCopy []string = make([]string, len(entryIds))
	copy(entryIdsCopy, entryIds)
	for _, entryId := range entryIdsCopy {
		var err error
		var entry ACLEntry
		entry, err = dbClient.getACLEntry(entryId)
		if err != nil { return err }
		var resource Resource
		resource, err = dbClient.getResource(entry.getResourceId())
		if err != nil { return err }
		var party Party
		party, err = dbClient.getParty(entry.getPartyId())
		if err != nil { return err }
		err = dbClient.deleteAccess(resource, party)
		if err != nil { return err }
	}
	
	return nil
}

func (realm *InMemRealm) deleteRepo(dbClient DBClient, repo Repo) error {
	
	// Remove the Repo from its Realm. Do this first so that no new actions
	// will be taken on the Repo.
	realm.RepoIds = apitypes.RemoveFrom(repo.getId(), realm.RepoIds)
	var err error
	err = dbClient.writeBack(realm)
	if err != nil { return err }
	
	// Remove the Repo's resources.
	err = repo.deleteAllChildResources(dbClient)
	if err != nil { return err }
	
	// Remove ACL entries.
	err = dbClient.deleteAllAccessToResource(repo)
	if err != nil { return err }
	
	return nil
}

func (realm *InMemRealm) isRealm() bool { return true }

func (realm *InMemRealm) writeBack(dbClient DBClient) error {
	// Important: it is assumed that the name has not changed.
	return dbClient.updateObject(realm)
}

func (realm *InMemRealm) asJSON() string {
	
	var json = "\"Realm\": {"
	json = json + realm.resourceFieldsAsJSON()
	json = json + fmt.Sprintf(
		", \"AdminUserId\": \"%s\", \"OrgFullName\": \"%s\", \"UserObjIds\": [",
		realm.AdminUserId, realm.OrgFullName)
	for i, id := range realm.UserObjIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "], \"GroupIds\": ["
	for i, id := range realm.GroupIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "], \"RepoIds\": ["
	for i, id := range realm.RepoIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + fmt.Sprintf("], \"FileDirectory\": \"%s\"}", realm.FileDirectory)
	return json
}

func (client *InMemClient) ReconstituteRealm(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time,
	adminUserId string, orgFullName string,
	userObjIds, groupIds, repoIds []string, fileDir string) (*InMemRealm, error) {

	var resource *InMemResource
	var err error
	resource, err = client.ReconstituteResource(id, aclEntryIds, name, desc, parentId,
		creationTime)
	if err != nil { return nil, err }
	
	return &InMemRealm{
		InMemResource: *resource,
		AdminUserId: adminUserId,
		OrgFullName: orgFullName,
		UserObjIds: userObjIds,
		GroupIds: groupIds,
		RepoIds: repoIds,
		FileDirectory: fileDir,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemRepo struct {
	InMemResource
	DockerfileIds []string
	DockerImageIds []string
	ScanConfigIds []string
	FlagIds []string
	FileDirectory string  // where this repo's files are stored
}

var _ Repo = &InMemRepo{}

func (client *InMemClient) NewInMemRepo(realmId, name, desc string) (*InMemRepo, error) {
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, realmId)
	if err != nil { return nil, err }
	var newRepo *InMemRepo = &InMemRepo{
		InMemResource: *resource,
		DockerfileIds: make([]string, 0),
		DockerImageIds: make([]string, 0),
		ScanConfigIds: make([]string, 0),
		FlagIds: make([]string, 0),
		FileDirectory: "",
	}
	return newRepo, client.updateObject(newRepo)
}

func (client *InMemClient) dbCreateRepo(realmId, name, desc string) (Repo, error) {
	var realm Realm
	var err error
	realm, err = client.getRealm(realmId)
	if err != nil { return nil, err }
	
	err = nameConformsToSafeHarborImageNameRules(name)
	if err != nil { return nil, err }
	
	//var repoId string = createUniqueDbObjectId()
	var newRepo *InMemRepo
	newRepo, err = client.NewInMemRepo(realmId, name, desc)
	if err != nil { return nil, err }

	var repoFileDir string
	repoFileDir, err = client.Persistence.assignRepoFileDir(client.txn, realm, newRepo.getId())
	if err != nil { return nil, err }
	newRepo.FileDirectory = repoFileDir
	err = client.writeBack(newRepo)
	if err != nil { return nil, err }
	fmt.Println("Created repo")
	err = realm.addRepo(client, newRepo)  // Add it to the realm.
	return newRepo, err
}

func (repo *InMemRepo) getFileDirectory() string {
	return repo.FileDirectory
}

func (client *InMemClient) getRepo(id string) (Repo, error) {
	var repo Repo
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Repo not found") }
	repo, isType = obj.(Repo)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + id + " is not a Repo") }
	return repo, nil
}

func (repo *InMemRepo) getRealmId() string { return repo.ParentId }

func (repo *InMemRepo) getRealm(dbClient DBClient) (Realm, error) {
	var realm Realm
	var err error
	var obj PersistObj
	obj, err = dbClient.getPersistentObject(repo.getRealmId())
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructServerError("Realm with Id " + repo.getRealmId() + " not found") }
	var isType bool
	realm, isType = obj.(Realm)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return realm, nil
}

func (repo *InMemRepo) getDockerfileIds() []string {
	return repo.DockerfileIds
}

func (repo *InMemRepo) getDockerImageIds() []string {
	return repo.DockerImageIds
}

func (repo *InMemRepo) getScanConfigIds() []string {
	return repo.ScanConfigIds
}

func (repo *InMemRepo) getFlagIds() []string {
	return repo.FlagIds
}

func (repo *InMemRepo) addDockerfile(dbClient DBClient, dockerfile Dockerfile) error {
	
	// Check if dockerfile with same name already exists in the repo.
	var d Dockerfile
	var err error
	d, err = repo.getDockerfileByName(dbClient, dockerfile.getName())
	if err != nil { return err }
	if d != nil { return utils.ConstructUserError(
		"A dockerfile with name " + d.getName() + " already exists in repo " + repo.getName())
	}
	
	repo.DockerfileIds = append(repo.DockerfileIds, dockerfile.getId())
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) addDockerImage(dbClient DBClient, image DockerImage) error {
	
	// Check if docker image with same name already exists in the repo.
	var d DockerImage
	var err error
	d, err = repo.getDockerImageByName(dbClient, image.getName())
	if err != nil { return err }
	if d != nil { return utils.ConstructUserError(
		"A docker image with name " + d.getName() + " already exists in repo " + repo.getName())
	}
	
	repo.DockerImageIds = append(repo.DockerImageIds, image.getId())
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) addScanConfig(dbClient DBClient, config ScanConfig) error {
	
	// Check if scan config with same name already exists in this repo.
	var s ScanConfig
	var err error
	s, err = repo.getScanConfigByName(dbClient, config.getName())
	if err != nil { return err }
	if s != nil { return utils.ConstructUserError(
		"A scan config with name " + s.getName() + " already exists in repo " + repo.getName())
	}
	
	repo.ScanConfigIds = append(repo.ScanConfigIds, config.getId())
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) deleteScanConfig(dbClient DBClient, config ScanConfig) error {
	
	// Nullify ScanConfig in ScanEvents.
	for _, scanEventId := range config.getScanEventIds() {
		
		var scanEvent ScanEvent
		var err error
		scanEvent, err = dbClient.getScanEvent(scanEventId)
		if err != nil { return err }
		
		err = scanEvent.nullifyScanConfig(dbClient)
		if err != nil { return err }
	}
	
	// Remove config's parameter values.
	config.deleteAllParameterValues(dbClient)
	
	// Remove reference from the flag.
	var flagId string = config.getFlagId()
	if flagId != "" {
		var err error
		var flag Flag
		flag, err = dbClient.getFlag(flagId)
		if err != nil { return err }
		err = flag.removeScanConfigRef(dbClient, config.getId())
		if err != nil { return err }
	}
	
	// Unlink from DockerImages that use this ScanConfig.
	for _, imageId := range config.getDockerImageIdsThatUse() {
		err = config.remDockerImage(dbClient, imageId)
		if err != nil { return err }
	}

	// Remove from repo.
	repo.ScanConfigIds = apitypes.RemoveFrom(config.getId(), repo.ScanConfigIds)

	// Remove ACL entries.
	var err error = dbClient.deleteAllAccessToResource(config)
	if err != nil { return err }

	// Remove from database.
	err = dbClient.deleteObject(config)
	if err != nil { return err }
	
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) deleteFlag(dbClient DBClient, flag Flag) error {
	if len(flag.usedByScanConfigIds()) > 0 {
		var sc ScanConfig
		var err error
		sc, err = dbClient.getScanConfig(flag.usedByScanConfigIds()[0])
		if err != nil { return err }
		return utils.ConstructUserError(
			"Cannot remove Flag: it is referenced by one or more ScanConfigs, " +
			"including " + sc.getName() + " (" + sc.getId() + ")")
	}

	// Remove the graphic file associated with the flag.
	fmt.Println("Removing file " + flag.getSuccessImagePath())
	var err error = os.Remove(flag.getSuccessImagePath())
	if err != nil { return err }
	
	// Remove from repo.
	repo.FlagIds = apitypes.RemoveFrom(flag.getId(), repo.FlagIds)
	
	// Remove ACL entries.
	err = dbClient.deleteAllAccessToResource(flag)
	if err != nil { return err }
	
	// Remove from database.
	err = dbClient.deleteObject(flag)
	if err != nil { return err }
	
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) deleteDockerfile(dbClient DBClient, dockerfile Dockerfile) error {
	
	// Nullify dockerfile in DockerfileExecEvents.
	for _, execEventId := range dockerfile.getDockerfileExecEventIds() {
		
		var event Event
		var err error
		event, err = dbClient.getEvent(execEventId)
		if err != nil { return err }
		
		var dockerfileExecEvent DockerfileExecEvent
		var isType bool
		dockerfileExecEvent, isType = event.(DockerfileExecEvent)
		if ! isType { return utils.ConstructServerError("Unexpected event type for dockerfile") }
		dockerfileExecEvent.nullifyDockerfile(dbClient)
	}
	
	var err error = dbClient.writeBack(dockerfile)
	if err != nil { return err }
	
	err = dbClient.deleteAllAccessToResource(dockerfile)
	if err != nil { return err }
	
	return nil
}

func (repo *InMemRepo) deleteDockerImage(dbClient DBClient, image DockerImage) error {
	
	// Remove each ImageVersion.
	for _, imageVersionId := range image.getImageVersionIds() {
		var dockerImageVersion DockerImageVersion
		var err error
		dockerImageVersion, err = dbClient.getDockerImageVersion(imageVersionId)
		if err != nil { return err }
		err = image.deleteImageVersion(dbClient, dockerImageVersion)
		if err != nil { return err }
	}
	
	// Remove ACL entries.
	var err error
	err = dbClient.deleteAllAccessToResource(image)
	if err != nil { return err }
	
	// Unlink from ScanConfigs.
	for _, configId := range config.getScanConfigsToUse() {
		var scanConfig ScanConfig
		scanConfig, err = dbClient.getScanConfig(configId)
		if err != nil { return err }
		err = scanConfig.remDockerImage(dbClient, image.getId())
		if err != nil { return err }
	}
	
	// Remove from repo.
	repo.DockerImageIds = apitypes.RemoveFrom(image.getId(), repo.DockerImageIds)
	
	// Remove from database.
	err = dbClient.deleteObject(image)
	if err != nil { return err }
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) addFlag(dbClient DBClient, flag Flag) error {
	
	// Check if flag with same name already exists in this repo.
	var f Flag
	var err error
	f, err = repo.getFlagByName(dbClient, flag.getName())
	if err != nil { return err }
	if f != nil { return utils.ConstructUserError(
		"A flag with name " + f.getName() + " already exists in repo " + repo.getName())
	}
	
	repo.FlagIds = append(repo.FlagIds, flag.getId())
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) getDockerfileByName(dbClient DBClient, name string) (Dockerfile, error) {
	for _, dockerfileId := range repo.DockerfileIds {
		var dockerfile Dockerfile
		var err error
		dockerfile, err = dbClient.getDockerfile(dockerfileId)
		if err != nil { return nil, err }
		if dockerfile == nil {
			return nil, utils.ConstructServerError("Internal error: list DockerfileIds contains an invalid entry")
		}
		if dockerfile.getName() == name { return dockerfile, nil }
	}
	return nil, nil
}

func (repo *InMemRepo) getFlagByName(dbClient DBClient, name string) (Flag, error) {
	for _, flagId := range repo.FlagIds {
		var flag Flag
		var err error
		flag, err = dbClient.getFlag(flagId)
		if err != nil { return nil, err }
		if flag == nil {
			return nil, utils.ConstructServerError("Internal error: list FlagIds contains an invalid entry")
		}
		if flag.getName() == name { return flag, nil }
	}
	return nil, nil
}

func (repo *InMemRepo) getDockerImageByName(dbClient DBClient, name string) (DockerImage, error) {
	for _, dockerImageId := range repo.DockerImageIds {
		var dockerImage DockerImage
		var err error
		dockerImage, err = dbClient.getDockerImage(dockerImageId)
		if err != nil { return nil, err }
		if dockerImage == nil {
			return nil, utils.ConstructServerError("List DockerImageIds contains an invalid entry")
		}
		if dockerImage.getName() == name { return dockerImage, nil }
	}
	return nil, nil
}

func (repo *InMemRepo) getScanConfigByName(dbClient DBClient, name string) (ScanConfig, error) {
	for _, configId := range repo.ScanConfigIds {
		var config ScanConfig
		var err error
		config, err = dbClient.getScanConfig(configId)
		if err != nil { return nil, err }
		if config == nil {
			return nil, utils.ConstructServerError("Internal error: list ScanConfigIds contains an invalid entry")
		}
		if config.getName() == name { return config, nil }
	}
	return nil, nil
}

func (repo *InMemRepo) deleteAllChildResources(dbClient DBClient) error {
	
	// Remove all resources from the Repo: Dockerfiles, ScanConfigs, Flags, Images.
	
	var err error
	for _, dockerfileId := range repo.getDockerfileIds() {
		
		var dockerfile Dockerfile
		dockerfile, err = dbClient.getDockerfile(dockerfileId)
		if err != nil { return err }
		
		err = repo.deleteDockerfile(dbClient, dockerfile)
		if err != nil { return err }
	}
	
	for _, scanConfigId := range repo.getScanConfigIds() {
		
		var scanConfig ScanConfig
		scanConfig, err = dbClient.getScanConfig(scanConfigId)
		if err != nil { return err }
		
		err = repo.deleteScanConfig(dbClient, scanConfig)
		if err != nil { return err }
	}
	
	for _, flagId := range repo.getFlagIds() {
		
		var flag Flag
		flag, err = dbClient.getFlag(flagId)
		if err != nil { return err }
		
		err = repo.deleteFlag(dbClient, flag)
		if err != nil { return err }
	}
	
	for _, imageId := range repo.getDockerImageIds() {
		
		var image DockerImage
		image, err = dbClient.getDockerImage(imageId)
		if err != nil { return err }
		
		err = repo.deleteDockerImage(dbClient, image)
		if err != nil { return err }
	}
	
	return dbClient.writeBack(repo)
}

func (repo *InMemRepo) isRepo() bool { return true }

func (repo *InMemRepo) asRepoDesc() *apitypes.RepoDesc {
	return apitypes.NewRepoDesc(repo.Id, repo.getRealmId(), repo.Name, repo.Description,
		repo.CreationTime, repo.getDockerfileIds())
}

func (repo *InMemRepo) asRepoPlusDockerfileDesc(newDockerfileId string) *apitypes.RepoPlusDockerfileDesc {
	return apitypes.NewRepoPlusDockerfileDesc(repo.getId(), repo.getRealmId(),
		repo.getName(), repo.getDescription(),
		repo.getCreationTime(), repo.getDockerfileIds(), newDockerfileId)
}

func (repo *InMemRepo) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(repo)
}

func (repo *InMemRepo) asJSON() string {
	
	var json = "\"Repo\": {"
	json = json + repo.resourceFieldsAsJSON()
	json = json + ", \"DockerFieldIds\": ["
	for i, id := range repo.DockerfileIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "], \"DockerImageIds\": ["
	for i, id := range repo.DockerImageIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "], \"ScanConfigIds\": ["
	for i, id := range repo.ScanConfigIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "], \"FlagIds\": ["
	for i, id := range repo.FlagIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + fmt.Sprintf("], \"FileDirectory\": \"%s\"}", repo.FileDirectory)
	return json
}

func (client *InMemClient) ReconstituteRepo(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time,
	dockerfileIds, imageIds, configIds, flagIds []string, fileDir string) (*InMemRepo, error) {

	var resource *InMemResource
	var err error
	resource, err = client.ReconstituteResource(id, aclEntryIds, name, desc, parentId,
		creationTime)
	if err != nil { return nil, err }
	
	return &InMemRepo{
		InMemResource: *resource,
		DockerfileIds: dockerfileIds,
		DockerImageIds: imageIds,
		ScanConfigIds: configIds,
		FlagIds: flagIds,
		FileDirectory: fileDir,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemDockerfile struct {
	InMemResource
	FilePath string
	DockerfileExecEventIds []string
}

var _ Dockerfile = &InMemDockerfile{}

func (client *InMemClient) NewInMemDockerfile(repoId, name, desc,
	filepath string) (*InMemDockerfile, error) {
	
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	var newDockerfile *InMemDockerfile = &InMemDockerfile{
		InMemResource: *resource,
		FilePath: filepath,
		DockerfileExecEventIds: make([]string, 0),
	}
	return newDockerfile, client.updateObject(newDockerfile)
}

func (client *InMemClient) dbCreateDockerfile(repoId, name,
	desc, filepath string) (Dockerfile, error) {
	
	var newDockerfile Dockerfile
	var err error
	newDockerfile, err = client.NewInMemDockerfile(repoId, name, desc, filepath)
	if err != nil { return nil, err }
	fmt.Println("Created Dockerfile")
	
	// Add to the Repo's list of Dockerfiles.
	var repo Repo
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	if repo == nil {
		fmt.Println("Repo with Id " + repoId + " not found")
		return nil, utils.ConstructUserError(fmt.Sprintf("Repo with Id %s not found", repoId))
	}
	err = repo.addDockerfile(client, newDockerfile)
	if err != nil { return nil, err }
	
	return newDockerfile, nil
}

func (client *InMemClient) getDockerfile(id string) (Dockerfile, error) {
	var dockerfile Dockerfile
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Dockerfile not found") }
	dockerfile, isType = obj.(Dockerfile)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + id + " is not a Dockerfile") }
	return dockerfile, nil
}

func (dockerfile *InMemDockerfile) replaceDockerfileFile(filepath, desc string) error {
	
	if desc == "" { desc = dockerfile.getDescription() }  // use current description.
	
	var oldFilePath = dockerfile.getExternalFilePath()
	
	dockerfile.FilePath = filepath
	dockerfile.Description = desc
	dockerfile.CreationTime = time.Now()
	
	// Delete old file.
	fmt.Println("Removing file " + oldFilePath)
	return os.Remove(oldFilePath)
}

//func (dockerfile *InMemDockerfile) getParameterValueIds() string {
	//....
//}

func (dockerfile *InMemDockerfile) getRepoId() string {
	return dockerfile.ParentId
}

func (dockerfile *InMemDockerfile) getRepo(dbClient DBClient) (Repo, error) {
	var repo Repo
	var obj PersistObj
	var err error
	obj, err = dbClient.getPersistentObject(dockerfile.getRepoId())
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Could not find obj with Id " + dockerfile.getRepoId()) }
	var isType bool
	repo, isType = obj.(Repo)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return repo, nil
}

func (dockerfile *InMemDockerfile) getDockerfileExecEventIds() []string {
	return dockerfile.DockerfileExecEventIds
}

func (dockerfile *InMemDockerfile) addEventId(dbClient DBClient, eventId string) error {
	
	dockerfile.DockerfileExecEventIds = append(dockerfile.DockerfileExecEventIds, eventId)
	return dbClient.writeBack(dockerfile)
}

func (dockerfile *InMemDockerfile) getExternalFilePath() string {
	return dockerfile.FilePath
}

func (dockerfile *InMemDockerfile) deleteAllChildResources(dbClient DBClient) error {
	return nil
}

func (dockerfile *InMemDockerfile) asDockerfileDesc() *apitypes.DockerfileDesc {
	return apitypes.NewDockerfileDesc(dockerfile.Id, dockerfile.getRepoId(), dockerfile.Name, dockerfile.Description)
}

func (dockerfile *InMemDockerfile) isDockerfile() bool { return true }

func (dockerfile *InMemDockerfile) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(dockerfile)
}

func (dockerfile *InMemDockerfile) asJSON() string {
	
	var json = "\"Dockerfile\": {"
	json = json + dockerfile.resourceFieldsAsJSON()
	json = json + fmt.Sprintf(
		", \"FilePath\": \"%s\", \"DockerfileExecEventIds\": [", dockerfile.FilePath)
	for i, id := range dockerfile.DockerfileExecEventIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteDockerfile(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time,
	filePath string, eventIds []string) (*InMemDockerfile, error) {

	var resource *InMemResource
	var err error
	resource, err = client.ReconstituteResource(id, aclEntryIds, name, desc, parentId,
		creationTime)
	if err != nil { return nil, err }

	return &InMemDockerfile{
		InMemResource: *resource,
		FilePath: filePath,
		DockerfileExecEventIds: eventIds,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemImage struct {  // abstract
	InMemResource
	VersionIds []string
}

var _ Image = &InMemImage{}

func (client *InMemClient) NewInMemImage(name, desc, repoId string) (*InMemImage, error) {
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	return &InMemImage{
		InMemResource: *resource,
		VersionIds: make([]string, 0),
	}, nil
}

func (client *InMemClient) getImage(id string) (Image, error) {
	
	var image Image
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Image not found") }
	image, isType = obj.(Image)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + id + " is not an Image") }
	return image, nil
}

func (image *InMemImage) writeBack(dbClient DBClient) error {
	panic("Abstract method should not be called")
}

func (image *InMemImage) getName() string {
	return image.Name
}

func (image *InMemImage) getRepoId() string {
	return image.ParentId
}

func (image *InMemImage) getRepo(dbClient DBClient) (Repo, error) {
	var repo Repo
	var obj PersistObj
	var err error
	obj, err = dbClient.getPersistentObject(image.getRepoId())
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Could not find obj with Id " + image.getRepoId()) }
	var isType bool
	repo, isType = obj.(Repo)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return repo, nil
}

func (image *InMemImage) deleteAllChildResources(dbClient DBClient) error {
	panic("Call to method that should be abstract")
}

func (image *InMemImage) addVersionId(dbClient DBClient, dockerImageVersionObjId string) error {
	panic("Call to method that should be abstract")
}

func (image *InMemImage) getImageVersionIds() []string {
	return image.VersionIds
}

func (image *InMemImage) getUniqueVersion(dbClient DBClient) (string, error) {
	
	var version string
	var err error
	version, err = dbClient.getPersistence().incrementDatabaseKey(
		ObjectScopeVersionNumbersPrefix + image.getId())
	return version, err
}

func (image *InMemImage) getMostRecentVersionId() string {
	return image.VersionIds[len(image.VersionIds)-1]
}

func (image *InMemImage) deleteImageVersion(dbClient DBClient, imageVersion ImageVersion) error {
	panic("Call to abstract method")
}

func (image *InMemImage) imageFieldsAsJSON() string {
	var json = image.resourceFieldsAsJSON()
	json = json + ", \"VersionIds\": ["
	for i, versionId := range image.VersionIds {
		if i > 0 { json = json + ", " }
		json = json + fmt.Sprintf("\"%s\"", versionId)
	}
	json = json + "]"
	return json
}

func (image *InMemImage) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstituteImage(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time, versionIds []string) (*InMemImage, error) {

	var resource *InMemResource
	var err error
	resource, err = client.ReconstituteResource(id, aclEntryIds, name, desc, parentId,
		creationTime)
	if err != nil { return nil, err }

	return &InMemImage{
		InMemResource: *resource,
		VersionIds: versionIds,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemDockerImage struct {
	InMemImage
	ScanConfigsToUse []string
}

var _ DockerImage = &InMemDockerImage{}

func (client *InMemClient) NewInMemDockerImage(name, desc, repoId string) (*InMemDockerImage, error) {
	var image *InMemImage
	var err error
	image, err = client.NewInMemImage(name, desc, repoId)
	if err != nil { return nil, err }

	var newDockerImage = &InMemDockerImage{
		InMemImage: *image,
		ScanConfigsToUse: make([]string, 0),
	}
	
	return newDockerImage, client.updateObject(newDockerImage)
}

func (client *InMemClient) dbCreateDockerImage(repoId, imageName, desc string) (DockerImage, error) {
	
	var repo Repo
	var err error
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	
	var newDockerImage *InMemDockerImage
	newDockerImage, err = client.NewInMemDockerImage(imageName, desc, repoId)
	if err != nil { return nil, err }
	fmt.Println("Created DockerImage")

	err = repo.addDockerImage(client, newDockerImage)  // Add to repo's list.
	if err != nil { return nil, err }

	return newDockerImage, err
}

func (client *InMemClient) getDockerImage(id string) (DockerImage, error) {
	
	var image DockerImage
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("DockerImage not found") }
	image, isType = obj.(DockerImage)
	if ! isType { return nil, utils.ConstructUserError(
		"Object with Id " + id + " is not a DockerImage: it is a " + reflect.TypeOf(obj).String()) }
	return image, nil
}

func (image *InMemDockerImage) addVersionId(dbClient DBClient, dockerImageVersionObjId string) error {
	
	image.VersionIds = append(image.VersionIds, dockerImageVersionObjId)
	var err = dbClient.writeBack(image)
	return err
}

func (image *InMemDockerImage) deleteImageVersion(dbClient DBClient, imageVersion ImageVersion) error {
	
	var dockerImageVersion DockerImageVersion
	var isType bool
	dockerImageVersion, isType = imageVersion.(DockerImageVersion)
	if ! isType { return utils.ConstructUserError("Image is not a docker image") }
	
	// Nullify Image in ImageCreationEvent.
	var imageCreationEventId string = dockerImageVersion.getImageCreationEventId()
	var event Event
	var err error
	event, err = dbClient.getEvent(imageCreationEventId)
	if err != nil { return err }
	var imageCreationEvent ImageCreationEvent
	if event == nil { return utils.ConstructServerError("Event is nil") }
	imageCreationEvent, isType = event.(ImageCreationEvent)
	if ! isType { return utils.ConstructServerError(
		"Internal error: Expected event to be an ImageCreationEvent: it is a " +
			reflect.TypeOf(event).String())
	}
	if imageCreationEvent == nil { fmt.Println("imageCreationEvent is nil") }
	imageCreationEvent.nullifyImageVersion()
	err = dbClient.updateObject(imageCreationEvent)
	if err != nil { return err }
	
	// Nullify Image in ScanEvents.
	for _, scanEventId := range dockerImageVersion.getScanEventIds() {
		
		var scanEvent ScanEvent
		var err error
		scanEvent, err = dbClient.getScanEvent(scanEventId)
		if err != nil { return err }
		
		err = scanEvent.nullifyDockerImageVersion(dbClient)
		if err != nil { return err }
	}
	
	// Remove from docker.
	var realmName, repoName, imageName, version string
	realmName, repoName, imageName, version, err = dockerImageVersion.getFullNameParts(dbClient)
	if err != nil { return err }
	var dockerImageName, tag string
	dockerImageName, tag = docker.ConstructDockerImageName(
		realmName, repoName, imageName, version)
	err = dbClient.getServer().DockerServices.RemoveDockerImage(dockerImageName, tag)
	if err != nil { return err }
	
	// Remove from image's list of versions.
	image.VersionIds = apitypes.RemoveFrom(imageVersion.getId(), image.VersionIds)
	
	// Remove from database.
	dbClient.deleteObject(imageVersion)
	dbClient.updateObject(image)
	
	return err
}

func (image *InMemDockerImage) getScanConfigsToUse() []string {
	return image.ScanConfigsToUse
}

func (image *InMemDockerImage) addScanConfigIdToList(scanConfigId string) {
	image.ScanConfigsToUse = apitypes.AddUniquely(scanConfigId, image.ScanConfigsToUse)
}

func (image *InMemDockerImage) remScanConfigIdFromList(scanConfigId string) {
	image.ScanConfigsToUse = apitypes.RemoveFrom(scanConfigId, image.ScanConfigsToUse)
}

func (image *InMemDockerImage) asDockerImageDesc() *apitypes.DockerImageDesc {
	return apitypes.NewDockerImageDesc(image.Id, image.getRepoId(), image.Name,
		image.Description)
}

func (image *InMemDockerImage) isDockerImage() bool { return true }

func (image *InMemDockerImage) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(image)
}

func (image *InMemDockerImage) asJSON() string {
	
	var json = "\"DockerImage\": {" + image.imageFieldsAsJSON() + ", \"ScanConfigsToUse\": ["
	for i, id := range image.ScanConfigsToUse {
		if i > 0 { json = json + ", " }
		json = json + fmt.Sprintf("\"%s\"", id)
	}
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteDockerImage(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time, versionIds,
	scanConfigsToUse []string) (*InMemDockerImage, error) {

	var image *InMemImage
	var err error
	image, err = client.ReconstituteImage(id, aclEntryIds, name, desc, parentId,
		creationTime, versionIds)
	
	if err != nil { return nil, err }
	
	return &InMemDockerImage{
		InMemImage: *image,
		ScanConfigsToUse: scanConfigsToUse,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemImageVersion struct {  // abstract
	InMemPersistObj
	Version string
	ImageObjId string
	ImageCreationEventId string
    CreationDate time.Time
}

var _ ImageVersion = &InMemImageVersion{}

func (client *InMemClient) NewInMemImageVersion(version, imageObjId string,
	creationEventId string, creationDate time.Time) (*InMemImageVersion, error) {
	
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	var newImageVersion = &InMemImageVersion{
		InMemPersistObj: *pers,
		Version: version,
		ImageObjId: imageObjId,
		ImageCreationEventId: creationEventId,
		CreationDate: creationDate,
	}
	
	return newImageVersion, nil
}

func (client *InMemClient) getImageVersion(id string) (ImageVersion, error) {
	
	var imageVersion ImageVersion
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("ImageVersion not found") }
	imageVersion, isType = obj.(ImageVersion)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + id + " is not a ImageVersion") }
	return imageVersion, nil
}

func (imageVersion *InMemImageVersion) writeBack(dbClient DBClient) error {
	panic("Abstract method should not be called")
}

func (imageVersion *InMemImageVersion) getFullName(dbClient DBClient) (string, error) {
	// See http://blog.thoward37.me/articles/where-are-docker-images-stored/
	var repo Repo
	var realm Realm
	var err error
	var image Image
	image, err = imageVersion.getImage(dbClient)
	if err != nil { return "", err }
	repo, err = dbClient.getRepo(image.getRepoId())
	if err != nil { return "", err }
	realm, err = dbClient.getRealm(repo.getRealmId())
	if err != nil { return "", err }
	
	var dockerImageName, tag string
	dockerImageName, tag = docker.ConstructDockerImageName(
		realm.getName(), repo.getName(), image.getName(), imageVersion.Version)
	return (dockerImageName + ":" + tag), nil
}

func (imageVersion *InMemImageVersion) getFullNameParts(dbClient DBClient) (
	string, string, string, string, error) {

	var repo Repo
	var realm Realm
	var err error
	var image Image
	image, err = imageVersion.getImage(dbClient)
	if err != nil { return "", "", "", "", err }
	repo, err = dbClient.getRepo(image.getRepoId())
	if err != nil { return "", "", "", "", err }
	realm, err = dbClient.getRealm(repo.getRealmId())
	if err != nil { return "", "", "", "", err }
	return realm.getName(), repo.getName(), image.getName(), imageVersion.Version, nil
}

func (imageVersion *InMemImageVersion) deleteAllChildResources(dbClient DBClient) error {
	
	var event Event
	var err error
	event, err = dbClient.getEvent(imageVersion.ImageCreationEventId)
	if err != nil { return err }

	var dockerfileExecEvent DockerfileExecEvent
	var isType bool
	dockerfileExecEvent, isType = event.(DockerfileExecEvent)
	if isType {
		err = dockerfileExecEvent.nullifyDockerfile(dbClient)
		if err != nil { return err }
	}
	
	return dbClient.writeBack(imageVersion)
}

func (imageVersion *InMemImageVersion) imageVersionFieldsAsJSON() string {
	return fmt.Sprintf("%s, \"Version\": \"%s\", \"ImageObjId\": \"%s\", " +
		"\"ImageCreationEventId\": \"%s\", \"CreationDate\": %s",
		imageVersion.persistObjFieldsAsJSON(), imageVersion.Version,
		imageVersion.ImageObjId, imageVersion.ImageCreationEventId,
		apitypes.FormatTimeAsJavascriptDate(imageVersion.CreationDate))
}

func (imageVersion *InMemImageVersion) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstituteImageVersion(id, version, imageObjId string,
	creationDate time.Time, imageCreationEventId string) (*InMemImageVersion, error) {
	
	var persObj *InMemPersistObj
	var err error
	persObj, err = client.ReconstitutePersistObj(id)
	if err != nil { return nil, err }
	return &InMemImageVersion{
		InMemPersistObj: *persObj,
		Version: version,
		ImageObjId: imageObjId,
		ImageCreationEventId: imageCreationEventId,
		CreationDate: creationDate,
	}, nil
}

func (imageVersion *InMemImageVersion) getVersion() string {
	return imageVersion.Version
}

func (imageVersion *InMemImageVersion) getImageObjId() string {
	return imageVersion.ImageObjId
}

func (imageVersion *InMemImageVersion) getImage(dbClient DBClient) (image Image, err error) {
	image, err = dbClient.getImage(imageVersion.ImageObjId)
	return image, err
}

func (imageVersion *InMemImageVersion) getCreationDate() time.Time {
	return imageVersion.CreationDate
}

func (imageVersion *InMemImageVersion) getImageCreationEventId() string {
	return imageVersion.ImageCreationEventId
}

func (imageVersion *InMemImageVersion) setImageCreationEventId(eventId string) {
	imageVersion.ImageCreationEventId = eventId
}

/*******************************************************************************
 * 
 */
type InMemDockerImageVersion struct {
	InMemImageVersion
	ScanEventIds []string
	Digest []byte
	Signature []byte
	DockerBuildOutput string
}

var _ DockerImageVersion = &InMemDockerImageVersion{}

func (client *InMemClient) NewInMemDockerImageVersion(version, imageObjId string,
	creationDate time.Time, buildOutput string,
	digest, signature []byte) (*InMemDockerImageVersion, error) {
	
	var imageVersion *InMemImageVersion
	var err error
	imageVersion, err = client.NewInMemImageVersion(version, imageObjId, "", creationDate)
	if err != nil { return nil, err }
	
	var newDockerImageVersion = &InMemDockerImageVersion{
		InMemImageVersion: *imageVersion,
		ScanEventIds: make([]string, 0),
		Digest: digest,
		Signature: signature,
		DockerBuildOutput: buildOutput,
	}

	return newDockerImageVersion, client.updateObject(newDockerImageVersion)
}

func (client *InMemClient) dbCreateDockerImageVersion(version, dockerImageObjId string,
	creationDate time.Time, buildOutput string,
	digest, signature []byte) (DockerImageVersion, error) {
	
	// Check if the image exists and is a DockerImage.
	var dockerImage DockerImage
	var err error
	dockerImage, err = client.getDockerImage(dockerImageObjId)
	if err != nil { return nil, err }
	
	var imageVersion *InMemDockerImageVersion
	imageVersion, err = client.NewInMemDockerImageVersion(version, dockerImageObjId,
		creationDate, buildOutput, digest, signature)
	if err != nil { return nil, err }
	
	// Add to Image.
	err = dockerImage.addVersionId(client, imageVersion.getId())
	if err != nil { return nil, err }
	return imageVersion, nil
}

func (client *InMemClient) getDockerImageVersion(id string) (DockerImageVersion, error) {
	
	var dockerImageVersion DockerImageVersion
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("DockerImageVersion not found") }
	dockerImageVersion, isType = obj.(DockerImageVersion)
	if ! isType { return nil, utils.ConstructUserError("Object with Id " + id + " is not a DockerImageVersion") }
	return dockerImageVersion, nil
}

func (imageVersion *InMemDockerImageVersion) getDockerImageTag() string {
	return imageVersion.Version
}

func (imageVersion *InMemDockerImageVersion) asDockerImageVersionDesc() *apitypes.DockerImageVersionDesc {
	return apitypes.NewDockerImageVersionDesc(imageVersion.getId(), imageVersion.Version,
		imageVersion.ImageObjId, imageVersion.ImageCreationEventId, imageVersion.CreationDate,
		imageVersion.Digest, imageVersion.Signature, imageVersion.ScanEventIds,
		imageVersion.DockerBuildOutput)
}

func (imageVersion *InMemDockerImageVersion) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(imageVersion)
}

func (imageVersion *InMemDockerImageVersion) asJSON() string {
	var json = "{" + imageVersion.imageVersionFieldsAsJSON()
	
	json = json + fmt.Sprintf(", \"%s\", \"ScanEventIds\": [", imageVersion.ImageCreationEventId)
	
	for i, id := range imageVersion.ScanEventIds {
		if i > 0 { json = json + ", " }
		json = json + id
	}
	json = json + "], \"Digest\": " + rest.ByteArrayAsJSON(imageVersion.Digest)
	json = json + ", \"Signature\": " + rest.ByteArrayAsJSON(imageVersion.Signature)
	json = json + ", \"DockerBuildOutput\": \"" +
		rest.EncodeStringForJSON(imageVersion.DockerBuildOutput) + "\"}"
	
	return json
}

func (client *InMemClient) ReconstituteDockerImageVersion(version, imageObjId string,
	creationDate time.Time, imageCreationEventId string, scanEventIds []string,
	digest, signature []byte, dockerBuildOutput string) (*InMemDockerImageVersion, error) {
	
	var imageVersion *InMemImageVersion
	var err error
	imageVersion, err = client.NewInMemImageVersion(
		version, imageObjId, imageCreationEventId, creationDate)
	if err != nil { return nil, err }
	
	return &InMemDockerImageVersion{
		InMemImageVersion: *imageVersion,
		ScanEventIds: scanEventIds,
		Digest: digest,
		Signature: signature,
		DockerBuildOutput: dockerBuildOutput,
	}, nil
}

func (imageVersion *InMemDockerImageVersion) addScanEventId(dbClient DBClient, id string) error {
	
	imageVersion.ScanEventIds = append(imageVersion.ScanEventIds, id)
	return dbClient.writeBack(imageVersion)
}

func (imageVersion *InMemDockerImageVersion) getScanEventIds() []string {
	return imageVersion.ScanEventIds
}

func (imageVersion *InMemDockerImageVersion) getMostRecentScanEventId() string {
	var numOfIds = len(imageVersion.ScanEventIds)
	if numOfIds == 0 {
		return ""
	} else {
		return imageVersion.ScanEventIds[numOfIds-1]
	}
}

func (imageVersion *InMemDockerImageVersion) getDigest() []byte {
	return imageVersion.Digest
}

func (imageVersion *InMemDockerImageVersion) getSignature() []byte {
	return imageVersion.Signature
}

/* ----- Not used anymore - we get the signature from the docker v2 registry -----
func (imageVersion *InMemDockerImageVersion) computeSignature() ([]byte, error) {
	var err error
	var tempFilePath string
	var imageFullName
	imageFullName, err = image.getFullName(dbClient)
	tempFilePath, err = docker.SaveImage(imageFullName)
	if err != nil { return nil, err }
	defer func() {
		fmt.Println("Removing all files at " + tempFilePath)
		os.RemoveAll(tempFilePath)
	}()
	var file *os.File
	file, _ = os.Open(tempFilePath)
	var fileInfo os.FileInfo
	fileInfo, _ = file.Stat()
	fmt.Println(fmt.Sprintf("Size of file %s is %d", tempFilePath, fileInfo.Size()))
	return image.Client.Server.authService.ComputeFileSignature(tempFilePath)
}
*/

func (imageVersion *InMemDockerImageVersion) getDockerBuildOutput() string {
	return imageVersion.DockerBuildOutput
}

/*******************************************************************************
 * 
 */
type InMemParameterValue struct {  // abstract
	InMemPersistObj
	Name string
	StringValue string
}

var _ ParameterValue = &InMemParameterValue{}

func (client *InMemClient) NewInMemParameterValue(name, value string) (*InMemParameterValue, error) {
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	var paramValue = &InMemParameterValue{
		InMemPersistObj: *pers,
		Name: name,
		StringValue: value,
	}
	return paramValue, nil
}

func (client *InMemClient) getParameterValue(id string) (ParameterValue, error) {
	var pv ParameterValue
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("ParameterValue not found") }
	pv, isType = obj.(ParameterValue)
	if ! isType { return nil, utils.ConstructServerError("Object with Id " + id + " is not a ParameterValue") }
	return pv, nil
}

func (paramValue *InMemParameterValue) writeBack(dbClient DBClient) error {
	panic("Abstract method should not be called")
}

func (paramValue *InMemParameterValue) getName() string {
	return paramValue.Name
}

//func (paramValue *InMemParameterValue) getTypeName() string {
//	return paramValue.TypeName
//}

func (paramValue *InMemParameterValue) getStringValue() string {
	return paramValue.StringValue
}

func (paramValue *InMemParameterValue) setStringValue(value string) {
	paramValue.StringValue = value
}

func (paramValue *InMemParameterValue) asParameterValueDesc() *apitypes.ParameterValueDesc {
	return apitypes.NewParameterValueDesc(paramValue.Name, //paramValue.TypeName,
		paramValue.StringValue)
}

func (paramValue *InMemParameterValue) parameterValueFieldsAsJSON() string {
	var json = "{" + paramValue.persistObjFieldsAsJSON()
	json = json + fmt.Sprintf("\"Name\": \"%s\", \"StringValue\": \"%s\"",
		paramValue.Name, paramValue.StringValue)
	return json
}

func (paramValue *InMemParameterValue) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstituteParameterValue(id string,
	name, strval string) (*InMemParameterValue, error) {

	var persistObj *InMemPersistObj
	var err error
	persistObj, err = client.ReconstitutePersistObj(id)
	if err != nil { return nil, err }
	
	return &InMemParameterValue{
		InMemPersistObj: *persistObj,
		Name: name,
		StringValue: strval,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemScanConfig struct {
	InMemResource
	SuccessExpression string
	ProviderName string
	ParameterValueIds []string
	FlagId string
	ScanEventIds []string
	DockerImageIdsThatUse []string
}

var _ ScanConfig = &InMemScanConfig{}

func (client *InMemClient) NewInMemScanConfig(name, desc, repoId,
	providerName string, paramValueIds []string, successExpr string,
	flagId string) (*InMemScanConfig, error) {
	
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	var scanConfig = &InMemScanConfig{
		InMemResource: *resource,
		SuccessExpression: successExpr,
		ProviderName: providerName,
		ParameterValueIds: paramValueIds,
		FlagId: flagId,
		ScanEventIds: make([]string, 0),
		DockerImageIdsThatUse: make([]string, 0),
	}
	return scanConfig, client.updateObject(scanConfig)
}

func (client *InMemClient) dbCreateScanConfig(name, desc, repoId,
	providerName string, paramValueIds []string, successExpr, flagId string) (ScanConfig, error) {
	
	// Check if a scanConfig with that name already exists within the repo.
	var repo Repo
	var err error
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	if repo == nil { return nil, utils.ConstructServerError(fmt.Sprintf(
		"Unidentified repo for repo Id %s", repoId))
	}
	var sc ScanConfig
	sc, err = repo.getScanConfigByName(client, name)
	if err != nil { return nil, err }
	if sc != nil { return nil, utils.ConstructUserError(
		fmt.Sprintf("ScanConfig named %s already exists within repo %s", name,
			repo.getName()))
	}
	
	//var scanConfigId string = createUniqueDbObjectId()
	var scanConfig *InMemScanConfig
	scanConfig, err = client.NewInMemScanConfig(name, desc, repoId, providerName,
		paramValueIds, successExpr, flagId)
	if flagId != "" {
		var flag Flag
		flag, err = client.getFlag(flagId)
		if err != nil { return nil, err }
		err = flag.addScanConfigRef(client, scanConfig.getId())
		if err != nil { return nil, err }
	}
	err = client.writeBack(scanConfig)
	if err != nil { return nil, err }
	
	// Link to repo
	repo.addScanConfig(client, scanConfig)
	
	fmt.Println("Created ScanConfig")
	return scanConfig, nil
}

func (client *InMemClient) getScanConfig(id string) (ScanConfig, error) {
	var scanConfig ScanConfig
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("ScanConfig not found") }
	scanConfig, isType = obj.(ScanConfig)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return scanConfig, nil
}

func (scanConfig *InMemScanConfig) getSuccessExpr() string {
	return scanConfig.SuccessExpression
}

func (scanConfig *InMemScanConfig) setSuccessExpression(dbClient DBClient, expr string) error {
	scanConfig.setSuccessExpressionDeferredUpdate(expr)
	return dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) setSuccessExpressionDeferredUpdate(expr string) {
	scanConfig.SuccessExpression = expr
}

func (scanConfig *InMemScanConfig) getRepoId() string {
	return scanConfig.ParentId
}

func (scanConfig *InMemScanConfig) getProviderName() string {
	return scanConfig.ProviderName
}

func (scanConfig *InMemScanConfig) setProviderName(dbClient DBClient, name string) error {
	scanConfig.setProviderNameDeferredUpdate(name)
	return dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) setProviderNameDeferredUpdate(name string) {
	scanConfig.ProviderName = name
}

func (scanConfig *InMemScanConfig) getParameterValueIds() []string {
	return scanConfig.ParameterValueIds
}

func (scanConfig *InMemScanConfig) addParameterValueId(dbClient DBClient, id string) {
	
	scanConfig.ParameterValueIds = append(scanConfig.ParameterValueIds, id)
}

func (scanConfig *InMemScanConfig) setParameterValue(dbClient DBClient, name, strValue string) (ParameterValue, error) {
	var paramValue ParameterValue
	var err error
	paramValue, err = scanConfig.setParameterValueDeferredUpdate(dbClient, name, strValue)
	if err != nil { return paramValue, err }
	err = dbClient.writeBack(paramValue)
	if err != nil { return paramValue, err }
	return paramValue, dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) setParameterValueDeferredUpdate(dbClient DBClient,
	name, strValue string) (ParameterValue, error) {
	
	// Check if a parameter value already exist for the parameter. If so, replace the value.
	for _, id := range scanConfig.ParameterValueIds {
		var pv ParameterValue
		var err error
		pv, err = dbClient.getParameterValue(id)
		if err != nil { return nil, err }
		if pv == nil {
			fmt.Println("Internal ERROR: broken ParameterValue list for scan config " + scanConfig.getName())
			continue
		}
		if pv.getName() == name {
			pv.setStringValue(strValue)
			return pv, nil
		}
	}
	
	// Did not find a value for a parameter of that name - create a new ParameterValue.
	var paramValue ParameterValue
	var err error
	paramValue, err = dbClient.dbCreateScanParameterValue(name, strValue, scanConfig.getId())
	if err != nil { return nil, err }
	return paramValue, nil
}

func (scanConfig *InMemScanConfig) deleteParameterValue(dbClient DBClient, name string) error {
	for i, id := range scanConfig.ParameterValueIds {
		var pv ParameterValue
		var err error
		pv, err = dbClient.getParameterValue(id)
		if err != nil { return err }
		if pv == nil {
			fmt.Println("Internal ERROR: broken ParameterValue list for scan config " + scanConfig.getName())
			continue
		}
		if pv.getName() == name {
			scanConfig.ParameterValueIds = apitypes.RemoveAt(i, scanConfig.ParameterValueIds)
			err = dbClient.deleteObject(pv)
			if err != nil { return err }
			return dbClient.writeBack(scanConfig)
		}
	}
	return utils.ConstructUserError("Did not find parameter named '" + name + "'")
}

func (scanConfig *InMemScanConfig) deleteAllParameterValues(dbClient DBClient) error {
	for _, paramValueId := range scanConfig.getParameterValueIds() {
		var err error
		var paramValue ParameterValue
		paramValue, err = dbClient.getParameterValue(paramValueId)
		if err != nil { return err }
		dbClient.deleteObject(paramValue)
	}
	scanConfig.ParameterValueIds = make([]string, 0)
	return dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) deleteAllChildResources(dbClient DBClient) error {
	
	for _, parameterValueId := range scanConfig.ParameterValueIds {
		
		var parameterValue ParameterValue
		var err error
		parameterValue, err = dbClient.getParameterValue(parameterValueId)
		if err != nil { return err }
		
		err = scanConfig.deleteParameterValue(dbClient, parameterValue.getName())
		if err != nil { return err }
	}
	
	return dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) setFlagId(dbClient DBClient, newFlagId string) error {
	if scanConfig.FlagId == newFlagId { return nil } // nothing to do
	var newFlag Flag
	var err error
	newFlag, err = dbClient.getFlag(newFlagId)
	if err != nil { return err }
	if scanConfig.FlagId != "" { // already set to a Flag - remove that one
		var oldFlag Flag
		oldFlag, err = dbClient.getFlag(scanConfig.FlagId)
		if err != nil { return err }
		oldFlag.removeScanConfigRef(dbClient, scanConfig.getId())
	}
	scanConfig.FlagId = newFlagId
	err = newFlag.addScanConfigRef(dbClient, scanConfig.getId())  // adds non-redundantly
	if err != nil { return err }
	return dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) getFlagId() string {
	return scanConfig.FlagId
}

func (scanConfig *InMemScanConfig) addScanEventId(dbClient DBClient, id string) {
	
	scanConfig.ScanEventIds = append(scanConfig.ScanEventIds, id)
	dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) getScanEventIds() []string {
	return scanConfig.ScanEventIds
}

func (scanConfig *InMemScanConfig) deleteScanEventId(dbClient DBClient, eventId string) error {
	scanConfig.ScanEventIds = apitypes.RemoveFrom(eventId, scanConfig.ScanEventIds)
	return dbClient.writeBack(scanConfig)
}

func (scanConfig *InMemScanConfig) addDockerImage(dbClient DBClient, dockerImageId string) error {
	
	scanConfig.DockerImageIdsThatUse = apitypes.AddUniquely(dockerImageId, scanConfig.DockerImageIdsThatUse)
	dockerImage.addScanConfigIdToList(scanConfig.getId())
	err = dbClient.updateObject(dockerImage)
	if err != nil { return err }
	err = dbClient.updateObject(scanConfig)
	if err != nil { return err }
}

func (scanConfig *InMemScanConfig) remDockerImage(dbClient DBClient, dockerImageId string) error {
	
	scanConfig.DockerImageIdsThatUse = apitypes.RemoveFrom(dockerImageId, scanConfig.DockerImageIdsThatUse)
	dockerImage.remScanConfigIdFromList(scanConfig.getId())
	err = dbClient.updateObject(dockerImage)
	if err != nil { return err }
	err = dbClient.updateObject(scanConfig)
	if err != nil { return err }
}

func (scanConfig *InMemScanConfig) getDockerImageIdsThatUse() []string {
	return scanConfig.DockerImageIdsThatUse
}

func (resource *InMemScanConfig) isScanConfig() bool {
	return true
}

func (scanConfig *InMemScanConfig) asScanConfigDesc(dbClient DBClient) *apitypes.ScanConfigDesc {
	
	var paramValueDescs []*apitypes.ScanParameterValueDesc = make([]*apitypes.ScanParameterValueDesc, 0)
	for _, valueId := range scanConfig.ParameterValueIds {
		var paramValue ScanParameterValue
		var err error
		paramValue, err = dbClient.getScanParameterValue(valueId)
		if err != nil {
			fmt.Println("Internal error: " + err.Error())
			continue
		}
		if paramValue == nil {
			fmt.Println("Internal error: Could not find ParameterValue with Id " + valueId)
			continue
		}
		paramValueDescs = append(paramValueDescs, paramValue.asScanParameterValueDesc(dbClient))
	}
	
	return apitypes.NewScanConfigDesc(scanConfig.Id, scanConfig.ProviderName,
		scanConfig.SuccessExpression, scanConfig.FlagId, paramValueDescs,
		scanConfig.DockerImageIdsThatUse)
}

func (scanConfig *InMemScanConfig) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(scanConfig)
}

func (scanConfig *InMemScanConfig) asJSON() string {
	
	var json = "\"ScanConfig\": {" + scanConfig.resourceFieldsAsJSON()
	json = json + fmt.Sprintf(
		", \"SuccessExpression\": \"%s\", \"ProviderName\": \"%s\", \"ParameterValueIds\": [",
		scanConfig.SuccessExpression, scanConfig.ProviderName)
	for i, id := range scanConfig.ParameterValueIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + fmt.Sprintf("], \"FlagId\": \"%s\", \"ScanEventIds\": [", scanConfig.FlagId)
	for i, id := range scanConfig.ScanEventIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "], \"DockerImageIdsThatUse\": ["
	for i, id := range scanConfig.DockerImageIdsThatUse {
		if i > 0 { json = json + ", " }
		json = json + fmt.Sprintf("\"%s\"", id)
	}
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteScanConfig(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time,
	successExpr string, providerName string, paramValueIds []string,
	flagId string, scanEventIds, dockerImageIdsThatUse []string) (*InMemScanConfig, error) {

	var resource *InMemResource
	var err error
	resource, err = client.ReconstituteResource(id, aclEntryIds, name, desc, parentId,
		creationTime)
	if err != nil { return nil, err }

	return &InMemScanConfig{
		InMemResource: *resource,
		SuccessExpression: successExpr,
		ProviderName: providerName,
		ParameterValueIds: paramValueIds,
		FlagId: flagId,
		ScanEventIds: scanEventIds,
		DockerImageIdsThatUse: dockerImageIdsThatUse,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemScanParameterValue struct {
	InMemParameterValue
	ConfigId string
}

var _ ScanParameterValue = &InMemScanParameterValue{}

func (client *InMemClient) NewInMemScanParameterValue(name, value, configId string) (*InMemScanParameterValue, error) {
	
	var paramValue *InMemParameterValue
	var err error
	paramValue, err = client.NewInMemParameterValue(name, value)
	if err != nil { return nil, err }
	var scanParamValue = &InMemScanParameterValue{
		InMemParameterValue: *paramValue,
		ConfigId: configId,
	}
	return scanParamValue, client.updateObject(scanParamValue)
}

func (client *InMemClient) dbCreateScanParameterValue(name, value, configId string) (ScanParameterValue, error) {
	
	var scanConfig ScanConfig
	var err error
	scanConfig, err = client.getScanConfig(configId)
	if err != nil { return nil, err }
	
	var paramValue ScanParameterValue
	paramValue, err = client.NewInMemScanParameterValue(name, value, configId)
	if err != nil { return nil, err }
	scanConfig.addParameterValueId(client, paramValue.getId())
	return paramValue, nil
}

func (client *InMemClient) getScanParameterValue(id string) (ScanParameterValue, error) {
	var pv ParameterValue
	var err error
	pv, err = client.getParameterValue(id)
	if err != nil { return nil, err }
	var scanpv ScanParameterValue
	var isType bool
	scanpv, isType = pv.(ScanParameterValue)
	if ! isType { return nil, utils.ConstructServerError("ParameterValue is not a ScanParameterValue") }
	return scanpv, nil
}

func (client *InMemClient) ReconstituteScanParameterValue(id,
	name, strval, configId string)  (*InMemScanParameterValue, error) {

	var paramValue *InMemParameterValue
	var err error
	paramValue, err = client.ReconstituteParameterValue(id, name, strval)
	if err != nil { return nil, err }
	
	return &InMemScanParameterValue{
		InMemParameterValue: *paramValue,
		ConfigId: configId,
	}, nil
}

func (client *InMemClient) asScanParameterValueDesc(paramValue ScanParameterValue) *apitypes.ScanParameterValueDesc {
	return paramValue.asScanParameterValueDesc(client)
}

func (paramValue *InMemScanParameterValue) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(paramValue)
}

func (paramValue *InMemScanParameterValue) asScanParameterValueDesc(dbClient DBClient) *apitypes.ScanParameterValueDesc {
	return apitypes.NewScanParameterValueDesc(paramValue.Name, paramValue.StringValue,
		paramValue.ConfigId)
}

func (paramValue *InMemScanParameterValue) scanParameterValueFieldsAsJSON() string {
	var json = "{" + paramValue.parameterValueFieldsAsJSON()
	json = json + fmt.Sprintf(", \"ConfigId\": \"%s\"}", paramValue.ConfigId)
	return json
}

func (paramValue *InMemScanParameterValue) asJSON() string {
	return "{" + paramValue.scanParameterValueFieldsAsJSON() + "}"
}

func (paramValue *InMemScanParameterValue) getConfigId() string {
	return paramValue.ConfigId
}

/*******************************************************************************
 * 
 */
type InMemFlag struct {
	InMemResource
	SuccessImagePath string
	UsedByScanConfigIds []string
}

var _ Flag = &InMemFlag{}

func (client *InMemClient) NewInMemFlag(name, desc, repoId,
	successImagePath string) (*InMemFlag, error) {
	
	var resource *InMemResource
	var err error
	resource, err = client.NewInMemResource(name, desc, repoId)
	if err != nil { return nil, err }
	var flag = &InMemFlag{
		InMemResource: *resource,
		SuccessImagePath: successImagePath,
		UsedByScanConfigIds: make([]string, 0),
	}
	return flag, client.updateObject(flag)
}

func (client *InMemClient) dbCreateFlag(name, desc, repoId, successImagePath string) (Flag, error) {
	var flag Flag
	var err error
	flag, err = client.NewInMemFlag(name, desc, repoId, successImagePath)
	if err != nil { return nil, err }
	
	var repo Repo
	repo, err = client.getRepo(repoId)
	if err != nil { return nil, err }
	
	// Add to repo's list of flags.
	err = repo.addFlag(client, flag)
	if err != nil { return nil, err }

	// Make persistent.
	err = client.writeBack(flag)
	
	return flag, err
}

func (client *InMemClient) getFlag(id string) (Flag, error) {
	var flag Flag
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Flag not found") }
	flag, isType = obj.(Flag)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return flag, nil
}

func (flag *InMemFlag) getRepoId() string {
	return flag.ParentId
}

func (flag *InMemFlag) setSuccessImagePath(path string) {
	flag.SuccessImagePath = path
}

func (flag *InMemFlag) getSuccessImagePath() string {
	return flag.SuccessImagePath
}

func (flag *InMemFlag) getSuccessImageURL() string {
	var baseURL = flag.Persistence.Server.GetBasePublicURL()
	baseURL = strings.TrimRight(baseURL, "/")
	return baseURL + "/getFlagImage/?Id=" + flag.getId()
}

func (flag *InMemFlag) addScanConfigRef(dbClient DBClient, scanConfigId string) error {
	flag.UsedByScanConfigIds = apitypes.AddUniquely(scanConfigId, flag.UsedByScanConfigIds)
	return dbClient.writeBack(flag)
}

func (flag *InMemFlag) removeScanConfigRef(dbClient DBClient, scanConfigId string) error {
	flag.UsedByScanConfigIds = apitypes.RemoveFrom(scanConfigId, flag.UsedByScanConfigIds)
	
	return dbClient.writeBack(flag)
}

func (flag *InMemFlag) usedByScanConfigIds() []string {
	return flag.UsedByScanConfigIds
}

func (resource *InMemFlag) isFlag() bool {
	return true
}

func (flag *InMemFlag) deleteAllChildResources(dbClient DBClient) error {
	return nil
}

func (flag *InMemFlag) asFlagDesc() *apitypes.FlagDesc {
	return apitypes.NewFlagDesc(flag.getId(), flag.getRepoId(), flag.getName(),
		flag.getSuccessImageURL())
}

func (flag *InMemFlag) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(flag)
}

func (flag *InMemFlag) asJSON() string {
	
	var json = "\"Flag\": {" + flag.resourceFieldsAsJSON()
	json = json + fmt.Sprintf(", \"SuccessImagePath\": \"%s\", \"UsedByScanConfigIds\": [",
		flag.SuccessImagePath)
	for i, id := range flag.UsedByScanConfigIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteFlag(id string, aclEntryIds []string,
	name, desc, parentId string, creationTime time.Time,
	successImagePath string, usedByScanConfigIds []string) (*InMemFlag, error) {

	var resource *InMemResource
	var err error
	resource, err = client.ReconstituteResource(id, aclEntryIds, name, desc, parentId,
		creationTime)
	if err != nil { return nil, err }

	return &InMemFlag{
		InMemResource: *resource,
		SuccessImagePath: successImagePath,
		UsedByScanConfigIds: usedByScanConfigIds,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemEvent struct {  // abstract
	InMemPersistObj
	When time.Time
	UserObjId string
}

var _ Event = &InMemEvent{}

func (client *InMemClient) NewInMemEvent(userObjId string) (*InMemEvent, error) {
	var pers *InMemPersistObj
	var err error
	pers, err = client.NewInMemPersistObj()
	if err != nil { return nil, err }
	return &InMemEvent{
		InMemPersistObj: *pers,
		When: time.Now(),
		UserObjId: userObjId,
	}, nil
}

func (client *InMemClient) getEvent(id string) (Event, error) {
	var event Event
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructUserError("Event not found") }
	event, isType = obj.(Event)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return event, nil
}

func (event *InMemEvent) getWhen() time.Time {
	return event.When
}

func (event *InMemEvent) getUserObjId() string {
	return event.UserObjId
}

func (event *InMemEvent) writeBack(dbClient DBClient) error {
	panic("Abstract method should not be called")
}

func (event *InMemEvent) asEventDesc(dbClient DBClient) apitypes.EventDesc {
	panic("Abstract method should not be called")
}

func (event *InMemEvent) eventFieldsAsJSON() string {
	var json = event.persistObjFieldsAsJSON()
	json = json + fmt.Sprintf(", \"When\": time \"%s\", \"UserObjId\": \"%s\"",
		apitypes.FormatTimeAsJavascriptDate(event.When), event.UserObjId)
	return json
}

func (event *InMemEvent) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) ReconstituteEvent(id string, when time.Time,
	userObjId string) (*InMemEvent, error) {

	var persistObj *InMemPersistObj
	var err error
	persistObj, err = client.ReconstitutePersistObj(id)
	if err != nil { return nil, err }
	
	return &InMemEvent{
		InMemPersistObj: *persistObj,
		When: when,
		UserObjId: userObjId,
	}, nil
}

func (client *InMemClient) asEventDesc(event Event) apitypes.EventDesc {
	var scanEvent ScanEvent
	var isType bool
	scanEvent, isType = event.(ScanEvent)
	if isType {
		return scanEvent.asEventDesc(client)
	} else {
		var dockerfileExecEvent DockerfileExecEvent
		dockerfileExecEvent, isType = event.(DockerfileExecEvent)
		if isType {
			return dockerfileExecEvent.asEventDesc(client)
		} else {
			panic("Unexpected event type: " + reflect.TypeOf(event).String())
		}
	}
	//return apitypes.NewEventDesc(event.getId(), event.getWhen(), event.getUserObjId())
}

/*******************************************************************************
 * 
 */
type InMemScanEvent struct {
	InMemEvent
	ScanConfigId string
	DockerImageVersionId string
	ProviderName string
	ActualParameterValueIds []string
	Score string
	Result providers.ScanResult
}

var _ ScanEvent = &InMemScanEvent{}

func (client *InMemClient) NewInMemScanEvent(scanConfigId, imageVersionId, userObjId,
	providerName string, score string, result *providers.ScanResult,
	actParamValueIds []string) (*InMemScanEvent, error) {
	
	var event *InMemEvent
	var err error
	event, err = client.NewInMemEvent(userObjId)
	if err != nil { return nil, err }
	var scanEvent *InMemScanEvent = &InMemScanEvent{
		InMemEvent: *event,
		ScanConfigId: scanConfigId,
		DockerImageVersionId: imageVersionId,
		ProviderName: providerName,
		ActualParameterValueIds: actParamValueIds,
		Score: score,
		Result: *result,
	}
	return scanEvent, client.updateObject(scanEvent)
}

func (client *InMemClient) dbCreateScanEvent(scanConfigId, providerName string,
	paramNames, paramValues []string, imageVersionId,
	userObjId, score string, result *providers.ScanResult) (ScanEvent, error) {
	
	// Create actual ParameterValues for the Event.
	var err error
	var actParamValueIds []string = make([]string, 0)
	for i, name := range paramNames {
		var actParamValue *InMemScanParameterValue
		actParamValue, err = client.NewInMemScanParameterValue(name, paramValues[i], scanConfigId)
		if err != nil { return nil, err }
		actParamValueIds = append(actParamValueIds, actParamValue.getId())
	}

	var scanConfig ScanConfig
	scanConfig, err = client.getScanConfig(scanConfigId)
	if err != nil { return nil, err }

	var scanEvent *InMemScanEvent
	scanEvent, err = client.NewInMemScanEvent(scanConfigId, imageVersionId, userObjId,
		providerName, score, result, actParamValueIds)
	if err != nil { return nil, err }
	err = client.writeBack(scanEvent)
	if err != nil { return nil, err }
	
	// Link to user.
	var user User
	user, err = client.getUser(userObjId)
	if err != nil { return nil, err }
	user.addEventId(client, scanEvent.getId())
	
	// Link to ScanConfig.
	scanConfig.addScanEventId(client, scanEvent.getId())
	
	// Link to ImageVersion.
	var imageVersion DockerImageVersion
	imageVersion, err = client.getDockerImageVersion(imageVersionId)
	if err != nil { return nil, err }
	imageVersion.addScanEventId(client, scanEvent.getId())

	fmt.Println("Created ScanEvent")
	return scanEvent, nil
}

func (client *InMemClient) getScanEvent(id string) (ScanEvent, error) {
	var scanEvent ScanEvent
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructServerError("ScanEvent not found") }
	scanEvent, isType = obj.(ScanEvent)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return scanEvent, nil
}

func (event *InMemScanEvent) getScore() string {
	return event.Score
}

func (event *InMemScanEvent) getDockerImageVersionId() string {
	return event.DockerImageVersionId
}

func (event *InMemScanEvent) getScanConfigId() string {
	return event.ScanConfigId
}

func (event *InMemScanEvent) getActualParameterValueIds() []string {
	return event.ActualParameterValueIds
}

func (event *InMemScanEvent) deleteAllParameterValues(dbClient DBClient) error {
	for _, paramId := range event.ActualParameterValueIds {
		var param ParameterValue
		var err error
		param, err = dbClient.getParameterValue(paramId)
		if err != nil { return err }
		dbClient.deleteObject(param)
	}
	event.ActualParameterValueIds = make([]string, 0)
	return dbClient.writeBack(event)
}

func (event *InMemScanEvent) nullifyDockerImageVersion(dbClient DBClient) error {
	event.DockerImageVersionId = ""
	return dbClient.writeBack(event)
}

func (event *InMemScanEvent) nullifyScanConfig(dbClient DBClient) error {
	event.ScanConfigId = ""
	return dbClient.writeBack(event)
}

func (event *InMemScanEvent) asScanEventDesc(dbClient DBClient) *apitypes.ScanEventDesc {
	
	var paramValueDescs []*apitypes.ScanParameterValueDesc = make([]*apitypes.ScanParameterValueDesc, 0)
	for _, valueId := range event.ActualParameterValueIds {
		var value ScanParameterValue
		var err error
		value, err = dbClient.getScanParameterValue(valueId)
		if err != nil {
			fmt.Println("Internal error:", err.Error())
			continue
		}
		paramValueDescs = append(paramValueDescs, value.asScanParameterValueDesc(dbClient))
	}
	
	return apitypes.NewScanEventDesc(event.Id, event.When, event.UserObjId,
		event.DockerImageVersionId, event.ScanConfigId, event.ProviderName, paramValueDescs,
		event.Score, event.Result.Vulnerabilities)
}

func (event *InMemScanEvent) asEventDesc(dbClient DBClient) apitypes.EventDesc {
	return event.asScanEventDesc(dbClient)
}

func (event *InMemScanEvent) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(event)
}

func (event *InMemScanEvent) asJSON() string {

	var json = "\"ScanEvent\": {" + event.eventFieldsAsJSON()
	json = json + fmt.Sprintf(
		", \"ScanConfigId\": \"%s\", \"DockerImageVersionId\": \"%s\", " +
		"\"ProviderName\": \"%s\", \"ActualParameterValueIds\": [",
		event.ScanConfigId, event.DockerImageVersionId, event.ProviderName)
	for i, id := range event.ActualParameterValueIds {
		if i != 0 { json = json + ", " }
		json = json + "\"" + id + "\""
	}
	json = json + fmt.Sprintf("], \"Score\": \"%s\", \"Result\": [", event.Score)
	
	// Serialize the Result field in a manner that we can later deserialize: as
	// an array of string arrays, where each string array contains the fields
	// of a providers.Vulnerability.
	for i, vuln := range event.Result.Vulnerabilities {
		if i > 0 { json = json + ", " }
		json = json + fmt.Sprintf("[\"%s\", \"%s\", \"%s\", \"%s\"]",
			vuln.VCE_ID, vuln.Link, vuln.Priority, vuln.Description)
	}
	
	json = json + "]}"
	return json
}

func (client *InMemClient) ReconstituteScanEvent(id string, when time.Time,
	userObjId string, scanConfigId, imageVersionId, providerName string,
	actParamValueIds []string, score string,
	vulnAr [][]string) (*InMemScanEvent, error) {

	var event *InMemEvent
	var err error
	event, err = client.ReconstituteEvent(id, when, userObjId)
	if err != nil { return nil, err }
	
	// Deserialize the json for the Result.
	var result  = &providers.ScanResult{
		Vulnerabilities: []*apitypes.VulnerabilityDesc{
			&apitypes.VulnerabilityDesc{ "", "", "", "" },
		},
	}
	for i, vuln := range vulnAr {
		result.Vulnerabilities[i] = &apitypes.VulnerabilityDesc{
				VCE_ID: vuln[0],
				Link: vuln[1],
				Priority: vuln[2],
				Description: vuln[3],
			}
	}
	
	return &InMemScanEvent{
		InMemEvent: *event,
		ScanConfigId: scanConfigId,
		DockerImageVersionId: imageVersionId,
		ProviderName: providerName,
		ActualParameterValueIds: actParamValueIds,
		Score: score,
		Result: *result,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemImageCreationEvent struct {  // abstract
	InMemEvent
	ImageVersionId string
}

var _ ImageCreationEvent = &InMemImageCreationEvent{}

func (client *InMemClient) NewInMemImageCreationEvent(userObjId, 
	imageVersionId string) (*InMemImageCreationEvent, error) {
	
	if imageVersionId == "" { fmt.Println("imageVersionId is nil"); panic("imageVersionId is nil") }
	
	
	var event *InMemEvent
	var err error
	event, err = client.NewInMemEvent(userObjId)
	if err != nil { return nil, err }
	return &InMemImageCreationEvent{
		InMemEvent: *event,
		ImageVersionId: imageVersionId,
	}, nil
}

func (event *InMemImageCreationEvent) writeBack(dbClient DBClient) error {
	panic("Abstract method should not be called")
}

func (event *InMemImageCreationEvent) nullifyImageVersion() {
	event.ImageVersionId = ""
}

func (event *InMemImageCreationEvent) getImageVersionId() string {
	return event.ImageVersionId
}

func (event *InMemImageCreationEvent) imageCreationEventFieldsAsJSON() string {
	var json = event.eventFieldsAsJSON()
	json = json + fmt.Sprintf(", \"ImageVersionId\": \"%s\"", event.ImageVersionId)
	return json
}

func (event *InMemImageCreationEvent) asJSON() string {
	panic("Call to method that should be abstract")
}

func (client *InMemClient) getImageCreationEvent(id string) (ImageCreationEvent, error) {
	var imageCreationEvent ImageCreationEvent
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructServerError("ImageCreationEvent not found") }
	imageCreationEvent, isType = obj.(ImageCreationEvent)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return imageCreationEvent, nil
}

func (client *InMemClient) ReconstituteImageCreationEvent(id string, when time.Time,
	userObjId string, imageVersionId string) (*InMemImageCreationEvent, error) {
	
	var event *InMemEvent
	var err error
	event, err = client.ReconstituteEvent(id, when, userObjId)
	if err != nil { return nil, err }
	
	return &InMemImageCreationEvent{
		InMemEvent: *event,
		ImageVersionId: imageVersionId,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemDockerfileExecEvent struct {
	InMemImageCreationEvent
	DockerfileId string
	ActualParameterValueIds []string
	DockerfileContent string
}

var _ DockerfileExecEvent = &InMemDockerfileExecEvent{}

func (client *InMemClient) NewInMemDockerfileExecEvent(dockerfileId, imageVersionId,
	userObjId string, actParamValueIds []string) (*InMemDockerfileExecEvent, error) {
	
	var ev *InMemImageCreationEvent
	var err error
	ev, err = client.NewInMemImageCreationEvent(userObjId, imageVersionId)
	if err != nil { return nil, err }
	
	var dockerfile Dockerfile
	dockerfile, err = client.getDockerfile(dockerfileId)
	if err != nil { return nil, err }
	
	var file *os.File
	file, err = os.Open(dockerfile.getExternalFilePath())
	if err != nil { return nil, err }
	var bytes []byte
	bytes, err = ioutil.ReadAll(file)
	if err != nil { return nil, err }
	var dockerfileContent = string(bytes)
	
	var event = &InMemDockerfileExecEvent{
		InMemImageCreationEvent: *ev,
		DockerfileId: dockerfileId,
		ActualParameterValueIds: actParamValueIds,
		DockerfileContent: dockerfileContent,
	}
	return event, client.updateObject(event)
}

func (client *InMemClient) dbCreateDockerfileExecEvent(dockerfileId string, 
	paramNames, paramValues []string, imageVersionId,
	userObjId string) (DockerfileExecEvent, error) {
	
	// Create actual ParameterValues for the Event.
	var err error
	var actParamValueIds []string = make([]string, 0)
	for i, name := range paramNames {
		var actParamValue *InMemDockerfileExecParameterValue
		actParamValue, err = client.NewInMemDockerfileExecParameterValue(
			name, paramValues[i], dockerfileId)
		if err != nil { return nil, err }
		actParamValueIds = append(actParamValueIds, actParamValue.getId())
	}
	
	var newDockerfileExecEvent *InMemDockerfileExecEvent
	newDockerfileExecEvent, err =
		client.NewInMemDockerfileExecEvent(dockerfileId, imageVersionId, userObjId, actParamValueIds)
	if err != nil { return nil, err }
	
	// Link with ImageVersion.
	var imageVersion DockerImageVersion
	imageVersion, err = client.getDockerImageVersion(imageVersionId)
	if err != nil { return nil, err }
	imageVersion.setImageCreationEventId(newDockerfileExecEvent.getId())
	
	// Link to Dockerfile.
	var dockerfile Dockerfile
	dockerfile, err = client.getDockerfile(dockerfileId)
	if err != nil { return nil, err }
	dockerfile.addEventId(client, newDockerfileExecEvent.getId())
	
	// Link to user.
	var user User
	user, err = client.getUser(userObjId)
	if err != nil { return nil, err }
	user.addEventId(client, newDockerfileExecEvent.getId())
	
	return newDockerfileExecEvent, nil
}

func (execEvent *InMemDockerfileExecEvent) getDockerfileId() string {
	return execEvent.DockerfileId
}

func (execEvent *InMemDockerfileExecEvent) getActualParameterValueIds() []string {
	return execEvent.ActualParameterValueIds
}

func (execEvent *InMemDockerfileExecEvent) deleteAllParameterValues(dbClient DBClient) error {
	for _, paramId := range execEvent.ActualParameterValueIds {
		var param ParameterValue
		var err error
		param, err = dbClient.getParameterValue(paramId)
		if err != nil { return err }
		dbClient.deleteObject(param)
	}
	execEvent.ActualParameterValueIds = make([]string, 0)
	return dbClient.writeBack(execEvent)
}

func (execEvent *InMemDockerfileExecEvent) getDockerfileContent() string {
	return execEvent.DockerfileContent
}

//func (execEvent *InMemDockerfileExecEvent) getDockerfileExternalObjId() string {
//	return execEvent.DockerfileExternalObjId
//}

func (execEvent *InMemDockerfileExecEvent) nullifyDockerfile(dbClient DBClient) error {
	execEvent.DockerfileId = ""
	//execEvent.DockerfileExternalObjId = ""
	return dbClient.writeBack(execEvent)
}

func (execEvent *InMemDockerfileExecEvent) nullifyDockerImageVersion(dbClient DBClient) error {
	execEvent.ImageVersionId = ""
	return dbClient.writeBack(execEvent)
}

func (execEvent *InMemDockerfileExecEvent) asDockerfileExecEventDesc(dbClient DBClient) *apitypes.DockerfileExecEventDesc {
	var paramValueDescs []*apitypes.DockerfileExecParameterValueDesc = make([]*apitypes.DockerfileExecParameterValueDesc, 0)
	for _, valueId := range execEvent.ActualParameterValueIds {
		var value DockerfileExecParameterValue
		var err error
		value, err = dbClient.getDockerfileExecParameterValue(valueId)
		if err != nil {
			fmt.Println("Internal error:", err.Error())
			continue
		}
		paramValueDescs = append(paramValueDescs, value.asDockerfileExecParameterValueDesc(dbClient))
	}
	
	return apitypes.NewDockerfileExecEventDesc(execEvent.getId(), execEvent.When,
		execEvent.UserObjId, execEvent.ImageVersionId, execEvent.DockerfileId, paramValueDescs, execEvent.DockerfileContent)
}

func (execEvent *InMemDockerfileExecEvent) asEventDesc(dbClient DBClient) apitypes.EventDesc {
	return execEvent.asDockerfileExecEventDesc(dbClient)
}

func (execEvent *InMemDockerfileExecEvent) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(execEvent)
}

func (execEvent *InMemDockerfileExecEvent) asJSON() string {
	
	var json = "\"DockerfileExecEvent\": {" + execEvent.imageCreationEventFieldsAsJSON()
	json = json + fmt.Sprintf(", \"DockerfileId\": \"%s\"", execEvent.DockerfileId)
	json = json + fmt.Sprintf(", \"DockerfileContent\": \"%s\"}",
		rest.EncodeStringForJSON(execEvent.DockerfileContent))
	return json
}

func (client *InMemClient) getDockerfileExecEvent(id string) (DockerfileExecEvent, error) {
	var dockerfileExecEvent DockerfileExecEvent
	var isType bool
	var obj PersistObj
	var err error
	obj, err = client.getPersistentObject(id)
	if err != nil { return nil, err }
	if obj == nil { return nil, utils.ConstructServerError("DockerfileExecEvent not found") }
	dockerfileExecEvent, isType = obj.(DockerfileExecEvent)
	if ! isType { return nil, utils.ConstructServerError("Internal error: object is an unexpected type") }
	return dockerfileExecEvent, nil
}

func (client *InMemClient) ReconstituteDockerfileExecEvent(id string, when time.Time,
	userObjId, imageId, dockerfileId, dockerfileContent string) (*InMemDockerfileExecEvent, error) {

	var imgCrEvent *InMemImageCreationEvent
	var err error
	imgCrEvent, err = client.ReconstituteImageCreationEvent(id, when, userObjId, imageId)
	if err != nil { return nil, err }
	
	return &InMemDockerfileExecEvent{
		InMemImageCreationEvent: *imgCrEvent,
		DockerfileId: dockerfileId,
		DockerfileContent: dockerfileContent,
	}, nil
}

/*******************************************************************************
 * 
 */
type InMemDockerfileExecParameterValue struct {
	InMemParameterValue
	DockerfileId string
}

var _ DockerfileExecParameterValue = &InMemDockerfileExecParameterValue{}

func (client *InMemClient) NewInMemDockerfileExecParameterValue(name, value,
	dockerfileId string) (*InMemDockerfileExecParameterValue, error) {
	
	var paramValue *InMemParameterValue
	var err error
	paramValue, err = client.NewInMemParameterValue(name, value)
	if err != nil { return nil, err }
	var execParamValue = &InMemDockerfileExecParameterValue{
		InMemParameterValue: *paramValue,
		DockerfileId: dockerfileId,
	}
	return execParamValue, client.updateObject(execParamValue)
}

func (client *InMemClient) dbCreateDockerfileExecParameterValue(name, value,
	dockerfileId string) (DockerfileExecParameterValue, error) {
	
	var paramValue DockerfileExecParameterValue
	var err error
	paramValue, err = client.NewInMemDockerfileExecParameterValue(name, value, dockerfileId)
	if err != nil { return nil, err }
	return paramValue, nil
}

func (client *InMemClient) getDockerfileExecParameterValue(id string) (DockerfileExecParameterValue, error) {
	var pv ParameterValue
	var err error
	pv, err = client.getParameterValue(id)
	if err != nil { return nil, err }
	var depv DockerfileExecParameterValue
	var isType bool
	depv, isType = pv.(DockerfileExecParameterValue)
	if ! isType { return nil, utils.ConstructServerError("ParameterValue is not a DockerfileExecParameterValue") }
	return depv, nil
}

func (client *InMemClient) ReconstituteDockerfileExecParameterValue(id,
	name, strval, dockerfileId string)  (*InMemDockerfileExecParameterValue, error) {
	
	var paramValue *InMemParameterValue
	var err error
	paramValue, err = client.ReconstituteParameterValue(id, name, strval)
	if err != nil { return nil, err }
	
	return &InMemDockerfileExecParameterValue{
		InMemParameterValue: *paramValue,
		DockerfileId: dockerfileId,
	}, nil
}

func (paramValue *InMemDockerfileExecParameterValue) getDockerfileId() string {
	return paramValue.DockerfileId
}

func (client *InMemClient) asDockerfileExecParameterValueDesc(paramValue DockerfileExecParameterValue) *apitypes.DockerfileExecParameterValueDesc {
	return paramValue.asDockerfileExecParameterValueDesc(client)
}

func (paramValue *InMemDockerfileExecParameterValue) writeBack(dbClient DBClient) error {
	return dbClient.updateObject(paramValue)
}

func (paramValue *InMemDockerfileExecParameterValue) asDockerfileExecParameterValueDesc(dbClient DBClient) *apitypes.DockerfileExecParameterValueDesc {
	return apitypes.NewDockerfileExecParameterValueDesc(paramValue.Name, paramValue.StringValue)
}

func (paramValue *InMemDockerfileExecParameterValue) dockerfileExecParameterValueFieldsAsJSON() string {
	var json = paramValue.parameterValueFieldsAsJSON()
	return json
}

func (paramValue *InMemDockerfileExecParameterValue) asJSON() string {
	var json = "{" + paramValue.dockerfileExecParameterValueFieldsAsJSON() + ", "
	json = json + fmt.Sprintf("\"DockerfileId\": \"%s\"}", paramValue.DockerfileId)
	return json
}

/*******************************************************************************
 * For test mode only.
 */
func (client *InMemClient) createTestObjects() {
	fmt.Println("Debug mode: creating realm testrealm")
	var realmInfo *apitypes.RealmInfo
	var err error
	realmInfo, err = apitypes.NewRealmInfo("testrealm", "Test Org", "For Testing")
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}
	var testRealm Realm
	testRealm, err = client.dbCreateRealm(realmInfo, "testuser1")
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}
	fmt.Println("Debug mode: creating user testuser1 in realm testrealm")
	var testUser1 User
	testUser1, err = client.dbCreateUser("testuser1", "Test User", 
		"testuser@gmail.com", "Password1", testRealm.getId())
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
	}
	fmt.Println("User", testUser1.getName())
	fmt.Println("created user, obj id=" + testUser1.getId())
	fmt.Println("Giving user admin access to the realm.")
	_, err = client.setAccess(testRealm, testUser1, []bool{true, true, true, true, true})
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
	}
}
