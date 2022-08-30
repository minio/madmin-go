// Code generated by "stringer -type=TraceType -trimprefix=Trace trace.go"; DO NOT EDIT.

package madmin

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[TraceOS-1]
	_ = x[TraceStorage-2]
	_ = x[TraceS3-4]
	_ = x[TraceInternal-8]
	_ = x[TraceScanner-16]
	_ = x[TraceDecommission-32]
	_ = x[TraceHealing-64]
	_ = x[TraceAll-127]
}

const (
	_TraceType_name_0 = "OSStorage"
	_TraceType_name_1 = "S3"
	_TraceType_name_2 = "Internal"
	_TraceType_name_3 = "Scanner"
	_TraceType_name_4 = "Decommission"
	_TraceType_name_5 = "Healing"
	_TraceType_name_6 = "All"
)

var (
	_TraceType_index_0 = [...]uint8{0, 2, 9}
)

func (i TraceType) String() string {
	switch {
	case 1 <= i && i <= 2:
		i -= 1
		return _TraceType_name_0[_TraceType_index_0[i]:_TraceType_index_0[i+1]]
	case i == 4:
		return _TraceType_name_1
	case i == 8:
		return _TraceType_name_2
	case i == 16:
		return _TraceType_name_3
	case i == 32:
		return _TraceType_name_4
	case i == 64:
		return _TraceType_name_5
	case i == 127:
		return _TraceType_name_6
	default:
		return "TraceType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
