package object_storage

import (
	"fmt"
	"reflect"
	"time"

	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/reader"
	parquettypes "github.com/xitongsys/parquet-go/types"
)

func parquetColumns(pr *reader.ParquetReader) ([]ParquetColumn, error) {
	if pr == nil || pr.SchemaHandler == nil || len(pr.SchemaHandler.SchemaElements) == 0 {
		return nil, fmt.Errorf("Parquet file has no schema")
	}
	elements := pr.SchemaHandler.SchemaElements
	root := elements[0]
	childCount := int(root.GetNumChildren())
	if childCount == 0 {
		return []ParquetColumn{}, nil
	}
	if childCount != len(elements)-1 {
		return nil, fmt.Errorf("unsupported nested Parquet schema: only flat primitive columns are supported")
	}

	columns := make([]ParquetColumn, 0, childCount)
	for i := 1; i < len(elements); i++ {
		element := elements[i]
		name := pr.SchemaHandler.GetExName(i)
		if element.GetNumChildren() != 0 {
			return nil, fmt.Errorf("unsupported Parquet column %q: nested fields are not supported", name)
		}
		repetition := element.GetRepetitionType()
		if repetition == parquet.FieldRepetitionType_REPEATED {
			return nil, fmt.Errorf("unsupported Parquet column %q: repeated fields are not supported", name)
		}
		if element.Type == nil {
			return nil, fmt.Errorf("unsupported Parquet column %q: physical type is missing", name)
		}
		standardType, err := parquetStandardType(element)
		if err != nil {
			return nil, fmt.Errorf("unsupported Parquet column %q: %w", name, err)
		}
		logicalType := ""
		if element.GetLogicalType() != nil {
			logicalType = element.GetLogicalType().String()
		} else if element.ConvertedType != nil {
			logicalType = element.GetConvertedType().String()
		}
		columns = append(columns, ParquetColumn{
			Name:           name,
			Type:           element.GetType().String(),
			StandardType:   standardType,
			RepetitionType: repetition.String(),
			LogicalType:    logicalType,
			Precision:      element.GetPrecision(),
			Scale:          element.GetScale(),
		})
	}
	return columns, nil
}

func parquetStandardType(element *parquet.SchemaElement) (string, error) {
	if logical := element.GetLogicalType(); logical != nil {
		switch {
		case logical.GetSTRING() != nil, logical.GetENUM() != nil:
			return "string", nil
		case logical.GetJSON() != nil:
			return "json", nil
		case logical.GetUUID() != nil:
			return "uuid", nil
		case logical.GetDECIMAL() != nil:
			return "decimal", nil
		case logical.GetDATE() != nil:
			return "date", nil
		case logical.GetTIME() != nil:
			return "time", nil
		case logical.GetTIMESTAMP() != nil:
			return "timestamp", nil
		case logical.GetINTEGER() != nil:
			if element.GetType() == parquet.Type_INT64 {
				return "bigint", nil
			}
			return "integer", nil
		case logical.GetBSON() != nil:
			return "binary", nil
		case logical.GetMAP() != nil, logical.GetLIST() != nil:
			return "", fmt.Errorf("nested MAP/LIST values are not supported")
		}
	}

	if converted := element.ConvertedType; converted != nil {
		switch *converted {
		case parquet.ConvertedType_UTF8, parquet.ConvertedType_ENUM:
			return "string", nil
		case parquet.ConvertedType_JSON:
			return "json", nil
		case parquet.ConvertedType_BSON, parquet.ConvertedType_INTERVAL:
			return "binary", nil
		case parquet.ConvertedType_DECIMAL:
			return "decimal", nil
		case parquet.ConvertedType_DATE:
			return "date", nil
		case parquet.ConvertedType_TIME_MILLIS, parquet.ConvertedType_TIME_MICROS:
			return "time", nil
		case parquet.ConvertedType_TIMESTAMP_MILLIS, parquet.ConvertedType_TIMESTAMP_MICROS:
			return "timestamp", nil
		case parquet.ConvertedType_INT_8, parquet.ConvertedType_INT_16,
			parquet.ConvertedType_INT_32, parquet.ConvertedType_UINT_8,
			parquet.ConvertedType_UINT_16, parquet.ConvertedType_UINT_32:
			return "integer", nil
		case parquet.ConvertedType_INT_64, parquet.ConvertedType_UINT_64:
			return "bigint", nil
		case parquet.ConvertedType_MAP, parquet.ConvertedType_MAP_KEY_VALUE,
			parquet.ConvertedType_LIST:
			return "", fmt.Errorf("nested MAP/LIST values are not supported")
		}
	}

	switch element.GetType() {
	case parquet.Type_BOOLEAN:
		return "boolean", nil
	case parquet.Type_INT32:
		return "integer", nil
	case parquet.Type_INT64:
		return "bigint", nil
	case parquet.Type_INT96:
		return "timestamp", nil
	case parquet.Type_FLOAT:
		return "float", nil
	case parquet.Type_DOUBLE:
		return "double", nil
	case parquet.Type_BYTE_ARRAY, parquet.Type_FIXED_LEN_BYTE_ARRAY:
		return "binary", nil
	default:
		return "", fmt.Errorf("physical type %s is not supported", element.GetType())
	}
}

func parquetIntegerIsUnsigned(element *parquet.SchemaElement) bool {
	if logical := element.GetLogicalType(); logical != nil && logical.GetINTEGER() != nil {
		return !logical.GetINTEGER().GetIsSigned()
	}
	if element.ConvertedType == nil {
		return false
	}
	switch element.GetConvertedType() {
	case parquet.ConvertedType_UINT_8, parquet.ConvertedType_UINT_16,
		parquet.ConvertedType_UINT_32, parquet.ConvertedType_UINT_64:
		return true
	default:
		return false
	}
}

func parquetRowToMap(item interface{}, columns []ParquetColumn, pr *reader.ParquetReader) (map[string]interface{}, error) {
	value := reflect.ValueOf(item)
	for value.IsValid() && value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil, fmt.Errorf("decoded Parquet row is nil")
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil, fmt.Errorf("decoded Parquet row has unexpected type %T", item)
	}

	row := make(map[string]interface{}, len(columns))
	for i, column := range columns {
		schemaIndex := i + 1 // Flat schemas have one root element followed by their primitive columns.
		fieldName := pr.SchemaHandler.GetInName(schemaIndex)
		field := value.FieldByName(fieldName)
		if !field.IsValid() || !field.CanInterface() {
			return nil, fmt.Errorf("decoded Parquet row is missing column %q", column.Name)
		}
		value, err := parquetValue(field, column, pr.Footer.Schema[schemaIndex])
		if err != nil {
			return nil, fmt.Errorf("failed to decode Parquet column %q: %w", column.Name, err)
		}
		row[column.Name] = value
	}
	return row, nil
}

func parquetValue(value reflect.Value, column ParquetColumn, element *parquet.SchemaElement) (interface{}, error) {
	for value.IsValid() && (value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil, nil
	}

	if column.StandardType == "decimal" {
		switch value.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return parquettypes.DECIMAL_INT_ToString(value.Int(), int(column.Precision), int(column.Scale)), nil
		case reflect.String:
			return parquettypes.DECIMAL_BYTE_ARRAY_ToString([]byte(value.String()), int(column.Precision), int(column.Scale)), nil
		default:
			return nil, fmt.Errorf("decimal physical value has unsupported Go type %s", value.Type())
		}
	}
	if element.GetType() == parquet.Type_INT96 {
		if value.Kind() != reflect.String {
			return nil, fmt.Errorf("INT96 value has unsupported Go type %s", value.Type())
		}
		if value.Len() != 12 {
			return nil, fmt.Errorf("INT96 value must contain 12 bytes")
		}
		return parquettypes.INT96ToTime(value.String()), nil
	}
	if column.StandardType == "uuid" {
		var raw []byte
		switch value.Kind() {
		case reflect.String:
			raw = []byte(value.String())
		case reflect.Slice:
			if value.Type().Elem().Kind() == reflect.Uint8 {
				raw = value.Bytes()
			}
		}
		if len(raw) == 36 {
			return string(raw), nil
		}
		if len(raw) != 16 {
			return nil, fmt.Errorf("UUID value must contain 16 bytes")
		}
		return fmt.Sprintf("%x-%x-%x-%x-%x", raw[:4], raw[4:6], raw[6:8], raw[8:10], raw[10:]), nil
	}
	if column.StandardType == "time" {
		if value.Kind() != reflect.Int32 && value.Kind() != reflect.Int64 {
			return nil, fmt.Errorf("time physical value has unsupported Go type %s", value.Type())
		}
		unitNanos := int64(0)
		format := "15:04:05.000000000"
		if logical := element.GetLogicalType(); logical != nil && logical.GetTIME() != nil {
			unit := logical.GetTIME().GetUnit()
			switch {
			case unit.GetMILLIS() != nil:
				unitNanos, format = int64(time.Millisecond), "15:04:05.000"
			case unit.GetMICROS() != nil:
				unitNanos, format = int64(time.Microsecond), "15:04:05.000000"
			case unit.GetNANOS() != nil:
				unitNanos = 1
			}
		}
		if element.ConvertedType != nil {
			switch element.GetConvertedType() {
			case parquet.ConvertedType_TIME_MILLIS:
				unitNanos, format = int64(time.Millisecond), "15:04:05.000"
			case parquet.ConvertedType_TIME_MICROS:
				unitNanos, format = int64(time.Microsecond), "15:04:05.000000"
			}
		}
		if unitNanos == 0 {
			return nil, fmt.Errorf("time unit is not supported")
		}
		return time.Unix(0, value.Int()*unitNanos).UTC().Format(format), nil
	}
	if (column.StandardType == "integer" || column.StandardType == "bigint") && parquetIntegerIsUnsigned(element) {
		switch value.Kind() {
		case reflect.Int32:
			return uint32(value.Int()), nil
		case reflect.Int64:
			return uint64(value.Int()), nil
		}
	}
	if column.StandardType == "date" && value.Kind() == reflect.Int32 {
		return time.Unix(int64(value.Int())*24*60*60, 0).UTC().Format("2006-01-02"), nil
	}
	if column.StandardType == "timestamp" && value.Kind() == reflect.Int64 {
		if logical := element.GetLogicalType(); logical != nil {
			if timestamp := logical.GetTIMESTAMP(); timestamp != nil {
				unit := timestamp.GetUnit()
				switch {
				case unit.GetMILLIS() != nil:
					return time.UnixMilli(value.Int()).UTC(), nil
				case unit.GetMICROS() != nil:
					return time.UnixMicro(value.Int()).UTC(), nil
				case unit.GetNANOS() != nil:
					return time.Unix(0, value.Int()).UTC(), nil
				}
			}
		}
		if element.ConvertedType != nil {
			switch element.GetConvertedType() {
			case parquet.ConvertedType_TIMESTAMP_MILLIS:
				return time.UnixMilli(value.Int()).UTC(), nil
			case parquet.ConvertedType_TIMESTAMP_MICROS:
				return time.UnixMicro(value.Int()).UTC(), nil
			}
		}
	}
	if column.StandardType == "binary" {
		switch value.Kind() {
		case reflect.String:
			return []byte(value.String()), nil
		case reflect.Slice:
			if value.Type().Elem().Kind() == reflect.Uint8 {
				return value.Bytes(), nil
			}
		}
	}
	if value.CanInterface() {
		return value.Interface(), nil
	}
	return nil, fmt.Errorf("Parquet value of type %s is inaccessible", value.Type())
}
