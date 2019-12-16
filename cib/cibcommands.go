package cib

// CRM (Pacemaker) commands

type crmCommand struct {
	executable string
	arguments  []string
}

const (
	crmUtility = "cibadmin"
)

// CreateCommand is the command for creating new resources.
var createCommand = crmCommand{crmUtility, []string{"--modify", "--allow-create", "--xml-pipe"}}

// UpdateCommand is the command for updating existing resources.
//
// Also used for deleting existing resources.
var updateCommand = crmCommand{crmUtility, []string{"--replace", "--xml-pipe"}}

// ListCommand is the command for reading the CIB
var listCommand = crmCommand{crmUtility, []string{"--query"}}
