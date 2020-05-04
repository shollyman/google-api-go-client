// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package disco

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
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
	return entries
}

func compareIdentifiers(old, new *Document) []*DiffEntry {
	var partialDiffs []*DiffEntry
	if d := diffString("ID", old.Name, new.Name); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	if d := diffString("Name", old.Name, new.Name); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	return partialDiffs
}

func compareVersioning(old, new *Document) []*DiffEntry {
	var partialDiffs []*DiffEntry
	if d := diffString("Revision", old.Revision, new.Revision); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	return partialDiffs
}

func compareServiceOptions(old, new *Document) []*DiffEntry {
	var partialDiffs []*DiffEntry
	if d := diffString("Title", old.Title, new.Title); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	if d := diffString("RootURL", old.RootURL, new.RootURL); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	if d := diffString("ServicePath", old.ServicePath, new.ServicePath); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	if d := diffString("BasePath", old.BasePath, new.BasePath); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	if d := diffString("DocumentationLink", old.DocumentationLink, new.DocumentationLink); d != nil {
		partialDiffs = append(partialDiffs, d)
	}
	//TODO(shollyman): compare Features, which is a slice of strings)
	return partialDiffs
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
	var fieldDiffs []*DiffEntry

	if d := diffString("ID", old.ID, new.ID); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("Type", old.Type, new.Type); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("Format", old.Format, new.Format); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if checkDescriptions {
		if d := diffString("Description", old.Description, new.Description); d != nil {
			fieldDiffs = append(fieldDiffs, d)
		}
	}
	if d := diffString("Ref", old.Ref, new.Ref); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("Default", old.Default, new.Default); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("Pattern", old.Pattern, new.Pattern); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("Name", old.Name, new.Name); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	// TODO: ItemSchema, AdditionalProperties, Enums, EnumDescriptions, Kind
	return fieldDiffs
}

func compareSingleMethod(old, new *Method, checkDescriptions bool) []*DiffEntry {
	var fieldDiffs []*DiffEntry
	if d := diffString("Name", old.Name, new.Name); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("ID", old.ID, new.ID); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("Path", old.Path, new.Path); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if d := diffString("HTTPMethod", old.HTTPMethod, new.HTTPMethod); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}
	if checkDescriptions {
		if d := diffString("Description", old.HTTPMethod, new.HTTPMethod); d != nil {
			fieldDiffs = append(fieldDiffs, d)
		}
	}

	// TODO Parameters
	// TODO ParameterOrder
	// TODO Request (schema)
	// TODO Response (schema)
	// TODO Scopes
	// TODO MediaUpload

	if d := diffBool("SupportsMediaDownload", old.SupportsMediaDownload, new.SupportsMediaDownload); d != nil {
		fieldDiffs = append(fieldDiffs, d)
	}

	// TODO(maybe?) JSONMap

	return fieldDiffs
}

func compareResources(old, new *Document, checkDesciptions bool) []*DiffEntry {
	// TODO: implement
	return nil
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
