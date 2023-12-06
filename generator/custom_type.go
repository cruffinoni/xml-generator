package generator

import (
	"log"
	"reflect"

	"github.com/cruffinoni/rimworld-editor/generator/paths"
	"github.com/cruffinoni/rimworld-editor/helper"
	"github.com/cruffinoni/rimworld-editor/xml"
)

type CustomType struct {
	Name string
	Pkg  string
	// Type1 is the first type in the custom type
	// It can be anything from reflect.Kind, *CustomType, *xml.Element, etc.
	Type1      any
	Type2      any
	ImportPath string
}

func IsEmptyType(c *CustomType) bool {
	if c == nil {
		return false
	}
	return c.Name == "Empty" && c.Pkg == "*primary"
}

func createEmptyType() any {
	return &CustomType{
		Name:       "Empty",
		Pkg:        "*primary",
		Type1:      nil,
		ImportPath: paths.PrimaryTypesPath,
	}
}

func IsSliceType(c *CustomType) bool {
	if c == nil {
		return false
	}
	return c.Name == "Slice" && c.Pkg == "*types"
}

func IsEmbeddedType(c *CustomType) bool {
	if c == nil {
		return false
	}
	return c.Name == "Type" && c.Pkg == "*embedded"
}

func createCustomSlice(e *xml.Element, flag uint) any {
	return &CustomType{
		Name:       "Slice",
		Pkg:        "*types",
		Type1:      createSubtype(e, flag, getTypeFromArrayOrSlice(e)),
		ImportPath: paths.CustomTypesPath,
	}
}

func IsMultipleType(c *CustomType) bool {
	return c.Name == "Type" && c.Pkg == "*multiple"
}

func createMultipleType() any {
	return &CustomType{
		Name:       "Type",
		Pkg:        "*multiple",
		Type1:      nil,
		ImportPath: paths.MultipleTypesPath,
	}
}

func createXMLElementType() any {
	return &xml.Element{}
}

// determineTypeFromData returns the type of data from the element.
// If the element is not a primitive type, it returns either a
// StructInfo or a CustomType. Otherwise, it returns the type of the
// element as a reflect.Kind
// If the type is invalid, we consider it as a *xml.Element.
// Is it useful for empty tags.
func determineTypeFromData(e *xml.Element, flag uint) any {
	t := any(getTypeFromArrayOrSlice(e))
	//log.Printf("Type of %v is %v", e.XMLPath(), t)
	// We need to define this struct with of this all members
	if t == reflect.Struct || t == reflect.Slice {
		c := e.Child
		// If the child is a list, let's create a slice from it
		if helper.IsListTag(c.Child.GetName()) {
			// We set the forceChild flag to true to force the function createStructure
			// to take the children of the list and not the list itself.
			t = createArrayOrSlice(c, flag|forceChild)
		} else {
			// Otherwise, a basic struct is created
			// We pass 'e' instead of 'c' because createStructure will take the children of 'e'
			t = createStructure(e, flag)
		}
	} else if t == reflect.Invalid {
		if e.Data == nil {
			t = createEmptyType()
		} else {
			t = e
		}
	} else {
		if !e.Attr.Empty() {
			//log.Println("primary.EmbeddedType: found attributes on path", e.XMLPath())
			return &CustomType{
				Name:       "Type",
				Pkg:        "embedded",
				Type1:      t,
				ImportPath: paths.EmbeddedTypePath,
			}
		}
	}
	return t
}

func createCustomTypeForMap(e *xml.Element, flag uint) any {
	if e.Child == nil {
		log.Panic("generate.createCustomTypeForMap: missing child")
	}

	//log.Printf("Determining key type from %s", e.Child.XMLPath())
	var (
		c = e.Child
		k = determineTypeFromData(c, flag|forceFullCheck)
		v any
	)
	if ct, ok := k.(*CustomType); ok {
		// primary.Empty does not implement comparable
		// Might be deleted when the types.Map type implements multiple as Key and not comparable anymore
		if ct.Name == "Empty" {
			k = reflect.String
		}
	}
	//log.Printf("Key type: %T", k)
	c = c.Next
	//log.Printf("Determining value type from '%v'", c.XMLPath())
	v = determineTypeFromData(c, flag|forceFullCheck)
	//log.Printf("Val type: %T", v)
	// By default, maps are strings to strings
	if k == reflect.Invalid || v == reflect.Invalid {
		return &CustomType{
			Name:       "Map",
			Pkg:        "types",
			Type1:      reflect.String,
			Type2:      reflect.String,
			ImportPath: paths.CustomTypesPath,
		}
	}
	return &CustomType{
		Name:       "Map",
		Pkg:        "types",
		Type1:      k,
		Type2:      v,
		ImportPath: paths.CustomTypesPath,
	}
}

func findFirstValidElementWithChild(e *xml.Element) *xml.Element {
	n := e
	for n != nil && n.Child == nil {
		n = n.Next
	}
	return n
}

func determineArrayOrSliceKind(e *xml.Element) reflect.Kind {
	k := e
	for k != nil {
		if k.Data == nil && k.Child == nil {
			return reflect.Array
		}
		k = k.Next
	}
	return reflect.Slice
}

// getTypeFromArrayOrSlice returns the type of the element as a reflect.Kind.
// If the element is not a valid type, it returns reflect.Invalid.
// flag can be modified by the function to indicates to bypass the first element of the array/slice
func getTypeFromArrayOrSlice(e *xml.Element) reflect.Kind {
	// Return Invalid if the element has no child
	if e.Child == nil {
		if e.Data == nil {
			return reflect.Invalid
		} else {
			return e.Data.Kind()
		}
	}

	k := e.Child
	kt := reflect.Invalid

	// Determine if the element is a structure or slice
	// In some cases, the first structure may have no data, so we check its siblings for a child
	// Note to run the check until we find a valid sibling with a child.
	// Think about the case where only the last value has a child of a huge array
	if k.Next != nil && k.Next.GetName() == k.GetName() {
		// This part of code is aimed to list with multiple elements
		n := k.Next
		for n != nil && n.GetName() == k.GetName() {
			if n.Child != nil {
				if helper.IsListTag(n.Child.GetName()) {
					return determineArrayOrSliceKind(n)
				}
				return reflect.Struct
			}
			n = n.Next
		}
	}
	if k.Child != nil {
		// On the other hand, this part only check the first element
		// but sometimes the first element has no data
		if helper.IsListTag(k.Child.GetName()) {
			return determineArrayOrSliceKind(k)
		}
		return reflect.Struct
	}

	for k != nil {
		if k.Data != nil {
			kdk := k.Data.Kind()
			if kt != reflect.Invalid && kdk != kt &&
				// Float64 and Int64 can be interchangeable
				!(kdk == reflect.Float64 && kt == reflect.Int64) &&
				!(kdk == reflect.Int64 && kt == reflect.Float64) {
				log.Printf("last: '%v' & kind %s", k.Data, k.Data.Kind())
				log.Panicf("getTypeFromArrayOrSlice: found type %v, expected %v on path %v ('%v')", kdk, kt, k.XMLPath(), k.Data.GetData())
			}
			// Float64 and Int64 can be interchangeable, but we prefer to keep Float64
			if !(kt == reflect.Float64 && kdk == reflect.Int64) {
				kt = kdk
			}
		}
		k = k.Next
	}
	// The first element may have no data, so we check its sibling (the second one) for a child
	if kt == reflect.Invalid && e.Next != nil && helper.IsListTag(e.Next.GetName()) && e.Next.GetName() == e.GetName() {
		siblingWithChild := e.Next
		for siblingWithChild != nil && siblingWithChild.Child == nil {
			siblingWithChild = siblingWithChild.Next
		}
		if siblingWithChild == nil || siblingWithChild.Child == nil {
			return reflect.Invalid
		}
		siblingType := createTypeFromElement(siblingWithChild, flagNone)
		if !IsSameType(siblingType, reflect.Invalid, 0) {
			return Complex
		}
		return kt
	}
	return kt
}
