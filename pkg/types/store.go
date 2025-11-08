package types

import (
	"encoding/json"
)

// Engine represents the database engine type
type Engine int32

const (
	Engine_ENGINE_UNSPECIFIED Engine = 0
	Engine_MYSQL              Engine = 1
	Engine_POSTGRES           Engine = 2
	Engine_TIDB               Engine = 3
	Engine_SNOWFLAKE          Engine = 4
	Engine_SQLITE             Engine = 5
	Engine_MONGODB            Engine = 6
	Engine_REDIS              Engine = 7
	Engine_ORACLE             Engine = 8
	Engine_SPANNER            Engine = 9
	Engine_MSSQL              Engine = 10
	Engine_REDSHIFT           Engine = 11
	Engine_MARIADB            Engine = 12
	Engine_OCEANBASE          Engine = 13
	Engine_DM                 Engine = 14
	Engine_RISINGWAVE         Engine = 15
	Engine_OCEANBASE_ORACLE   Engine = 16
	Engine_STARROCKS          Engine = 17
	Engine_DORIS              Engine = 18
	Engine_HIVE               Engine = 19
	Engine_ELASTICSEARCH      Engine = 20
	Engine_BIGQUERY           Engine = 21
	Engine_CLICKHOUSE         Engine = 22
	Engine_DATABRICKS         Engine = 23
	Engine_DYNAMODB           Engine = 24
)

func (e Engine) String() string {
	switch e {
	case Engine_ENGINE_UNSPECIFIED:
		return "ENGINE_UNSPECIFIED"
	case Engine_MYSQL:
		return "MYSQL"
	case Engine_POSTGRES:
		return "POSTGRES"
	case Engine_TIDB:
		return "TIDB"
	case Engine_SNOWFLAKE:
		return "SNOWFLAKE"
	case Engine_SQLITE:
		return "SQLITE"
	case Engine_MONGODB:
		return "MONGODB"
	case Engine_REDIS:
		return "REDIS"
	case Engine_ORACLE:
		return "ORACLE"
	case Engine_SPANNER:
		return "SPANNER"
	case Engine_MSSQL:
		return "MSSQL"
	case Engine_REDSHIFT:
		return "REDSHIFT"
	case Engine_MARIADB:
		return "MARIADB"
	case Engine_OCEANBASE:
		return "OCEANBASE"
	case Engine_DM:
		return "DM"
	case Engine_RISINGWAVE:
		return "RISINGWAVE"
	case Engine_OCEANBASE_ORACLE:
		return "OCEANBASE_ORACLE"
	case Engine_STARROCKS:
		return "STARROCKS"
	case Engine_DORIS:
		return "DORIS"
	case Engine_HIVE:
		return "HIVE"
	case Engine_ELASTICSEARCH:
		return "ELASTICSEARCH"
	case Engine_BIGQUERY:
		return "BIGQUERY"
	case Engine_CLICKHOUSE:
		return "CLICKHOUSE"
	case Engine_DATABRICKS:
		return "DATABRICKS"
	case Engine_DYNAMODB:
		return "DYNAMODB"
	default:
		return "UNKNOWN"
	}
}

// UnmarshalYAML implements yaml.Unmarshaler for Engine
func (e *Engine) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	switch s {
	case "MYSQL":
		*e = Engine_MYSQL
	case "POSTGRES", "POSTGRESQL":
		*e = Engine_POSTGRES
	case "TIDB":
		*e = Engine_TIDB
	case "SNOWFLAKE":
		*e = Engine_SNOWFLAKE
	case "SQLITE":
		*e = Engine_SQLITE
	case "MONGODB":
		*e = Engine_MONGODB
	case "REDIS":
		*e = Engine_REDIS
	case "ORACLE":
		*e = Engine_ORACLE
	case "SPANNER":
		*e = Engine_SPANNER
	case "MSSQL", "SQLSERVER":
		*e = Engine_MSSQL
	case "REDSHIFT":
		*e = Engine_REDSHIFT
	case "MARIADB":
		*e = Engine_MARIADB
	case "OCEANBASE":
		*e = Engine_OCEANBASE
	case "DM":
		*e = Engine_DM
	case "RISINGWAVE":
		*e = Engine_RISINGWAVE
	case "OCEANBASE_ORACLE":
		*e = Engine_OCEANBASE_ORACLE
	case "STARROCKS":
		*e = Engine_STARROCKS
	case "DORIS":
		*e = Engine_DORIS
	case "HIVE":
		*e = Engine_HIVE
	case "ELASTICSEARCH":
		*e = Engine_ELASTICSEARCH
	case "BIGQUERY":
		*e = Engine_BIGQUERY
	case "CLICKHOUSE":
		*e = Engine_CLICKHOUSE
	case "DATABRICKS":
		*e = Engine_DATABRICKS
	case "DYNAMODB":
		*e = Engine_DYNAMODB
	default:
		*e = Engine_ENGINE_UNSPECIFIED
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler for Engine
func (e *Engine) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "MYSQL":
		*e = Engine_MYSQL
	case "POSTGRES", "POSTGRESQL":
		*e = Engine_POSTGRES
	case "TIDB":
		*e = Engine_TIDB
	case "SNOWFLAKE":
		*e = Engine_SNOWFLAKE
	case "SQLITE":
		*e = Engine_SQLITE
	case "MONGODB":
		*e = Engine_MONGODB
	case "REDIS":
		*e = Engine_REDIS
	case "ORACLE":
		*e = Engine_ORACLE
	case "SPANNER":
		*e = Engine_SPANNER
	case "MSSQL", "SQLSERVER":
		*e = Engine_MSSQL
	case "REDSHIFT":
		*e = Engine_REDSHIFT
	case "MARIADB":
		*e = Engine_MARIADB
	case "OCEANBASE":
		*e = Engine_OCEANBASE
	case "DM":
		*e = Engine_DM
	case "RISINGWAVE":
		*e = Engine_RISINGWAVE
	case "OCEANBASE_ORACLE":
		*e = Engine_OCEANBASE_ORACLE
	case "STARROCKS":
		*e = Engine_STARROCKS
	case "DORIS":
		*e = Engine_DORIS
	case "HIVE":
		*e = Engine_HIVE
	case "ELASTICSEARCH":
		*e = Engine_ELASTICSEARCH
	case "BIGQUERY":
		*e = Engine_BIGQUERY
	case "CLICKHOUSE":
		*e = Engine_CLICKHOUSE
	case "DATABRICKS":
		*e = Engine_DATABRICKS
	case "DYNAMODB":
		*e = Engine_DYNAMODB
	default:
		*e = Engine_ENGINE_UNSPECIFIED
	}
	return nil
}

// SQLReviewRuleLevel represents the severity level of a rule
type SQLReviewRuleLevel int32

const (
	SQLReviewRuleLevel_LEVEL_UNSPECIFIED SQLReviewRuleLevel = 0
	SQLReviewRuleLevel_ERROR             SQLReviewRuleLevel = 1
	SQLReviewRuleLevel_WARNING           SQLReviewRuleLevel = 2
	SQLReviewRuleLevel_DISABLED          SQLReviewRuleLevel = 3
)

// UnmarshalYAML implements yaml.Unmarshaler for SQLReviewRuleLevel
func (l *SQLReviewRuleLevel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	switch s {
	case "ERROR":
		*l = SQLReviewRuleLevel_ERROR
	case "WARNING":
		*l = SQLReviewRuleLevel_WARNING
	case "DISABLED":
		*l = SQLReviewRuleLevel_DISABLED
	default:
		*l = SQLReviewRuleLevel_LEVEL_UNSPECIFIED
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler for SQLReviewRuleLevel
func (l *SQLReviewRuleLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "ERROR":
		*l = SQLReviewRuleLevel_ERROR
	case "WARNING":
		*l = SQLReviewRuleLevel_WARNING
	case "DISABLED":
		*l = SQLReviewRuleLevel_DISABLED
	default:
		*l = SQLReviewRuleLevel_LEVEL_UNSPECIFIED
	}
	return nil
}

// Advice_Status represents the status of an advice
type Advice_Status int32

const (
	Advice_STATUS_UNSPECIFIED Advice_Status = 0
	Advice_SUCCESS            Advice_Status = 1
	Advice_WARNING            Advice_Status = 2
	Advice_ERROR              Advice_Status = 3
)

// PlanCheckRunConfig_ChangeDatabaseType represents the type of database change
type PlanCheckRunConfig_ChangeDatabaseType int32

const (
	PlanCheckRunConfig_CHANGE_DATABASE_TYPE_UNSPECIFIED PlanCheckRunConfig_ChangeDatabaseType = 0
	PlanCheckRunConfig_DDL                              PlanCheckRunConfig_ChangeDatabaseType = 1
	PlanCheckRunConfig_DML                              PlanCheckRunConfig_ChangeDatabaseType = 2
	PlanCheckRunConfig_SDL                              PlanCheckRunConfig_ChangeDatabaseType = 3
)

// SQLReviewRule represents a SQL review rule
type SQLReviewRule struct {
	Type    string                 `json:"type"              yaml:"type"`
	Level   SQLReviewRuleLevel     `json:"level"             yaml:"level"`
	Payload map[string]interface{} `json:"payload,omitempty" yaml:"payload,omitempty"`
	Engine  Engine                 `json:"engine"            yaml:"engine"`
	Comment string                 `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// Advice represents a piece of advice from the advisor
type Advice struct {
	Status        Advice_Status `json:"status"`
	Code          int32         `json:"code"`
	Title         string        `json:"title"`
	Content       string        `json:"content"`
	StartPosition *Position     `json:"startPosition"`
}

// Error code constants for different rule violations
const (
	Internal = 1 // Matches advisor.Internal

	// 101 ~ 199 compatibility error code.
	CompatibilityDropDatabase  = 101 // Matches advisor.CompatibilityDropDatabase
	CompatibilityRenameTable   = 102 // Matches advisor.CompatibilityRenameTable
	CompatibilityDropTable     = 103 // Matches advisor.CompatibilityDropTable
	CompatibilityRenameColumn  = 104 // Matches advisor.CompatibilityRenameColumn
	CompatibilityDropColumn    = 105 // Matches advisor.CompatibilityDropColumn
	CompatibilityAddPrimaryKey = 106 // Matches advisor.CompatibilityAddPrimaryKey
	CompatibilityAddUniqueKey  = 107 // Matches advisor.CompatibilityAddUniqueKey
	CompatibilityAddForeignKey = 108 // Matches advisor.CompatibilityAddForeignKey
	CompatibilityAddCheck      = 109 // Matches advisor.CompatibilityAddCheck
	CompatibilityAlterCheck    = 110 // Matches advisor.CompatibilityAlterCheck
	CompatibilityAlterColumn   = 111 // Matches advisor.CompatibilityAlterColumn
	CompatibilityDropSchema    = 112 // Matches advisor.CompatibilityDropSchema

	StatementSyntaxError                        = 201  // Matches advisor.StatementSyntaxErrorCode
	StatementDisallowCommit                     = 206  // Matches advisor.StatementDisallowCommit
	StatementSelectAll                          = 203  // Matches advisor.StatementSelectAll
	StatementDMLDryRunFailed                    = 208  // Matches advisor.StatementDMLDryRunFailed
	StatementLeadingWildcardLike                = 204  // Matches advisor.StatementLeadingWildcardLike
	StatementRedundantAlterTable                = 207  // Matches advisor.StatementRedundantAlterTable
	StatementDisallowMixDDLDML                  = 227  // Matches advisor.StatementDisallowMixDDLDML
	StatementAffectedRowExceedsLimit            = 209  // Matches advisor.StatementAffectedRowExceedsLimit
	DisabledCharset                             = 1001 // Matches advisor.DisabledCharset
	DisabledCollation                           = 1201 // Matches advisor.DisabledCollation
	StatementNoAlgorithmOption                  = 236  // Matches advisor.StatementNoAlgorithmOption
	StatementNoLockOption                       = 237  // Matches advisor.StatementNoLockOption
	StatementWhereMaximumLogicalOperatorCount   = 225  // Matches advisor.StatementWhereMaximumLogicalOperatorCount
	StatementJoinColumnAttrsNotMatch            = 226  // Matches advisor.StatementJoinColumnAttrsNotMatch
	StatementAddColumnWithPosition              = 231  // Matches advisor.StatementAddColumnWithPosition
	InsertTooManyRows                           = 1101 // Matches advisor.InsertTooManyRows
	UpdateUseLimit                              = 1102 // Matches advisor.UpdateUseLimit
	InsertUseLimit                              = 1103 // Matches advisor.InsertUseLimit
	UpdateUseOrderBy                            = 1104 // Matches advisor.UpdateUseOrderBy
	DeleteUseOrderBy                            = 1105 // Matches advisor.DeleteUseOrderBy
	DeleteUseLimit                              = 1106 // Matches advisor.DeleteUseLimit
	InsertNotSpecifyColumn                      = 1107 // Matches advisor.InsertNotSpecifyColumn
	InsertUseOrderByRand                        = 1108 // Matches advisor.InsertUseOrderByRand
	StatementNoWhere                            = 202  // Matches advisor.StatementNoWhere
	StatementWhereNoWhere                       = 102
	NamingTableConvention                       = 301  // Matches advisor.NamingTableConventionMismatch
	NamingColumnConvention                      = 302  // Matches advisor.NamingColumnConventionMismatch
	NamingIndexConventionMismatch               = 303  // Matches advisor.NamingIndexConventionMismatch
	TableRequirePK                              = 601  // Matches advisor.TableNoPK
	CreateTablePartition                        = 608  // Matches advisor.CreateTablePartition
	CreateTableTrigger                          = 610  // Matches advisor.CreateTableTrigger
	DisallowSetCharset                          = 612  // Matches advisor.DisallowSetCharset
	DuplicateIndexInTable                       = 815  // Matches advisor.DuplicateIndexInTable
	TableDropNamingConventionMismatch           = 603  // Matches advisor.TableDropNamingConventionMismatch
	DisallowCreateEvent                         = 1501 // Matches advisor.DisallowCreateEvent
	DisallowCreateFunction                      = 1701 // Matches advisor.DisallowCreateFunction
	DisabledFunction                            = 1702 // Matches advisor.DisabledFunction
	DisallowCreateProcedure                     = 1401 // Matches advisor.DisallowCreateProcedure
	DisallowCreateView                          = 1601 // Matches advisor.DisallowCreateView
	ColumnCannotNull                            = 402  // Matches advisor.ColumnCannotNull
	ColumnRequireComment                        = 1032 // Matches advisor.NoColumnComment
	ColumnCommentTooLong                        = 1301 // Matches advisor.CommentTooLong
	ColumnRequireDefault                        = 420  // Matches advisor.NoDefault
	TableRequireComment                         = 1032 // Matches advisor.CommentEmpty
	TableCommentTooLong                         = 1301 // Matches advisor.CommentTooLong
	ColumnRequired                              = 401  // Matches advisor.NoRequiredColumn
	ColumnNotNullNoDefault                      = 404  // Matches advisor.NotNullColumnWithNoDefault
	IndexDuplicateColumn                        = 812  // Matches advisor.DuplicateColumnInIndex
	CharLengthExceedsLimit                      = 415  // Matches advisor.CharLengthExceedsLimit
	AutoIncrementInitialValueNotMatch           = 416  // Matches advisor.AutoIncrementColumnInitialValueNotMatch
	AutoIncrementColumnNotInteger               = 410  // Matches advisor.AutoIncrementColumnNotInteger
	AutoIncrementColumnSigned                   = 417  // Matches advisor.AutoIncrementColumnSigned
	SetColumnCharset                            = 414  // Matches advisor.SetColumnCharset
	DefaultCurrentTimeColumnCountExceedsLimit   = 418  // Matches advisor.DefaultCurrentTimeColumnCountExceedsLimit
	OnUpdateCurrentTimeColumnCountExceedsLimit  = 419  // Matches advisor.OnUpdateCurrentTimeColumnCountExceedsLimit
	DropIndexColumn                             = 424  // Matches advisor.DropIndexColumn
	DropColumn                                  = 425  // Matches advisor.DropColumn
	NotInnoDBEngine                             = 501  // Matches advisor.NotInnoDBEngine
	TotalTextLengthExceedsLimit                 = 611  // Matches advisor.TotalTextLengthExceedsLimit
	TableDisallowDDL                            = 613  // Matches advisor.TableDisallowDDL
	TableDisallowDML                            = 614  // Matches advisor.TableDisallowDML
	NoCharset                                   = 616  // Matches advisor.NoCharset
	NoCollation                                 = 617  // Matches advisor.NoCollation
	IndexKeyNumberExceedsLimit                  = 802  // Matches advisor.IndexKeyNumberExceedsLimit
	TableHasFK                                  = 602  // Matches advisor.TableHasFK
	UseChangeColumnStatement                    = 406  // Matches advisor.UseChangeColumnStatement
	ChangeColumnOrder                           = 407  // Matches advisor.ChangeColumnOrder
	ChangeColumnType                            = 403  // Matches advisor.ChangeColumnType
	VarcharLengthExceedsLimit                   = 422  // Matches advisor.VarcharLengthExceedsLimit
	DisabledColumnType                          = 411  // Matches advisor.DisabledColumnType
	DatabaseNotEmpty                            = 701  // Matches advisor.DatabaseNotEmpty
	NotCurrentDatabase                          = 502  // Matches advisor.NotCurrentDatabase
	IndexPKType                                 = 803  // Matches advisor.IndexPKType
	IndexCountExceedsLimit                      = 813  // Matches advisor.IndexCountExceedsLimit
	TableExceedLimitSize                        = 615  // Matches advisor.TableExceedLimitSize
	IndexTypeNotAllowed                         = 816  // Matches advisor.IndexTypeNotAllowed
	IndexTypeNoBlob                             = 804  // Matches advisor.IndexTypeNoBlob
	NamingUKConventionMismatch                  = 304  // Matches advisor.NamingUKConventionMismatch
	NamingFKConventionMismatch                  = 305  // Matches advisor.NamingFKConventionMismatch
	NamingPKConventionMismatch                  = 306  // Matches advisor.NamingPKConventionMismatch
	NamingAutoIncrementColumnConventionMismatch = 307  // Matches advisor.NamingAutoIncrementColumnConventionMismatch
	NameIsKeywordIdentifier                     = 308  // Matches advisor.NameIsKeywordIdentifier
	StatementCheckSelectFullTableScanFailed     = 214  // Matches advisor.StatementCheckSelectFullTableScanFailed
	StatementHasTableFullScan                   = 215  // Matches advisor.StatementHasTableFullScan
	StatementHasUsingFilesort                   = 219  // Matches advisor.StatementHasUsingFilesort
	StatementHasUsingTemporary                  = 220  // Matches advisor.StatementHasUsingTemporary
	StatementWhereNoEqualNull                   = 221  // Matches advisor.StatementWhereNoEqualNull
	StatementExceedMaximumLimitValue            = 222  // Matches advisor.StatementExceedMaximumLimitValue
	StatementMaximumJoinTableCount              = 223  // Matches advisor.StatementMaximumJoinTableCount
	StatementUnwantedQueryPlanLevel             = 224  // Matches advisor.StatementUnwantedQueryPlanLevel
	StatementMaximumStatementsInTransaction     = 228  // New error code for max statements in transaction
	StatementNoMaxExecutionTime                 = 235  // Matches advisor.StatementNoMaxExecutionTime
	DatabaseNotExists                           = 704  // Matches advisor.DatabaseNotExists
	AdviseOnlineMigration                       = 1801 // Matches advisor.AdviseOnlineMigration
	AdviseOnlineMigrationForStatement           = 1802 // Matches advisor.AdviseOnlineMigrationForStatement
	AdviseNoOnlineMigration                     = 1803 // Matches advisor.AdviseNoOnlineMigration
)

// Position represents a position in the source code
type Position struct {
	Line   int32 `json:"line"`
	Column int32 `json:"column"`
}

// DatabaseSchemaMetadata represents database schema metadata
type DatabaseSchemaMetadata struct {
	Name           string               `json:"name"`
	Schemas        []*SchemaMetadata    `json:"schemas"`
	CharacterSet   string               `json:"characterSet"`
	Collation      string               `json:"collation"`
	Extensions     []*ExtensionMetadata `json:"extensions"`
	SchemaConfigs  []*SchemaConfig      `json:"schemaConfigs"`
	DatabaseConfig *DatabaseConfig      `json:"databaseConfig"`
}

// SchemaMetadata represents schema metadata
type SchemaMetadata struct {
	Name           string                   `json:"name"`
	Tables         []*TableMetadata         `json:"tables"`
	ExternalTables []*ExternalTableMetadata `json:"externalTables"`
	Views          []*ViewMetadata          `json:"views"`
	Functions      []*FunctionMetadata      `json:"functions"`
	Procedures     []*ProcedureMetadata     `json:"procedures"`
	Streams        []*StreamMetadata        `json:"streams"`
	Tasks          []*TaskMetadata          `json:"tasks"`
}

// TableMetadata represents table metadata
type TableMetadata struct {
	Name             string                     `json:"name"`
	Columns          []*ColumnMetadata          `json:"columns"`
	Indexes          []*IndexMetadata           `json:"indexes"`
	Engine           string                     `json:"engine"`
	Collation        string                     `json:"collation"`
	RowCount         int64                      `json:"rowCount"`
	DataSize         int64                      `json:"dataSize"`
	IndexSize        int64                      `json:"indexSize"`
	DataFree         int64                      `json:"dataFree"`
	CreateOptions    string                     `json:"createOptions"`
	Comment          string                     `json:"comment"`
	UserComment      string                     `json:"userComment"`
	ForeignKeys      []*ForeignKeyMetadata      `json:"foreignKeys"`
	Partitions       []*TablePartitionMetadata  `json:"partitions"`
	CheckConstraints []*CheckConstraintMetadata `json:"checkConstraints"`
}

// ColumnMetadata represents column metadata
type ColumnMetadata struct {
	Name              string `json:"name"`
	Position          int32  `json:"position"`
	HasDefault        bool   `json:"hasDefault"`
	DefaultNull       bool   `json:"defaultNull"`
	DefaultString     string `json:"defaultString"`
	DefaultExpression string `json:"defaultExpression"`
	OnUpdate          string `json:"onUpdate"`
	Nullable          bool   `json:"nullable"`
	Type              string `json:"type"`
	CharacterSet      string `json:"characterSet"`
	Collation         string `json:"collation"`
	Comment           string `json:"comment"`
	UserComment       string `json:"userComment"`
	Effective         bool   `json:"effective"`
	Classification    string `json:"classification"`
}

// IndexMetadata represents index metadata
type IndexMetadata struct {
	Name        string   `json:"name"`
	Expressions []string `json:"expressions"`
	Type        string   `json:"type"`
	Unique      bool     `json:"unique"`
	Primary     bool     `json:"primary"`
	Visible     bool     `json:"visible"`
	Comment     string   `json:"comment"`
	Definition  string   `json:"definition"`
}

// ViewMetadata represents view metadata
type ViewMetadata struct {
	Name             string             `json:"name"`
	Definition       string             `json:"definition"`
	Comment          string             `json:"comment"`
	DependentColumns []*DependentColumn `json:"dependentColumns"`
}

// FunctionMetadata represents function metadata
type FunctionMetadata struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// ProcedureMetadata represents procedure metadata
type ProcedureMetadata struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// StreamMetadata represents stream metadata
type StreamMetadata struct {
	Name       string              `json:"name"`
	Definition string              `json:"definition"`
	Type       StreamMetadata_Type `json:"type"`
	Mode       StreamMetadata_Mode `json:"mode"`
	TableName  string              `json:"tableName"`
	Owner      string              `json:"owner"`
	Comment    string              `json:"comment"`
}

type StreamMetadata_Type int32

const (
	StreamMetadata_TYPE_UNSPECIFIED StreamMetadata_Type = 0
	StreamMetadata_DELTA            StreamMetadata_Type = 1
)

type StreamMetadata_Mode int32

const (
	StreamMetadata_MODE_UNSPECIFIED StreamMetadata_Mode = 0
	StreamMetadata_DEFAULT          StreamMetadata_Mode = 1
	StreamMetadata_APPEND_ONLY      StreamMetadata_Mode = 2
	StreamMetadata_INSERT_ONLY      StreamMetadata_Mode = 3
)

// TaskMetadata represents task metadata
type TaskMetadata struct {
	Name         string             `json:"name"`
	Id           string             `json:"id"`
	Owner        string             `json:"owner"`
	Comment      string             `json:"comment"`
	Warehouse    string             `json:"warehouse"`
	Schedule     string             `json:"schedule"`
	Predecessors []string           `json:"predecessors"`
	State        TaskMetadata_State `json:"state"`
	Condition    string             `json:"condition"`
	Definition   string             `json:"definition"`
}

type TaskMetadata_State int32

const (
	TaskMetadata_STATE_UNSPECIFIED TaskMetadata_State = 0
	TaskMetadata_STARTED           TaskMetadata_State = 1
	TaskMetadata_SUSPENDED         TaskMetadata_State = 2
)

// ExternalTableMetadata represents external table metadata
type ExternalTableMetadata struct {
	Name    string            `json:"name"`
	Columns []*ColumnMetadata `json:"columns"`
}

// ForeignKeyMetadata represents foreign key metadata
type ForeignKeyMetadata struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedSchema  string   `json:"referencedSchema"`
	ReferencedTable   string   `json:"referencedTable"`
	ReferencedColumns []string `json:"referencedColumns"`
	OnDelete          string   `json:"onDelete"`
	OnUpdate          string   `json:"onUpdate"`
	MatchType         string   `json:"matchType"`
}

// TablePartitionMetadata represents table partition metadata
type TablePartitionMetadata struct {
	Name          string                      `json:"name"`
	Type          TablePartitionMetadata_Type `json:"type"`
	Expression    string                      `json:"expression"`
	Value         string                      `json:"value"`
	UseDefault    string                      `json:"useDefault"`
	Subpartitions []*TablePartitionMetadata   `json:"subpartitions"`
}

type TablePartitionMetadata_Type int32

const (
	TablePartitionMetadata_TYPE_UNSPECIFIED TablePartitionMetadata_Type = 0
	TablePartitionMetadata_RANGE            TablePartitionMetadata_Type = 1
	TablePartitionMetadata_LIST             TablePartitionMetadata_Type = 2
	TablePartitionMetadata_HASH             TablePartitionMetadata_Type = 3
)

// CheckConstraintMetadata represents check constraint metadata
type CheckConstraintMetadata struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

// DependentColumn represents a dependent column
type DependentColumn struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Column string `json:"column"`
}

// ExtensionMetadata represents extension metadata
type ExtensionMetadata struct {
	Name        string `json:"name"`
	Schema      string `json:"schema"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// SchemaConfig represents schema configuration
type SchemaConfig struct {
	Name string `json:"name"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Name string `json:"name"`
}

// DataClassificationSetting_DataClassificationConfig represents data classification config
type DataClassificationSetting_DataClassificationConfig struct {
	Title          string                                                                            `json:"title"`
	Levels         []*DataClassificationSetting_DataClassificationConfig_Level                       `json:"levels"`
	Classification map[string]*DataClassificationSetting_DataClassificationConfig_DataClassification `json:"classification"`
}

// DataClassificationSetting_DataClassificationConfig_Level represents classification level
type DataClassificationSetting_DataClassificationConfig_Level struct {
	Id          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// DataClassificationSetting_DataClassificationConfig_DataClassification represents data classification
type DataClassificationSetting_DataClassificationConfig_DataClassification struct {
	Id          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	LevelId     string `json:"levelId"`
}
