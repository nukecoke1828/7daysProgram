package schema

import (
	"go/ast"
	"reflect"

	"github.com/nukecoke1828/7daysProgram/Geeorm/dialect"
)

type Field struct {
	Name string // 数据库字段名
	Type string // 数据库字段类型
	Tag  string // 字段标签
}

type Schema struct {
	Model      interface{}       // 原始结构体实例
	Name       string            // 表名
	Fields     []*Field          // 字段列表
	FieldNames []string          // 字段名列表
	fieldMap   map[string]*Field // 字段名-字段映射
}

// 获取字段信息
func (Schema *Schema) GetField(name string) *Field {
	return Schema.fieldMap[name]
}

// 解析结构体，获取字段信息
// dest：结构体实例
func Parse(dest interface{}, d dialect.Dialect) *Schema {
	modelType := reflect.Indirect(reflect.ValueOf(dest)).Type() // 获取指针指向的类型
	schema := &Schema{                                          // 初始化Schema
		Model:    dest,             // 保留原始对象指针
		Name:     modelType.Name(), // 使用结构体名作为表名
		fieldMap: make(map[string]*Field),
	}
	for i := 0; i < modelType.NumField(); i++ { // 遍历结构体字段
		p := modelType.Field(i)
		// !p.Anonymous：排除嵌入式字段（非匿名字段）
		// ast.IsExported(p.Name)：只处理导出字段（首字母大写）
		if !p.Anonymous && ast.IsExported(p.Name) {
			field := &Field{
				Name: p.Name,
				// reflect.New(p.Type) 得到 *T，
				// 再用 Indirect 拿到 T，
				// 最后交给方言 DataTypeOF 得到 "text" / "integer" / "datetime" 等
				Type: d.DataTypeOF(reflect.Indirect(reflect.New(p.Type))),
			}
			if v, ok := p.Tag.Lookup("geeorm"); ok { // 解析tag
				field.Tag = v
			}
			schema.Fields = append(schema.Fields, field)
			schema.FieldNames = append(schema.FieldNames, p.Name)
			schema.fieldMap[p.Name] = field
		}
	}
	return schema
}

// 把对象实例转成“列值切片”(把「实例对象」翻译成「按列顺序排好的值切片」，供 SQL 占位符使用)
func (schema *Schema) RecordValues(dest interface{}) []interface{} {
	destValue := reflect.Indirect(reflect.ValueOf(dest)) // 获取指针指向的实例
	var fieldValues []interface{}
	for _, field := range schema.Fields {
		// destValue.FieldByName("Age")	根据名字反射取值
		// .Interface()	把 reflect.Value 还原成普通值
		fieldValues = append(fieldValues, destValue.FieldByName(field.Name).Interface())
	}
	return fieldValues
}
