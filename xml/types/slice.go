package types

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/cruffinoni/xml-generator/xml/interface"
	"github.com/cruffinoni/xml-generator/xml/saver/xmlFile"
	"github.com/cruffinoni/xml-generator/xml/types/primary"

	"github.com/cruffinoni/xml-generator/xml"
	"github.com/cruffinoni/xml-generator/xml/attributes"
	"github.com/cruffinoni/xml-generator/xml/saver"
	"github.com/cruffinoni/xml-generator/xml/unmarshal"
)

type sliceData[T any] struct {
	data T
	// str is a string representation of data
	str  string
	attr attributes.Attributes
	fmt.Stringer
	tag    string
	hidden bool
	kind   reflect.Kind
}

func (s *sliceData[T]) Assign(e *xml.Element) error {
	var err error
	//log.Printf("Assign on slicedata called: %v (%v) > %T", e.XMLPath(), e.Attr, s.data)
	s.kind = reflect.TypeOf(s.data).Kind()
	if s.kind == reflect.Ptr {
		err = unmarshal.Element(e, s.data)
		s.hidden = reflect.ValueOf(s.data).IsZero()
		//log.Printf("Kind is ptr. Is it hidden ? %v", s.hidden)
	} else if utils.IsReflectPrimaryType(s.kind) {
		s.hidden = e.Data == nil
		switch s.kind {
		case reflect.String:
			s.data = castTemplate[T](e.Data.GetString())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			s.data = castTemplate[T](e.Data.GetInt64())
		case reflect.Bool:
			s.data = castTemplate[T](e.Data.GetBool())
		case reflect.Float32, reflect.Float64:
			s.data = castTemplate[T](e.Data.GetFloat64())
		default:
			return fmt.Errorf("sliceData.Assign: can't assign primary type %T to %T", e.Data.GetData(), s.data)
		}
	} else {
		err = unmarshal.Element(e, &s.data)
	}
	if err != nil {
		return err
	}
	s.UpdateStringRepresentation()
	return err
}

func (s *sliceData[T]) GetPath() string {
	return ""
}

func (s *sliceData[T]) SetAttributes(attributes attributes.Attributes) {
	s.attr = attributes
}

func (s *sliceData[T]) GetAttributes() attributes.Attributes {
	return s.attr
}

func (s *sliceData[T]) UpdateStringRepresentation() {
	t := reflect.TypeOf(s.data)
	// We check if the type T implements the interface fmt.Stringer and has a
	// custom String() method.
	if ok := t.Implements(reflect.TypeOf(new(fmt.Stringer)).Elem()); ok {
		var m reflect.Method
		// If it's the case, we get the method String() of the type T and
		// call it.
		if m, ok = t.MethodByName("String"); ok {
			s.str = m.Func.Call([]reflect.Value{reflect.ValueOf(s.data)})[0].String()
		}
	} else {
		// Otherwise we use a basic string representation.
		s.str = fmt.Sprintf("'%+v'", s.data)
	}
}

func (s *sliceData[T]) String() string {
	return s.str
}

func (s *sliceData[T]) GetXMLTag() []byte {
	return nil
}

func (s *sliceData[T]) TransformToXML(b *saver.Buffer) error {
	//log.Printf("sliceData.TransformToXML => %v (? %T) [Hidden: %v | %v]", s.tag, s.data, s.hidden, s.attr)
	//log.Printf("Is hidden: %v (%T)", s.hidden, s.data)

	_, okEmpty := any(s.data).(*primary.Empty)
	elem, okElem := any(s.data).(*xml.Element)
	if implFieldValidating, ok := castToInterface[_interface.FieldValidator](s.data); ok && implFieldValidating.CountValidatedField() == 0 || okEmpty || okElem && elem.IsEmpty() {
		b.WriteEmptyTag(s.tag, s.attr)
		return nil
	}
	b.OpenTag(s.tag, s.attr)
	if err := xmlFile.Save(s.data, b, ""); err != nil {
		return err
	}
	if utils.IsReflectPrimaryType(s.kind) {
		if s.kind == reflect.String && strings.Contains(reflect.ValueOf(s.data).String(), "\n") {
			b.CloseTagWithIndent(s.tag)
		}
		b.CloseTag(s.tag)
		return nil
	}
	b.CloseTagWithIndent(s.tag)
	return nil
}

// Slice is a slice of data that is represented by sliceData.
// The main difference with a normal slice is that it can get and set attributes.
// Like a normal slice, the slice is a slice of T and you can iterate over it.
type Slice[T any] struct {
	data         []sliceData[T]
	attr         attributes.Attributes
	repeatingTag string
	name         string
	cap          int
}

func (s *Slice[T]) TransformToXML(b *saver.Buffer) error {
	//if s.repeatingTag == "" {
	//	log.Print("Slice.TransformToXML: No repeating tag specified.")
	//	return nil
	//}
	// log.Printf("Name: '%v' w/ cap %d w/ %s / %v", s.name, s.cap, s.repeatingTag, s.data)
	if s.cap == 0 {
		return xmlFile.ErrEmptyValue
	}
	for _, v := range s.data {
		if err := v.TransformToXML(b); err != nil {
			return err
		}
		_, _ = b.Write([]byte("\n"))
	}
	//b.WriteString("\n")
	//b.CloseTagWithIndent(s.repeatingTag)
	//b.WriteString("\n")
	return nil
}

func (s *Slice[T]) GetXMLTag() []byte {
	return []byte(s.repeatingTag)
}

func (s *Slice[T]) Capacity() int {
	return s.cap
}

func (s *Slice[T]) Set(value T, attr attributes.Attributes, idx int) {
	old := s.data[idx]
	if attr == nil {
		attr = old.attr
	}
	if idx >= s.cap || idx < 0 {
		panic("Slice index out of bounds")
	}
	d := sliceData[T]{
		data: value,
		attr: attr,
		tag:  old.tag,
		kind: old.kind,
	}
	d.UpdateStringRepresentation()
	if utils.IsReflectPrimaryType(d.kind) {
		d.hidden = true
	}
}

func (s *Slice[T]) Add(value T, attr attributes.Attributes) {
	d := sliceData[T]{
		data: value,
		attr: attr,
		tag:  s.repeatingTag,
		kind: reflect.TypeOf(value).Kind(),
	}
	d.UpdateStringRepresentation()
	if utils.IsReflectPrimaryType(d.kind) {
		d.hidden = true
	}
	s.data = append(s.data, d)
}

func (s *Slice[T]) At(idx int) T {
	if idx < 0 || idx >= len(s.data) {
		panic("out of bound/At: overflow/underflow")
	}
	return s.data[idx].data
}

func (s *Slice[T]) Remove(idx int) {
	if idx < 0 || idx >= len(s.data) {
		panic("out of bound/Remove: overflow/underflow")
	}
	if idx == s.cap-1 {
		s.data = s.data[:idx]
	} else {
		s.data = append(s.data[:idx], s.data[idx+1:]...)
	}
	s.cap--
}

func (s *Slice[T]) Reset() {
	s.cap = 0
	s.data = make([]sliceData[T], 0)
}

func (s *Slice[T]) Assign(e *xml.Element) error {
	if s.cap == 0 {
		s.data = make([]sliceData[T], 0)
	}
	n := e
	if n == nil {
		log.Printf("n is nil")
		return nil
	}
	if n.Parent != nil {
		s.name = n.Parent.GetName()
		//log.Printf("Slice.Assign: Parent is %s", n.Parent.GetName())
	} else {
		log.Printf("Slice.Assign: Assigning to slice without parent")
		s.name = "unknown"
	}
	s.repeatingTag = n.GetName()
	//if !strings.Contains(reflect.TypeOf(zero[T]()).Name(), "types.Slice") {
	//	for n.Child != nil && n.Child.Child != nil {
	//		n = n.Child
	//	}
	//}
	//log.Printf("Assigning: %v / %v", e.XMLPath(), e.Attr)
	for n != nil {
		sd := sliceData[T]{
			tag: n.GetName(),
		}
		// Set sd.data to zero depending on the type of T. Either a pointer or a
		// value.
		//log.Printf("Type: %v / %v", reflect.TypeOf(sd.data).Kind(), n.Child)
		switch tType := reflect.TypeOf(sd.data).Kind(); tType {
		case reflect.Ptr, reflect.Interface, reflect.Struct, reflect.Map, reflect.Slice:
			sd.data = reflect.New(reflect.TypeOf(*new(T)).Elem()).Interface().(T)
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			sd.data = zero[T]()
		}

		//log.Printf("Child ? %v", n.Child != nil)
		if n.Child != nil {
			if err := unmarshal.Element(n.Child, &sd); err != nil {
				return err
			}
		} else {
			if err := unmarshal.Element(n, &sd); err != nil {
				return err
			}
		}
		//log.Printf("Slice.Assign: %+v", sd.data)
		sd.SetAttributes(n.Attr)
		s.data = append(s.data, sd)
		n = n.Next
	}
	s.cap = len(s.data)
	//log.Printf("Slice.Assign: end => %s", s)

	//for _, d := range s.data {
	//	log.Printf("Slice.Assign: %v", d)
	//}
	return nil
}

func (s *Slice[T]) String() string {
	b := strings.Builder{}
	b.WriteString("[")
	for i, d := range s.data {
		if i > 0 {
			b.WriteString(", " + d.String())
		} else {
			b.WriteString(d.String())
		}
	}
	b.WriteString("]")
	return b.String()
}

func (s *Slice[T]) GetPath() string {
	// We avoid to use GetPath in generics types like Slice or Map
	return ""
}

func (s *Slice[T]) SetAttributes(attributes attributes.Attributes) {
	s.attr = attributes
}

func (s *Slice[T]) GetAttributes() attributes.Attributes {
	return s.attr
}

func (s *Slice[T]) ValidateField(_ string) {
}

func (s *Slice[T]) IsValidField(_ string) bool {
	return true
}

func (s *Slice[T]) CountValidatedField() int {
	return s.cap
}
