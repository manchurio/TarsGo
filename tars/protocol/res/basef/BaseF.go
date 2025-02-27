// Package basef comment
// This file was generated by tars2go 1.2.1
// Generated from BaseF.tars
package basef

import (
	"fmt"

	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = fmt.Errorf
var _ = codec.FromInt8

// const as define in tars file
const (
	TARSVERSION             int16 = 0x01
	TUPVERSION              int16 = 0x03
	XMLVERSION              int16 = 0x04
	JSONVERSION             int16 = 0x05
	TARSNORMAL              int8  = 0x00
	TARSONEWAY              int8  = 0x01
	TARSSERVERSUCCESS       int32 = 0
	TARSSERVERDECODEERR     int32 = -1
	TARSSERVERENCODEERR     int32 = -2
	TARSSERVERNOFUNCERR     int32 = -3
	TARSSERVERNOSERVANTERR  int32 = -4
	TARSSERVERRESETGRID     int32 = -5
	TARSSERVERQUEUETIMEOUT  int32 = -6
	TARSASYNCCALLTIMEOUT    int32 = -7
	TARSINVOKETIMEOUT       int32 = -7
	TARSPROXYCONNECTERR     int32 = -8
	TARSSERVEROVERLOAD      int32 = -9
	TARSADAPTERNULL         int32 = -10
	TARSINVOKEBYINVALIDESET int32 = -11
	TARSCLIENTDECODEERR     int32 = -12
	TARSSENDREQUESTERR      int32 = -13
	TARSSERVERUNKNOWNERR    int32 = -99
	TARSMESSAGETYPENULL     int32 = 0x00
	TARSMESSAGETYPEHASH     int32 = 0x01
	TARSMESSAGETYPEGRID     int32 = 0x02
	TARSMESSAGETYPEDYED     int32 = 0x04
	TARSMESSAGETYPESAMPLE   int32 = 0x08
	TARSMESSAGETYPEASYNC    int32 = 0x10
	TARSMESSAGETYPESETNAME  int32 = 0x80
	TARSMESSAGETYPETRACE    int32 = 0x100
)
