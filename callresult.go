package turnpike

// CallResult represents the result of a CALL.
type CallResult struct {
	args   []interface{}
	kwargs map[string]interface{}
	err    URI
}

func newCallResult() *CallResult {
	return &CallResult{
		args:   make([]interface{}, 0, 0),
		kwargs: make(map[string]interface{}),
		err:    "",
	}
}

func ValueResult(value interface{}) *CallResult {
	result := newCallResult()
	result.args = append(result.args, value)
	return result
}

func SliceResult(slice []interface{}) *CallResult {
	result := newCallResult()
	result.args = slice
	return result
}

func MapResult(map_value map[string]interface{}) *CallResult {
	result := newCallResult()
	result.kwargs = map_value
	return result
}

func SimpleErrorResult(err URI) *CallResult {
	result := newCallResult()
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
