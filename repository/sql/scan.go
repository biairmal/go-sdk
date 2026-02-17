package sql

import (
	"database/sql"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var columnMappingCache sync.Map // map[reflect.Type]map[string]int (column name lower -> field index)

var uuidType = reflect.TypeOf(uuid.UUID{})

// ScanRow maps one row from rows into *T using struct tag `db:"column_name"`.
// Fields without `db` or with `db:"-"` are skipped. Column names are matched case-insensitively.
// Supports common types, uuid.UUID and *uuid.UUID (scanned via string then parsed), and *time.Time.
// Caller must advance rows (e.g. rows.Next()) before calling ScanRow.
func ScanRow[T any](rows *sql.Rows) (*T, error) {
	var zero T
	typ := reflect.TypeOf(&zero).Elem()
	if typ.Kind() != reflect.Struct {
		return nil, nil
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	mapping := getColumnMapping(typ)
	ptr := reflect.New(typ)
	dest := make([]any, len(columns))
	uuidScans := make([]*string, len(columns))
	for i, col := range columns {
		idx, ok := mapping[strings.ToLower(col)]
		if !ok {
			var dummy any
			dest[i] = &dummy
			continue
		}
		field := ptr.Elem().Field(idx)
		if !field.CanSet() {
			var dummy any
			dest[i] = &dummy
			continue
		}
		ft := field.Type()
		if ft == uuidType {
			dest[i] = &uuidScans[i]
			continue
		}
		if ft.Kind() == reflect.Ptr && ft.Elem() == uuidType {
			dest[i] = &uuidScans[i]
			continue
		}
		dest[i] = field.Addr().Interface()
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	for i, col := range columns {
		idx, ok := mapping[strings.ToLower(col)]
		if !ok {
			continue
		}
		field := ptr.Elem().Field(idx)
		ft := field.Type()
		if ft == uuidType {
			if uuidScans[i] != nil && *uuidScans[i] != "" {
				if u, err := uuid.Parse(*uuidScans[i]); err == nil {
					field.Set(reflect.ValueOf(u))
				}
			}
			continue
		}
		if ft.Kind() == reflect.Ptr && ft.Elem() == uuidType {
			if uuidScans[i] != nil && *uuidScans[i] != "" {
				if u, err := uuid.Parse(*uuidScans[i]); err == nil {
					field.Set(reflect.ValueOf(&u))
				}
			}
		}
	}
	return ptr.Interface().(*T), nil
}

// ReflectScan returns a function that maps rows to *T using struct tag `db:"column_name"`.
// Deprecated: use ScanRow[T] directly for new code.
func ReflectScan[T any]() func(*sql.Rows) (*T, error) {
	return ScanRow[T]
}

// getColumnMapping returns column name (lower) -> struct field index for typ.
func getColumnMapping(typ reflect.Type) map[string]int {
	key := typ
	if v, ok := columnMappingCache.Load(key); ok {
		return v.(map[string]int)
	}
	m := make(map[string]int)
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
		m[strings.ToLower(name)] = i
	}
	columnMappingCache.Store(key, m)
	return m
}

// NullTime is used to scan nullable time into *time.Time.
type NullTime struct {
	Time  time.Time
	Valid bool
}

// Scan implements sql.Scanner.
func (n *NullTime) Scan(value any) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	n.Valid = true
	switch v := value.(type) {
	case time.Time:
		n.Time = v
		return nil
	default:
		// Fallback for drivers that return different types
		var nt sql.NullTime
		if err := nt.Scan(value); err != nil {
			return err
		}
		n.Valid = nt.Valid
		if nt.Valid {
			n.Time = nt.Time
		}
		return nil
	}
}
