// Runtime confirmation for finding SP-2: an IAM policy whose Allow is scoped
// by a Condition is parsed and evaluated as an UNCONDITIONAL allow, because
// the shared iampolicy.Statement type has no Condition field. This is a
// defensive check of our own policy engine's over-grant behavior.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/mulgadc/predastore/pkg/iampolicy"
)

func main() {
	// A least-privilege intent: allow ec2:StopInstances ONLY when the request
	// carries MFA. In AWS this Condition gates the Allow.
	policyJSON := `{
      "Version": "2012-10-17",
      "Statement": [{
        "Sid": "StopOnlyWithMFA",
        "Effect": "Allow",
        "Action": "ec2:StopInstances",
        "Resource": "*",
        "Condition": {"Bool": {"aws:MultiFactorAuthPresent": "true"}}
      }]
    }`

	var doc iampolicy.PolicyDocument
	if err := json.Unmarshal([]byte(policyJSON), &doc); err != nil {
		fmt.Println("unmarshal error:", err)
		return
	}

	// Re-marshal what the engine actually retained, to show the Condition was
	// dropped on the floor.
	retained, _ := json.Marshal(doc)
	fmt.Printf("policy as retained by iampolicy: %s\n", retained)

	// Evaluate the gated action as if NO MFA context existed. Least-privilege
	// intent = Deny (the condition is unmet). If the engine returns Allow, the
	// condition was ignored.
	decision := iampolicy.Evaluate("ec2:StopInstances", "*", []iampolicy.PolicyDocument{doc})
	fmt.Printf("Evaluate(ec2:StopInstances, no-MFA-context) = %v (0=Deny,1=Allow)\n", decision)

	if decision == iampolicy.Allow {
		fmt.Println("CONFIRMED SP-2: condition-scoped Allow applied unconditionally (over-grant).")
	} else {
		fmt.Println("NOT CONFIRMED: engine denied; condition appears honored.")
	}
}
