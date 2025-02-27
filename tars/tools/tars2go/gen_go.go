package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var gE = flag.Bool("E", false, "Generate code before fmt for troubleshooting")
var gAddServant = flag.Bool("add-servant", true, "Generate AddServant function")
var gModuleCycle = flag.Bool("module-cycle", false, "support jce module cycle include(do not support jce file cycle include)")
var gModuleUpper = flag.Bool("module-upper", false, "native module names are supported, otherwise the system will upper the first letter of the module name")
var gJsonOmitEmpty = flag.Bool("json-omitempty", false, "Generate json omitempty support")
var dispatchReporter = flag.Bool("dispatch-reporter", false, "Dispatch reporter support")
var debug = flag.Bool("debug", false, "enable debug mode")

var gFileMap map[string]bool

func init() {
	gFileMap = make(map[string]bool)
}

// GenGo record go code information.
type GenGo struct {
	I        []string // imports with path
	code     bytes.Buffer
	vc       int // var count. Used to generate unique variable names
	path     string
	tarsPath string
	module   string
	prefix   string
	p        *Parse

	// proto file name(not include .tars)
	ProtoName string
}

// NewGenGo build up a new path
func NewGenGo(path string, module string, outdir string) *GenGo {
	if outdir != "" {
		b := []byte(outdir)
		last := b[len(b)-1:]
		if string(last) != "/" {
			outdir += "/"
		}
	}

	return &GenGo{path: path, module: module, prefix: outdir, ProtoName: path2ProtoName(path)}
}

func path2ProtoName(path string) string {
	iBegin := strings.LastIndex(path, "/")
	if iBegin == -1 || iBegin >= len(path)-1 {
		iBegin = 0
	} else {
		iBegin++
	}
	iEnd := strings.LastIndex(path, ".tars")
	if iEnd == -1 {
		iEnd = len(path)
	}

	return path[iBegin:iEnd]
}

// Initial capitalization
func upperFirstLetter(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) == 1 {
		return strings.ToUpper(string(s[0]))
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}

func getShortTypeName(src string) string {
	vec := strings.Split(src, "::")
	return vec[len(vec)-1]
}

func errString(hasRet bool) string {
	var retStr string
	if hasRet {
		retStr = "return ret, err"
	} else {
		retStr = "return err"
	}
	return `if err != nil {
  ` + retStr + `
  }
`
}

func genForHead(vc string) string {
	i := `i` + vc
	e := `e` + vc
	return ` for ` + i + `,` + e + ` := int32(0), length;` + i + `<` + e + `;` + i + `++ `
}

// === rename area ===
// 0. rename module
func (p *Parse) rename() {
	p.OriginModule = p.Module
	if *gModuleUpper {
		p.Module = upperFirstLetter(p.Module)
	}
}

// 1. struct rename
// struct Name { 1 require Mb type}
func (st *StructInfo) rename() {
	st.OriginName = st.Name
	st.Name = upperFirstLetter(st.Name)
	for i := range st.Mb {
		st.Mb[i].OriginKey = st.Mb[i].Key
		st.Mb[i].Key = upperFirstLetter(st.Mb[i].Key)
	}
}

// 1. interface rename
// interface Name { Fun }
func (itf *InterfaceInfo) rename() {
	itf.OriginName = itf.Name
	itf.Name = upperFirstLetter(itf.Name)
	for i := range itf.Fun {
		itf.Fun[i].rename()
	}
}

func (en *EnumInfo) rename() {
	en.OriginName = en.Name
	en.Name = upperFirstLetter(en.Name)
	for i := range en.Mb {
		en.Mb[i].Key = upperFirstLetter(en.Mb[i].Key)
	}
}

func (cst *ConstInfo) rename() {
	cst.OriginName = cst.Name
	cst.Name = upperFirstLetter(cst.Name)
}

// 2. func rename
// type Fun (arg ArgType), in case keyword and name conflicts,argname need to capitalize.
// Fun (type int32)
func (fun *FunInfo) rename() {
	fun.OriginName = fun.Name
	fun.Name = upperFirstLetter(fun.Name)
	for i := range fun.Args {
		fun.Args[i].OriginName = fun.Args[i].Name
		// func args donot upper firs
		//fun.Args[i].Name = upperFirstLetter(fun.Args[i].Name)
	}
}

// 3. genType rename all Type

// === rename end ===

// Gen to parse file.
func (gen *GenGo) Gen() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			// set exit code
			os.Exit(1)
		}
	}()

	gen.p = ParseFile(gen.path, make([]string, 0))
	gen.genAll()
}

func (gen *GenGo) genAll() {
	if gFileMap[gen.path] {
		// already compiled
		return
	}
	gFileMap[gen.path] = true

	gen.p.rename()
	gen.genInclude(gen.p.IncParse)

	gen.code.Reset()
	gen.genHead()
	gen.genPackage()

	for _, v := range gen.p.Enum {
		gen.genEnum(&v)
	}

	gen.genConst(gen.p.Const)

	for _, v := range gen.p.Struct {
		gen.genStruct(&v)
	}
	if len(gen.p.Enum) > 0 || len(gen.p.Const) > 0 || len(gen.p.Struct) > 0 {
		gen.saveToSourceFile(path2ProtoName(gen.path) + ".go")
	}

	for _, v := range gen.p.Interface {
		gen.genInterface(&v)
	}
}

func (gen *GenGo) genErr(err string) {
	panic(err)
}

func (gen *GenGo) saveToSourceFile(filename string) {
	var beauty []byte
	var err error
	prefix := gen.prefix

	if !*gE {
		beauty, err = format.Source(gen.code.Bytes())
		if err != nil {
			if *debug {
				fmt.Println("------------------")
				fmt.Println(string(gen.code.Bytes()))
				fmt.Println("------------------")
			}
			gen.genErr("go fmt fail. " + filename + " " + err.Error())
		}
	} else {
		beauty = gen.code.Bytes()
	}

	if filename == "stdout" {
		fmt.Println(string(beauty))
	} else {
		var mkPath string
		if *gModuleCycle == true {
			mkPath = prefix + gen.ProtoName + "/" + gen.p.Module
		} else {
			mkPath = prefix + gen.p.Module
		}
		err = os.MkdirAll(mkPath, 0766)

		if err != nil {
			gen.genErr(err.Error())
		}
		err = ioutil.WriteFile(mkPath+"/"+filename, beauty, 0666)

		if err != nil {
			gen.genErr(err.Error())
		}
	}
}

func (gen *GenGo) genVariableName(prefix, name string) string {
	if strings.HasPrefix(name, "(*") && strings.HasSuffix(name, ")") {
		return strings.Trim(name, "()")
	}
	return prefix + name
}

func (gen *GenGo) genHead() {
	gen.code.WriteString(`// Package ` + gen.p.Module + ` comment
// This file was generated by tars2go ` + VERSION + `
// Generated from ` + filepath.Base(gen.path) + `
`)
}

func (gen *GenGo) genPackage() {
	gen.code.WriteString("package " + gen.p.Module + "\n\n")
	gen.code.WriteString(`
import (
	"fmt"

`)
	gen.code.WriteString(`"` + gen.tarsPath + "/protocol/codec\"\n")

	mImports := make(map[string]bool)
	for _, st := range gen.p.Struct {
		if *gModuleCycle == true {
			for k, v := range st.DependModuleWithJce {
				gen.genStructImport(k, v, mImports)
			}
		} else {
			for k := range st.DependModule {
				gen.genStructImport(k, "", mImports)
			}
		}
	}
	for path := range mImports {
		gen.code.WriteString(path + "\n")
	}

	gen.code.WriteString(`)

	// Reference imports to suppress errors if they are not otherwise used.
	var _ = fmt.Errorf
	var _ = codec.FromInt8

`)
}

func (gen *GenGo) genStructImport(module string, protoName string, mImports map[string]bool) {
	var moduleStr string
	var jcePath string
	var moduleAlia string
	if *gModuleCycle == true {
		moduleStr = module[len(protoName)+1:]
		jcePath = protoName + "/"
		moduleAlia = module + " "
	} else {
		moduleStr = module
	}

	for _, p := range gen.I {
		if strings.HasSuffix(p, "/"+moduleStr) {
			mImports[`"`+p+`"`] = true
			return
		}
	}

	if *gModuleUpper {
		moduleAlia = upperFirstLetter(moduleAlia)
	}

	// example:
	// TarsTest.tars, MyApp
	// gomod:
	// github.com/xxx/yyy/tars-protocol/MyApp
	// github.com/xxx/yyy/tars-protocol/TarsTest/MyApp
	//
	// gopath:
	// MyApp
	// TarsTest/MyApp
	var modulePath string
	if gen.module != "" {
		mf := filepath.Clean(filepath.Join(gen.module, gen.prefix))
		if runtime.GOOS == "windows" {
			mf = strings.ReplaceAll(mf, string(os.PathSeparator), string('/'))
		}
		modulePath = fmt.Sprintf("%s/%s%s", mf, jcePath, moduleStr)
	} else {
		modulePath = fmt.Sprintf("%s%s", jcePath, moduleStr)
	}
	mImports[moduleAlia+`"`+modulePath+`"`] = true
}

func (gen *GenGo) genIFPackage(itf *InterfaceInfo) {
	gen.code.WriteString("package " + gen.p.Module + "\n\n")
	gen.code.WriteString(`
import (
	"bytes"
	"context"
	"fmt"
	"unsafe"
	"encoding/json"
`)
	if *gAddServant {
		gen.code.WriteString(`"` + gen.tarsPath + "\"\n")
	}

	gen.code.WriteString(`"` + gen.tarsPath + "/protocol/res/requestf\"\n")
	gen.code.WriteString(`m "` + gen.tarsPath + "/model\"\n")
	gen.code.WriteString(`"` + gen.tarsPath + "/protocol/codec\"\n")
	gen.code.WriteString(`"` + gen.tarsPath + "/protocol/tup\"\n")
	gen.code.WriteString(`"` + gen.tarsPath + "/protocol/res/basef\"\n")
	gen.code.WriteString(`"` + gen.tarsPath + "/util/tools\"\n")
	gen.code.WriteString(`"` + gen.tarsPath + "/util/endpoint\"\n")
	gen.code.WriteString(`"` + gen.tarsPath + "/util/current\"\n")
	if !withoutTrace {
		gen.code.WriteString("tarstrace \"" + gen.tarsPath + "/util/trace\"\n")
	}

	if *gModuleCycle == true {
		for k, v := range itf.DependModuleWithJce {
			gen.genIFImport(k, v)
		}
	} else {
		for k := range itf.DependModule {
			gen.genIFImport(k, "")
		}
	}
	gen.code.WriteString(`)

	// Reference imports to suppress errors if they are not otherwise used.
	var (
		_ = fmt.Errorf
		_ = codec.FromInt8
		_ = unsafe.Pointer(nil)
		_ = bytes.ErrTooLarge
	)
`)
}

func (gen *GenGo) genIFImport(module string, protoName string) {
	var moduleStr string
	var jcePath string
	var moduleAlia string
	if *gModuleCycle == true {
		moduleStr = module[len(protoName)+1:]
		jcePath = protoName + "/"
		moduleAlia = module + " "
	} else {
		moduleStr = module
	}
	for _, p := range gen.I {
		if strings.HasSuffix(p, "/"+moduleStr) {
			gen.code.WriteString(`"` + p + `"` + "\n")
			return
		}
	}

	if *gModuleUpper {
		moduleAlia = upperFirstLetter(moduleAlia)
	}

	// example:
	// TarsTest.tars, MyApp
	// gomod:
	// github.com/xxx/yyy/tars-protocol/MyApp
	// github.com/xxx/yyy/tars-protocol/TarsTest/MyApp
	//
	// gopath:
	// MyApp
	// TarsTest/MyApp
	var modulePath string
	if gen.module != "" {
		mf := filepath.Clean(filepath.Join(gen.module, gen.prefix))
		if runtime.GOOS == "windows" {
			mf = strings.ReplaceAll(mf, string(os.PathSeparator), string('/'))
		}
		modulePath = fmt.Sprintf("%s/%s%s", mf, jcePath, moduleStr)
	} else {
		modulePath = fmt.Sprintf("%s%s", jcePath, moduleStr)
	}
	gen.code.WriteString(moduleAlia + `"` + modulePath + `"` + "\n")
}

func (gen *GenGo) genType(ty *VarType) string {
	ret := ""
	switch ty.Type {
	case tkTBool:
		ret = "bool"
	case tkTInt:
		if ty.Unsigned {
			ret = "uint32"
		} else {
			ret = "int32"
		}
	case tkTShort:
		if ty.Unsigned {
			ret = "uint16"
		} else {
			ret = "int16"
		}
	case tkTByte:
		if ty.Unsigned {
			ret = "uint8"
		} else {
			ret = "int8"
		}
	case tkTLong:
		if ty.Unsigned {
			ret = "uint64"
		} else {
			ret = "int64"
		}
	case tkTFloat:
		ret = "float32"
	case tkTDouble:
		ret = "float64"
	case tkTString:
		ret = "string"
	case tkTVector:
		ret = "[]" + gen.genType(ty.TypeK)
	case tkTMap:
		ret = "map[" + gen.genType(ty.TypeK) + "]" + gen.genType(ty.TypeV)
	case tkName:
		ret = strings.Replace(ty.TypeSt, "::", ".", -1)
		vec := strings.Split(ty.TypeSt, "::")
		for i := range vec {
			if *gModuleUpper {
				vec[i] = upperFirstLetter(vec[i])
			} else {
				if i == (len(vec) - 1) {
					vec[i] = upperFirstLetter(vec[i])
				}
			}
		}
		ret = strings.Join(vec, ".")
	case tkTArray:
		ret = "[" + fmt.Sprintf("%v", ty.TypeL) + "]" + gen.genType(ty.TypeK)
	default:
		gen.genErr("Unknown Type " + TokenMap[ty.Type])
	}
	return ret
}

func (gen *GenGo) genStructDefine(st *StructInfo) {
	c := &gen.code
	c.WriteString("// " + st.Name + " struct implement\n")
	c.WriteString("type " + st.Name + " struct {\n")

	for _, v := range st.Mb {
		if *gJsonOmitEmpty {
			c.WriteString("\t" + v.Key + " " + gen.genType(v.Type) + " `json:\"" + v.OriginKey + ",omitempty\"`\n")
		} else {
			c.WriteString("\t" + v.Key + " " + gen.genType(v.Type) + " `json:\"" + v.OriginKey + "\"`\n")
		}
	}
	c.WriteString("}\n")
}

func (gen *GenGo) genFunResetDefault(st *StructInfo) {
	c := &gen.code

	c.WriteString("func (st *" + st.Name + ") ResetDefault() {\n")

	for _, v := range st.Mb {
		if v.Type.CType == tkStruct {
			c.WriteString("st." + v.Key + ".ResetDefault()\n")
		}
		if v.Default == "" {
			continue
		}
		c.WriteString("st." + v.Key + " = " + v.Default + "\n")
	}
	c.WriteString("}\n")
}

func (gen *GenGo) genWriteSimpleList(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	tag := strconv.Itoa(int(mb.Tag))
	unsign := "Int8"
	if mb.Type.TypeK.Unsigned {
		unsign = "Uint8"
	}
	errStr := errString(hasRet)
	c.WriteString(`
err = buf.WriteHead(codec.SimpleList, ` + tag + `)
` + errStr + `
err = buf.WriteHead(codec.BYTE, 0)
` + errStr + `
err = buf.WriteInt32(int32(len(` + gen.genVariableName(prefix, mb.Key) + `)), 0)
` + errStr + `
err = buf.WriteSlice` + unsign + `(` + gen.genVariableName(prefix, mb.Key) + `)
` + errStr + `
`)
}

func (gen *GenGo) genWriteVector(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code

	// SimpleList
	if mb.Type.TypeK.Type == tkTByte && !mb.Type.TypeK.Unsigned {
		gen.genWriteSimpleList(mb, prefix, hasRet)
		return
	}
	errStr := errString(hasRet)

	// LIST
	tag := strconv.Itoa(int(mb.Tag))
	c.WriteString(`
err = buf.WriteHead(codec.LIST, ` + tag + `)
` + errStr + `
err = buf.WriteInt32(int32(len(` + gen.genVariableName(prefix, mb.Key) + `)), 0)
` + errStr + `
for _, v := range ` + gen.genVariableName(prefix, mb.Key) + ` {
`)
	// for _, v := range can nesting for _, v := range，does not conflict, support multidimensional arrays

	dummy := &StructMember{}
	dummy.Type = mb.Type.TypeK
	dummy.Key = "v"
	gen.genWriteVar(dummy, "", hasRet)

	c.WriteString("}\n")
}

func (gen *GenGo) genWriteArray(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code

	// SimpleList
	if mb.Type.TypeK.Type == tkTByte && !mb.Type.TypeK.Unsigned {
		gen.genWriteSimpleList(mb, prefix, hasRet)
		return
	}
	errStr := errString(hasRet)

	// LIST
	tag := strconv.Itoa(int(mb.Tag))
	c.WriteString(`
err = buf.WriteHead(codec.LIST, ` + tag + `)
` + errStr + `
err = buf.WriteInt32(int32(len(` + gen.genVariableName(prefix, mb.Key) + `)), 0)
` + errStr + `
for _, v := range ` + gen.genVariableName(prefix, mb.Key) + ` {
`)
	// for _, v := range can nesting for _, v := range，does not conflict, support multidimensional arrays

	dummy := &StructMember{}
	dummy.Type = mb.Type.TypeK
	dummy.Key = "v"
	gen.genWriteVar(dummy, "", hasRet)

	c.WriteString("}\n")
}

func (gen *GenGo) genWriteStruct(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	tag := strconv.Itoa(int(mb.Tag))
	c.WriteString(`
err = ` + prefix + mb.Key + `.WriteBlock(buf, ` + tag + `)
` + errString(hasRet) + `
`)
}

func (gen *GenGo) genWriteMap(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	tag := strconv.Itoa(int(mb.Tag))
	vc := strconv.Itoa(gen.vc)
	gen.vc++
	errStr := errString(hasRet)
	c.WriteString(`
err = buf.WriteHead(codec.MAP, ` + tag + `)
` + errStr + `
err = buf.WriteInt32(int32(len(` + gen.genVariableName(prefix, mb.Key) + `)), 0)
` + errStr + `
for k` + vc + `, v` + vc + ` := range ` + gen.genVariableName(prefix, mb.Key) + ` {
`)
	// for _, v := range can nesting for _, v := range，does not conflict, support multidimensional arrays

	dummy := &StructMember{}
	dummy.Type = mb.Type.TypeK
	dummy.Key = "k" + vc
	gen.genWriteVar(dummy, "", hasRet)

	dummy = &StructMember{}
	dummy.Type = mb.Type.TypeV
	dummy.Key = "v" + vc
	dummy.Tag = 1
	gen.genWriteVar(dummy, "", hasRet)

	c.WriteString("}\n")
}

func (gen *GenGo) genWriteVar(v *StructMember, prefix string, hasRet bool) {
	c := &gen.code

	switch v.Type.Type {
	case tkTVector:
		gen.genWriteVector(v, prefix, hasRet)
	case tkTArray:
		gen.genWriteArray(v, prefix, hasRet)
	case tkTMap:
		gen.genWriteMap(v, prefix, hasRet)
	case tkName:
		if v.Type.CType == tkEnum {
			// tkEnum enumeration processing
			tag := strconv.Itoa(int(v.Tag))
			c.WriteString(`
err = buf.WriteInt32(int32(` + gen.genVariableName(prefix, v.Key) + `),` + tag + `)
` + errString(hasRet) + `
`)
		} else {
			gen.genWriteStruct(v, prefix, hasRet)
		}
	default:
		tag := strconv.Itoa(int(v.Tag))
		c.WriteString(`
err = buf.Write` + upperFirstLetter(gen.genType(v.Type)) + `(` + gen.genVariableName(prefix, v.Key) + `, ` + tag + `)
` + errString(hasRet) + `
`)
	}
}

func (gen *GenGo) genFunWriteBlock(st *StructInfo) {
	c := &gen.code

	// WriteBlock function head
	c.WriteString(`// WriteBlock encode struct
func (st *` + st.Name + `) WriteBlock(buf *codec.Buffer, tag byte) error {
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
`)
}

func (gen *GenGo) genFunWriteTo(st *StructInfo) {
	c := &gen.code

	c.WriteString(`// WriteTo encode struct to buffer
func (st *` + st.Name + `) WriteTo(buf *codec.Buffer) (err error) {
`)
	for _, v := range st.Mb {
		gen.genWriteVar(&v, "st.", false)
	}

	c.WriteString(`
	return err
}
`)
}

func (gen *GenGo) genReadSimpleList(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	unsign := "Int8"
	if mb.Type.TypeK.Unsigned {
		unsign = "Uint8"
	}
	errStr := errString(hasRet)

	c.WriteString(`
_, err = readBuf.SkipTo(codec.BYTE, 0, true)
` + errStr + `
err = readBuf.ReadInt32(&length, 0, true)
` + errStr + `
err = readBuf.ReadSlice` + unsign + `(&` + prefix + mb.Key + `, length, true)
` + errStr + `
`)
}

func (gen *GenGo) genReadVector(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	errStr := errString(hasRet)

	// LIST
	tag := strconv.Itoa(int(mb.Tag))
	vc := strconv.Itoa(gen.vc)
	gen.vc++
	if mb.Require {
		c.WriteString(`
_, ty, err = readBuf.SkipToNoCheck(` + tag + `, true)
` + errStr + `
`)
	} else {
		c.WriteString(`
have, ty, err = readBuf.SkipToNoCheck(` + tag + `, false)
` + errStr + `
if have {`)
		// 结束标记
		defer func() {
			c.WriteString("}\n")
		}()
	}

	c.WriteString(`
if ty == codec.LIST {
	err = readBuf.ReadInt32(&length, 0, true)
  ` + errStr + `
  ` + gen.genVariableName(prefix, mb.Key) + ` = make(` + gen.genType(mb.Type) + `, length)
  ` + genForHead(vc) + `{
`)

	dummy := &StructMember{}
	dummy.Type = mb.Type.TypeK
	dummy.Key = mb.Key + "[i" + vc + "]"
	gen.genReadVar(dummy, prefix, hasRet)

	c.WriteString(`}
} else if ty == codec.SimpleList {
`)
	if mb.Type.TypeK.Type == tkTByte {
		gen.genReadSimpleList(mb, prefix, hasRet)
	} else {
		c.WriteString(`err = fmt.Errorf("not support SimpleList type")
    ` + errStr)
	}
	c.WriteString(`
} else {
  err = fmt.Errorf("require vector, but not")
  ` + errStr + `
}
`)
}

func (gen *GenGo) genReadArray(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	errStr := errString(hasRet)

	// LIST
	tag := strconv.Itoa(int(mb.Tag))
	vc := strconv.Itoa(gen.vc)
	gen.vc++

	if mb.Require {
		c.WriteString(`
_, ty, err = readBuf.SkipToNoCheck(` + tag + `, true)
` + errStr + `
`)
	} else {
		c.WriteString(`
have, ty, err = readBuf.SkipToNoCheck(` + tag + `, false)
` + errStr + `
if have {`)
		// 结束标记
		defer func() {
			c.WriteString("}\n")
		}()
	}

	c.WriteString(`
if ty == codec.LIST {
	err = readBuf.ReadInt32(&length, 0, true)
  ` + errStr + `
  ` + genForHead(vc) + `{
`)

	dummy := &StructMember{}
	dummy.Type = mb.Type.TypeK
	dummy.Key = mb.Key + "[i" + vc + "]"
	gen.genReadVar(dummy, prefix, hasRet)

	c.WriteString(`}
} else if ty == codec.SimpleList {
`)
	if mb.Type.TypeK.Type == tkTByte {
		gen.genReadSimpleList(mb, prefix, hasRet)
	} else {
		c.WriteString(`err = fmt.Errorf("not support SimpleList type")
    ` + errStr)
	}
	c.WriteString(`
} else {
  err = fmt.Errorf("require array, but not")
  ` + errStr + `
}
`)
}

func (gen *GenGo) genReadStruct(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	tag := strconv.Itoa(int(mb.Tag))
	require := "false"
	if mb.Require {
		require = "true"
	}
	c.WriteString(`
err = ` + prefix + mb.Key + `.ReadBlock(readBuf, ` + tag + `, ` + require + `)
` + errString(hasRet) + `
`)
}

func (gen *GenGo) genReadMap(mb *StructMember, prefix string, hasRet bool) {
	c := &gen.code
	tag := strconv.Itoa(int(mb.Tag))
	errStr := errString(hasRet)
	vc := strconv.Itoa(gen.vc)
	gen.vc++

	if mb.Require {
		c.WriteString(`
_, err = readBuf.SkipTo(codec.MAP, ` + tag + `, true)
` + errStr + `
`)
	} else {
		c.WriteString(`
have, err = readBuf.SkipTo(codec.MAP, ` + tag + `, false)
` + errStr + `
if have {`)
		// 结束标记
		defer func() {
			c.WriteString("}\n")
		}()
	}

	c.WriteString(`
err = readBuf.ReadInt32(&length, 0, true)
` + errStr + `
` + gen.genVariableName(prefix, mb.Key) + ` = make(` + gen.genType(mb.Type) + `)
` + genForHead(vc) + `{
	var k` + vc + ` ` + gen.genType(mb.Type.TypeK) + `
	var v` + vc + ` ` + gen.genType(mb.Type.TypeV) + `
`)

	dummy := &StructMember{}
	dummy.Type = mb.Type.TypeK
	dummy.Key = "k" + vc
	gen.genReadVar(dummy, "", hasRet)

	dummy = &StructMember{}
	dummy.Type = mb.Type.TypeV
	dummy.Key = "v" + vc
	dummy.Tag = 1
	gen.genReadVar(dummy, "", hasRet)

	c.WriteString(`
	` + prefix + mb.Key + `[k` + vc + `] = v` + vc + `
}
`)
}

func (gen *GenGo) genReadVar(v *StructMember, prefix string, hasRet bool) {
	c := &gen.code

	switch v.Type.Type {
	case tkTVector:
		gen.genReadVector(v, prefix, hasRet)
	case tkTArray:
		gen.genReadArray(v, prefix, hasRet)
	case tkTMap:
		gen.genReadMap(v, prefix, hasRet)
	case tkName:
		if v.Type.CType == tkEnum {
			require := "false"
			if v.Require {
				require = "true"
			}
			tag := strconv.Itoa(int(v.Tag))
			c.WriteString(`
err = readBuf.ReadInt32((*int32)(&` + prefix + v.Key + `),` + tag + `, ` + require + `)
` + errString(hasRet) + `
`)
		} else {
			gen.genReadStruct(v, prefix, hasRet)
		}
	default:
		require := "false"
		if v.Require {
			require = "true"
		}
		tag := strconv.Itoa(int(v.Tag))
		c.WriteString(`
err = readBuf.Read` + upperFirstLetter(gen.genType(v.Type)) + `(&` + prefix + v.Key + `, ` + tag + `, ` + require + `)
` + errString(hasRet) + `
`)
	}
}

func (gen *GenGo) genFunReadFrom(st *StructInfo) {
	c := &gen.code

	c.WriteString(`// ReadFrom reads  from readBuf and put into struct.
func (st *` + st.Name + `) ReadFrom(readBuf *codec.Reader) error {
	var (
		err error
		length int32
		have bool
		ty byte
	)
	st.ResetDefault()

`)

	for _, v := range st.Mb {
		gen.genReadVar(&v, "st.", false)
	}

	c.WriteString(`
	_ = err
	_ = length
	_ = have
	_ = ty
	return nil
}
`)
}

func (gen *GenGo) genFunReadBlock(st *StructInfo) {
	c := &gen.code

	c.WriteString(`// ReadBlock reads struct from the given tag , require or optional.
func (st *` + st.Name + `) ReadBlock(readBuf *codec.Reader, tag byte, require bool) error {
	var (
		err error
		have bool
	)
	st.ResetDefault()

	have, err = readBuf.SkipTo(codec.StructBegin, tag, require)
	if err != nil {
		return err
	}
	if !have {
		if require {
			return fmt.Errorf("require ` + st.Name + `, but not exist. tag %d", tag)
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
`)
}

func (gen *GenGo) genStruct(st *StructInfo) {
	gen.vc = 0
	st.rename()

	gen.genStructDefine(st)
	gen.genFunResetDefault(st)

	gen.genFunReadFrom(st)
	gen.genFunReadBlock(st)

	gen.genFunWriteTo(st)
	gen.genFunWriteBlock(st)
}

func (gen *GenGo) makeEnumName(en *EnumInfo, mb *EnumMember) string {
	return upperFirstLetter(en.Name) + "_" + upperFirstLetter(mb.Key)
}

func (gen *GenGo) genEnum(en *EnumInfo) {
	if len(en.Mb) == 0 {
		return
	}

	en.rename()

	c := &gen.code
	c.WriteString("type " + en.Name + " int32\n")
	c.WriteString("const (\n")
	var it int32
	for _, v := range en.Mb {
		if v.Type == 0 {
			//use value
			c.WriteString(gen.makeEnumName(en, &v) + ` = ` + strconv.Itoa(int(v.Value)) + "\n")
			it = v.Value + 1
		} else if v.Type == 1 {
			// use name
			find := false
			for _, ref := range en.Mb {
				if ref.Key == v.Name {
					find = true
					c.WriteString(gen.makeEnumName(en, &v) + ` = ` + gen.makeEnumName(en, &ref) + "\n")
					it = ref.Value + 1
					break
				}
				if ref.Key == v.Key {
					break
				}
			}
			if !find {
				gen.genErr(v.Name + " not define before use.")
			}
		} else {
			// use auto add
			c.WriteString(gen.makeEnumName(en, &v) + ` = ` + strconv.Itoa(int(it)) + "\n")
			it++
		}

	}

	c.WriteString(")\n")
}

func (gen *GenGo) genConst(cst []ConstInfo) {
	if len(cst) == 0 {
		return
	}

	c := &gen.code
	c.WriteString("//const as define in tars file\n")
	c.WriteString("const (\n")

	for _, v := range gen.p.Const {
		v.rename()
		c.WriteString(v.Name + " " + gen.genType(v.Type) + " = " + v.Value + "\n")
	}

	c.WriteString(")\n")
}

func (gen *GenGo) genInclude(ps []*Parse) {
	for _, v := range ps {
		gen2 := &GenGo{
			path:      v.Source,
			module:    gen.module,
			prefix:    gen.prefix,
			tarsPath:  gTarsPath,
			ProtoName: path2ProtoName(v.Source),
		}
		gen2.p = v
		gen2.genAll()
	}
}

func (gen *GenGo) genInterface(itf *InterfaceInfo) {
	gen.code.Reset()
	itf.rename()

	gen.genHead()
	gen.genIFPackage(itf)

	gen.genIFProxy(itf)

	gen.genIFServer(itf)
	gen.genIFServerWithContext(itf)

	gen.genIFDispatch(itf)

	gen.saveToSourceFile(itf.Name + ".tars.go")
}

func (gen *GenGo) genIFProxy(itf *InterfaceInfo) {
	c := &gen.code
	c.WriteString("// " + itf.Name + " struct\n")
	c.WriteString("type " + itf.Name + ` struct {
	servant m.Servant
}
`)

	c.WriteString(`// SetServant sets servant for the service.
func (obj *` + itf.Name + `) SetServant(servant m.Servant) {
	obj.servant = servant
}
`)
	c.WriteString(`// TarsSetTimeout sets the timeout for the servant which is in ms.
func (obj *` + itf.Name + `) TarsSetTimeout(timeout int) {
	obj.servant.TarsSetTimeout(timeout)
}
`)

	c.WriteString(`// TarsSetProtocol sets the protocol for the servant.
func (obj *` + itf.Name + `) TarsSetProtocol(p m.Protocol) {
	obj.servant.TarsSetProtocol(p)
}
`)

	c.WriteString(`// Endpoints returns all active endpoint.Endpoint
func (obj *` + itf.Name + `) Endpoints() []*endpoint.Endpoint {
	return obj.servant.Endpoints()
}
`)

	if *gAddServant {
		c.WriteString(`// AddServant adds servant  for the service.
func (obj *` + itf.Name + `) AddServant(imp ` + itf.Name + `Servant, servantObj string) {
  tars.AddServant(obj, imp, servantObj)
}
`)
		c.WriteString(`// AddServantWithContext adds servant  for the service with context.
func (obj *` + itf.Name + `) AddServantWithContext(imp ` + itf.Name + `ServantWithContext, servantObj string) {
  tars.AddServantWithContext(obj, imp, servantObj)
}
`)
	}

	for _, v := range itf.Fun {
		gen.genIFProxyFun(itf.Name, &v, false, false)
		gen.genIFProxyFun(itf.Name, &v, true, false)
		gen.genIFProxyFun(itf.Name, &v, true, true)
	}
}

func (gen *GenGo) genIFProxyFun(interfName string, fun *FunInfo, withContext bool, isOneWay bool) {
	c := &gen.code
	if withContext {
		if isOneWay {
			c.WriteString("// " + fun.Name + "OneWayWithContext is the proxy function for the method defined in the tars file, with the context\n")
			c.WriteString("func (obj *" + interfName + ") " + fun.Name + "OneWayWithContext(tarsCtx context.Context,")
		} else {
			c.WriteString("// " + fun.Name + "WithContext is the proxy function for the method defined in the tars file, with the context\n")
			c.WriteString("func (obj *" + interfName + ") " + fun.Name + "WithContext(tarsCtx context.Context,")
		}
	} else {
		c.WriteString("// " + fun.Name + " is the proxy function for the method defined in the tars file, with the context\n")
		c.WriteString("func (obj *" + interfName + ") " + fun.Name + "(")
	}
	for _, v := range fun.Args {
		gen.genArgs(&v)
	}

	c.WriteString(" opts ...map[string]string)")

	// not WithContext caller WithContext method
	if !withContext {
		if fun.HasRet {
			c.WriteString("(" + gen.genType(fun.RetType) + ", error) {\n")
		} else {
			c.WriteString("error { \n")
		}

		c.WriteString("return obj." + fun.Name + "WithContext(context.Background(),")
		for _, v := range fun.Args {
			c.WriteString(v.Name + ",")
		}
		c.WriteString(" opts ...)\n")
		c.WriteString("}\n")
		return
	}

	if fun.HasRet {
		c.WriteString("(ret " + gen.genType(fun.RetType) + ", err error) {\n")
	} else {
		c.WriteString("(err error) {\n")
	}

	c.WriteString(`	var (
		length int32
		have bool
		ty byte
	)
  `)
	c.WriteString("buf := codec.NewBuffer()")
	var isOut bool
	for k, v := range fun.Args {
		if v.IsOut {
			isOut = true
		}
		dummy := &StructMember{}
		dummy.Type = v.Type
		dummy.Key = v.Name
		dummy.Tag = int32(k + 1)
		if v.IsOut {
			dummy.Key = "(*" + dummy.Key + ")"
		}
		gen.genWriteVar(dummy, "", fun.HasRet)
	}
	// empty args and below separate
	c.WriteString("\n")
	errStr := errString(fun.HasRet)

	// trace
	if !isOneWay && !withoutTrace {
		c.WriteString(`
trace, ok := current.GetTarsTrace(tarsCtx)
if ok && trace.Call() {
	var traceParam string
	trace.NewSpan()
	traceParamFlag := trace.NeedTraceParam(tarstrace.EstCS, uint(buf.Len()))
	if traceParamFlag == tarstrace.EnpNormal {
		value := map[string]interface{}{}
`)
		for _, v := range fun.Args {
			if !v.IsOut {
				c.WriteString(`value["` + v.Name + `"] = ` + v.Name + "\n")
			}
		}
		c.WriteString(`jm, _ := json.Marshal(value)
		traceParam = string(jm)
	} else if traceParamFlag == tarstrace.EnpOverMaxLen {
`)
		c.WriteString("traceParam = `{\"trace_param_over_max_len\":true}`")
		c.WriteString(`
	}
	tars.Trace(trace.GetTraceKey(tarstrace.EstCS), tarstrace.AnnotationCS, tars.GetClientConfig().ModuleName, obj.servant.Name(), "` + fun.Name + `", 0, traceParam, "")
}`)
		c.WriteString("\n\n")
	}
	c.WriteString(`var statusMap map[string]string
var contextMap map[string]string
if len(opts) == 1{
	contextMap =opts[0]
}else if len(opts) == 2 {
	contextMap = opts[0]
	statusMap = opts[1]
}

tarsResp := new(requestf.ResponsePacket)`)

	if isOneWay {
		c.WriteString(`
		err = obj.servant.TarsInvoke(tarsCtx, 1, "` + fun.OriginName + `", buf.ToBytes(), statusMap, contextMap, tarsResp)
		` + errStr + `
		`)
	} else {
		c.WriteString(`
		err = obj.servant.TarsInvoke(tarsCtx, 0, "` + fun.OriginName + `", buf.ToBytes(), statusMap, contextMap, tarsResp)
		` + errStr + `
		`)
	}

	if (isOut || fun.HasRet) && !isOneWay {
		c.WriteString("readBuf := codec.NewReader(tools.Int8ToByte(tarsResp.SBuffer))")
	}
	if fun.HasRet && !isOneWay {
		dummy := &StructMember{}
		dummy.Type = fun.RetType
		dummy.Key = "ret"
		dummy.Tag = 0
		dummy.Require = true
		gen.genReadVar(dummy, "", fun.HasRet)
	}

	if !isOneWay {
		for k, v := range fun.Args {
			if v.IsOut {
				dummy := &StructMember{}
				dummy.Type = v.Type
				dummy.Key = "(*" + v.Name + ")"
				dummy.Tag = int32(k + 1)
				dummy.Require = true
				gen.genReadVar(dummy, "", fun.HasRet)
			}
		}
		if withContext && !withoutTrace {
			traceParamFlag := "traceParamFlag := trace.NeedTraceParam(tarstrace.EstCR, uint(0))"
			if isOut || fun.HasRet {
				traceParamFlag = "traceParamFlag := trace.NeedTraceParam(tarstrace.EstCR, uint(readBuf.Len()))"
			}
			c.WriteString(`
if ok && trace.Call() {
	var traceParam string
	` + traceParamFlag + `
	if traceParamFlag == tarstrace.EnpNormal {
		value := map[string]interface{}{}
`)
			if fun.HasRet {
				c.WriteString(`value[""] = ret` + "\n")
			}
			for _, v := range fun.Args {
				if v.IsOut {
					c.WriteString(`value["` + v.Name + `"] = *` + v.Name + "\n")
				}
			}
			c.WriteString(`jm, _ := json.Marshal(value)
		traceParam = string(jm)
	} else if traceParamFlag == tarstrace.EnpOverMaxLen {
`)
			c.WriteString("traceParam = `{\"trace_param_over_max_len\":true}`")
			c.WriteString(`
	}
	tars.Trace(trace.GetTraceKey(tarstrace.EstCR), tarstrace.AnnotationCR, tars.GetClientConfig().ModuleName, obj.servant.Name(), "` + fun.Name + `", tarsResp.IRet, traceParam, "")
}`)
			c.WriteString("\n\n")
		}

		c.WriteString(`
	if len(opts) == 1 {
		for k := range(contextMap){
			delete(contextMap, k)
		}
		for k, v := range(tarsResp.Context){
			contextMap[k] = v
		}
	} else if len(opts) == 2 {
		for k := range(contextMap){
			delete(contextMap, k)
		}
		for k, v := range(tarsResp.Context){
			contextMap[k] = v
		}
		for k := range(statusMap){
			delete(statusMap, k)
		}
		for k, v := range(tarsResp.Status){
			statusMap[k] = v
		}
	}`)
	}

	c.WriteString(`
  _ = length
  _ = have
  _ = ty
`)
	if fun.HasRet {
		c.WriteString("return ret, nil\n")
	} else {
		c.WriteString("return nil\n")
	}

	c.WriteString("}\n")
}

func (gen *GenGo) genArgs(arg *ArgInfo) {
	c := &gen.code
	c.WriteString(arg.Name + " ")
	if arg.IsOut || arg.Type.CType == tkStruct {
		c.WriteString("*")
	}

	c.WriteString(gen.genType(arg.Type) + ",")
}

func (gen *GenGo) genIFServer(itf *InterfaceInfo) {
	c := &gen.code
	c.WriteString("type " + itf.Name + "Servant interface {\n")
	for _, v := range itf.Fun {
		gen.genIFServerFun(&v)
	}
	c.WriteString("}\n")
}

func (gen *GenGo) genIFServerWithContext(itf *InterfaceInfo) {
	c := &gen.code
	c.WriteString("type " + itf.Name + "ServantWithContext interface {\n")
	for _, v := range itf.Fun {
		gen.genIFServerFunWithContext(&v)
	}
	c.WriteString("} \n")
}

func (gen *GenGo) genIFServerFun(fun *FunInfo) {
	c := &gen.code
	c.WriteString(fun.Name + "(")
	for _, v := range fun.Args {
		gen.genArgs(&v)
	}
	c.WriteString(")(")

	if fun.HasRet {
		c.WriteString("ret " + gen.genType(fun.RetType) + ", ")
	}
	c.WriteString("err error)\n")
}

func (gen *GenGo) genIFServerFunWithContext(fun *FunInfo) {
	c := &gen.code
	c.WriteString(fun.Name + "(tarsCtx context.Context, ")
	for _, v := range fun.Args {
		gen.genArgs(&v)
	}
	c.WriteString(")(")

	if fun.HasRet {
		c.WriteString("ret " + gen.genType(fun.RetType) + ", ")
	}
	c.WriteString("err error)\n")
}

func (gen *GenGo) genIFDispatch(itf *InterfaceInfo) {
	c := &gen.code
	c.WriteString("// Dispatch is used to call the server side implement for the method defined in the tars file. withContext shows using context or not.  \n")
	c.WriteString("func(obj *" + itf.Name + `) Dispatch(tarsCtx context.Context, val interface{}, tarsReq *requestf.RequestPacket, tarsResp *requestf.ResponsePacket, withContext bool) (err error) {
	var (
		length int32
		have bool
		ty byte
	)
  `)

	var param bool
	for _, v := range itf.Fun {
		if len(v.Args) > 0 {
			param = true
			break
		}
	}

	if param {
		c.WriteString("readBuf := codec.NewReader(tools.Int8ToByte(tarsReq.SBuffer))")
	} else {
		c.WriteString("readBuf := codec.NewReader(nil)")
	}
	c.WriteString(`
	buf := codec.NewBuffer()
	switch tarsReq.SFuncName {
`)

	for _, v := range itf.Fun {
		gen.genSwitchCase(itf.Name, &v)
	}

	c.WriteString(`
	default:
		return fmt.Errorf("func mismatch")
	}
	var statusMap map[string]string
	if status, ok := current.GetResponseStatus(tarsCtx); ok  && status != nil {
		statusMap = status
	}
	var contextMap map[string]string
	if ctx, ok := current.GetResponseContext(tarsCtx); ok && ctx != nil  {
		contextMap = ctx
	}
	*tarsResp = requestf.ResponsePacket{
		IVersion:     tarsReq.IVersion,
		CPacketType:  0,
		IRequestId:   tarsReq.IRequestId,
		IMessageType: 0,
		IRet:         0,
		SBuffer:      tools.ByteToInt8(buf.ToBytes()),
		Status:       statusMap,
		SResultDesc:  "",
		Context:      contextMap,
	}

	_ = readBuf
	_ = buf
	_ = length
	_ = have
	_ = ty
	return nil
}
`)
}

func (gen *GenGo) genSwitchCase(tname string, fun *FunInfo) {
	c := &gen.code
	c.WriteString(`case "` + fun.OriginName + `":` + "\n")

	inArgsCount := 0
	outArgsCount := 0
	for _, v := range fun.Args {
		c.WriteString("var " + v.Name + " " + gen.genType(v.Type) + "\n")
		if v.Type.Type == tkTMap {
			c.WriteString(v.Name + " = make(" + gen.genType(v.Type) + ")\n")
		} else if v.Type.Type == tkTVector {
			c.WriteString(v.Name + " = make(" + gen.genType(v.Type) + ", 0)\n")
		}
		if v.IsOut {
			outArgsCount++
		} else {
			inArgsCount++
		}
	}

	//fmt.Println("args count, in, out:", inArgsCount, outArgsCount)

	c.WriteString("\n")

	if inArgsCount > 0 {
		c.WriteString("if tarsReq.IVersion == basef.TARSVERSION {\n")

		for k, v := range fun.Args {

			if !v.IsOut {
				dummy := &StructMember{}
				dummy.Type = v.Type
				dummy.Key = v.Name
				dummy.Tag = int32(k + 1)
				dummy.Require = true
				gen.genReadVar(dummy, "", false)
			}
		}

		c.WriteString(`} else if tarsReq.IVersion == basef.TUPVERSION {
		reqTup := tup.NewUniAttribute()
		reqTup.Decode(readBuf)

		var tupBuffer []byte

		`)
		for _, v := range fun.Args {
			if !v.IsOut {
				c.WriteString("\n")
				c.WriteString(`reqTup.GetBuffer("` + v.Name + `", &tupBuffer)` + "\n")
				c.WriteString("readBuf.Reset(tupBuffer)")

				dummy := &StructMember{}
				dummy.Type = v.Type
				dummy.Key = v.Name
				dummy.Tag = 0
				dummy.Require = true
				gen.genReadVar(dummy, "", false)
			}
		}

		c.WriteString(`} else if tarsReq.IVersion == basef.JSONVERSION {
		var jsonData map[string]interface{}
		decoder := json.NewDecoder(bytes.NewReader(readBuf.ToBytes()))
		decoder.UseNumber()
		err = decoder.Decode(&jsonData)
		if err != nil {
			return fmt.Errorf("decode reqpacket failed, error: %+v", err)
		}
		`)

		for _, v := range fun.Args {
			if !v.IsOut {
				c.WriteString("{\n")
				c.WriteString(`jsonStr, _ := json.Marshal(jsonData["` + v.Name + `"])` + "\n")
				if v.Type.CType == tkStruct {
					c.WriteString(v.Name + ".ResetDefault()\n")
				}
				c.WriteString("if err = json.Unmarshal(jsonStr, &" + v.Name + "); err != nil {")
				c.WriteString(`
					return err
				}
				}
				`)
			}
		}

		c.WriteString(`
		} else {
			err = fmt.Errorf("decode reqpacket fail, error version: %d", tarsReq.IVersion)
			return err
		}`)

		c.WriteString("\n\n")
	}
	if !withoutTrace {
		c.WriteString(`
trace, ok := current.GetTarsTrace(tarsCtx)
if ok && trace.Call() {
	var traceParam string
	traceParamFlag := trace.NeedTraceParam(tarstrace.EstSR, uint(readBuf.Len()))
	if traceParamFlag == tarstrace.EnpNormal {
		value := map[string]interface{}{}
`)
		for _, v := range fun.Args {
			if !v.IsOut {
				c.WriteString(`value["` + v.Name + `"] = ` + v.Name + "\n")
			}
		}
		c.WriteString(`jm, _ := json.Marshal(value)
		traceParam = string(jm)
	} else if traceParamFlag == tarstrace.EnpOverMaxLen {
`)
		c.WriteString("traceParam = `{\"trace_param_over_max_len\":true}`")
		c.WriteString(`
	}
	tars.Trace(trace.GetTraceKey(tarstrace.EstSR), tarstrace.AnnotationSR, tars.GetClientConfig().ModuleName, tarsReq.SServantName, "` + fun.OriginName + `", 0, traceParam, "")
}`)
		c.WriteString("\n\n")
	}

	if fun.HasRet {
		c.WriteString("var funRet " + gen.genType(fun.RetType) + "\n")

		c.WriteString(`if !withContext {
		imp := val.(` + tname + `Servant)
		funRet, err = imp.` + fun.Name + `(`)
		for _, v := range fun.Args {
			if v.IsOut || v.Type.CType == tkStruct {
				c.WriteString("&" + v.Name + ",")
			} else {
				c.WriteString(v.Name + ",")
			}
		}
		c.WriteString(")")

		c.WriteString(`
		} else {
		imp := val.(` + tname + `ServantWithContext)
		funRet, err = imp.` + fun.Name + `(tarsCtx ,`)
		for _, v := range fun.Args {
			if v.IsOut || v.Type.CType == tkStruct {
				c.WriteString("&" + v.Name + ",")
			} else {
				c.WriteString(v.Name + ",")
			}
		}
		c.WriteString(") \n}\n")

	} else {
		c.WriteString(`if !withContext {
		imp := val.(` + tname + `Servant)
		err = imp.` + fun.Name + `(`)
		for _, v := range fun.Args {
			if v.IsOut || v.Type.CType == tkStruct {
				c.WriteString("&" + v.Name + ",")
			} else {
				c.WriteString(v.Name + ",")
			}
		}
		c.WriteString(")")

		c.WriteString(`
		} else {
		imp := val.(` + tname + `ServantWithContext)
		err = imp.` + fun.Name + `(tarsCtx ,`)
		for _, v := range fun.Args {
			if v.IsOut || v.Type.CType == tkStruct {
				c.WriteString("&" + v.Name + ",")
			} else {
				c.WriteString(v.Name + ",")
			}
		}
		c.WriteString(") \n}\n")
	}

	if *dispatchReporter {
		var inArgStr, outArgStr, retArgStr string
		if fun.HasRet {
			retArgStr = "funRet, err"
		} else {
			retArgStr = "err"
		}
		for _, v := range fun.Args {
			prefix := ""
			if v.Type.CType == tkStruct {
				prefix = "&"
			}
			if v.IsOut {
				outArgStr += prefix + v.Name + ","
			} else {
				inArgStr += prefix + v.Name + ","
			}
		}
		c.WriteString(`if dp := tars.GetDispatchReporter(); dp != nil {
			dp(tarsCtx, []interface{}{` + inArgStr + `}, []interface{}{` + outArgStr + `}, []interface{}{` + retArgStr + `})
		}`)

	}
	c.WriteString(`
	if err != nil {
		return err
	}
	`)

	c.WriteString(`
	if tarsReq.IVersion == basef.TARSVERSION {
	buf.Reset()
	`)

	if fun.HasRet {
		dummy := &StructMember{}
		dummy.Type = fun.RetType
		dummy.Key = "funRet"
		dummy.Tag = 0
		dummy.Require = true
		gen.genWriteVar(dummy, "", false)
	}

	for k, v := range fun.Args {
		if v.IsOut {
			dummy := &StructMember{}
			dummy.Type = v.Type
			dummy.Key = v.Name
			dummy.Tag = int32(k + 1)
			dummy.Require = true
			gen.genWriteVar(dummy, "", false)
		}
	}

	c.WriteString(`
} else if tarsReq.IVersion == basef.TUPVERSION {
rspTup := tup.NewUniAttribute()
`)
	if fun.HasRet {
		dummy := &StructMember{}
		dummy.Type = fun.RetType
		dummy.Key = "funRet"
		dummy.Tag = 0
		dummy.Require = true
		gen.genWriteVar(dummy, "", false)

		c.WriteString(`
		rspTup.PutBuffer("", buf.ToBytes())
		rspTup.PutBuffer("tars_ret", buf.ToBytes())
`)
	}

	for _, v := range fun.Args {
		if v.IsOut {
			c.WriteString(`
		buf.Reset()`)
			dummy := &StructMember{}
			dummy.Type = v.Type
			dummy.Key = v.Name
			dummy.Tag = 0
			dummy.Require = true
			gen.genWriteVar(dummy, "", false)

			c.WriteString(`rspTup.PutBuffer("` + v.Name + `", buf.ToBytes())` + "\n")
		}
	}

	c.WriteString(`
	buf.Reset()
	err = rspTup.Encode(buf)
	if err != nil {
		return err
	}
} else if tarsReq.IVersion == basef.JSONVERSION {
	rspJson := map[string]interface{}{}
`)
	if fun.HasRet {
		c.WriteString(`rspJson["tars_ret"] = funRet` + "\n")
	}

	for _, v := range fun.Args {
		if v.IsOut {
			c.WriteString(`rspJson["` + v.Name + `"] = ` + v.Name + "\n")
		}
	}

	c.WriteString(`
		var rspByte []byte
		if rspByte, err = json.Marshal(rspJson); err != nil {
			return err
		}

		buf.Reset()
		err = buf.WriteSliceUint8(rspByte)
		if err != nil {
			return err
		}
}`)

	c.WriteString("\n")
	if !withoutTrace {
		c.WriteString(`
if ok && trace.Call() {
	var traceParam string
	traceParamFlag := trace.NeedTraceParam(tarstrace.EstSS, uint(buf.Len()))
	if traceParamFlag == tarstrace.EnpNormal {
		value := map[string]interface{}{}
`)
		if fun.HasRet {
			c.WriteString(`value[""] = funRet` + "\n")
		}
		for _, v := range fun.Args {
			if v.IsOut {
				c.WriteString(`value["` + v.Name + `"] = ` + v.Name + "\n")
			}
		}
		c.WriteString(`jm, _ := json.Marshal(value)
		traceParam = string(jm)
	} else if traceParamFlag == tarstrace.EnpOverMaxLen {
`)
		c.WriteString("traceParam = `{\"trace_param_over_max_len\":true}`")
		c.WriteString(`
}
	tars.Trace(trace.GetTraceKey(tarstrace.EstSS), tarstrace.AnnotationSS, tars.GetClientConfig().ModuleName, tarsReq.SServantName, "` + fun.OriginName + `", 0, traceParam, "")
}
`)
	}
}
