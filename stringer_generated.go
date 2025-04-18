// Code generated by "stringer -type=groupType,classType,flowType -linecomment -output stringer_generated.go"; DO NOT EDIT.

package nject

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[invokeGroup-0]
	_ = x[literalGroup-1]
	_ = x[staticGroup-2]
	_ = x[runGroup-3]
	_ = x[finalGroup-4]
}

const _groupType_name = "invokeliteralstaticrunfinal"

var _groupType_index = [...]uint8{0, 6, 13, 19, 22, 27}

func (i groupType) String() string {
	if i < 0 || i >= groupType(len(_groupType_index)-1) {
		return "groupType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _groupType_name[_groupType_index[i]:_groupType_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[unsetClassType-0]
	_ = x[fallibleInjectorFunc-1]
	_ = x[fallibleStaticInjectorFunc-2]
	_ = x[injectorFunc-3]
	_ = x[wrapperFunc-4]
	_ = x[finalFunc-5]
	_ = x[staticInjectorFunc-6]
	_ = x[literalValue-7]
	_ = x[initFunc-8]
	_ = x[invokeFunc-9]
}

const _classType_name = "?fallible-injectorfallible-static-injectorinjectorwrapper-funcfinal-funcstatic-injectorliteral-valueinit-funcinvoke-func"

var _classType_index = [...]uint8{0, 1, 18, 42, 50, 62, 72, 87, 100, 109, 120}

func (i classType) String() string {
	if i < 0 || i >= classType(len(_classType_index)-1) {
		return "classType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _classType_name[_classType_index[i]:_classType_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[returnParams-0]
	_ = x[outputParams-1]
	_ = x[inputParams-2]
	_ = x[receivedParams-3]
	_ = x[bypassParams-4]
	_ = x[lastFlowType-5]
}

const _flowType_name = "returnsoutputsinputsreceivedbypassUNUSED"

var _flowType_index = [...]uint8{0, 7, 14, 20, 28, 34, 40}

func (i flowType) String() string {
	if i < 0 || i >= flowType(len(_flowType_index)-1) {
		return "flowType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _flowType_name[_flowType_index[i]:_flowType_index[i+1]]
}
