package cib

// CRM (Pacemaker) commands

type command interface {
	execute(stdin string) (string, string, error)
}

type crmCommand struct {
	executable string
	arguments  []string
}

func (c *crmCommand) execute(stdin string) (string, string, error) {
	return execute(stdin, c.executable, c.arguments...)
}

const (
	crmUtility = "cibadmin"
)

var (
	// CreateCommand is the command for creating new resources.
	createCommand command = &crmCommand{crmUtility, []string{"--modify", "--allow-create", "--xml-pipe"}}

	// UpdateCommand is the command for updating existing resources.
	//
	// Also used for deleting existing resources.
	updateCommand command = &crmCommand{crmUtility, []string{"--replace", "--xml-pipe"}}

	// ListCommand is the command for reading the CIB
	listCommand command = &crmCommand{crmUtility, []string{"--query"}}
)
