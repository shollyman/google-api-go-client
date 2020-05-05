// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package disco

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"log"
)

// DiffOptions is a bitmask for defining diff behavior.
type DiffOptions uint16

const (
	VersioningOption DiffOptions = 1 << iota
	DescriptionOption
	ServiceOption
	SchemaOption
	ResourceOption
)

// AllOptions enables all bits, even unused.
const AllOptions = DiffOptions(^uint16(0))

// ElementKind indicates what kind of element a diff references: a simple field, a Schema object, a Method, etc.
type ElementKind string

const (
	SchemaKind      ElementKind = "SCHEMA"
	ResourceKind    ElementKind = "RESOURCE"
	MethodKind      ElementKind = "METHOD"
	StringFieldKind ElementKind = "STRING_FIELD"
	BoolFieldKind   ElementKind = "BOOL_FIELD"
	StringListKind  ElementKind = "LIST_OF_STRINGS"
	MediaUploadKind ElementKind = "MEDIA_UPLOAD"
	ParameterKind   ElementKind = "METHOD_PARAMETER"
)

// Set applies an option to a mask.
func Set(mask, option DiffOptions) DiffOptions { return mask | option }

// Clear removes an option from a mask.
func Clear(mask, option DiffOptions) DiffOptions { return mask &^ option }

// Has probes a mask for a given option
func Has(mask, option DiffOptions) bool { return mask&option != 0 }

// DiffEntry describes a specific change in a discovery document.
// Because a discovery document is structured, a change can contain child changes (e.g. an object and its fields).
type DiffEntry struct {
	ChangeType  ChangeType
	ElementKind ElementKind
	ElementID   string
	OldValue    string
	NewValue    string
	Children    []*DiffEntry
}

// ChangeType describes whether this change is a add/modify/delete.
type ChangeType string

const (
	// AddChange indicates something was added.
	AddChange ChangeType = "ADD"
	// ModifyChange indicates a value was present and modified.
	ModifyChange ChangeType = "MODIFY"
	// DeleteChange indicates a value was deleted.
	DeleteChange ChangeType = "DELETED"
)

// DiffDocs compares two discovery documents for changes.
func DiffDocs(old, new *Document, options DiffOptions) []*DiffEntry {
	var entries []*DiffEntry
	// Always check basic identifiers for changes
	if diffs := compareIdentifiers(old, new); diffs != nil {
		entries = append(entries, diffs...)
	}
	// Versioning often changes, so skip these if not requested.
	if Has(options, VersioningOption) {
		if diffs := compareVersioning(old, new); diffs != nil {
			entries = append(entries, diffs...)
		}
	}
	// Check for path/endpoints changes
	if Has(options, ServiceOption) {
		if diffs := compareServiceOptions(old, new); diffs != nil {
			entries = append(entries, diffs...)
		}
	}
	if Has(options, SchemaOption) {
		if diffs := compareSchemas(old, new, Has(options, DescriptionOption)); diffs != nil {
			entries = append(entries, diffs...)
		}
	}
	if Has(options, ResourceOption) {
		if diffs := compareResources(old.Resources, new.Resources, Has(options, DescriptionOption)); diffs != nil {
			entries = append(entries, diffs...)
		}
	}
	return entries
}

func compareIdentifiers(old, new *Document) []*DiffEntry {
	diffs, err := getFieldDiffs(old, new, []string{"ID", "Name"}, false)
	if err != nil {
		log.Fatalf("compareIdentifiers: %v", err)
		return nil
	}
	return diffs
}

func compareVersioning(old, new *Document) []*DiffEntry {
	diffs, err := getFieldDiffs(old, new, []string{"Revision"}, false)
	if err != nil {
		log.Fatalf("compareVersioning: %v", err)
		return nil
	}
	return diffs
}

func compareServiceOptions(old, new *Document) []*DiffEntry {
	//TODO(shollyman): compare Features, which is a slice of strings
	diffs, err := getFieldDiffs(old, new, []string{"Title", "RootURL", "ServicePath", "BasePath", "DocumentationLink"}, false)
	if err != nil {
		log.Fatalf("compareVersioning: %v", err)
		return nil
	}
	return diffs
}

func compareSchemas(old, new *Document, checkDescriptions bool) []*DiffEntry {

	var partialDiffs []*DiffEntry

	// Collect the keys from the schemas of both new and old maps, and then sort it
	// to avoid confusion from element reordering.
	keys := make(map[string]bool)

	for k := range old.Schemas {
		keys[k] = true
	}
	for k := range new.Schemas {
		keys[k] = true
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Walk each of the keys, and determine if we've added, removed, changed any specific
	// schemas.  We inspect the contents of each schema and attach those changes of children.
	for _, k := range sortedKeys {
		keyName := k

		entry := &DiffEntry{
			ElementKind: SchemaKind,
			ElementID:   fmt.Sprintf("Schemas.%s", keyName),
		}
		oldSchema := old.Schemas[keyName]
		newSchema := new.Schemas[keyName]
		if oldSchema == nil {
			if newSchema != nil {
				fieldDiffs := compareSingleSchema(&Schema{}, newSchema, checkDescriptions)
				if fieldDiffs != nil {
					entry.ChangeType = AddChange
					// amend the child diffs to attribute them as additions
					for _, f := range fieldDiffs {
						f.ChangeType = AddChange
						entry.Children = append(entry.Children, f)
					}
					partialDiffs = append(partialDiffs, entry)
				}
			}
			continue
		}
		if newSchema != nil {
			fieldDiffs := compareSingleSchema(oldSchema, newSchema, checkDescriptions)
			if fieldDiffs != nil {
				entry.ChangeType = ModifyChange
				entry.Children = fieldDiffs
				partialDiffs = append(partialDiffs, entry)
			}
		} else {
			entry.ChangeType = DeleteChange
			partialDiffs = append(partialDiffs, entry)
		}
	}
	return partialDiffs
}

func compareSingleSchema(old, new *Schema, checkDescriptions bool) []*DiffEntry {

	// TODO: ItemSchema, AdditionalProperties, Enums, EnumDescriptions, Kind
	diffs, err := getFieldDiffs(old, new, []string{"ID", "Type", "Format", "Description", "Ref", "Default", "Pattern", "Name"}, false)
	if err != nil {
		log.Fatalf("compareSingleSchema: %v", err)
		return nil
	}
	return diffs
}

func compareMethods(old, new MethodList, checkDescriptions bool) []*DiffEntry {
	return nil
}

func compareSingleMethod(old, new *Method, checkDescriptions bool) []*DiffEntry {

	// TODO Parameters
	// TODO ParameterOrder
	// TODO Request (schema)
	// TODO Response (schema)
	// TODO Scopes
	// TODO MediaUpload
	// TODO(maybe?) JSONMap

	diffs, err := getFieldDiffs(old, new, []string{"Name", "ID", "Path", "HTTPMethod", "Description", "SupportsMediaDownload"}, false)
	if err != nil {
		log.Fatalf("compareSingleMethod: %v", err)
		return nil
	}
	return diffs
}

func compareResources(oldList, newList ResourceList, checkDescriptions bool) []*DiffEntry {
	// Resources are presented in the discovery document using a list, rather than keyed by
	// identifier in a map as schemas are.  Thereforce, we use the Name field of each resource
	// element for the comparison identity.

	//The discovery reference doesn't indicate special uniqueness rules.

	var partialDiffs []*DiffEntry

	// first, we'll place the old and new resources into maps keyed on Name to make accessing simpler.
	keys := make(map[string]bool)
	oldMap := make(map[string]*Resource)
	newMap := make(map[string]*Resource)
	for _, m := range oldList {
		keyName := m.Name
		oldMap[keyName] = m
		keys[keyName] = true
	}
	for _, m := range newList {
		keyName := m.Name
		newMap[keyName] = m
		keys[keyName] = true
	}

	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Walk each of the keys to compare resources.
	for _, k := range sortedKeys {
		keyName := k

		entry := &DiffEntry{
			ElementKind: ResourceKind,
			ElementID:   fmt.Sprintf("Resources.%s", keyName),
		}
		oldResource := oldMap[keyName]
		newResource := newMap[keyName]
		if oldResource == nil {
			if newResource != nil {
				fieldDiffs := compareSingleResource(&Resource{}, newResource, checkDescriptions)
				if fieldDiffs != nil {
					entry.ChangeType = AddChange
					// recursively amend this, since all "modifications" are actually additions
					amendChangeType(fieldDiffs, AddChange)
					entry.Children = fieldDiffs
					partialDiffs = append(partialDiffs, entry)
				}
			}
			continue
		}
		if newResource != nil {
			fieldDiffs := compareSingleResource(oldResource, newResource, checkDescriptions)
			if fieldDiffs != nil {
				entry.ChangeType = ModifyChange
				entry.Children = fieldDiffs
				partialDiffs = append(partialDiffs, entry)
			}
		} else {
			entry.ChangeType = DeleteChange
			partialDiffs = append(partialDiffs, entry)
		}
	}
	return partialDiffs
}

func amendChangeType(entries []*DiffEntry, newType ChangeType) {
	for _, e := range entries {
		e.ChangeType = newType
		if e.Children != nil {
			amendChangeType(e.Children, newType)
		}
	}
}

func compareSingleResource(old, new *Resource, checkDescriptions bool) []*DiffEntry {
	// It's turtles all the way down.  A resource can have a list of resources as children, as well
	// as a list of methods.
	var partialDiffs []*DiffEntry

	if d := diffString("Name", old.Name, new.Name); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	if dSlice := compareResources(old.Resources, new.Resources, checkDescriptions); dSlice != nil {
		partialDiffs = append(partialDiffs, dSlice...)
	}
	if dSlice := compareMethods(old.Methods, new.Methods, checkDescriptions); dSlice != nil {
		partialDiffs = append(partialDiffs, dSlice...)
	}
	return partialDiffs
}

func renderDiff(entries []*DiffEntry) string {
	return renderDiffInternal(entries, 0)
}

func renderDiffInternal(entries []*DiffEntry, level int) string {
	if entries == nil {
		return ""
	}
	var buf bytes.Buffer
	for _, e := range entries {
		if e != nil {
			buf.WriteString(strings.Repeat(" ", level*2))
			buf.WriteString(fmt.Sprintf("%s %s%s", renderChangeType(e.ChangeType), renderElementKind(e.ElementKind), e.ElementID))
			if e.ChangeType == ModifyChange && canRenderDelta(e.ElementKind) {
				buf.WriteString(fmt.Sprintf(" [ \"%s\" ==> \"%s\" ]", e.OldValue, e.NewValue))
			}
			if e.ChangeType == AddChange && e.NewValue != "" {
				buf.WriteString(fmt.Sprintf(" [ \"%s\" ]", e.NewValue))
			}
			buf.WriteString("\n")
			if e.Children != nil {
				buf.WriteString(renderDiffInternal(e.Children, level+1))
			}
		}
	}
	return buf.String()
}

func renderChangeType(t ChangeType) string {
	switch t {
	case AddChange:
		return "+"
	case ModifyChange:
		return "M"
	case DeleteChange:
		return "-"
	default:
		return "?"
	}
}

func renderElementKind(e ElementKind) string {
	switch e {
	case SchemaKind:
		return "<Schema> "
	case MethodKind:
		return "<Method> "
	case ResourceKind:
		return "<Resource> "
	case StringFieldKind:
		return "."
	case BoolFieldKind:
		return "."
	default:
		return "???"
	}
}

func canRenderDelta(e ElementKind) bool {
	if e == StringFieldKind || e == BoolFieldKind {
		return true
	}
	return false
}

func diffString(id, old, new string) *DiffEntry {
	if old != new {
		return &DiffEntry{
			ChangeType:  ModifyChange,
			ElementKind: StringFieldKind,
			ElementID:   id,
			OldValue:    old,
			NewValue:    new,
		}
	}
	return nil
}

func diffBool(id string, old, new bool) *DiffEntry {
	if old != new {
		return &DiffEntry{
			ChangeType:  ModifyChange,
			ElementKind: BoolFieldKind,
			ElementID:   id,
			OldValue:    fmt.Sprintf("%t", old),
			NewValue:    fmt.Sprintf("%t", new),
		}
	}
	return nil
}

// getFieldDiffs is able to compute diffs for simple fields like strings/bools, using reflection,
func getFieldDiffs(old, new interface{}, fieldNames []string, checkDescriptions bool) ([]*DiffEntry, error) {
	var partialDiffs []*DiffEntry

	vOld := reflect.ValueOf(old).Elem()
	vNew := reflect.ValueOf(new).Elem()

	if !vOld.IsValid() || !vNew.IsValid() {
		return nil, fmt.Errorf("getFieldDiffs: invalid references for old and new")
	}
	if vOld.Type() != vNew.Type() {
		return nil, fmt.Errorf("getFieldDiffs: incompatible types %v and %v", vOld.Type(), vNew.Type())
	}
	for _, fn := range fieldNames {
		fOld := vOld.FieldByName(fn)
		fNew := vNew.FieldByName(fn)
		if !fOld.IsValid() || !fNew.IsValid() {
			return nil, fmt.Errorf("getFieldDiffs: field %s invalid", fn)
		}
		switch fOld.Kind() {
		case reflect.String:
			if checkDescriptions || fn != "Description" {
				if fOld.String() != fNew.String() {
					partialDiffs = append(partialDiffs, &DiffEntry{
						ChangeType:  ModifyChange,
						ElementKind: StringFieldKind,
						ElementID:   fn,
						OldValue:    fOld.String(),
						NewValue:    fNew.String(),
					})
				}
			}
		case reflect.Bool:
			if fOld.Bool() != fNew.Bool() {
				partialDiffs = append(partialDiffs, &DiffEntry{
					ChangeType:  ModifyChange,
					ElementKind: BoolFieldKind,
					ElementID:   fn,
					OldValue:    fmt.Sprintf("%t", fOld.Bool()),
					NewValue:    fmt.Sprintf("%t", fNew.Bool()),
				})
			}
		default:
			return nil, fmt.Errorf("getFieldDiffs: field %s not simple scalar", fn)
		}
	}
	return partialDiffs, nil
}

func getObjectDiffs(old, new interface{}, checkDescriptions bool) ([]*DiffEntry, error) {
	tOld := reflect.TypeOf(old).Elem()
	tNew := reflect.TypeOf(new).Elem()

	if tOld.Name() != tNew.Name() {
		return nil, fmt.Errorf("mismatch on old/new types: old is %s, new is %s", tOld.Name(), tNew.Name())
	}

	vOld := reflect.ValueOf(old).Elem()
	vNew := reflect.ValueOf(new).Elem()

	if vOld.Type() != vNew.Type() {
		return nil, fmt.Errorf("mismatch on value types: %v, %v", vOld.Type(), vNew.Type())
	}
	fmt.Printf("type name: %s\n", tOld.Name())

	return nil, nil
}
