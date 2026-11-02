package googlecompute

// InstanceState is an in-process SDK fact; official Google wire structs own JSON.
func (InstanceState) OffWireEnum()    {}
func (v InstanceState) IsValid() bool { return v.Validate() == nil }
func (v InstanceState) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "PENDING", "PROVISIONING", "STAGING", "RUNNING", "STOPPING", "SUSPENDING", "SUSPENDED", "REPAIRING", "TERMINATED"}[v]
}

// OperationState is an in-process SDK fact; official Google wire structs own JSON.
func (OperationState) OffWireEnum()    {}
func (v OperationState) IsValid() bool { return v.Validate() == nil }
func (v OperationState) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "PENDING", "RUNNING", "DONE"}[v]
}

// OperationType is an in-process SDK fact; official Google wire structs own JSON.
func (OperationType) OffWireEnum()    {}
func (v OperationType) IsValid() bool { return v.Validate() == nil }
func (v OperationType) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "start", "stop"}[v]
}
