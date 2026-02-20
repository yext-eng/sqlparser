/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

%{
package sqlparser

import "strings"

func setParseTree(yylex interface{}, stmt Statement) {
  yylex.(*Tokenizer).ParseTree = stmt
}

func setAllowComments(yylex interface{}, allow bool) {
  yylex.(*Tokenizer).AllowComments = allow
}

func setDDL(yylex interface{}, ddl *DDL) {
  yylex.(*Tokenizer).partialDDL = ddl
}

func incNesting(yylex interface{}) bool {
  yylex.(*Tokenizer).nesting++
  if yylex.(*Tokenizer).nesting == 200 {
    return true
  }
  return false
}

func decNesting(yylex interface{}) {
  yylex.(*Tokenizer).nesting--
}

// forceEOF forces the lexer to end prematurely. Not all SQL statements
// are supported by the Parser, thus calling forceEOF will make the lexer
// return EOF early.
func forceEOF(yylex interface{}) {
  yylex.(*Tokenizer).ForceEOF = true
}

func isAtSign(tok []byte) bool {
  return len(tok) == 1 && tok[0] == '@'
}

func rejectDeprecatedSetVar(yylex interface{}, name ColIdent) bool {
  lowered := name.Lowered()
  if lowered == "tx_isolation" || lowered == "tx_read_only" {
    yylex.(*Tokenizer).Error("deprecated system variable in set statement")
    return true
  }
  return false
}

func isAllowedGenericShowType(id []byte) bool {
  switch NewColIdent(string(id)).Lowered() {
  case "binlog", "collation", "engine", "engines", "errors", "events",
    "function", "grants", "indexes", "master", "open", "plugins",
    "privileges", "profile", "profiles", "relaylog", "slave", "storage",
    "triggers", "warnings":
    return true
  default:
    return false
  }
}

type addConstraintObject struct {
  Constraint *ConstraintDefinition
  Index      *IndexDefinition
}

type columnAttrSet struct {
  NotNullSet       bool
  NotNull          BoolVal
  Default          Expr
  OnUpdate         *SQLVal
  AutoIncrementSet bool
  AutoIncrement    BoolVal
  VisibilitySet    bool
  Visibility       string
  KeyOptSet        bool
  KeyOpt           ColumnKeyOption
  Comment          *SQLVal
  Reference        *ReferenceDefinition
}

func mergeColumnAttrSet(yylex interface{}, dst *columnAttrSet, src *columnAttrSet) bool {
  if src.NotNullSet {
    if dst.NotNullSet {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.NotNullSet = true
    dst.NotNull = src.NotNull
  }
  if src.Default != nil {
    if dst.Default != nil {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.Default = src.Default
  }
  if src.OnUpdate != nil {
    if dst.OnUpdate != nil {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.OnUpdate = src.OnUpdate
  }
  if src.AutoIncrementSet {
    if dst.AutoIncrementSet {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.AutoIncrementSet = true
    dst.AutoIncrement = src.AutoIncrement
  }
  if src.VisibilitySet {
    if dst.VisibilitySet {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.VisibilitySet = true
    dst.Visibility = src.Visibility
  }
  if src.KeyOptSet {
    if dst.KeyOptSet {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.KeyOptSet = true
    dst.KeyOpt = src.KeyOpt
  }
  if src.Comment != nil {
    if dst.Comment != nil {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.Comment = src.Comment
  }
  if src.Reference != nil {
    if dst.Reference != nil {
      yylex.(*Tokenizer).Error("syntax error")
      return true
    }
    dst.Reference = src.Reference
  }
  return false
}

func applyColumnAttrSet(colType ColumnType, attrs *columnAttrSet) ColumnType {
  if attrs == nil {
    return colType
  }
  if attrs.NotNullSet {
    colType.NotNull = attrs.NotNull
  }
  if attrs.Default != nil {
    colType.Default = attrs.Default
  }
  if attrs.OnUpdate != nil {
    colType.OnUpdate = attrs.OnUpdate
  }
  if attrs.AutoIncrementSet {
    colType.Autoincrement = attrs.AutoIncrement
  }
  if attrs.VisibilitySet {
    colType.Visibility = attrs.Visibility
  }
  if attrs.KeyOptSet {
    colType.KeyOpt = attrs.KeyOpt
  }
  if attrs.Comment != nil {
    colType.Comment = attrs.Comment
  }
  if attrs.Reference != nil {
    colType.Reference = attrs.Reference
  }
  return colType
}

func normalizeDefaultExpr(expr Expr) Expr {
  if expr == nil {
    return nil
  }
  if fn, ok := expr.(*FuncExpr); ok && fn != nil {
    lowered := fn.Name.Lowered()
    if lowered == "current_timestamp" && fn.Qualifier.IsEmpty() && !fn.Distinct && fn.Exprs == nil && fn.Over == nil {
      return NewValArg([]byte("current_timestamp"))
    }
    return expr
  }
  paren, ok := expr.(*ParenExpr)
  if !ok || paren == nil {
    return expr
  }
  switch inner := paren.Expr.(type) {
  case *SQLVal, *NullVal, BoolVal:
    return inner
  case *FuncExpr:
    lowered := inner.Name.Lowered()
    if lowered == "current_timestamp" && inner.Qualifier.IsEmpty() && !inner.Distinct && len(inner.Exprs) == 0 && inner.Over == nil {
      return NewValArg([]byte("current_timestamp"))
    }
    return expr
  default:
    return expr
  }
}

%}

%union {
  empty         struct{}
  statement     Statement
  selStmt       SelectStatement
  ddl           *DDL
  ins           *Insert
  byt           byte
  bytes         []byte
  bytes2        [][]byte
  str           string
  strs          []string
  selectExprs   SelectExprs
  selectExpr    SelectExpr
  columns       Columns
  partitions    Partitions
  colName       *ColName
  tableExprs    TableExprs
  tableExpr     TableExpr
  joinCondition JoinCondition
  tableName     TableName
  tableNames    TableNames
  indexHints    *IndexHints
  expr          Expr
  exprs         Exprs
  boolVal       BoolVal
  colTuple      ColTuple
  values        Values
  valTuple      ValTuple
  subquery      *Subquery
  whens         []*When
  when          *When
  orderBy       OrderBy
  order         *Order
  limit         *Limit
  updateExprs   UpdateExprs
  setExprs      SetExprs
  updateExpr    *UpdateExpr
  setExpr       *SetExpr
  colIdent      ColIdent
  tableIdent    TableIdent
  convertType   *ConvertType
  aliasedTableName *AliasedTableExpr
  TableSpec  *TableSpec
  columnType    ColumnType
  colKeyOpt     ColumnKeyOption
  optVal        *SQLVal
  LengthScaleOption LengthScaleOption
  columnDefinition *ColumnDefinition
  referenceDefinition *ReferenceDefinition
  columnAttrs *columnAttrSet
  indexDefinition *IndexDefinition
  constraintDefinition *ConstraintDefinition
  addConstraintObject *addConstraintObject
  indexInfo     *IndexInfo
  indexOption   *IndexOption
  indexOptions  []*IndexOption
  indexColumn   *IndexColumn
  indexColumns  []*IndexColumn
  partDefs      []*PartitionDefinition
  partDef       *PartitionDefinition
  partSpec      *PartitionSpec
  showFilter    *ShowFilter
  privilege     Privilege
  privileges    Privileges
  privilegeObject *PrivilegeObject
  accountName   *AccountName
  accountNames  AccountNames
  with          *With
  commonTableExpr *CommonTableExpr
  commonTableExprs CommonTableExprs
  jsonTableExpr *JSONTableExpr
  jsonTableColumn JSONTableColumn
  jsonTableColumns JSONTableColumns
  windows       WindowDefinitions
  window        *WindowDefinition
  windowSpec    *WindowSpec
  overClause    *OverClause
  frame         *WindowFrame
  frameBound    *WindowFrameBound
}

%token LEX_ERROR
%left <bytes> UNION EXCEPT
%left <bytes> INTERSECT
%token <bytes> SELECT INSERT UPDATE DELETE FROM WHERE GROUP HAVING ORDER BY LIMIT OFFSET FOR
%token <bytes> ALL DISTINCT AS EXISTS ASC DESC INTO DUPLICATE KEY DEFAULT SET LOCK KEYS
%token <bytes> VALUES LAST_INSERT_ID
%token <bytes> VALUE SHARE MODE NOWAIT SKIP LOCKED
%token <bytes> SQL_NO_CACHE SQL_CACHE
%left <bytes> JOIN STRAIGHT_JOIN LEFT RIGHT INNER OUTER CROSS NATURAL USE FORCE
%left <bytes> ON USING
%token <empty> '(' ',' ')'
%token <bytes> ID HEX STRING INTEGRAL FLOAT HEXNUM VALUE_ARG COMMENT COMMENT_KEYWORD BIT_LITERAL
%token <bytes> NULL TRUE FALSE OF

// Precedence dictated by mysql. But the vitess grammar is simplified.
// Some of these operators don't conflict in our situation. Nevertheless,
// it's better to have these listed in the correct order. Also, we don't
// support all operators yet.
%left <bytes> OR
%left <bytes> AND
%right <bytes> NOT '!'
%left <bytes> BETWEEN CASE WHEN THEN ELSE END
%left <bytes> '=' '<' '>' LE GE NE NULL_SAFE_EQUAL IS LIKE REGEXP MEMBER IN
%left <bytes> '|'
%left <bytes> '&'
%left <bytes> SHIFT_LEFT SHIFT_RIGHT
%left <bytes> '+' '-'
%left <bytes> '*' '/' DIV '%' MOD
%left <bytes> '^'
%right <bytes> '~' UNARY
%left <bytes> COLLATE
%right <bytes> BINARY UNDERSCORE_BINARY
%right <bytes> INTERVAL
%nonassoc <bytes> '.'

// There is no need to define precedence for the JSON
// operators because the syntax is restricted enough that
// they don't cause conflicts.
%token <empty> JSON_EXTRACT_OP JSON_UNQUOTE_EXTRACT_OP

// DDL Tokens
%token <bytes> CREATE ALTER DROP RENAME ANALYZE ADD AFTER ALGORITHM FIRST
%token <bytes> SCHEMA TABLE TEMPORARY INDEX VIEW TO IGNORE IF UNIQUE PRIMARY COLUMN CONSTRAINT CHECK SPATIAL FULLTEXT FOREIGN REFERENCES KEY_BLOCK_SIZE
%token <bytes> SHOW DESCRIBE EXPLAIN DATE ESCAPE REPAIR OPTIMIZE TRUNCATE CHANGE MODIFY
%token <bytes> MAXVALUE PARTITION REORGANIZE LESS THAN PROCEDURE TRIGGER
%token <bytes> VINDEX
%token <bytes> STATUS VARIABLES
%token <bytes> GRANT REVOKE OPTION
%token <bytes> JSON_TABLE COLUMNS PATH ORDINALITY NESTED
%token <bytes> CASCADE RESTRICT ACTION NO

// Transaction Tokens
%token <bytes> BEGIN START TRANSACTION COMMIT ROLLBACK

// Type Tokens
%token <bytes> BIT TINYINT SMALLINT MEDIUMINT INT INTEGER BIGINT INTNUM
%token <bytes> REAL DOUBLE FLOAT_TYPE DECIMAL NUMERIC
%token <bytes> TIME TIMESTAMP DATETIME YEAR
%token <bytes> CHAR VARCHAR BOOL CHARACTER VARBINARY NCHAR
%token <bytes> TEXT TINYTEXT MEDIUMTEXT LONGTEXT
%token <bytes> BLOB TINYBLOB MEDIUMBLOB LONGBLOB JSON ENUM
%token <bytes> GEOMETRY POINT LINESTRING POLYGON GEOMETRYCOLLECTION MULTIPOINT MULTILINESTRING MULTIPOLYGON

// Type Modifiers
%token <bytes> NULLX AUTO_INCREMENT APPROXNUM SIGNED UNSIGNED ZEROFILL GENERATED ALWAYS STORED VIRTUAL VISIBLE INVISIBLE

// Supported SHOW tokens
%token <bytes> DATABASES TABLES EXTENDED FULL PROCESSLIST

// SET tokens
%token <bytes> NAMES CHARSET GLOBAL SESSION ISOLATION LEVEL READ WRITE ONLY REPEATABLE COMMITTED UNCOMMITTED SERIALIZABLE

// Functions
%token <bytes> CURRENT_TIMESTAMP DATABASE CURRENT_DATE
%token <bytes> CURRENT_TIME LOCALTIME LOCALTIMESTAMP
%token <bytes> UTC_DATE UTC_TIME UTC_TIMESTAMP
%token <bytes> REPLACE
%token <bytes> CONVERT CAST
%token <bytes> SUBSTR SUBSTRING
%token <bytes> GROUP_CONCAT SEPARATOR

// Match
%token <bytes> MATCH AGAINST BOOLEAN LANGUAGE WITH RECURSIVE QUERY EXPANSION
%token <bytes> OVER WINDOW ROWS RANGE PRECEDING FOLLOWING CURRENT ROW UNBOUNDED

// MySQL reserved words that are unused by this grammar will map to this token.
%token <bytes> UNUSED

%type <statement> command
%type <selStmt> select_statement select_statement_no_with base_select union_lhs union_rhs intersect_rhs
%type <statement> insert_statement update_statement delete_statement set_statement
%type <statement> values_statement
%type <statement> create_statement alter_statement rename_statement drop_statement truncate_statement
%type <ddl> create_table_prefix
%type <statement> analyze_statement show_statement use_statement other_statement
%type <statement> begin_statement commit_statement rollback_statement
%type <statement> grant_statement revoke_statement
%type <privilege> privilege
%type <privileges> privilege_list
%type <privilegeObject> privilege_object
%type <accountName> account_name
%type <accountNames> account_name_list
%type <bytes2> comment_opt comment_list
%type <str> union_op union_or_except_op intersect_op except_op insert_or_replace
%type <str> recursive_opt
%type <str> distinct_opt straight_join_opt cache_opt match_option separator_opt
%type <expr> like_escape_opt
%type <selectExprs> select_expression_list select_expression_list_opt
%type <selectExpr> select_expression
%type <expr> expression
%type <tableExprs> from_opt table_references
%type <tableExpr> table_reference table_factor join_table
%type <jsonTableExpr> json_table_expr
%type <jsonTableColumn> json_table_column
%type <jsonTableColumns> json_table_column_list
%type <joinCondition> join_condition join_condition_opt on_expression_opt
%type <tableNames> table_name_list
%type <str> inner_join outer_join straight_join natural_join
%type <tableName> table_name into_table_name
%type <aliasedTableName> aliased_table_name
%type <indexHints> index_hint_list
%type <expr> where_expression_opt
%type <expr> condition
%type <boolVal> boolean_value
%type <str> compare
%type <ins> insert_data
%type <expr> value value_expression
%type <expr> function_call_keyword function_call_nonkeyword function_call_generic function_call_conflict
%type <selectExprs> func_datetime_precision_opt
%type <str> is_suffix
%type <colTuple> col_tuple
%type <exprs> expression_list
%type <values> tuple_list
%type <values> values_row_list
%type <valTuple> row_tuple tuple_or_empty
%type <valTuple> values_row
%type <expr> tuple_expression
%type <subquery> subquery
%type <colName> column_name
%type <whens> when_expression_list
%type <when> when_expression
%type <expr> expression_opt else_expression_opt
%type <exprs> group_by_opt
%type <expr> having_opt
%type <windows> window_clause_opt window_definition_list
%type <window> window_definition
%type <windowSpec> window_spec
%type <frame> window_frame_clause
%type <frameBound> window_frame_bound
%type <overClause> opt_over_clause
%type <orderBy> order_by_opt order_list
%type <order> order
%type <str> asc_desc_opt
%type <limit> limit_opt
%type <str> lock_opt lock_modifier_opt
%type <columns> ins_column_list column_list
%type <partitions> opt_partition_clause partition_list
%type <updateExprs> on_dup_opt
%type <updateExprs> update_list
%type <setExprs> set_list transaction_chars
%type <bytes> charset_or_character_set
%type <updateExpr> update_expression
%type <setExpr> set_expression transaction_char isolation_level
%type <str> ignore_opt default_opt
%type <str> extended_opt full_opt from_database_opt tables_or_processlist
%type <showFilter> like_or_where_opt
%type <byt> exists_opt not_exists_opt
%type <empty> non_add_drop_or_rename_operation to_opt index_opt constraint_opt
%type <empty> alter_table_operation alter_table_operation_list alter_table_spec alter_table_option change_column_definition column_position_opt
%type <addConstraintObject> add_constraint_object
%type <bytes> reserved_keyword non_reserved_keyword
%type <colIdent> sql_id reserved_sql_id col_alias as_ci_opt using_opt
%type <with> with_opt with_clause
%type <columns> common_table_expr_columns_opt
%type <commonTableExpr> common_table_expr
%type <commonTableExprs> common_table_expr_list
%type <expr> charset_value
%type <tableIdent> table_id reserved_table_id table_alias as_opt_id
%type <empty> as_opt
%type <empty> force_eof ddl_force_eof
%type <str> charset
%type <str> set_session_or_global show_session_or_global
%type <convertType> convert_type
%type <columnType> column_type
%type <columnType> int_type decimal_type numeric_type time_type char_type spatial_type
%type <optVal> length_opt column_comment_attr column_comment_opt on_update_column_attr
%type <expr> column_default_attr
%type <optVal> current_timestamp_opt
%type <str> charset_opt collate_opt
%type <boolVal> unsigned_opt zero_fill_opt
%type <LengthScaleOption> float_length_opt decimal_length_opt
%type <boolVal> null_attr auto_increment_attr
%type <str> generated_storage_opt
%type <str> column_visibility_attr
%type <colKeyOpt> column_key_opt column_key_attr
%type <strs> enum_values
%type <columnDefinition> column_definition
%type <columnAttrs> column_attr_list column_attr
%type <indexDefinition> index_definition
%type <indexDefinition> alter_index_definition
%type <indexDefinition> named_constraint_index_definition
%type <constraintDefinition> constraint_definition
%type <constraintDefinition> foreign_key_definition
%type <referenceDefinition> reference_definition
%type <str> index_or_key
%type <str> index_or_key_opt
%type <colIdent> index_name_opt
%type <str> equal_opt
%type <TableSpec> table_spec table_column_list
%type <str> table_option_list table_option table_opt_value
%type <indexInfo> index_info
%type <indexInfo> alter_index_info
%type <bytes> index_kind
%type <indexColumn> index_column
%type <indexColumns> index_column_list
%type <indexOption> index_option
%type <indexOptions> index_option_list
%type <str> reference_action reference_option
%type <strs> reference_option_list reference_option_list_opt
%type <partDefs> partition_definitions
%type <partDef> partition_definition
%type <partSpec> partition_operation partition_opt

%start any_command

%%

any_command:
  command semicolon_opt
  {
    setParseTree(yylex, $1)
  }

semicolon_opt:
/*empty*/ {}
| ';' {}

command:
  select_statement
  {
    $$ = $1
  }
| insert_statement
| values_statement
| update_statement
| delete_statement
| set_statement
| create_statement
| alter_statement
| rename_statement
| drop_statement
| truncate_statement
| analyze_statement
| show_statement
| use_statement
| begin_statement
| commit_statement
| rollback_statement
| grant_statement
| revoke_statement
| other_statement

select_statement:
  select_statement_no_with
  {
    $$ = $1
  }
| with_clause select_statement_no_with
  {
    switch stmt := $2.(type) {
    case *Select:
      stmt.With = $1
    case *Union:
      stmt.With = $1
    }
    $$ = $2
  }

select_statement_no_with:
  base_select order_by_opt limit_opt lock_opt
  {
    sel := $1.(*Select)
    sel.OrderBy = $2
    sel.Limit = $3
    sel.Lock = $4
    $$ = sel
  }
| union_lhs union_or_except_op intersect_rhs order_by_opt limit_opt lock_opt
  {
    $$ = &Union{Type: $2, Left: $1, Right: $3, OrderBy: $4, Limit: $5, Lock: $6}
  }
| union_lhs intersect_op union_rhs order_by_opt limit_opt lock_opt
  {
    $$ = &Union{Type: $2, Left: $1, Right: $3, OrderBy: $4, Limit: $5, Lock: $6}
  }

with_opt:
  {
    $$ = nil
  }
| with_clause
  {
    $$ = $1
  }

with_clause:
  WITH recursive_opt common_table_expr_list
  {
    $$ = &With{Recursive: $2 != "", Ctes: $3}
  }

recursive_opt:
  {
    $$ = ""
  }
| RECURSIVE
  {
    $$ = string($1)
  }

common_table_expr_list:
  common_table_expr
  {
    $$ = CommonTableExprs{$1}
  }
| common_table_expr_list ',' common_table_expr
  {
    $$ = append($1, $3)
  }

common_table_expr:
  table_id common_table_expr_columns_opt AS subquery
  {
    $$ = &CommonTableExpr{ID: $1, Columns: $2, Subquery: $4}
  }

common_table_expr_columns_opt:
  {
    $$ = nil
  }
| openb column_list closeb
  {
    $$ = $2
  }

// base_select is an unparenthesized SELECT with no order by clause or beyond.
base_select:
  SELECT comment_opt cache_opt distinct_opt straight_join_opt select_expression_list from_opt where_expression_opt group_by_opt having_opt window_clause_opt
  {
    $$ = &Select{Comments: Comments($2), Cache: $3, Distinct: $4, Hints: $5, SelectExprs: $6, From: $7, Where: NewWhere(WhereStr, $8), GroupBy: GroupBy($9), Having: NewWhere(HavingStr, $10), Windows: $11}
  }

union_lhs:
  select_statement
  {
    $$ = $1
  }
| openb select_statement closeb
  {
    $$ = &ParenSelect{Select: $2}
  }

union_rhs:
  base_select
  {
    $$ = $1
  }
| openb select_statement closeb
  {
    $$ = &ParenSelect{Select: $2}
  }


insert_statement:
  with_opt insert_or_replace comment_opt ignore_opt into_table_name opt_partition_clause insert_data on_dup_opt
  {
    // insert_data returns a *Insert pre-filled with Columns & Values
    ins := $7
    ins.With = $1
    ins.Action = $2
    ins.Comments = $3
    ins.Ignore = $4
    ins.Table = $5
    ins.Partitions = $6
    ins.OnDup = OnDup($8)
    $$ = ins
  }
| with_opt insert_or_replace comment_opt ignore_opt into_table_name opt_partition_clause SET update_list on_dup_opt
  {
    cols := make(Columns, 0, len($8))
    vals := make(ValTuple, 0, len($9))
    for _, updateList := range $8 {
      cols = append(cols, updateList.Name.Name)
      vals = append(vals, updateList.Expr)
    }
    $$ = &Insert{With: $1, Action: $2, Comments: Comments($3), Ignore: $4, Table: $5, Partitions: $6, Columns: cols, Rows: Values{vals}, OnDup: OnDup($9)}
  }

values_statement:
  VALUES values_row_list
  {
    $$ = &ValuesStatement{Rows: $2}
  }

insert_or_replace:
  INSERT
  {
    $$ = InsertStr
  }
| REPLACE
  {
    $$ = ReplaceStr
  }

update_statement:
  with_opt UPDATE comment_opt table_references SET update_list where_expression_opt order_by_opt limit_opt
  {
    $$ = &Update{With: $1, Comments: Comments($3), TableExprs: $4, Exprs: $6, Where: NewWhere(WhereStr, $7), OrderBy: $8, Limit: $9}
  }

delete_statement:
  with_opt DELETE comment_opt FROM table_name opt_partition_clause where_expression_opt order_by_opt limit_opt
  {
    $$ = &Delete{With: $1, Comments: Comments($3), TableExprs:  TableExprs{&AliasedTableExpr{Expr:$5}}, Partitions: $6, Where: NewWhere(WhereStr, $7), OrderBy: $8, Limit: $9}
  }
| with_opt DELETE comment_opt FROM table_name_list USING table_references where_expression_opt
  {
    $$ = &Delete{With: $1, Comments: Comments($3), Targets: $5, TableExprs: $7, Where: NewWhere(WhereStr, $8)}
  }
| with_opt DELETE comment_opt table_name_list from_or_using table_references where_expression_opt
  {
    $$ = &Delete{With: $1, Comments: Comments($3), Targets: $4, TableExprs: $6, Where: NewWhere(WhereStr, $7)}
  }

from_or_using:
  FROM {}
| USING {}

table_name_list:
  table_name
  {
    $$ = TableNames{$1}
  }
| table_name_list ',' table_name
  {
    $$ = append($$, $3)
  }

opt_partition_clause:
  {
    $$ = nil
  }
| PARTITION openb partition_list closeb
  {
  $$ = $3
  }

set_statement:
  SET comment_opt set_list
  {
    $$ = &Set{Comments: Comments($2), Exprs: $3}
  }
| SET comment_opt set_session_or_global set_list
  {
    $$ = &Set{Comments: Comments($2), Scope: $3, Exprs: $4}
  }
| SET comment_opt set_session_or_global TRANSACTION transaction_chars
  {
    $$ = &Set{Comments: Comments($2), Scope: $3, Exprs: $5}
  }
| SET comment_opt TRANSACTION transaction_chars
  {
    $$ = &Set{Comments: Comments($2), Exprs: $4}
  }

transaction_chars:
  transaction_char
  {
    $$ = SetExprs{$1}
  }
| transaction_chars ',' transaction_char
  {
    $$ = append($$, $3)
  }

transaction_char:
  ISOLATION LEVEL isolation_level
  {
    $$ = $3
  }
| READ WRITE
  {
    $$ = &SetExpr{Name: NewColIdent("tx_read_only"), Expr: NewIntVal([]byte("0"))}
  }
| READ ONLY
  {
    $$ = &SetExpr{Name: NewColIdent("tx_read_only"), Expr: NewIntVal([]byte("1"))}
  }

isolation_level:
  REPEATABLE READ
  {
    $$ = &SetExpr{Name: NewColIdent("tx_isolation"), Expr: NewStrVal([]byte("repeatable read"))}
  }
| READ COMMITTED
  {
    $$ = &SetExpr{Name: NewColIdent("tx_isolation"), Expr: NewStrVal([]byte("read committed"))}
  }
| READ UNCOMMITTED
  {
    $$ = &SetExpr{Name: NewColIdent("tx_isolation"), Expr: NewStrVal([]byte("read uncommitted"))}
  }
| SERIALIZABLE
  {
    $$ = &SetExpr{Name: NewColIdent("tx_isolation"), Expr: NewStrVal([]byte("serializable"))}
  }

set_session_or_global:
  SESSION
  {
    $$ = SessionStr
  }
| GLOBAL
  {
    $$ = GlobalStr
  }

create_statement:
  create_table_prefix table_spec partition_opt
  {
    $1.TableSpec = $2
    $1.PartitionSpec = $3
    $$ = $1
  }
| create_table_prefix LIKE table_name
  {
    $1.LikeTable = $3
    $$ = $1
  }
| create_table_prefix AS select_statement
  {
    $1.OptSelect = $3
    $$ = $1
  }
| CREATE constraint_opt INDEX sql_id using_opt ON table_name ddl_force_eof
  {
    // Change this to an alter statement
    $$ = &DDL{Action: AlterStr, Table: $7, NewName:$7}
  }
| CREATE VIEW table_name AS select_statement
  {
    $$ = &DDL{Action: CreateStr, NewName: $3.ToViewName(), OptSelect: $5}
  }
| CREATE OR REPLACE VIEW table_name AS select_statement
  {
    $$ = &DDL{Action: CreateStr, NewName: $5.ToViewName(), OptSelect: $7}
  }
| CREATE DATABASE not_exists_opt ID ddl_force_eof
  {
    $$ = &DBDDL{Action: CreateStr, DBName: string($4)}
  }
| CREATE SCHEMA not_exists_opt ID ddl_force_eof
  {
    $$ = &DBDDL{Action: CreateStr, DBName: string($4)}
  }

create_table_prefix:
  CREATE TABLE not_exists_opt table_name
  {
    $$ = &DDL{Action: CreateStr, IfNotExists: $3 == 1, NewName: $4}
    setDDL(yylex, $$)
  }
| CREATE TEMPORARY TABLE not_exists_opt table_name
  {
    $$ = &DDL{Action: CreateStr, Temporary: true, IfNotExists: $4 == 1, NewName: $5}
    setDDL(yylex, $$)
  }

table_spec:
  '(' table_column_list ')' table_option_list
  {
    $$ = $2
    $$.Options = $4
  }

table_column_list:
  column_definition
  {
    $$ = &TableSpec{}
    $$.AddColumn($1)
  }
| table_column_list ',' column_definition
  {
    $$.AddColumn($3)
  }
| table_column_list ',' index_definition
  {
    $$.AddIndex($3)
  }
| table_column_list ',' constraint_definition
  {
    $$.AddConstraint($3)
  }
| table_column_list ',' CONSTRAINT sql_id named_constraint_index_definition
  {
    if $5 != nil && $5.Info != nil && $5.Info.Name.IsEmpty() {
      $5.Info.Name = $4
    }
    $$.AddIndex($5)
  }

column_definition:
  sql_id column_type column_attr_list
  {
    $2 = applyColumnAttrSet($2, $3)
    $$ = &ColumnDefinition{Name: $1, Type: $2}
  }
| sql_id column_type GENERATED ALWAYS AS openb expression closeb generated_storage_opt column_key_opt column_comment_opt
  {
    $2.GeneratedExpr = $7
    $2.GeneratedStorage = $9
    $2.KeyOpt = $10
    $2.Comment = $11
    $$ = &ColumnDefinition{Name: $1, Type: $2}
  }
column_type:
  numeric_type unsigned_opt zero_fill_opt
  {
    $$ = $1
    $$.Unsigned = $2
    $$.Zerofill = $3
  }
| char_type
| time_type
| spatial_type

numeric_type:
  int_type length_opt
  {
    $$ = $1
    $$.Length = $2
  }
| decimal_type
  {
    $$ = $1
  }

int_type:
  BIT
  {
    $$ = ColumnType{Type: string($1)}
  }
| BOOL
  {
    $$ = ColumnType{Type: string($1)}
  }
| BOOLEAN
  {
    $$ = ColumnType{Type: string($1)}
  }
| TINYINT
  {
    $$ = ColumnType{Type: string($1)}
  }
| SMALLINT
  {
    $$ = ColumnType{Type: string($1)}
  }
| MEDIUMINT
  {
    $$ = ColumnType{Type: string($1)}
  }
| INT
  {
    $$ = ColumnType{Type: string($1)}
  }
| INTEGER
  {
    $$ = ColumnType{Type: string($1)}
  }
| BIGINT
  {
    $$ = ColumnType{Type: string($1)}
  }

decimal_type:
REAL float_length_opt
  {
    $$ = ColumnType{Type: string($1)}
    $$.Length = $2.Length
    $$.Scale = $2.Scale
  }
| DOUBLE float_length_opt
  {
    $$ = ColumnType{Type: string($1)}
    $$.Length = $2.Length
    $$.Scale = $2.Scale
  }
| FLOAT_TYPE float_length_opt
  {
    $$ = ColumnType{Type: string($1)}
    $$.Length = $2.Length
    $$.Scale = $2.Scale
  }
| DECIMAL decimal_length_opt
  {
    $$ = ColumnType{Type: string($1)}
    $$.Length = $2.Length
    $$.Scale = $2.Scale
  }
| NUMERIC decimal_length_opt
  {
    $$ = ColumnType{Type: string($1)}
    $$.Length = $2.Length
    $$.Scale = $2.Scale
  }

time_type:
  DATE
  {
    $$ = ColumnType{Type: string($1)}
  }
| TIME length_opt
  {
    $$ = ColumnType{Type: string($1), Length: $2}
  }
| TIMESTAMP length_opt
  {
    $$ = ColumnType{Type: string($1), Length: $2}
  }
| DATETIME length_opt
  {
    $$ = ColumnType{Type: string($1), Length: $2}
  }
| YEAR
  {
    $$ = ColumnType{Type: string($1)}
  }

char_type:
  CHAR length_opt charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), Length: $2, Charset: $3, Collate: $4}
  }
| VARCHAR length_opt charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), Length: $2, Charset: $3, Collate: $4}
  }
| BINARY length_opt
  {
    $$ = ColumnType{Type: string($1), Length: $2}
  }
| VARBINARY length_opt
  {
    $$ = ColumnType{Type: string($1), Length: $2}
  }
| TEXT charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), Charset: $2, Collate: $3}
  }
| TINYTEXT charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), Charset: $2, Collate: $3}
  }
| MEDIUMTEXT charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), Charset: $2, Collate: $3}
  }
| LONGTEXT charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), Charset: $2, Collate: $3}
  }
| BLOB
  {
    $$ = ColumnType{Type: string($1)}
  }
| TINYBLOB
  {
    $$ = ColumnType{Type: string($1)}
  }
| MEDIUMBLOB
  {
    $$ = ColumnType{Type: string($1)}
  }
| LONGBLOB
  {
    $$ = ColumnType{Type: string($1)}
  }
| JSON
  {
    $$ = ColumnType{Type: string($1)}
  }
| ENUM '(' enum_values ')' charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), EnumValues: $3, Charset: $5, Collate: $6}
  }
// need set_values / SetValues ?
| SET '(' enum_values ')' charset_opt collate_opt
  {
    $$ = ColumnType{Type: string($1), EnumValues: $3, Charset: $5, Collate: $6}
  }

spatial_type:
  GEOMETRY
  {
    $$ = ColumnType{Type: string($1)}
  }
| POINT
  {
    $$ = ColumnType{Type: string($1)}
  }
| LINESTRING
  {
    $$ = ColumnType{Type: string($1)}
  }
| POLYGON
  {
    $$ = ColumnType{Type: string($1)}
  }
| GEOMETRYCOLLECTION
  {
    $$ = ColumnType{Type: string($1)}
  }
| MULTIPOINT
  {
    $$ = ColumnType{Type: string($1)}
  }
| MULTILINESTRING
  {
    $$ = ColumnType{Type: string($1)}
  }
| MULTIPOLYGON
  {
    $$ = ColumnType{Type: string($1)}
  }

enum_values:
  STRING
  {
    $$ = make([]string, 0, 4)
    $$ = append($$, "'" + string($1) + "'")
  }
| enum_values ',' STRING
  {
    $$ = append($1, "'" + string($3) + "'")
  }

length_opt:
  {
    $$ = nil
  }
| '(' INTEGRAL ')'
  {
    $$ = NewIntVal($2)
  }

float_length_opt:
  {
    $$ = LengthScaleOption{}
  }
| '(' INTEGRAL ',' INTEGRAL ')'
  {
    $$ = LengthScaleOption{
        Length: NewIntVal($2),
        Scale: NewIntVal($4),
    }
  }

decimal_length_opt:
  {
    $$ = LengthScaleOption{}
  }
| '(' INTEGRAL ')'
  {
    $$ = LengthScaleOption{
        Length: NewIntVal($2),
    }
  }
| '(' INTEGRAL ',' INTEGRAL ')'
  {
    $$ = LengthScaleOption{
        Length: NewIntVal($2),
        Scale: NewIntVal($4),
    }
  }

unsigned_opt:
  {
    $$ = BoolVal(false)
  }
| UNSIGNED
  {
    $$ = BoolVal(true)
  }

zero_fill_opt:
  {
    $$ = BoolVal(false)
  }
| ZEROFILL
  {
    $$ = BoolVal(true)
  }

column_attr_list:
  {
    $$ = &columnAttrSet{}
  }
| column_attr_list column_attr
  {
    if mergeColumnAttrSet(yylex, $1, $2) {
      return 1
    }
    $$ = $1
  }

column_attr:
  null_attr
  {
    $$ = &columnAttrSet{NotNullSet: true, NotNull: $1}
  }
| column_default_attr
  {
    $$ = &columnAttrSet{Default: $1}
  }
| on_update_column_attr
  {
    $$ = &columnAttrSet{OnUpdate: $1}
  }
| auto_increment_attr
  {
    $$ = &columnAttrSet{AutoIncrementSet: true, AutoIncrement: $1}
  }
| column_visibility_attr
  {
    $$ = &columnAttrSet{VisibilitySet: true, Visibility: $1}
  }
| column_key_attr
  {
    $$ = &columnAttrSet{KeyOptSet: true, KeyOpt: $1}
  }
| column_comment_attr
  {
    $$ = &columnAttrSet{Comment: $1}
  }
| reference_definition
  {
    $$ = &columnAttrSet{Reference: $1}
  }

// null_attr returns false for NULL and true for NOT NULL.
null_attr:
  NULL
  {
    $$ = BoolVal(false)
  }
| NOT NULL
  {
    $$ = BoolVal(true)
  }

column_default_attr:
  DEFAULT value_expression
  {
    $$ = normalizeDefaultExpr($2)
  }

on_update_column_attr:
  ON UPDATE current_timestamp_opt
{
  $$ = $3
}

current_timestamp_opt:
  CURRENT_TIMESTAMP
  {
    $$ = NewValArg($1)
  }
| CURRENT_TIMESTAMP openb closeb
  {
    value := make([]byte, 0, len($1)+2)
    value = append(value, $1...)
    value = append(value, '(')
    value = append(value, ')')
    $$ = NewValArg(value)
  }
| CURRENT_TIMESTAMP openb INTEGRAL closeb
  {
    value := make([]byte, 0, len($1)+len($3)+2)
    value = append(value, $1...)
    value = append(value, '(')
    value = append(value, $3...)
    value = append(value, ')')
    $$ = NewValArg(value)
  }

auto_increment_attr:
  AUTO_INCREMENT
  {
    $$ = BoolVal(true)
  }

column_visibility_attr:
  VISIBLE
  {
    $$ = string($1)
  }
| INVISIBLE
  {
    $$ = string($1)
  }

generated_storage_opt:
  {
    $$ = ""
  }
| VIRTUAL
  {
    $$ = string($1)
  }
| STORED
  {
    $$ = string($1)
  }

charset_opt:
  {
    $$ = ""
  }
| CHARACTER SET ID
  {
    $$ = string($3)
  }
| CHARACTER SET BINARY
  {
    $$ = string($3)
  }

collate_opt:
  {
    $$ = ""
  }
| COLLATE ID
  {
    $$ = string($2)
  }

column_key_opt:
  {
    $$ = colKeyNone
  }
| column_key_attr
  {
    $$ = $1
  }

column_key_attr:
  PRIMARY KEY
  {
    $$ = colKeyPrimary
  }
| KEY
  {
    $$ = colKey
  }
| UNIQUE KEY
  {
    $$ = colKeyUniqueKey
  }
| UNIQUE
  {
    $$ = colKeyUnique
  }

column_comment_attr:
  COMMENT_KEYWORD STRING
  {
    $$ = NewStrVal($2)
  }

column_comment_opt:
  {
    $$ = nil
  }
| column_comment_attr
  {
    $$ = $1
  }

index_definition:
  index_info '(' index_column_list ')' index_option_list
  {
    $$ = &IndexDefinition{Info: $1, Columns: $3, Options: $5}
  }
| index_info '(' index_column_list ')'
  {
    $$ = &IndexDefinition{Info: $1, Columns: $3}
  }

named_constraint_index_definition:
  UNIQUE index_or_key_opt '(' index_column_list ')' index_option_list
  {
    indexType := string($1)
    if $2 != "" {
      indexType += " " + string($2)
    }
    $$ = &IndexDefinition{
      Info: &IndexInfo{Type: indexType, Name: NewColIdent(""), Unique: true},
      Columns: $4,
      Options: $6,
    }
  }
| UNIQUE index_or_key_opt '(' index_column_list ')'
  {
    indexType := string($1)
    if $2 != "" {
      indexType += " " + string($2)
    }
    $$ = &IndexDefinition{
      Info: &IndexInfo{Type: indexType, Name: NewColIdent(""), Unique: true},
      Columns: $4,
    }
  }

alter_index_definition:
  alter_index_info '(' index_column_list ')' index_option_list
  {
    $$ = &IndexDefinition{Info: $1, Columns: $3, Options: $5}
  }
| alter_index_info '(' index_column_list ')'
  {
    $$ = &IndexDefinition{Info: $1, Columns: $3}
  }

constraint_definition:
  CHECK openb expression closeb
  {
    $$ = &ConstraintDefinition{Expr: $3}
  }
| CONSTRAINT sql_id CHECK openb expression closeb
  {
    $$ = &ConstraintDefinition{Name: $2, Expr: $5}
  }
| foreign_key_definition
| CONSTRAINT foreign_key_definition
  {
    $$ = $2
  }
| CONSTRAINT sql_id foreign_key_definition
  {
    $3.Name = $2
    $$ = $3
  }

foreign_key_definition:
  FOREIGN KEY openb column_list closeb reference_definition
  {
    if $6 == nil {
      yylex.(*Tokenizer).Error("missing reference definition")
      return 1
    }
    $$ = &ConstraintDefinition{
      ForeignKeyColumns: $4,
      ReferencedTable: $6.ReferencedTable,
      ReferencedColumns: $6.ReferencedColumns,
      OnDeleteAction: $6.OnDeleteAction,
      OnUpdateAction: $6.OnUpdateAction,
    }
  }

reference_definition:
  REFERENCES table_name openb column_list closeb reference_option_list_opt
  {
    $$ = &ReferenceDefinition{
      ReferencedTable: $2,
      ReferencedColumns: $4,
    }
    for _, option := range $6 {
      if strings.HasPrefix(option, "delete:") {
        $$.OnDeleteAction = strings.TrimPrefix(option, "delete:")
      } else if strings.HasPrefix(option, "update:") {
        $$.OnUpdateAction = strings.TrimPrefix(option, "update:")
      }
    }
  }

reference_option_list_opt:
  {
    $$ = nil
  }
| reference_option_list

reference_option_list:
  reference_option
  {
    $$ = []string{$1}
  }
| reference_option_list reference_option
  {
    $$ = append($1, $2)
  }

reference_option:
  ON DELETE reference_action
  {
    $$ = "delete:" + $3
  }
| ON UPDATE reference_action
  {
    $$ = "update:" + $3
  }

reference_action:
  RESTRICT
  {
    $$ = string($1)
  }
| CASCADE
  {
    $$ = string($1)
  }
| SET NULL
  {
    $$ = string($1) + " " + string($2)
  }
| SET DEFAULT
  {
    $$ = string($1) + " " + string($2)
  }
| NO ACTION
  {
    $$ = string($1) + " " + string($2)
  }

index_option_list:
  index_option
  {
    $$ = []*IndexOption{$1}
  }
| index_option_list index_option
  {
    $$ = append($$, $2)
  }

index_option:
  USING ID
  {
    $$ = &IndexOption{Name: string($1), Using: string($2)}
  }
| KEY_BLOCK_SIZE equal_opt INTEGRAL
  {
    // should not be string
    $$ = &IndexOption{Name: string($1), Value: NewIntVal($3)}
  }
| COMMENT_KEYWORD STRING
  {
    $$ = &IndexOption{Name: string($1), Value: NewStrVal($2)}
  }

equal_opt:
  /* empty */
  {
    $$ = ""
  }
| '='
  {
    $$ = string($1)
  }

index_info:
  PRIMARY KEY
  {
    $$ = &IndexInfo{Type: string($1) + " " + string($2), Name: NewColIdent("PRIMARY"), Primary: true, Unique: true}
  }
| index_kind index_or_key sql_id
  {
    $$ = &IndexInfo{Type: string($1) + " " + string($2), Name: $3, Spatial: strings.EqualFold(string($1), "spatial"), Unique: false}
  }
| UNIQUE index_or_key_opt index_name_opt
  {
    indexType := string($1)
    if $2 != "" {
      indexType += " " + string($2)
    }
    $$ = &IndexInfo{Type: indexType, Name: $3, Unique: true}
  }
| index_or_key index_name_opt
  {
    $$ = &IndexInfo{Type: string($1), Name: $2, Unique: false}
  }

alter_index_info:
  PRIMARY KEY
  {
    $$ = &IndexInfo{Type: string($1) + " " + string($2), Name: NewColIdent("PRIMARY"), Primary: true, Unique: true}
  }
| index_kind index_or_key sql_id
  {
    $$ = &IndexInfo{Type: string($1) + " " + string($2), Name: $3, Spatial: strings.EqualFold(string($1), "spatial"), Unique: false}
  }
| UNIQUE index_or_key_opt index_name_opt
  {
    indexType := string($1)
    if $2 != "" {
      indexType += " " + string($2)
    }
    $$ = &IndexInfo{Type: indexType, Name: $3, Unique: true}
  }
| index_or_key index_name_opt
  {
    $$ = &IndexInfo{Type: string($1), Name: $2, Unique: false}
  }

index_or_key:
    INDEX
  {
    $$ = string($1)
  }
  | KEY
  {
    $$ = string($1)
  }

index_kind:
  FULLTEXT
  {
    $$ = $1
  }
| SPATIAL
  {
    $$ = $1
  }

index_or_key_opt:
  index_or_key
  {
    $$ = $1
  }
| /* empty */
  {
    $$ = ""
  }

index_name_opt:
  sql_id
  {
    $$ = $1
  }
| /* empty */
  {
    $$ = NewColIdent("")
  }

index_column_list:
  index_column
  {
    $$ = []*IndexColumn{$1}
  }
| index_column_list ',' index_column
  {
    $$ = append($$, $3)
  }

index_column:
  sql_id length_opt
  {
      $$ = &IndexColumn{Column: $1, Length: $2}
  }

table_option_list:
  {
    $$ = ""
  }
| table_option
  {
    $$ = " " + string($1)
  }
| table_option_list ',' table_option
  {
    $$ = string($1) + ", " + string($3)
  }

// rather than explicitly parsing the various keywords for table options,
// just accept any number of keywords, IDs, strings, numbers, and '='
table_option:
  table_opt_value
  {
    $$ = $1
  }
| table_option table_opt_value
  {
    $$ = $1 + " " + $2
  }
| table_option '=' table_opt_value
  {
    $$ = $1 + "=" + $3
  }

table_opt_value:
  reserved_sql_id
  {
    $$ = $1.String()
  }
| STRING
  {
    $$ = "'" + string($1) + "'"
  }
| INTEGRAL
  {
    $$ = string($1)
  }

alter_statement:
  ALTER ignore_opt TABLE table_name non_add_drop_or_rename_operation force_eof
  {
    $$ = &DDL{Action: AlterStr, Table: $4, NewName: $4}
  }
| ALTER ignore_opt TABLE table_name alter_table_operation_list force_eof
  {
    ddl := &DDL{Action: AlterStr, Table: $4, NewName: $4}
    if yylex.(*Tokenizer).partialDDL != nil {
      ddl.TableSpec = yylex.(*Tokenizer).partialDDL.TableSpec
      ddl.AlterConstraint = yylex.(*Tokenizer).partialDDL.AlterConstraint
      ddl.AlterDropForeignKey = yylex.(*Tokenizer).partialDDL.AlterDropForeignKey
      ddl.AlterIndex = yylex.(*Tokenizer).partialDDL.AlterIndex
    }
    $$ = ddl
  }
| ALTER ignore_opt TABLE table_name RENAME to_opt table_name
  {
    // Change this to a rename statement
    $$ = &DDL{Action: RenameStr, Table: $4, NewName: $7}
  }
| ALTER VIEW table_name AS select_statement
  {
    $$ = &DDL{Action: AlterStr, Table: $3.ToViewName(), NewName: $3.ToViewName(), OptSelect: $5}
  }
| ALTER ignore_opt TABLE table_name partition_operation
  {
    $$ = &DDL{Action: AlterStr, Table: $4, PartitionSpec: $5}
  }

alter_table_operation_list:
  alter_table_operation
  {
    $$ = struct{}{}
  }
| alter_table_operation_list ',' alter_table_operation
  {
    setDDL(yylex, nil)
    $$ = struct{}{}
  }

alter_table_operation:
  alter_table_spec
  {
    $$ = struct{}{}
  }
| alter_table_option
  {
    $$ = struct{}{}
  }

alter_table_spec:
  ADD COLUMN column_definition column_position_opt
  {
    $$ = struct{}{}
  }
| ADD column_definition column_position_opt
  {
    $$ = struct{}{}
  }
| ADD openb table_column_list closeb
  {
    setDDL(yylex, &DDL{TableSpec: $3})
    $$ = struct{}{}
  }
| ADD add_constraint_object
  {
    ddl := &DDL{}
    if $2 != nil {
      if $2.Constraint != nil {
        ddl.AlterConstraint = $2.Constraint
      } else if $2.Index != nil {
        ddl.AlterIndex = $2.Index
      }
    }
    setDDL(yylex, ddl)
    $$ = struct{}{}
  }
| DROP index_opt sql_id
  {
    $$ = struct{}{}
  }
| DROP COLUMN sql_id
  {
    $$ = struct{}{}
  }
| DROP sql_id
  {
    $$ = struct{}{}
  }
| DROP FOREIGN KEY sql_id
  {
    setDDL(yylex, &DDL{AlterDropForeignKey: $4})
    $$ = struct{}{}
  }
| DROP PRIMARY KEY
  {
    $$ = struct{}{}
  }
| MODIFY COLUMN column_definition column_position_opt
  {
    $$ = struct{}{}
  }
| MODIFY column_definition column_position_opt
  {
    $$ = struct{}{}
  }
| CHANGE COLUMN sql_id sql_id change_column_definition column_position_opt
  {
    $$ = struct{}{}
  }
| CHANGE sql_id sql_id change_column_definition column_position_opt
  {
    $$ = struct{}{}
  }
| RENAME index_opt sql_id TO sql_id
  {
    $$ = struct{}{}
  }

change_column_definition:
  column_type column_attr_list
  {
    $$ = struct{}{}
  }
| column_type GENERATED ALWAYS AS openb expression closeb generated_storage_opt column_key_opt column_comment_opt
  {
    $$ = struct{}{}
  }

column_position_opt:
  {
    $$ = struct{}{}
  }
| FIRST
  {
    $$ = struct{}{}
  }
| AFTER sql_id
  {
    $$ = struct{}{}
  }

alter_table_option:
  ALGORITHM equal_opt reserved_sql_id
  {
    if !$3.EqualString("default") && !$3.EqualString("instant") && !$3.EqualString("inplace") && !$3.EqualString("copy") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = struct{}{}
  }
| LOCK equal_opt reserved_sql_id
  {
    if !$3.EqualString("default") && !$3.EqualString("none") && !$3.EqualString("shared") && !$3.EqualString("exclusive") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = struct{}{}
  }

add_constraint_object:
  CONSTRAINT sql_id PRIMARY KEY
  {
    $$ = &addConstraintObject{}
  }
| CONSTRAINT sql_id alter_index_definition
  {
    if $3 != nil && $3.Info != nil && $3.Info.Name.IsEmpty() {
      $3.Info.Name = $2
    }
    $$ = &addConstraintObject{Index: $3}
  }
| CONSTRAINT PRIMARY KEY
  {
    $$ = &addConstraintObject{}
  }
| CONSTRAINT alter_index_definition
  {
    $$ = &addConstraintObject{Index: $2}
  }
| PRIMARY KEY
  {
    $$ = &addConstraintObject{}
  }
| alter_index_definition
  {
    $$ = &addConstraintObject{Index: $1}
  }
| constraint_definition
  {
    $$ = &addConstraintObject{Constraint: $1}
  }

partition_operation:
  REORGANIZE PARTITION sql_id INTO openb partition_definitions closeb
  {
    $$ = &PartitionSpec{Action: ReorganizeStr, Name: $3, Definitions: $6}
  }

partition_opt:
  {
    $$ = nil
  }
| PARTITION BY RANGE openb value_expression closeb openb partition_definitions closeb
  {
    $$ = &PartitionSpec{Action: PartitionByRangeStr, Expr: $5, Definitions: $8}
  }

partition_definitions:
  partition_definition
  {
    $$ = []*PartitionDefinition{$1}
  }
| partition_definitions ',' partition_definition
  {
    $$ = append($1, $3)
  }

partition_definition:
  PARTITION sql_id VALUES LESS THAN openb value_expression closeb
  {
    $$ = &PartitionDefinition{Name: $2, Limit: $7}
  }
| PARTITION sql_id VALUES LESS THAN openb value_expression closeb sql_id '=' sql_id
  {
    if !$9.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Limit: $7, Engine: $11}
  }
| PARTITION sql_id VALUES LESS THAN openb value_expression closeb sql_id sql_id
  {
    if !$9.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Limit: $7, Engine: $10}
  }
| PARTITION sql_id VALUES LESS THAN openb value_expression closeb sql_id sql_id '=' sql_id
  {
    if !$9.EqualString("storage") || !$10.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Limit: $7, Engine: $12}
  }
| PARTITION sql_id VALUES LESS THAN openb value_expression closeb sql_id sql_id sql_id
  {
    if !$9.EqualString("storage") || !$10.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Limit: $7, Engine: $11}
  }
| PARTITION sql_id VALUES LESS THAN openb MAXVALUE closeb
  {
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true}
  }
| PARTITION sql_id VALUES LESS THAN openb MAXVALUE closeb sql_id '=' sql_id
  {
    if !$9.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $11}
  }
| PARTITION sql_id VALUES LESS THAN openb MAXVALUE closeb sql_id sql_id
  {
    if !$9.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $10}
  }
| PARTITION sql_id VALUES LESS THAN openb MAXVALUE closeb sql_id sql_id '=' sql_id
  {
    if !$9.EqualString("storage") || !$10.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $12}
  }
| PARTITION sql_id VALUES LESS THAN openb MAXVALUE closeb sql_id sql_id sql_id
  {
    if !$9.EqualString("storage") || !$10.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $11}
  }
| PARTITION sql_id VALUES LESS THAN MAXVALUE
  {
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true}
  }
| PARTITION sql_id VALUES LESS THAN MAXVALUE sql_id '=' sql_id
  {
    if !$7.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $9}
  }
| PARTITION sql_id VALUES LESS THAN MAXVALUE sql_id sql_id
  {
    if !$7.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $8}
  }
| PARTITION sql_id VALUES LESS THAN MAXVALUE sql_id sql_id '=' sql_id
  {
    if !$7.EqualString("storage") || !$8.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $10}
  }
| PARTITION sql_id VALUES LESS THAN MAXVALUE sql_id sql_id sql_id
  {
    if !$7.EqualString("storage") || !$8.EqualString("engine") {
      yylex.Error("syntax error")
      return 1
    }
    $$ = &PartitionDefinition{Name: $2, Maxvalue: true, Engine: $9}
  }

rename_statement:
  RENAME TABLE table_name TO table_name
  {
    $$ = &DDL{Action: RenameStr, Table: $3, NewName: $5}
  }

drop_statement:
  DROP TABLE exists_opt table_name
  {
    var exists bool
    if $3 != 0 {
      exists = true
    }
    $$ = &DDL{Action: DropStr, Table: $4, IfExists: exists}
  }
| DROP INDEX sql_id ON table_name ddl_force_eof
  {
    // Change this to an alter statement
    $$ = &DDL{Action: AlterStr, Table: $5, NewName: $5}
  }
| DROP VIEW exists_opt table_name ddl_force_eof
  {
    var exists bool
        if $3 != 0 {
          exists = true
        }
    $$ = &DDL{Action: DropStr, Table: $4.ToViewName(), IfExists: exists}
  }
| DROP DATABASE exists_opt ID
  {
    $$ = &DBDDL{Action: DropStr, DBName: string($4)}
  }
| DROP SCHEMA exists_opt ID
  {
    $$ = &DBDDL{Action: DropStr, DBName: string($4)}
  }

truncate_statement:
  TRUNCATE TABLE table_name
  {
    $$ = &DDL{Action: TruncateStr, Table: $3}
  }
| TRUNCATE table_name
  {
    $$ = &DDL{Action: TruncateStr, Table: $2}
  }
analyze_statement:
  ANALYZE TABLE table_name
  {
    $$ = &DDL{Action: AlterStr, Table: $3, NewName: $3}
  }

show_statement:
  SHOW BINARY ID ddl_force_eof /* SHOW BINARY LOGS */
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
| SHOW CHARACTER SET ddl_force_eof
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
| SHOW CREATE DATABASE ddl_force_eof
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
/* Rule to handle SHOW CREATE EVENT, SHOW CREATE FUNCTION, etc. */
| SHOW CREATE ID ddl_force_eof
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
| SHOW CREATE PROCEDURE ddl_force_eof
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
| SHOW CREATE TABLE ddl_force_eof
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
| SHOW CREATE TRIGGER ddl_force_eof
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
| SHOW CREATE VIEW ddl_force_eof
  {
    $$ = &Show{Type: string($2) + " " + string($3)}
  }
| SHOW DATABASES ddl_force_eof
  {
    $$ = &Show{Type: string($2)}
  }
| SHOW INDEX ddl_force_eof
  {
    $$ = &Show{Type: string($2)}
  }
| SHOW KEYS ddl_force_eof
  {
    $$ = &Show{Type: string($2)}
  }
| SHOW PROCEDURE ddl_force_eof
  {
    $$ = &Show{Type: string($2)}
  }
| SHOW show_session_or_global STATUS ddl_force_eof
  {
    $$ = &Show{Scope: $2, Type: string($3)}
  }
| SHOW TABLE ddl_force_eof
  {
    $$ = &Show{Type: string($2)}
  }
| SHOW extended_opt full_opt tables_or_processlist from_database_opt like_or_where_opt
  {
    // this is ugly, but I couldn't find a better way for now
    if $4 == "processlist" {
      $$ = &Show{Type: $4}
    } else {
      showTablesOpt := &ShowTablesOpt{Extended: $2, Full:$3, DbName:$5, Filter:$6}
      $$ = &Show{Type: $4, ShowTablesOpt: showTablesOpt}
    }
  }
| SHOW show_session_or_global VARIABLES ddl_force_eof
  {
    $$ = &Show{Scope: $2, Type: string($3)}
  }
| SHOW ID ddl_force_eof
  {
    if !isAllowedGenericShowType($2) {
      yylex.Error("invalid show statement")
      return 1
    }
    $$ = &Show{Type: string($2)}
  }

tables_or_processlist:
  TABLES
  {
    $$ = string($1)
  }
| PROCESSLIST
  {
    $$ = string($1)
  }

extended_opt:
  /* empty */
  {
    $$ = ""
  }
| EXTENDED
  {
    $$ = "extended "
  }

full_opt:
  /* empty */
  {
    $$ = ""
  }
| FULL
  {
    $$ = "full "
  }

from_database_opt:
  /* empty */
  {
    $$ = ""
  }
| FROM table_id
  {
    $$ = $2.v
  }
| IN table_id
  {
    $$ = $2.v
  }

like_or_where_opt:
  /* empty */
  {
    $$ = nil
  }
| LIKE STRING
  {
    $$ = &ShowFilter{Like:string($2)}
  }
| WHERE expression
  {
    $$ = &ShowFilter{Filter:$2}
  }

show_session_or_global:
  /* empty */
  {
    $$ = ""
  }
| SESSION
  {
    $$ = SessionStr
  }
| GLOBAL
  {
    $$ = GlobalStr
  }

use_statement:
  USE table_id
  {
    $$ = &Use{DBName: $2}
  }
| USE
  {
    $$ = &Use{DBName:TableIdent{v:""}}
  }

begin_statement:
  BEGIN
  {
    $$ = &Begin{}
  }
| START TRANSACTION
  {
    $$ = &Begin{}
  }

commit_statement:
  COMMIT
  {
    $$ = &Commit{}
  }

rollback_statement:
  ROLLBACK
  {
    $$ = &Rollback{}
  }

grant_statement:
  GRANT privilege_list ON privilege_object TO account_name_list
  {
    $$ = &Grant{
      Privileges: $2,
      PrivilegeObject: $4,
      Targets: $6,
    }
  }
| GRANT privilege_list ON privilege_object TO account_name_list WITH GRANT OPTION
  {
    $$ = &Grant{
      Privileges: $2,
      PrivilegeObject: $4,
      Targets: $6,
      WithGrantOption: true,
    }
  }

revoke_statement:
  REVOKE privilege_list ON privilege_object FROM account_name_list
  {
    $$ = &Revoke{
      Privileges: $2,
      PrivilegeObject: $4,
      Targets: $6,
    }
  }
| REVOKE GRANT OPTION FOR privilege_list ON privilege_object FROM account_name_list
  {
    $$ = &Revoke{
      GrantOptionFor: true,
      Privileges: $5,
      PrivilegeObject: $7,
      Targets: $9,
    }
  }

privilege_list:
  privilege
  {
    $$ = Privileges{$1}
  }
| privilege_list ',' privilege
  {
    $$ = append($$, $3)
  }

privilege:
  reserved_sql_id
  {
    $$ = Privilege($1.Lowered())
  }
| ALL
  {
    $$ = Privilege(string($1))
  }
| ALL sql_id
  {
    // Support common "ALL PRIVILEGES" form without introducing a dedicated token.
    $$ = Privilege(string($1))
  }

privilege_object:
  '*' '.' '*'
  {
    $$ = &PrivilegeObject{Global: true}
  }
| table_id '.' '*'
  {
    $$ = &PrivilegeObject{DBName: $1}
  }
| table_name
  {
    $$ = &PrivilegeObject{TableName: $1}
  }

account_name_list:
  account_name
  {
    $$ = AccountNames{$1}
  }
| account_name_list ',' account_name
  {
    $$ = append($1, $3)
  }

account_name:
  STRING
  {
    $$ = &AccountName{User: NewStrVal($1)}
  }
| ID
  {
    $$ = &AccountName{User: NewStrVal($1)}
  }
| STRING ID STRING
  {
    if !isAtSign($2) {
      yylex.Error("expecting @ in account name")
      return 1
    }
    $$ = &AccountName{User: NewStrVal($1), Host: NewStrVal($3)}
  }

other_statement:
  DESC force_eof
  {
    $$ = &OtherRead{}
  }
| DESCRIBE force_eof
  {
    $$ = &OtherRead{}
  }
| EXPLAIN force_eof
  {
    $$ = &OtherRead{}
  }
| REPAIR TABLE force_eof
  {
    $$ = &OtherAdmin{}
  }
| OPTIMIZE TABLE force_eof
  {
    $$ = &OtherAdmin{}
  }

comment_opt:
  {
    setAllowComments(yylex, true)
  }
  comment_list
  {
    $$ = $2
    setAllowComments(yylex, false)
  }

comment_list:
  {
    $$ = nil
  }
| comment_list COMMENT
  {
    $$ = append($1, $2)
  }

union_op:
  UNION
  {
    $$ = UnionStr
  }
| UNION ALL
  {
    $$ = UnionAllStr
  }
| UNION DISTINCT
  {
    $$ = UnionDistinctStr
  }

union_or_except_op:
  union_op
  {
    $$ = $1
  }
| except_op
  {
    $$ = $1
  }

intersect_op:
  INTERSECT
  {
    $$ = IntersectStr
  }
| INTERSECT ALL
  {
    $$ = IntersectAllStr
  }
| INTERSECT DISTINCT
  {
    $$ = IntersectDistinctStr
  }

except_op:
  EXCEPT
  {
    $$ = ExceptStr
  }
| EXCEPT ALL
  {
    $$ = ExceptAllStr
  }
| EXCEPT DISTINCT
  {
    $$ = ExceptDistinctStr
  }

intersect_rhs:
  union_rhs
  {
    $$ = $1
  }
| intersect_rhs intersect_op union_rhs
  {
    $$ = &Union{Type: $2, Left: $1, Right: $3}
  }

cache_opt:
{
  $$ = ""
}
| SQL_NO_CACHE
{
  $$ = SQLNoCacheStr
}
| SQL_CACHE
{
  $$ = SQLCacheStr
}

distinct_opt:
  {
    $$ = ""
  }
| DISTINCT
  {
    $$ = DistinctStr
  }

straight_join_opt:
  {
    $$ = ""
  }
| STRAIGHT_JOIN
  {
    $$ = StraightJoinHint
  }

select_expression_list_opt:
  {
    $$ = nil
  }
| select_expression_list
  {
    $$ = $1
  }

select_expression_list:
  select_expression
  {
    $$ = SelectExprs{$1}
  }
| select_expression_list ',' select_expression
  {
    $$ = append($$, $3)
  }

select_expression:
  '*'
  {
    $$ = &StarExpr{}
  }
| expression as_ci_opt
  {
    $$ = &AliasedExpr{Expr: $1, As: $2}
  }
| table_id '.' '*'
  {
    $$ = &StarExpr{TableName: TableName{Name: $1}}
  }
| table_id '.' reserved_table_id '.' '*'
  {
    $$ = &StarExpr{TableName: TableName{Qualifier: $1, Name: $3}}
  }

as_ci_opt:
  {
    $$ = ColIdent{}
  }
| col_alias
  {
    $$ = $1
  }
| AS col_alias
  {
    $$ = $2
  }

col_alias:
  sql_id
| STRING
  {
    $$ = NewColIdent(string($1))
  }

from_opt:
  {
    $$ = TableExprs{&AliasedTableExpr{Expr:TableName{Name: NewTableIdent("dual")}}}
  }
| FROM table_references
  {
    $$ = $2
  }

table_references:
  table_reference
  {
    $$ = TableExprs{$1}
  }
| table_references ',' table_reference
  {
    $$ = append($$, $3)
  }

table_reference:
  table_factor
| join_table

table_factor:
  aliased_table_name
  {
    $$ = $1
  }
| json_table_expr as_opt_id
  {
    $$ = &AliasedTableExpr{Expr:$1, As: $2}
  }
| subquery as_opt table_id
  {
    $$ = &AliasedTableExpr{Expr:$1, As: $3}
  }
| openb table_references closeb
  {
    $$ = &ParenTableExpr{Exprs: $2}
  }

aliased_table_name:
table_name as_opt_id index_hint_list
  {
    $$ = &AliasedTableExpr{Expr:$1, As: $2, Hints: $3}
  }
| table_name PARTITION openb partition_list closeb as_opt_id index_hint_list
  {
    $$ = &AliasedTableExpr{Expr:$1, Partitions: $4, As: $6, Hints: $7}
  }

json_table_expr:
  JSON_TABLE openb expression ',' STRING COLUMNS openb json_table_column_list closeb closeb
  {
    $$ = &JSONTableExpr{
      Expr: $3,
      Path: NewStrVal($5),
      Columns: $8,
    }
  }

json_table_column_list:
  json_table_column
  {
    $$ = JSONTableColumns{$1}
  }
| json_table_column_list ',' json_table_column
  {
    $$ = append($$, $3)
  }

json_table_column:
  // Initial support intentionally omits ON EMPTY / ON ERROR and DEFAULT clauses.
  sql_id FOR ORDINALITY
  {
    $$ = &JSONTableOrdinalityColumn{Name: $1}
  }
| sql_id column_type PATH STRING
  {
    $$ = &JSONTablePathColumn{Name: $1, Type: $2, Path: NewStrVal($4)}
  }
| sql_id column_type EXISTS PATH STRING
  {
    $$ = &JSONTablePathColumn{Name: $1, Type: $2, Exists: true, Path: NewStrVal($5)}
  }
| NESTED PATH STRING COLUMNS openb json_table_column_list closeb
  {
    $$ = &JSONTableNestedPathColumn{Path: NewStrVal($3), Columns: $6}
  }
| NESTED STRING COLUMNS openb json_table_column_list closeb
  {
    $$ = &JSONTableNestedPathColumn{Path: NewStrVal($2), Columns: $5}
  }

column_list:
  sql_id
  {
    $$ = Columns{$1}
  }
| column_list ',' sql_id
  {
    $$ = append($$, $3)
  }

partition_list:
  sql_id
  {
    $$ = Partitions{$1}
  }
| partition_list ',' sql_id
  {
    $$ = append($$, $3)
  }

// There is a grammar conflict here:
// 1: INSERT INTO a SELECT * FROM b JOIN c ON b.i = c.i
// 2: INSERT INTO a SELECT * FROM b JOIN c ON DUPLICATE KEY UPDATE a.i = 1
// When yacc encounters the ON clause, it cannot determine which way to
// resolve. The %prec override below makes the parser choose the
// first construct, which automatically makes the second construct a
// syntax error. This is the same behavior as MySQL.
join_table:
  table_reference inner_join table_factor join_condition_opt
  {
    $$ = &JoinTableExpr{LeftExpr: $1, Join: $2, RightExpr: $3, Condition: $4}
  }
| table_reference straight_join table_factor on_expression_opt
  {
    $$ = &JoinTableExpr{LeftExpr: $1, Join: $2, RightExpr: $3, Condition: $4}
  }
| table_reference outer_join table_reference join_condition
  {
    $$ = &JoinTableExpr{LeftExpr: $1, Join: $2, RightExpr: $3, Condition: $4}
  }
| table_reference natural_join table_factor
  {
    $$ = &JoinTableExpr{LeftExpr: $1, Join: $2, RightExpr: $3}
  }

join_condition:
  ON expression
  { $$ = JoinCondition{On: $2} }
| USING '(' column_list ')'
  { $$ = JoinCondition{Using: $3} }

join_condition_opt:
%prec JOIN
  { $$ = JoinCondition{} }
| join_condition
  { $$ = $1 }

on_expression_opt:
%prec JOIN
  { $$ = JoinCondition{} }
| ON expression
  { $$ = JoinCondition{On: $2} }

as_opt:
  { $$ = struct{}{} }
| AS
  { $$ = struct{}{} }

as_opt_id:
  {
    $$ = NewTableIdent("")
  }
| table_alias
  {
    $$ = $1
  }
| AS table_alias
  {
    $$ = $2
  }

table_alias:
  table_id
| STRING
  {
    $$ = NewTableIdent(string($1))
  }

inner_join:
  JOIN
  {
    $$ = JoinStr
  }
| INNER JOIN
  {
    $$ = JoinStr
  }
| CROSS JOIN
  {
    $$ = JoinStr
  }

straight_join:
  STRAIGHT_JOIN
  {
    $$ = StraightJoinStr
  }

outer_join:
  LEFT JOIN
  {
    $$ = LeftJoinStr
  }
| LEFT OUTER JOIN
  {
    $$ = LeftJoinStr
  }
| RIGHT JOIN
  {
    $$ = RightJoinStr
  }
| RIGHT OUTER JOIN
  {
    $$ = RightJoinStr
  }

natural_join:
 NATURAL JOIN
  {
    $$ = NaturalJoinStr
  }
| NATURAL outer_join
  {
    if $2 == LeftJoinStr {
      $$ = NaturalLeftJoinStr
    } else {
      $$ = NaturalRightJoinStr
    }
  }

into_table_name:
  INTO table_name
  {
    $$ = $2
  }
| table_name
  {
    $$ = $1
  }

table_name:
  table_id
  {
    $$ = TableName{Name: $1}
  }
| table_id '.' reserved_table_id
  {
    $$ = TableName{Qualifier: $1, Name: $3}
  }

index_hint_list:
  {
    $$ = nil
  }
| USE INDEX openb column_list closeb
  {
    $$ = &IndexHints{Type: UseStr, Indexes: $4}
  }
| IGNORE INDEX openb column_list closeb
  {
    $$ = &IndexHints{Type: IgnoreStr, Indexes: $4}
  }
| FORCE INDEX openb column_list closeb
  {
    $$ = &IndexHints{Type: ForceStr, Indexes: $4}
  }

where_expression_opt:
  {
    $$ = nil
  }
| WHERE expression
  {
    $$ = $2
  }

expression:
  condition
  {
    $$ = $1
  }
| expression AND expression
  {
    $$ = &AndExpr{Left: $1, Right: $3}
  }
| expression OR expression
  {
    $$ = &OrExpr{Left: $1, Right: $3}
  }
| NOT expression
  {
    $$ = &NotExpr{Expr: $2}
  }
| expression IS is_suffix
  {
    $$ = &IsExpr{Operator: $3, Expr: $1}
  }
| value_expression
  {
    $$ = $1
  }
| DEFAULT default_opt
  {
    $$ = &Default{ColName: $2}
  }

default_opt:
  /* empty */
  {
    $$ = ""
  }
| openb ID closeb
  {
    $$ = string($2)
  }

boolean_value:
  TRUE
  {
    $$ = BoolVal(true)
  }
| FALSE
  {
    $$ = BoolVal(false)
  }

condition:
  value_expression compare value_expression
  {
    $$ = &ComparisonExpr{Left: $1, Operator: $2, Right: $3}
  }
| value_expression IN col_tuple
  {
    $$ = &ComparisonExpr{Left: $1, Operator: InStr, Right: $3}
  }
| value_expression NOT IN col_tuple
  {
    $$ = &ComparisonExpr{Left: $1, Operator: NotInStr, Right: $4}
  }
| value_expression LIKE value_expression like_escape_opt
  {
    $$ = &ComparisonExpr{Left: $1, Operator: LikeStr, Right: $3, Escape: $4}
  }
| value_expression NOT LIKE value_expression like_escape_opt
  {
    $$ = &ComparisonExpr{Left: $1, Operator: NotLikeStr, Right: $4, Escape: $5}
  }
| value_expression REGEXP value_expression
  {
    $$ = &ComparisonExpr{Left: $1, Operator: RegexpStr, Right: $3}
  }
| value_expression NOT REGEXP value_expression
  {
    $$ = &ComparisonExpr{Left: $1, Operator: NotRegexpStr, Right: $4}
  }
| value_expression MEMBER OF openb expression closeb
  {
    $$ = &ComparisonExpr{Left: $1, Operator: MemberOfStr, Right: &ParenExpr{Expr: $5}}
  }
| value_expression NOT MEMBER OF openb expression closeb
  {
    $$ = &ComparisonExpr{Left: $1, Operator: NotMemberOfStr, Right: &ParenExpr{Expr: $6}}
  }
| value_expression BETWEEN value_expression AND value_expression
  {
    $$ = &RangeCond{Left: $1, Operator: BetweenStr, From: $3, To: $5}
  }
| value_expression NOT BETWEEN value_expression AND value_expression
  {
    $$ = &RangeCond{Left: $1, Operator: NotBetweenStr, From: $4, To: $6}
  }
| EXISTS subquery
  {
    $$ = &ExistsExpr{Subquery: $2}
  }

is_suffix:
  NULL
  {
    $$ = IsNullStr
  }
| NOT NULL
  {
    $$ = IsNotNullStr
  }
| TRUE
  {
    $$ = IsTrueStr
  }
| NOT TRUE
  {
    $$ = IsNotTrueStr
  }
| FALSE
  {
    $$ = IsFalseStr
  }
| NOT FALSE
  {
    $$ = IsNotFalseStr
  }

compare:
  '='
  {
    $$ = EqualStr
  }
| '<'
  {
    $$ = LessThanStr
  }
| '>'
  {
    $$ = GreaterThanStr
  }
| LE
  {
    $$ = LessEqualStr
  }
| GE
  {
    $$ = GreaterEqualStr
  }
| NE
  {
    $$ = NotEqualStr
  }
| NULL_SAFE_EQUAL
  {
    $$ = NullSafeEqualStr
  }

like_escape_opt:
  {
    $$ = nil
  }
| ESCAPE value_expression
  {
    $$ = $2
  }

col_tuple:
  row_tuple
  {
    $$ = $1
  }
| subquery
  {
    $$ = $1
  }

subquery:
  openb select_statement closeb
  {
    $$ = &Subquery{$2}
  }

expression_list:
  expression
  {
    $$ = Exprs{$1}
  }
| expression_list ',' expression
  {
    $$ = append($1, $3)
  }

value_expression:
  value
  {
    $$ = $1
  }
| boolean_value
  {
    $$ = $1
  }
| column_name
  {
    $$ = $1
  }
| tuple_expression
  {
    $$ = $1
  }
| subquery
  {
    $$ = $1
  }
| value_expression '&' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: BitAndStr, Right: $3}
  }
| value_expression '|' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: BitOrStr, Right: $3}
  }
| value_expression '^' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: BitXorStr, Right: $3}
  }
| value_expression '+' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: PlusStr, Right: $3}
  }
| value_expression '-' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: MinusStr, Right: $3}
  }
| value_expression '*' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: MultStr, Right: $3}
  }
| value_expression '/' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: DivStr, Right: $3}
  }
| value_expression DIV value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: IntDivStr, Right: $3}
  }
| value_expression '%' value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: ModStr, Right: $3}
  }
| value_expression MOD value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: ModStr, Right: $3}
  }
| value_expression SHIFT_LEFT value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: ShiftLeftStr, Right: $3}
  }
| value_expression SHIFT_RIGHT value_expression
  {
    $$ = &BinaryExpr{Left: $1, Operator: ShiftRightStr, Right: $3}
  }
| column_name JSON_EXTRACT_OP value
  {
    $$ = &BinaryExpr{Left: $1, Operator: JSONExtractOp, Right: $3}
  }
| column_name JSON_UNQUOTE_EXTRACT_OP value
  {
    $$ = &BinaryExpr{Left: $1, Operator: JSONUnquoteExtractOp, Right: $3}
  }
| value_expression COLLATE charset
  {
    $$ = &CollateExpr{Expr: $1, Charset: $3}
  }
| BINARY value_expression %prec UNARY
  {
    $$ = &UnaryExpr{Operator: BinaryStr, Expr: $2}
  }
| UNDERSCORE_BINARY value_expression %prec UNARY
  {
    $$ = &UnaryExpr{Operator: UBinaryStr, Expr: $2}
  }
| '+'  value_expression %prec UNARY
  {
    if num, ok := $2.(*SQLVal); ok && num.Type == IntVal {
      $$ = num
    } else {
      $$ = &UnaryExpr{Operator: UPlusStr, Expr: $2}
    }
  }
| '-'  value_expression %prec UNARY
  {
    if num, ok := $2.(*SQLVal); ok && num.Type == IntVal {
      // Handle double negative
      if num.Val[0] == '-' {
        num.Val = num.Val[1:]
        $$ = num
      } else {
        $$ = NewIntVal(append([]byte("-"), num.Val...))
      }
    } else {
      $$ = &UnaryExpr{Operator: UMinusStr, Expr: $2}
    }
  }
| '~'  value_expression
  {
    $$ = &UnaryExpr{Operator: TildaStr, Expr: $2}
  }
| '!' value_expression %prec UNARY
  {
    $$ = &UnaryExpr{Operator: BangStr, Expr: $2}
  }
| INTERVAL value_expression sql_id
  {
    // This rule prevents the usage of INTERVAL
    // as a function. If support is needed for that,
    // we'll need to revisit this. The solution
    // will be non-trivial because of grammar conflicts.
    $$ = &IntervalExpr{Expr: $2, Unit: $3.String()}
  }
| function_call_generic
| function_call_keyword
| function_call_nonkeyword
| function_call_conflict

/*
  Regular function calls without special token or syntax, guaranteed to not
  introduce side effects due to being a simple identifier
*/
function_call_generic:
  sql_id openb select_expression_list_opt closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: $1, Exprs: $3, Over: $5}
  }
| sql_id openb DISTINCT select_expression_list closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: $1, Distinct: true, Exprs: $4, Over: $6}
  }
| table_id '.' reserved_sql_id openb select_expression_list_opt closeb opt_over_clause
  {
    $$ = &FuncExpr{Qualifier: $1, Name: $3, Exprs: $5, Over: $7}
  }

/*
  Function calls using reserved keywords, with dedicated grammar rules
  as a result
*/
function_call_keyword:
  LEFT openb select_expression_list closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: NewColIdent("left"), Exprs: $3, Over: $5}
  }
| RIGHT openb select_expression_list closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: NewColIdent("right"), Exprs: $3, Over: $5}
  }
| CONVERT openb expression ',' convert_type closeb
  {
    $$ = &ConvertExpr{Expr: $3, Type: $5}
  }
| CAST openb expression AS convert_type closeb
  {
    $$ = &ConvertExpr{Expr: $3, Type: $5}
  }
| CONVERT openb expression USING charset closeb
  {
    $$ = &ConvertUsingExpr{Expr: $3, Type: $5}
  }
| SUBSTR openb column_name ',' value_expression closeb
  {
    $$ = &SubstrExpr{Name: $3, From: $5, To: nil}
  }
| SUBSTR openb column_name ',' value_expression ',' value_expression closeb
  {
    $$ = &SubstrExpr{Name: $3, From: $5, To: $7}
  }
| SUBSTR openb column_name FROM value_expression FOR value_expression closeb
  {
    $$ = &SubstrExpr{Name: $3, From: $5, To: $7}
  }
| SUBSTRING openb column_name ',' value_expression closeb
  {
    $$ = &SubstrExpr{Name: $3, From: $5, To: nil}
  }
| SUBSTRING openb column_name ',' value_expression ',' value_expression closeb
  {
    $$ = &SubstrExpr{Name: $3, From: $5, To: $7}
  }
| SUBSTRING openb column_name FROM value_expression FOR value_expression closeb
  {
    $$ = &SubstrExpr{Name: $3, From: $5, To: $7}
  }
| MATCH openb select_expression_list closeb AGAINST openb value_expression match_option closeb
  {
  $$ = &MatchExpr{Columns: $3, Expr: $7, Option: $8}
  }
| GROUP_CONCAT openb distinct_opt select_expression_list order_by_opt separator_opt closeb opt_over_clause
  {
    $$ = &GroupConcatExpr{Distinct: $3, Exprs: $4, OrderBy: $5, Separator: $6, Over: $8}
  }
| CASE expression_opt when_expression_list else_expression_opt END
  {
    $$ = &CaseExpr{Expr: $2, Whens: $3, Else: $4}
  }
| VALUES openb column_name closeb
  {
    $$ = &ValuesFuncExpr{Name: $3}
  }

/*
  Function calls using non reserved keywords but with special syntax forms.
  Dedicated grammar rules are needed because of the special syntax
*/
function_call_nonkeyword:
  CURRENT_TIMESTAMP func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("current_timestamp"), Exprs: $2, Over: $3}
  }
| UTC_TIMESTAMP func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("utc_timestamp"), Exprs: $2, Over: $3}
  }
| UTC_TIME func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("utc_time"), Exprs: $2, Over: $3}
  }
| UTC_DATE func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("utc_date"), Exprs: $2, Over: $3}
  }
  // now
| LOCALTIME func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("localtime"), Exprs: $2, Over: $3}
  }
  // now
| LOCALTIMESTAMP func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("localtimestamp"), Exprs: $2, Over: $3}
  }
  // curdate
| CURRENT_DATE func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("current_date"), Exprs: $2, Over: $3}
  }
  // curtime
| CURRENT_TIME func_datetime_precision_opt opt_over_clause
  {
    $$ = &FuncExpr{Name:NewColIdent("current_time"), Exprs: $2, Over: $3}
  }

func_datetime_precision_opt:
  /* empty */
  {
    $$ = nil
  }
| openb closeb
  {
    $$ = SelectExprs{}
  }
| openb INTEGRAL closeb
  {
    $$ = SelectExprs{&AliasedExpr{Expr: NewIntVal($2)}}
  }

/*
  Function calls using non reserved keywords with *normal* syntax forms. Because
  the names are non-reserved, they need a dedicated rule so as not to conflict
*/
function_call_conflict:
  IF openb select_expression_list closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: NewColIdent("if"), Exprs: $3, Over: $5}
  }
| DATABASE openb select_expression_list_opt closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: NewColIdent("database"), Exprs: $3, Over: $5}
  }
| MOD openb select_expression_list closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: NewColIdent("mod"), Exprs: $3, Over: $5}
  }
| REPLACE openb select_expression_list closeb opt_over_clause
  {
    $$ = &FuncExpr{Name: NewColIdent("replace"), Exprs: $3, Over: $5}
  }

opt_over_clause:
  {
    $$ = nil
  }
| OVER sql_id
  {
    $$ = &OverClause{Name: $2}
  }
| OVER openb window_spec closeb
  {
    $$ = &OverClause{Spec: $3}
  }

match_option:
/*empty*/
  {
    $$ = ""
  }
| IN BOOLEAN MODE
  {
    $$ = BooleanModeStr
  }
| IN NATURAL LANGUAGE MODE
 {
    $$ = NaturalLanguageModeStr
 }
| IN NATURAL LANGUAGE MODE WITH QUERY EXPANSION
 {
    $$ = NaturalLanguageModeWithQueryExpansionStr
 }
| WITH QUERY EXPANSION
 {
    $$ = QueryExpansionStr
 }

charset:
  ID
{
    $$ = string($1)
}
| STRING
{
    $$ = string($1)
}

convert_type:
  BINARY length_opt
  {
    $$ = &ConvertType{Type: string($1), Length: $2}
  }
| CHAR length_opt charset_opt
  {
    $$ = &ConvertType{Type: string($1), Length: $2, Charset: $3, Operator: CharacterSetStr}
  }
| CHAR length_opt ID
  {
    $$ = &ConvertType{Type: string($1), Length: $2, Charset: string($3)}
  }
| DATE
  {
    $$ = &ConvertType{Type: string($1)}
  }
| DATETIME length_opt
  {
    $$ = &ConvertType{Type: string($1), Length: $2}
  }
| DECIMAL decimal_length_opt
  {
    $$ = &ConvertType{Type: string($1)}
    $$.Length = $2.Length
    $$.Scale = $2.Scale
  }
| JSON
  {
    $$ = &ConvertType{Type: string($1)}
  }
| NCHAR length_opt
  {
    $$ = &ConvertType{Type: string($1), Length: $2}
  }
| SIGNED
  {
    $$ = &ConvertType{Type: string($1)}
  }
| SIGNED INTEGER
  {
    $$ = &ConvertType{Type: string($1)}
  }
| TIME length_opt
  {
    $$ = &ConvertType{Type: string($1), Length: $2}
  }
| UNSIGNED
  {
    $$ = &ConvertType{Type: string($1)}
  }
| UNSIGNED INTEGER
  {
    $$ = &ConvertType{Type: string($1)}
  }

expression_opt:
  {
    $$ = nil
  }
| expression
  {
    $$ = $1
  }

separator_opt:
  {
    $$ = string("")
  }
| SEPARATOR STRING
  {
    $$ = " separator '"+string($2)+"'"
  }

when_expression_list:
  when_expression
  {
    $$ = []*When{$1}
  }
| when_expression_list when_expression
  {
    $$ = append($1, $2)
  }

when_expression:
  WHEN expression THEN expression
  {
    $$ = &When{Cond: $2, Val: $4}
  }

else_expression_opt:
  {
    $$ = nil
  }
| ELSE expression
  {
    $$ = $2
  }

column_name:
  sql_id
  {
    $$ = &ColName{Name: $1}
  }
| table_id '.' reserved_sql_id
  {
    $$ = &ColName{Qualifier: TableName{Name: $1}, Name: $3}
  }
| table_id '.' reserved_table_id '.' reserved_sql_id
  {
    $$ = &ColName{Qualifier: TableName{Qualifier: $1, Name: $3}, Name: $5}
  }

value:
  STRING
  {
    $$ = NewStrVal($1)
  }
| HEX
  {
    $$ = NewHexVal($1)
  }
| BIT_LITERAL
  {
    $$ = NewBitVal($1)
  }
| INTEGRAL
  {
    $$ = NewIntVal($1)
  }
| FLOAT
  {
    $$ = NewFloatVal($1)
  }
| HEXNUM
  {
    $$ = NewHexNum($1)
  }
| VALUE_ARG
  {
    $$ = NewValArg($1)
  }
| NULL
  {
    $$ = &NullVal{}
  }

group_by_opt:
  {
    $$ = nil
  }
| GROUP BY expression_list
  {
    $$ = $3
  }

having_opt:
  {
    $$ = nil
  }
| HAVING expression
  {
    $$ = $2
  }

window_clause_opt:
  {
    $$ = nil
  }
| WINDOW window_definition_list
  {
    $$ = $2
  }

window_definition_list:
  window_definition
  {
    $$ = WindowDefinitions{$1}
  }
| window_definition_list ',' window_definition
  {
    $$ = append($$, $3)
  }

window_definition:
  sql_id AS openb window_spec closeb
  {
    $$ = &WindowDefinition{Name: $1, Spec: $4}
  }

window_spec:
  sql_id
  {
    $$ = &WindowSpec{Name: $1}
  }
| PARTITION BY expression_list
  {
    $$ = &WindowSpec{PartitionBy: $3}
  }
| ORDER BY order_list
  {
    $$ = &WindowSpec{OrderBy: $3}
  }
| PARTITION BY expression_list ORDER BY order_list
  {
    $$ = &WindowSpec{PartitionBy: $3, OrderBy: $6}
  }
| sql_id PARTITION BY expression_list
  {
    $$ = &WindowSpec{Name: $1, PartitionBy: $4}
  }
| sql_id ORDER BY order_list
  {
    $$ = &WindowSpec{Name: $1, OrderBy: $4}
  }
| sql_id PARTITION BY expression_list ORDER BY order_list
  {
    $$ = &WindowSpec{Name: $1, PartitionBy: $4, OrderBy: $7}
  }
| PARTITION BY expression_list window_frame_clause
  {
    $$ = &WindowSpec{PartitionBy: $3, Frame: $4}
  }
| ORDER BY order_list window_frame_clause
  {
    $$ = &WindowSpec{OrderBy: $3, Frame: $4}
  }
| PARTITION BY expression_list ORDER BY order_list window_frame_clause
  {
    $$ = &WindowSpec{PartitionBy: $3, OrderBy: $6, Frame: $7}
  }
| sql_id PARTITION BY expression_list window_frame_clause
  {
    $$ = &WindowSpec{Name: $1, PartitionBy: $4, Frame: $5}
  }
| sql_id ORDER BY order_list window_frame_clause
  {
    $$ = &WindowSpec{Name: $1, OrderBy: $4, Frame: $5}
  }
| sql_id PARTITION BY expression_list ORDER BY order_list window_frame_clause
  {
    $$ = &WindowSpec{Name: $1, PartitionBy: $4, OrderBy: $7, Frame: $8}
  }

window_frame_clause:
  ROWS window_frame_bound
  {
    $$ = &WindowFrame{Unit: RowsStr, Start: $2}
  }
| ROWS BETWEEN window_frame_bound AND window_frame_bound
  {
    $$ = &WindowFrame{Unit: RowsStr, Start: $3, End: $5}
  }
| RANGE window_frame_bound
  {
    $$ = &WindowFrame{Unit: RangeStr, Start: $2}
  }
| RANGE BETWEEN window_frame_bound AND window_frame_bound
  {
    $$ = &WindowFrame{Unit: RangeStr, Start: $3, End: $5}
  }

window_frame_bound:
  UNBOUNDED PRECEDING
  {
    $$ = &WindowFrameBound{Type: UnboundedPrecedingStr}
  }
| UNBOUNDED FOLLOWING
  {
    $$ = &WindowFrameBound{Type: UnboundedFollowingStr}
  }
| CURRENT ROW
  {
    $$ = &WindowFrameBound{Type: CurrentRowStr}
  }
| value_expression PRECEDING
  {
    $$ = &WindowFrameBound{Type: ExprPrecedingStr, Expr: $1}
  }
| value_expression FOLLOWING
  {
    $$ = &WindowFrameBound{Type: ExprFollowingStr, Expr: $1}
  }

order_by_opt:
  {
    $$ = nil
  }
| ORDER BY order_list
  {
    $$ = $3
  }

order_list:
  order
  {
    $$ = OrderBy{$1}
  }
| order_list ',' order
  {
    $$ = append($1, $3)
  }

order:
  expression asc_desc_opt
  {
    $$ = &Order{Expr: $1, Direction: $2}
  }

asc_desc_opt:
  {
    $$ = AscScr
  }
| ASC
  {
    $$ = AscScr
  }
| DESC
  {
    $$ = DescScr
  }

limit_opt:
  {
    $$ = nil
  }
| LIMIT expression
  {
    $$ = &Limit{Rowcount: $2}
  }
| LIMIT expression ',' expression
  {
    $$ = &Limit{Offset: $2, Rowcount: $4}
  }
| LIMIT expression OFFSET expression
  {
    $$ = &Limit{Offset: $4, Rowcount: $2}
  }

lock_opt:
  {
    $$ = ""
  }
| FOR UPDATE lock_modifier_opt
  {
    $$ = ForUpdateStr + $3
  }
| FOR SHARE lock_modifier_opt
  {
    $$ = ForShareStr + $3
  }
| LOCK IN SHARE MODE
  {
    $$ = ShareModeStr
  }

lock_modifier_opt:
  {
    $$ = ""
  }
| NOWAIT
  {
    $$ = NoWaitStr
  }
| SKIP LOCKED
  {
    $$ = SkipLockedStr
  }

// insert_data expands all combinations into a single rule.
// This avoids a shift/reduce conflict while encountering the
// following two possible constructs:
// insert into t1(a, b) (select * from t2)
// insert into t1(select * from t2)
// Because the rules are together, the parser can keep shifting
// the tokens until it disambiguates a as sql_id and select as keyword.
insert_data:
  VALUES tuple_list
  {
    $$ = &Insert{Rows: $2}
  }
| select_statement
  {
    $$ = &Insert{Rows: $1}
  }
| openb select_statement closeb
  {
    // Drop the redundant parenthesis.
    $$ = &Insert{Rows: $2}
  }
| openb ins_column_list closeb VALUES tuple_list
  {
    $$ = &Insert{Columns: $2, Rows: $5}
  }
| openb ins_column_list closeb select_statement
  {
    $$ = &Insert{Columns: $2, Rows: $4}
  }
| openb ins_column_list closeb openb select_statement closeb
  {
    // Drop the redundant parenthesis.
    $$ = &Insert{Columns: $2, Rows: $5}
  }

ins_column_list:
  sql_id
  {
    $$ = Columns{$1}
  }
| sql_id '.' sql_id
  {
    $$ = Columns{$3}
  }
| ins_column_list ',' sql_id
  {
    $$ = append($$, $3)
  }
| ins_column_list ',' sql_id '.' sql_id
  {
    $$ = append($$, $5)
  }

on_dup_opt:
  {
    $$ = nil
  }
| ON DUPLICATE KEY UPDATE update_list
  {
    $$ = $5
  }

tuple_list:
  tuple_or_empty
  {
    $$ = Values{$1}
  }
| tuple_list ',' tuple_or_empty
  {
    $$ = append($1, $3)
  }

values_row_list:
  values_row
  {
    $$ = Values{$1}
  }
| values_row_list ',' values_row
  {
    $$ = append($1, $3)
  }

values_row:
  ROW row_tuple
  {
    $$ = $2
  }
| row_tuple
  {
    $$ = $1
  }

tuple_or_empty:
  row_tuple
  {
    $$ = $1
  }
| openb closeb
  {
    $$ = ValTuple{}
  }

row_tuple:
  openb expression_list closeb
  {
    $$ = ValTuple($2)
  }

tuple_expression:
  row_tuple
  {
    if len($1) == 1 {
      $$ = &ParenExpr{$1[0]}
    } else {
      $$ = $1
    }
  }

update_list:
  update_expression
  {
    $$ = UpdateExprs{$1}
  }
| update_list ',' update_expression
  {
    $$ = append($1, $3)
  }

update_expression:
  column_name '=' expression
  {
    $$ = &UpdateExpr{Name: $1, Expr: $3}
  }

set_list:
  set_expression
  {
    $$ = SetExprs{$1}
  }
| set_list ',' set_expression
  {
    $$ = append($1, $3)
  }

set_expression:
  reserved_sql_id '=' ON
  {
    if rejectDeprecatedSetVar(yylex, $1) {
      return 1
    }
    $$ = &SetExpr{Name: $1, Expr: NewStrVal([]byte("on"))}
  }
| reserved_sql_id '=' expression
  {
    if rejectDeprecatedSetVar(yylex, $1) {
      return 1
    }
    $$ = &SetExpr{Name: $1, Expr: $3}
  }
| charset_or_character_set charset_value collate_opt
  {
    $$ = &SetExpr{Name: NewColIdent(string($1)), Expr: $2}
  }

charset_or_character_set:
  CHARSET
| CHARACTER SET
  {
    $$ = []byte("charset")
  }
| NAMES

charset_value:
  sql_id
  {
    $$ = NewStrVal([]byte($1.String()))
  }
| STRING
  {
    $$ = NewStrVal($1)
  }
| DEFAULT
  {
    $$ = &Default{}
  }

exists_opt:
  { $$ = 0 }
| IF EXISTS
  { $$ = 1 }

not_exists_opt:
  { $$ = 0 }
| IF NOT EXISTS
  { $$ = 1 }

ignore_opt:
  { $$ = "" }
| IGNORE
  { $$ = IgnoreStr }

non_add_drop_or_rename_operation:
  ALTER
  { $$ = struct{}{} }
| AUTO_INCREMENT
  { $$ = struct{}{} }
| CHARACTER
  { $$ = struct{}{} }
| COMMENT_KEYWORD
  { $$ = struct{}{} }
| DEFAULT
  { $$ = struct{}{} }
| ORDER
  { $$ = struct{}{} }
| CONVERT
  { $$ = struct{}{} }
| PARTITION
  { $$ = struct{}{} }
| UNUSED
  { $$ = struct{}{} }
| ID
  { $$ = struct{}{} }

to_opt:
  { $$ = struct{}{} }
| TO
  { $$ = struct{}{} }
| AS
  { $$ = struct{}{} }

index_opt:
  INDEX
  { $$ = struct{}{} }
| KEY
  { $$ = struct{}{} }

constraint_opt:
  { $$ = struct{}{} }
| UNIQUE
  { $$ = struct{}{} }
| sql_id
  { $$ = struct{}{} }

using_opt:
  { $$ = ColIdent{} }
| USING sql_id
  { $$ = $2 }

sql_id:
  ID
  {
    $$ = NewColIdent(string($1))
  }
| non_reserved_keyword
  {
    $$ = NewColIdent(string($1))
  }

reserved_sql_id:
  sql_id
| reserved_keyword
  {
    $$ = NewColIdent(string($1))
  }

table_id:
  ID
  {
    $$ = NewTableIdent(string($1))
  }
| non_reserved_keyword
  {
    $$ = NewTableIdent(string($1))
  }

reserved_table_id:
  table_id
| reserved_keyword
  {
    $$ = NewTableIdent(string($1))
  }

/*
  These are not all necessarily reserved in MySQL, but some are.

  These are more importantly reserved because they may conflict with our grammar.
  If you want to move one that is not reserved in MySQL (i.e. ESCAPE) to the
  non_reserved_keywords, you'll need to deal with any conflicts.

  Sorted alphabetically
*/
reserved_keyword:
  ACTION
| ADD
| AND
| AS
| ASC
| AUTO_INCREMENT
| BETWEEN
| BINARY
| BY
| CASCADE
| CASE
| CHANGE
| CHECK
| COLLATE
| CONVERT
| CREATE
| CROSS
| CURRENT_DATE
| CURRENT_TIME
| CURRENT_TIMESTAMP
| SUBSTR
| SUBSTRING
| DATABASE
| DATABASES
| DEFAULT
| DELETE
| DESC
| DESCRIBE
| DISTINCT
| DIV
| DROP
| ELSE
| END
| ESCAPE
| EXISTS
| EXPLAIN
| FALSE
| FOR
| FORCE
| FROM
| GROUP
| GRANT
| HAVING
| IF
| IGNORE
| IN
| INDEX
| INNER
| INSERT
| INTERVAL
| INTO
| IS
| JOIN
| KEY
| LEFT
| LIKE
| LIMIT
| LOCALTIME
| LOCALTIMESTAMP
| LOCK
| MATCH
| MAXVALUE
| MEMBER
| MOD
| NATURAL
| NO
| NOT
| NULL
| OF
| ON
| OR
| ORDER
| OVER
| OUTER
| REGEXP
| REFERENCES
| RENAME
| REPLACE
| RESTRICT
| RIGHT
| SCHEMA
| SELECT
| SEPARATOR
| SET
| SHOW
| STRAIGHT_JOIN
| TABLE
| TABLES
| TEMPORARY
| THEN
| TO
| TRUE
| UNION
| UNIQUE
| UPDATE
| USE
| USING
| UTC_DATE
| UTC_TIME
| UTC_TIMESTAMP
| VALUES
| WHEN
| WHERE
| WINDOW

/*
  These are non-reserved Vitess, because they don't cause conflicts in the grammar.
  Some of them may be reserved in MySQL. The good news is we backtick quote them
  when we rewrite the query, so no issue should arise.

  Sorted alphabetically
*/
non_reserved_keyword:
  AGAINST
| AFTER
| ALWAYS
| BEGIN
| BIGINT
| BIT
| BLOB
| BOOL
| CHAR
| CHARACTER
| CHARSET
| COMMENT_KEYWORD
| COMMIT
| COMMITTED
| COLUMNS
| CURRENT
| DATE
| DATETIME
| DECIMAL
| DOUBLE
| DUPLICATE
| ENUM
| EXPANSION
| FIRST
| FLOAT_TYPE
| FOLLOWING
| FOREIGN
| FULLTEXT
| GENERATED
| GEOMETRY
| GEOMETRYCOLLECTION
| GLOBAL
| INT
| INTEGER
| ISOLATION
| INVISIBLE
| JSON
| JSON_TABLE
| KEY_BLOCK_SIZE
| KEYS
| LANGUAGE
| LAST_INSERT_ID
| LESS
| LEVEL
| LINESTRING
| LONGBLOB
| LOCKED
| LONGTEXT
| MEDIUMBLOB
| MEDIUMINT
| MEDIUMTEXT
| MODE
| MULTILINESTRING
| MULTIPOINT
| MULTIPOLYGON
| NAMES
| NCHAR
| NESTED
| NOWAIT
| NUMERIC
| OFFSET
| ONLY
| OPTION
| OPTIMIZE
| ORDINALITY
| PATH
| POINT
| POLYGON
| PRECEDING
| PRIMARY
| PROCEDURE
| QUERY
| RANGE
| READ
| RECURSIVE
| REAL
| REORGANIZE
| REPAIR
| REPEATABLE
| REVOKE
| ROLLBACK
| ROW
| ROWS
| SESSION
| SERIALIZABLE
| SHARE
| SKIP
| SIGNED
| SMALLINT
| SPATIAL
| START
| STATUS
| STORED
| TEXT
| THAN
| TIME
| TIMESTAMP
| TINYBLOB
| TINYINT
| TINYTEXT
| TRANSACTION
| TRIGGER
| TRUNCATE
| UNCOMMITTED
| UNBOUNDED
| UNSIGNED
| UNUSED
| VARBINARY
| VARCHAR
| VARIABLES
| VIEW
| VINDEX
| VISIBLE
| VIRTUAL
| WITH
| WRITE
| YEAR
| ZEROFILL

openb:
  '('
  {
    if incNesting(yylex) {
      yylex.Error("max nesting level reached")
      return 1
    }
  }

closeb:
  ')'
  {
    decNesting(yylex)
  }

force_eof:
{
  forceEOF(yylex)
}

ddl_force_eof:
  {
    forceEOF(yylex)
  }
| openb
  {
    forceEOF(yylex)
  }
| reserved_sql_id
  {
    forceEOF(yylex)
  }
