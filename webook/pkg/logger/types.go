package logger

type Field struct {
	Key string
	Val any
}
type LoggerV1 interface {
	Debug(msg string, args ...Field)
	Info(msg string, args ...Field)
	Warn(msg string, args ...Field)
	Error(msg string, args ...Field)
}

func exampleV1() {
	var l LoggerV1
	// 这是一个新用户 union_id=123
	l.Info("这是一个新用户", Field{Key: "union_id", Val: 123})
}
