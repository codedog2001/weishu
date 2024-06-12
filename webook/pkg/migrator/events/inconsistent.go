package events

// kafka 在这里是用来修复文件的
type InconsistentEvent struct {
	ID        int64
	Direction string
	Type      string
}

const (
	//校验目标数据时候，缺了一条
	InconsistentEventTypeTargetMissing = "target_missing"
	//InconsistentEventTypeNEQ 不相等
	InconsistentEventTypeNEQ = "neq"
	//源数据库缺失数据
	InconsistentEventTypeTBaseMissing = "Base_missing"
)
