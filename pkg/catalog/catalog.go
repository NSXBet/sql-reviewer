package catalog

import (
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// FinderContext is the context for finder.
type FinderContext struct {
	// CheckIntegrity defines the policy for integrity checking.
	CheckIntegrity bool

	// EngineType is the engine type for database engine.
	EngineType types.Engine

	// Ignore case sensitive is the policy for identifier name comparison case-sensitive.
	IgnoreCaseSensitive bool
}

// Copy returns the deep copy.
func (ctx *FinderContext) Copy() *FinderContext {
	return &FinderContext{
		CheckIntegrity:      ctx.CheckIntegrity,
		EngineType:          ctx.EngineType,
		IgnoreCaseSensitive: ctx.IgnoreCaseSensitive,
	}
}

// Finder is the service for finding schema information in database.
type Finder struct {
	Origin *DatabaseState
	Final  *DatabaseState
}

// NewFinder creates a new finder.
func NewFinder(database *types.DatabaseSchemaMetadata, ctx *FinderContext) *Finder {
	return &Finder{Origin: newDatabaseState(database, ctx), Final: newDatabaseState(database, ctx)}
}

// NewEmptyFinder creates a finder with empty database.
func NewEmptyFinder(ctx *FinderContext) *Finder {
	return &Finder{
		Origin: newDatabaseState(&types.DatabaseSchemaMetadata{}, ctx),
		Final:  newDatabaseState(&types.DatabaseSchemaMetadata{}, ctx),
	}
}

// WalkThrough does the walk through.
func (f *Finder) WalkThrough(ast any) error {
	return f.Final.WalkThrough(ast)
}
