package taskmanager

import "github.com/deliri/primitive/v2026/core"

const taskManagerActiveText = "active"

// ProjectLifecycle partitions bounded project queries.
type ProjectLifecycle uint8

const (
	ProjectLifecycleUnknown ProjectLifecycle = iota
	ProjectLifecycleActive
	ProjectLifecycleCompleted
	projectLifecycleLimit
)

func projectLifecycleFacts() []enumFact[ProjectLifecycle] {
	return []enumFact[ProjectLifecycle]{
		{value: ProjectLifecycleActive, name: taskManagerActiveText},
		{value: ProjectLifecycleCompleted, name: core.CompletionStateText},
	}
}

func (l ProjectLifecycle) Validate() error {
	_, err := enumName(l, projectLifecycleFacts())
	return err
}

func (l ProjectLifecycle) IsValid() bool { return l.Validate() == nil }

func (l ProjectLifecycle) String() string {
	name, _ := enumName(l, projectLifecycleFacts())
	return name
}

func (l ProjectLifecycle) MarshalJSON() ([]byte, error) {
	return marshalEnum(l, projectLifecycleFacts())
}

func (l *ProjectLifecycle) UnmarshalJSON(data []byte) error {
	if l == nil {
		return jsonContractError()
	}
	parsed, err := unmarshalEnum(data, projectLifecycleFacts())
	if err != nil {
		return err
	}
	*l = parsed
	return nil
}

// PageOrder is the closed ordering direction for one bounded collection.
type PageOrder uint8

const (
	PageOrderUnknown PageOrder = iota
	PageOrderAscending
	PageOrderDescending
	pageOrderLimit
)

func pageOrderFacts() []enumFact[PageOrder] {
	return []enumFact[PageOrder]{
		{value: PageOrderAscending, name: "ascending"},
		{value: PageOrderDescending, name: "descending"},
	}
}

func (o PageOrder) Validate() error {
	_, err := enumName(o, pageOrderFacts())
	return err
}

func (o PageOrder) IsValid() bool { return o.Validate() == nil }

func (o PageOrder) String() string {
	name, _ := enumName(o, pageOrderFacts())
	return name
}

func (o PageOrder) MarshalJSON() ([]byte, error) { return marshalEnum(o, pageOrderFacts()) }

func (o *PageOrder) UnmarshalJSON(data []byte) error {
	if o == nil {
		return jsonContractError()
	}
	parsed, err := unmarshalEnum(data, pageOrderFacts())
	if err != nil {
		return err
	}
	*o = parsed
	return nil
}

// TaskKind classifies one unit of work.
type TaskKind uint8

const (
	TaskKindUnknown TaskKind = iota
	TaskKindFeature
	TaskKindBug
	TaskKindChore
	taskKindLimit
)

func taskKindFacts() []enumFact[TaskKind] {
	return []enumFact[TaskKind]{
		{value: TaskKindFeature, name: "feature"},
		{value: TaskKindBug, name: "bug"},
		{value: TaskKindChore, name: "chore"},
	}
}

func (k TaskKind) Validate() error {
	_, err := enumName(k, taskKindFacts())
	return err
}

func (k TaskKind) IsValid() bool { return k.Validate() == nil }

func (k TaskKind) String() string {
	name, _ := enumName(k, taskKindFacts())
	return name
}

func (k TaskKind) MarshalJSON() ([]byte, error) { return marshalEnum(k, taskKindFacts()) }

func (k *TaskKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonContractError()
	}
	parsed, err := unmarshalEnum(data, taskKindFacts())
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}

// TaskState is the independently queryable lifecycle of one task.
type TaskState uint8

const (
	TaskStateUnknown TaskState = iota
	TaskStateBacklog
	TaskStateInProgress
	TaskStateBlocked
	TaskStateCompleted
	TaskStateCancelled
	taskStateLimit
)

func taskStateFacts() []enumFact[TaskState] {
	return []enumFact[TaskState]{
		{value: TaskStateBacklog, name: "backlog"},
		{value: TaskStateInProgress, name: "in_progress"},
		{value: TaskStateBlocked, name: "blocked"},
		{value: TaskStateCompleted, name: core.CompletionStateText},
		{value: TaskStateCancelled, name: "cancelled"},
	}
}

func (s TaskState) Validate() error {
	_, err := enumName(s, taskStateFacts())
	return err
}

func (s TaskState) IsValid() bool { return s.Validate() == nil }

func (s TaskState) String() string {
	name, _ := enumName(s, taskStateFacts())
	return name
}

func (s TaskState) MarshalJSON() ([]byte, error) { return marshalEnum(s, taskStateFacts()) }

func (s *TaskState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonContractError()
	}
	parsed, err := unmarshalEnum(data, taskStateFacts())
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// TaskCollection partitions active work from terminal history so callers do
// not pay to read completed tasks until they ask for them.
type TaskCollection uint8

const (
	TaskCollectionUnknown TaskCollection = iota
	TaskCollectionActive
	TaskCollectionCompleted
	taskCollectionLimit
)

func taskCollectionFacts() []enumFact[TaskCollection] {
	return []enumFact[TaskCollection]{
		{value: TaskCollectionActive, name: taskManagerActiveText},
		{value: TaskCollectionCompleted, name: core.CompletionStateText},
	}
}

func (c TaskCollection) Validate() error {
	_, err := enumName(c, taskCollectionFacts())
	return err
}

func (c TaskCollection) IsValid() bool { return c.Validate() == nil }

func (c TaskCollection) String() string {
	name, _ := enumName(c, taskCollectionFacts())
	return name
}

func (c TaskCollection) MarshalJSON() ([]byte, error) {
	return marshalEnum(c, taskCollectionFacts())
}

func (c *TaskCollection) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonContractError()
	}
	parsed, err := unmarshalEnum(data, taskCollectionFacts())
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// EvidenceKind classifies one durable proof reference without interpreting
// the referenced payload.
type EvidenceKind uint8

const (
	EvidenceKindUnknown EvidenceKind = iota
	EvidenceKindNote
	EvidenceKindTest
	EvidenceKindBenchmark
	EvidenceKindScreenshot
	EvidenceKindGitInventory
	EvidenceKindArtifact
	evidenceKindLimit
)

func evidenceKindFacts() []enumFact[EvidenceKind] {
	return []enumFact[EvidenceKind]{
		{value: EvidenceKindNote, name: "note"},
		{value: EvidenceKindTest, name: "test"},
		{value: EvidenceKindBenchmark, name: "benchmark"},
		{value: EvidenceKindScreenshot, name: "screenshot"},
		{value: EvidenceKindGitInventory, name: "git_inventory"},
		{value: EvidenceKindArtifact, name: "artifact"},
	}
}

func (k EvidenceKind) Validate() error {
	_, err := enumName(k, evidenceKindFacts())
	return err
}

func (k EvidenceKind) IsValid() bool { return k.Validate() == nil }

func (k EvidenceKind) String() string {
	name, _ := enumName(k, evidenceKindFacts())
	return name
}

func (k EvidenceKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(k, evidenceKindFacts())
}

func (k *EvidenceKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonContractError()
	}
	parsed, err := unmarshalEnum(data, evidenceKindFacts())
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}

var (
	_ core.ValidatedJSONMarshaler = ProjectLifecycle(0)
	_ core.ValidatedJSONMarshaler = PageOrder(0)
	_ core.ValidatedJSONMarshaler = TaskKind(0)
	_ core.ValidatedJSONMarshaler = TaskState(0)
	_ core.ValidatedJSONMarshaler = TaskCollection(0)
	_ core.ValidatedJSONMarshaler = EvidenceKind(0)
)
