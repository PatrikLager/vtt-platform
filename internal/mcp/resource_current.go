package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Keep these keyed by message name, never by tool: handlerFor has no
// per-command switch.
const (
	actorMessage    protoreflect.FullName = "vtt.v1.Actor"
	resourceMessage protoreflect.FullName = "vtt.v1.Resource"
	currentField    protoreflect.Name     = "current"
)

// refuseValuelessCurrent refuses arguments holding a vtt.v1.Resource whose
// current is written null or left out, naming the enclosing actor and the
// resource's key (VTT-671). Run it only on arguments protojson has accepted:
// it reads them by their descriptor and trusts the shapes protojson checked.
func refuseValuelessCurrent(md protoreflect.MessageDescriptor, args []byte) error {
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	obj, _ := v.(map[string]any)
	return walkForCurrent(md, obj, "", "")
}

func walkForCurrent(md protoreflect.MessageDescriptor, obj map[string]any, actorID, path string) error {
	if obj == nil {
		return nil
	}
	switch md.FullName() {
	case actorMessage:
		actorID, path = stringOf(obj, md, "actor_id"), ""
	case resourceMessage:
		if cur, ok := valueOf(obj, md.Fields().ByName(currentField)); !ok || cur == nil {
			return fmt.Errorf("actor %q: %s.current must be a number, 0 included; a resource gives its current", actorID, path)
		}
		return nil
	}
	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if fd.Message() == nil || fd.Message().FullName().Parent() == "google.protobuf" {
			continue
		}
		v, _ := valueOf(obj, fd)
		name := string(fd.Name())
		if !fd.IsMap() {
			// A repeated message field reads as no object here: give it a list arm
			// before a command can hold a vtt.v1.Resource inside one.
			child, _ := v.(map[string]any)
			if err := walkForCurrent(fd.Message(), child, actorID, join(path, name)); err != nil {
				return err
			}
			continue
		}
		vmd := fd.MapValue().Message()
		if vmd == nil {
			continue
		}
		entries, _ := v.(map[string]any)
		for _, k := range slices.Sorted(maps.Keys(entries)) {
			child, _ := entries[k].(map[string]any)
			if err := walkForCurrent(vmd, child, actorID, join(path, name+"."+k)); err != nil {
				return err
			}
		}
	}
	return nil
}

// valueOf reads a field by either of the names protojson accepts.
func valueOf(obj map[string]any, fd protoreflect.FieldDescriptor) (any, bool) {
	if v, ok := obj[fd.JSONName()]; ok {
		return v, true
	}
	v, ok := obj[string(fd.Name())]
	return v, ok
}

func stringOf(obj map[string]any, md protoreflect.MessageDescriptor, name protoreflect.Name) string {
	v, _ := valueOf(obj, md.Fields().ByName(name))
	s, _ := v.(string)
	return s
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}
