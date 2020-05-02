// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package disco

import (
	"bytes"
	"fmt"
	"strings"
)

// DiffOptions is a bitmask for defining diff behavior.
type DiffOptions uint16

const (
	VersioningOption DiffOptions = 1 << iota
	DescriptionOption
	MethodsOption
	SchemaOption
	ServiceOption
)

// AllOptions enables all bits, even unused.
const AllOptions = DiffOptions(^uint16(0))

// Set applies an option to a mask.
func Set(mask, option DiffOptions) DiffOptions { return mask | option }

// Clear removes an option from a mask.
func Clear(mask, option DiffOptions) DiffOptions { return mask &^ option }

// Has probes a mask for a given option
func Has(mask, option DiffOptions) bool { return mask&option != 0 }

// DiffEntry describes a specific change in a discovery document.
type DiffEntry struct {
	ChangeType  ChangeType
	ElementKind string
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

func compareSchema(old, new *Document, checkDescriptions bool) []*DiffEntry {
	return nil
}

func compareMethods(old, new *Document, checkDescriptions bool) []*DiffEntry {
	return nil
}

func compareResources(old, new *Document, checkDesciptions bool) []*DiffEntry {
	return nil
}

func printDiff(entries []*DiffEntry) string {
	return printDiffInternal(entries, 0)
}

func printDiffInternal(entries []*DiffEntry, level int) string {
	if entries == nil {
		return ""
	}
	var buf bytes.Buffer
	for _, e := range entries {
		if e != nil {
			buf.WriteString(strings.Repeat(" ", level*2))
			buf.WriteString(fmt.Sprintf("%s %s %s", e.ChangeType, e.ElementKind, e.ElementID))
			if e.ChangeType == ModifyChange {
				buf.WriteString(fmt.Sprintf(" [ %s ==> %s ]", e.OldValue, e.NewValue))
			}
			if e.ChangeType == AddChange && e.NewValue != "" {
				buf.WriteString(fmt.Sprintf("[ %s ]", e.NewValue))
			}
			buf.WriteString("\n")
			if e.Children != nil {
				buf.WriteString(printDiffInternal(e.Children, level+1))
			}
		}
	}
	return buf.String()
}

/*
func compareSchemaMap(old, new *Document) []*DiffEntry {
	var diffs []*DiffEntry

	// combine keys from old and new
	keys := make(map[string]bool)

	for k := range old.Schemas {
		keys[k] = true
	}
	for k := range new.Schemas {
		keys[k] = true
	}
	var sortedKeys []string
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, k := range sortedKeys {
		oldSchema := old.Schemas[k]
		newSchema := new.Schemas[k]
		if oldSchema == nil {
			if newSchema != nil {
				d := compareSchema(k, &Schema{}, newSchema)
				d.ChangeType = AddChange
				for _, v := range d.Children {
					v.ChangeType = AddChange
				}
				diffs = append(diffs, d)
			}
		}
		if newSchema != nil {
			d := compareSchema(k, oldSchema, newSchema)
			if d != nil {
				diffs = append(diffs, d)
			}
		} else {
			diffs = append(diffs, &DiffEntry{
				ChangeType:  ChangeTypeDelete,
				ElementKind: "schema",
				ElementID:   k,
			})
		}
	}
	if len(diffs) > 0 {
		return diffs
	}
	return nil
}

func compareSchema(name string, old, new *Schema) *DiffEntry {
	if old == nil || new == nil {
		return nil
	}
	baseDiff := &DiffEntry{
		ChangeType:  ChangeTypeModify,
		ElementKind: "schema",
		ElementID:   name,
	}

	var childDiffs []*DiffEntry
	maybeDiffs := []*DiffEntry{
		diffString("ID", old.ID, new.ID),
		diffString("Type", old.Type, new.Type),
		diffString("Format", old.Format, new.Format),
		diffString("Description", old.Description, new.Description),
		diffString("Ref", old.Ref, new.Ref),
		diffString("Default", old.Default, new.Default),
		diffString("Pattern", old.Pattern, new.Pattern),
	}

	for _, d := range maybeDiffs {
		if d != nil {
			childDiffs = append(childDiffs, d)
		}
	}

	if len(childDiffs) > 0 {
		baseDiff.Children = childDiffs
		return baseDiff
	}
	return nil
}
*/

func diffString(kind, old, new string) *DiffEntry {
	if old != new {
		return &DiffEntry{
			ChangeType:  ModifyChange,
			ElementKind: kind,
			OldValue:    old,
			NewValue:    new,
		}
	}
	return nil
}
