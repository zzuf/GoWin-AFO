// Package generator exposes stable command-facing APIs while implementation
// details remain under generator/internal.
package generator

import (
	"context"

	"go-windows-api.local/generator/internal/source"
)

type SourceManager = source.Manager
type SourceResult = source.Result
type LockedSource = source.Locked
type SourceUpdate = source.Update

func OpenSourceManager(root string) (*SourceManager, error)                      { return source.Open(root) }
func FetchSources(ctx context.Context, m *SourceManager) ([]SourceResult, error) { return m.Fetch(ctx) }
