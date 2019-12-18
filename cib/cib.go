package cib

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	xmltree "github.com/beevik/etree"
	log "github.com/sirupsen/logrus"
)

type CIB struct {
	Doc *xmltree.Document
}

// Maximum number of CIB poll retries when waiting for CRM resources to stop
var maxWaitStopRetries = 10

// Delay between CIB polls in milliseconds
var cibPollRetryDelay = 2000 * time.Millisecond

var (
	ErrCibFailed = errors.New("Failed to read the CRM configuration. Maybe the cluster is not started on this node?")
)

// Pacemaker CIB attribute names
const (
	cibAttrKeyID           = "id"
	cibAttrKeyName         = "name"
	cibAttrKeyValue        = "value"
	cibAttrKeyOperation    = "operation"
	cibAttrKeyRcCode       = "rc-code"
	cibAttrValueTargetRole = "target-role"
	cibAttrValueStarted    = "Started"
	cibAttrValueStopped    = "Stopped"
	cibAttrValueStop       = "stop"
	cibAttrValueStart      = "start"
	cibAttrValueMonitor    = "monitor"
)

// Pacemaker OCF resource agent exit codes
const (
	ocfSuccess          = 0
	ocfErrGeneric       = 1
	ocfErrArgs          = 2
	ocfErrUnimplemented = 3
	ocfErrPerm          = 4
	ocfErrInstalled     = 5
	ocfNotRunning       = 7
	ocfRunningMaster    = 8
	ocfFailedMaster     = 9
)

// Pacemaker CIB XML tag names
const (
	cibTagLocation   = "rsc_location"
	cibTagColocation = "rsc_colocation"
	cibTagOrder      = "rsc_order"
	cibTagRscRef     = "resource_ref"
	cibTagMetaAttr   = "meta_attributes"
	cibTagInstAttr   = "instance_attributes"
	cibTagNvPair     = "nvpair"
	cibTagLrm        = "lrm"
	cibTagLrmRsclist = "lrm_resources"
	cibTagLrmRsc     = "lrm_resource"
	cibTagLrmRscOp   = "lrm_rsc_op"
)

type ClusterProperty string

const (
	StonithEnabled ClusterProperty = "cib-bootstrap-options-stonith-enabled"
)

// LrmRunState represents the state of a CRM resource.
type LrmRunState int

const (
	// Unknown means that the resource's state could not be retrieved
	Unknown LrmRunState = iota
	// Running means that the resource is verified as running
	Running
	// Stopped means that the resource is verfied as stopped
	Stopped
)

func (l LrmRunState) String() string {
	switch l {
	case Running:
		return "Running"
	case Stopped:
		return "Stopped"
	}
	return "Unknown"
}

func (l LrmRunState) MarshalJSON() ([]byte, error) { return json.Marshal(l.String()) }

// ReadConfiguration calls the crm list command and parses the XML data it returns.
func (c *CIB) ReadConfiguration() error {
	stdout, _, err := listCommand.execute("")
	if err != nil {
		// TODO maybe we can benefit from error wrapping here, but for
		// now this is good enough
		log.Error(err)
		return ErrCibFailed
	}

	c.Doc = xmltree.NewDocument()
	err = c.Doc.ReadFromString(stdout)
	if err != nil {
		return err
	}

	return nil
}

func (c *CIB) CreateResource(xml string) error {
	// Call cibadmin and pipe the CIB update data to the cluster resource manager
	_, _, err := createCommand.execute(xml)
	if err != nil {
		return err
	}

	return nil
}

func (c *CIB) SetStonithEnabled(value bool) error {
	return c.setClusterProperty(StonithEnabled, strconv.FormatBool(value))
}

func (c *CIB) setClusterProperty(prop ClusterProperty, value string) error {
	err := c.ReadConfiguration()
	if err != nil {
		return fmt.Errorf("could not read configuration: %w", err)
	}

	root := c.Doc.FindElement("/cib")
	if root == nil {
		return fmt.Errorf("invalid cib state: root element not found")
	}

	configuration := root.FindElement("configuration")
	if configuration == nil {
		configuration = root.CreateElement("configuration")
	}

	crmConfig := configuration.FindElement("crm_config")
	if crmConfig == nil {
		crmConfig = configuration.CreateElement("crm_config")
	}

	cps := crmConfig.FindElement("cluster_property_set[@id='cib-bootstrap-options']")
	if cps == nil {
		cps = crmConfig.CreateElement("cluster_property_set")
		cps.CreateAttr("id", "cib-bootstrap-options")
	}
	id := string(prop)
	elem := cps.FindElement("nvpair[@id='" + id + "']")
	if elem == nil {
		elem = cps.CreateElement(cibTagNvPair)
		elem.CreateAttr(cibAttrKeyID, id)
		name := id[len("cib-bootstrap-options-"):]
		elem.CreateAttr(cibAttrKeyName, name)
	}

	elem.CreateAttr(cibAttrKeyValue, value)

	err = c.Update()
	if err != nil {
		return fmt.Errorf("could not update CIB: %w", err)
	}
	return nil
}

func (c *CIB) StartResource(id string) error {
	return c.modifyTargetRole(id, true)
}

func (c *CIB) StopResource(id string) error {
	return c.modifyTargetRole(id, false)
}

// ModifyTargetRole sets the target-role of a resource in CRM.
//
// The id has to be a valid CRM resource identifier.
// A target-role of "Stopped" (startFlag == false) indicates to CRM that
// the it should stop the resource. A target role of "Started" (startFlag == true)
// indicates that the resource is already started and that CRM should not try
// to start it.
func (c *CIB) modifyTargetRole(id string, startFlag bool) error {
	// Process the CIB XML document tree and insert meta attributes for target-role=Stopped
	rscElem := c.FindResource(id)
	if rscElem == nil {
		return errors.New("CRM resource not found in the CIB, cannot modify role.")
	}

	var tgtRoleEntry *xmltree.Element
	metaAttr := rscElem.FindElement(cibTagMetaAttr)
	if metaAttr != nil {
		// Meta attributes exist, find the entry that sets the target-role
		tgtRoleEntry = metaAttr.FindElement(cibTagNvPair + "[@" + cibAttrKeyName + "='" + cibAttrValueTargetRole + "']")
	} else {
		// No meta attributes present, create XML element
		metaAttr = rscElem.CreateElement(cibTagMetaAttr)
		metaAttr.CreateAttr(cibAttrKeyID, id+"-meta_attributes")
	}
	if tgtRoleEntry == nil {
		// No nvpair entry that sets the target-role, create entry
		tgtRoleEntry = metaAttr.CreateElement(cibTagNvPair)
		tgtRoleEntry.CreateAttr(cibAttrKeyID, id+"-meta_attributes-target-role")
		tgtRoleEntry.CreateAttr(cibAttrKeyName, cibAttrValueTargetRole)
	}
	// Set the target-role
	var tgtRoleValue string
	if startFlag {
		tgtRoleValue = cibAttrValueStarted
	} else {
		tgtRoleValue = cibAttrValueStopped
	}
	tgtRoleEntry.CreateAttr(cibAttrKeyValue, tgtRoleValue)

	return nil
}

func (c *CIB) FindResource(id string) *xmltree.Element {
	return c.Doc.FindElement("//primitive[@id='" + id + "']")
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// WaitForResourcesStop waits for CRM resources to stop
//
// It returns a flag indicating whether resources are stopped (true) or
// not (false), and an error.
func (c *CIB) WaitForResourcesStop(idsToStop []string) (bool, error) {
	// Read the current CIB XML
	err := c.ReadConfiguration()
	if err != nil {
		return false, err
	}

	for _, id := range idsToStop {
		if c.FindResource(id) == nil {
			log.WithFields(log.Fields{
				"resource": id,
			}).Warning("Resource not found in the CIB, will be ignored.")
			idsToStop = remove(idsToStop, id)
		}
	}

	log.Debug("Waiting for the following CRM resources to stop:")
	for _, id := range idsToStop {
		log.Debugf("    %s", id)
	}

	isStopped := false
	retries := 0
	for !isStopped {
		// check if all resources are stopped
		allStopped := true
		for _, item := range idsToStop {
			state := c.FindLrmState(item)
			if state != Stopped {
				allStopped = false
				break
			}
		}

		if allStopped {
			// success; we stopped all resources
			isStopped = true
			break
		}

		if retries > maxWaitStopRetries {
			// timeout
			isStopped = false
			break
		}

		time.Sleep(cibPollRetryDelay)

		// Re-read the current CIB XML
		err = c.ReadConfiguration()
		if err != nil {
			return false, err
		}

		retries++
	}

	if isStopped {
		log.Debug("The resources are stopped")
	} else {
		log.Warning("Could not confirm that the resources are stopped")
	}

	return isStopped, nil
}

func GetNvPairValue(elem *xmltree.Element, name string) (*xmltree.Attr, error) {
	xpath := fmt.Sprintf("./instance_attributes/nvpair[@name='%s']", name)

	var nvpair *xmltree.Element
	if nvpair = elem.FindElement(xpath); nvpair == nil {
		return nil, errors.New("key not found")
	}

	var attr *xmltree.Attr
	if attr = nvpair.SelectAttr("value"); attr == nil {
		return nil, errors.New("value not found")
	}

	return attr, nil
}

func (c *CIB) FindLrmState(id string) LrmRunState {
	state := Unknown
	xpath := "cib/status/node_state/lrm/lrm_resources/lrm_resource[@id='" + id + "']"
	elems := c.Doc.FindElements(xpath)
	for _, elem := range elems {
		state = updateRunState(id, elem, state)
	}

	return state
}

func (c *CIB) Update() error {
	// Serialize the modified XML document tree into a string containing the XML document (CIB update data)
	cibData, err := c.Doc.WriteToString()
	if err != nil {
		return err
	}

	// Call cibadmin and pipe the CIB update data to the cluster resource manager
	_, _, err = updateCommand.execute(cibData)
	if err != nil {
		log.Warn("CRM command execution returned an error")
		log.Trace("The updated CIB data sent to the command was:")
		log.Trace(cibData)
	}

	return err
}

// Creates and returns a copy of a map[string]string
func copyMap(srcMap map[string]string) map[string]string {
	resultMap := make(map[string]string, len(srcMap))
	for key, value := range srcMap {
		resultMap[key] = value
	}
	return resultMap
}

// Removes CRM constraints that refer to the specified delItems names from the CIB XML document tree
func (c *CIB) DissolveConstraints(delItems []string) {
	// TODO: I think it's possible to to "XPath injection" here. Since
	// delItems is user controlled, inserting a ' or something could
	// potentially make the program panic. But let's worry about this
	// another day...

	xpaths := []string{
		// colocation references (if we had a better xpath library we could do this at once...)
		`configuration/constraints/rsc_colocation[@rsc='%s']`,
		`configuration/constraints/rsc_colocation[@with-rsc='%s']`,
		// order references
		`configuration/constraints/rsc_order[@first='%s']`,
		`configuration/constraints/rsc_order[@then='%s']`,
		// rsc_location -> resource_ref references
		`configuration/constraints/rsc_location/resource_set/resource_ref[@id='%s']/../..`,
		// rsc_location with direct rsc
		`configuration/constraints/rsc_location[@rsc='%s']`,
		// lrm status references
		`status/node_state/lrm/lrm_resources/lrm_resource[@id='%s']`,
	}

	for _, resourceName := range delItems {
		for _, xpathFormat := range xpaths {
			xpath := fmt.Sprintf(xpathFormat, resourceName)
			elems := c.Doc.Root().FindElements(xpath)
			for _, elem := range elems {
				parent := elem.Parent()
				if parent == nil {
					continue
				}
				parent.RemoveChild(elem)

				idAttr := elem.SelectAttr("id")
				if idAttr != nil {
					log.WithFields(log.Fields{
						"type": elem.Tag,
						"id":   idAttr.Value,
					}).Debug("Deleting dependency")
				}
			}
		}
	}
}

// updateRunState updates the run state information of a single resource
//
// For a resource to be considered stopped, this function must find
// - either a successful stop action
// - or a monitor action with rc-code ocfNotRunning and no stop action
//
// If a stop action is present, the monitor action can still show "running"
// (rc-code ocfSuccess == 0) although the resource is actually stopped. The
// monitor action's rc-code is only interesting if there is no stop action present.
func updateRunState(rscName string, lrmRsc *xmltree.Element, runState LrmRunState) LrmRunState {
	contextLog := log.WithFields(log.Fields{"resource": rscName})
	newRunState := runState
	stopEntry := lrmRsc.FindElement(cibTagLrmRscOp + "[@" + cibAttrKeyOperation + "='" + cibAttrValueStop + "']")
	if stopEntry != nil {
		rc, err := getLrmRcCode(stopEntry)
		if err != nil {
			contextLog.Warning(err)
		} else if rc == ocfSuccess {
			if newRunState == Unknown {
				newRunState = Stopped
			}
		} else {
			newRunState = Running
		}

		return newRunState
	}

	monEntry := lrmRsc.FindElement(cibTagLrmRscOp + "[@" + cibAttrKeyOperation + "='" + cibAttrValueMonitor + "']")
	if monEntry != nil {
		rc, err := getLrmRcCode(monEntry)
		if err != nil {
			contextLog.Warning(err)
		} else if rc == ocfNotRunning {
			if newRunState == Unknown {
				newRunState = Stopped
			}
		} else {
			newRunState = Running
		}

		return newRunState
	}

	startEntry := lrmRsc.FindElement(cibTagLrmRscOp + "[@" + cibAttrKeyOperation + "='" + cibAttrValueStart + "']")
	if startEntry != nil {
		rc, err := getLrmRcCode(startEntry)
		if err != nil {
			contextLog.Warning(err)
		} else if rc == ocfRunningMaster || rc == ocfSuccess {
			if newRunState == Unknown {
				newRunState = Running
			}
		} else {
			newRunState = Stopped
		}

		return newRunState
	}

	return newRunState
}

// getLrmRcCode extracts the rc-code value from an LRM operation entry
func getLrmRcCode(entry *xmltree.Element) (int, error) {
	rcAttr := entry.SelectAttr(cibAttrKeyRcCode)
	if rcAttr == nil {
		return 0, errors.New("Found LRM resource operation data without a status code")
	}

	rc, err := strconv.ParseInt(rcAttr.Value, 10, 16)
	return int(rc), err
}
