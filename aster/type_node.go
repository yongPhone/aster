// Copyright 2018 henrylee2cn. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aster

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

type superType struct {
	*super
	isAssign bool // is there `=` for declared type?
	methods  []FuncNode
}

func (f *File) newSuperType(namePtr *string, kind Kind, doc *ast.CommentGroup,
	isAssign bool) *superType {
	return &superType{
		super:    f.newSuper(namePtr, kind, doc),
		isAssign: isAssign,
	}
}

func (s *superType) typeNode() {}

// IsAssign is there `=` for declared type?
func (s *superType) IsAssign() bool {
	return s.isAssign
}

// Method returns the i'th method in the type's method set.
// It panics if i is not in the range [0, NumMethod()).
//
// For a non-interface type T or *T, the returned Method's Type and Func
// fields describe a function whose first argument is the receiver.
//
// For an interface type, the returned Method's Type field gives the
// method signature, without a receiver, and the Func field is nil.
func (s *superType) Method(i int) (FuncNode, bool) {
	if i < 0 || i >= len(s.methods) {
		return nil, false
	}
	return s.methods[i], true
}

// MethodByName returns the method with that name in the type's
// method set and a boolean indicating if the method was found.
//
// For a non-interface type T or *T, the returned Method's Type and Func
// fields describe a function whose first argument is the receiver.
//
// For an interface type, the returned Method's Type field gives the
// method signature, without a receiver, and the Func field is nil.
func (s *superType) MethodByName(name string) (FuncNode, bool) {
	for _, m := range s.methods {
		if m.Name() == name {
			return m, true
		}
	}
	return nil, false
}

// NumMethod returns the number of exported methods in the type's method set.
func (s *superType) NumMethod() int {
	return len(s.methods)
}

// Implements reports whether the type implements the interface type u.
func (s *superType) Implements(u TypeNode) bool {
	for i := u.NumMethod() - 1; i >= 0; i-- {
		um, _ := u.Method(i)
		cm, ok := s.MethodByName(um.Name())
		if !ok ||
			um.IsVariadic() != cm.IsVariadic() ||
			um.NumParam() != cm.NumParam() ||
			um.NumResult() != cm.NumResult() {
			return false
		}
		for j := um.NumParam(); j >= 0; j-- {
			uf, _ := um.Param(j)
			cf, _ := cm.Param(j)
			if uf.TypeName != cf.TypeName {
				return false
			}
		}
		for j := um.NumResult(); j >= 0; j-- {
			uf, _ := um.Result(j)
			cf, _ := cm.Result(j)
			if uf.TypeName != cf.TypeName {
				return false
			}
		}
	}
	return true
}

func (s *superType) addMethod(method FuncNode) error {
	field, ok := method.Recv()
	if !ok {
		return fmt.Errorf("not method: %s", method.Name())
	}
	if field.TypeName != s.Name() {
		return fmt.Errorf("reveiver do not match method: %s, want: %s, got: %s",
			method.Name(), s.Name(), field.TypeName)
	}
	s.methods = append(s.methods, method)
	return nil
}

// BasicType basic type
type BasicType struct {
	*superType
	ast.Expr
}

func (f *File) newBasicType(namePtr *string, doc *ast.CommentGroup, assign token.Pos,
	node ast.Expr) (*BasicType, bool) {
	basicName := strings.TrimLeft(f.tryFormat(node), "*")
	kind, found := getBasicKind(basicName)
	if !found {
		return nil, false
	}
	return &BasicType{
		superType: f.newSuperType(namePtr, kind, doc, assign != token.NoPos),
		Expr:      node,
	}, true
}

// ListType represents an array or slice type.
type ListType struct {
	*superType
	*ast.ArrayType
}

func (f *File) newListType(namePtr *string, doc *ast.CommentGroup, assign token.Pos,
	node *ast.ArrayType) *ListType {
	kind := Slice
	if node.Len != nil {
		kind = Array
	}
	return &ListType{
		superType: f.newSuperType(namePtr, kind, doc, assign != token.NoPos),
		ArrayType: node,
	}
}

// Len returns list's length if it is array type,
// otherwise returns false.
func (l *ListType) Len() (int, bool) {
	if l.Kind() == Slice {
		return -1, false
	}
	cnt, _ := strconv.Atoi(l.ArrayType.Len.(*ast.BasicLit).Value)
	return cnt, true
}

// MapType represents a map type.
type MapType struct {
	*superType
	*ast.MapType
}

func (f *File) newMapType(namePtr *string, doc *ast.CommentGroup, assign token.Pos,
	node *ast.MapType) *MapType {
	return &MapType{
		superType: f.newSuperType(namePtr, Map, doc, assign != token.NoPos),
		MapType:   node,
	}
}

// ChanType represents a channel type.
type ChanType struct {
	*superType
	*ast.ChanType
}

func (f *File) newChanType(namePtr *string, doc *ast.CommentGroup, assign token.Pos,
	node *ast.ChanType) *ChanType {
	return &ChanType{
		superType: f.newSuperType(namePtr, Chan, doc, assign != token.NoPos),
		ChanType:  node,
	}
}

// Dir returns a channel type's direction.
func (c *ChanType) Dir() ast.ChanDir {
	return c.ChanType.Dir
}

// InterfaceType represents a interface type.
type InterfaceType struct {
	*superType
	*ast.InterfaceType
}

func (f *File) newInterfaceType(namePtr *string, doc *ast.CommentGroup, assign token.Pos,
	node *ast.InterfaceType) *InterfaceType {
	return &InterfaceType{
		superType:     f.newSuperType(namePtr, Interface, doc, assign != token.NoPos),
		InterfaceType: node,
	}
}

// StructType represents a struct type.
type StructType struct {
	*superType
	*ast.StructType
	fields []*StructField // sorted by offset
}

func (f *File) newStructType(namePtr *string, doc *ast.CommentGroup, assign token.Pos,
	node *ast.StructType) *StructType {
	return &StructType{
		superType:  f.newSuperType(namePtr, Struct, doc, assign != token.NoPos),
		StructType: node,
	}
}

func (s *StructType) addFields(field ...*StructField) {
	s.fields = append(s.fields, field...)
}

// A StructField describes a single field in a struct.
type StructField struct {
	Name      string    // the field name
	Type      TypeNode  // field type
	Tag       StructTag // field tag string
	Index     []int     // index sequence for Type.FieldByIndex
	Anonymous bool      // is an embedded field
	Doc       string    // lead comment
	Comment   string    // line comment
}

// A StructTag is the tag string in a struct field.
//
// By convention, tag strings are a concatenation of
// optionally space-separated key:"value" pairs.
// Each key is a non-empty string consisting of non-control
// characters other than space (U+0020 ' '), quote (U+0022 '"'),
// and colon (U+003A ':').  Each value is quoted using U+0022 '"'
// characters and Go string literal syntax.
type StructTag = reflect.StructTag

// NumField returns a struct type's field count.
// It panics if the type's Kind is not Struct.
func (s *StructType) NumField() int {
	return len(s.fields)
}

// Field returns a struct type's i'th field.
func (s *StructType) Field(i int) (field *StructField, found bool) {
	if i < 0 || i >= len(s.fields) {
		return
	}
	return s.fields[i], true
}

// FieldByName returns the struct field with the given name
// and a boolean indicating if the field was found.
func (s *StructType) FieldByName(name string) (field *StructField, found bool) {
	for _, field := range s.fields {
		if field.Name == name {
			return field, true
		}
	}
	return nil, false
}