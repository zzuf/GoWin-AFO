// Package com provides the minimal ownership and vtable runtime used by
// generated COM consumer bindings. COM objects are never assigned finalizers;
// callers must balance references and close owned allocations explicitly.
package com
