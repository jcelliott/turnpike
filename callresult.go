package turnpike

type CallResult struct {
	args   []interface{}
	kwargs map[string]interface{}
	err    URI
}

func NewCallResult() *CallResult {
	return &CallResult{
		args:   make([]interface{}, 0, 0),
		kwargs: make(map[string]interface{}),
		err:    "",
	}
}

func ValueResult(value interface{}) *CallResult {
	result := NewCallResult()
	result.args = append(result.args, value)
	return result
}

func SliceResult(slice []interface{}) *CallResult {
	result := NewCallResult()
	result.args = slice
	return result
}

func MapResult(map_value map[string]interface{}) *CallResult {
	result := NewCallResult()
	result.kwargs = map_value
	return result
}

func SimpleErrorResult(err URI) *CallResult {
	result := NewCallResult()
	result.err = err
	return result
}

func ErrorResult(err URI, args []interface{}, kwargs map[string]interface{}) *CallResult {
	return &CallResult{
		args:   args,
		kwargs: kwargs,
		err:    err,
	}
}
