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
	_ = x[TraceBatchReplication-128]
	_ = x[TraceBatchKeyRotation-256]
	_ = x[TraceBatchExpire-512]
	_ = x[TraceRebalance-1024]
	_ = x[TraceReplicationResync-2048]
	_ = x[TraceBootstrap-4096]
	_ = x[TraceFTP-8192]
	_ = x[TraceILM-16384]
	_ = x[TraceKMS-32768]
	_ = x[TraceAll-65535]
}

const _TraceType_name = "OSStorageS3InternalScannerDecommissionHealingBatchReplicationBatchKeyRotationBatchExpireRebalanceReplicationResyncBootstrapFTPILMKMSAll"

var _TraceType_map = map[TraceType]string{
	1:     _TraceType_name[0:2],
	2:     _TraceType_name[2:9],
	4:     _TraceType_name[9:11],
	8:     _TraceType_name[11:19],
	16:    _TraceType_name[19:26],
	32:    _TraceType_name[26:38],
	64:    _TraceType_name[38:45],
	128:   _TraceType_name[45:61],
	256:   _TraceType_name[61:77],
	512:   _TraceType_name[77:88],
	1024:  _TraceType_name[88:97],
	2048:  _TraceType_name[97:114],
	4096:  _TraceType_name[114:123],
	8192:  _TraceType_name[123:126],
	16384: _TraceType_name[126:129],
	32768: _TraceType_name[129:132],
	65535: _TraceType_name[132:135],
}

func (i TraceType) String() string {
	if str, ok := _TraceType_map[i]; ok {
		return str
	}
	return "TraceType(" + strconv.FormatInt(int64(i), 10) + ")"
}
