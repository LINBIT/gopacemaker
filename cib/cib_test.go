package cib

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rsto/xmltest"
	log "github.com/sirupsen/logrus"
)

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

	n := xmltest.Normalizer{OmitWhitespace: true}

	// store normalized version of expected XML
	var buf bytes.Buffer
	if err := n.Normalize(&buf, strings.NewReader(expect)); err != nil {
		t.Fatal(err)
	}
	normExpect := buf.String()

	for _, c := range cases {
		var cib CIB
		listCommand = crmCommand{"echo", []string{c.input}}
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

		actual, err := cib.Doc.WriteToString()
		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		if err := n.Normalize(&buf, strings.NewReader(actual)); err != nil {
			t.Fatal(err)
		}
		normActual := buf.String()

		if normActual != normExpect {
			t.Errorf("XML does not match (input '%s')", c.desc)
			t.Errorf("Expected: %s", normExpect)
			t.Errorf("Actual: %s", normActual)
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

	listCommand = crmCommand{"echo", []string{xml}}

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

	n := xmltest.Normalizer{OmitWhitespace: true}

	for _, c := range cases {
		var cib CIB
		err := cib.ReadConfiguration()
		if err != nil {
			t.Fatal(err)
		}
		// store normalized version of expected XML
		var buf bytes.Buffer
		if err := n.Normalize(&buf, strings.NewReader(c.expect)); err != nil {
			t.Fatal(err)
		}
		normExpect := buf.String()

		cib.DissolveConstraints(c.resources)

		actual, err := cib.Doc.WriteToString()
		if err != nil {
			t.Fatal(err)
		}

		buf.Reset()
		if err := n.Normalize(&buf, strings.NewReader(actual)); err != nil {
			t.Fatal(err)
		}
		normActual := buf.String()

		if normActual != normExpect {
			t.Errorf("XML does not match (input '%s')", c.desc)
			t.Errorf("Expected: %s", normExpect)
			t.Errorf("Actual: %s", normActual)
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
	listCommand = crmCommand{"echo", []string{xml}}
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
