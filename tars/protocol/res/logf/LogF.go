// Package logf comment
// This file was generated by tars2go 1.2.1
// Generated from LogF.tars
package logf

import (
	"fmt"

	"github.com/TarsCloud/TarsGo/tars/protocol/codec"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = fmt.Errorf
var _ = codec.FromInt8

// LogInfo struct implement
type LogInfo struct {
	Appname           string `json:"appname"`
	Servername        string `json:"servername"`
	SFilename         string `json:"sFilename"`
	SFormat           string `json:"sFormat"`
	Setdivision       string `json:"setdivision"`
	BHasSufix         bool   `json:"bHasSufix"`
	BHasAppNamePrefix bool   `json:"bHasAppNamePrefix"`
	BHasSquareBracket bool   `json:"bHasSquareBracket"`
	SConcatStr        string `json:"sConcatStr"`
	SSepar            string `json:"sSepar"`
	SLogType          string `json:"sLogType"`
}

func (st *LogInfo) ResetDefault() {
	st.BHasSufix = true
	st.BHasAppNamePrefix = true
	st.BHasSquareBracket = false
	st.SConcatStr = "_"
	st.SSepar = "|"
	st.SLogType = ""
}

// ReadFrom reads  from readBuf and put into struct.
func (st *LogInfo) ReadFrom(readBuf *codec.Reader) error {
	var (
		err    error
		length int32
		have   bool
		ty     byte
	)
	st.ResetDefault()

	err = readBuf.ReadString(&st.Appname, 0, true)
	if err != nil {
		return err
	}

	err = readBuf.ReadString(&st.Servername, 1, true)
	if err != nil {
		return err
	}

	err = readBuf.ReadString(&st.SFilename, 2, true)
	if err != nil {
		return err
	}

	err = readBuf.ReadString(&st.SFormat, 3, true)
	if err != nil {
		return err
	}

	err = readBuf.ReadString(&st.Setdivision, 4, false)
	if err != nil {
		return err
	}

	err = readBuf.ReadBool(&st.BHasSufix, 5, false)
	if err != nil {
		return err
	}

	err = readBuf.ReadBool(&st.BHasAppNamePrefix, 6, false)
	if err != nil {
		return err
	}

	err = readBuf.ReadBool(&st.BHasSquareBracket, 7, false)
	if err != nil {
		return err
	}

	err = readBuf.ReadString(&st.SConcatStr, 8, false)
	if err != nil {
		return err
	}

	err = readBuf.ReadString(&st.SSepar, 9, false)
	if err != nil {
		return err
	}

	err = readBuf.ReadString(&st.SLogType, 10, false)
	if err != nil {
		return err
	}

	_ = err
	_ = length
	_ = have
	_ = ty
	return nil
}

// ReadBlock reads struct from the given tag , require or optional.
func (st *LogInfo) ReadBlock(readBuf *codec.Reader, tag byte, require bool) error {
	var (
		err  error
		have bool
	)
	st.ResetDefault()

	have, err = readBuf.SkipTo(codec.StructBegin, tag, require)
	if err != nil {
		return err
	}
	if !have {
		if require {
			return fmt.Errorf("require LogInfo, but not exist. tag %d", tag)
		}
		return nil
	}

	err = st.ReadFrom(readBuf)
	if err != nil {
		return err
	}

	err = readBuf.SkipToStructEnd()
	if err != nil {
		return err
	}
	_ = have
	return nil
}

// WriteTo encode struct to buffer
func (st *LogInfo) WriteTo(buf *codec.Buffer) (err error) {

	err = buf.WriteString(st.Appname, 0)
	if err != nil {
		return err
	}

	err = buf.WriteString(st.Servername, 1)
	if err != nil {
		return err
	}

	err = buf.WriteString(st.SFilename, 2)
	if err != nil {
		return err
	}

	err = buf.WriteString(st.SFormat, 3)
	if err != nil {
		return err
	}

	err = buf.WriteString(st.Setdivision, 4)
	if err != nil {
		return err
	}

	err = buf.WriteBool(st.BHasSufix, 5)
	if err != nil {
		return err
	}

	err = buf.WriteBool(st.BHasAppNamePrefix, 6)
	if err != nil {
		return err
	}

	err = buf.WriteBool(st.BHasSquareBracket, 7)
	if err != nil {
		return err
	}

	err = buf.WriteString(st.SConcatStr, 8)
	if err != nil {
		return err
	}

	err = buf.WriteString(st.SSepar, 9)
	if err != nil {
		return err
	}

	err = buf.WriteString(st.SLogType, 10)
	if err != nil {
		return err
	}

	return err
}

// WriteBlock encode struct
func (st *LogInfo) WriteBlock(buf *codec.Buffer, tag byte) error {
	var err error
	err = buf.WriteHead(codec.StructBegin, tag)
	if err != nil {
		return err
	}

	err = st.WriteTo(buf)
	if err != nil {
		return err
	}

	err = buf.WriteHead(codec.StructEnd, 0)
	if err != nil {
		return err
	}
	return nil
}
