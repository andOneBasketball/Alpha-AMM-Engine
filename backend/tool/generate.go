package main

// gorm gen configure

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gorm.io/gen"
)

var tables = []string{
	"alpha_amm_pool",
	"alpha_pool_liquidity_event",
	"alpha_token",
}

const MySQLDSN = "root:a1b3c3d4@tcp(172.20.57.191:3306)/alpha_amm_engine?parseTime=true&loc=Local&charset=utf8mb4&collation=utf8mb4_unicode_ci"

func connectDB(dsn string) *gorm.DB {
	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		panic(fmt.Errorf("connect db fail: %w", err))
	}
	return db
}

func main() {
	// 指定生成代码的具体相对目录(相对当前文件)，默认为：./query
	// 默认生成需要使用WithContext之后才可以查询的代码，但可以通过设置gen.WithoutContext禁用该模式
	g := gen.NewGenerator(gen.Config{
		// 默认会在 OutPath 目录生成CRUD代码，并且同目录下生成 model 包
		// 所以OutPath最终package不能设置为model，在有数据库表同步的情况下会产生冲突
		// 若一定要使用可以通过ModelPkgPath单独指定model package的名称
		OutPath:      "../internal/dao",
		ModelPkgPath: "../internal/dao/sqlmodel",

		// gen.WithoutContext：禁用WithContext模式
		// gen.WithDefaultQuery：生成一个全局Query对象Q
		// gen.WithQueryInterface：生成Query接口
		Mode: gen.WithDefaultQuery | gen.WithQueryInterface,
	})

	// 将数据库的 decimal 映射为 decimal.Decimal
	g.WithImportPkgPath("github.com/shopspring/decimal")
	g.WithDataTypeMap(map[string]func(columnType gorm.ColumnType) (dataType string){
		"decimal": func(columnType gorm.ColumnType) (dataType string) {
			return "decimal.Decimal"
		},
		"numeric": func(columnType gorm.ColumnType) (dataType string) {
			return "decimal.Decimal"
		},
	})

	// 通常复用项目中已有的SQL连接配置db(*gorm.DB)
	// 非必需，但如果需要复用连接时的gorm.Config或需要连接数据库同步表信息则必须设置
	g.UseDB(connectDB(MySQLDSN))

	// 从连接的数据库为所有表生成Model结构体和CRUD代码
	// 也可以手动指定需要生成代码的数据表
	models := make([]any, 0, 32)
	for _, table := range tables {
		models = append(models, g.GenerateModel(table))
	}
	g.ApplyBasic(models...)

	// 执行并生成代码
	g.Execute()
}
