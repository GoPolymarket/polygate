package main

import (
	"fmt"
	"reflect"

	"github.com/GoPolymarket/polymarket-go-sdk"
)

func main() {
	client := polymarket.NewClient()
	t := reflect.TypeOf(client)
	
	fmt.Println("--- Client Methods ---")
	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		fmt.Printf("Method: %s\n", method.Name)
	}

	// 检查是否有 Relayer 字段
	v := reflect.ValueOf(client).Elem()
	typeOfV := v.Type()
	
	fmt.Println("\n--- Client Fields ---")
	for i := 0; i < v.NumField(); i++ {
		field := typeOfV.Field(i)
		fmt.Printf("Field: %s (Type: %s)\n", field.Name, field.Type)
	}
}
