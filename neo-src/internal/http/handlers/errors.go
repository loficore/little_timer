package handlers

// handlerError 是 pathID 辅助函数使用的小型错误类型。
type handlerError struct {
	message string
}

func (e *handlerError) Error() string { return e.message }
