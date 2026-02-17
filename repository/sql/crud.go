package sql

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// orderedColumn holds column name and struct field index for stable ordering.
type orderedColumn struct {
	Name  string
	Index int
}

var orderedColumnsCache sync.Map // map[reflect.Type][]orderedColumn

var (
	uuidTypeRef = reflect.TypeOf(uuid.UUID{})
	timeTypeRef = reflect.TypeOf(time.Time{})
)

// getOrderedColumns returns db-tagged columns in struct field order.
func getOrderedColumns(typ reflect.Type) []orderedColumn {
	if typ.Kind() != reflect.Struct {
		return nil
	}
	key := typ
	if v, ok := orderedColumnsCache.Load(key); ok {
		return v.([]orderedColumn)
	}
	var cols []orderedColumn
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.TrimSpace(tag)
		if idx := strings.Index(name, ","); idx >= 0 {
			name = strings.TrimSpace(name[:idx])
		}
		if name == "" {
			continue
		}
		cols = append(cols, orderedColumn{Name: name, Index: i})
	}
	orderedColumnsCache.Store(key, cols)
	return cols
}

// isFieldZero returns true if v is the zero value for its type (nil ptr, zero int, uuid.Nil, empty string, etc.).
// For pointer types (e.g. *uuid.UUID), the pointer is considered zero if it is nil or if it points to a zero value.
func isFieldZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return true
		}
		return isFieldZero(v.Elem())
	case reflect.Slice, reflect.Map:
		return v.IsNil()
	case reflect.Struct:
		if v.Type() == uuidTypeRef {
			return v.Interface().(uuid.UUID) == uuid.Nil
		}
		if v.Type() == timeTypeRef {
			return v.Interface().(time.Time).IsZero()
		}
	}
	return v.IsZero()
}

// IsEntityIDZero returns true if the entity's ID field (matching idColumn) is zero or nil.
// Use this to decide whether to omit ID from INSERT so the DB can set it via DEFAULT.
func IsEntityIDZero[T any](entity *T, idColumn string) bool {
	if entity == nil || idColumn == "" {
		return true
	}
	typ := reflect.TypeOf(entity).Elem()
	val := reflect.ValueOf(entity).Elem()
	idColLower := strings.ToLower(idColumn)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.TrimSpace(tag)
		if idx := strings.Index(name, ","); idx >= 0 {
			name = strings.TrimSpace(name[:idx])
		}
		if strings.ToLower(name) != idColLower {
			continue
		}
		return isFieldZero(val.Field(i))
	}
	return true
}

// BuildInsertQuery builds INSERT INTO table (cols...) VALUES (placeholders) using dialect.
// When excludeIDColumn is true, the column matching idColumn is omitted (for DB default).
func BuildInsertQuery(table, idColumn string, dialect Dialect, typ reflect.Type, excludeIDColumn bool) string {
	if dialect == nil {
		dialect = DefaultDialect
	}
	cols := getOrderedColumns(typ)
	if len(cols) == 0 {
		return ""
	}
	idColLower := strings.ToLower(idColumn)
	var names []string
	var placeholders []string
	argIdx := 1
	for _, c := range cols {
		if excludeIDColumn && strings.ToLower(c.Name) == idColLower {
			continue
		}
		names = append(names, c.Name)
		placeholders = append(placeholders, dialect.Placeholder(argIdx))
		argIdx++
	}
	if len(names) == 0 {
		return ""
	}
	return "INSERT INTO " + table + " (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
}

// fieldValueToAny converts a struct field value to a value suitable for SQL (INSERT/UPDATE).
func fieldValueToAny(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		return fieldValueToAny(v.Elem())
	}
	if v.Type() == uuidTypeRef {
		u := v.Interface().(uuid.UUID)
		return u.String()
	}
	// time.Time, int, string, bool, etc. pass through
	return v.Interface()
}

// ExtractInsertValues returns values for INSERT in the same order as columns (optionally excluding ID).
// When excludeIDColumn is true, the value for the column matching idColumn is omitted (for DB default).
func ExtractInsertValues[T any](entity *T, idColumn string, excludeIDColumn bool) []any {
	if entity == nil {
		return nil
	}
	typ := reflect.TypeOf(entity).Elem()
	cols := getOrderedColumns(typ)
	if len(cols) == 0 {
		return nil
	}
	idColLower := strings.ToLower(idColumn)
	val := reflect.ValueOf(entity).Elem()
	out := make([]any, 0, len(cols))
	for _, c := range cols {
		if excludeIDColumn && strings.ToLower(c.Name) == idColLower {
			continue
		}
		out = append(out, fieldValueToAny(val.Field(c.Index)))
	}
	return out
}

// RowScanner is implemented by *sql.Row. Used to scan RETURNING id without importing database/sql in this package.
type RowScanner interface {
	Scan(dest ...any) error
}

// scanUUID scans a single UUID column from row. database/sql converts driver []byte to string when scanning into *string.
func scanUUID(row RowScanner) (uuid.UUID, error) {
	var s string
	if err := row.Scan(&s); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(s)
}

// getEntityIDFieldInfo returns the ID field index and type for the entity's column matching idColumn.
func getEntityIDFieldInfo[T any](entity *T, idColumn string) (fieldIndex int, fieldType reflect.Type, ok bool) {
	if entity == nil || idColumn == "" {
		return 0, nil, false
	}
	typ := reflect.TypeOf(entity).Elem()
	idColLower := strings.ToLower(idColumn)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.TrimSpace(tag)
		if idx := strings.Index(name, ","); idx >= 0 {
			name = strings.TrimSpace(name[:idx])
		}
		if strings.ToLower(name) != idColLower {
			continue
		}
		return i, f.Type, true
	}
	return 0, nil, false
}

// IsEntityIDFieldInt64 returns true if the entity's ID field is int64 or *int64 (so LastInsertId can be used).
func IsEntityIDFieldInt64[T any](entity *T, idColumn string) bool {
	_, ft, ok := getEntityIDFieldInfo(entity, idColumn)
	if !ok {
		return false
	}
	if ft.Kind() == reflect.Int64 {
		return true
	}
	if ft.Kind() == reflect.Ptr && ft.Elem().Kind() == reflect.Int64 {
		return true
	}
	return false
}

// ScanReturnedIDAndSetEntity runs row.Scan and sets the entity's ID field from the returned value.
// Supports uuid.UUID, *uuid.UUID, string, int64, *int64. Used after INSERT ... RETURNING id for DB-generated IDs.
func ScanReturnedIDAndSetEntity[T any](entity *T, idColumn string, row RowScanner) error {
	if entity == nil || idColumn == "" || row == nil {
		return nil
	}
	idx, ft, ok := getEntityIDFieldInfo(entity, idColumn)
	if !ok {
		return nil
	}
	val := reflect.ValueOf(entity).Elem()
	field := val.Field(idx)
	if !field.CanSet() {
		return nil
	}

	switch ft {
	case uuidTypeRef:
		u, err := scanUUID(row)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(u))
		return nil
	}
	if ft.Kind() == reflect.Ptr && ft.Elem() == uuidTypeRef {
		u, err := scanUUID(row)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(&u))
		return nil
	}
	if ft.Kind() == reflect.String {
		var s string
		if err := row.Scan(&s); err != nil {
			return err
		}
		field.SetString(s)
		return nil
	}
	if ft.Kind() == reflect.Int64 {
		var i int64
		if err := row.Scan(&i); err != nil {
			return err
		}
		field.SetInt(i)
		return nil
	}
	if ft.Kind() == reflect.Ptr && ft.Elem().Kind() == reflect.Int64 {
		var i int64
		if err := row.Scan(&i); err != nil {
			return err
		}
		field.Set(reflect.ValueOf(&i))
		return nil
	}
	return nil
}

// SetEntityID sets the entity's ID field to id if it is an int64 column named idColumn (case-insensitive).
func SetEntityID[T any](entity *T, id int64, idColumn string) error {
	if entity == nil || idColumn == "" {
		return nil
	}
	typ := reflect.TypeOf(entity).Elem()
	val := reflect.ValueOf(entity).Elem()
	idColLower := strings.ToLower(idColumn)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.TrimSpace(tag)
		if idx := strings.Index(name, ","); idx >= 0 {
			name = strings.TrimSpace(name[:idx])
		}
		if strings.ToLower(name) != idColLower {
			continue
		}
		field := val.Field(i)
		if field.Kind() == reflect.Ptr {
			if field.Type().Elem().Kind() != reflect.Int64 {
				return nil
			}
			field.Set(reflect.ValueOf(&id))
			return nil
		}
		if field.Kind() == reflect.Int64 && field.CanSet() {
			field.SetInt(id)
			return nil
		}
		return nil
	}
	return nil
}

// BuildUpdateQuery builds UPDATE table SET col1=ph1, ... WHERE idCol=phN using dialect.
// idColumn is excluded from SET and used in WHERE.
func BuildUpdateQuery(table, idColumn string, dialect Dialect, typ reflect.Type) string {
	if dialect == nil {
		dialect = DefaultDialect
	}
	cols := getOrderedColumns(typ)
	idColLower := strings.ToLower(idColumn)
	var setCols []orderedColumn
	for _, c := range cols {
		if strings.ToLower(c.Name) == idColLower {
			continue
		}
		setCols = append(setCols, c)
	}
	if len(setCols) == 0 {
		return ""
	}
	parts := make([]string, len(setCols))
	for i, c := range setCols {
		parts[i] = c.Name + " = " + dialect.Placeholder(i+1)
	}
	whereArgIdx := len(setCols) + 1
	return "UPDATE " + table + " SET " + strings.Join(parts, ", ") + " WHERE " + idColumn + " = " + dialect.Placeholder(whereArgIdx)
}

// ExtractUpdateValues returns values for UPDATE SET clause in column order (excluding id), then appends idVal.
func ExtractUpdateValues[T any](entity *T, idVal any, idColumn string) []any {
	if entity == nil {
		return nil
	}
	typ := reflect.TypeOf(entity).Elem()
	cols := getOrderedColumns(typ)
	idColLower := strings.ToLower(idColumn)
	val := reflect.ValueOf(entity).Elem()
	var out []any
	for _, c := range cols {
		if strings.ToLower(c.Name) == idColLower {
			continue
		}
		out = append(out, fieldValueToAny(val.Field(c.Index)))
	}
	out = append(out, idVal)
	return out
}
