// package util implements some utility functions for parsing/converting gir files
// TODO: Maybe some of this can more easily be done with regexes?
//
//	But using regexes introduces 2 problems :^)
package util

import (
	"path/filepath"
	"strings"
)

// glibTypeConfig defines a mapping between Go types, GLib types, and their getter/setter methods
type glibTypeConfig struct {
	GoType           string
	GLibType         string
	SetterMethod     string
	GetterMethod     string
	SetterTemplate   string
	GetterTemplate   string
	CustomSetterFunc func(valueName, objPrefix string) string
	CustomGetterFunc func(goType, baseGoType string, isInterface, isRecord bool) string
}

// vectorTypeConfig defines a mapping for vector/array types that need special handling
type vectorTypeConfig struct {
	GoType               string
	GLibType             string
	SetterFunction       func(objPrefix, glibPrefix, propertyName string, useBaseObj bool) string
	GetterFunction       func(objPrefix, corePrefix, propertyName string, useBaseObj bool) string
	UsesStrvGetType      bool
	PropertySetterMethod string
	PropertyGetterMethod string
}

// glibTypeConstant defines a GLib type constant with its name and value
// This matches the constants defined in templates/gobject
type glibTypeConstant struct {
	Name  string // The constant name (e.g. "TypeBooleanVal")
	Value string // The constant value expression (e.g. "5 << 2")
}

func generateStringSliceSetter(objPrefix, glibPrefix, propertyName string, useBaseObj bool) string {
	objAccess := "x"
	result := `var v ` + objPrefix + `Value
     v.Init(` + glibPrefix + `StrvGetType())

     cStrBytes := make([][]byte, len(value))
     cStrings := make([]uintptr, len(value)+1)
     for i, s := range value {
          cStrBytes[i] = make([]byte, len(s)+1)
          copy(cStrBytes[i], s)
          cStrBytes[i][len(s)] = 0
          cStrings[i] = uintptr(unsafe.Pointer(&cStrBytes[i][0]))
     }
     cStrings[len(value)] = 0

     v.SetBoxed(uintptr(unsafe.Pointer(&cStrings[0])))`

	if useBaseObj {
		objAccess = "obj"
		result += "\n     obj := " + objPrefix + "Object{Ptr: x.GoPointer()}"
	}

	result += "\n     " + objAccess + `.SetProperty("` + propertyName + `", &v)

     v.Unset()`
	return result
}

func generateStringSliceGetter(objPrefix, corePrefix, propertyName string, useBaseObj bool) string {
	objAccess := "x"
	result := `var v ` + objPrefix + `Value`

	if useBaseObj {
		objAccess = "obj"
		result += "\n     obj := " + objPrefix + "Object{Ptr: x.GoPointer()}"
	}

	result += `
     ` + objAccess + `.GetProperty("` + propertyName + `", &v)
     defer v.Unset()

     strvPtr := v.GetBoxed()
     if strvPtr == 0 {
          return nil
     }

     var result []string
     for i := 0; ; i++ {
          charPtr := *(*uintptr)(unsafe.Pointer(strvPtr + uintptr(i)*unsafe.Sizeof(uintptr(0))))
          if charPtr == 0 {
               break
          }
          result = append(result, ` + corePrefix + `GoString(charPtr))
     }

     return result`
	return result
}

func generateByteSliceSetter(objPrefix, glibPrefix, propertyName string, useBaseObj bool) string {
	objAccess := "x"
	result := `var v ` + objPrefix + `Value
     v.Init(` + objPrefix + `TypePointerVal)

     if len(value) > 0 {
          v.SetPointer(uintptr(unsafe.Pointer(&value[0])))
     } else {
          v.SetPointer(0)
     }`

	if useBaseObj {
		objAccess = "obj"
		result += "\n     obj := " + objPrefix + "Object{Ptr: x.GoPointer()}"
	}

	result += "\n     " + objAccess + `.SetProperty("` + propertyName + `", &v)

     v.Unset()`
	return result
}

func generateByteSliceGetter(objPrefix, corePrefix, propertyName string, useBaseObj bool) string {
	objAccess := "x"
	result := `var v ` + objPrefix + `Value`

	if useBaseObj {
		objAccess = "obj"
		result += "\n     obj := " + objPrefix + "Object{Ptr: x.GoPointer()}"
	}

	result += `
     ` + objAccess + `.GetProperty("` + propertyName + `", &v)
     defer v.Unset()

     ptr := v.GetPointer()
     if ptr == 0 {
          return nil
     }

     return *(*[]byte)(unsafe.Pointer(ptr))`
	return result
}

var (
	// Variable names that should not be dereferenced when using ConvertPtr() in handlePtr mode
	// TODO: This was mostly discovered via trial and error, and might point towards issues in
	// the GIR files
	specialConvertPtrVars = []string{
		"ModelVar",
		"TreeModelVar",
		"OutChildVar",
		"ChildVar",
	}

	goToGLibTypeConfigs = []glibTypeConfig{
		{GoType: "bool", GLibType: "TypeBooleanVal", SetterMethod: "SetBoolean", GetterMethod: "GetBoolean"},
		{GoType: "int", GLibType: "TypeIntVal", SetterMethod: "SetInt", GetterMethod: "GetInt"},
		{GoType: "int64", GLibType: "TypeInt64Val", SetterMethod: "SetInt64", GetterMethod: "GetInt64"},
		{GoType: "uint", GLibType: "TypeUintVal", SetterMethod: "SetUint", GetterMethod: "GetUint"},
		{GoType: "uint64", GLibType: "TypeUint64Val", SetterMethod: "SetUint64", GetterMethod: "GetUint64"},
		{GoType: "float32", GLibType: "TypeFloatVal", SetterMethod: "SetFloat", GetterMethod: "GetFloat"},
		{GoType: "float64", GLibType: "TypeDoubleVal", SetterMethod: "SetDouble", GetterMethod: "GetDouble"},
		{GoType: "string", GLibType: "TypeStringVal", SetterMethod: "SetString", GetterMethod: "GetString"},
		{GoType: "uintptr", GLibType: "TypePointerVal", SetterMethod: "SetPointer", GetterMethod: "GetPointer"},
		{GoType: "byte", GLibType: "TypeUcharVal", SetterMethod: "SetUchar", GetterMethod: "GetUchar"},
		{GoType: "int32", GLibType: "TypeIntVal", SetterTemplate: "v.SetInt(int(%s))", GetterTemplate: "return int32(v.GetInt())"},
		{GoType: "uint32", GLibType: "TypeUintVal", SetterTemplate: "v.SetUint(uint(%s))", GetterTemplate: "return uint32(v.GetUint())"},
	}

	internalGLibTypeConfigs = map[string]glibTypeConfig{
		"TypeEnumVal": {
			GLibType:       "TypeEnumVal",
			SetterTemplate: "v.SetEnum(int(%s))",
			GetterTemplate: "%s(v.GetEnum())",
		},
		"TypeFlagsVal": {
			GLibType:       "TypeFlagsVal",
			SetterTemplate: "v.SetFlags(uint(%s))",
			GetterTemplate: "%s(v.GetFlags())",
		},
		"TypeGtypeVal": {
			GLibType:     "TypeGtypeVal",
			SetterMethod: "SetGtype",
			GetterMethod: "GetGtype",
		},
		"TypeObjectVal": {
			GLibType: "TypeObjectVal",
			CustomSetterFunc: func(valueName, objPrefix string) string {
				return "v.SetObject(&" + objPrefix + "Object{Ptr: " + valueName + ".GoPointer()})"
			},
			CustomGetterFunc: func(goType, baseGoType string, isInterface, isRecord bool) string {
				result := "ptr := v.GetObject().GoPointer(); if ptr == 0 { return nil }; "
				if isInterface {
					result += "result := &" + baseGoType + "Base{}; result.Ptr = ptr; return result"
				} else if isRecord {
					result += "return (*" + baseGoType + ")(unsafe.Pointer(ptr))"
				} else {
					result += "result := &" + baseGoType + "{}; result.Ptr = ptr; return result"
				}
				return result
			},
		},
		"TypePointerVal": {
			GLibType:     "TypePointerVal",
			SetterMethod: "SetPointer",
			GetterMethod: "GetPointer",
			CustomGetterFunc: func(goType, baseGoType string, isInterface, isRecord bool) string {
				return "return nil"
			},
		},
	}

	vectorTypeConfigs = map[string]vectorTypeConfig{
		"[]string": {
			GoType:               "[]string",
			PropertySetterMethod: "SetBoxed",
			PropertyGetterMethod: "GetBoxed",
			UsesStrvGetType:      true,
			SetterFunction:       generateStringSliceSetter,
			GetterFunction:       generateStringSliceGetter,
		},
		"[]byte": {
			GoType:               "[]byte",
			GLibType:             "TypePointerVal",
			PropertySetterMethod: "SetPointer",
			PropertyGetterMethod: "GetPointer",
			UsesStrvGetType:      false,
			SetterFunction:       generateByteSliceSetter,
			GetterFunction:       generateByteSliceGetter,
		},
	}

	glibTypeConstants = []glibTypeConstant{
		{Name: "TypeInvalidVal", Value: "0"},
		{Name: "TypeNoneVal", Value: "1 << 2"},
		{Name: "TypeInterfaceVal", Value: "2 << 2"},
		{Name: "TypeCharVal", Value: "3 << 2"},
		{Name: "TypeUcharVal", Value: "4 << 2"},
		{Name: "TypeBooleanVal", Value: "5 << 2"},
		{Name: "TypeIntVal", Value: "6 << 2"},
		{Name: "TypeUintVal", Value: "7 << 2"},
		{Name: "TypeLongVal", Value: "8 << 2"},
		{Name: "TypeUlongVal", Value: "9 << 2"},
		{Name: "TypeInt64Val", Value: "10 << 2"},
		{Name: "TypeUint64Val", Value: "11 << 2"},
		{Name: "TypeEnumVal", Value: "12 << 2"},
		{Name: "TypeFlagsVal", Value: "13 << 2"},
		{Name: "TypeFloatVal", Value: "14 << 2"},
		{Name: "TypeDoubleVal", Value: "15 << 2"},
		{Name: "TypeStringVal", Value: "16 << 2"},
		{Name: "TypePointerVal", Value: "17 << 2"},
		{Name: "TypeBoxedVal", Value: "18 << 2"},
		{Name: "TypeParamVal", Value: "19 << 2"},
		{Name: "TypeObjectVal", Value: "20 << 2"},
		{Name: "TypeReservedGLibLastVal", Value: "31 << 2"},
		{Name: "TypeReservedBseFirstVal", Value: "32 << 2"},
		{Name: "TypeReservedBseLastVal", Value: "48 << 2"},
		{Name: "TypeReservedUserFirstVal", Value: "49 << 2"},
		// TypeGtypeVal is special, it's initialized at runtime via g_gtype_get_type()
	}

	specialTypeToGLibType = map[string]string{
		"TypeGtype":   "TypeGtypeVal",
		"types.GType": "TypeGtypeVal",
		"enum":        "TypeEnumVal",
		"flags":       "TypeFlagsVal",
		"bitfield":    "TypeFlagsVal",
		"object":      "TypeObjectVal",
		"pointer":     "TypePointerVal",
		"slice":       "TypePointerVal",
	}
)

func gGLibTypeConfigByGoType(goType string) *glibTypeConfig {
	for _, config := range goToGLibTypeConfigs {
		if config.GoType == goType {
			return &config
		}
	}

	return nil
}

func gLibTypeConfigByGLibType(glibType string) *glibTypeConfig {
	if mapping, ok := internalGLibTypeConfigs[glibType]; ok {
		return &mapping
	}

	return nil
}

// IsVectorType returns true if the given Go type is a vector/array type with special handling
func IsVectorType(goType string) bool {
	_, ok := vectorTypeConfigs[goType]

	return ok
}

// GetGLibTypeConstant returns the GLib type constant name for well-known special types
// Returns empty string if not a known special type
func GetGLibTypeConstant(typeName string) string {
	if constant, ok := specialTypeToGLibType[typeName]; ok {
		return constant
	}

	return ""
}

// GetAllGLibTypeConstants returns all GLib type constant definitions
// This can be used to generate the constants block in templates/gobject
func GetAllGLibTypeConstants() []glibTypeConstant {
	return glibTypeConstants
}

// GetGLibTypeConstantValue returns the value expression for a given constant name
// Returns empty string if the constant is not found
func GetGLibTypeConstantValue(constantName string) string {
	for _, constant := range glibTypeConstants {
		if constant.Name == constantName {
			return constant.Value
		}
	}

	return ""
}

// GetGLibTypeConstants generates the Go const block for all GLib type constants
// This is used in templates to generate the type constants block
func GetGLibTypeConstants() string {
	var result strings.Builder
	result.WriteString("// types\n")
	result.WriteString("const (\n")
	for i, constant := range glibTypeConstants {
		if i == 0 {
			// First constant gets the Type annotation
			result.WriteString("\t" + constant.Name + " Type = " + constant.Value + "\n")
		} else {
			// Subsequent constants omit the type
			result.WriteString("\t" + constant.Name + " = " + constant.Value + "\n")
		}
	}
	result.WriteString(")\n")
	return result.String()
}

// GGLibTypeByGoType returns the GLib type constant for a given Go type
func GGLibTypeByGoType(goType string) string {
	if config := gGLibTypeConfigByGoType(goType); config != nil {
		return config.GLibType
	}

	return ""
}

// delimToCamel to camel converts a string with parts separated by `delim` to CamelCase
func delimToCamel(s string, delim string) string {
	var sb strings.Builder
	words := strings.Split(s, delim)
	for _, w := range words {
		sb.WriteString(strings.Title(w))
	}
	return sb.String()
}

// StarsInFront adds pointer characters (*, stars) in front of the type
// if there is a slice in front
// we need to add the slice and then afterwards the stars
// e.g. [2]foo becomes [2]*foo with n=1
func StarsInFront(str string, n int) string {
	b := strings.Index(str, "[")
	e := strings.Index(str, "]")
	stars := strings.Repeat("*", n)
	if b == 0 && e != -1 {
		return str[b:e+1] + stars + str[e+1:]
	}
	return stars + str
}

// SnakeToCamel converts hello_world to HelloWorld
func SnakeToCamel(s string) string {
	return delimToCamel(s, "_")
}

// DashToCamel converts hello-world to HelloWorld
func DashToCamel(s string) string {
	return delimToCamel(s, "-")
}

// RemoveSnakePrefix removes `prefix` from string `s` if that prefix ise separated with a _
// it removes lowercase or all u
func RemoveSnakePrefix(s string, prefix string) string {
	parts := strings.Split(s, "_")
	if len(parts) <= 1 {
		return s
	}
	if strings.EqualFold(parts[0], prefix) {
		parts = parts[1:]
	}
	return strings.Join(parts, "_")
}

// ReplaceExtension replaces an extension from filename with ext
// the extension is found by splitting on "." and taking the last part
func ReplaceExtension(filename string, ext string) string {
	splt := strings.Split(filename, ".")
	if len(splt) == 1 {
		return filename
	}
	splt[len(splt)-1] = ext
	return strings.Join(splt, ".")
}

func PrefixValue(val, prefix string) string {
	// if it's a slice, it has to come first
	b := strings.Index(val, "[")
	e := strings.Index(val, "]")
	if b == 0 && e != -1 {
		return val[b:e+1] + prefix + val[e+1:]
	}
	return prefix + val
}

func AddNamespace(val, ns string) string {
	if ns == "" || strings.Count(val, ".") >= 1 {
		return val
	}
	return PrefixValue(val, ns+".")
}

// NormalizeNamespace converts a type to one that always includes a lowercase namespace
// if no namespace is found, it adds `ns`, unless if strip is True then namespaces always equaling `ns` will be removed
func NormalizeNamespace(ns string, gotype string, strip bool) string {
	if ns == "" {
		return ""
	}
	gotype = strings.Trim(gotype, "*")
	splt := strings.Split(gotype, ".")
	if len(splt) <= 1 {
		splt = append([]string{ns}, splt...)
	}
	splt[0] = strings.ToLower(splt[0])
	if strip && splt[0] == strings.ToLower(ns) {
		splt = splt[1:]
	}
	return strings.Join(splt, ".")
}

// TranslateFilename translates a file path by renaming the file to a go suitable file
func TranslateFilename(filename string) string {
	if filename == "" {
		return "main.go"
	}

	b := filepath.Base(filename)
	return ReplaceExtension(b, "go")
}

func ConvertArgs(a []string) string {
	return strings.Join(a, ", ")
}

func ConvertArgsComma(a []string) string {
	if len(a) == 0 {
		return ""
	}
	return ", " + strings.Join(a, ", ")
}

func convertCallbackArgs(a []string, prependComma, skipEmpty, skipErr, handlePtr bool) string {
	var validArgs []string
	for _, arg := range a {
		if skipEmpty && arg == "" {
			continue
		}
		if skipErr && arg == "&cerr" {
			continue
		}

		if strings.Contains(arg, "{Ptr:") {
			if !handlePtr {
				// For ConvertCallbackArgs: remove * prefix and add &
				arg = strings.TrimPrefix(arg, "*")
			}
			validArgs = append(validArgs, "&"+arg)
		} else if strings.Contains(arg, "ConvertPtr(") && handlePtr {
			isSpecialVar := false
			for _, specialVar := range specialConvertPtrVars {
				if strings.Contains(arg, specialVar) {
					isSpecialVar = true

					break
				}
			}

			if isSpecialVar {
				validArgs = append(validArgs, arg)
			} else {
				validArgs = append(validArgs, "*"+arg)
			}
		} else {
			validArgs = append(validArgs, arg)
		}
	}

	if len(validArgs) == 0 {
		return ""
	}

	result := strings.Join(validArgs, ", ")
	if prependComma {
		return ", " + result
	}

	return result
}

func ConvertCallbackArgs(a []string) string {
	return convertCallbackArgs(a, false, false, false, false)
}

func ConvertArgsCommaDeref(a []string) string {
	return convertCallbackArgs(a, true, true, false, true)
}

func ConvertArgsDeref(a []string) string {
	return convertCallbackArgs(a, false, true, false, true)
}

func ConvertCallbackArgsNoErr(a []string) string {
	return convertCallbackArgs(a, false, true, true, true)
}

// ConstructorName returns a Go friendly constructor name given the raw constructor name `name` and the class/record name `outer`
func ConstructorName(name string, outer string) string {
	cname := SnakeToCamel(name)
	// construct the final constructor name
	// for example if we have gtk_builder
	// gtk_builder_new_from_file
	// cname will be NewFromFile
	// we convert it to NewBuilderFromFile
	if strings.HasPrefix(cname, "New") {
		return "New" + outer + cname[3:]
	}
	// the default is just a concatenation if the constructor doesn't start with New
	return outer + cname
}

// PropertyScalarSet generates the appropriate v.SetXXX(value) call based on the property's GoType and GLibType
func PropertyScalarSet(goType, glibType, valueName, objPrefix string) string {
	// First, try to find by Go type
	if mapping := gGLibTypeConfigByGoType(goType); mapping != nil {
		if mapping.CustomSetterFunc != nil {
			return mapping.CustomSetterFunc(valueName, objPrefix)
		}

		if mapping.SetterTemplate != "" {
			return strings.Replace(mapping.SetterTemplate, "%s", valueName, 1)
		}

		if mapping.SetterMethod != "" {
			return "v." + mapping.SetterMethod + "(" + valueName + ")"
		}
	}

	// Try to find by GLib type for special types
	if mapping := gLibTypeConfigByGLibType(glibType); mapping != nil {
		if mapping.CustomSetterFunc != nil {
			return mapping.CustomSetterFunc(valueName, objPrefix)
		}

		if mapping.SetterTemplate != "" {
			return strings.Replace(mapping.SetterTemplate, "%s", valueName, 1)
		}

		if mapping.SetterMethod != "" {
			return "v." + mapping.SetterMethod + "(" + valueName + ")"
		}
	}

	return "v.SetPointer(uintptr(" + valueName + "))"
}

// PropertyScalarGet generates the appropriate v.GetXXX() expression based on the property's GoType and GLibType
func PropertyScalarGet(goType, glibType, baseGoType string, isInterface, isRecord bool) string {
	// First, try to find by Go type
	if mapping := gGLibTypeConfigByGoType(goType); mapping != nil {
		if mapping.CustomGetterFunc != nil {
			return mapping.CustomGetterFunc(goType, baseGoType, isInterface, isRecord)
		}

		if mapping.GetterTemplate != "" {
			return mapping.GetterTemplate
		}

		if mapping.GetterMethod != "" {
			return "return v." + mapping.GetterMethod + "()"
		}
	}

	// Then, try to find by GLib type for special types
	if mapping := gLibTypeConfigByGLibType(glibType); mapping != nil {
		if mapping.CustomGetterFunc != nil {
			return mapping.CustomGetterFunc(goType, baseGoType, isInterface, isRecord)
		}

		if mapping.GetterTemplate != "" {
			return "return " + strings.Replace(mapping.GetterTemplate, "%s", goType, 1)
		}

		if mapping.GetterMethod != "" {
			return "return v." + mapping.GetterMethod + "()"
		}
	}

	return "return " + goType + "(v.GetPointer())"
}

// PropertyVectorSet generates the array conversion and v.SetXXX(value) call for array types
func PropertyVectorSet(goType, objPrefix, glibPrefix, propertyName string, useBaseObj bool) string {
	if config, ok := vectorTypeConfigs[goType]; ok {
		return config.SetterFunction(objPrefix, glibPrefix, propertyName, useBaseObj)
	}

	return ""
}

// PropertyVectorGet generates the array conversion and v.GetXXX() call for array types
func PropertyVectorGet(goType, objPrefix, corePrefix, propertyName string, useBaseObj bool) string {
	if config, ok := vectorTypeConfigs[goType]; ok {
		return config.GetterFunction(objPrefix, corePrefix, propertyName, useBaseObj)
	}

	return ""
}
