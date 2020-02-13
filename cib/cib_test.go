package cib

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rsto/xmltest"
	log "github.com/sirupsen/logrus"
)

type commandHook func(string) (string, string, error)

type testCommand struct {
	hook commandHook
}

func (c *testCommand) execute(stdin string) (string, string, error) {
	return c.hook(stdin)
}

func normalizeXML(t *testing.T, xml string) string {
	n := xmltest.Normalizer{OmitWhitespace: true}
	var buf bytes.Buffer
	if err := n.Normalize(&buf, strings.NewReader(xml)); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestStopResource(t *testing.T) {
	expect := `<cib><configuration><resources>
			<primitive id="p_iscsi_example">
				<meta_attributes id="p_iscsi_example-meta_attributes">
					<nvpair name="target-role" value="Stopped" id="p_iscsi_example-meta_attributes-target-role"/>
				</meta_attributes>
			</primitive>
		</resources></configuration></cib>`

	cases := []struct {
		desc        string
		input       string
		expectError bool
	}{{
		desc: "nvpair present",
		input: `<cib><configuration><resources>
			<primitive id="p_iscsi_example">
				<meta_attributes id="p_iscsi_example-meta_attributes">
					<nvpair name="target-role" value="Started" id="p_iscsi_example-meta_attributes-target-role"/>
				</meta_attributes>
			</primitive>
		</resources></configuration></cib>`,
	}, {
		desc: "no nvpair present",
		input: `<cib><configuration><resources>
			<primitive id="p_iscsi_example">
				<meta_attributes id="p_iscsi_example-meta_attributes">
				</meta_attributes>
			</primitive>
		</resources></configuration></cib>`,
	}, {
		desc: "no meta_attributes present",
		input: `<cib><configuration><resources>
			<primitive id="p_iscsi_example">
			</primitive>
		</resources></configuration></cib>`,
	}, {
		desc: "no primitive present",
		input: `<cib><configuration><resources>
		</resources></configuration></cib>`,
		expectError: true,
	}}

	// store normalized version of expected XML
	normExpect := normalizeXML(t, expect)

	for _, c := range cases {
		var cib CIB
		listCommand = &testCommand{
			func(_ string) (string, string, error) {
				return c.input, "", nil
			},
		}

		updateCommand = &testCommand{
			func(actual string) (string, string, error) {
				normActual := normalizeXML(t, actual)

				if normActual != normExpect {
					t.Errorf("XML does not match (input '%s')", c.desc)
					t.Errorf("Expected: %s", normExpect)
					t.Errorf("Actual: %s", normActual)
				}
				return "", "", nil
			},
		}

		err := cib.ReadConfiguration()
		if err != nil {
			t.Fatal(err)
		}

		err = cib.StopResource("p_iscsi_example")
		if err != nil {
			if !c.expectError {
				t.Error("Unexpected error: ", err)
			}
			continue
		}

		if c.expectError {
			t.Error("Expected error")
			continue
		}

		err = cib.Update()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDissolveConstraints(t *testing.T) {
	xml := `<cib><configuration><constraints>
<rsc_location id="lo_iscsi_example" resource-discovery="never">
	<resource_set id="lo_iscsi_example-0">
		<resource_ref id="p_iscsi_example_lu1"/>
		<resource_ref id="p_iscsi_example"/>
	</resource_set>
	<rule score="-INFINITY" id="lo_iscsi_example-rule">
		<expression attribute="#uname" operation="ne" value="li0" id="lo_iscsi_example-rule-expression-0"/>
		<expression attribute="#uname" operation="ne" value="li1" id="lo_iscsi_example-rule-expression-1"/>
	</rule>
</rsc_location>
<rsc_colocation id="co_pblock_example" score="INFINITY" rsc="p_pblock_example" with-rsc="p_iscsi_example_ip"/>
<rsc_colocation id="co_iscsi_example" score="INFINITY" rsc="p_iscsi_example" with-rsc="p_pblock_example"/>
<rsc_colocation id="co_iscsi_example_lu1" score="INFINITY" rsc="p_iscsi_example_lu1" with-rsc="p_iscsi_example"/>
<rsc_colocation id="co_punblock_example" score="INFINITY" rsc="p_punblock_example" with-rsc="p_iscsi_example_ip"/>
<rsc_location id="lo_iscsi_example_lu1" rsc="p_iscsi_example_lu1" resource-discovery="never">
	<rule score="0" id="lo_iscsi_example_lu1-rule">
		<expression attribute="#uname" operation="ne" value="li0" id="lo_iscsi_example_lu1-rule-expression-0"/>
		<expression attribute="#uname" operation="ne" value="li1" id="lo_iscsi_example_lu1-rule-expression-1"/>
	</rule>
</rsc_location>
<rsc_order id="o_pblock_example" score="INFINITY" first="p_iscsi_example_ip" then="p_pblock_example"/>
<rsc_order id="o_iscsi_example" score="INFINITY" first="p_pblock_example" then="p_iscsi_example"/>
<rsc_order id="o_iscsi_example_lu1" score="INFINITY" first="p_iscsi_example" then="p_iscsi_example_lu1"/>
<rsc_order id="o_punblock_example" score="INFINITY" first="p_iscsi_example_lu1" then="p_punblock_example"/>
</constraints></configuration><status>
	<node_state><lrm id="171"><lrm_resources>
		<lrm_resource id="p_iscsi_example_ip" type="IPaddr2" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_pblock_example" type="portblock" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_iscsi_example" type="iSCSITarget" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_iscsi_example_lu1" type="iSCSILogicalUnit" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_punblock_example" type="portblock" class="ocf" provider="heartbeat"/>
	</lrm_resources></lrm></node_state>
</status></cib>`
	listCommand = &testCommand{
		func(_ string) (string, string, error) {
			return xml, "", nil
		},
	}

	cases := []struct {
		desc      string
		resources []string
		expect    string
	}{{
		desc:      "remove target",
		resources: []string{"p_iscsi_example"},
		expect: `<cib><configuration><constraints>
<rsc_colocation id="co_pblock_example" score="INFINITY" rsc="p_pblock_example" with-rsc="p_iscsi_example_ip"/>
<rsc_colocation id="co_punblock_example" score="INFINITY" rsc="p_punblock_example" with-rsc="p_iscsi_example_ip"/>
<rsc_location id="lo_iscsi_example_lu1" rsc="p_iscsi_example_lu1" resource-discovery="never">
	<rule score="0" id="lo_iscsi_example_lu1-rule">
		<expression attribute="#uname" operation="ne" value="li0" id="lo_iscsi_example_lu1-rule-expression-0"/>
		<expression attribute="#uname" operation="ne" value="li1" id="lo_iscsi_example_lu1-rule-expression-1"/>
	</rule>
</rsc_location>
<rsc_order id="o_pblock_example" score="INFINITY" first="p_iscsi_example_ip" then="p_pblock_example"/>
<rsc_order id="o_punblock_example" score="INFINITY" first="p_iscsi_example_lu1" then="p_punblock_example"/>
</constraints></configuration><status>
	<node_state><lrm id="171"><lrm_resources>
		<lrm_resource id="p_iscsi_example_ip" type="IPaddr2" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_pblock_example" type="portblock" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_iscsi_example_lu1" type="iSCSILogicalUnit" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_punblock_example" type="portblock" class="ocf" provider="heartbeat"/>
	</lrm_resources></lrm></node_state>
</status></cib>`,
	}, {
		desc:      "remove target, lu",
		resources: []string{"p_iscsi_example", "p_iscsi_example_lu1"},
		expect: `<cib><configuration><constraints>
<rsc_colocation id="co_pblock_example" score="INFINITY" rsc="p_pblock_example" with-rsc="p_iscsi_example_ip"/>
<rsc_colocation id="co_punblock_example" score="INFINITY" rsc="p_punblock_example" with-rsc="p_iscsi_example_ip"/>
<rsc_order id="o_pblock_example" score="INFINITY" first="p_iscsi_example_ip" then="p_pblock_example"/>
</constraints></configuration><status>
	<node_state><lrm id="171"><lrm_resources>
		<lrm_resource id="p_iscsi_example_ip" type="IPaddr2" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_pblock_example" type="portblock" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_punblock_example" type="portblock" class="ocf" provider="heartbeat"/>
	</lrm_resources></lrm></node_state>
</status></cib>`,
	}, {
		desc:      "remove target, lu, ip",
		resources: []string{"p_iscsi_example", "p_iscsi_example_lu1", "p_iscsi_example_ip"},
		expect: `<cib><configuration><constraints></constraints></configuration><status>
	<node_state><lrm id="171"><lrm_resources>
		<lrm_resource id="p_pblock_example" type="portblock" class="ocf" provider="heartbeat"/>
		<lrm_resource id="p_punblock_example" type="portblock" class="ocf" provider="heartbeat"/>
	</lrm_resources></lrm></node_state>
</status></cib>`,
	}, {
		desc:      "remove target, lu, ip, pblock",
		resources: []string{"p_iscsi_example", "p_iscsi_example_lu1", "p_iscsi_example_ip", "p_pblock_example", "p_punblock_example"},
		expect:    `<cib><configuration><constraints></constraints></configuration><status><node_state><lrm id="171"><lrm_resources></lrm_resources></lrm></node_state></status></cib>`,
	}}

	for _, c := range cases {
		var cib CIB

		updateCommand = &testCommand{
			func(actual string) (string, string, error) {
				// store normalized version of expected XML
				normExpect := normalizeXML(t, c.expect)
				normActual := normalizeXML(t, actual)

				if normActual != normExpect {
					t.Errorf("XML does not match (input '%s')", c.desc)
					t.Errorf("Expected: %s", normExpect)
					t.Errorf("Actual: %s", normActual)
				}

				return "", "", nil
			},
		}

		err := cib.ReadConfiguration()
		if err != nil {
			t.Fatal(err)
		}
		cib.DissolveConstraints(c.resources)
		err = cib.Update()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindLrmState(t *testing.T) {
	xml := `<cib><status>
	<node_state><lrm id="171"><lrm_resources>
		<lrm_resource id="p_iscsi_example1">
			<lrm_rsc_op operation="monitor" rc-code="7"/>
		</lrm_resource>
		<lrm_resource id="p_iscsi_example2">
			<lrm_rsc_op operation="monitor" rc-code="8"/>
		</lrm_resource>
		<lrm_resource id="p_iscsi_example3">
			<lrm_rsc_op operation="stop" rc-code="0"/>
		</lrm_resource>
		<lrm_resource id="p_iscsi_example4">
			<lrm_rsc_op operation="stop" rc-code="1"/>
		</lrm_resource>
		<lrm_resource id="p_iscsi_example5">
			<lrm_rsc_op operation="start" rc-code="0"/>
		</lrm_resource>
		<lrm_resource id="p_iscsi_example6">
			<lrm_rsc_op operation="start" rc-code="1"/>
		</lrm_resource>
		<lrm_resource id="p_iscsi_example7"/>
		<lrm_resource id="p_iscsi_example8">
			<lrm_rsc_op operation="start"/>
		</lrm_resource>
	</lrm_resources></lrm></node_state>
</status></cib>`
	listCommand = &crmCommand{"echo", []string{xml}}
	var cib CIB
	err := cib.ReadConfiguration()
	if err != nil {
		t.Fatalf("Invalid XML in test data: %v", err)
	}

	cases := []struct {
		desc   string
		id     string
		expect LrmRunState
	}{{
		desc:   "nonexistent ID",
		id:     "p_iscsi_notexample",
		expect: Unknown,
	}, {
		desc:   "monitor action with rc-code 'not running'",
		id:     "p_iscsi_example1",
		expect: Stopped,
	}, {
		desc:   "monitor action with rc-code 'running master'",
		id:     "p_iscsi_example2",
		expect: Running,
	}, {
		desc:   "successful stop action",
		id:     "p_iscsi_example3",
		expect: Stopped,
	}, {
		desc:   "unsucessful stop action",
		id:     "p_iscsi_example4",
		expect: Running,
	}, {
		desc:   "successful start action",
		id:     "p_iscsi_example5",
		expect: Running,
	}, {
		desc:   "unsucessful start action",
		id:     "p_iscsi_example6",
		expect: Stopped,
	}, {
		desc:   "ID without op",
		id:     "p_iscsi_example7",
		expect: Unknown,
	}, {
		desc:   "op without rc-code",
		id:     "p_iscsi_example8",
		expect: Unknown,
	}}

	// to hide the warning on "op without rc-code"
	log.SetLevel(log.FatalLevel)

	for _, c := range cases {
		actual := cib.FindLrmState(c.id)
		if actual != c.expect {
			t.Errorf("State does not match for case %s", c.desc)
			t.Errorf("Expected: %v", c.expect)
			t.Errorf("Actual: %v", actual)
		}
	}
}

func TestWaitForResourcesStop(t *testing.T) {
	cases := []struct {
		desc          string
		resources     []string
		list          commandHook
		expectStopped bool
		expectError   bool
	}{{
		desc:      "normal case: all resources stopped on the first iteration",
		resources: []string{"p_iscsi_example"},
		list: func(_ string) (string, string, error) {
			xml := `<cib><configuration><resources>
				<primitive id="p_iscsi_example"></primitive>
			</resources></configuration>
			<status>
				<node_state><lrm id="171"><lrm_resources>
					<lrm_resource id="p_iscsi_example">
					<lrm_rsc_op operation="stop" rc-code="0"/>
				</lrm_resource></lrm_resources></lrm></node_state>
			</status></cib>`
			return xml, "", nil
		},
		expectStopped: true,
		expectError:   false,
	}, {
		desc:      "resource not stopped, timeout",
		resources: []string{"p_iscsi_example"},
		list: func(_ string) (string, string, error) {
			xml := `<cib><configuration><resources>
				<primitive id="p_iscsi_example"></primitive>
			</resources></configuration>
			<status>
				<node_state><lrm id="171"><lrm_resources>
					<lrm_resource id="p_iscsi_example">
					<lrm_rsc_op operation="start" rc-code="0"/>
				</lrm_resource></lrm_resources></lrm></node_state>
			</status></cib>`
			return xml, "", nil
		},
		expectStopped: false,
		expectError:   false,
	}}

	// speed it up by waiting only for 1ms
	cibPollRetryDelay = 1 * time.Millisecond

	for _, c := range cases {
		var cib CIB
		listCommand = &testCommand{c.list}
		stopped, err := cib.WaitForResourcesStop(c.resources)
		if err != nil {
			if !c.expectError {
				t.Error("Unexpected error: ", err)
			}
			continue
		}

		if c.expectError {
			t.Error("Expected error")
			continue
		}

		if c.expectStopped != stopped {
			t.Errorf("Expected stopped to be %t, was %t", c.expectStopped, stopped)
		}
	}
}

func TestSetClusterProperty(t *testing.T) {
	cases := []struct {
		desc        string
		input       string
		expect      string
		expectError bool
	}{{
		desc: "nvpair does not exist yet",
		input: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
		</cluster_property_set></crm_config></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
		</cluster_property_set></crm_config></configuration></cib>`,
	}, {
		desc: "nvpair exists",
		input: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="true"/>
		</cluster_property_set></crm_config></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
		</cluster_property_set></crm_config></configuration></cib>`,
	}, {
		desc:        "cib does not exist",
		input:       ``,
		expectError: true,
	}, {
		desc:  "configuration does not exist",
		input: `<cib></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
		</cluster_property_set></crm_config></configuration></cib>`,
	}, {
		desc:  "crm_config does not exist",
		input: `<cib><configuration></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
		</cluster_property_set></crm_config></configuration></cib>`,
	}, {
		desc:  "cps does not exist",
		input: `<cib><configuration><crm_config></crm_config></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
		</cluster_property_set></crm_config></configuration></cib>`,
	}}

	for _, c := range cases {
		var cib CIB
		listCommand = &testCommand{
			func(_ string) (string, string, error) {
				return c.input, "", nil
			},
		}

		updateCommand = &testCommand{
			func(actual string) (string, string, error) {
				normExpect := normalizeXML(t, c.expect)
				normActual := normalizeXML(t, actual)

				if normActual != normExpect {
					t.Errorf("XML does not match (input '%s')", c.desc)
					t.Errorf("Expected: %s", normExpect)
					t.Errorf("Actual: %s", normActual)
				}
				return "", "", nil
			},
		}

		err := cib.setClusterProperty(StonithEnabled, "false")
		if err != nil {
			if !c.expectError {
				t.Error("Unexpected error: ", err)
			}
			continue
		}

		if c.expectError {
			t.Error("Expected error")
			continue
		}
	}
}

func TestGetClusterProperty(t *testing.T) {
	cases := []struct {
		desc        string
		input       string
		expect      string
		expectError bool
	}{{
		desc:        "<cib> does not exist",
		input:       `<someotherroot></someotherroot>`,
		expectError: true,
	}, {
		desc:   "<configuration> does not exist",
		input:  `<cib></cib>`,
		expect: "",
	}, {
		desc:   "<crm_config> does not exist",
		input:  `<cib><configuration></configuration></cib>`,
		expect: "",
	}, {
		desc:   "cps does not exist",
		input:  `<cib><configuration><crm_config></crm_config></configuration></cib>`,
		expect: "",
	}, {
		desc: "<nvpair> does not exist",
		input: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
		</cluster_property_set></crm_config></configuration></cib>`,
		expect: "",
	}, {
		desc: "<nvpair> does not have value",
		input: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" />
		</cluster_property_set></crm_config></configuration></cib>`,
		expect: "",
	}, {
		desc: "<nvpair> has value",
		input: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" value="false" />
		</cluster_property_set></crm_config></configuration></cib>`,
		expect: "false",
	}}

	for _, c := range cases {
		var cib CIB
		listCommand = &testCommand{
			func(_ string) (string, string, error) {
				return c.input, "", nil
			},
		}

		actual, err := cib.getClusterProperty(StonithEnabled)
		if err != nil {
			if !c.expectError {
				t.Error("Unexpected error: ", err)
			}
			continue
		}

		if c.expectError {
			t.Error("Expected error")
			continue
		}

		if actual != c.expect {
			t.Errorf("Return value does not match (input '%s')", c.desc)
			t.Errorf("Expected: %s", c.expect)
			t.Errorf("Actual: %s", actual)
		}
	}
}

func TestFindNodeState(t *testing.T) {
	var cib CIB

	cases := []struct {
		desc        string
		xml         string
		expect      NodeState
		expectError bool
	}{{
		desc: "normal case",
		xml:  `<cib><status><node_state uname="node1" in_ccm="true" crmd="online" join="member" expected="member"></node_state></cib></status>`,
		expect: NodeState{
			InCCM:        true,
			Crmd:         true,
			Join:         JoinMember,
			JoinExpected: JoinMember,
		},
	}, {
		desc: "node down",
		xml:  `<cib><status><node_state uname="node1" in_ccm="false" crmd="offline" join="down" expected="banned"></node_state></cib></status>`,
		expect: NodeState{
			InCCM:        false,
			Crmd:         false,
			Join:         JoinDown,
			JoinExpected: JoinBanned,
		},
	}, {
		desc:        "missing attribute",
		xml:         `<cib><status><node_state uname="node1"></node_state></cib></status>`,
		expectError: true,
	}, {
		desc:        "unknown node",
		xml:         `<cib><status><node_state uname="some_other_node"></node_state></cib></status>`,
		expectError: true,
	}}

	for _, c := range cases {
		listCommand = &crmCommand{"echo", []string{c.xml}}
		actual, err := cib.FindNodeState("node1")
		if err != nil {
			if !c.expectError {
				t.Error("Unexpected error: ", err)
			}
			continue
		}

		if c.expectError {
			t.Error("Expected error")
			continue
		}

		if actual != c.expect {
			t.Errorf("State does not match for case \"%s\"", c.desc)
			t.Errorf("Expected: %+v", c.expect)
			t.Errorf("Actual: %+v", actual)
		}
	}
}

func TestNodeStandby(t *testing.T) {
	cases := []struct {
		desc        string
		input       string
		expect      string
		expectError bool
	}{{
		desc: "set standby la1",
		input: `<cib><configuration><crm_config>
			<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			<nvpair id="cib-bootstrap-options-have-watchdog" name="have-watchdog" value="false"/>
			<nvpair id="cib-bootstrap-options-dc-version" name="dc-version" value="2.0.2.linbit-3.0.el8-744a30d655"/>
			<nvpair id="cib-bootstrap-options-cluster-infrastructure" name="cluster-infrastructure" value="corosync"/>
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1"/>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			<nvpair id="cib-bootstrap-options-have-watchdog" name="have-watchdog" value="false"/>
			<nvpair id="cib-bootstrap-options-dc-version" name="dc-version" value="2.0.2.linbit-3.0.el8-744a30d655"/>
			<nvpair id="cib-bootstrap-options-cluster-infrastructure" name="cluster-infrastructure" value="corosync"/>
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1">
				<instance_attributes id="nodes-1">
				    <nvpair id="nodes-1-standby" name="standby" value="on"/>
                </instance_attributes>
			</node>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
	}, {
		desc: "set standby la1",
		input: `<cib><configuration><crm_config>
			<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1"><instance_attributes id="nodes-1"/></node>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1">
				<instance_attributes id="nodes-1">
				    <nvpair id="nodes-1-standby" name="standby" value="on"/>
                </instance_attributes>
			</node>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
	}}

	for _, c := range cases {
		var cib CIB
		listCommand = &testCommand{
			func(_ string) (string, string, error) {
				return c.input, "", nil
			},
		}

		updateCommand = &testCommand{
			func(actual string) (string, string, error) {
				normExpect := normalizeXML(t, c.expect)
				normActual := normalizeXML(t, actual)

				if normActual != normExpect {
					t.Errorf("XML does not match (input '%s')", c.desc)
					t.Errorf("Expected: %s", normExpect)
					t.Errorf("Actual: %s", normActual)
				}
				return "", "", nil
			},
		}

		err := cib.StandbyNode("la1")

		if err != nil {
			if !c.expectError {
				t.Error("Unexpected error: ", err)
			}
			continue
		}

		if c.expectError {
			t.Error("Expected error")
			continue
		}
	}
}

func TestNodeUnStandby(t *testing.T) {
	cases := []struct {
		desc        string
		input       string
		expect      string
		expectError bool
	}{{
		desc: "unstandby la1",
		input: `<cib><configuration><crm_config>
			<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			<nvpair id="cib-bootstrap-options-have-watchdog" name="have-watchdog" value="false"/>
			<nvpair id="cib-bootstrap-options-dc-version" name="dc-version" value="2.0.2.linbit-3.0.el8-744a30d655"/>
			<nvpair id="cib-bootstrap-options-cluster-infrastructure" name="cluster-infrastructure" value="corosync"/>
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1"/>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			<nvpair id="cib-bootstrap-options-have-watchdog" name="have-watchdog" value="false"/>
			<nvpair id="cib-bootstrap-options-dc-version" name="dc-version" value="2.0.2.linbit-3.0.el8-744a30d655"/>
			<nvpair id="cib-bootstrap-options-cluster-infrastructure" name="cluster-infrastructure" value="corosync"/>
			<nvpair id="cib-bootstrap-options-cluster-name" name="cluster-name" value="la"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1"/>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
	}, {
		desc: "unstandby la1",
		input: `<cib><configuration><crm_config>
			<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1">
			<instance_attributes id="nodes-1">
				<nvpair id="nodes-1-standby" name="standby" value="on"/>
			</instance_attributes>
			</node>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
		expect: `<cib><configuration><crm_config>
		<cluster_property_set id="cib-bootstrap-options">
			<nvpair id="cib-bootstrap-options-stonith-enabled" name="stonith-enabled" value="false"/>
			</cluster_property_set>
		</crm_config>
		<nodes>
			<node id="1" uname="la1">
				<instance_attributes id="nodes-1"></instance_attributes>
			</node>
			<node id="3" uname="la2"/>
			<node id="2" uname="la3"/>
		</nodes></configuration></cib>`,
	}}

	for _, c := range cases {
		var cib CIB
		listCommand = &testCommand{
			func(_ string) (string, string, error) {
				return c.input, "", nil
			},
		}

		updateCommand = &testCommand{
			func(actual string) (string, string, error) {
				normExpect := normalizeXML(t, c.expect)
				normActual := normalizeXML(t, actual)

				if normActual != normExpect {
					t.Errorf("XML does not match (input '%s')", c.desc)
					t.Errorf("Expected: %s", normExpect)
					t.Errorf("Actual: %s", normActual)
				}
				return "", "", nil
			},
		}

		err := cib.UnStandbyNode("la1")

		if err != nil {
			if !c.expectError {
				t.Error("Unexpected error: ", err)
			}
			continue
		}

		if c.expectError {
			t.Error("Expected error")
			continue
		}
	}
}

func TestGetNodeOfResource(t *testing.T) {
	var cib CIB

	cases := []struct {
		desc   string
		xml    string
		expect string
	}{{
		desc: "successful monitor action, one node",
		xml: `<cib><status><node_state uname="node1"><lrm>
			<lrm_resources>
				<lrm_resource id="p_test">
					<lrm_rsc_op operation="monitor" rc-code="0" />
				</lrm_resource>
			</lrm_resources>
		</lrm></node_state></status></cib>`,
		expect: "node1",
	}, {
		desc: "successful monitor action, multiple nodes",
		xml: `<cib><status>
			<node_state uname="node1"><lrm>
				<lrm_resources>
					<lrm_resource id="p_test">
						<lrm_rsc_op operation="monitor" rc-code="7" />
					</lrm_resource>
				</lrm_resources>
			</lrm></node_state>
			<node_state uname="node2"><lrm>
				<lrm_resources>
					<lrm_resource id="p_test">
						<lrm_rsc_op operation="monitor" rc-code="0" />
					</lrm_resource>
				</lrm_resources>
			</lrm></node_state>
			<node_state uname="node3"><lrm>
				<lrm_resources>
				</lrm_resources>
			</lrm></node_state>
		</status></cib>`,
		expect: "node2",
	}}

	for _, c := range cases {
		listCommand = &crmCommand{"echo", []string{c.xml}}
		actual := cib.GetNodeOfResource("p_test")

		if actual != c.expect {
			t.Errorf("State does not match for case \"%s\"", c.desc)
			t.Errorf("Expected: %+v", c.expect)
			t.Errorf("Actual: %+v", actual)
		}
	}
}

func TestUpdate(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unexpected panic: %s", r)
		}
	}()

	var c CIB
	err := c.Update() // this should not panic
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestMarshalLrmRunState(t *testing.T) {
	// We want to make sure LrmRunState is JSON marshallable
	j := struct {
		RunStates []LrmRunState `json:"run_states"`
	}{
		[]LrmRunState{Unknown, Running, Stopped},
	}

	bytes, err := json.Marshal(j)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	expect := `{"run_states":["Unknown","Running","Stopped"]}`
	actual := string(bytes)
	if string(bytes) != expect {
		t.Errorf("Unexpected marshalled JSON")
		t.Errorf("Expected: %s", expect)
		t.Errorf("Actual: %s", actual)
	}
}
