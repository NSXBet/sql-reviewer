package catalog

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
)

type mysqlParseResult struct {
	Tree     antlr.Tree
	BaseLine int
}

// ConvertMySQLParseResults converts mysqlparser.ParseResult to the internal format for catalog walkthrough
func ConvertMySQLParseResults(parseResults []*mysqlparser.ParseResult) []*mysqlParseResult {
	results := make([]*mysqlParseResult, len(parseResults))
	for i, pr := range parseResults {
		results[i] = &mysqlParseResult{
			Tree:     pr.Tree,
			BaseLine: pr.BaseLine,
		}
	}
	return results
}

func (d *DatabaseState) mysqlWalkThrough(ast any) error {
	// We define the Catalog as Database -> Schema -> Table. The Schema is only for PostgreSQL.
	// So we use a Schema whose name is empty for other engines, such as MySQL.
	// If there is no empty-string-name schema, create it to avoid corner cases.
	if _, exists := d.schemaSet[""]; !exists {
		d.createSchema()
	}

	nodeList, ok := ast.([]*mysqlParseResult)
	if !ok {
		return fmt.Errorf("invalid ast type %T", ast)
	}
	for _, node := range nodeList {
		if err := d.mysqlChangeState(node); err != nil {
			return err
		}
	}

	return nil
}

type mysqlListener struct {
	*mysql.BaseMySQLParserListener

	baseLine      int
	lineNumber    int
	text          string
	databaseState *DatabaseState
	err           *WalkThroughError
}

func (l *mysqlListener) EnterQuery(ctx *mysql.QueryContext) {
	l.text = ctx.GetParser().GetTokenStream().GetTextFromRuleContext(ctx)
	l.lineNumber = l.baseLine + ctx.GetStart().GetLine()
}

// EnterRenameTableStatement is called when production renameTableStatement is entered.
func (l *mysqlListener) EnterRenameTableStatement(ctx *mysql.RenameTableStatementContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	for _, pair := range ctx.AllRenamePair() {
		schema, exists := l.databaseState.schemaSet[""]
		if !exists {
			schema = l.databaseState.createSchema()
		}

		_, oldTableName := mysqlparser.NormalizeMySQLTableRef(pair.TableRef())
		_, newTableName := mysqlparser.NormalizeMySQLTableName(pair.TableName())

		if l.databaseState.mysqlTheCurrentDatabase(pair) {
			if compareIdentifier(oldTableName, newTableName, l.databaseState.ctx.IgnoreCaseSensitive) {
				return
			}
			table, exists := schema.getTable(oldTableName)
			if !exists {
				if schema.ctx.CheckIntegrity {
					l.err = NewTableNotExistsError(oldTableName)
					return
				}
				table = schema.createIncompleteTable(oldTableName)
			}
			if _, exists := schema.getTable(newTableName); exists {
				l.err = NewTableExistsError(newTableName)
				return
			}
			delete(schema.tableSet, table.name)
			table.name = newTableName
			schema.tableSet[table.name] = table
		} else if l.databaseState.mysqlMoveToOtherDatabase(pair) {
			_, exists := schema.getTable(oldTableName)
			if !exists && schema.ctx.CheckIntegrity {
				l.err = NewTableNotExistsError(oldTableName)
				return
			}
			delete(schema.tableSet, oldTableName)
		} else {
			l.err = NewAccessOtherDatabaseError(l.databaseState.name, l.databaseState.mysqlTargetDatabase(pair))
			return
		}
	}
}

// EnterCreateDatabase is called when production createDatabase is entered.
func (l *mysqlListener) EnterCreateDatabase(ctx *mysql.CreateDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.SchemaName() == nil {
		return
	}

	databaseName := mysqlparser.NormalizeMySQLSchemaName(ctx.SchemaName())
	if !l.databaseState.isCurrentDatabase(databaseName) {
		l.err = &WalkThroughError{
			Type:    ErrorTypeAccessOtherDatabase,
			Content: fmt.Sprintf("Database `%s` is not the current database `%s`", databaseName, l.databaseState.name),
		}
	}
}

// EnterDropDatabase is called when production dropDatabase is entered.
func (l *mysqlListener) EnterDropDatabase(ctx *mysql.DropDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.SchemaRef() == nil {
		return
	}

	databaseName := mysqlparser.NormalizeMySQLSchemaRef(ctx.SchemaRef())
	if l.databaseState.isCurrentDatabase(databaseName) {
		l.databaseState.deleted = true
	} else {
		l.err = &WalkThroughError{
			Type:    ErrorTypeAccessOtherDatabase,
			Content: fmt.Sprintf("Database `%s` is not the current database `%s`", databaseName, l.databaseState.name),
		}
	}
}

// EnterAlterDatabase is called when production alterDatabase is entered.
func (l *mysqlListener) EnterAlterDatabase(ctx *mysql.AlterDatabaseContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}

	// Check if this is modifying the current database
	if ctx.SchemaRef() != nil {
		databaseName := mysqlparser.NormalizeMySQLSchemaRef(ctx.SchemaRef())
		if !l.databaseState.isCurrentDatabase(databaseName) {
			l.err = &WalkThroughError{
				Type:    ErrorTypeAccessOtherDatabase,
				Content: fmt.Sprintf("Database `%s` is not the current database `%s`", databaseName, l.databaseState.name),
			}
			return
		}
	}
	// If no database name is specified, it applies to the current database

	// Process database options
	for _, option := range ctx.AllAlterDatabaseOption() {
		if option.CreateDatabaseOption() != nil {
			dbOption := option.CreateDatabaseOption()
			if dbOption.DefaultCharset() != nil && dbOption.DefaultCharset().CharsetName() != nil {
				l.databaseState.characterSet = mysqlparser.NormalizeMySQLCharsetName(dbOption.DefaultCharset().CharsetName())
			}
			if dbOption.DefaultCollation() != nil && dbOption.DefaultCollation().CollationName() != nil {
				l.databaseState.collation = mysqlparser.NormalizeMySQLCollationName(dbOption.DefaultCollation().CollationName())
			}
		}
	}
}

func (d *DatabaseState) mysqlChangeState(in *mysqlParseResult) (err *WalkThroughError) {
	defer func() {
		if err == nil {
			return
		}
		if err.Line == 0 {
			err.Line = in.BaseLine
		}
	}()

	if d.deleted {
		return &WalkThroughError{
			Type:    ErrorTypeDatabaseIsDeleted,
			Content: fmt.Sprintf("Database `%s` is deleted", d.name),
		}
	}

	listener := &mysqlListener{
		baseLine:      in.BaseLine,
		databaseState: d,
	}
	antlr.ParseTreeWalkerDefault.Walk(listener, in.Tree)
	if listener.err != nil {
		if listener.err.Line == 0 {
			listener.err.Line = listener.lineNumber
		}
		return listener.err
	}
	return nil
}

// EnterCreateTable is called when production createTable is entered.
func (l *mysqlListener) EnterCreateTable(ctx *mysql.CreateTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableName() == nil {
		return
	}
	databaseName, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	if databaseName != "" && !l.databaseState.isCurrentDatabase(databaseName) {
		l.err = &WalkThroughError{
			Type:    ErrorTypeAccessOtherDatabase,
			Content: fmt.Sprintf("Database `%s` is not the current database `%s`", databaseName, l.databaseState.name),
		}
		return
	}

	schema, exists := l.databaseState.schemaSet[""]
	if !exists {
		schema = l.databaseState.createSchema()
	}
	if _, exists = schema.getTable(tableName); exists {
		if ctx.IfNotExists() != nil {
			return
		}
		l.err = &WalkThroughError{
			Type:    ErrorTypeTableExists,
			Content: fmt.Sprintf("Table `%s` already exists", tableName),
		}
		return
	}

	if ctx.DuplicateAsQueryExpression() != nil {
		l.err = &WalkThroughError{
			Type:    ErrorTypeUseCreateTableAs,
			Content: fmt.Sprintf("Disallow the CREATE TABLE AS statement but \"%s\" uses", l.text),
		}
		return
	}

	if ctx.LIKE_SYMBOL() != nil {
		_, referTable := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
		l.err = l.databaseState.mysqlCopyTable(databaseName, tableName, referTable)
		return
	}

	table := &TableState{
		name:           tableName,
		engine:         newEmptyStringPointer(),
		collation:      newEmptyStringPointer(),
		comment:        newEmptyStringPointer(),
		columnSet:      make(columnStateMap),
		indexSet:       make(IndexStateMap),
		dependencyView: make(map[string]bool),
	}
	schema.tableSet[table.name] = table

	if ctx.TableElementList() == nil {
		return
	}

	hasAutoIncrement := false
	for _, tableElement := range ctx.TableElementList().AllTableElement() {
		switch {
		// handle column
		case tableElement.ColumnDefinition() != nil:
			if tableElement.ColumnDefinition().FieldDefinition() == nil || tableElement.ColumnDefinition().ColumnName() == nil {
				continue
			}
			if mysqlparser.IsAutoIncrement(tableElement.ColumnDefinition().FieldDefinition()) {
				if hasAutoIncrement {
					l.err = &WalkThroughError{
						Type: ErrorTypeAutoIncrementExists,
						// The content comes from MySQL error content.
						Content: fmt.Sprintf("There can be only one auto column for table `%s`", table.name),
					}
				}
				hasAutoIncrement = true
			}
			_, _, columnName := mysqlparser.NormalizeMySQLColumnName(tableElement.ColumnDefinition().ColumnName())
			if err := table.mysqlCreateColumn(l.databaseState.ctx, columnName, tableElement.ColumnDefinition().FieldDefinition(), nil /* position */); err != nil {
				err.Line = l.baseLine + tableElement.GetStart().GetLine()
				l.err = err
				return
			}
		case tableElement.TableConstraintDef() != nil:
			if err := table.mysqlCreateConstraint(l.databaseState.ctx, tableElement.TableConstraintDef()); err != nil {
				err.Line = tableElement.GetStart().GetLine()
				l.err = err
				return
			}
		}
	}
}

// isCurrentDatabase returns true if the given database is the current database of the state.
func (d *DatabaseState) isCurrentDatabase(database string) bool {
	return compareIdentifier(d.name, database, d.ctx.IgnoreCaseSensitive)
}

func (d *DatabaseState) mysqlCopyTable(databaseName, tableName, referTable string) *WalkThroughError {
	targetTable, err := d.mysqlFindTableState(databaseName, referTable)
	if err != nil {
		return err
	}

	schema := d.schemaSet[""]
	table := targetTable.copy()
	table.name = tableName
	schema.tableSet[table.name] = table
	return nil
}

func (d *DatabaseState) mysqlFindTableState(databaseName, tableName string) (*TableState, *WalkThroughError) {
	if databaseName != "" && !d.isCurrentDatabase(databaseName) {
		return nil, NewAccessOtherDatabaseError(d.name, databaseName)
	}

	schema, exists := d.schemaSet[""]
	if !exists {
		schema = d.createSchema()
	}

	table, exists := schema.getTable(tableName)
	if !exists {
		if schema.ctx.CheckIntegrity {
			return nil, NewTableNotExistsError(tableName)
		}
		table = schema.createIncompleteTable(tableName)
	}

	return table, nil
}

func (s *SchemaState) createIncompleteTable(name string) *TableState {
	table := &TableState{
		name:           name,
		columnSet:      make(columnStateMap),
		indexSet:       make(IndexStateMap),
		dependencyView: make(map[string]bool),
	}
	s.tableSet[name] = table
	return table
}

//nolint:unused
func (t *TableState) createIncompleteColumn(name string) *ColumnState {
	column := &ColumnState{
		name:           name,
		dependencyView: make(map[string]bool),
	}
	t.columnSet[strings.ToLower(name)] = column
	return column
}

//nolint:unused
func (t *TableState) createIncompleteIndex(name string) *IndexState {
	index := &IndexState{
		name: name,
	}
	t.indexSet[strings.ToLower(name)] = index
	return index
}

func (t *TableState) mysqlCreateColumn(
	ctx *FinderContext,
	columnName string,
	fieldDef mysql.IFieldDefinitionContext,
	position *mysqlColumnPosition,
) *WalkThroughError {
	if _, exists := t.columnSet[strings.ToLower(columnName)]; exists {
		return &WalkThroughError{
			Type:    ErrorTypeColumnExists,
			Content: fmt.Sprintf("Column `%s` already exists in table `%s`", columnName, t.name),
		}
	}

	// todo: handle position.
	pos := len(t.columnSet) + 1
	if position != nil && ctx.CheckIntegrity {
		var err *WalkThroughError
		pos, err = t.mysqlReorderColumn(position)
		if err != nil {
			return err
		}
	}
	columnType := ""
	characterSet := ""
	collation := ""
	if fieldDef.DataType() == nil {
		// todo: add more error detail.
		return nil
	}
	columnType = mysqlparser.NormalizeMySQLDataType(fieldDef.DataType(), true /* compact */)
	characterSet = mysqlparser.GetCharSetName(fieldDef.DataType())
	collation = mysqlparser.GetCollationName(fieldDef)

	col := &ColumnState{
		name:           columnName,
		position:       &pos,
		defaultValue:   nil,
		nullable:       newTruePointer(),
		columnType:     newStringPointer(columnType),
		characterSet:   newStringPointer(characterSet),
		collation:      newStringPointer(collation),
		comment:        newEmptyStringPointer(),
		dependencyView: make(map[string]bool),
	}
	setNullDefault := false

	for _, attribute := range fieldDef.AllColumnAttribute() {
		if attribute == nil {
			continue
		}
		if attribute.CheckConstraint() != nil {
			// we do not deal with CHECK constraint.
			continue
		}
		// not null.
		if attribute.NullLiteral() != nil && attribute.NOT_SYMBOL() != nil {
			col.nullable = newFalsePointer()
		}
		if attribute.GetValue() != nil {
			switch attribute.GetValue().GetTokenType() {
			// default value.
			case mysql.MySQLParserDEFAULT_SYMBOL:
				if err := mysqlCheckDefault(columnName, fieldDef); err != nil {
					return err
				}
				if attribute.SignedLiteral() == nil {
					continue
				}
				// handle default null.
				if attribute.SignedLiteral().Literal() != nil && attribute.SignedLiteral().Literal().NullLiteral() != nil {
					setNullDefault = true
					continue
				}
				// handle default 'null' etc.
				defaultValue := mysqlparser.NormalizeMySQLSignedLiteral(attribute.SignedLiteral())
				col.defaultValue = &defaultValue
			// comment.
			case mysql.MySQLParserCOMMENT_SYMBOL:
				if attribute.TextLiteral() == nil {
					continue
				}
				comment := mysqlparser.NormalizeMySQLTextLiteral(attribute.TextLiteral())
				col.comment = &comment
			// on update now().
			case mysql.MySQLParserON_SYMBOL:
				if attribute.UPDATE_SYMBOL() == nil || attribute.NOW_SYMBOL() == nil {
					continue
				}
				if !mysqlparser.IsTimeType(fieldDef.DataType()) {
					return &WalkThroughError{
						Type:    ErrorTypeOnUpdateColumnNotDatetimeOrTimestamp,
						Content: fmt.Sprintf("Column `%s` use ON UPDATE but is not DATETIME or TIMESTAMP", col.name),
					}
				}
			// primary key.
			case mysql.MySQLParserKEY_SYMBOL:
				// the key attribute for in a column meaning primary key.
				col.nullable = newFalsePointer()
				// we need to check the key type which generated by tidb parser.
				if err := t.mysqlCreatePrimaryKey([]string{strings.ToLower(col.name)}, "BTREE"); err != nil {
					return err
				}
			// unique key.
			case mysql.MySQLParserUNIQUE_SYMBOL:
				// unique index.
				if err := t.mysqlCreateIndex("", []string{strings.ToLower(col.name)}, true /* unique */, "BTREE", mysql.NewEmptyTableConstraintDefContext(), mysql.NewEmptyCreateIndexContext()); err != nil {
					return err
				}
			// auto_increment.
			case mysql.MySQLParserAUTO_INCREMENT_SYMBOL:
				// we do not deal with AUTO_INCREMENT.
			// column_format.
			case mysql.MySQLParserCOLUMN_FORMAT_SYMBOL:
				// we do not deal with COLUMN_FORMAT.
			// storage.
			case mysql.MySQLParserSTORAGE_SYMBOL:
				// we do not deal with STORAGE.
			}
		}
	}

	if col.nullable != nil && !*col.nullable && setNullDefault {
		return &WalkThroughError{
			Type: ErrorTypeSetNullDefaultForNotNullColumn,
			// Content comes from MySQL Error content.
			Content: fmt.Sprintf("Invalid default value for column `%s`", col.name),
		}
	}

	t.columnSet[strings.ToLower(col.name)] = col
	return nil
}

func (t *TableState) mysqlCreateConstraint(ctx *FinderContext, constraintDef mysql.ITableConstraintDefContext) *WalkThroughError {
	if constraintDef.GetType_() != nil {
		switch constraintDef.GetType_().GetTokenType() {
		// PRIMARY KEY.
		case mysql.MySQLParserPRIMARY_SYMBOL:
			if constraintDef.KeyListVariants() == nil {
				// never reach here.
				return nil
			}
			if err := t.mysqlValidateKeyListVariants(ctx, constraintDef.KeyListVariants(), true /* primary */, false /* isSpatial*/); err != nil {
				return err
			}
			keyList := mysqlparser.NormalizeKeyListVariants(constraintDef.KeyListVariants())
			if err := t.mysqlCreatePrimaryKey(keyList, mysqlGetIndexType(constraintDef)); err != nil {
				return err
			}
		// normal KEY/INDEX.
		case mysql.MySQLParserKEY_SYMBOL, mysql.MySQLParserINDEX_SYMBOL:
			if constraintDef.KeyListVariants() == nil {
				// never reach here.
				return nil
			}
			if err := t.mysqlValidateKeyListVariants(ctx, constraintDef.KeyListVariants(), false /* primary */, false /* isSpatial */); err != nil {
				return err
			}

			indexName := ""
			if constraintDef.IndexNameAndType() != nil && constraintDef.IndexNameAndType().IndexName() != nil {
				indexName = mysqlparser.NormalizeIndexName(constraintDef.IndexNameAndType().IndexName())
			}
			keyList := mysqlparser.NormalizeKeyListVariants(constraintDef.KeyListVariants())
			if err := t.mysqlCreateIndex(indexName, keyList, false /* unique */, mysqlGetIndexType(constraintDef), constraintDef, mysql.NewEmptyCreateIndexContext()); err != nil {
				return err
			}
		// UNIQUE KEY.
		case mysql.MySQLParserUNIQUE_SYMBOL:
			if constraintDef.KeyListVariants() == nil {
				// never reach here.
				return nil
			}
			if err := t.mysqlValidateKeyListVariants(ctx, constraintDef.KeyListVariants(), false /* primary */, false /* isSpatial*/); err != nil {
				return err
			}

			indexName := ""
			if constraintDef.ConstraintName() != nil {
				indexName = mysqlparser.NormalizeConstraintName(constraintDef.ConstraintName())
			}
			if constraintDef.IndexNameAndType() != nil && constraintDef.IndexNameAndType().IndexName() != nil {
				indexName = mysqlparser.NormalizeIndexName(constraintDef.IndexNameAndType().IndexName())
			}
			keyList := mysqlparser.NormalizeKeyListVariants(constraintDef.KeyListVariants())
			if err := t.mysqlCreateIndex(indexName, keyList, true /* unique */, mysqlGetIndexType(constraintDef), constraintDef, mysql.NewEmptyCreateIndexContext()); err != nil {
				return err
			}
		// FULLTEXT KEY.
		case mysql.MySQLParserFULLTEXT_SYMBOL:
			if constraintDef.KeyListVariants() == nil {
				// never reach here.
				return nil
			}
			if err := t.mysqlValidateKeyListVariants(ctx, constraintDef.KeyListVariants(), false /* primary */, false /* isSpatial*/); err != nil {
				return err
			}
			indexName := ""
			if constraintDef.IndexName() != nil {
				indexName = mysqlparser.NormalizeIndexName(constraintDef.IndexName())
			}
			keyList := mysqlparser.NormalizeKeyListVariants(constraintDef.KeyListVariants())
			if err := t.mysqlCreateIndex(indexName, keyList, false /* unique */, mysqlGetIndexType(constraintDef), constraintDef, mysql.NewEmptyCreateIndexContext()); err != nil {
				return err
			}
		case mysql.MySQLParserFOREIGN_SYMBOL:
			// we do not deal with FOREIGN KEY constraints.
		}
	}

	// we do not deal with check constraints.
	// if constraintDef.CheckConstraint() != nil {}
	return nil
}

// mysqlValidateKeyListVariants validates the key list variants.
func (t *TableState) mysqlValidateKeyListVariants(
	ctx *FinderContext,
	keyList mysql.IKeyListVariantsContext,
	primary bool,
	isSpatial bool,
) *WalkThroughError {
	if keyList.KeyList() != nil {
		columns := mysqlparser.NormalizeKeyList(keyList.KeyList())
		if err := t.mysqlValidateColumnList(ctx, columns, primary, isSpatial); err != nil {
			return err
		}
	}
	if keyList.KeyListWithExpression() != nil {
		expressions := mysqlparser.NormalizeKeyListWithExpression(keyList.KeyListWithExpression())
		if err := t.mysqlValidateExpressionList(ctx, expressions, primary, isSpatial); err != nil {
			return err
		}
	}
	return nil
}

func (t *TableState) mysqlValidateColumnList(
	ctx *FinderContext,
	columnList []string,
	primary bool,
	isSpatial bool,
) *WalkThroughError {
	for _, columnName := range columnList {
		column, exists := t.columnSet[strings.ToLower(columnName)]
		if !exists {
			if ctx.CheckIntegrity {
				return NewColumnNotExistsError(t.name, columnName)
			}
		} else {
			if primary {
				column.nullable = newFalsePointer()
			}
			if isSpatial && column.nullable != nil && *column.nullable {
				return &WalkThroughError{
					Type: ErrorTypeSpatialIndexKeyNullable,
					// The error content comes from MySQL.
					Content: fmt.Sprintf("All parts of a SPATIAL index must be NOT NULL, but `%s` is nullable", column.name),
				}
			}
		}
	}
	return nil
}

// mysqlValidateExpressionList validates the expression list.
// TODO: update expression validation.
func (t *TableState) mysqlValidateExpressionList(
	_ *FinderContext,
	expressionList []string,
	primary bool,
	isSpatial bool,
) *WalkThroughError {
	for _, expression := range expressionList {
		column, exists := t.columnSet[strings.ToLower(expression)]
		// If expression is not a column, we do not need to validate it.
		if !exists {
			continue
		}

		if primary {
			column.nullable = newFalsePointer()
		}
		if isSpatial && column.nullable != nil && *column.nullable {
			return &WalkThroughError{
				Type: ErrorTypeSpatialIndexKeyNullable,
				// The error content comes from MySQL.
				Content: fmt.Sprintf("All parts of a SPATIAL index must be NOT NULL, but `%s` is nullable", column.name),
			}
		}
	}
	return nil
}

func mysqlGetIndexType(tableConstraint mysql.ITableConstraintDefContext) string {
	if tableConstraint.GetType_() == nil {
		return "BTREE"
	}

	// I still need to handle IndexNameAndType to get index type(algorithm).
	switch tableConstraint.GetType_().GetTokenType() {
	case mysql.MySQLParserPRIMARY_SYMBOL,
		mysql.MySQLParserKEY_SYMBOL,
		mysql.MySQLParserINDEX_SYMBOL,
		mysql.MySQLParserUNIQUE_SYMBOL:

		if tableConstraint.IndexNameAndType() != nil {
			if tableConstraint.IndexNameAndType().IndexType() != nil {
				indexType := tableConstraint.IndexNameAndType().IndexType().GetText()
				return strings.ToUpper(indexType)
			}
		}

		for _, option := range tableConstraint.AllIndexOption() {
			if option == nil || option.IndexTypeClause() == nil {
				continue
			}

			indexType := option.IndexTypeClause().IndexType().GetText()
			return strings.ToUpper(indexType)
		}
	case mysql.MySQLParserFULLTEXT_SYMBOL:
		return "FULLTEXT"
	case mysql.MySQLParserFOREIGN_SYMBOL:
	}
	// for mysql, we use BTREE as default index type.
	return "BTREE"
}

type columnPositionType int

const (
	ColumnPositionNone columnPositionType = iota
	ColumnPositionFirst
	ColumnPositionAfter
)

type mysqlColumnPosition struct {
	tp             columnPositionType
	relativeColumn string
}

// reorderColumn reorders the columns for new column and returns the new column position.
func (t *TableState) mysqlReorderColumn(position *mysqlColumnPosition) (int, *WalkThroughError) {
	switch position.tp {
	case ColumnPositionNone:
		return len(t.columnSet) + 1, nil
	case ColumnPositionFirst:
		for _, column := range t.columnSet {
			*column.position++
		}
		return 1, nil
	case ColumnPositionAfter:
		columnName := strings.ToLower(position.relativeColumn)
		column, exist := t.columnSet[columnName]
		if !exist {
			return 0, NewColumnNotExistsError(t.name, columnName)
		}
		for _, col := range t.columnSet {
			if *col.position > *column.position {
				*col.position++
			}
		}
		return *column.position + 1, nil
	}
	return 0, &WalkThroughError{
		Type:    ErrorTypeUnsupported,
		Content: fmt.Sprintf("Unsupported column position type: %d", position.tp),
	}
}

func (t *TableState) mysqlCreateIndex(
	name string,
	keyList []string,
	unique bool,
	tp string,
	tableConstraint mysql.ITableConstraintDefContext,
	createIndexDef mysql.ICreateIndexContext,
) *WalkThroughError {
	if len(keyList) == 0 {
		return &WalkThroughError{
			Type:    ErrorTypeIndexEmptyKeys,
			Content: fmt.Sprintf("Index `%s` in table `%s` has empty key", name, t.name),
		}
	}
	// construct a index name if name is empty.
	if name != "" {
		if _, exists := t.indexSet[strings.ToLower(name)]; exists {
			return NewIndexExistsError(t.name, name)
		}
	} else {
		suffix := 1
		for {
			name = keyList[0]
			if suffix > 1 {
				name = fmt.Sprintf("%s_%d", keyList[0], suffix)
			}
			if _, exists := t.indexSet[strings.ToLower(name)]; !exists {
				break
			}
			suffix++
		}
	}

	index := &IndexState{
		name:           name,
		expressionList: keyList,
		indexType:      &tp,
		unique:         &unique,
		primary:        newFalsePointer(),
		visible:        newTruePointer(),
		comment:        newEmptyStringPointer(),
	}

	// need to check the visibility of index.
	// we need a for-loop to determined the visibility of index.

	// NORMAL KEY/INDEX.
	// PRIMARY KEY.
	// UNIQUE KEY.

	// for create table statement.
	for _, attribute := range tableConstraint.AllIndexOption() {
		if attribute == nil || attribute.CommonIndexOption() == nil {
			continue
		}
		if attribute.CommonIndexOption().Visibility() != nil &&
			attribute.CommonIndexOption().Visibility().INVISIBLE_SYMBOL() != nil {
			index.visible = newFalsePointer()
		}
	}

	// for create index statement.
	for _, attribute := range createIndexDef.AllIndexOption() {
		if attribute == nil || attribute.CommonIndexOption() == nil {
			continue
		}
		if attribute.CommonIndexOption().Visibility() != nil &&
			attribute.CommonIndexOption().Visibility().INVISIBLE_SYMBOL() != nil {
			index.visible = newFalsePointer()
		}
	}

	// FULLTEXT INDEX.
	// for create table statement.
	for _, attribute := range tableConstraint.AllFulltextIndexOption() {
		if attribute == nil || attribute.CommonIndexOption() == nil {
			continue
		}
		if attribute.CommonIndexOption().Visibility() != nil &&
			attribute.CommonIndexOption().Visibility().INVISIBLE_SYMBOL() != nil {
			index.visible = newFalsePointer()
		}
	}

	// for create index statement.
	for _, attribute := range createIndexDef.AllFulltextIndexOption() {
		if attribute == nil || attribute.CommonIndexOption() == nil {
			continue
		}
		if attribute.CommonIndexOption().Visibility() != nil &&
			attribute.CommonIndexOption().Visibility().INVISIBLE_SYMBOL() != nil {
			index.visible = newFalsePointer()
		}
	}

	// SPATIAL INDEX.
	// for create table statement.
	for _, attribute := range tableConstraint.AllSpatialIndexOption() {
		if attribute == nil || attribute.CommonIndexOption() == nil {
			continue
		}
		if attribute.CommonIndexOption().Visibility() != nil &&
			attribute.CommonIndexOption().Visibility().INVISIBLE_SYMBOL() != nil {
			index.visible = newFalsePointer()
		}
	}

	// for create index statement.
	for _, attribute := range createIndexDef.AllSpatialIndexOption() {
		if attribute == nil || attribute.CommonIndexOption() == nil {
			continue
		}
		if attribute.CommonIndexOption().Visibility() != nil &&
			attribute.CommonIndexOption().Visibility().INVISIBLE_SYMBOL() != nil {
			index.visible = newFalsePointer()
		}
	}

	t.indexSet[strings.ToLower(name)] = index
	return nil
}

func (t *TableState) mysqlCreatePrimaryKey(keys []string, tp string) *WalkThroughError {
	if _, exists := t.indexSet[strings.ToLower(PrimaryKeyName)]; exists {
		return &WalkThroughError{
			Type:    ErrorTypePrimaryKeyExists,
			Content: fmt.Sprintf("Primary key exists in table `%s`", t.name),
		}
	}

	pk := &IndexState{
		name:           PrimaryKeyName,
		expressionList: keys,
		indexType:      &tp,
		unique:         newTruePointer(),
		primary:        newTruePointer(),
		visible:        newTruePointer(),
		comment:        newEmptyStringPointer(),
	}
	t.indexSet[strings.ToLower(pk.name)] = pk
	return nil
}

func mysqlCheckDefault(columnName string, fieldDefinition mysql.IFieldDefinitionContext) *WalkThroughError {
	if fieldDefinition.DataType() == nil || fieldDefinition.DataType().GetType_() == nil {
		return nil
	}

	switch fieldDefinition.DataType().GetType_().GetTokenType() {
	case mysql.MySQLParserTEXT_SYMBOL,
		mysql.MySQLParserTINYTEXT_SYMBOL,
		mysql.MySQLParserMEDIUMTEXT_SYMBOL,
		mysql.MySQLParserLONGTEXT_SYMBOL,
		mysql.MySQLParserBLOB_SYMBOL,
		mysql.MySQLParserTINYBLOB_SYMBOL,
		mysql.MySQLParserMEDIUMBLOB_SYMBOL,
		mysql.MySQLParserLONGBLOB_SYMBOL,
		mysql.MySQLParserLONG_SYMBOL,
		mysql.MySQLParserSERIAL_SYMBOL,
		mysql.MySQLParserJSON_SYMBOL,
		mysql.MySQLParserGEOMETRY_SYMBOL,
		mysql.MySQLParserGEOMETRYCOLLECTION_SYMBOL,
		mysql.MySQLParserPOINT_SYMBOL,
		mysql.MySQLParserMULTIPOINT_SYMBOL,
		mysql.MySQLParserLINESTRING_SYMBOL,
		mysql.MySQLParserMULTILINESTRING_SYMBOL,
		mysql.MySQLParserPOLYGON_SYMBOL,
		mysql.MySQLParserMULTIPOLYGON_SYMBOL:
		return &WalkThroughError{
			Type: ErrorTypeInvalidColumnTypeForDefaultValue,
			// Content comes from MySQL Error content.
			Content: fmt.Sprintf("BLOB, TEXT, GEOMETRY or JSON column `%s` can't have a default value", columnName),
		}
	}

	return nil
}

func (t *TableState) dropColumn(ctx *FinderContext, columnName string) *WalkThroughError {
	if ctx.CheckIntegrity {
		return t.completeTableDropColumn(columnName)
	}
	return t.incompleteTableDropColumn(columnName)
}

func (t *TableState) completeTableDropColumn(columnName string) *WalkThroughError {
	column, exists := t.columnSet[strings.ToLower(columnName)]
	if !exists {
		return NewColumnNotExistsError(t.name, columnName)
	}

	// Cannot drop all columns in a table using ALTER TABLE DROP COLUMN.
	if len(t.columnSet) == 1 {
		return &WalkThroughError{
			Type: ErrorTypeDropAllColumns,
			// Error content comes from MySQL error content.
			Content: fmt.Sprintf("Can't delete all columns with ALTER TABLE; use DROP TABLE %s instead", t.name),
		}
	}

	// If columns are dropped from a table, the columns are also removed from any index of which they are a part.
	for _, index := range t.indexSet {
		index.dropColumn(columnName)
		// If all columns that make up an index are dropped, the index is dropped as well.
		if len(index.expressionList) == 0 {
			delete(t.indexSet, strings.ToLower(index.name))
		}
	}

	// modify the column position
	for _, col := range t.columnSet {
		if *col.position > *column.position {
			*col.position--
		}
	}

	delete(t.columnSet, strings.ToLower(columnName))
	return nil
}

func (t *TableState) incompleteTableDropColumn(columnName string) *WalkThroughError {
	// If columns are dropped from a table, the columns are also removed from any index of which they are a part.
	for _, index := range t.indexSet {
		if len(index.expressionList) == 0 {
			continue
		}
		index.dropColumn(columnName)
		// If all columns that make up an index are dropped, the index is dropped as well.
		if len(index.expressionList) == 0 {
			delete(t.indexSet, strings.ToLower(index.name))
		}
	}

	delete(t.columnSet, strings.ToLower(columnName))
	return nil
}

func (idx *IndexState) dropColumn(columnName string) {
	if len(idx.expressionList) == 0 {
		return
	}
	var newKeyList []string
	for _, key := range idx.expressionList {
		if !strings.EqualFold(key, columnName) {
			newKeyList = append(newKeyList, key)
		}
	}

	idx.expressionList = newKeyList
}

func (t *TableState) dropIndex(ctx *FinderContext, indexName string) *WalkThroughError {
	if ctx.CheckIntegrity {
		if _, exists := t.indexSet[strings.ToLower(indexName)]; !exists {
			if strings.EqualFold(indexName, PrimaryKeyName) {
				return &WalkThroughError{
					Type:    ErrorTypePrimaryKeyNotExists,
					Content: fmt.Sprintf("Primary key does not exist in table `%s`", t.name),
				}
			}
			return NewIndexNotExistsError(t.name, indexName)
		}
	}

	delete(t.indexSet, strings.ToLower(indexName))
	return nil
}

//nolint:unused
func (t *TableState) renameColumn(ctx *FinderContext, oldName string, newName string) *WalkThroughError {
	if strings.EqualFold(oldName, newName) {
		return nil
	}

	column, exists := t.columnSet[strings.ToLower(oldName)]
	if !exists {
		if ctx.CheckIntegrity {
			return &WalkThroughError{
				Type:    ErrorTypeColumnNotExists,
				Content: fmt.Sprintf("Column `%s` does not exist in table `%s`", oldName, t.name),
			}
		}
		column = t.createIncompleteColumn(oldName)
	}

	if _, exists := t.columnSet[strings.ToLower(newName)]; exists {
		return &WalkThroughError{
			Type:    ErrorTypeColumnExists,
			Content: fmt.Sprintf("Column `%s` already exists in table `%s", newName, t.name),
		}
	}

	column.name = newName
	delete(t.columnSet, strings.ToLower(oldName))
	t.columnSet[strings.ToLower(newName)] = column

	t.renameColumnInIndexKey(oldName, newName)
	return nil
}

//nolint:unused
func (t *TableState) renameColumnInIndexKey(oldName string, newName string) {
	if strings.EqualFold(oldName, newName) {
		return
	}
	for _, index := range t.indexSet {
		for i, key := range index.expressionList {
			if strings.EqualFold(key, oldName) {
				index.expressionList[i] = newName
			}
		}
	}
}

// Helper methods for RENAME TABLE three-way logic
func (d *DatabaseState) mysqlTargetDatabase(renamePair mysql.IRenamePairContext) string {
	oldDatabaseName, _ := mysqlparser.NormalizeMySQLTableRef(renamePair.TableRef())
	if oldDatabaseName != "" && !d.isCurrentDatabase(oldDatabaseName) {
		return oldDatabaseName
	}
	newDatabaseName, _ := mysqlparser.NormalizeMySQLTableName(renamePair.TableName())
	return newDatabaseName
}

func (d *DatabaseState) mysqlMoveToOtherDatabase(renamePair mysql.IRenamePairContext) bool {
	oldDatabaseName, _ := mysqlparser.NormalizeMySQLTableRef(renamePair.TableRef())
	if oldDatabaseName != "" && !d.isCurrentDatabase(oldDatabaseName) {
		return false
	}
	newDatabaseName, _ := mysqlparser.NormalizeMySQLTableName(renamePair.TableName())
	return oldDatabaseName != newDatabaseName
}

func (d *DatabaseState) mysqlTheCurrentDatabase(renamePair mysql.IRenamePairContext) bool {
	newDatabaseName, _ := mysqlparser.NormalizeMySQLTableName(renamePair.TableName())
	if newDatabaseName != "" && !d.isCurrentDatabase(newDatabaseName) {
		return false
	}
	oldDatabaseName, _ := mysqlparser.NormalizeMySQLTableRef(renamePair.TableRef())
	if oldDatabaseName != "" && !d.isCurrentDatabase(oldDatabaseName) {
		return false
	}
	return true
}

// EnterAlterTable is called when production alterTable is entered.
func (l *mysqlListener) EnterAlterTable(ctx *mysql.AlterTableContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableRef() == nil {
		return
	}

	databaseName, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	table, err := l.databaseState.mysqlFindTableState(databaseName, tableName)
	if err != nil {
		l.err = err
		return
	}

	if ctx.AlterTableActions() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList() == nil {
		return
	}
	if ctx.AlterTableActions().AlterCommandList().AlterList() == nil {
		return
	}

	// Handle table-level options like ENGINE, COMMENT, COLLATE
	for _, option := range ctx.AlterTableActions().AlterCommandList().AlterList().AllCreateTableOptionsSpaceSeparated() {
		for _, op := range option.AllCreateTableOption() {
			switch {
			// engine.
			case op.ENGINE_SYMBOL() != nil:
				if op.EngineRef() == nil {
					continue
				}
				engine := op.EngineRef().GetText()
				table.engine = newStringPointer(engine)
			// table comment.
			case op.COMMENT_SYMBOL() != nil && op.TextStringLiteral() != nil:
				comment := mysqlparser.NormalizeMySQLTextStringLiteral(op.TextStringLiteral())
				table.comment = newStringPointer(comment)
			// table collation.
			case op.DefaultCollation() != nil && op.DefaultCollation().CollationName() != nil:
				collation := mysqlparser.NormalizeMySQLCollationName(op.DefaultCollation().CollationName())
				table.collation = newStringPointer(collation)
			default:
			}
		}
	}

	// Handle column and index operations
	for _, item := range ctx.AlterTableActions().AlterCommandList().AlterList().AllAlterListItem() {
		if item == nil {
			continue
		}

		switch {
		case item.ADD_SYMBOL() != nil:
			switch {
			// add single column.
			case item.Identifier() != nil && item.FieldDefinition() != nil:
				columnName := mysqlparser.NormalizeMySQLIdentifier(item.Identifier())
				if err := table.mysqlCreateColumn(l.databaseState.ctx, columnName, item.FieldDefinition(), positionFromPlaceContext(item.Place())); err != nil {
					l.err = err
					return
				}
			// add constraint (index).
			case item.TableConstraintDef() != nil:
				if err := table.mysqlCreateConstraint(l.databaseState.ctx, item.TableConstraintDef()); err != nil {
					l.err = err
					return
				}
			}
		// drop column or key.
		case item.DROP_SYMBOL() != nil && item.ALTER_SYMBOL() == nil:
			switch {
			// drop column.
			case item.ColumnInternalRef() != nil:
				columnName := mysqlparser.NormalizeMySQLColumnInternalRef(item.ColumnInternalRef())
				if err := table.dropColumn(l.databaseState.ctx, columnName); err != nil {
					l.err = err
					return
				}
				// drop primary key.
			case item.PRIMARY_SYMBOL() != nil && item.KEY_SYMBOL() != nil:
				if err := table.dropIndex(l.databaseState.ctx, PrimaryKeyName); err != nil {
					l.err = err
					return
				}
				// drop key/index.
			case item.KeyOrIndex() != nil && item.IndexRef() != nil:
				_, _, indexName := mysqlparser.NormalizeIndexRef(item.IndexRef())
				if err := table.dropIndex(l.databaseState.ctx, indexName); err != nil {
					l.err = err
					return
				}
			}
		}
	}
}

// EnterCreateIndex is called when production createIndex is entered.
func (l *mysqlListener) EnterCreateIndex(ctx *mysql.CreateIndexContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().TableRef() == nil {
		return
	}
	databaseName, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.CreateIndexTarget().TableRef())
	table, err := l.databaseState.mysqlFindTableState(databaseName, tableName)
	if err != nil {
		l.err = err
		return
	}

	unique := false
	isSpatial := false
	tp := "BTREE"

	if ctx.GetType_() == nil {
		return
	}
	switch ctx.GetType_().GetTokenType() {
	case mysql.MySQLParserFULLTEXT_SYMBOL:
		tp = FullTextName
	case mysql.MySQLParserSPATIAL_SYMBOL:
		isSpatial = true
		tp = SpatialName
	case mysql.MySQLParserINDEX_SYMBOL:
	}
	if ctx.UNIQUE_SYMBOL() != nil {
		unique = true
	}

	indexName := ""
	if ctx.IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexName())
	}
	if ctx.IndexNameAndType() != nil && ctx.IndexNameAndType().IndexName() != nil {
		indexName = mysqlparser.NormalizeIndexName(ctx.IndexNameAndType().IndexName())
	}

	if ctx.CreateIndexTarget() == nil || ctx.CreateIndexTarget().KeyListVariants() == nil {
		return
	}
	if err := table.mysqlValidateKeyListVariants(l.databaseState.ctx, ctx.CreateIndexTarget().KeyListVariants(), false /* primary */, isSpatial); err != nil {
		l.err = err
		return
	}

	columnList := mysqlparser.NormalizeKeyListVariants(ctx.CreateIndexTarget().KeyListVariants())
	if err := table.mysqlCreateIndex(indexName, columnList, unique, tp, mysql.NewEmptyTableConstraintDefContext(), ctx); err != nil {
		l.err = err
		return
	}
}

// EnterDropIndex is called when production dropIndex is entered.
func (l *mysqlListener) EnterDropIndex(ctx *mysql.DropIndexContext) {
	if !mysqlparser.IsTopMySQLRule(&ctx.BaseParserRuleContext) {
		return
	}
	if ctx.TableRef() == nil {
		return
	}
	databaseName, tableName := mysqlparser.NormalizeMySQLTableRef(ctx.TableRef())
	table, err := l.databaseState.mysqlFindTableState(databaseName, tableName)
	if err != nil {
		l.err = err
		return
	}

	if ctx.IndexRef() == nil {
		return
	}

	_, _, indexName := mysqlparser.NormalizeIndexRef(ctx.IndexRef())
	if err := table.dropIndex(l.databaseState.ctx, indexName); err != nil {
		l.err = err
	}
}

// Helper function for column position
func positionFromPlaceContext(place mysql.IPlaceContext) *mysqlColumnPosition {
	if place == nil {
		return &mysqlColumnPosition{tp: ColumnPositionNone}
	}
	if place.FIRST_SYMBOL() != nil {
		return &mysqlColumnPosition{tp: ColumnPositionFirst}
	}
	if place.AFTER_SYMBOL() != nil && place.Identifier() != nil {
		return &mysqlColumnPosition{
			tp:             ColumnPositionAfter,
			relativeColumn: mysqlparser.NormalizeMySQLIdentifier(place.Identifier()),
		}
	}
	return &mysqlColumnPosition{tp: ColumnPositionNone}
}
