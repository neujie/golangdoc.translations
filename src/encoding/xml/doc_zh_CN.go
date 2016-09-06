// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ingore

// Package xml implements a simple XML 1.0 parser that
// understands XML name spaces.

// Package xml implements a simple XML 1.0 parser that
//
//     understands XML name spaces.
package xml

import (
	"bufio"
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	// A generic XML header suitable for use with the output of Marshal.
	// This is not automatically added to any output of this package,
	// it is provided as a convenience.
	Header = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
)

// HTMLAutoClose is the set of HTML elements that
// should be considered to close automatically.

// HTMLAutoClose是应当考虑到自动关闭的HTML元素的集合。
var HTMLAutoClose = htmlAutoClose

// HTMLEntity is an entity map containing translations for the
// standard HTML entity characters.

// HTMLEntity是标准HTML entity字符到其翻译的映射。
var HTMLEntity = htmlEntity

// An Attr represents an attribute in an XML element (Name=Value).

// Attr代表一个XML元素的一条属性（Name=Value）
type Attr struct {
	Name  Name
	Value string
}

// A CharData represents XML character data (raw text),
// in which XML escape sequences have been replaced by
// the characters they represent.

// CharData类型代表XML字符数据（原始文本），其中XML转义序列已经被它们所代表的字
// 符取代。
type CharData []byte

// A Comment represents an XML comment of the form <!--comment-->.
// The bytes do not include the <!-- and --> comment markers.

// Comment代表XML注释，格式为<!--comment-->，切片中不包含注释标记<!—和-->。
type Comment []byte

// A Decoder represents an XML parser reading a particular input stream.
// The parser assumes that its input is encoded in UTF-8.

// Decoder代表一个XML解析器，可以读取输入流的部分数据，该解析器假定输入是utf-8编
// 码的。
type Decoder struct {
	// Strict defaults to true, enforcing the requirements of the XML
	// specification. If set to false, the parser allows input containing common
	// mistakes:
	//
	// 	* If an element is missing an end tag, the parser invents
	// 	  end tags as necessary to keep the return values from Token
	// 	  properly balanced.
	// 	* In attribute values and character data, unknown or malformed
	// 	  character entities (sequences beginning with &) are left alone.
	//
	// Setting:
	//
	// 	d.Strict = false;
	// 	d.AutoClose = HTMLAutoClose;
	// 	d.Entity = HTMLEntity
	//
	// creates a parser that can handle typical HTML.
	//
	// Strict mode does not enforce the requirements of the XML name spaces TR.
	// In particular it does not reject name space tags using undefined
	// prefixes. Such tags are recorded with the unknown prefix as the name
	// space URL.
	Strict bool

	// When Strict == false, AutoClose indicates a set of elements to
	// consider closed immediately after they are opened, regardless
	// of whether an end element is present.
	AutoClose []string

	// Entity can be used to map non-standard entity names to string
	// replacements. The parser behaves as if these standard mappings are
	// present in the map, regardless of the actual map content:
	//
	// 	"lt": "<",
	// 	"gt": ">",
	// 	"amp": "&",
	// 	"apos": "'",
	// 	"quot": `"`,
	Entity map[string]string

	// CharsetReader, if non-nil, defines a function to generate
	// charset-conversion readers, converting from the provided
	// non-UTF-8 charset into UTF-8. If CharsetReader is nil or
	// returns an error, parsing stops with an error. One of the
	// the CharsetReader's result values must be non-nil.
	CharsetReader func(charset string, input io.Reader) (io.Reader, error)

	// DefaultSpace sets the default name space used for unadorned tags,
	// as if the entire XML stream were wrapped in an element containing
	// the attribute xmlns="DefaultSpace".
	DefaultSpace string
}

// A Directive represents an XML directive of the form <!text>.
// The bytes do not include the <! and > markers.

// Directive代表XML指示，格式为<!directive>，切片中不包含标记<!和>。
type Directive []byte

// An Encoder writes XML data to an output stream.

// Encoder向输出流中写入XML数据。
type Encoder struct {
}

// An EndElement represents an XML end element.

// EndElement代表一个XML结束元素。
type EndElement struct {
	Name Name
}

// Marshaler is the interface implemented by objects that can marshal
// themselves into valid XML elements.
//
// MarshalXML encodes the receiver as zero or more XML elements.
// By convention, arrays or slices are typically encoded as a sequence
// of elements, one per entry.
// Using start as the element tag is not required, but doing so
// will enable Unmarshal to match the XML elements to the correct
// struct field.
// One common implementation strategy is to construct a separate
// value with a layout corresponding to the desired XML and then
// to encode it using e.EncodeElement.
// Another common strategy is to use repeated calls to e.EncodeToken
// to generate the XML output one token at a time.
// The sequence of encoded tokens must make up zero or more valid
// XML elements.

// 实现了Marshaler接口的类型可以将自身序列化为合法的XML元素。
//
// MarshalXML方法将自身调用者编码为零或多个XML元素。 按照惯例，数组或切片会编码
// 为一系列元素，每个成员一条。使用start作为元素标签并不是必须的，但这么做可以帮
// 助Unmarshal方法正确的匹配XML元素和结构体字段。一个常用的策略是在同一个层次里
// 将每个独立的值对应到期望的XML然后使用e.EncodeElement进行编码。另一个常用的策
// 略是重复调用e.EncodeToken来一次一个token的生成XML输出。编码后的token必须组成
// 零或多个XML元素。
type Marshaler interface {
	MarshalXML(e *Encoder, start StartElement)error
}

// MarshalerAttr is the interface implemented by objects that can marshal
// themselves into valid XML attributes.
//
// MarshalXMLAttr returns an XML attribute with the encoded value of the
// receiver. Using name as the attribute name is not required, but doing so will
// enable Unmarshal to match the attribute to the correct struct field. If
// MarshalXMLAttr returns the zero attribute Attr{}, no attribute will be
// generated in the output. MarshalXMLAttr is used only for struct fields with
// the "attr" option in the field tag.

// 实现了MarshalerAttr接口的类型可以将自身序列化为合法的XML属性。
//
// MarshalXMLAttr返回一个值为方法调用者编码后的值的XML属性。使用name作为属性的
// name并非必须的，但这么做可以帮助Unmarshal方法正确的匹配属性和结构体字段。如果
// MarshalXMLAttr返回一个零值属性Attr{}，将不会生成属性输出。MarshalXMLAttr只用
// 于有标签且标签有"attr"选项的结构体字段。
type MarshalerAttr interface {
	MarshalXMLAttr(name Name) (Attr, error)
}

// A Name represents an XML name (Local) annotated
// with a name space identifier (Space).
// In tokens returned by Decoder.Token, the Space identifier
// is given as a canonical URL, not the short prefix used
// in the document being parsed.

// Name代表一个XML名称（Local字段），并指定名字空间（Space）。Decoder.Token方法
// 返回的Token中，Space标识符是典型的URL而不是被解析的文档里的短前缀。
type Name struct {
	Space, Local string
}

// A ProcInst represents an XML processing instruction of the form <?target
// inst?>

// ProcInst代表XML处理指令，格式为<?target inst?>。
type ProcInst struct {
	Target string
	Inst   []byte
}

// A StartElement represents an XML start element.

// StartElement代表一个XML起始元素。
type StartElement struct {
	Name Name
	Attr []Attr
}

// A SyntaxError represents a syntax error in the XML input stream.

// SyntaxError代表XML输入流的格式错误。
type SyntaxError struct {
	Msg  string
	Line int
}

// A TagPathError represents an error in the unmarshalling process
// caused by the use of field tags with conflicting paths.

// 反序列化时，如果字段标签的路径有冲突，就会返回TagPathError。
type TagPathError struct {
	Struct       reflect.Type
	Field1, Tag1 string
	Field2, Tag2 string
}

// A Token is an interface holding one of the token types:
// StartElement, EndElement, CharData, Comment, ProcInst, or Directive.

// Token接口用于保存token类型（CharData、Comment、Directive、ProcInst、
// StartElement、EndElement）的值。
type Token interface {
}

// An UnmarshalError represents an error in the unmarshalling process.

// UnmarshalError代表反序列化时出现的错误。
type UnmarshalError string

// Unmarshaler is the interface implemented by objects that can unmarshal
// an XML element description of themselves.
//
// UnmarshalXML decodes a single XML element
// beginning with the given start element.
// If it returns an error, the outer call to Unmarshal stops and
// returns that error.
// UnmarshalXML must consume exactly one XML element.
// One common implementation strategy is to unmarshal into
// a separate value with a layout matching the expected XML
// using d.DecodeElement,  and then to copy the data from
// that value into the receiver.
// Another common strategy is to use d.Token to process the
// XML object one token at a time.
// UnmarshalXML may not use d.RawToken.

// 实现了Unmarshaler接口的类型可以根据自身的XML元素描述反序列化自身。
//
// UnmarshalXML方法解码以start起始单个XML元素。如果它返回了错误，外层Unmarshal的
// 调用将停止执行并返回该错误。UnmarshalXML方法必须正好“消费”一个XML元素。一个
// 常用的策略是使用d.DecodeElement 将XML分别解码到各独立值，然后再将这些值写入
// UnmarshalXML的调用者。另一个常用的策略是使用d.Token一次一个token的处理XML对象
// 。UnmarshalXML通常不使用d.RawToken。
type Unmarshaler interface {
	UnmarshalXML(d *Decoder, start StartElement)error
}

// UnmarshalerAttr is the interface implemented by objects that can unmarshal
// an XML attribute description of themselves.
//
// UnmarshalXMLAttr decodes a single XML attribute.
// If it returns an error, the outer call to Unmarshal stops and
// returns that error.
// UnmarshalXMLAttr is used only for struct fields with the
// "attr" option in the field tag.

// 实现了UnmarshalerAttr接口的类型可以根据自身的XML属性形式的描述反序列化自身。
//
// UnmarshalXMLAttr解码单个的XML属性。如果它返回一个错误，外层的Umarshal调用会停
// 止执行并返回该错误。UnmarshalXMLAttr只有在结构体字段的标签有"attr"选项时才被
// 使用。
type UnmarshalerAttr interface {
	UnmarshalXMLAttr(attr Attr)error
}

// A MarshalXMLError is returned when Marshal encounters a type
// that cannot be converted into XML.

// 当序列化时，如果遇到不能转化为XML的类型，就会返回UnsupportedTypeError。
type UnsupportedTypeError struct {
	Type reflect.Type
}

// CopyToken returns a copy of a Token.

// CopyToken返回一个Token的拷贝。
func CopyToken(t Token) Token

// Escape is like EscapeText but omits the error return value.
// It is provided for backwards compatibility with Go 1.0.
// Code targeting Go 1.1 or later should use EscapeText.

// Escape类似EscapeText函数但会忽略返回的错误。本函数是用于保证和Go
// 1.0的向后兼容。应用于Go 1.1及以后版本的代码请使用EscapeText。
func Escape(w io.Writer, s []byte)

// EscapeText writes to w the properly escaped XML equivalent
// of the plain text data s.

// EscapeText向w中写入经过适当转义的、有明文s具有相同意义的XML文本。
func EscapeText(w io.Writer, s []byte) error

// Marshal returns the XML encoding of v.
//
// Marshal handles an array or slice by marshalling each of the elements.
// Marshal handles a pointer by marshalling the value it points at or, if the
// pointer is nil, by writing nothing. Marshal handles an interface value by
// marshalling the value it contains or, if the interface value is nil, by
// writing nothing. Marshal handles all other data by writing one or more XML
// elements containing the data.
//
// The name for the XML elements is taken from, in order of preference:
//
// 	- the tag on the XMLName field, if the data is a struct
// 	- the value of the XMLName field of type Name
// 	- the tag of the struct field used to obtain the data
// 	- the name of the struct field used to obtain the data
// 	- the name of the marshalled type
//
// The XML element for a struct contains marshalled elements for each of the
// exported fields of the struct, with these exceptions:
//
// 	- the XMLName field, described above, is omitted.
// 	- a field with tag "-" is omitted.
// 	- a field with tag "name,attr" becomes an attribute with
// 	  the given name in the XML element.
// 	- a field with tag ",attr" becomes an attribute with the
// 	  field name in the XML element.
// 	- a field with tag ",chardata" is written as character data,
// 	  not as an XML element.
// 	- a field with tag ",cdata" is written as character data
// 	  wrapped in one or more <![CDATA[ ... ]]> tags, not as an XML element.
// 	- a field with tag ",innerxml" is written verbatim, not subject
// 	  to the usual marshalling procedure.
// 	- a field with tag ",comment" is written as an XML comment, not
// 	  subject to the usual marshalling procedure. It must not contain
// 	  the "--" string within it.
// 	- a field with a tag including the "omitempty" option is omitted
// 	  if the field value is empty. The empty values are false, 0, any
// 	  nil pointer or interface value, and any array, slice, map, or
// 	  string of length zero.
// 	- an anonymous struct field is handled as if the fields of its
// 	  value were part of the outer struct.
//
// If a field uses a tag "a>b>c", then the element c will be nested inside
// parent elements a and b. Fields that appear next to each other that name the
// same parent will be enclosed in one XML element.
//
// See MarshalIndent for an example.
//
// Marshal will return an error if asked to marshal a channel, function, or map.

// Marshal returns the XML encoding of v.
//
// Marshal handles an array or slice by marshalling each of the elements.
// Marshal handles a pointer by marshalling the value it points at or, if the
// pointer is nil, by writing nothing. Marshal handles an interface value by
// marshalling the value it contains or, if the interface value is nil, by
// writing nothing. Marshal handles all other data by writing one or more XML
// elements containing the data.
//
// The name for the XML elements is taken from, in order of preference:
//
// 	- the tag on the XMLName field, if the data is a struct
// 	- the value of the XMLName field of type xml.Name
// 	- the tag of the struct field used to obtain the data
// 	- the name of the struct field used to obtain the data
// 	- the name of the marshalled type
//
// The XML element for a struct contains marshalled elements for each of the
// exported fields of the struct, with these exceptions:
//
// 	- the XMLName field, described above, is omitted.
// 	- a field with tag "-" is omitted.
// 	- a field with tag "name,attr" becomes an attribute with
// 	  the given name in the XML element.
// 	- a field with tag ",attr" becomes an attribute with the
// 	  field name in the XML element.
// 	- a field with tag ",chardata" is written as character data,
// 	  not as an XML element.
// 	- a field with tag ",cdata" is written as character data
// 	  wrapped in one or more <![CDATA[ ... ]]> tags, not as an XML element.
// 	- a field with tag ",innerxml" is written verbatim, not subject
// 	  to the usual marshalling procedure.
// 	- a field with tag ",comment" is written as an XML comment, not
// 	  subject to the usual marshalling procedure. It must not contain
// 	  the "--" string within it.
// 	- a field with a tag including the "omitempty" option is omitted
// 	  if the field value is empty. The empty values are false, 0, any
// 	  nil pointer or interface value, and any array, slice, map, or
// 	  string of length zero.
// 	- an anonymous struct field is handled as if the fields of its
// 	  value were part of the outer struct.
//
// If a field uses a tag "a>b>c", then the element c will be nested inside
// parent elements a and b. Fields that appear next to each other that name the
// same parent will be enclosed in one XML element.
//
// See MarshalIndent for an example.
//
// Marshal will return an error if asked to marshal a channel, function, or map.
func Marshal(v interface{}) ([]byte, error)

// MarshalIndent works like Marshal, but each XML element begins on a new
// indented line that starts with prefix and is followed by one or more
// copies of indent according to the nesting depth.

// MarshalIndent功能类似Marshal。但每个XML元素会另起一行并缩进，该行以prefix起始
// ，后跟一或多个indent的拷贝（根据嵌套层数）。
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error)

// NewDecoder creates a new XML parser reading from r.
// If r does not implement io.ByteReader, NewDecoder will
// do its own buffering.

// 创建一个从r读取XML数据的解析器。如果r未实现io.ByteReader接口，NewDecoder会为
// 其添加缓存。
func NewDecoder(r io.Reader) *Decoder

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder

// Unmarshal parses the XML-encoded data and stores the result in
// the value pointed to by v, which must be an arbitrary struct,
// slice, or string. Well-formed data that does not fit into v is
// discarded.
//
// Because Unmarshal uses the reflect package, it can only assign
// to exported (upper case) fields. Unmarshal uses a case-sensitive
// comparison to match XML element names to tag values and struct
// field names.
//
// Unmarshal maps an XML element to a struct using the following rules.
// In the rules, the tag of a field refers to the value associated with the
// key 'xml' in the struct field's tag (see the example above).
//
//   * If the struct has a field of type []byte or string with tag
//      ",innerxml", Unmarshal accumulates the raw XML nested inside the
//      element in that field. The rest of the rules still apply.
//
//   * If the struct has a field named XMLName of type Name,
//      Unmarshal records the element name in that field.
//
//   * If the XMLName field has an associated tag of the form
//      "name" or "namespace-URL name", the XML element must have
//      the given name (and, optionally, name space) or else Unmarshal
//      returns an error.
//
//   * If the XML element has an attribute whose name matches a
//      struct field name with an associated tag containing ",attr" or
//      the explicit name in a struct field tag of the form "name,attr",
//      Unmarshal records the attribute value in that field.
//
//   * If the XML element contains character data, that data is
//      accumulated in the first struct field that has tag ",chardata".
//      The struct field may have type []byte or string.
//      If there is no such field, the character data is discarded.
//
//   * If the XML element contains comments, they are accumulated in
//      the first struct field that has tag ",comment".  The struct
//      field may have type []byte or string. If there is no such
//      field, the comments are discarded.
//
//   * If the XML element contains a sub-element whose name matches
//      the prefix of a tag formatted as "a" or "a>b>c", unmarshal
//      will descend into the XML structure looking for elements with the
//      given names, and will map the innermost elements to that struct
//      field. A tag starting with ">" is equivalent to one starting
//      with the field name followed by ">".
//
//   * If the XML element contains a sub-element whose name matches
//      a struct field's XMLName tag and the struct field has no
//      explicit name tag as per the previous rule, unmarshal maps
//      the sub-element to that struct field.
//
//   * If the XML element contains a sub-element whose name matches a
//      field without any mode flags (",attr", ",chardata", etc), Unmarshal
//      maps the sub-element to that struct field.
//
//   * If the XML element contains a sub-element that hasn't matched any
//      of the above rules and the struct has a field with tag ",any",
//      unmarshal maps the sub-element to that struct field.
//
//   * An anonymous struct field is handled as if the fields of its
//      value were part of the outer struct.
//
//   * A struct field with tag "-" is never unmarshalled into.
//
// Unmarshal maps an XML element to a string or []byte by saving the
// concatenation of that element's character data in the string or
// []byte. The saved []byte is never nil.
//
// Unmarshal maps an attribute value to a string or []byte by saving
// the value in the string or slice.
//
// Unmarshal maps an XML element to a slice by extending the length of
// the slice and mapping the element to the newly created value.
//
// Unmarshal maps an XML element or attribute value to a bool by
// setting it to the boolean value represented by the string.
//
// Unmarshal maps an XML element or attribute value to an integer or
// floating-point field by setting the field to the result of
// interpreting the string value in decimal. There is no check for
// overflow.
//
// Unmarshal maps an XML element to a Name by recording the element
// name.
//
// Unmarshal maps an XML element to a pointer by setting the pointer
// to a freshly allocated value and then mapping the element to that value.

// Unmarshal parses the XML-encoded data and stores the result in
// the value pointed to by v, which must be an arbitrary struct,
// slice, or string. Well-formed data that does not fit into v is
// discarded.
//
// Because Unmarshal uses the reflect package, it can only assign
// to exported (upper case) fields. Unmarshal uses a case-sensitive
// comparison to match XML element names to tag values and struct
// field names.
//
// Unmarshal maps an XML element to a struct using the following rules.
// In the rules, the tag of a field refers to the value associated with the
// key 'xml' in the struct field's tag (see the example above).
//
//   * If the struct has a field of type []byte or string with tag
//      ",innerxml", Unmarshal accumulates the raw XML nested inside the
//      element in that field. The rest of the rules still apply.
//
//   * If the struct has a field named XMLName of type xml.Name,
//      Unmarshal records the element name in that field.
//
//   * If the XMLName field has an associated tag of the form
//      "name" or "namespace-URL name", the XML element must have
//      the given name (and, optionally, name space) or else Unmarshal
//      returns an error.
//
//   * If the XML element has an attribute whose name matches a
//      struct field name with an associated tag containing ",attr" or
//      the explicit name in a struct field tag of the form "name,attr",
//      Unmarshal records the attribute value in that field.
//
//   * If the XML element contains character data, that data is
//      accumulated in the first struct field that has tag ",chardata".
//      The struct field may have type []byte or string.
//      If there is no such field, the character data is discarded.
//
//   * If the XML element contains comments, they are accumulated in
//      the first struct field that has tag ",comment".  The struct
//      field may have type []byte or string. If there is no such
//      field, the comments are discarded.
//
//   * If the XML element contains a sub-element whose name matches
//      the prefix of a tag formatted as "a" or "a>b>c", unmarshal
//      will descend into the XML structure looking for elements with the
//      given names, and will map the innermost elements to that struct
//      field. A tag starting with ">" is equivalent to one starting
//      with the field name followed by ">".
//
//   * If the XML element contains a sub-element whose name matches
//      a struct field's XMLName tag and the struct field has no
//      explicit name tag as per the previous rule, unmarshal maps
//      the sub-element to that struct field.
//
//   * If the XML element contains a sub-element whose name matches a
//      field without any mode flags (",attr", ",chardata", etc), Unmarshal
//      maps the sub-element to that struct field.
//
//   * If the XML element contains a sub-element that hasn't matched any
//      of the above rules and the struct has a field with tag ",any",
//      unmarshal maps the sub-element to that struct field.
//
//   * An anonymous struct field is handled as if the fields of its
//      value were part of the outer struct.
//
//   * A struct field with tag "-" is never unmarshalled into.
//
// Unmarshal maps an XML element to a string or []byte by saving the
// concatenation of that element's character data in the string or
// []byte. The saved []byte is never nil.
//
// Unmarshal maps an attribute value to a string or []byte by saving
// the value in the string or slice.
//
// Unmarshal maps an XML element to a slice by extending the length of
// the slice and mapping the element to the newly created value.
//
// Unmarshal maps an XML element or attribute value to a bool by
// setting it to the boolean value represented by the string.
//
// Unmarshal maps an XML element or attribute value to an integer or
// floating-point field by setting the field to the result of
// interpreting the string value in decimal. There is no check for
// overflow.
//
// Unmarshal maps an XML element to an xml.Name by recording the
// element name.
//
// Unmarshal maps an XML element to a pointer by setting the pointer
// to a freshly allocated value and then mapping the element to that value.
func Unmarshal(data []byte, v interface{}) error

// Decode works like Unmarshal, except it reads the decoder
// stream to find the start element.

// Decode works like xml.Unmarshal, except it reads the decoder
// stream to find the start element.
func (d *Decoder) Decode(v interface{}) error

// DecodeElement works like Unmarshal except that it takes
// a pointer to the start XML element to decode into v.
// It is useful when a client reads some raw XML tokens itself
// but also wants to defer to Unmarshal for some elements.

// DecodeElement works like xml.Unmarshal except that it takes
// a pointer to the start XML element to decode into v.
// It is useful when a client reads some raw XML tokens itself
// but also wants to defer to Unmarshal for some elements.
func (d *Decoder) DecodeElement(v interface{}, start *StartElement) error

// InputOffset returns the input stream byte offset of the current decoder
// position. The offset gives the location of the end of the most recently
// returned token and the beginning of the next token.
func (d *Decoder) InputOffset() int64

// RawToken is like Token but does not verify that
// start and end elements match and does not translate
// name space prefixes to their corresponding URLs.

// RawToken方法Token方法，但不会验证起始和结束标签，也不将名字空间前缀翻译为它们
// 相应的URL。
func (d *Decoder) RawToken() (Token, error)

// Skip reads tokens until it has consumed the end element
// matching the most recent start element already consumed.
// It recurs if it encounters a start element, so it can be used to
// skip nested structures.
// It returns nil if it finds an end element matching the start
// element; otherwise it returns an error describing the problem.

// Skip从底层读取token，直到读取到最近一次读取到的起始标签对应的结束标签。如果读
// 取中遇到别的起始标签会进行迭代，因此可以跳过嵌套结构。如果本方法找到了对应起
// 始标签的结束标签，会返回nil；否则返回一个描述该问题的错误。
func (d *Decoder) Skip() error

// Token returns the next XML token in the input stream.
// At the end of the input stream, Token returns nil, io.EOF.
//
// Slices of bytes in the returned token data refer to the
// parser's internal buffer and remain valid only until the next
// call to Token. To acquire a copy of the bytes, call CopyToken
// or the token's Copy method.
//
// Token expands self-closing elements such as <br/>
// into separate start and end elements returned by successive calls.
//
// Token guarantees that the StartElement and EndElement
// tokens it returns are properly nested and matched:
// if Token encounters an unexpected end element
// or EOF before all expected end elements,
// it will return an error.
//
// Token implements XML name spaces as described by
// http://www.w3.org/TR/REC-xml-names/.  Each of the
// Name structures contained in the Token has the Space
// set to the URL identifying its name space when known.
// If Token encounters an unrecognized name space prefix,
// it uses the prefix as the Space rather than report an error.

// Token返回输入流里的下一个XML token。在输入流的结尾处，会返回(nil, io.EOF)
//
// 返回的token数据里的[]byte数据引用自解析器内部的缓存，只在下一次调用Token之前
// 有效。如要获取切片的拷贝，调用CopyToken函数或者token的Copy方法。
//
// 成功调用的Token方法会将自我闭合的元素（如<br/>）扩展为分离的起始和结束标签。
//
// Token方法会保证它返回的StartElement和EndElement两种token正确的嵌套和匹配：如
// 果本方法遇到了不正确的结束标签，会返回一个错误。
//
// Token方法实现了XML名字空间，细节参见http://www.w3.org/TR/REC-xml-names/。每一
// 个包含在Token里的Name结构体，都会将Space字段设为URL标识（如果可知的话）。如果
// Token遇到未知的名字空间前缀，它会使用该前缀作为名字空间，而不是报错。
func (d *Decoder) Token() (Token, error)

// Encode writes the XML encoding of v to the stream.
//
// See the documentation for Marshal for details about the conversion
// of Go values to XML.
//
// Encode calls Flush before returning.

// Encode writes the XML encoding of v to the stream.
//
// See the documentation for Marshal for details about the conversion of Go
// values to XML.
//
// Encode calls Flush before returning.
func (enc *Encoder) Encode(v interface{}) error

// EncodeElement writes the XML encoding of v to the stream,
// using start as the outermost tag in the encoding.
//
// See the documentation for Marshal for details about the conversion
// of Go values to XML.
//
// EncodeElement calls Flush before returning.

// EncodeElement writes the XML encoding of v to the stream, using start as the
// outermost tag in the encoding.
//
// See the documentation for Marshal for details about the conversion of Go
// values to XML.
//
// EncodeElement calls Flush before returning.
func (enc *Encoder) EncodeElement(v interface{}, start StartElement) error

// EncodeToken writes the given XML token to the stream. It returns an error if
// StartElement and EndElement tokens are not properly matched.
//
// EncodeToken does not call Flush, because usually it is part of a larger
// operation such as Encode or EncodeElement (or a custom Marshaler's MarshalXML
// invoked during those), and those will call Flush when finished. Callers that
// create an Encoder and then invoke EncodeToken directly, without using Encode
// or EncodeElement, need to call Flush when finished to ensure that the XML is
// written to the underlying writer.
//
// EncodeToken allows writing a ProcInst with Target set to "xml" only as the
// first token in the stream.
func (enc *Encoder) EncodeToken(t Token) error

// Flush flushes any buffered XML to the underlying writer.
// See the EncodeToken documentation for details about when it is necessary.

// Flush flushes any buffered XML to the underlying writer. See the EncodeToken
// documentation for details about when it is necessary.
func (enc *Encoder) Flush() error

// Indent sets the encoder to generate XML in which each element
// begins on a new indented line that starts with prefix and is followed by
// one or more copies of indent according to the nesting depth.

// Indent sets the encoder to generate XML in which each element begins on a new
// indented line that starts with prefix and is followed by one or more copies
// of indent according to the nesting depth.
func (enc *Encoder) Indent(prefix, indent string)

func (e *SyntaxError) Error() string

func (e *TagPathError) Error() string

func (e *UnsupportedTypeError) Error() string

func (c CharData) Copy() CharData

func (c Comment) Copy() Comment

func (d Directive) Copy() Directive

func (p ProcInst) Copy() ProcInst

func (e StartElement) Copy() StartElement

// End returns the corresponding XML end element.

// 返回e对应的XML结束元素。
func (e StartElement) End() EndElement

func (e UnmarshalError) Error() string

